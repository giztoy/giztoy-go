package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/pkg/net/core"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

func TestServerPeerPing(t *testing.T) {
	dir := t.TempDir()
	listenAddr := allocateUDPAddr(t)
	cfg := Config{
		DataDir:    dir,
		ListenAddr: listenAddr,
		ConfigPath: writeTempConfig(t, minimalTestConfig),
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New err=%v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()
	waitForServerRPCReady(t, srv)

	serverAddr := listenAddr
	serverPK := srv.keyPair.Public

	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	clientListener, err := peer.Listen(clientKey,
		core.WithBindAddr("127.0.0.1:0"),
		core.WithAllowUnknown(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	startServerTestReadLoop(clientListener.UDP())
	defer clientListener.Close()

	udpAddr, err := parseUDPAddr(serverAddr)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := clientListener.Dial(serverPK, udpAddr)
	if err != nil {
		t.Fatal(err)
	}

	stream, err := conn.OpenService(peer.ServicePublic)
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
	listenAddr := allocateUDPAddr(t)
	cfg := Config{
		DataDir:    dir,
		ListenAddr: listenAddr,
		ConfigPath: writeTempConfig(t, minimalTestConfig),
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()
	waitForServerRPCReady(t, srv)

	serverAddr := listenAddr
	serverPK := srv.keyPair.Public

	clientKey, _ := noise.GenerateKeyPair()
	clientListener, err := peer.Listen(clientKey,
		core.WithBindAddr("127.0.0.1:0"),
		core.WithAllowUnknown(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	startServerTestReadLoop(clientListener.UDP())
	defer clientListener.Close()

	udpAddr, _ := parseUDPAddr(serverAddr)
	conn, err := clientListener.Dial(serverPK, udpAddr)
	if err != nil {
		t.Fatal(err)
	}

	stream, err := conn.OpenService(peer.ServicePublic)
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

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ListenAddr != ":9820" {
		t.Fatalf("ListenAddr=%q", cfg.ListenAddr)
	}
	if cfg.DataDir == "" {
		t.Fatal("DataDir is empty")
	}
}

func TestServerListenAddrBeforeRun(t *testing.T) {
	srv, err := New(Config{
		DataDir:    t.TempDir(),
		ListenAddr: "127.0.0.1:0",
		ConfigPath: writeTempConfig(t, minimalTestConfig),
	})
	if err != nil {
		t.Fatalf("New err=%v", err)
	}
	t.Cleanup(func() { srv.stores.Close() })
	if got := srv.ListenAddr(); got != "" {
		t.Fatalf("ListenAddr before Run=%q, want empty", got)
	}
}

func TestServerRunListenError(t *testing.T) {
	srv, err := New(Config{
		DataDir:    t.TempDir(),
		ListenAddr: "bad-listen-addr",
		ConfigPath: writeTempConfig(t, minimalTestConfig),
	})
	if err != nil {
		t.Fatalf("New err=%v", err)
	}
	t.Cleanup(func() { srv.stores.Close() })

	if err := srv.Run(context.Background()); err == nil {
		t.Fatal("Run with bad listen addr should fail")
	}
}

func TestHandleStreamReadError(t *testing.T) {
	srv := &Server{logger: log.New(io.Discard, "", 0)}
	serverSide, clientSide := net.Pipe()
	done := make(chan struct{})
	go func() {
		srv.handleStream(serverSide)
		close(done)
	}()

	_ = clientSide.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleStream did not return after read failure")
	}
}

func TestHandlePeerPingWriteError(t *testing.T) {
	srv := &Server{logger: log.New(io.Discard, "", 0)}
	srv.handlePeerPing(errConn{writeErr: io.ErrClosedPipe}, &RPCRequest{ID: "ping"})
}

func TestMarkPeerOfflineDeletesActivePeer(t *testing.T) {
	srv := &Server{activePeers: make(map[string]*activePeer)}
	conn := &peer.Conn{}
	srv.markPeerOnline("device-pk", conn)
	if runtime := srv.peerRuntime("device-pk"); !runtime.Online {
		t.Fatalf("peer should be online before offline mark: %+v", runtime)
	}
	srv.markPeerOffline("device-pk", conn)
	if _, ok := srv.activePeers["device-pk"]; ok {
		t.Fatal("peer should be removed after disconnect")
	}
	if runtime := srv.peerRuntime("device-pk"); runtime.Online || runtime.LastSeenAt != 0 {
		t.Fatalf("runtime after removal = %+v", runtime)
	}
}

func TestMarkPeerOfflineKeepsNewerConnection(t *testing.T) {
	srv := &Server{activePeers: make(map[string]*activePeer)}
	oldConn := &peer.Conn{}
	newConn := &peer.Conn{}
	srv.markPeerOnline("device-pk", oldConn)
	srv.markPeerOnline("device-pk", newConn)
	srv.markPeerOffline("device-pk", oldConn)

	got, ok := srv.activePeer("device-pk")
	if !ok || got != newConn {
		t.Fatalf("activePeer after old disconnect = %v, %v", got, ok)
	}
	if runtime := srv.peerRuntime("device-pk"); !runtime.Online {
		t.Fatalf("runtime after old disconnect = %+v", runtime)
	}
}

func TestWriteRPCResponseMarshalError(t *testing.T) {
	resp := &RPCResponse{
		V:      1,
		ID:     "bad",
		Result: json.RawMessage("{bad-result"),
	}
	if err := WriteRPCResponse(io.Discard, resp); err == nil {
		t.Fatal("WriteRPCResponse(marshal error) should fail")
	}
}

func allocateUDPAddr(t *testing.T) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocateUDPAddr: %v", err)
	}
	addr := pc.LocalAddr().(*net.UDPAddr)
	pc.Close()
	return fmt.Sprintf("127.0.0.1:%d", addr.Port)
}

func parseUDPAddr(addr string) (*net.UDPAddr, error) {
	return net.ResolveUDPAddr("udp", addr)
}

func waitForServerRPCReady(t *testing.T, srv *Server) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if srv.listener == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		clientKey, err := noise.GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair(ready check): %v", err)
		}
		clientListener, err := peer.Listen(clientKey,
			core.WithBindAddr("127.0.0.1:0"),
			core.WithAllowUnknown(true),
		)
		if err != nil {
			t.Fatalf("peer.Listen(ready check): %v", err)
		}
		startServerTestReadLoop(clientListener.UDP())

		ready := false
		func() {
			defer clientListener.Close()

			udpAddr, err := parseUDPAddr(srv.listener.HostInfo().Addr.String())
			if err != nil {
				t.Fatalf("parseUDPAddr(ready check): %v", err)
			}
			conn, err := clientListener.Dial(srv.keyPair.Public, udpAddr)
			if err != nil {
				return
			}
			stream, err := conn.OpenService(peer.ServicePublic)
			if err != nil {
				return
			}
			defer stream.Close()
			_ = stream.SetDeadline(time.Now().Add(200 * time.Millisecond))

			req := RPCRequest{V: 1, ID: "ready-check", Method: "peer.ping"}
			reqData, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("json.Marshal(ready check): %v", err)
			}
			if err := WriteFrame(stream, reqData); err != nil {
				return
			}
			if _, err := ReadFrame(stream); err == nil {
				ready = true
			}
		}()

		if ready {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("server rpc did not become ready")
}

func startServerTestReadLoop(u *core.UDP) {
	go func() {
		buf := make([]byte, 65535)
		for {
			if _, _, err := u.ReadFrom(buf); err != nil {
				return
			}
		}
	}()
}

type errConn struct {
	writeErr error
}

func (c errConn) Read(_ []byte) (int, error)         { return 0, io.EOF }
func (c errConn) Write(_ []byte) (int, error)        { return 0, c.writeErr }
func (c errConn) Close() error                       { return nil }
func (c errConn) LocalAddr() net.Addr                { return dummyAddr("local") }
func (c errConn) RemoteAddr() net.Addr               { return dummyAddr("remote") }
func (c errConn) SetDeadline(_ time.Time) error      { return nil }
func (c errConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c errConn) SetWriteDeadline(_ time.Time) error { return nil }

type dummyAddr string

func (a dummyAddr) Network() string { return string(a) }
func (a dummyAddr) String() string  { return string(a) }
