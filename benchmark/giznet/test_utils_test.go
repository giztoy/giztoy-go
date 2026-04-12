package giznet_test

import (
	"net"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/pkg/giznet"
)

// peerBenchMux is the minimal mux surface used by public benchmarks.
type peerBenchMux interface {
	Write(protocol byte, data []byte) (n int, err error)
	Read(buf []byte) (protocol byte, n int, err error)
	OpenStream(service uint64) (net.Conn, error)
	AcceptStream(service uint64) (net.Conn, error)
}

func newBenchUDPNode(tb testing.TB, key *giznet.KeyPair) *giznet.UDP {
	tb.Helper()

	l, err := giznet.Listen(key,
		giznet.WithBindAddr("127.0.0.1:0"),
		giznet.WithAllowUnknown(true),
		giznet.WithDecryptWorkers(1),
	)
	if err != nil {
		tb.Fatalf("giznet.Listen failed: %v", err)
	}
	tb.Cleanup(func() { _ = l.Close() })

	u := l.UDP()
	go func() {
		buf := make([]byte, 65535)
		for {
			if _, _, err := u.ReadFrom(buf); err != nil {
				return
			}
		}
	}()

	return u
}

func connectBenchNodes(tb testing.TB, client *giznet.UDP, clientKey *giznet.KeyPair, server *giznet.UDP, serverKey *giznet.KeyPair) {
	tb.Helper()

	client.SetPeerEndpoint(serverKey.Public, server.HostInfo().Addr)
	server.SetPeerEndpoint(clientKey.Public, client.HostInfo().Addr)

	if err := client.Connect(serverKey.Public); err != nil {
		tb.Fatalf("Connect failed: %v", err)
	}

	waitBenchPeerEstablished(tb, client, serverKey.Public)
	waitBenchPeerEstablished(tb, server, clientKey.Public)
}

func waitBenchPeerEstablished(tb testing.TB, u *giznet.UDP, pk giznet.PublicKey) {
	tb.Helper()

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
		tb.Fatalf("peer %x was not registered before timeout", pk)
	}
	tb.Fatalf("peer %x state=%v, want %v", pk, info.State, giznet.PeerStateEstablished)
}

func mustPeerBenchMux(tb testing.TB, u *giznet.UDP, pk giznet.PublicKey) peerBenchMux {
	tb.Helper()

	smux, err := u.GetServiceMux(pk)
	if err != nil {
		tb.Fatalf("GetServiceMux failed: %v", err)
	}
	return smux
}

func newBenchListenerNode(tb testing.TB, key *giznet.KeyPair, opts ...giznet.Option) *giznet.Listener {
	tb.Helper()

	defaults := []giznet.Option{
		giznet.WithBindAddr("127.0.0.1:0"),
		giznet.WithAllowUnknown(true),
		giznet.WithDecryptWorkers(1),
	}
	l, err := giznet.Listen(key, append(defaults, opts...)...)
	if err != nil {
		tb.Fatalf("giznet.Listen failed: %v", err)
	}
	tb.Cleanup(func() { _ = l.Close() })

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

func connectBenchListenerNodes(tb testing.TB, client *giznet.Listener, clientKey *giznet.KeyPair, server *giznet.Listener, serverKey *giznet.KeyPair) {
	tb.Helper()

	client.SetPeerEndpoint(serverKey.Public, server.HostInfo().Addr)
	server.SetPeerEndpoint(clientKey.Public, client.HostInfo().Addr)
	if err := client.Connect(serverKey.Public); err != nil {
		tb.Fatalf("Connect failed: %v", err)
	}

	waitBenchPeerEstablished(tb, client.UDP(), serverKey.Public)
	waitBenchPeerEstablished(tb, server.UDP(), clientKey.Public)
}
