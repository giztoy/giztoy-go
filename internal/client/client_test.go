package client

import (
	"context"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/internal/server"
	"github.com/haivivi/giztoy/go/pkg/net/noise"
)

func TestDialAndPing(t *testing.T) {
	dir := t.TempDir()
	cfg := server.Config{DataDir: dir, ListenAddr: "127.0.0.1:0"}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()
	time.Sleep(200 * time.Millisecond)

	serverPK := srv.PublicKey()
	// Access listener info via a ping to discover the address.
	// We need to get the actual listen address since we used :0.
	// The server logs it, but we need it programmatically.
	// Let's use a helper that reads it from the server.
	serverAddr := getServerAddr(t, srv)

	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	c, err := Dial(clientKey, serverAddr, serverPK)
	if err != nil {
		t.Fatalf("Dial err=%v", err)
	}
	defer c.Close()

	result, err := c.Ping()
	if err != nil {
		t.Fatalf("Ping err=%v", err)
	}

	if result.ServerTime.IsZero() {
		t.Fatal("ServerTime is zero")
	}
	if result.RTT <= 0 {
		t.Fatalf("RTT=%v", result.RTT)
	}

	// Clock diff should be small for localhost.
	if result.ClockDiff > time.Second || result.ClockDiff < -time.Second {
		t.Fatalf("ClockDiff=%v (too large for localhost)", result.ClockDiff)
	}

	t.Logf("RTT=%v ClockDiff=%v", result.RTT, result.ClockDiff)

	cancel()
	<-errCh
}

func TestDialBadAddr(t *testing.T) {
	key, _ := noise.GenerateKeyPair()
	_, err := Dial(key, "not-a-valid-address", noise.PublicKey{})
	if err == nil {
		t.Fatal("Dial(bad addr) should fail")
	}
}

func TestDialTimeout(t *testing.T) {
	key, _ := noise.GenerateKeyPair()
	serverKey, _ := noise.GenerateKeyPair()
	// Connect to a port that nobody is listening on.
	_, err := Dial(key, "127.0.0.1:19999", serverKey.Public)
	if err == nil {
		t.Fatal("Dial(no server) should fail with timeout")
	}
}

// getServerAddr extracts the actual listen address from a running server.
// This is a test helper that accesses unexported fields via the exported API.
func getServerAddr(t *testing.T, srv *server.Server) string {
	t.Helper()
	// We need to get the address. The server exposes PublicKey() but not the address.
	// For testing, we'll use a workaround: the server test already showed how.
	// Since Server doesn't export listener, we need to add a method or use reflection.
	// For now, let's just add a ListenAddr method to Server.
	return srv.ListenAddr()
}
