package peer

import (
	"net"

	"github.com/haivivi/giztoy/go/pkg/net/core"
	"github.com/haivivi/giztoy/go/pkg/net/noise"
)

type Conn struct {
	udp *core.UDP
	pk  noise.PublicKey
}

func (c *Conn) OpenService(service uint64) (net.Conn, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c.udp.OpenStream(c.pk, service)
}

// AcceptService is the peer-layer wrapper around the per-service accept path.
func (c *Conn) AcceptService(service uint64) (net.Conn, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c.udp.AcceptStreamOn(c.pk, service)
}

func (c *Conn) OpenRPC() (net.Conn, error) {
	return c.OpenService(0)
}

func (c *Conn) AcceptRPC() (net.Conn, error) {
	return c.AcceptService(0)
}

func (c *Conn) SendEvent(evt Event) error {
	if err := c.validate(); err != nil {
		return err
	}

	payload, err := EncodeEvent(evt)
	if err != nil {
		return err
	}

	_, err = c.udp.Write(c.pk, noise.ProtocolEVENT, payload)
	return err
}

func (c *Conn) ReadEvent() (Event, error) {
	if err := c.validate(); err != nil {
		return Event{}, err
	}

	buf := make([]byte, noise.MaxPayloadSize)
	proto, n, err := c.udp.Read(c.pk, buf)
	if err != nil {
		return Event{}, err
	}
	if proto != noise.ProtocolEVENT {
		return Event{}, ErrUnexpectedProtocol
	}

	return DecodeEvent(buf[:n])
}

func (c *Conn) SendOpusFrame(frame StampedOpusFrame) error {
	if err := c.validate(); err != nil {
		return err
	}
	if err := frame.Validate(); err != nil {
		return err
	}

	_, err := c.udp.Write(c.pk, noise.ProtocolOPUS, frame)
	return err
}

func (c *Conn) ReadOpusFrame() (StampedOpusFrame, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	buf := make([]byte, noise.MaxPayloadSize)
	proto, n, err := c.udp.Read(c.pk, buf)
	if err != nil {
		return nil, err
	}
	if proto != noise.ProtocolOPUS {
		return nil, ErrUnexpectedProtocol
	}

	return ParseStampedOpusFrame(buf[:n])
}

func (c *Conn) Close() error {
	if err := c.validate(); err != nil {
		return err
	}
	return nil
}

func (c *Conn) PublicKey() noise.PublicKey {
	if c == nil {
		return noise.PublicKey{}
	}
	return c.pk
}

func (c *Conn) validate() error {
	if c == nil || c.udp == nil {
		return ErrNilConn
	}
	return nil
}
