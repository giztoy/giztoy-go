package gizclaw

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/audio/codec/opus"
	"github.com/GizClaw/gizclaw-go/pkg/audio/pcm"
	"github.com/GizClaw/gizclaw-go/pkg/audio/stampedopus"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/rpc"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"golang.org/x/sync/errgroup"
)

var (
	ErrNilGearPeer        = errors.New("gizclaw: nil gear peer")
	ErrNilGearPeerConn    = errors.New("gizclaw: nil gear peer conn")
	ErrNilGearPeerService = errors.New("gizclaw: nil gear peer service")
	ErrNilGearPeerMixer   = errors.New("gizclaw: nil gear peer mixer")
)

const gearPeerMixerFormat = pcm.L16Mono16K

const gearPeerOpusFrameDuration = 20 * time.Millisecond

// GearPeer is the in-memory runtime peer for one active gear.
// It wraps the existing PeerService bundle and serves one live conn at a time.
type GearPeer struct {
	Conn    *giznet.Conn
	Service *PeerService

	closeOnce              sync.Once
	mixer                  *pcm.Mixer
	lastOpusFrameTimestamp atomic.Uint64
	closed                 atomic.Bool
}

// CreateAudioTrack creates a writable audio track on the peer mixer.
// The mixer itself is intentionally kept private to GearPeer.
func (h *GearPeer) CreateAudioTrack(opts ...pcm.TrackOption) (pcm.Track, *pcm.TrackCtrl, error) {
	mx, err := h.audioMixer()
	if err != nil {
		return nil, nil, err
	}
	return mx.CreateTrack(opts...)
}

// serve proxies to the existing PeerService implementation for one live conn.
func (h *GearPeer) serve() error {
	if h == nil {
		return ErrNilGearPeer
	}
	if h.Conn == nil {
		return ErrNilGearPeerConn
	}
	if h.Service == nil {
		return ErrNilGearPeerService
	}
	h.init()

	var g errgroup.Group
	g.Go(h.serveService)
	g.Go(h.servePackets)
	g.Go(h.serveRPC)
	err := g.Wait()
	if err != nil {
		_ = h.close()
	}
	return err
}

func (h *GearPeer) serveService() error {
	defer func() {
		_ = h.close()
	}()
	return h.Service.ServeConn(h.Conn)
}

func (h *GearPeer) servePackets() error {
	if _, err := h.audioMixer(); err != nil {
		return err
	}
	h.streamMixedAudioLoop()
	return nil
}

func (h *GearPeer) serveRPC() error {
	listener := h.Conn.ListenService(ServiceRPC)
	defer func() {
		_ = listener.Close()
	}()
	for {
		stream, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		h.Service.touchPeer(h.Conn)

		go func(stream net.Conn) {
			if err := h.serveRPCStream(stream); err != nil {
				_ = stream.Close()
			}
		}(stream)
	}
}

func (h *GearPeer) serveRPCStream(stream net.Conn) error {
	req, err := rpc.ReadRequest(stream)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	}
	h.Service.touchPeer(h.Conn)

	resp, err := h.dispatchRPC(context.Background(), req)
	if err != nil {
		return err
	}
	if resp == nil {
		resp = &rpc.RPCResponse{V: 1, Id: req.Id}
	}
	if resp.Id == "" {
		resp.Id = req.Id
	}
	if resp.V == 0 {
		resp.V = 1
	}
	if err := rpc.WriteResponse(stream, resp); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	}
	return nil
}

func (h *GearPeer) dispatchRPC(ctx context.Context, req *rpc.RPCRequest) (*rpc.RPCResponse, error) {
	switch req.Method {
	case rpc.MethodPing:
		if req.Params == nil {
			return rpc.ErrorResponse(req.Id, -32602, "missing params"), nil
		}
		response, err := h.handlePing(ctx, *req.Params)
		if err != nil {
			return nil, err
		}
		return rpc.ResultResponse(req.Id, response), nil
	default:
		return rpc.ErrorResponse(req.Id, -1, fmt.Sprintf("unknown method: %s", req.Method)), nil
	}
}

