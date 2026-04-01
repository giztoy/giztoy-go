package client

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/internal/server"
	"github.com/giztoy/giztoy-go/pkg/net/core"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
	"github.com/giztoy/giztoy-go/pkg/net/peer"
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

func startReadLoop(u *core.UDP) {
	go func() {
		buf := make([]byte, 65535)
		for {
			if _, _, err := u.ReadFrom(buf); err != nil {
				return
			}
		}
	}()
}

func startMockRPCServer(t *testing.T, handler func(net.Conn)) (string, noise.PublicKey, func()) {
	t.Helper()

	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(server): %v", err)
	}

	serverListener, err := peer.Listen(serverKey, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("Listen(server): %v", err)
	}
	startReadLoop(serverListener.UDP())

	go func() {
		conn, err := serverListener.Accept()
		if err != nil {
			return
		}
		stream, err := conn.AcceptService(0)
		if err != nil {
			return
		}
		handler(stream)
	}()

	closeFn := func() {
		_ = serverListener.Close()
	}
	return serverListener.HostInfo().Addr.String(), serverKey.Public, closeFn
}

func dialMockClient(t *testing.T, handler func(net.Conn)) *Client {
	t.Helper()

	addr, serverPK, closeServer := startMockRPCServer(t, handler)
	t.Cleanup(closeServer)

	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(client): %v", err)
	}

	c, err := Dial(clientKey, addr, serverPK)
	if err != nil {
		t.Fatalf("Dial err=%v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestPingAfterClose(t *testing.T) {
	c := dialMockClient(t, func(stream net.Conn) {
		defer stream.Close()
		_, _ = server.ReadFrame(stream)
	})

	if err := c.Close(); err != nil {
		t.Fatalf("Close err=%v", err)
	}

	if _, err := c.Ping(); err == nil {
		t.Fatal("Ping after Close should fail")
	}
}

func TestPingServerError(t *testing.T) {
	c := dialMockClient(t, func(stream net.Conn) {
		_, _ = server.ReadFrame(stream)
		resp := &server.RPCResponse{
			V:  1,
			ID: "ping",
			Error: &server.RPCError{
				Code:    -1,
				Message: "boom",
			},
		}
		_ = server.WriteRPCResponse(stream, resp)
	})

	if _, err := c.Ping(); err == nil || err.Error() != "client: server error: boom" {
		t.Fatalf("Ping server error=%v", err)
	}
}

func TestPingBadResponseJSON(t *testing.T) {
	c := dialMockClient(t, func(stream net.Conn) {
		_, _ = server.ReadFrame(stream)
		_ = server.WriteFrame(stream, []byte("{bad-json"))
	})

	if _, err := c.Ping(); err == nil || !contains(err, "client: unmarshal:") {
		t.Fatalf("Ping bad response err=%v", err)
	}
}

func TestPingBadResultJSON(t *testing.T) {
	c := dialMockClient(t, func(stream net.Conn) {
		_, _ = server.ReadFrame(stream)
		_ = server.WriteFrame(stream, []byte(`{"v":1,"id":"ping","result":{bad-result}`))
	})

	if _, err := c.Ping(); err == nil || !contains(err, "client: unmarshal:") {
		t.Fatalf("Ping bad result err=%v", err)
	}
}

func TestPingReadError(t *testing.T) {
	c := dialMockClient(t, func(stream net.Conn) {
		_, _ = server.ReadFrame(stream)
		_ = stream.Close()
	})

	if _, err := c.Ping(); err == nil || !contains(err, "client: read:") {
		t.Fatalf("Ping read error=%v", err)
	}
}

func TestClientCloseNilSafe(t *testing.T) {
	var c Client
	if err := c.Close(); err != nil {
		t.Fatalf("Close nil-safe err=%v", err)
	}
}

func contains(err error, want string) bool {
	return err != nil && strings.Contains(err.Error(), want)
}
