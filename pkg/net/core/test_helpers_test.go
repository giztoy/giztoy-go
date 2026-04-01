package core

import (
	"net"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/net/noise"
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
