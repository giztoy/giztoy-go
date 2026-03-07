package server

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/net/core"
	"github.com/haivivi/giztoy/go/pkg/net/noise"
	"github.com/haivivi/giztoy/go/pkg/net/peer"
)

func TestServerPeerPing(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{DataDir: dir, ListenAddr: "127.0.0.1:0"}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New err=%v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()

	// Wait for server to start listening.
	time.Sleep(200 * time.Millisecond)

	info := srv.listener.HostInfo()
	serverAddr := info.Addr.String()
	serverPK := srv.keyPair.Public

	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	clientUDP, err := core.NewUDP(clientKey,
		core.WithBindAddr("127.0.0.1:0"),
		core.WithAllowUnknown(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer clientUDP.Close()

	udpAddr, err := parseUDPAddr(serverAddr)
	if err != nil {
		t.Fatal(err)
	}
	clientUDP.SetPeerEndpoint(serverPK, udpAddr)
	clientUDP.Connect(serverPK)

	waitForHandshake(t, clientUDP, serverPK, 3*time.Second)

	clientListener, err := peer.Wrap(clientUDP)
	if err != nil {
		t.Fatal(err)
	}
	defer clientListener.Close()

	conn, err := clientListener.Peer(serverPK)
	if err != nil {
		t.Fatalf("Peer err=%v", err)
	}

	stream, err := conn.OpenService(0)
	if err != nil {
		t.Fatalf("OpenService err=%v", err)
	}
	defer stream.Close()

	req := RPCRequest{V: 1, ID: "test-ping", Method: "peer.ping"}
	reqData, _ := json.Marshal(req)
	if err := WriteFrame(stream, reqData); err != nil {
		t.Fatalf("WriteFrame err=%v", err)
	}

	respData, err := ReadFrame(stream)
	if err != nil {
		t.Fatalf("ReadFrame err=%v", err)
	}

	var resp RPCResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		t.Fatalf("unmarshal resp: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("resp error: %+v", resp.Error)
	}
	if resp.ID != "test-ping" {
		t.Fatalf("resp ID=%q", resp.ID)
	}

	var result PingResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.ServerTime == 0 {
		t.Fatal("ServerTime is zero")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Logf("server shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down")
	}
}

func TestServerUnknownMethod(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{DataDir: dir, ListenAddr: "127.0.0.1:0"}

	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.Run(ctx)
	time.Sleep(200 * time.Millisecond)

	info := srv.listener.HostInfo()
	serverAddr := info.Addr.String()
	serverPK := srv.keyPair.Public

	clientKey, _ := noise.GenerateKeyPair()
	clientUDP, err := core.NewUDP(clientKey,
		core.WithBindAddr("127.0.0.1:0"),
		core.WithAllowUnknown(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer clientUDP.Close()

	udpAddr, _ := parseUDPAddr(serverAddr)
	clientUDP.SetPeerEndpoint(serverPK, udpAddr)
	clientUDP.Connect(serverPK)
	waitForHandshake(t, clientUDP, serverPK, 3*time.Second)

	clientListener, _ := peer.Wrap(clientUDP)
	defer clientListener.Close()

	conn, _ := clientListener.Peer(serverPK)
	stream, err := conn.OpenService(0)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	req := RPCRequest{V: 1, ID: "bad", Method: "nonexistent"}
	reqData, _ := json.Marshal(req)
	WriteFrame(stream, reqData)

	respData, err := ReadFrame(stream)
	if err != nil {
		t.Fatalf("ReadFrame err=%v", err)
	}
	var resp RPCResponse
	json.Unmarshal(respData, &resp)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ListenAddr != ":9820" {
		t.Fatalf("ListenAddr=%q", cfg.ListenAddr)
	}
	if cfg.DataDir == "" {
		t.Fatal("DataDir is empty")
	}
}

func parseUDPAddr(addr string) (*net.UDPAddr, error) {
	return net.ResolveUDPAddr("udp", addr)
}

func waitForHandshake(t *testing.T, u *core.UDP, pk noise.PublicKey, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		info := u.PeerInfo(pk)
		if info != nil && info.State == core.PeerStateEstablished {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("handshake with %s did not complete within %v", pk.ShortString(), timeout)
}
