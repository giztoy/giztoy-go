package peer

import (
	"net"
	"sync/atomic"

	"github.com/giztoy/giztoy-go/pkg/net/core"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

type Conn struct {
	udp      *core.UDP
	pk       noise.PublicKey
	listener *Listener
	closed   atomic.Bool
}

func (c *Conn) OpenService(service uint64) (net.Conn, error) {
	smux, err := c.serviceMux()
	if err != nil {
		return nil, err
	}
	return smux.OpenStream(service)
}

// AcceptService is the peer-layer wrapper around the per-service accept path.
func (c *Conn) AcceptService(service uint64) (net.Conn, error) {
	smux, err := c.serviceMux()
	if err != nil {
		return nil, err
	}
	return smux.AcceptStream(service)
}

func (c *Conn) OpenRPC() (net.Conn, error) {
	return c.OpenService(ServicePublic)
}

func (c *Conn) AcceptRPC() (net.Conn, error) {
	return c.AcceptService(ServicePublic)
}

func (c *Conn) CloseService(service uint64) error {
	smux, err := c.serviceMux()
	if err != nil {
		return err
	}
	return smux.CloseService(service)
}

func (c *Conn) StopAcceptingService(service uint64) error {
	smux, err := c.serviceMux()
	if err != nil {
		return err
	}
	return smux.StopAcceptingService(service)
}

func (c *Conn) SendEvent(evt Event) error {
	if err := c.validate(); err != nil {
		return err
	}

	payload, err := EncodeEvent(evt)
	if err != nil {
		return err
	}

	smux, err := c.serviceMux()
	if err != nil {
		return err
	}
	_, err = smux.Write(core.ProtocolEVENT, payload)
	return err
}

func (c *Conn) ReadEvent() (Event, error) {
	if err := c.validate(); err != nil {
		return Event{}, err
	}

	smux, err := c.serviceMux()
	if err != nil {
		return Event{}, err
	}
	buf := make([]byte, noise.MaxPayloadSize)
	n, err := smux.ReadProtocol(core.ProtocolEVENT, buf)
	if err != nil {
		return Event{}, err
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

	smux, err := c.serviceMux()
	if err != nil {
		return err
	}
	_, err = smux.Write(core.ProtocolOPUS, frame)
	return err
}

func (c *Conn) ReadOpusFrame() (StampedOpusFrame, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	smux, err := c.serviceMux()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, noise.MaxPayloadSize)
	n, err := smux.ReadProtocol(core.ProtocolOPUS, buf)
	if err != nil {
		return nil, err
	}
	return ParseStampedOpusFrame(buf[:n])
}

// Close marks this handle as closed and releases the peer from the listener's
// known set so it can be re-accepted via Listener.Accept. It does NOT tear
// down the underlying UDP peer or KCP session — those stay alive until the
// owning UDP transport is closed or the peer is explicitly removed there.
func (c *Conn) Close() error {
	if err := c.validate(); err != nil {
		return err
	}
	c.closed.Store(true)
	if c.listener != nil {
		c.listener.release(c.pk)
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
	if c.closed.Load() {
		return ErrConnClosed
	}
	return nil
}

func (c *Conn) serviceMux() (*core.ServiceMux, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c.udp.GetServiceMux(c.pk)
}
