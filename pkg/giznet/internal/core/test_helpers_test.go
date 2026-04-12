package core

import (
	"net"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/giznet/internal/noise"
)

// Non-ProtocolKCP payload markers used across core tests (legacy EVENT/OPUS bytes).
const (
	testDirectProtoA byte = 0x03
	testDirectProtoB byte = 0x10
)

func mustServiceMux(t *testing.T, u *UDP, pk noise.PublicKey) *ServiceMux {
	t.Helper()

	smux, err := u.GetServiceMux(pk)
	if err != nil {
		t.Fatalf("GetServiceMux failed: %v", err)
	}
	return smux
}

func mustOpenStream(t *testing.T, u *UDP, pk noise.PublicKey, service uint64) net.Conn {
	t.Helper()

	stream, err := mustServiceMux(t, u, pk).OpenStream(service)
	if err != nil {
		t.Fatalf("OpenStream(service=%d) failed: %v", service, err)
	}
	return stream
}

func mustAcceptStream(t *testing.T, u *UDP, pk noise.PublicKey, service uint64) net.Conn {
	t.Helper()

	stream, err := mustServiceMux(t, u, pk).AcceptStream(service)
	if err != nil {
		t.Fatalf("AcceptStream(service=%d) failed: %v", service, err)
	}
	return stream
}
