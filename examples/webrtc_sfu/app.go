package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

//go:embed ui/*
var uiFS embed.FS

type app struct {
	logger   *slog.Logger
	api      *webrtc.API
	upgrader websocket.Upgrader
	static   http.Handler

	nextPeerID atomic.Uint64

	roomsMu sync.Mutex
	rooms   map[string]*room
}

type room struct {
	app    *app
	id     string
	logger *slog.Logger

	mu     sync.Mutex
	peers  map[string]*peer
	tracks map[string]*roomTrack
}

type roomTrack struct {
	id       string
	ownerID  string
	kind     string
	streamID string
	trackID  string
	local    *webrtc.TrackLocalStaticRTP
}

type peer struct {
	id   string
	name string

	logger *slog.Logger
	room   *room
	ws     *websocket.Conn
	pc     *webrtc.PeerConnection

	closeOnce sync.Once
	wsMu      sync.Mutex
	mu        sync.Mutex

	closed           bool
	negotiating      bool
	needsNegotiation bool
	senders          map[string]*webrtc.RTPSender
	chat             *webrtc.DataChannel
}

type peerSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type signalMessage struct {
	Type        string                     `json:"type"`
	ID          string                     `json:"id,omitempty"`
	Room        string                     `json:"room,omitempty"`
	Name        string                     `json:"name,omitempty"`
	Error       string                     `json:"error,omitempty"`
	Peers       []peerSummary              `json:"peers,omitempty"`
	Description *webrtc.SessionDescription `json:"description,omitempty"`
	Candidate   *webrtc.ICECandidateInit   `json:"candidate,omitempty"`
}

type chatEnvelope struct {
	Type string `json:"type"`
	From string `json:"from"`
	Name string `json:"name"`
	Text string `json:"text"`
	At   string `json:"at"`
}

func newApp(logger *slog.Logger) (*app, error) {
	if logger == nil {
		logger = slog.Default()
	}

	staticFS, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return nil, fmt.Errorf("sub ui fs: %w", err)
	}

	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, fmt.Errorf("register default codecs: %w", err)
	}

	return &app{
		logger: logger,
		api:    webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine)),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		static: http.FileServer(http.FS(staticFS)),
		rooms:  make(map[string]*room),
	}, nil
}

func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/healthz":
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
	case "/ws":
		a.handleWebSocket(w, r)
	default:
		a.static.ServeHTTP(w, r)
	}
}

func (a *app) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := normalizeRoomID(r.URL.Query().Get("room"))
	name := normalizeDisplayName(r.URL.Query().Get("name"))

	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		a.logger.Error("upgrade websocket failed", "error", err)
		return
	}

	peer, err := a.newPeer(conn, roomID, name)
	if err != nil {
		_ = conn.Close()
		a.logger.Error("create peer failed", "error", err, "room", roomID)
		return
	}

	room := a.getOrCreateRoom(roomID)
	peer.room = room
	room.addPeer(peer)
	if err := peer.sendSignal(signalMessage{
		Type: "welcome",
		ID:   peer.id,
		Room: room.id,
		Name: peer.name,
	}); err != nil {
		peer.closeWithReason("send welcome failed", err)
		return
	}

	if err := peer.readLoop(r.Context()); err != nil && !isExpectedClose(err) {
		peer.logger.Info("peer websocket closed", "error", err)
	}
	peer.close()
}

func (a *app) newPeer(ws *websocket.Conn, roomID, name string) (*peer, error) {
	pc, err := a.api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("new peer connection: %w", err)
	}

	id := fmt.Sprintf("peer-%03d", a.nextPeerID.Add(1))
	p := &peer{
		id:      id,
		name:    name,
		logger:  a.logger.With("peer", id, "room", roomID),
		ws:      ws,
		pc:      pc,
		senders: make(map[string]*webrtc.RTPSender),
	}

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		init := candidate.ToJSON()
		if err := p.sendSignal(signalMessage{
			Type:      "candidate",
			Candidate: &init,
		}); err != nil && !isExpectedClose(err) {
			p.logger.Debug("send local candidate failed", "error", err)
		}
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		p.logger.Info("peer connection state changed", "state", state.String())
		switch state {
		case webrtc.PeerConnectionStateFailed,
			webrtc.PeerConnectionStateClosed,
			webrtc.PeerConnectionStateDisconnected:
			p.close()
		}
	})

	pc.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		p.logger.Info(
			"received remote track",
			"kind", remote.Kind().String(),
			"track_id", remote.ID(),
			"stream_id", remote.StreamID(),
			"ssrc", remote.SSRC(),
		)
		go p.handleRemoteTrack(remote)
	})

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		p.logger.Info("received data channel", "label", dc.Label())
		if dc.Label() != "chat" {
			return
		}
		p.bindChatChannel(dc)
	})

	return p, nil
}

