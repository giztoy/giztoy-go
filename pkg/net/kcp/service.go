package kcp

import (
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrServiceMuxClosed    = errors.New("kcp: service mux closed")
	ErrServiceNotFound     = errors.New("kcp: service not found")
	ErrServiceRejected     = errors.New("kcp: service rejected")
	ErrAcceptQueueClosed   = errors.New("kcp: accept queue closed")
	ErrInvalidServiceFrame = errors.New("kcp: invalid service frame")
)

const (
	serviceFrameData byte = iota
	serviceFrameClose
	serviceFrameCloseAck
)

const (
	serviceCloseReasonLocal byte = iota
	serviceCloseReasonPeer
)

const (
	activeCloseAckTimeout    = 400 * time.Millisecond
	activeCloseRetryInterval = 100 * time.Millisecond
)

type ServiceMuxConfig struct {
	IsClient      bool
	Output        func(service uint64, data []byte) error
	OnOutputError func(service uint64, err error)
	OnNewService  func(service uint64) bool
}

type serviceEntry struct {
	conn      *KCPConn
	announced atomic.Bool
	readyOnce sync.Once
	readyCh   chan struct{}
}

type closeToken struct {
	service uint64
	id      uint64
}

type ServiceMux struct {
	config ServiceMuxConfig

	services   map[uint64]*serviceEntry
	servicesMu sync.RWMutex

	acceptCh chan acceptResult
	closeCh  chan struct{}

	closing   atomic.Bool
	closed    atomic.Bool
	closeOnce sync.Once

	outputErrors atomic.Uint64
	closeSeq     atomic.Uint64

	closeAckMu      sync.Mutex
	closeAckWaiters map[closeToken]chan struct{}
}

type acceptResult struct {
	conn    net.Conn
	service uint64
}

type directStream struct{ conn net.Conn }

func (s *directStream) Read(b []byte) (int, error)         { return s.conn.Read(b) }
func (s *directStream) Write(b []byte) (int, error)        { return s.conn.Write(b) }
func (s *directStream) Close() error                       { return nil }
func (s *directStream) LocalAddr() net.Addr                { return s.conn.LocalAddr() }
func (s *directStream) RemoteAddr() net.Addr               { return s.conn.RemoteAddr() }
func (s *directStream) SetDeadline(t time.Time) error      { return s.conn.SetDeadline(t) }
func (s *directStream) SetReadDeadline(t time.Time) error  { return s.conn.SetReadDeadline(t) }
func (s *directStream) SetWriteDeadline(t time.Time) error { return s.conn.SetWriteDeadline(t) }

func wrapDirectStream(conn net.Conn) net.Conn {
	if conn == nil {
		return nil
	}
	return &directStream{conn: conn}
}

func NewServiceMux(cfg ServiceMuxConfig) *ServiceMux {
	return &ServiceMux{
		config:          cfg,
		services:        make(map[uint64]*serviceEntry),
		acceptCh:        make(chan acceptResult, 4096),
		closeCh:         make(chan struct{}),
		closeAckWaiters: make(map[closeToken]chan struct{}),
	}
}

func (m *ServiceMux) Input(service uint64, data []byte) error {
	if len(data) == 0 {
		return ErrInvalidServiceFrame
	}

	frameType := data[0]
	payload := data[1:]

	switch frameType {
	case serviceFrameData:
		if m.closed.Load() {
			return ErrServiceMuxClosed
		}
		return m.handleDataFrame(service, payload)
	case serviceFrameClose:
		return m.handleCloseFrame(service, payload)
	case serviceFrameCloseAck:
		return m.handleCloseAckFrame(service, payload)
	default:
		return ErrInvalidServiceFrame
	}
}

func (m *ServiceMux) handleDataFrame(service uint64, payload []byte) error {
	if m.closing.Load() {
		return ErrServiceMuxClosed
	}

	entry, err := m.getOrCreateService(service)
	if err != nil {
		return err
	}

	if err := entry.conn.Input(payload); err != nil {
		if !errors.Is(err, ErrConnClosed) {
			return err
		}
		entry, err = m.recreateService(service, entry)
		if err != nil {
			return err
		}
		if err := entry.conn.Input(payload); err != nil {
			return err
		}
	}

	m.announceAccept(service, entry)
	return nil
}

func (m *ServiceMux) handleCloseFrame(service uint64, payload []byte) error {
	if len(payload) < 9 {
		return ErrInvalidServiceFrame
	}

	closeID := binary.BigEndian.Uint64(payload[:8])
	_ = payload[8]

	m.sendCloseAck(service, closeID)
	m.closeService(service, serviceCloseReasonPeer)
	return nil
}

