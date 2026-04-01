package adapters

import (
	"github.com/giztoy/giztoy-go/benchmark/net/internal/framework"
	"github.com/giztoy/giztoy-go/pkg/net/kcp"
)

// KCPNoisePair is a KCP-over-Noise benchmark adapter over local UDP sockets.
type KCPNoisePair struct {
	Link *framework.UDPPair
	A    *kcp.KCPConn
	B    *kcp.KCPConn

	noiseA *noiseEndpoint
	noiseB *noiseEndpoint
}

type noiseEndpoint struct {
	enc, dec noiseSession
}

type noiseSession interface {
	Encrypt(plaintext []byte) (ciphertext []byte, nonce uint64, err error)
	Decrypt(ciphertext []byte, nonce uint64) ([]byte, error)
}

// NewKCPNoisePair creates a KCP-over-Noise pair.
func NewKCPNoisePair(loss float64) (*KCPNoisePair, error) {
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

	p := &KCPNoisePair{
		Link:   link,
		noiseA: &noiseEndpoint{enc: aSession, dec: aSession},
		noiseB: &noiseEndpoint{enc: bSession, dec: bSession},
	}

	var aConn, bConn *kcp.KCPConn
	aConn = kcp.NewKCPConn(1, func(data []byte) {
		packet, err := encodeNoiseTransport(aSession, data)
		if err != nil {
			return
		}
		_ = link.SendAToB(packet)
	})
	bConn = kcp.NewKCPConn(1, func(data []byte) {
		packet, err := encodeNoiseTransport(bSession, data)
		if err != nil {
			return
		}
		_ = link.SendBToA(packet)
	})

	link.SetOnA(func(packet []byte) {
		pt, err := decodeNoiseTransport(aSession, packet)
		if err != nil {
			return
		}
		_ = aConn.Input(pt)
	})
	link.SetOnB(func(packet []byte) {
		pt, err := decodeNoiseTransport(bSession, packet)
		if err != nil {
			return
		}
		_ = bConn.Input(pt)
	})

	p.A = aConn
	p.B = bConn
	return p, nil
}

// Close closes both KCP conns and UDP link.
func (p *KCPNoisePair) Close() error {
	var first error
	if p.A != nil {
		if err := p.A.Close(); err != nil && first == nil {
			first = err
		}
	}
	if p.B != nil {
		if err := p.B.Close(); err != nil && first == nil {
			first = err
		}
	}
	if p.Link != nil {
		if err := p.Link.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
