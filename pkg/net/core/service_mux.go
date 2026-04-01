package core

import (
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"

	"github.com/giztoy/giztoy-go/pkg/net/kcp"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

var (
	ErrServiceMuxClosed    = kcp.ErrServiceMuxClosed
	ErrAcceptQueueClosed   = kcp.ErrAcceptQueueClosed
	ErrServiceRejected     = kcp.ErrServiceRejected
	ErrInboundQueueFull    = kcp.ErrInboundQueueFull
	ErrInvalidServiceFrame = kcp.ErrInvalidServiceFrame
)

var afterServiceMuxDirectReadHook func()

const (
	serviceMuxFrameOpen byte = iota
	serviceMuxFrameData
	serviceMuxFrameClose
	serviceMuxFrameCloseAck
)

const (
	serviceStreamCloseReasonClose byte = iota
	serviceStreamCloseReasonAbort
	serviceStreamCloseReasonInvalid
)

type ServiceMuxConfig struct {
	IsClient            bool
	Output              func(peer noise.PublicKey, service uint64, protocol byte, data []byte) error
	OnOutputError       func(peer noise.PublicKey, service uint64, err error)
	OnNewService        func(peer noise.PublicKey, service uint64) bool
	DefaultKcpMuxConfig kcp.KcpMuxConfig
}

type serviceState struct {
	serviceID        uint64
	mux              *kcp.KcpMux
	eventInbound     chan protoPacket
	opusInbound      chan protoPacket
	closed           bool
	acceptingStopped bool
}

type ServiceMux struct {
	config ServiceMuxConfig
	peer   noise.PublicKey

	mu       sync.RWMutex
	services map[uint64]*serviceState
	closeCh  chan struct{}

	closed    atomic.Bool
	closeOnce sync.Once

	outputErrors atomic.Uint64
}

func NewServiceMux(peer noise.PublicKey, cfg ServiceMuxConfig) *ServiceMux {
	if cfg.OnNewService == nil {
		cfg.OnNewService = func(_ noise.PublicKey, service uint64) bool {
			return service == 0
		}
	}
	return &ServiceMux{
		config:   cfg,
		peer:     peer,
		services: make(map[uint64]*serviceState),
		closeCh:  make(chan struct{}),
	}
}

func (m *ServiceMux) Input(service uint64, protocol byte, data []byte) error {
	if m.closed.Load() {
		return ErrServiceMuxClosed
	}

	state, ok := m.getService(service)
	if ok {
		if state.closed {
			return m.rejectUnknownService(service, protocol, data)
		}
		if state.acceptingStopped && shouldRejectStoppedService(protocol, data) {
			return m.rejectUnknownService(service, protocol, data)
		}
		return m.inputToService(state, protocol, data)
	}

	switch protocol {
	case ProtocolHTTP, ProtocolRPC, ProtocolEVENT, ProtocolOPUS:
	default:
		return ErrUnsupportedProtocol
	}

	if !m.config.OnNewService(m.peer, service) {
		return m.rejectUnknownService(service, protocol, data)
	}

	state, err := m.getOrCreateService(service)
	if err != nil {
		return err
	}
	return m.inputToService(state, protocol, data)
}

func (m *ServiceMux) Read(buf []byte) (protocol byte, n int, err error) {
	state, err := m.getOrCreateService(0)
	if err != nil {
		return 0, 0, err
	}
	if m.closed.Load() {
		return 0, 0, ErrServiceMuxClosed
	}

	select {
	case pkt, ok := <-state.eventInbound:
		n, err = m.copyDirectPacket(pkt, ok, buf)
		if err != nil {
			return 0, 0, err
		}
		return pkt.protocol, n, nil
	case pkt, ok := <-state.opusInbound:
		n, err = m.copyDirectPacket(pkt, ok, buf)
		if err != nil {
			return 0, 0, err
		}
		return pkt.protocol, n, nil
	case <-m.closeCh:
		return 0, 0, ErrServiceMuxClosed
	}
}

func (m *ServiceMux) ReadProtocol(protocol byte, buf []byte) (n int, err error) {
	return m.ReadServiceProtocol(0, protocol, buf)
}

func (m *ServiceMux) ReadServiceProtocol(service uint64, protocol byte, buf []byte) (n int, err error) {
	state, err := m.getOrCreateService(service)
	if err != nil {
		return 0, err
	}
	if m.closed.Load() {
		return 0, ErrServiceMuxClosed
	}

	ch, err := state.directInbound(protocol)
	if err != nil {
		return 0, err
	}

	select {
	case pkt, ok := <-ch:
		return m.copyDirectPacket(pkt, ok, buf)
	case <-m.closeCh:
		return 0, ErrServiceMuxClosed
	}
}

func (m *ServiceMux) Write(protocol byte, data []byte) (n int, err error) {
	if m.closed.Load() {
		return 0, ErrServiceMuxClosed
	}

	switch protocol {
	case ProtocolRPC:
		return 0, ErrRPCMustUseStream
	case ProtocolHTTP:
		return 0, ErrHTTPMustUseStream
	case ProtocolEVENT, ProtocolOPUS:
	default:
		return 0, ErrUnsupportedProtocol
	}

	if m.config.Output == nil {
		return 0, ErrNoSession
	}
	if err := m.config.Output(m.peer, 0, protocol, data); err != nil {
		m.reportOutputError(0, err)
		return 0, err
	}
	return len(data), nil
}

func (m *ServiceMux) OpenStream(service uint64) (net.Conn, error) {
	if m.closed.Load() {
		return nil, ErrServiceMuxClosed
	}
	state, err := m.getOrCreateService(service)
	if err != nil {
		return nil, err
	}
	mux, err := m.ensureKcpMux(state)
	if err != nil {
		return nil, err
	}
	return mux.Open()
}

func (m *ServiceMux) AcceptStream(service uint64) (net.Conn, error) {
	if m.closed.Load() {
		return nil, ErrServiceMuxClosed
	}
	state, err := m.getOrCreateService(service)
	if err != nil {
		return nil, err
	}
	mux, err := m.ensureKcpMux(state)
	if err != nil {
		return nil, err
	}
	return mux.Accept()
}

func (m *ServiceMux) CloseService(service uint64) error {
	if m.closed.Load() {
		return ErrServiceMuxClosed
	}

	m.mu.Lock()
	state, ok := m.services[service]
	if !ok {
		state = &serviceState{
			serviceID:    service,
			eventInbound: make(chan protoPacket, InboundChanSize),
			opusInbound:  make(chan protoPacket, InboundChanSize),
		}
		m.services[service] = state
	}
	state.closed = true
	state.acceptingStopped = true
	m.mu.Unlock()
	if state.mux != nil {
		return state.mux.Close()
	}
	return nil
}

func (m *ServiceMux) StopAcceptingService(service uint64) error {
	if m.closed.Load() {
		return ErrServiceMuxClosed
	}

	m.mu.Lock()
	state, ok := m.services[service]
	if !ok {
		state = &serviceState{
			serviceID:    service,
			eventInbound: make(chan protoPacket, InboundChanSize),
			opusInbound:  make(chan protoPacket, InboundChanSize),
		}
		m.services[service] = state
	}
	state.acceptingStopped = true
	m.mu.Unlock()
	if state.mux == nil {
		return nil
	}
	return state.mux.StopAccepting()
}

func (m *ServiceMux) Close() error {
	m.closeOnce.Do(func() {
		m.closed.Store(true)
		close(m.closeCh)

		m.mu.Lock()
		states := make([]*serviceState, 0, len(m.services))
		for _, state := range m.services {
			states = append(states, state)
		}
		m.services = make(map[uint64]*serviceState)
		m.mu.Unlock()

		for _, state := range states {
			if state.mux != nil {
				_ = state.mux.Close()
			}
		}
	})
	return nil
}

func (m *ServiceMux) NumServices() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.services)
}

