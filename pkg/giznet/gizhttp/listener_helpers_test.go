package gizhttp

import (
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/pkg/giznet"
)

// newListenerNode creates a giznet.Listener for tests using only public APIs.
func newListenerNode(t *testing.T, key *giznet.KeyPair, opts ...giznet.Option) *giznet.Listener {
	t.Helper()

	defaults := []giznet.Option{
		giznet.WithBindAddr("127.0.0.1:0"),
		giznet.WithAllowUnknown(true),
		giznet.WithDecryptWorkers(1),
	}
	l, err := giznet.Listen(key, append(defaults, opts...)...)
	if err != nil {
		t.Fatalf("giznet.Listen failed: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	u := l.UDP()
	go func() {
		buf := make([]byte, 65535)
		for {
			if _, _, err := u.ReadFrom(buf); err != nil {
				return
			}
		}
	}()

	return l
}

func connectListenerNodes(t *testing.T, client *giznet.Listener, clientKey *giznet.KeyPair, server *giznet.Listener, serverKey *giznet.KeyPair) {
	t.Helper()

	client.SetPeerEndpoint(serverKey.Public, server.HostInfo().Addr)
	server.SetPeerEndpoint(clientKey.Public, client.HostInfo().Addr)

	if err := client.Connect(serverKey.Public); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	waitForPeerEstablished(t, client.UDP(), serverKey.Public)
	waitForPeerEstablished(t, server.UDP(), clientKey.Public)
}

func waitForPeerEstablished(t *testing.T, u *giznet.UDP, pk giznet.PublicKey) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		info := u.PeerInfo(pk)
		if info != nil && info.State == giznet.PeerStateEstablished {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	info := u.PeerInfo(pk)
	if info == nil {
		t.Fatalf("peer %x was not registered before timeout", pk)
	}
	t.Fatalf("peer %x state=%v, want %v", pk, info.State, giznet.PeerStateEstablished)
}
