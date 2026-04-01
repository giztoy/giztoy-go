package adapters

import (
	"fmt"
	"time"

	"github.com/giztoy/giztoy-go/benchmark/net/internal/framework"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

type noiseRecv struct {
	data []byte
	err  error
}

// NoisePair is a raw Noise transport benchmark adapter over UDP.
//
// A->B uses session A encrypt / session B decrypt.
// B->A uses session B encrypt / session A decrypt.
type NoisePair struct {
	Link *framework.UDPPair
	a    *noise.Session
	b    *noise.Session

	recvA chan noiseRecv
	recvB chan noiseRecv
}

// NewNoisePair creates a Noise transport pair over local UDP sockets.
func NewNoisePair(loss float64) (*NoisePair, error) {
	aSession, bSession, err := newNoiseSessionPair()
	if err != nil {
		return nil, err
	}

	link, err := framework.NewUDPPairWithError(framework.UDPPairConfig{
		LossAB: loss,
		LossBA: loss,
	})
	if err != nil {
		return nil, err
	}

	p := &NoisePair{
		Link:  link,
		a:     aSession,
		b:     bSession,
		recvA: make(chan noiseRecv, 8192),
		recvB: make(chan noiseRecv, 8192),
	}

	link.SetOnA(func(packet []byte) {
		pt, err := decodeNoiseTransport(p.a, packet)
		p.recvA <- noiseRecv{data: pt, err: err}
	})
	link.SetOnB(func(packet []byte) {
		pt, err := decodeNoiseTransport(p.b, packet)
		p.recvB <- noiseRecv{data: pt, err: err}
	})

	return p, nil
}

// SendAToB sends one plaintext payload from A to B.
func (p *NoisePair) SendAToB(payload []byte) error {
	packet, err := encodeNoiseTransport(p.a, payload)
	if err != nil {
		return err
	}
	return p.Link.SendAToB(packet)
}

// SendBToA sends one plaintext payload from B to A.
func (p *NoisePair) SendBToA(payload []byte) error {
	packet, err := encodeNoiseTransport(p.b, payload)
	if err != nil {
		return err
	}
	return p.Link.SendBToA(packet)
}

// RecvOnA receives one decrypted payload on endpoint A.
func (p *NoisePair) RecvOnA(timeout time.Duration) ([]byte, error) {
	select {
	case r := <-p.recvA:
		return r.data, r.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("noise recv A timeout after %s", timeout)
	}
}

// RecvOnB receives one decrypted payload on endpoint B.
func (p *NoisePair) RecvOnB(timeout time.Duration) ([]byte, error) {
	select {
	case r := <-p.recvB:
		return r.data, r.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("noise recv B timeout after %s", timeout)
	}
}

// RecvOnABlocking receives one decrypted payload on A without timeout overhead.
func (p *NoisePair) RecvOnABlocking() ([]byte, error) {
	r := <-p.recvA
	return r.data, r.err
}

// RecvOnBBlocking receives one decrypted payload on B without timeout overhead.
func (p *NoisePair) RecvOnBBlocking() ([]byte, error) {
	r := <-p.recvB
	return r.data, r.err
}

// Close closes underlying UDP link.
func (p *NoisePair) Close() error {
	if p.Link != nil {
		return p.Link.Close()
	}
	return nil
}
