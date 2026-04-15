package giznet

import (
	"net"
	"sync/atomic"

	"github.com/GizClaw/gizclaw-go/pkg/giznet/internal/core"
)

type Conn struct {
	udp      *core.UDP
	pk       PublicKey
	listener *Listener
	closed   atomic.Bool
}

func (c *Conn) Dial(service uint64) (net.Conn, error) {
	smux, err := c.serviceMux()
	if err != nil {
		return nil, err
	}
	return smux.OpenStream(service)
}

func (c *Conn) CloseService(service uint64) error {
	smux, err := c.serviceMux()
	if err != nil {
		return err
	}
	return smux.CloseService(service)
}

func (c *Conn) Read(buf []byte) (byte, int, error) {
	if err := c.validate(); err != nil {
		return 0, 0, err
	}
	smux, err := c.serviceMux()
	if err != nil {
		return 0, 0, err
	}
	return smux.Read(buf)
}

func (c *Conn) Write(protocol byte, payload []byte) (int, error) {
	if err := c.validate(); err != nil {
		return 0, err
	}
	smux, err := c.serviceMux()
	if err != nil {
		return 0, err
	}
	return smux.Write(protocol, payload)
}

// Close marks this handle as closed and releases the peer from the listener's
// known set so it can be re-accepted via Listener.Accept. It does NOT tear
// down the underlying UDP peer or KCP session.
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

func (c *Conn) PublicKey() PublicKey {
	if c == nil {
		return PublicKey{}
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
