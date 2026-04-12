package noise

import "net"

// Addr represents a transport-layer address.
// Use net.Addr directly for new code.
type Addr = net.Addr

// Transport is a compatibility interface for datagram transports.
// New code can depend on net.PacketConn directly.
type Transport interface {
	net.PacketConn

	// SendTo sends data to the specified address.
	SendTo(data []byte, addr Addr) error

	// RecvFrom receives data into the provided buffer.
	RecvFrom(buf []byte) (n int, addr Addr, err error)
}

type packetConnTransport struct {
	net.PacketConn
}

// WrapPacketConn adapts a standard net.PacketConn to the legacy Transport
// interface. If pc already implements Transport, it is returned as-is.
func WrapPacketConn(pc net.PacketConn) Transport {
	if pc == nil {
		return nil
	}
	if transport, ok := pc.(Transport); ok {
		return transport
	}
	return &packetConnTransport{PacketConn: pc}
}

func (t *packetConnTransport) SendTo(data []byte, addr Addr) error {
	_, err := t.WriteTo(data, addr)
	return err
}

func (t *packetConnTransport) RecvFrom(buf []byte) (int, Addr, error) {
	return t.ReadFrom(buf)
}
