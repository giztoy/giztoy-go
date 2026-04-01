package adapters

import (
	"github.com/giztoy/giztoy-go/benchmark/net/internal/framework"
	"github.com/giztoy/giztoy-go/pkg/net/kcp"
)

// KCPPair is a raw KCP-over-UDP benchmark adapter.
type KCPPair struct {
	Link *framework.UDPPair
	A    *kcp.KCPConn
	B    *kcp.KCPConn
}

// NewKCPPair creates a raw KCP pair over local UDP sockets.
func NewKCPPair(loss float64) (*KCPPair, error) {
	link, err := framework.NewUDPPairWithError(framework.UDPPairConfig{
		LossAB: loss,
		LossBA: loss,
	})
	if err != nil {
		return nil, err
	}

	var aConn, bConn *kcp.KCPConn
	aConn = kcp.NewKCPConn(1, func(data []byte) {
		_ = link.SendAToB(data)
	})
	bConn = kcp.NewKCPConn(1, func(data []byte) {
		_ = link.SendBToA(data)
	})

	link.SetOnA(func(data []byte) {
		_ = aConn.Input(data)
	})
	link.SetOnB(func(data []byte) {
		_ = bConn.Input(data)
	})

	return &KCPPair{Link: link, A: aConn, B: bConn}, nil
}

// Close closes KCP endpoints and underlying UDP link.
func (p *KCPPair) Close() error {
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
