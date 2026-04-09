package client

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/internal/server"
	"github.com/giztoy/giztoy-go/internal/stores"
	itest "github.com/giztoy/giztoy-go/internal/testutil"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/net/core"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

func TestDialAndPing(t *testing.T) {
	dir := t.TempDir()
	listenAddr := itest.AllocateUDPAddr(t)
	cfg := server.Config{
		DataDir:    dir,
		ListenAddr: listenAddr,
		Stores: map[string]stores.Config{
			"mem": {Kind: stores.KindKeyValue, Backend: "memory"},
			"fw":  {Kind: stores.KindFS, Backend: "filesystem", Dir: "firmware"},
		},
		Gears: server.GearsConfig{
			Store: "mem",
			RegistrationTokens: map[string]gears.RegistrationToken{
				"device_default": {Role: gears.GearRoleDevice},
			},
		},
		Depots: server.DepotsConfig{Store: "fw"},
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	testServerAddrs.Store(srv, listenAddr)
	t.Cleanup(func() {
		testServerAddrs.Delete(srv)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()
	testServerRunErrs.Store(srv, errCh)
	t.Cleanup(func() {
		testServerRunErrs.Delete(srv)
	})
	waitForTestServerReady(t, srv)

	serverPK := srv.PublicKey()
	serverAddr := listenAddr

	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	c, err := Dial(clientKey, serverAddr, serverPK)
	if err != nil {
		t.Fatalf("Dial err=%v", err)
	}
	waitForClientPublicReady(t, c)
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

func startMockRPCServer(t *testing.T, handler func(net.Conn)) (string, noise.PublicKey, <-chan struct{}, func()) {
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

	accepted := make(chan struct{})
	go func() {
		conn, err := serverListener.Accept()
		if err != nil {
			return
		}
		close(accepted)
		stream, err := conn.AcceptService(0)
		if err != nil {
			return
		}
		handler(stream)
	}()

	closeFn := func() {
		_ = serverListener.Close()
	}
	return serverListener.HostInfo().Addr.String(), serverKey.Public, accepted, closeFn
}

func dialMockClient(t *testing.T, handler func(net.Conn)) *Client {
	t.Helper()

	addr, serverPK, accepted, closeServer := startMockRPCServer(t, handler)
	t.Cleanup(closeServer)

	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(client): %v", err)
	}

	c, err := Dial(clientKey, addr, serverPK)
	if err != nil {
		t.Fatalf("Dial err=%v", err)
	}
	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("mock server did not accept peer")
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
	serverErr := make(chan error, 1)
	release := make(chan struct{})
	defer close(release)

	c := dialMockClient(t, func(stream net.Conn) {
		defer stream.Close()
		_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err := server.ReadFrame(stream); err != nil {
			serverErr <- err
			return
		}
		resp := &server.RPCResponse{
			V:  1,
			ID: "ping",
			Error: &server.RPCError{
				Code:    -1,
				Message: "boom",
			},
		}
		if err := server.WriteRPCResponse(stream, resp); err != nil {
			serverErr <- err
			return
		}
		<-release
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := c.Ping()
		errCh <- err
	}()

	select {
	case err := <-serverErr:
		t.Fatalf("mock server error: %v", err)
	case err := <-errCh:
		if err == nil || err.Error() != "client: server error: boom" {
			t.Fatalf("Ping server error=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Ping server error did not return")
	}
}

func TestPingBadResponseJSON(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	c := dialMockClient(t, func(stream net.Conn) {
		defer stream.Close()
		_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err := server.ReadFrame(stream); err != nil {
			return
		}
		_ = server.WriteFrame(stream, []byte("{bad-json"))
		<-release
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := c.Ping()
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err == nil || !contains(err, "client: unmarshal:") {
			t.Fatalf("Ping bad response err=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Ping bad response did not return")
	}
}

func TestPingBadResultJSON(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	c := dialMockClient(t, func(stream net.Conn) {
		defer stream.Close()
		_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err := server.ReadFrame(stream); err != nil {
			return
		}
		_ = server.WriteFrame(stream, []byte(`{"v":1,"id":"ping","result":{bad-result}`))
		<-release
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := c.Ping()
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err == nil || !contains(err, "client: unmarshal:") {
			t.Fatalf("Ping bad result err=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Ping bad result did not return")
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
