// Package net provides network abstractions built on the Noise Protocol.
package core

import (
	"errors"
	"iter"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

// PeerState represents the connection state of a peer.
type PeerState int

const (
	// PeerStateNew indicates a newly registered peer.
	PeerStateNew PeerState = iota
	// PeerStateConnecting indicates the peer is performing handshake.
	PeerStateConnecting
	// PeerStateEstablished indicates the peer has an active session.
	PeerStateEstablished
	// PeerStateFailed indicates the connection attempt failed.
	PeerStateFailed
)

func (s PeerState) String() string {
	switch s {
	case PeerStateNew:
		return "new"
	case PeerStateConnecting:
		return "connecting"
	case PeerStateEstablished:
		return "established"
	case PeerStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// PeerEvent is emitted when a peer's state changes.
type PeerEvent struct {
	PublicKey noise.PublicKey
	State     PeerState
}

// HostInfo contains information about the local host.
type HostInfo struct {
	PublicKey noise.PublicKey
	Addr      *net.UDPAddr
	PeerCount int
	RxBytes   uint64
	TxBytes   uint64
	LastSeen  time.Time

	// Observability counters (network hot path)
	DroppedOutputPackets  uint64 // dropped due to full outputChan
	DroppedDecryptPackets uint64 // dropped due to full decryptChan
	DroppedInboundPackets uint64 // dropped from ServiceMux direct-packet routing
	RPCRouteErrors        uint64 // failed to route RPC into ServiceMux
	KCPOutputErrors       uint64 // ServiceMux Output callback returned error
	DroppedPeerEvents     uint64 // peer events dropped by OnPeerEvent callback
}

// PeerInfo contains information about a peer.
type PeerInfo struct {
	PublicKey noise.PublicKey
	Endpoint  net.Addr
	State     PeerState
	RxBytes   uint64
	TxBytes   uint64
	LastSeen  time.Time
}

// Peer represents a complete peer with info and connection.
type Peer struct {
	Info *PeerInfo
	Conn *Conn
}

// Errors
var (
	ErrClosed          = errors.New("net: udp closed")
	ErrPeerNotFound    = errors.New("net: peer not found")
	ErrNoEndpoint      = errors.New("net: peer has no endpoint")
	ErrNoSession       = errors.New("net: peer has no established session")
	ErrHandshakeFailed = errors.New("net: handshake failed")
	ErrNoData          = errors.New("net: no data available")
)

// protoPacket represents a received non-KCP packet with its protocol byte.
type protoPacket struct {
	protocol byte
	payload  []byte
}

// packet represents a packet in the processing pipeline.
// It carries raw data and gets decrypted in parallel by workers.
// Consumers wait on the ready channel before accessing decrypted data.
type packet struct {
	// Input (set by ioLoop)
	data []byte       // buffer from pool (owns the memory)
	n    int          // actual data length
	from *net.UDPAddr // sender address

	// Output (set by decryptWorker)
	pk       noise.PublicKey // sender's public key (after decrypt)
	protocol byte            // protocol byte
	payload  []byte          // decrypted payload (slice into data or copy)
	payloadN int             // payload length
	err      error           // decrypt error (if any)

	// Reference count for packet ownership.
	// +1: decrypt path ownership (always)
	// +1: output path ownership (only if queued to outputChan)
	// Packet is returned to pool when refs reaches 0.
	refs atomic.Int32

	// Release guard: prevents double-release when multiple goroutines
	// race to release the same packet (e.g., during shutdown).
	released atomic.Bool

	// Synchronization
	ready chan struct{} // closed when decryption is complete
}

// outstandingPackets tracks the number of packets currently acquired from the pool
// but not yet released. Used for leak detection in tests.
var outstandingPackets atomic.Int64

// bufferPool provides reusable buffers for receiving UDP packets.
var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, noise.MaxPacketSize)
	},
}

// packetPool provides reusable packet structs.
var packetPool = sync.Pool{
	New: func() any {
		return &packet{
			ready: make(chan struct{}),
		}
	},
}

var afterDecryptTransportDecryptHook func()

// acquirePacket gets a packet from the pool and resets it.
func acquirePacket() *packet {
	outstandingPackets.Add(1)
	p := packetPool.Get().(*packet)
	p.data = bufferPool.Get().([]byte)
	p.n = 0
	p.pk = noise.PublicKey{}
	p.protocol = 0
	p.payload = nil
	p.payloadN = 0
	p.err = nil
	p.refs.Store(0)
	p.released.Store(false)
	p.ready = make(chan struct{})
	return p
}

