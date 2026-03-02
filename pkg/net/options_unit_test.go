package net

import (
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/net/noise"
)

func TestOptionsAndClosedChan(t *testing.T) {
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	cfg := FullSocketConfig()
	cfg.RecvBufSize = 2 * 1024 * 1024
	cfg.SendBufSize = 2 * 1024 * 1024

	u, err := NewUDP(
		key,
		WithBindAddr("127.0.0.1:0"),
		WithAllowUnknown(true),
		WithRawChanSize(17),
		WithSocketConfig(cfg),
	)
	if err != nil {
		t.Fatalf("NewUDP failed: %v", err)
	}

	if cap(u.decryptChan) != 17 {
		t.Fatalf("decryptChan cap=%d, want 17", cap(u.decryptChan))
	}
	if u.socketConfig != cfg {
		t.Fatalf("socketConfig mismatch: got=%+v want=%+v", u.socketConfig, cfg)
	}

	ch := u.closedChan()
	if ch != u.closeChan {
		t.Fatalf("closedChan should return internal closeChan")
	}

	if err := u.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	select {
	case <-ch:
		// expected
	case <-time.After(1 * time.Second):
		t.Fatal("close channel not closed in time")
	}
}

func TestGetServiceMuxAndSendDirectWrapper(t *testing.T) {
	server, client, serverKey, clientKey := createConnectedPair(t)
	defer server.Close()
	defer client.Close()

	if _, err := client.GetServiceMux(noise.PublicKey{}); err != ErrPeerNotFound {
		t.Fatalf("GetServiceMux(non-existent) err=%v, want %v", err, ErrPeerNotFound)
	}

	smux, err := client.GetServiceMux(serverKey.Public)
	if err != nil {
		t.Fatalf("GetServiceMux(existing) failed: %v", err)
	}
	if smux == nil {
		t.Fatal("GetServiceMux(existing) returned nil")
	}

	client.mu.RLock()
	peer := client.peers[serverKey.Public]
	client.mu.RUnlock()
	if peer == nil {
		t.Fatal("peer not found in client map")
	}

	if err := client.sendDirect(peer, noise.ProtocolEVENT, []byte("wrapper-path")); err != nil {
		t.Fatalf("sendDirect wrapper failed: %v", err)
	}

	proto, payload := readPeerWithTimeout(t, server, clientKey.Public, 3*time.Second)
	if proto != noise.ProtocolEVENT {
		t.Fatalf("server got proto=%d, want %d", proto, noise.ProtocolEVENT)
	}
	if string(payload) != "wrapper-path" {
		t.Fatalf("server got payload=%q, want %q", string(payload), "wrapper-path")
	}
}

func readPeerWithTimeout(t *testing.T, u *UDP, pk noise.PublicKey, timeout time.Duration) (byte, []byte) {
	t.Helper()

	type result struct {
		proto   byte
		payload []byte
		err     error
	}

	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, 4096)
		proto, n, err := u.Read(pk, buf)
		if err != nil {
			ch <- result{err: err}
			return
		}
		data := make([]byte, n)
		copy(data, buf[:n])
		ch <- result{proto: proto, payload: data}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("Read failed: %v", r.err)
		}
		return r.proto, r.payload
	case <-time.After(timeout):
		t.Fatalf("Read timeout after %s", timeout)
		return 0, nil
	}
}