func (a *app) getOrCreateRoom(id string) *room {
	a.roomsMu.Lock()
	defer a.roomsMu.Unlock()
	if existing := a.rooms[id]; existing != nil {
		return existing
	}
	r := &room{
		app:    a,
		id:     id,
		logger: a.logger.With("room", id),
		peers:  make(map[string]*peer),
		tracks: make(map[string]*roomTrack),
	}
	a.rooms[id] = r
	return r
}

func (r *room) addPeer(p *peer) {
	r.mu.Lock()
	r.peers[p.id] = p
	peers := r.peerSummariesLocked()
	tracks := r.trackListLocked()
	r.mu.Unlock()

	for _, track := range tracks {
		if p.ensureSender(track) {
			p.scheduleNegotiation()
		}
	}
	r.broadcastPeerList(peers)
}

func (r *room) removePeer(p *peer) {
	var (
		peers        []peerSummary
		remaining    []*peer
		removedTrack []string
	)

	r.mu.Lock()
	if _, ok := r.peers[p.id]; !ok {
		r.mu.Unlock()
		return
	}
	delete(r.peers, p.id)
	for id, track := range r.tracks {
		if track.ownerID == p.id {
			delete(r.tracks, id)
			removedTrack = append(removedTrack, id)
		}
	}
	for _, peer := range r.peers {
		remaining = append(remaining, peer)
	}
	peers = r.peerSummariesLocked()
	r.mu.Unlock()

	for _, other := range remaining {
		changed := false
		for _, trackID := range removedTrack {
			if other.removeSender(trackID) {
				changed = true
			}
		}
		if changed {
			other.scheduleNegotiation()
		}
	}
	if len(remaining) == 0 && r.app != nil {
		r.app.deleteRoomIfEmpty(r)
	}
	r.broadcastPeerList(peers)
}

func (a *app) deleteRoomIfEmpty(r *room) {
	if a == nil || r == nil {
		return
	}
	a.roomsMu.Lock()
	defer a.roomsMu.Unlock()
	if a.rooms[r.id] != r {
		return
	}
	r.mu.Lock()
	empty := len(r.peers) == 0
	r.mu.Unlock()
	if empty {
		delete(a.rooms, r.id)
	}
}

func (r *room) broadcastPeerList(peers []peerSummary) {
	r.mu.Lock()
	targets := make([]*peer, 0, len(r.peers))
	for _, peer := range r.peers {
		targets = append(targets, peer)
	}
	r.mu.Unlock()

	msg := signalMessage{
		Type:  "peers",
		Peers: peers,
	}
	for _, peer := range targets {
		if err := peer.sendSignal(msg); err != nil && !isExpectedClose(err) {
			peer.logger.Debug("send peers update failed", "error", err)
		}
	}
}

func (r *room) peerSummariesLocked() []peerSummary {
	out := make([]peerSummary, 0, len(r.peers))
	for _, peer := range r.peers {
		out = append(out, peerSummary{ID: peer.id, Name: peer.name})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (r *room) trackListLocked() []*roomTrack {
	out := make([]*roomTrack, 0, len(r.tracks))
	for _, track := range r.tracks {
		out = append(out, track)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].id < out[j].id
	})
	return out
}

func (r *room) publishTrack(owner *peer, remote *webrtc.TrackRemote) (*roomTrack, error) {
	trackID := makeRoomTrackID(owner.id, remote.StreamID(), remote.ID())
	local, err := webrtc.NewTrackLocalStaticRTP(remote.Codec().RTPCodecCapability, remote.ID(), remote.StreamID())
	if err != nil {
		return nil, fmt.Errorf("new local track: %w", err)
	}

	track := &roomTrack{
		id:       trackID,
		ownerID:  owner.id,
		kind:     remote.Kind().String(),
		streamID: remote.StreamID(),
		trackID:  remote.ID(),
		local:    local,
	}

	r.mu.Lock()
	r.tracks[track.id] = track
	targets := make([]*peer, 0, len(r.peers))
	for _, peer := range r.peers {
		if peer.id != owner.id {
			targets = append(targets, peer)
		}
	}
	r.mu.Unlock()

	for _, peer := range targets {
		if peer.ensureSender(track) {
			peer.scheduleNegotiation()
		}
	}
	return track, nil
}