// releasePacket returns a packet to the pool.
// Safe to call from multiple goroutines — only the first call takes effect.
func releasePacket(p *packet) {
	if !p.released.CompareAndSwap(false, true) {
		return
	}
	outstandingPackets.Add(-1)
	if p.data != nil {
		bufferPool.Put(p.data)
		p.data = nil
	}
	packetPool.Put(p)
}

func unrefPacket(p *packet) {
	if p.refs.Add(-1) == 0 {
		releasePacket(p)
	}
}

// UDP represents a UDP-based network using the Noise Protocol.
// It manages multiple peers, handles handshakes, and supports roaming.
type UDP struct {
	socket   *net.UDPConn
	localKey *noise.KeyPair

	// Socket configuration (for GSO/GRO, busy-poll, buffer sizes)
	socketConfig SocketConfig

	// Options
	allowUnknown     bool
	serviceMuxConfig ServiceMuxConfig

	// Peer management
	mu      sync.RWMutex
	peers   map[noise.PublicKey]*peerState
	byIndex map[uint32]*peerState // lookup by session index

	// Pending handshakes (as initiator)
	pending map[uint32]*pendingHandshake

	// Pipeline channels for async I/O processing
	decryptChan chan *packet   // ioLoop -> decryptWorkers
	outputChan  chan *packet   // ioLoop -> ReadFrom (same packet, wait for ready)
	closeChan   chan struct{}  // signal to stop goroutines
	wg          sync.WaitGroup // tracks running goroutines

	// Statistics
	totalRx  atomic.Uint64
	totalTx  atomic.Uint64
	lastSeen atomic.Value // time.Time

	// Observability counters (hot-path drops / routing failures)
	droppedOutputPackets  atomic.Uint64
	droppedDecryptPackets atomic.Uint64
	droppedInboundPackets atomic.Uint64
	rpcRouteErrors        atomic.Uint64
	kcpOutputErrors       atomic.Uint64
	droppedPeerEvents     atomic.Uint64
	// Peer state callback (set once via OnPeerEvent option; called synchronously).
	// Returns true if the event was consumed, false if dropped.
	onPeerEvent func(PeerEvent) bool

	// State
	closing atomic.Bool
	closed  atomic.Bool
}

// peerState holds the internal state for a peer.
type peerState struct {
	mu       sync.RWMutex
	pk       noise.PublicKey
	endpoint *net.UDPAddr
	session  *noise.Session
	hsState  *noise.HandshakeState // during handshake
	state    PeerState
	removed  bool
	rxBytes  uint64
	txBytes  uint64
	lastSeen time.Time

	// Stream multiplexing (initialized when session is established)
	serviceMux *ServiceMux
}

// pendingHandshake tracks an outgoing handshake.
type pendingHandshake struct {
	peer      *peerState
	hsState   *noise.HandshakeState
	localIdx  uint32
	done      chan error
	createdAt time.Time
}

// Option configures UDP options.
type Option func(*options)

type options struct {
	bindAddr          string
	allowUnknown      bool
	decryptWorkers    int // 0 = runtime.NumCPU()
	rawChanSize       int // 0 = use RawChanSize constant
	decryptedChanSize int // 0 = use DecryptedChanSize constant
	socketConfig      SocketConfig
	serviceMuxConfig  ServiceMuxConfig
	onPeerEvent       func(PeerEvent) bool
}

// WithBindAddr sets the local address to bind to.
// Default is ":0" (random port).
func WithBindAddr(addr string) Option {
	return func(o *options) {
		o.bindAddr = addr
	}
}

// WithAllowUnknown allows accepting connections from unknown peers.
func WithAllowUnknown(allow bool) Option {
	return func(o *options) {
		o.allowUnknown = allow
	}
}

// WithDecryptWorkers sets the number of parallel decrypt workers.
// Default is runtime.NumCPU().
func WithDecryptWorkers(n int) Option {
	return func(o *options) {
		o.decryptWorkers = n
	}
}

// WithRawChanSize sets the raw packet channel size.
// Default is RawChanSize (4096).
func WithRawChanSize(n int) Option {
	return func(o *options) {
		o.rawChanSize = n
	}
}

// WithDecryptedChanSize sets the decrypted packet channel size.
// Default is DecryptedChanSize (256).
func WithDecryptedChanSize(n int) Option {
	return func(o *options) {
		o.decryptedChanSize = n
	}
}

// WithSocketConfig sets the socket configuration (GSO, GRO, busy-poll, buffer sizes).
// Default is DefaultSocketConfig().
func WithSocketConfig(cfg SocketConfig) Option {
	return func(o *options) {
		o.socketConfig = cfg
	}
}