func (m *ServiceMux) handleCloseAckFrame(service uint64, payload []byte) error {
	if len(payload) < 8 {
		return ErrInvalidServiceFrame
	}

	closeID := binary.BigEndian.Uint64(payload[:8])
	m.notifyCloseAck(service, closeID)
	return nil
}

func (m *ServiceMux) OpenStream(service uint64) (net.Conn, error) {
	if m.closed.Load() || m.closing.Load() {
		return nil, ErrServiceMuxClosed
	}

	entry, err := m.getOrCreateService(service)
	if err != nil {
		return nil, err
	}

	if entry.conn.IsClosed() {
		entry, err = m.recreateService(service, entry)
		if err != nil {
			return nil, err
		}
	}

	return wrapDirectStream(entry.conn), nil
}

func (m *ServiceMux) AcceptStream() (net.Conn, uint64, error) {
	if m.closed.Load() || m.closing.Load() {
		return nil, 0, ErrServiceMuxClosed
	}

	select {
	case result := <-m.acceptCh:
		if m.closed.Load() {
			return nil, 0, ErrServiceMuxClosed
		}
		return wrapDirectStream(result.conn), result.service, nil
	case <-m.closeCh:
		return nil, 0, ErrServiceMuxClosed
	}
}

func (m *ServiceMux) AcceptStreamOn(service uint64) (net.Conn, error) {
	if m.closed.Load() || m.closing.Load() {
		return nil, ErrServiceMuxClosed
	}

	entry, err := m.getOrCreateService(service)
	if err != nil {
		return nil, err
	}

	if entry.announced.Load() {
		if m.closed.Load() {
			return nil, ErrServiceMuxClosed
		}
		return wrapDirectStream(entry.conn), nil
	}

	select {
	case <-entry.readyCh:
		if m.closed.Load() {
			return nil, ErrServiceMuxClosed
		}
		return wrapDirectStream(entry.conn), nil
	case <-m.closeCh:
		return nil, ErrServiceMuxClosed
	}
}

func (m *ServiceMux) Close() error {
	m.closeOnce.Do(func() {
		m.closed.Store(true)
		m.closing.Store(true)
		close(m.closeCh)

		services := m.detachServices()
		m.activeCloseServices(services)

		for _, entry := range services {
			m.closeServiceEntry(entry, serviceCloseReasonLocal)
		}
		m.clearCloseAckWaiters()
	})
	return nil
}

func (m *ServiceMux) detachServices() map[uint64]*serviceEntry {
	m.servicesMu.Lock()
	defer m.servicesMu.Unlock()

	detached := make(map[uint64]*serviceEntry, len(m.services))
	for service, entry := range m.services {
		detached[service] = entry
	}
	m.services = make(map[uint64]*serviceEntry)
	return detached
}

func (m *ServiceMux) closeService(service uint64, reason byte) {
	m.servicesMu.Lock()
	entry, ok := m.services[service]
	if ok {
		delete(m.services, service)
	}
	m.servicesMu.Unlock()

	if ok {
		m.closeServiceEntry(entry, reason)
	}
}

func (m *ServiceMux) closeServiceEntry(entry *serviceEntry, reason byte) {
	if entry == nil {
		return
	}
	closeErr := ErrConnClosedLocal
	if reason == serviceCloseReasonPeer {
		closeErr = ErrConnClosedByPeer
	}
	_ = entry.conn.closeWithReason(closeErr)
}

func (m *ServiceMux) activeCloseServices(services map[uint64]*serviceEntry) {
	if len(services) == 0 {
		return
	}

	var wg sync.WaitGroup
	for service := range services {
		svc := service
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.sendCloseAndWaitAck(svc)
		}()
	}
	wg.Wait()
}

func (m *ServiceMux) sendCloseAndWaitAck(service uint64) {
	if m.config.Output == nil {
		return
	}

	closeID := m.closeSeq.Add(1)
	waiter := m.registerCloseAck(service, closeID)
	defer m.unregisterCloseAck(service, closeID)

	m.sendCloseFrame(service, closeID, serviceCloseReasonLocal)

	deadline := time.NewTimer(activeCloseAckTimeout)
	retry := time.NewTicker(activeCloseRetryInterval)
	defer deadline.Stop()
	defer retry.Stop()

	for {
		select {
		case <-waiter:
			return
		case <-retry.C:
			m.sendCloseFrame(service, closeID, serviceCloseReasonLocal)
		case <-deadline.C:
			return
		}
	}
}

func (m *ServiceMux) sendCloseFrame(service uint64, closeID uint64, reason byte) {
	frame := make([]byte, 1+8+1)
	frame[0] = serviceFrameClose
	binary.BigEndian.PutUint64(frame[1:9], closeID)
	frame[9] = reason
	if err := m.config.Output(service, frame); err != nil {
		m.reportOutputError(service, err)
	}
}

