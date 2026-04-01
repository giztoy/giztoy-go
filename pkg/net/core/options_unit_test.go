package core

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/pkg/net/noise"
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
		WithServiceMuxConfig(ServiceMuxConfig{
			OnNewService: func(peer noise.PublicKey, service uint64) bool {
				return service == 1
			},
		}),
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
	if u.serviceMuxConfig.OnNewService == nil {
		t.Fatal("serviceMuxConfig should be injected by WithServiceMuxConfig")
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

func TestWithServiceMuxConfigOption(t *testing.T) {
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	wantCfg := ServiceMuxConfig{
		OnNewService: func(peer noise.PublicKey, service uint64) bool {
			return service == 9
		},
	}

	u, err := NewUDP(
		key,
		WithBindAddr("127.0.0.1:0"),
		WithServiceMuxConfig(wantCfg),
	)
	if err != nil {
		t.Fatalf("NewUDP failed: %v", err)
	}
	defer u.Close()

	if u.serviceMuxConfig.OnNewService == nil {
		t.Fatal("serviceMuxConfig.OnNewService is nil")
	}
	if u.serviceMuxConfig.OnNewService(noise.PublicKey{}, 9) != wantCfg.OnNewService(noise.PublicKey{}, 9) {
		t.Fatal("serviceMuxConfig.OnNewService not applied")
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

	if err := client.sendDirect(peer, ProtocolEVENT, []byte("wrapper-path")); err != nil {
		t.Fatalf("sendDirect wrapper failed: %v", err)
	}

	proto, payload := readPeerWithTimeout(t, server, clientKey.Public, 3*time.Second)
	if proto != ProtocolEVENT {
		t.Fatalf("server got proto=%d, want %d", proto, ProtocolEVENT)
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
		smux, err := u.GetServiceMux(pk)
		if err != nil {
			ch <- result{err: err}
			return
		}
		buf := make([]byte, 4096)
		proto, n, err := smux.Read(buf)
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

func TestCreateServiceMux_UsesInjectedPeerAwareOnNewService(t *testing.T) {
	localKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(local) failed: %v", err)
	}
	remoteKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(remote) failed: %v", err)
	}

	var gotPeer noise.PublicKey
	var gotService uint64
	u := &UDP{
		localKey: localKey,
		serviceMuxConfig: ServiceMuxConfig{
			OnNewService: func(peer noise.PublicKey, service uint64) bool {
				gotPeer = peer
				gotService = service
				return peer == remoteKey.Public && service == 3
			},
		},
	}
	peer := &peerState{pk: remoteKey.Public}
	smux := u.createServiceMux(peer)
	defer smux.Close()

	streamID := uint64(0)
	if !u.isKCPClient(remoteKey.Public) {
		streamID = 1
	}
	frame := binary.AppendUvarint(nil, streamID)
	frame = append(frame, 0)

	if err := smux.Input(3, ProtocolRPC, frame); err != nil {
		t.Fatalf("Input(service=3) failed: %v", err)
	}
	if gotPeer != remoteKey.Public {
		t.Fatalf("OnNewService peer=%v, want %v", gotPeer, remoteKey.Public)
	}
	if gotService != 3 {
		t.Fatalf("OnNewService service=%d, want 3", gotService)
	}
	if smux.NumServices() != 1 {
		t.Fatalf("NumServices=%d, want 1", smux.NumServices())
	}
}