// WithServiceMuxConfig injects the per-peer ServiceMux configuration template.
// Transport-owned fields are filled by UDP when a peer session is established.
func WithServiceMuxConfig(cfg ServiceMuxConfig) Option {
	return func(o *options) {
		o.serviceMuxConfig = cfg
	}
}

// WithOnPeerEvent registers a callback invoked synchronously whenever a
// peer's state changes. The callback must not block. It returns true if the
// event was consumed, false if it was dropped (counted in DroppedPeerEvents).
func WithOnPeerEvent(fn func(PeerEvent) bool) Option {
	return func(o *options) {
		o.onPeerEvent = fn
	}
}

// NewUDP creates a new UDP network.
func NewUDP(key *noise.KeyPair, opts ...Option) (*UDP, error) {
	if key == nil {
		return nil, errors.New("net: key is required")
	}

	// Apply options
	o := &options{
		bindAddr: ":0",
	}
	for _, opt := range opts {
		opt(o)
	}

	// Resolve and bind address
	addr, err := net.ResolveUDPAddr("udp", o.bindAddr)
	if err != nil {
		return nil, err
	}

	socket, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	// Apply socket configuration (ApplySocketOptions handles zero values individually)
	socketConfig := o.socketConfig
	ApplySocketOptions(socket, socketConfig)

	rawSize := o.rawChanSize
	if rawSize <= 0 {
		rawSize = RawChanSize
	}
	decryptedSize := o.decryptedChanSize
	if decryptedSize <= 0 {
		decryptedSize = DecryptedChanSize
	}

	u := &UDP{
		socket:           socket,
		localKey:         key,
		socketConfig:     socketConfig,
		allowUnknown:     o.allowUnknown,
		serviceMuxConfig: o.serviceMuxConfig,
		peers:            make(map[noise.PublicKey]*peerState),
		byIndex:          make(map[uint32]*peerState),
		pending:          make(map[uint32]*pendingHandshake),
		decryptChan:      make(chan *packet, rawSize),
		outputChan:       make(chan *packet, decryptedSize),
		onPeerEvent:      o.onPeerEvent,
		closeChan:        make(chan struct{}),
	}
	u.lastSeen.Store(time.Time{})

	// Determine number of decrypt workers
	workers := o.decryptWorkers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	// Start pipeline goroutines
	u.wg.Add(1 + workers)
	go func() {
		defer u.wg.Done()
		u.ioLoop()
	}()
	for i := 0; i < workers; i++ {
		go func() {
			defer u.wg.Done()
			u.decryptWorker()
		}()
	}

	return u, nil
}