func (r *room) unpublishTrack(trackID string) {
	var targets []*peer

	r.mu.Lock()
	if _, ok := r.tracks[trackID]; !ok {
		r.mu.Unlock()
		return
	}
	delete(r.tracks, trackID)
	for _, peer := range r.peers {
		targets = append(targets, peer)
	}
	r.mu.Unlock()

	for _, peer := range targets {
		if peer.removeSender(trackID) {
			peer.scheduleNegotiation()
		}
	}
}

func (r *room) broadcastChat(from *peer, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	r.mu.Lock()
	targets := make([]*peer, 0, len(r.peers))
	for _, peer := range r.peers {
		targets = append(targets, peer)
	}
	r.mu.Unlock()

	msg := chatEnvelope{
		Type: "chat",
		From: from.id,
		Name: from.name,
		Text: text,
		At:   time.Now().Format(time.RFC3339),
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		from.logger.Debug("marshal chat message failed", "error", err)
		return
	}

	for _, peer := range targets {
		if err := peer.sendChat(payload); err != nil && !isExpectedDataChannelError(err) {
			peer.logger.Debug("send chat message failed", "error", err)
		}
	}
}

func (p *peer) readLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var msg signalMessage
		if err := p.ws.ReadJSON(&msg); err != nil {
			return err
		}
		if err := p.handleSignal(msg); err != nil {
			_ = p.sendSignal(signalMessage{
				Type:  "error",
				Error: err.Error(),
			})
			return err
		}
	}
}

func (p *peer) handleSignal(msg signalMessage) error {
	switch msg.Type {
	case "offer":
		if msg.Description == nil {
			return errors.New("offer missing description")
		}
		if err := p.pc.SetRemoteDescription(*msg.Description); err != nil {
			return fmt.Errorf("set remote offer: %w", err)
		}
		answer, err := p.pc.CreateAnswer(nil)
		if err != nil {
			return fmt.Errorf("create answer: %w", err)
		}
		if err := p.pc.SetLocalDescription(answer); err != nil {
			return fmt.Errorf("set local answer: %w", err)
		}
		return p.sendSignal(signalMessage{
			Type:        "answer",
			Description: p.pc.LocalDescription(),
		})
	case "answer":
		if msg.Description == nil {
			return errors.New("answer missing description")
		}
		if err := p.pc.SetRemoteDescription(*msg.Description); err != nil {
			return fmt.Errorf("set remote answer: %w", err)
		}
		p.finishNegotiation()
		return nil
	case "candidate":
		if msg.Candidate == nil {
			return errors.New("candidate missing payload")
		}
		if err := p.pc.AddICECandidate(*msg.Candidate); err != nil {
			return fmt.Errorf("add ice candidate: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported signal type %q", msg.Type)
	}
}

func (p *peer) handleRemoteTrack(remote *webrtc.TrackRemote) {
	if p.room == nil {
		return
	}

	track, err := p.room.publishTrack(p, remote)
	if err != nil {
		p.logger.Error("publish track failed", "error", err)
		return
	}
	defer p.room.unpublishTrack(track.id)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if remote.Kind() == webrtc.RTPCodecTypeVideo {
		go p.sendPeriodicPLI(ctx, remote)
	}

	for {
		packet, _, err := remote.ReadRTP()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				p.logger.Debug("read rtp failed", "error", err, "track_id", remote.ID())
			}
			return
		}
		if err := track.local.WriteRTP(packet); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			p.logger.Debug("forward rtp failed", "error", err, "track_id", remote.ID())
		}
	}
}

func (p *peer) sendPeriodicPLI(ctx context.Context, remote *webrtc.TrackRemote) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if writeErr := p.pc.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{MediaSSRC: uint32(remote.SSRC())},
			}); writeErr != nil {
				p.logger.Debug("write pli failed", "error", writeErr)
				return
			}
		}
	}
}

