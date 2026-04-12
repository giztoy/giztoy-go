package framework

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

const (
	defaultUDPBasePort = 41000
	envUDPBasePort     = "BENCH_NET_UDP_BASE_PORT"
)

// PacketHandler handles an inbound UDP packet copy.
type PacketHandler func(data []byte)

// UDPPairConfig controls local UDP test harness behavior.
type UDPPairConfig struct {
	BasePort int
	LossAB   float64       // A -> B simulated drop rate [0,1]
	LossBA   float64       // B -> A simulated drop rate [0,1]
	Delay    time.Duration // Simulated fixed one-way delay
	SeedAB   uint64
	SeedBA   uint64
}

// UDPPair provides two local UDP endpoints with optional software loss/delay.
type UDPPair struct {
	aConn *net.UDPConn
	bConn *net.UDPConn
	aAddr *net.UDPAddr
	bAddr *net.UDPAddr

	lossAB *LossModel
	lossBA *LossModel
	delay  time.Duration

	mu    sync.RWMutex
	onA   PacketHandler
	onB   PacketHandler
	stop  chan struct{}
	close sync.Once
	wg    sync.WaitGroup
}

// NewUDPPair creates two localhost UDP sockets and starts read loops.
func NewUDPPair(tb testing.TB, cfg UDPPairConfig) *UDPPair {
	tb.Helper()
	p, err := NewUDPPairWithError(cfg)
	if err != nil {
		tb.Fatalf("new udp pair failed: %v", err)
	}
	tb.Cleanup(func() {
		_ = p.Close()
	})
	return p
}

// NewUDPPairWithError creates two localhost UDP sockets and starts read loops.
func NewUDPPairWithError(cfg UDPPairConfig) (*UDPPair, error) {
	basePort := cfg.BasePort
	if basePort <= 0 {
		basePort = resolveBasePortFromEnv(defaultUDPBasePort)
	}

	aAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: basePort}
	bAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: basePort + 1}

	aConn, err := net.ListenUDP("udp", aAddr)
	if err != nil {
		return nil, fmt.Errorf("listen udp A(%s) failed: %w", aAddr, err)
	}
	bConn, err := net.ListenUDP("udp", bAddr)
	if err != nil {
		_ = aConn.Close()
		return nil, fmt.Errorf("listen udp B(%s) failed: %w", bAddr, err)
	}

	p := &UDPPair{
		aConn:  aConn,
		bConn:  bConn,
		aAddr:  aConn.LocalAddr().(*net.UDPAddr),
		bAddr:  bConn.LocalAddr().(*net.UDPAddr),
		lossAB: NewLossModel(cfg.LossAB, nonZeroSeed(cfg.SeedAB, 0x11111111)),
		lossBA: NewLossModel(cfg.LossBA, nonZeroSeed(cfg.SeedBA, 0x22222222)),
		delay:  cfg.Delay,
		stop:   make(chan struct{}),
	}

	p.wg.Add(2)
	go p.readLoop(p.aConn, true)
	go p.readLoop(p.bConn, false)
	return p, nil
}

// AddrA returns endpoint A local address.
func (p *UDPPair) AddrA() *net.UDPAddr { return p.aAddr }

// AddrB returns endpoint B local address.
func (p *UDPPair) AddrB() *net.UDPAddr { return p.bAddr }

// SetOnA sets inbound handler for packets received by endpoint A.
func (p *UDPPair) SetOnA(h PacketHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onA = h
}

// SetOnB sets inbound handler for packets received by endpoint B.
func (p *UDPPair) SetOnB(h PacketHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onB = h
}

// SendAToB sends one UDP datagram from A to B with optional simulated loss.
func (p *UDPPair) SendAToB(data []byte) error {
	if p.lossAB.ShouldDrop() {
		return nil
	}
	if p.delay > 0 {
		time.Sleep(p.delay)
	}
	_, err := p.aConn.WriteToUDP(data, p.bAddr)
	return err
}

// SendBToA sends one UDP datagram from B to A with optional simulated loss.
func (p *UDPPair) SendBToA(data []byte) error {
	if p.lossBA.ShouldDrop() {
		return nil
	}
	if p.delay > 0 {
		time.Sleep(p.delay)
	}
	_, err := p.bConn.WriteToUDP(data, p.aAddr)
	return err
}

// LossAB returns A->B loss model.
func (p *UDPPair) LossAB() *LossModel { return p.lossAB }

// LossBA returns B->A loss model.
func (p *UDPPair) LossBA() *LossModel { return p.lossBA }

// Close closes both endpoints and read loops.
func (p *UDPPair) Close() error {
	var closeErr error
	p.close.Do(func() {
		close(p.stop)
		if err := p.aConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			closeErr = err
		}
		if err := p.bConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) && closeErr == nil {
			closeErr = err
		}
		p.wg.Wait()
	})
	return closeErr
}

func (p *UDPPair) readLoop(conn *net.UDPConn, forA bool) {
	defer p.wg.Done()
	buf := make([]byte, 64*1024)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-p.stop:
				return
			default:
				return
			}
		}
		if n <= 0 {
			continue
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		p.mu.RLock()
		var h PacketHandler
		if forA {
			h = p.onA
		} else {
			h = p.onB
		}
		p.mu.RUnlock()

		if h != nil {
			h(data)
		}
	}
}

func resolveBasePortFromEnv(def int) int {
	raw := os.Getenv(envUDPBasePort)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 || v > 65533 {
		return def
	}
	return v
}

func nonZeroSeed(v, fallback uint64) uint64 {
	if v != 0 {
		return v
	}
	return fallback
}

// PortsString returns "A,B" for diagnostics and scripts.
func (p *UDPPair) PortsString() string {
	return fmt.Sprintf("%d,%d", p.aAddr.Port, p.bAddr.Port)
}