// SetPeerEndpoint sets or updates a peer's endpoint address.
// If the peer doesn't exist, it creates a new peer entry.
func (u *UDP) SetPeerEndpoint(pk noise.PublicKey, endpoint *net.UDPAddr) {
	if u.closed.Load() || u.closing.Load() {
		return
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	peer, exists := u.peers[pk]
	if !exists {
		peer = &peerState{
			pk:    pk,
			state: PeerStateNew,
		}
		u.peers[pk] = peer
	}

	peer.mu.Lock()
	peer.endpoint = endpoint
	peer.mu.Unlock()
}

// RemovePeer removes a peer and its associated state.
func (u *UDP) RemovePeer(pk noise.PublicKey) {
	var smux *ServiceMux
	var session *noise.Session
	var peer *peerState

	u.mu.Lock()
	var exists bool
	peer, exists = u.peers[pk]
	if !exists {
		u.mu.Unlock()
		return
	}
	delete(u.peers, pk)

	peer.mu.Lock()
	smux = peer.serviceMux
	session = peer.session
	peer.mu.Unlock()
	u.mu.Unlock()

	if smux != nil {
		_ = smux.Close()
	}

	u.mu.Lock()
	peer.mu.Lock()
	peer.removed = true
	peer.state = PeerStateFailed
	if session != nil {
		if current, ok := u.byIndex[session.LocalIndex()]; ok && current == peer {
			delete(u.byIndex, session.LocalIndex())
		}
		if peer.session == session {
			peer.session = nil
		}
	}
	if peer.serviceMux == smux {
		peer.serviceMux = nil
	}
	peer.mu.Unlock()
	u.mu.Unlock()

	u.emitPeerEvent(pk, PeerStateFailed)
}

// HostInfo returns information about the local host.
func (u *UDP) HostInfo() *HostInfo {
	u.mu.RLock()
	peerCount := len(u.peers)
	u.mu.RUnlock()

	lastSeen, _ := u.lastSeen.Load().(time.Time)

	return &HostInfo{
		PublicKey: u.localKey.Public,
		Addr:      u.socket.LocalAddr().(*net.UDPAddr),
		PeerCount: peerCount,
		RxBytes:   u.totalRx.Load(),
		TxBytes:   u.totalTx.Load(),
		LastSeen:  lastSeen,

		DroppedOutputPackets:  u.droppedOutputPackets.Load(),
		DroppedDecryptPackets: u.droppedDecryptPackets.Load(),
		DroppedInboundPackets: u.droppedInboundPackets.Load(),
		RPCRouteErrors:        u.rpcRouteErrors.Load(),
		KCPOutputErrors:       u.kcpOutputErrors.Load(),
		DroppedPeerEvents:     u.droppedPeerEvents.Load(),
	}
}

// PeerInfo returns information about a specific peer.
func (u *UDP) PeerInfo(pk noise.PublicKey) *PeerInfo {
	u.mu.RLock()
	peer, exists := u.peers[pk]
	u.mu.RUnlock()

	if !exists {
		return nil
	}

	peer.mu.RLock()
	defer peer.mu.RUnlock()

	var endpoint net.Addr
	if peer.endpoint != nil {
		endpoint = peer.endpoint
	}

	return &PeerInfo{
		PublicKey: peer.pk,
		Endpoint:  endpoint,
		State:     peer.state,
		RxBytes:   peer.rxBytes,
		TxBytes:   peer.txBytes,
		LastSeen:  peer.lastSeen,
	}
}

func (u *UDP) emitPeerEvent(pk noise.PublicKey, state PeerState) {
	if fn := u.onPeerEvent; fn != nil {
		if !fn(PeerEvent{PublicKey: pk, State: state}) {
			u.droppedPeerEvents.Add(1)
		}
	}
}

// Peers returns an iterator over all peers.
func (u *UDP) Peers() iter.Seq[*Peer] {
	return func(yield func(*Peer) bool) {
		u.mu.RLock()
		// Copy keys to avoid holding lock during iteration
		keys := make([]noise.PublicKey, 0, len(u.peers))
		for pk := range u.peers {
			keys = append(keys, pk)
		}
		u.mu.RUnlock()

		for _, pk := range keys {
			u.mu.RLock()
			ps, exists := u.peers[pk]
			u.mu.RUnlock()

			if !exists {
				continue
			}

			ps.mu.RLock()
			var endpoint net.Addr
			if ps.endpoint != nil {
				endpoint = ps.endpoint
			}
			info := &PeerInfo{
				PublicKey: ps.pk,
				Endpoint:  endpoint,
				State:     ps.state,
				RxBytes:   ps.rxBytes,
				TxBytes:   ps.txBytes,
				LastSeen:  ps.lastSeen,
			}
			ps.mu.RUnlock()

			peer := &Peer{Info: info}

			if !yield(peer) {
				return
			}
		}
	}
}

// WriteTo sends encrypted data to a peer.
// Uses EVENT protocol by default.
func (u *UDP) WriteTo(pk noise.PublicKey, data []byte) error {
	if u.closed.Load() || u.closing.Load() {
		return ErrClosed
	}

	u.mu.RLock()
	peer, exists := u.peers[pk]
	u.mu.RUnlock()

	if !exists {
		return ErrPeerNotFound
	}

	return u.sendToPeer(peer, ProtocolEVENT, data)
}

// ReadFrom reads the next decrypted message from any peer.
// It handles handshakes internally and only returns transport data.
// Returns the sender's public key, number of bytes, and any error.
func (u *UDP) ReadFrom(buf []byte) (pk noise.PublicKey, n int, err error) {
	pk, _, n, err = u.ReadPacket(buf)
	return
}

// ReadPacket reads the next decrypted message from any peer, including the protocol byte.
// Unlike ReadFrom, this also returns the protocol byte from the encrypted payload.
// Returns the sender's public key, protocol byte, number of bytes, and any error.
func (u *UDP) ReadPacket(buf []byte) (pk noise.PublicKey, proto byte, n int, err error) {
	for {
		if u.closed.Load() {
			return pk, 0, 0, ErrClosed
		}

		// Get next packet from output queue
		var pkt *packet
		select {
		case p, ok := <-u.outputChan:
			if !ok {
				return pk, 0, 0, ErrClosed
			}
			pkt = p
		case <-u.closeChan:
			return pk, 0, 0, ErrClosed
		}

		// Wait for decryption to complete
		select {
		case <-pkt.ready:
			// Decryption done
		case <-u.closeChan:
			// Release output ownership and return.
			unrefPacket(pkt)
			return pk, 0, 0, ErrClosed
		}

		// Check for errors (handshake, KCP routed internally, etc.)
		if pkt.err != nil {
			unrefPacket(pkt)
			continue // Try next packet
		}

		// Copy decrypted data to caller's buffer
		n = copy(buf, pkt.payload[:pkt.payloadN])
		pk = pkt.pk
		proto = pkt.protocol
		unrefPacket(pkt)
		return pk, proto, n, nil
	}
}

// handleHandshakeInit processes an incoming handshake initiation.
func (u *UDP) handleHandshakeInit(data []byte, from *net.UDPAddr) {
	msg, err := noise.ParseHandshakeInit(data)
	if err != nil {
		return
	}

	// Create handshake state to process the init
	hs, err := noise.NewHandshakeState(noise.Config{
		Pattern:     noise.PatternIK,
		Initiator:   false,
		LocalStatic: u.localKey,
	})
	if err != nil {
		return
	}

	// Build Noise message from wire format
	// Noise IK message 1: e(32) + encrypted_s(48) = 80 bytes
	noiseMsg := make([]byte, noise.KeySize+48)
	copy(noiseMsg[:noise.KeySize], msg.Ephemeral[:])
	copy(noiseMsg[noise.KeySize:], msg.Static)

	// Read the handshake message
	_, err = hs.ReadMessage(noiseMsg)
	if err != nil {
		return
	}

	// Get the remote's public key
	remotePK := hs.RemoteStatic()

	// Check if peer is known or if we allow unknown peers
	u.mu.Lock()
	peer, exists := u.peers[remotePK]
	if !exists {
		if !u.allowUnknown {
			u.mu.Unlock()
			return
		}
		// Create new peer
		peer = &peerState{
			pk:    remotePK,
			state: PeerStateNew,
		}
		u.peers[remotePK] = peer
	}
	u.mu.Unlock()

	// Generate local index for response
	localIdx, err := noise.GenerateIndex()
	if err != nil {
		return
	}

	// Write response message
	respPayload, err := hs.WriteMessage(nil)
	if err != nil {
		return
	}

	// Build wire message
	// Noise IK message 2: e(32) + encrypted_empty(16) = 48 bytes
	ephemeral := hs.LocalEphemeral()
	wireMsg := noise.BuildHandshakeResp(localIdx, msg.SenderIndex, ephemeral, respPayload[noise.KeySize:])

	// Send response
	_, err = u.socket.WriteToUDP(wireMsg, from)
	if err != nil {
		return
	}

	// Complete handshake and create session
	sendCS, recvCS, err := hs.Split()
	if err != nil {
		return
	}

	session, err := noise.NewSession(noise.SessionConfig{
		LocalIndex:  localIdx,
		RemoteIndex: msg.SenderIndex,
		SendKey:     sendCS.Key(),
		RecvKey:     recvCS.Key(),
		RemotePK:    remotePK,
	})
	if err != nil {
		return
	}

	peer.mu.Lock()
	if peer.serviceMux == nil {
		peer.serviceMux = u.createServiceMux(peer)
	}
	oldSession := peer.session
	peer.endpoint = from
	peer.session = session
	peer.state = PeerStateEstablished
	peer.lastSeen = time.Now()
	peer.mu.Unlock()

	// Register in index map and clean up stale entry
	u.mu.Lock()
	if oldSession != nil {
		if current, ok := u.byIndex[oldSession.LocalIndex()]; ok && current == peer {
			delete(u.byIndex, oldSession.LocalIndex())
		}
	}
	u.byIndex[localIdx] = peer
	u.mu.Unlock()

	u.emitPeerEvent(remotePK, PeerStateEstablished)
}

// handleHandshakeResp processes an incoming handshake response.
func (u *UDP) handleHandshakeResp(data []byte, from *net.UDPAddr) {
	msg, err := noise.ParseHandshakeResp(data)
	if err != nil {
		return
	}

	// Find the pending handshake by receiver index (our local index)
	u.mu.Lock()
	pending, exists := u.pending[msg.ReceiverIndex]
	if !exists {
		u.mu.Unlock()
		return
	}
	delete(u.pending, msg.ReceiverIndex)
	u.mu.Unlock()

	// Build Noise message from wire format
	// Noise IK message 2: e(32) + encrypted_empty(16) = 48 bytes
	noiseMsg := make([]byte, noise.KeySize+16)
	copy(noiseMsg[:noise.KeySize], msg.Ephemeral[:])
	copy(noiseMsg[noise.KeySize:], msg.Empty)

	// Read the handshake response
	_, err = pending.hsState.ReadMessage(noiseMsg)
	if err != nil {
		pending.peer.mu.Lock()
		pending.peer.state = PeerStateFailed
		pending.peer.mu.Unlock()
		u.emitPeerEvent(pending.peer.pk, PeerStateFailed)
		if pending.done != nil {
			pending.done <- ErrHandshakeFailed
		}
		return
	}

	// Complete handshake and create session
	sendCS, recvCS, err := pending.hsState.Split()
	if err != nil {
		pending.peer.mu.Lock()
		pending.peer.state = PeerStateFailed
		pending.peer.mu.Unlock()
		u.emitPeerEvent(pending.peer.pk, PeerStateFailed)
		if pending.done != nil {
			pending.done <- err
		}
		return
	}

	session, err := noise.NewSession(noise.SessionConfig{
		LocalIndex:  pending.localIdx,
		RemoteIndex: msg.SenderIndex,
		SendKey:     sendCS.Key(),
		RecvKey:     recvCS.Key(),
		RemotePK:    pending.peer.pk,
	})
	if err != nil {
		pending.peer.mu.Lock()
		pending.peer.state = PeerStateFailed
		pending.peer.mu.Unlock()
		u.emitPeerEvent(pending.peer.pk, PeerStateFailed)
		if pending.done != nil {
			pending.done <- err
		}
		return
	}

	peer := pending.peer
	smux := u.createServiceMux(peer)

	peer.mu.Lock()
	peer.endpoint = from // Roaming: update endpoint
	peer.session = session
	peer.serviceMux = smux
	peer.state = PeerStateEstablished
	peer.lastSeen = time.Now()
	peer.mu.Unlock()

	u.mu.Lock()
	u.byIndex[pending.localIdx] = peer
	u.mu.Unlock()

	u.emitPeerEvent(peer.pk, PeerStateEstablished)

	if pending.done != nil {
		pending.done <- nil
	}
}

// Connect initiates a handshake with a peer.
// The peer must have an endpoint set via SetPeerEndpoint.
// A receive loop (ReadFrom) must be running to process the handshake response.
func (u *UDP) Connect(pk noise.PublicKey) error {
	if u.closed.Load() || u.closing.Load() {
		return ErrClosed
	}

	u.mu.RLock()
	peer, exists := u.peers[pk]
	u.mu.RUnlock()

	if !exists {
		return ErrPeerNotFound
	}

	return u.initiateHandshake(peer)
}

// initiateHandshake starts a direct handshake with a peer.
func (u *UDP) initiateHandshake(peer *peerState) error {
	peer.mu.Lock()
	endpoint := peer.endpoint
	pk := peer.pk
	peer.state = PeerStateConnecting
	peer.mu.Unlock()

	u.emitPeerEvent(pk, PeerStateConnecting)

	if endpoint == nil {
		return ErrNoEndpoint
	}

	localIdx, err := noise.GenerateIndex()
	if err != nil {
		return err
	}

	hs, err := noise.NewHandshakeState(noise.Config{
		Pattern:      noise.PatternIK,
		Initiator:    true,
		LocalStatic:  u.localKey,
		RemoteStatic: &pk,
	})
	if err != nil {
		return err
	}

	msg1, err := hs.WriteMessage(nil)
	if err != nil {
		return err
	}

	ephemeral := hs.LocalEphemeral()
	wireMsg := noise.BuildHandshakeInit(localIdx, ephemeral, msg1[noise.KeySize:])

	done := make(chan error, 1)
	u.mu.Lock()
	u.pending[localIdx] = &pendingHandshake{
		peer:      peer,
		hsState:   hs,
		localIdx:  localIdx,
		done:      done,
		createdAt: time.Now(),
	}
	u.mu.Unlock()

	// Direct handshake to endpoint
	_, err = u.socket.WriteToUDP(wireMsg, endpoint)

	if err != nil {
		u.mu.Lock()
		delete(u.pending, localIdx)
		u.mu.Unlock()
		return err
	}

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		u.mu.Lock()
		delete(u.pending, localIdx)
		u.mu.Unlock()
		peer.mu.Lock()
		peer.state = PeerStateFailed
		peer.mu.Unlock()
		u.emitPeerEvent(pk, PeerStateFailed)
		return errors.New("net: handshake timeout")
	}
}

