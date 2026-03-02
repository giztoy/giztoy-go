package net

import (
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/net/noise"
)

func TestDispatchToChannelsDropCounters(t *testing.T) {
	t.Run("output queue full increments drop counter", func(t *testing.T) {
		u := &UDP{
			outputChan:  make(chan *packet), // 无接收方：始终触发 default 丢弃
			decryptChan: make(chan *packet, 1),
			closeChan:   make(chan struct{}),
		}

		pkt := acquirePacket()
		u.dispatchToChannels(pkt)

		if got := u.droppedOutputPackets.Load(); got != 1 {
			t.Fatalf("droppedOutputPackets=%d, want 1", got)
		}
		if got := u.droppedDecryptPackets.Load(); got != 0 {
			t.Fatalf("droppedDecryptPackets=%d, want 0", got)
		}

		select {
		case routed := <-u.decryptChan:
			if routed != pkt {
				t.Fatal("decrypt queue packet mismatch")
			}
			// 仅 decrypt 引用仍持有，手动释放避免池泄漏。
			unrefPacket(routed)
		default:
			t.Fatal("packet was not routed to decryptChan")
		}
	})

	t.Run("decrypt queue full increments drop counter", func(t *testing.T) {
		u := &UDP{
			outputChan:  make(chan *packet, 1),
			decryptChan: make(chan *packet), // 无接收方：始终触发 default 丢弃
			closeChan:   make(chan struct{}),
		}

		pkt := acquirePacket()
		u.dispatchToChannels(pkt)

		if got := u.droppedDecryptPackets.Load(); got != 1 {
			t.Fatalf("droppedDecryptPackets=%d, want 1", got)
		}
		if got := u.droppedOutputPackets.Load(); got != 0 {
			t.Fatalf("droppedOutputPackets=%d, want 0", got)
		}

		select {
		case routed := <-u.outputChan:
			select {
			case <-routed.ready:
			default:
				t.Fatal("routed packet should be marked ready when decrypt path drops")
			}
			if routed.err != ErrNoData {
				t.Fatalf("routed packet err=%v, want %v", routed.err, ErrNoData)
			}
			// 仅 output 引用仍持有，手动释放避免池泄漏。
			unrefPacket(routed)
		default:
			t.Fatal("packet was not queued to outputChan")
		}
	})
}

func TestRPCRouteErrorCounterOnSmuxInputFailure(t *testing.T) {
	server, client, serverKey, clientKey := createConnectedPair(t)
	defer server.Close()
	defer client.Close()

	server.mu.RLock()
	serverPeer := server.peers[clientKey.Public]
	server.mu.RUnlock()
	if serverPeer == nil {
		t.Fatal("server peer not found")
	}

	serverPeer.mu.RLock()
	smux := serverPeer.serviceMux
	serverPeer.mu.RUnlock()
	if smux == nil {
		t.Fatal("server service mux not initialized")
	}
	_ = smux.Close() // 后续 RPC 路由将触发 Input 错误

	client.mu.RLock()
	clientPeer := client.peers[serverKey.Public]
	client.mu.RUnlock()
	if clientPeer == nil {
		t.Fatal("client peer not found")
	}

	before := server.HostInfo().RPCRouteErrors
	if err := client.sendToPeer(clientPeer, noise.ProtocolRPC, []byte("rpc-route-err")); err != nil {
		t.Fatalf("sendToPeer(RPC) failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := server.HostInfo().RPCRouteErrors; got > before {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("RPCRouteErrors did not increase: before=%d after=%d", before, server.HostInfo().RPCRouteErrors)
}

func TestInboundDropCounterWhenPeerQueueFull(t *testing.T) {
	server, client, serverKey, clientKey := createConnectedPair(t)
	defer server.Close()
	defer client.Close()

	server.mu.RLock()
	serverPeer := server.peers[clientKey.Public]
	server.mu.RUnlock()
	if serverPeer == nil {
		t.Fatal("server peer not found")
	}

	serverPeer.mu.Lock()
	serverPeer.inboundChan = make(chan protoPacket, 1)
	serverPeer.inboundChan <- protoPacket{protocol: noise.ProtocolEVENT, payload: []byte("seed")}
	serverPeer.mu.Unlock()

	before := server.HostInfo().DroppedInboundPackets
	if _, err := client.Write(serverKey.Public, noise.ProtocolEVENT, []byte("drop-me")); err != nil {
		t.Fatalf("client.Write(EVENT) failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := server.HostInfo().DroppedInboundPackets; got > before {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("DroppedInboundPackets did not increase: before=%d after=%d", before, server.HostInfo().DroppedInboundPackets)
}

func TestKCPOutputErrorCounterFromServiceMux(t *testing.T) {
	localKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(local) failed: %v", err)
	}
	remoteKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(remote) failed: %v", err)
	}

	u := &UDP{localKey: localKey}
	peer := &peerState{pk: remoteKey.Public}
	smux := u.createServiceMux(peer)
	defer smux.Close()

	stream, err := smux.OpenStream(0)
	if err != nil {
		t.Fatalf("OpenStream(service=0) failed: %v", err)
	}
	defer stream.Close()

	_ = stream.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
	_, _ = stream.Write([]byte("trigger-output-error"))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := u.kcpOutputErrors.Load(); got > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("kcpOutputErrors did not increase, got=%d", u.kcpOutputErrors.Load())
}
