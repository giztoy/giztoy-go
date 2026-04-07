package core

import (
	"net"

	itransport "github.com/giztoy/giztoy-go/pkg/net/internal/transport"
)

// Transport primitives used by core network stack.

// UDPAddr wraps net.UDPAddr.
type UDPAddr = itransport.UDPAddr

// UDPAddrFromNetAddr wraps a net.UDPAddr.
func UDPAddrFromNetAddr(addr *net.UDPAddr) *UDPAddr {
	return itransport.UDPAddrFromNetAddr(addr)
}

// UDPTransport is a UDP-based transport implementation.
type UDPTransport = itransport.UDPTransport

// NewUDPTransport creates a new UDP transport bound to the specified address.
func NewUDPTransport(addr string) (*UDPTransport, error) {
	return itransport.NewUDPTransport(addr)
}

// MockAddr is a simple address for testing.
type MockAddr = itransport.MockAddr

// NewMockAddr creates a new mock address.
func NewMockAddr(name string) *MockAddr {
	return itransport.NewMockAddr(name)
}

// MockTransport is an in-memory transport for testing.
type MockTransport = itransport.MockTransport

// NewMockTransport creates a new mock transport.
func NewMockTransport(name string) *MockTransport {
	return itransport.NewMockTransport(name)
}

// MockTransport errors.
var (
	ErrMockTransportClosed = itransport.ErrMockTransportClosed
	ErrMockNoPeer          = itransport.ErrMockNoPeer
	ErrMockInboxFull       = itransport.ErrMockInboxFull
)