// Close closes the UDP network.
func (u *UDP) Close() error {
	if u.closed.Load() || u.closing.Swap(true) {
		return nil // Already closed
	}

	// Close all per-peer ServiceMux instances first so AcceptStream callers
	// don't block forever on open accept queues.
	//
	// Keep the peer.serviceMux pointer alive until smux.Close() completes,
	// so in-flight CLOSE_ACK frames from the UDP decrypt path can still route back.
	type peerMuxRef struct {
		peer *peerState
		smux *ServiceMux
	}
	refs := make([]peerMuxRef, 0)
	u.mu.RLock()
	for _, peer := range u.peers {
		peer.mu.RLock()
		smux := peer.serviceMux
		peer.mu.RUnlock()
		if smux != nil {
			refs = append(refs, peerMuxRef{peer: peer, smux: smux})
		}
	}
	u.mu.RUnlock()

	for _, ref := range refs {
		_ = ref.smux.Close()
	}

	for _, ref := range refs {
		ref.peer.mu.Lock()
		if ref.peer.serviceMux == ref.smux {
			ref.peer.serviceMux = nil
		}
		ref.peer.mu.Unlock()
	}

	u.closed.Store(true)

	// Signal goroutines to stop
	close(u.closeChan)

	// Close socket (will unblock ioLoop's ReadFromUDP)
	err := u.socket.Close()

	// Wait for all goroutines to finish BEFORE closing channels
	// This prevents race condition where ioLoop is writing to channels
	// while we're closing them
	u.wg.Wait()

	// Now safe to close channels (all writers have exited)
	close(u.decryptChan)
	close(u.outputChan)

	return err
}

