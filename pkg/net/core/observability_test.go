package core

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

func TestDispatchToChannelsDropCounters(t *testing.T) {
	t.Run("output queue full increments drop counter", func(t *testing.T) {
		u := &UDP{
			outputChan:  make(chan *packet), // no receiver: always triggers default drop
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
			// Only the decrypt ref remains; release manually to avoid pool leak.
			unrefPacket(routed)
		default:
			t.Fatal("packet was not routed to decryptChan")
		}
	})

	t.Run("decrypt queue full increments drop counter", func(t *testing.T) {
		u := &UDP{
			outputChan:  make(chan *packet, 1),
			decryptChan: make(chan *packet), // no receiver: always triggers default drop
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
			// Only the output ref remains; release manually to avoid pool leak.
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
	_ = smux.Close() // subsequent RPC routing will trigger Input errors

	client.mu.RLock()
	clientPeer := client.peers[serverKey.Public]
	client.mu.RUnlock()
	if clientPeer == nil {
		t.Fatal("client peer not found")
	}

	before := server.HostInfo().RPCRouteErrors
	frame := binary.AppendUvarint(nil, 1)
	frame = append(frame, 0)
	if err := client.sendToPeer(clientPeer, ProtocolRPC, frame); err != nil {
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

	if serverPeer.serviceMux == nil {
		t.Fatal("server service mux not initialized")
	}

	for i := 0; i < InboundChanSize; i++ {
		if err := serverPeer.serviceMux.Input(0, ProtocolEVENT, []byte("seed")); err != nil {
			t.Fatalf("failed to fill service mux inbound queue at %d: %v", i, err)
		}
	}

	before := server.HostInfo().DroppedInboundPackets
	clientMux, err := client.GetServiceMux(serverKey.Public)
	if err != nil {
		t.Fatalf("client.GetServiceMux failed: %v", err)
	}
	if _, err := clientMux.Write(ProtocolEVENT, []byte("drop-me")); err != nil {
		t.Fatalf("client mux Write(EVENT) failed: %v", err)
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

	if _, err := smux.OpenStream(0); err == nil {
		t.Fatal("OpenStream(service=0) should fail without a peer endpoint")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := u.kcpOutputErrors.Load(); got > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("kcpOutputErrors did not increase, got=%d", u.kcpOutputErrors.Load())
}