func (m *ServiceMux) sendCloseAck(service uint64, closeID uint64) {
	if m.config.Output == nil {
		return
	}

	frame := make([]byte, 1+8)
	frame[0] = serviceFrameCloseAck
	binary.BigEndian.PutUint64(frame[1:9], closeID)
	if err := m.config.Output(service, frame); err != nil {
		m.reportOutputError(service, err)
	}
}

func (m *ServiceMux) registerCloseAck(service uint64, closeID uint64) chan struct{} {
	token := closeToken{service: service, id: closeID}
	ch := make(chan struct{})

	m.closeAckMu.Lock()
	m.closeAckWaiters[token] = ch
	m.closeAckMu.Unlock()

	return ch
}

func (m *ServiceMux) unregisterCloseAck(service uint64, closeID uint64) {
	token := closeToken{service: service, id: closeID}
	m.closeAckMu.Lock()
	delete(m.closeAckWaiters, token)
	m.closeAckMu.Unlock()
}

func (m *ServiceMux) notifyCloseAck(service uint64, closeID uint64) {
	token := closeToken{service: service, id: closeID}

	m.closeAckMu.Lock()
	ch, ok := m.closeAckWaiters[token]
	if ok {
		delete(m.closeAckWaiters, token)
	}
	m.closeAckMu.Unlock()

	if ok {
		close(ch)
	}
}

func (m *ServiceMux) clearCloseAckWaiters() {
	m.closeAckMu.Lock()
	defer m.closeAckMu.Unlock()

	for token, ch := range m.closeAckWaiters {
		close(ch)
		delete(m.closeAckWaiters, token)
	}
}

func (m *ServiceMux) NumServices() int {
	m.servicesMu.RLock()
	defer m.servicesMu.RUnlock()
	return len(m.services)
}

func (m *ServiceMux) NumStreams() int { return m.NumServices() }

func (m *ServiceMux) OutputErrorCount() uint64 { return m.outputErrors.Load() }

func (m *ServiceMux) reportOutputError(service uint64, err error) {
	if err == nil {
		return
	}
	m.outputErrors.Add(1)
	if m.config.OnOutputError != nil {
		m.config.OnOutputError(service, err)
	}
}

func (m *ServiceMux) announceAccept(service uint64, entry *serviceEntry) {
	if !entry.announced.CompareAndSwap(false, true) {
		return
	}

	entry.readyOnce.Do(func() {
		close(entry.readyCh)
	})

	select {
	case m.acceptCh <- acceptResult{conn: entry.conn, service: service}:
	case <-m.closeCh:
	}
}

func (m *ServiceMux) getOrCreateService(service uint64) (*serviceEntry, error) {
	if m.closing.Load() {
		return nil, ErrServiceMuxClosed
	}

	m.servicesMu.RLock()
	entry, ok := m.services[service]
	m.servicesMu.RUnlock()
	if ok && !entry.conn.IsClosed() {
		return entry, nil
	}

	m.servicesMu.Lock()
	defer m.servicesMu.Unlock()

	if entry, ok := m.services[service]; ok {
		if !entry.conn.IsClosed() {
			return entry, nil
		}
		_ = entry.conn.Close()
		delete(m.services, service)
	}

	if m.closed.Load() || m.closing.Load() {
		return nil, ErrServiceMuxClosed
	}

	if m.config.OnNewService != nil && !m.config.OnNewService(service) {
		return nil, ErrServiceRejected
	}
	if m.closed.Load() || m.closing.Load() {
		return nil, ErrServiceMuxClosed
	}

	entry = m.createServiceLocked(service)
	m.services[service] = entry
	return entry, nil
}

func (m *ServiceMux) recreateService(service uint64, stale *serviceEntry) (*serviceEntry, error) {
	m.servicesMu.Lock()
	defer m.servicesMu.Unlock()

	if m.closed.Load() || m.closing.Load() {
		return nil, ErrServiceMuxClosed
	}

	current, ok := m.services[service]
	if ok && current != stale && !current.conn.IsClosed() {
		return current, nil
	}

	if ok {
		_ = current.conn.Close()
	}

	entry := m.createServiceLocked(service)
	m.services[service] = entry
	return entry, nil
}

func (m *ServiceMux) createServiceLocked(service uint64) *serviceEntry {
	conn := NewKCPConn(uint32(service), func(data []byte) {
		if m.config.Output == nil {
			return
		}
		framed := make([]byte, 1+len(data))
		framed[0] = serviceFrameData
		copy(framed[1:], data)
		if err := m.config.Output(service, framed); err != nil {
			m.reportOutputError(service, err)
		}
	})

	return &serviceEntry{conn: conn, readyCh: make(chan struct{})}
}