// ioLoop reads packets from the socket and dispatches them.
// Each packet goes to both decryptChan (for workers) and outputChan (for ReadFrom).
// On Linux, uses recvmmsg batch reading for reduced syscall overhead.
// This goroutine only does I/O, no decryption, to maximize throughput.
func (u *UDP) ioLoop() {
	bc := newBatchConn(u.socket, DefaultBatchSize)
	if bc != nil {
		u.ioLoopBatch(bc)
	} else {
		u.ioLoopSingle()
	}
}

// ioLoopBatch reads packets using recvmmsg (Linux).
func (u *UDP) ioLoopBatch(bc *batchConn) {
	pkts := make([]*packet, DefaultBatchSize)
	bufs := make([][]byte, DefaultBatchSize)

	for {
		if u.closed.Load() {
			return
		}

		// Acquire batch of packets from pool
		count := 0
		for count < DefaultBatchSize {
			pkts[count] = acquirePacket()
			bufs[count] = pkts[count].data
			count++
		}

		// Batch read (blocks until ≥1 packet available)
		n, err := bc.ReadBatch(bufs[:count])
		if err != nil {
			for i := 0; i < count; i++ {
				releasePacket(pkts[i])
			}
			if u.closed.Load() {
				return
			}
			continue
		}

		// Release unused packets
		for i := n; i < count; i++ {
			releasePacket(pkts[i])
		}

		// Dispatch received packets
		for i := 0; i < n; i++ {
			pkt := pkts[i]
			pkt.n = bc.ReceivedN(i)
			pkt.from = bc.ReceivedFrom(i)

			if pkt.n < 1 || pkt.from == nil {
				releasePacket(pkt)
				continue
			}

			u.totalRx.Add(uint64(pkt.n))
			u.lastSeen.Store(time.Now())

			u.dispatchToChannels(pkt)
		}
	}
}

