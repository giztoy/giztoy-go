package gizhttp

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/giznet"
)

func TestRoundTrip(t *testing.T) {
	serverKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	serverListener := newListenerNode(t, serverKey, giznet.WithServiceMuxConfig(giznet.ServiceMuxConfig{
		OnNewService: func(_ giznet.PublicKey, service uint64) bool {
			return service == 7
		},
	}))
	defer serverListener.Close()
	clientListener := newListenerNode(t, clientKey)
	defer clientListener.Close()
	connectListenerNodes(t, clientListener, clientKey, serverListener, serverKey)

	clientConn, err := clientListener.Peer(serverKey.Public)
	if err != nil {
		t.Fatal(err)
	}
	serverConn, err := serverListener.Peer(clientKey.Public)
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(serverConn, 7, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		w.Header().Set("X-Test", "ok")
		_, _ = w.Write([]byte("echo:" + string(payload)))
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	go func() {
		_ = srv.Serve()
	}()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://gizclaw/echo", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := NewClient(clientConn, 7).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "echo:hello" {
		t.Fatalf("body = %q", body)
	}
	if resp.Header.Get("X-Test") != "ok" {
		t.Fatalf("X-Test header = %q", resp.Header.Get("X-Test"))
	}
}

func TestListenerCloseUnblocksAccept(t *testing.T) {
	serverKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	serverListener := newListenerNode(t, serverKey)
	defer serverListener.Close()
	clientListener := newListenerNode(t, clientKey)
	defer clientListener.Close()
	connectListenerNodes(t, clientListener, clientKey, serverListener, serverKey)

	serverConn, err := serverListener.Peer(clientKey.Public)
	if err != nil {
		t.Fatal(err)
	}

	l := NewListener(serverConn, 9)
	if l.Addr().Network() != "kcp-http" {
		t.Fatalf("Addr().Network() = %q", l.Addr().Network())
	}
	done := make(chan error, 1)
	go func() {
		_, err := l.Accept()
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	if err := l.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	select {
	case err := <-done:
		if !IsClosed(err) && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("Accept err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Accept did not unblock after Close")
	}
}

func TestPeerCloseUnblocksAccept(t *testing.T) {
	serverKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	serverListener := newListenerNode(t, serverKey)
	defer serverListener.Close()
	clientListener := newListenerNode(t, clientKey)
	defer clientListener.Close()
	connectListenerNodes(t, clientListener, clientKey, serverListener, serverKey)

	serverConn, err := serverListener.Peer(clientKey.Public)
	if err != nil {
		t.Fatal(err)
	}
	defer serverConn.Close()

	l := NewListener(serverConn, 11)
	done := make(chan error, 1)
	go func() {
		_, err := l.Accept()
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)

	if err := clientListener.Close(); err != nil {
		t.Fatalf("client listener close error: %v", err)
	}

	select {
	case err := <-done:
		if !IsClosed(err) && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("Accept err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Accept did not unblock after peer close")
	}

	if _, err := l.Accept(); !IsClosed(err) && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Accept after peer close err = %v", err)
	}
}
