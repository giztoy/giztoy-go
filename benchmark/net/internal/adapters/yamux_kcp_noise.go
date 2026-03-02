package adapters

import (
	"encoding/binary"
	"errors"

	"github.com/haivivi/giztoy/go/benchmark/net/internal/framework"
	"github.com/haivivi/giztoy/go/pkg/net/kcp"
	"github.com/haivivi/giztoy/go/pkg/net/noise"
)

const serviceHeaderSize = 8

// YamuxKCPNoisePair is a yamux-over-kcp-over-noise adapter over local UDP sockets.
type YamuxKCPNoisePair struct {
	Link   *framework.UDPPair
	Client *kcp.ServiceMux
	Server *kcp.ServiceMux
}

// NewYamuxKCPNoisePair creates a client/server ServiceMux pair over Noise+UDP.
func NewYamuxKCPNoisePair(loss float64) (*YamuxKCPNoisePair, error) {
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

	pair := &YamuxKCPNoisePair{Link: link}

	var clientMux, serverMux *kcp.ServiceMux
	clientMux = kcp.NewServiceMux(kcp.ServiceMuxConfig{
		IsClient: true,
		Output: func(service uint64, data []byte) error {
			packet, err := encodeServiceNoisePacket(aSession, service, data)
			if err != nil {
				return err
			}
			return link.SendAToB(packet)
		},
	})
	serverMux = kcp.NewServiceMux(kcp.ServiceMuxConfig{
		IsClient: false,
		Output: func(service uint64, data []byte) error {
			packet, err := encodeServiceNoisePacket(bSession, service, data)
			if err != nil {
				return err
			}
			return link.SendBToA(packet)
		},
	})

	link.SetOnA(func(packet []byte) {
		service, plaintext, err := decodeServiceNoisePacket(aSession, packet)
		if err != nil {
			return
		}
		_ = clientMux.Input(service, plaintext)
	})
	link.SetOnB(func(packet []byte) {
		service, plaintext, err := decodeServiceNoisePacket(bSession, packet)
		if err != nil {
			return
		}
		_ = serverMux.Input(service, plaintext)
	})

	pair.Client = clientMux
	pair.Server = serverMux
	return pair, nil
}

// Close closes service muxes and UDP link.
func (p *YamuxKCPNoisePair) Close() error {
	var first error
	if p.Client != nil {
		if err := p.Client.Close(); err != nil && first == nil {
			first = err
		}
	}
	if p.Server != nil {
		if err := p.Server.Close(); err != nil && first == nil {
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

func encodeServiceNoisePacket(s *noise.Session, service uint64, data []byte) ([]byte, error) {
	pt := make([]byte, serviceHeaderSize+len(data))
	binary.LittleEndian.PutUint64(pt[:serviceHeaderSize], service)
	copy(pt[serviceHeaderSize:], data)
	return encodeNoiseTransport(s, pt)
}

func decodeServiceNoisePacket(s *noise.Session, packet []byte) (service uint64, plaintext []byte, err error) {
	pt, err := decodeNoiseTransport(s, packet)
	if err != nil {
		return 0, nil, err
	}
	if len(pt) < serviceHeaderSize {
		return 0, nil, errors.New("service packet too short")
	}
	service = binary.LittleEndian.Uint64(pt[:serviceHeaderSize])
	return service, pt[serviceHeaderSize:], nil
}