// ioLoopSingle reads packets one at a time (non-Linux fallback).
func (u *UDP) ioLoopSingle() {
	for {
		if u.closed.Load() {
			return
		}

		pkt := acquirePacket()

		n, from, err := u.socket.ReadFromUDP(pkt.data)
		if err != nil {
			releasePacket(pkt)
			if u.closed.Load() {
				return
			}
			continue
		}

		if u.closed.Load() {
			releasePacket(pkt)
			return
		}

		if n < 1 {
			releasePacket(pkt)
			continue
		}

		pkt.n = n
		pkt.from = from

		u.totalRx.Add(uint64(n))
		u.lastSeen.Store(time.Now())

		u.dispatchToChannels(pkt)
	}
}

// dispatchToChannels sends a packet to both outputChan and decryptChan.
func (u *UDP) dispatchToChannels(pkt *packet) {
	select {
	case <-u.closeChan:
		releasePacket(pkt)
		return
	default:
	}

	// Ownership model:
	// - Always reserve 1 ref for decrypt path.
	// - Optionally reserve +1 for output path when queued to outputChan.
	pkt.refs.Store(1)

	outputQueued := false
	pkt.refs.Add(1) // reserve output ref before enqueue to avoid races
	select {
	case u.outputChan <- pkt:
		outputQueued = true
	case <-u.closeChan:
		unrefPacket(pkt) // output ref
		unrefPacket(pkt) // decrypt ref
		return
	default:
		u.droppedOutputPackets.Add(1)
		unrefPacket(pkt) // drop output ref; not queued to outputChan
	}

	select {
	case u.decryptChan <- pkt:
		// Sent to decrypt worker
	case <-u.closeChan:
		if outputQueued {
			pkt.err = ErrNoData
			close(pkt.ready)
		}
		unrefPacket(pkt) // drop decrypt ref
		return
	default:
		// Decrypt queue full. If packet is in outputChan,
		// mark it as error and signal ready so ReadFrom skips it.
		u.droppedDecryptPackets.Add(1)
		if outputQueued {
			pkt.err = ErrNoData
			close(pkt.ready)
		}
		unrefPacket(pkt) // drop decrypt ref
	}
}