// Ping opens a fresh RPC stream, sends one ping, and closes it.
//
// Our current RPC transport uses one KCP stream per round trip so multiple RPC
// requests can run concurrently on separate streams. This is closer to
// HTTP/1.0-style request lifecycles; HTTP/1.1-style stream reuse is not
// supported yet.
func (h *GearPeer) Ping(ctx context.Context, id string) (*rpc.PingResponse, error) {
	rpcClient, err := h.rpcClient()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rpcClient.Close() }()
	return rpcClient.Ping(ctx, id)
}

func (h *GearPeer) rpcClient() (*rpc.Client, error) {
	conn := h.Conn
	stream, err := conn.Dial(ServiceRPC)
	if err != nil {
		return nil, fmt.Errorf("gizclaw: dial rpc stream: %w", err)
	}
	return rpc.NewClient(stream), nil
}

func (h *GearPeer) handlePing(_ context.Context, _ rpc.PingRequest) (*rpc.PingResponse, error) {
	return &rpc.PingResponse{ServerTime: time.Now().UnixMilli()}, nil
}

func (h *GearPeer) init() {
	h.initMixer()
}

func (h *GearPeer) initMixer() {
	if h == nil {
		return
	}
	if h.mixer == nil {
		h.mixer = pcm.NewMixer(gearPeerMixerFormat)
	}
}

func (h *GearPeer) audioMixer() (*pcm.Mixer, error) {
	if h == nil {
		return nil, ErrNilGearPeer
	}
	if h.mixer == nil {
		return nil, ErrNilGearPeerMixer
	}
	return h.mixer, nil
}

func (h *GearPeer) close() error {
	if h == nil {
		return nil
	}
	var closeErr error
	h.closeOnce.Do(func() {
		h.closed.Store(true)
		mx := h.mixer
		if mx != nil {
			closeErr = mx.Close()
		}
	})
	return closeErr
}

func (h *GearPeer) streamMixedAudioLoop() {
	hasWrittenBefore := false
	for !h.isClosed() {
		wrote, err := h.streamMixedAudio(hasWrittenBefore)
		hasWrittenBefore = hasWrittenBefore || wrote
		if err != nil {
			slog.Error("gizclaw: mixed audio stream failed; retrying", "error", err)
		}
	}
}

func (h *GearPeer) streamMixedAudio(hasWrittenBefore bool) (wrote bool, err error) {
	mx := h.mixer
	enc, err := opus.NewEncoder(gearPeerMixerFormat.SampleRate(), gearPeerMixerFormat.Channels(), opus.ApplicationAudio)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = enc.Close()
	}()

	frameSize := int(gearPeerMixerFormat.SamplesInDuration(gearPeerOpusFrameDuration))
	for {
		chunk, err := gearPeerMixerFormat.ReadChunk(mx, gearPeerOpusFrameDuration)
		if err != nil {
			if h.isClosed() && errors.Is(err, io.ErrClosedPipe) {
				return wrote, nil
			}
			return wrote, err
		}

		packet, err := enc.Encode(gearPeerPCMChunkToInt16(chunk), frameSize)
		if err != nil {
			return wrote, err
		}
		if !hasWrittenBefore {
			h.lastOpusFrameTimestamp.Store(uint64(time.Now().UnixMilli()))
			hasWrittenBefore = true
			wrote = true
		}
		payload := stampedopus.Pack(h.lastOpusFrameTimestamp.Load(), packet)
		if _, err := h.Conn.Write(ProtocolStampedOpus, payload); err != nil {
			return wrote, err
		}
		h.lastOpusFrameTimestamp.Add(uint64(gearPeerOpusFrameDuration / time.Millisecond))
	}
}

func (h *GearPeer) isClosed() bool {
	if h == nil {
		return true
	}
	return h.closed.Load()
}

func gearPeerPCMChunkToInt16(chunk pcm.Chunk) []int16 {
	dataChunk, ok := chunk.(*pcm.DataChunk)
	if !ok || len(dataChunk.Data) == 0 {
		return nil
	}
	data := dataChunk.Data
	out := make([]int16, len(data)/2)
	for i := range out {
		lo := uint16(data[i*2])
		hi := uint16(data[i*2+1]) << 8
		out[i] = int16(lo | hi)
	}
	return out
}
