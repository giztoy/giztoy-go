package transport

import (
	"net"
	"time"

	"github.com/giztoy/giztoy-go/pkg/net/internal/socketopt"
)

// UDPAddr wraps net.UDPAddr.
type UDPAddr struct {
	addr *net.UDPAddr
}

// Network returns "udp".
func (a *UDPAddr) Network() string {
	return "udp"
}

// String returns the address string.
func (a *UDPAddr) String() string {
	return a.addr.String()
}

// UDPAddrFromNetAddr wraps a net.UDPAddr.
func UDPAddrFromNetAddr(addr *net.UDPAddr) *UDPAddr {
	return &UDPAddr{addr: addr}
}

// UDPTransport is a UDP-based transport implementation.
type UDPTransport struct {
	conn *net.UDPConn
}

// NewUDPTransport creates a new UDP transport bound to the specified address.
// Use "127.0.0.1:0" or ":0" to bind to a random available port.
func NewUDPTransport(addr string) (*UDPTransport, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	socketopt.Apply(conn, socketopt.DefaultConfig())

	return &UDPTransport{conn: conn}, nil
}

// WriteTo sends data to the specified address.
func (t *UDPTransport) WriteTo(data []byte, addr net.Addr) (int, error) {
	var udpAddr *net.UDPAddr

	switch a := addr.(type) {
	case *UDPAddr:
		udpAddr = a.addr
	case *net.UDPAddr:
		udpAddr = a
	default:
		// Try to resolve from string
		var err error
		udpAddr, err = net.ResolveUDPAddr("udp", addr.String())
		if err != nil {
			return 0, err
		}
	}

	return t.conn.WriteTo(data, udpAddr)
}

// SendTo is a compatibility wrapper around WriteTo.
func (t *UDPTransport) SendTo(data []byte, addr net.Addr) error {
	_, err := t.WriteTo(data, addr)
	return err
}

// ReadFrom receives data and returns the sender's address.
func (t *UDPTransport) ReadFrom(buf []byte) (int, net.Addr, error) {
	return t.conn.ReadFrom(buf)
}

// RecvFrom is a compatibility wrapper around ReadFrom.
func (t *UDPTransport) RecvFrom(buf []byte) (int, net.Addr, error) {
	return t.ReadFrom(buf)
}

// Close closes the transport.
func (t *UDPTransport) Close() error {
	return t.conn.Close()
}

// LocalAddr returns the local address.
func (t *UDPTransport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}

// SetDeadline sets both read and write deadlines.
func (t *UDPTransport) SetDeadline(tt time.Time) error {
	return t.conn.SetDeadline(tt)
}

// SetReadDeadline sets the read deadline.
func (t *UDPTransport) SetReadDeadline(tt time.Time) error {
	return t.conn.SetReadDeadline(tt)
}

// SetWriteDeadline sets the write deadline.
func (t *UDPTransport) SetWriteDeadline(tt time.Time) error {
	return t.conn.SetWriteDeadline(tt)
}