// decryptWorker processes packets from decryptChan.
// Multiple workers run in parallel for higher throughput.
// After processing, it signals ready so ReadFrom can consume.
// The worker drops the decrypt-path reference; packet is released when
// all references (decrypt/output) are dropped.
func (u *UDP) decryptWorker() {
	for {
		select {
		case pkt, ok := <-u.decryptChan:
			if !ok {
				return // channel closed
			}
			u.processPacket(pkt)
			close(pkt.ready)
			unrefPacket(pkt) // drop decrypt ref
		case <-u.closeChan:
			return
		}
	}
}

// processPacket handles a single packet - parses, decrypts, and fills result fields.
// Called by decryptWorker. Sets pkt.err if processing fails.
func (u *UDP) processPacket(pkt *packet) {
	data := pkt.data[:pkt.n]
	from := pkt.from

	if len(data) < 1 {
		pkt.err = ErrNoData
		return
	}

	// Parse message type
	msgType := data[0]

	switch msgType {
	case noise.MessageTypeHandshakeInit:
		u.handleHandshakeInit(data, from)
		pkt.err = ErrNoData // Not a data packet

	case noise.MessageTypeHandshakeResp:
		u.handleHandshakeResp(data, from)
		pkt.err = ErrNoData // Not a data packet

	case noise.MessageTypeTransport:
		u.decryptTransport(pkt, data, from)

	default:
		pkt.err = ErrNoData
	}
}

// decryptTransport decrypts a transport message and fills pkt fields.
// Also routes RPC/KCP packets to the service mux and enqueues direct UDP packets there.
func (u *UDP) decryptTransport(pkt *packet, data []byte, from *net.UDPAddr) {
	msg, err := noise.ParseTransportMessage(data)
	if err != nil {
		pkt.err = err
		return
	}

	// Find peer by receiver index
	u.mu.RLock()
	peer, exists := u.byIndex[msg.ReceiverIndex]
	u.mu.RUnlock()

	if !exists {
		pkt.err = ErrPeerNotFound
		return
	}

	peer.mu.RLock()
	session := peer.session
	peer.mu.RUnlock()

	if session == nil {
		pkt.err = ErrNoSession
		return
	}

	// Decrypt
	plaintext, err := session.Decrypt(msg.Ciphertext, msg.Counter)
	if err != nil {
		pkt.err = err
		return
	}
	if afterDecryptTransportDecryptHook != nil {
		afterDecryptTransportDecryptHook()
	}

	// Update peer state (roaming + stats) and get mux
	peer.mu.Lock()
	if peer.removed {
		peer.mu.Unlock()
		pkt.err = ErrPeerNotFound
		return
	}
	if peer.endpoint == nil || peer.endpoint.String() != from.String() {
		peer.endpoint = from // Roaming
	}
	peer.rxBytes += uint64(len(data))
	peer.lastSeen = time.Now()
	smux := peer.serviceMux
	peer.mu.Unlock()

	pkt.pk = peer.pk

	if len(plaintext) == 0 {
		pkt.err = ErrNoData
		return
	}

	protocol, payload, err := noise.DecodePayload(plaintext)
	if err != nil {
		pkt.err = err
		return
	}

	pkt.protocol = protocol
	pkt.payload = make([]byte, len(payload))
	copy(pkt.payload, payload)
	pkt.payloadN = len(payload)
	// Stream protocols are routed through KCP streams, not the raw passthrough path.
	// Wire format: service_varint + kcp_data
	if IsStreamProtocol(protocol) {
		if smux == nil {
			u.rpcRouteErrors.Add(1)
			pkt.err = ErrNoSession
			return
		}
		service, n, err := noise.DecodeVarint(payload)
		if err != nil {
			u.rpcRouteErrors.Add(1)
			pkt.err = err
			return
		}
		if err := smux.Input(service, protocol, payload[n:]); err != nil {
			u.rpcRouteErrors.Add(1)
			pkt.err = err
			return
		}
		pkt.err = ErrNoData // Stream protocol data handed off to smux/KCP
		return
	}

	if !IsFoundationProtocol(protocol) {
		pkt.err = ErrUnsupportedProtocol
		return
	}

	// Non-KCP packets are delivered through the peer service mux.
	if smux == nil {
		u.droppedInboundPackets.Add(1)
		pkt.err = ErrNoData
		return
	}
	if err := smux.Input(0, protocol, pkt.payload); err != nil {
		u.droppedInboundPackets.Add(1)
	}
}