func (p *peer) ensureSender(track *roomTrack) bool {
	p.mu.Lock()
	if p.closed || track.ownerID == p.id {
		p.mu.Unlock()
		return false
	}
	if _, ok := p.senders[track.id]; ok {
		p.mu.Unlock()
		return false
	}
	sender, err := p.pc.AddTrack(track.local)
	if err != nil {
		p.mu.Unlock()
		p.logger.Debug("add track failed", "error", err, "track", track.id)
		return false
	}
	p.senders[track.id] = sender
	p.mu.Unlock()

	go drainRTCP(sender)
	return true
}

func (p *peer) removeSender(trackID string) bool {
	p.mu.Lock()
	sender, ok := p.senders[trackID]
	if ok {
		delete(p.senders, trackID)
	}
	p.mu.Unlock()

	if !ok {
		return false
	}
	if err := p.pc.RemoveTrack(sender); err != nil {
		p.logger.Debug("remove track failed", "error", err, "track", trackID)
	}
	return true
}

func (p *peer) scheduleNegotiation() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	if p.negotiating {
		p.needsNegotiation = true
		p.mu.Unlock()
		return
	}
	p.negotiating = true
	p.mu.Unlock()

	go p.negotiate()
}

func (p *peer) negotiate() {
	offer, err := p.pc.CreateOffer(nil)
	if err != nil {
		p.logger.Debug("create offer failed", "error", err)
		p.finishNegotiation()
		return
	}
	if err := p.pc.SetLocalDescription(offer); err != nil {
		p.logger.Debug("set local offer failed", "error", err)
		p.finishNegotiation()
		return
	}
	if err := p.sendSignal(signalMessage{
		Type:        "offer",
		Description: p.pc.LocalDescription(),
	}); err != nil {
		p.logger.Debug("send offer failed", "error", err)
		p.finishNegotiation()
		return
	}
}

func (p *peer) finishNegotiation() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	again := p.needsNegotiation
	p.negotiating = false
	p.needsNegotiation = false
	p.mu.Unlock()

	if again {
		p.scheduleNegotiation()
	}
}

func (p *peer) bindChatChannel(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		p.mu.Lock()
		p.chat = dc
		p.mu.Unlock()
		p.logger.Info("chat channel open", "label", dc.Label())
	})
	dc.OnClose(func() {
		p.mu.Lock()
		if p.chat == dc {
			p.chat = nil
		}
		p.mu.Unlock()
		p.logger.Info("chat channel closed", "label", dc.Label())
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if p.room == nil {
			return
		}
		p.room.broadcastChat(p, string(msg.Data))
	})
}

func (p *peer) sendSignal(msg signalMessage) error {
	if p.ws == nil {
		return nil
	}
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	return p.ws.WriteJSON(msg)
}

func (p *peer) sendChat(payload []byte) error {
	p.mu.Lock()
	dc := p.chat
	closed := p.closed
	p.mu.Unlock()
	if closed || dc == nil || dc.ReadyState() != webrtc.DataChannelStateOpen {
		return nil
	}
	return dc.SendText(string(payload))
}

func (p *peer) closeWithReason(message string, err error) {
	if err != nil {
		p.logger.Info(message, "error", err)
	} else {
		p.logger.Info(message)
	}
	p.close()
}

func (p *peer) close() {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()

		if p.room != nil {
			p.room.removePeer(p)
		}
		if p.pc != nil {
			_ = p.pc.Close()
		}
		if p.ws != nil {
			_ = p.ws.Close()
		}
	})
}

func normalizeRoomID(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "demo"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "demo"
	}
	return b.String()
}

func normalizeDisplayName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "guest"
	}
	if len(s) > 32 {
		s = s[:32]
	}
	return s
}

func makeRoomTrackID(ownerID, streamID, trackID string) string {
	return ownerID + "/" + streamID + "/" + trackID
}

func drainRTCP(sender *webrtc.RTPSender) {
	if sender == nil {
		return
	}
	buf := make([]byte, 1500)
	for {
		if _, _, err := sender.Read(buf); err != nil {
			return
		}
	}
}

func isExpectedClose(err error) bool {
	return websocket.IsCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseAbnormalClosure,
	)
}

func isExpectedDataChannelError(err error) bool {
	return errors.Is(err, io.ErrClosedPipe)
}
