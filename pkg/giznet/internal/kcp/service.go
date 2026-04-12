package kcp

import (
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"sync/atomic"
)

var (
	ErrServiceMuxClosed    = errors.New("kcp: service mux closed")
	ErrServiceNotFound     = errors.New("kcp: service not found")
	ErrServiceRejected     = errors.New("kcp: service rejected")
	ErrAcceptQueueClosed   = errors.New("kcp: accept queue closed")
	ErrInboundQueueFull    = errors.New("kcp: inbound queue full")
	ErrInvalidServiceFrame = errors.New("kcp: invalid service frame")
)

type ServiceMuxConfig struct {
	IsClient            bool
	Output              func(service uint64, data []byte) error
	OnOutputError       func(service uint64, err error)
	OnNewService        func(service uint64) bool
	DefaultKcpMuxConfig KcpMuxConfig
}

type serviceState struct {
	serviceID uint64
	mux       *KcpMux
}

type ServiceMux struct {
	config ServiceMuxConfig

	mu       sync.RWMutex
	services map[uint64]*serviceState
	closeCh  chan struct{}

	closed    atomic.Bool
	closeOnce sync.Once

	outputErrors atomic.Uint64
}

func NewServiceMux(cfg ServiceMuxConfig) *ServiceMux {
	return &ServiceMux{
		config:   cfg,
		services: make(map[uint64]*serviceState),
		closeCh:  make(chan struct{}),
	}
}

func (m *ServiceMux) Input(service uint64, data []byte) error {
	if m.closed.Load() {
		return ErrServiceMuxClosed
	}

	state, ok := m.getService(service)
	if ok {
		return state.mux.Input(data)
	}

	streamID, frameType, _, err := decodeMuxFrame(data)
	if err != nil {
		return err
	}

	if frameType == kcpMuxFrameOpen && m.config.OnNewService != nil && m.config.OnNewService(service) {
		state, err = m.getOrCreateService(service)
		if err != nil {
			return err
		}
		return state.mux.Input(data)
	}

	switch frameType {
	case kcpMuxFrameOpen:
		m.sendControlFrame(service, streamID, kcpMuxFrameClose, []byte{streamCloseReasonAbort})
	case kcpMuxFrameClose:
		m.sendControlFrame(service, streamID, kcpMuxFrameCloseAck, nil)
	case kcpMuxFrameCloseAck:
		return nil
	default:
		m.sendControlFrame(service, streamID, kcpMuxFrameClose, []byte{streamCloseReasonInvalid})
	}
	return nil
}

func (m *ServiceMux) OpenStream(service uint64) (net.Conn, error) {
	if m.closed.Load() {
		return nil, ErrServiceMuxClosed
	}
	state, err := m.getOrCreateService(service)
	if err != nil {
		return nil, err
	}
	return state.mux.Open()
}

func (m *ServiceMux) AcceptStream(service uint64) (net.Conn, error) {
	if m.closed.Load() {
		return nil, ErrServiceMuxClosed
	}
	state, err := m.getOrCreateService(service)
	if err != nil {
		return nil, err
	}
	return state.mux.Accept()
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
			_ = state.mux.Close()
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
		total += state.mux.NumStreams()
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
		return state, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		return nil, ErrServiceMuxClosed
	}
	if state, ok := m.services[service]; ok {
		return state, nil
	}

	state = &serviceState{
		serviceID: service,
		mux: NewKcpMux(
			service,
			m.config.IsClient,
			m.config.DefaultKcpMuxConfig,
			m.config.Output,
			m.reportOutputError,
		),
	}
	m.services[service] = state
	return state, nil
}

func (m *ServiceMux) sendControlFrame(service uint64, streamID uint64, frameType byte, payload []byte) {
	frame := make([]byte, 0, len(payload)+16)
	frame = binary.AppendUvarint(frame, streamID)
	frame = append(frame, frameType)
	frame = append(frame, payload...)
	if m.config.Output == nil {
		return
	}
	if err := m.config.Output(service, frame); err != nil {
		m.reportOutputError(service, err)
	}
}

func (m *ServiceMux) reportOutputError(service uint64, err error) {
	if err == nil {
		return
	}
	m.outputErrors.Add(1)
	if m.config.OnOutputError != nil {
		m.config.OnOutputError(service, err)
	}
}