func (m *ServiceMux) NumStreams() int {
	m.mu.RLock()
	states := make([]*serviceState, 0, len(m.services))
	for _, state := range m.services {
		states = append(states, state)
	}
	m.mu.RUnlock()

	total := 0
	for _, state := range states {
		if state.mux != nil {
			total += state.mux.NumStreams()
		}
	}
	return total
}

func (m *ServiceMux) OutputErrorCount() uint64 {
	return m.outputErrors.Load()
}

func (m *ServiceMux) getService(service uint64) (*serviceState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.services[service]
	return state, ok
}

func (m *ServiceMux) getOrCreateService(service uint64) (*serviceState, error) {
	if m.closed.Load() {
		return nil, ErrServiceMuxClosed
	}

	m.mu.RLock()
	state, ok := m.services[service]
	m.mu.RUnlock()
	if ok {
		if state.closed {
			return nil, ErrServiceMuxClosed
		}
		return state, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		return nil, ErrServiceMuxClosed
	}
	if state, ok := m.services[service]; ok {
		if state.closed {
			return nil, ErrServiceMuxClosed
		}
		return state, nil
	}

	state = &serviceState{
		serviceID:    service,
		eventInbound: make(chan protoPacket, InboundChanSize),
		opusInbound:  make(chan protoPacket, InboundChanSize),
	}
	m.services[service] = state
	return state, nil
}

func (m *ServiceMux) ensureKcpMux(state *serviceState) (*kcp.KcpMux, error) {
	if state.mux != nil {
		return state.mux, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		return nil, ErrServiceMuxClosed
	}
	if state.mux != nil {
		return state.mux, nil
	}
	if state.closed {
		return nil, ErrServiceMuxClosed
	}

	state.mux = kcp.NewKcpMux(
		state.serviceID,
		m.config.IsClient,
		m.config.DefaultKcpMuxConfig,
		func(service uint64, data []byte) error {
			if m.config.Output == nil {
				return ErrNoSession
			}
			return m.config.Output(m.peer, service, ProtocolRPC, data)
		},
		m.reportOutputError,
	)
	if state.acceptingStopped {
		if err := state.mux.StopAccepting(); err != nil {
			return nil, err
		}
	}
	return state.mux, nil
}

func (m *ServiceMux) reportOutputError(service uint64, err error) {
	if err == nil {
		return
	}
	m.outputErrors.Add(1)
	if m.config.OnOutputError != nil {
		m.config.OnOutputError(m.peer, service, err)
	}
}

func (m *ServiceMux) rejectUnknownService(service uint64, protocol byte, data []byte) error {
	if !IsStreamProtocol(protocol) {
		return ErrServiceRejected
	}

	streamID, frameType, err := decodeServiceMuxFrame(data)
	if err != nil {
		return err
	}

	switch frameType {
	case serviceMuxFrameOpen:
		m.sendServiceControlFrame(service, streamID, serviceMuxFrameClose, []byte{serviceStreamCloseReasonAbort})
	case serviceMuxFrameClose:
		m.sendServiceControlFrame(service, streamID, serviceMuxFrameCloseAck, nil)
	case serviceMuxFrameCloseAck:
		return nil
	default:
		m.sendServiceControlFrame(service, streamID, serviceMuxFrameClose, []byte{serviceStreamCloseReasonInvalid})
	}
	return nil
}

func (m *ServiceMux) sendServiceControlFrame(service uint64, streamID uint64, frameType byte, payload []byte) {
	frame := binary.AppendUvarint(nil, streamID)
	frame = append(frame, frameType)
	frame = append(frame, payload...)
	if m.config.Output == nil {
		return
	}
	if err := m.config.Output(m.peer, service, ProtocolRPC, frame); err != nil {
		m.reportOutputError(service, err)
	}
}

func decodeServiceMuxFrame(data []byte) (uint64, byte, error) {
	streamID, n := binary.Uvarint(data)
	if n <= 0 {
		return 0, 0, ErrInvalidServiceFrame
	}
	if len(data[n:]) == 0 {
		return 0, 0, ErrInvalidServiceFrame
	}
	return streamID, data[n], nil
}

func shouldRejectStoppedService(protocol byte, data []byte) bool {
	if !IsStreamProtocol(protocol) {
		return false
	}
	_, frameType, err := decodeServiceMuxFrame(data)
	return err == nil && frameType == serviceMuxFrameOpen
}

func (m *ServiceMux) copyDirectPacket(pkt protoPacket, ok bool, buf []byte) (int, error) {
	if !ok {
		return 0, ErrServiceMuxClosed
	}
	if afterServiceMuxDirectReadHook != nil {
		afterServiceMuxDirectReadHook()
	}
	if m.closed.Load() {
		return 0, ErrServiceMuxClosed
	}
	return copy(buf, pkt.payload), nil
}

func (m *ServiceMux) inputToService(state *serviceState, protocol byte, data []byte) error {
	switch protocol {
	case ProtocolHTTP, ProtocolRPC:
		mux, err := m.ensureKcpMux(state)
		if err != nil {
			return err
		}
		return mux.Input(data)
	case ProtocolEVENT, ProtocolOPUS:
		ch, err := state.directInbound(protocol)
		if err != nil {
			return err
		}
		select {
		case ch <- protoPacket{protocol: protocol, payload: data}:
			return nil
		default:
			return ErrInboundQueueFull
		}
	default:
		return ErrUnsupportedProtocol
	}
}

func (s *serviceState) directInbound(protocol byte) (chan protoPacket, error) {
	switch protocol {
	case ProtocolEVENT:
		return s.eventInbound, nil
	case ProtocolOPUS:
		return s.opusInbound, nil
	case ProtocolRPC:
		return nil, ErrRPCMustUseStream
	case ProtocolHTTP:
		return nil, ErrHTTPMustUseStream
	default:
		return nil, ErrUnsupportedProtocol
	}
}
