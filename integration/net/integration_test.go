package net_test

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/integration/testutil"
	"github.com/giztoy/giztoy-go/pkg/net/core"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

// TestIntegration_ConnectionPoolCapacity verifies the server can handle
// 64 concurrent peer connections simultaneously.
func TestIntegration_ConnectionPoolCapacity(t *testing.T) {
	const peerCount = 64

	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()

	clients := make([]*core.UDP, 0, peerCount)
	clientKeys := make([]*noise.KeyPair, 0, peerCount)
	defer func() {
		for _, c := range clients {
			_ = c.Close()
		}
	}()

	for i := 0; i < peerCount; i++ {
		clientKey, err := noise.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Generate client key[%d] failed: %v", i, err)
		}
		client := testutil.NewUDPNode(t, clientKey)
		clients = append(clients, client)
		clientKeys = append(clientKeys, clientKey)

		testutil.ConnectNodes(t, client, clientKey, server, serverKey)
	}

	if got := server.HostInfo().PeerCount; got < peerCount {
		t.Fatalf("server peer count=%d, want >= %d", got, peerCount)
	}

	for i := range peerCount {
		msg := []byte(fmt.Sprintf("peer-%02d", i))
		n, err := testutil.MustServiceMux(t, clients[i], serverKey.Public).Write(core.ProtocolEVENT, msg)
		if err != nil {
			t.Fatalf("client[%d] Write failed: %v", i, err)
		}
		if n != len(msg) {
			t.Fatalf("client[%d] Write bytes=%d, want %d", i, n, len(msg))
		}

		proto, got := testutil.ReadFromPeerWithTimeout(t, server, clientKeys[i].Public, 3*time.Second)
		if proto != core.ProtocolEVENT {
			t.Fatalf("server received proto=%d from client[%d], want %d", proto, i, core.ProtocolEVENT)
		}
		if !bytes.Equal(got, msg) {
			t.Fatalf("server payload mismatch from client[%d]: got=%q want=%q", i, string(got), string(msg))
		}
	}
}

// TestIntegration_NetworkInterruptionReconnect verifies that a peer with the
// same key can reconnect from a new endpoint and the server updates accordingly.
func TestIntegration_NetworkInterruptionReconnect(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()

	clientV1 := testutil.NewUDPNode(t, clientKey)
	testutil.ConnectNodes(t, clientV1, clientKey, server, serverKey)

	oldEndpoint := clientV1.HostInfo().Addr.String()
	_ = clientV1.Close()

	clientV2 := testutil.NewUDPNode(t, clientKey)
	defer clientV2.Close()
	testutil.ConnectNodes(t, clientV2, clientKey, server, serverKey)

	newEndpoint := clientV2.HostInfo().Addr.String()
	if oldEndpoint == newEndpoint {
		t.Fatalf("expected reconnect with a new local endpoint, got same=%s", newEndpoint)
	}

	msg := []byte("after-reconnect")
	if _, err := testutil.MustServiceMux(t, clientV2, serverKey.Public).Write(core.ProtocolEVENT, msg); err != nil {
		t.Fatalf("clientV2 Write failed: %v", err)
	}

	proto, got := testutil.ReadFromPeerWithTimeout(t, server, clientKey.Public, 3*time.Second)
	if proto != core.ProtocolEVENT {
		t.Fatalf("server proto after reconnect=%d, want %d", proto, core.ProtocolEVENT)
	}
	if !bytes.Equal(got, msg) {
		t.Fatalf("server payload after reconnect mismatch: got=%q want=%q", string(got), string(msg))
	}

	info := server.PeerInfo(clientKey.Public)
	if info == nil || info.Endpoint == nil {
		t.Fatalf("server peer info missing after reconnect")
	}
	if info.Endpoint.String() != newEndpoint {
		t.Fatalf("server endpoint after reconnect=%s, want %s", info.Endpoint.String(), newEndpoint)
	}
}

// TestIntegration_KCPService0Stream verifies that a KCP stream on service=0
// can be established and supports bidirectional communication.
func TestIntegration_KCPService0Stream(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()
	client := testutil.NewUDPNode(t, clientKey)
	defer client.Close()

	testutil.ConnectNodes(t, client, clientKey, server, serverKey)

	acceptCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		stream, err := testutil.MustServiceMux(t, server, clientKey.Public).AcceptStream(0)
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- stream
	}()

	clientStream, err := testutil.MustServiceMux(t, client, serverKey.Public).OpenStream(0)
	if err != nil {
		t.Fatalf("client OpenStream(service=0) failed: %v", err)
	}
	defer clientStream.Close()

	request := []byte("kcp-stream-request")
	if _, err := clientStream.Write(request); err != nil {
		t.Fatalf("client stream write failed: %v", err)
	}

	var serverStream net.Conn
	select {
	case serverStream = <-acceptCh:
		defer serverStream.Close()
	case err := <-errCh:
		t.Fatalf("server AcceptStream failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server AcceptStream timeout")
	}

	if got := testutil.ReadExactWithTimeout(t, serverStream, len(request), 5*time.Second); !bytes.Equal(got, request) {
		t.Fatalf("server stream payload mismatch: got=%q want=%q", string(got), string(request))
	}

	reply := []byte("kcp-stream-reply")
	if _, err := serverStream.Write(reply); err != nil {
		t.Fatalf("server stream write failed: %v", err)
	}
	if got := testutil.ReadExactWithTimeout(t, clientStream, len(reply), 5*time.Second); !bytes.Equal(got, reply) {
		t.Fatalf("client stream payload mismatch: got=%q want=%q", string(got), string(reply))
	}
}

// TestIntegration_KCPStreamActiveClose verifies that after one end closes,
// the other end's blocking stream Read fails promptly.
func TestIntegration_KCPStreamActiveClose(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()
	client := testutil.NewUDPNode(t, clientKey)
	defer client.Close()

	testutil.ConnectNodes(t, client, clientKey, server, serverKey)

	acceptCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		stream, err := testutil.MustServiceMux(t, server, clientKey.Public).AcceptStream(0)
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- stream
	}()

	clientStream, err := testutil.MustServiceMux(t, client, serverKey.Public).OpenStream(0)
	if err != nil {
		t.Fatalf("client OpenStream failed: %v", err)
	}
	defer clientStream.Close()

	if _, err := clientStream.Write([]byte("x")); err != nil {
		t.Fatalf("client stream priming write failed: %v", err)
	}

	var serverStream net.Conn
	select {
	case serverStream = <-acceptCh:
		defer serverStream.Close()
	case err := <-errCh:
		t.Fatalf("server AcceptStream failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server AcceptStream timeout")
	}

	if got := testutil.ReadExactWithTimeout(t, serverStream, 1, 5*time.Second); !bytes.Equal(got, []byte("x")) {
		t.Fatalf("server stream priming payload mismatch: got=%q want=%q", string(got), "x")
	}

	readErrCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 1)
		_, readErr := serverStream.Read(buf)
		readErrCh <- readErr
	}()

	time.Sleep(20 * time.Millisecond)
	start := time.Now()
	if err := client.Close(); err != nil {
		t.Fatalf("client Close failed: %v", err)
	}

	select {
	case readErr := <-readErrCh:
		if readErr == nil {
			t.Fatal("server stream Read should fail after peer close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server stream Read did not fail within 2s after peer close")
	}

	if took := time.Since(start); took >= 380*time.Millisecond {
		t.Fatalf("active close should return before ACK-timeout path: took=%v", took)
	}
}

// TestIntegration_KCPMultiStreamSoak runs a short end-to-end stream churn test
// to catch regressions in open/accept/close under concurrent load.
func TestIntegration_KCPMultiStreamSoak(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()
	client := testutil.NewUDPNode(t, clientKey)
	defer client.Close()

	testutil.ConnectNodes(t, client, clientKey, server, serverKey)

	const rounds = 4
	const perService = 3
	services := []uint64{0, 7}

	for round := 0; round < rounds; round++ {
		var wg sync.WaitGroup
		errCh := make(chan error, len(services)*perService*2)
		recvCh := make(chan string, len(services)*perService)
		expected := make(map[string]struct{}, len(services)*perService)

		for _, serviceID := range services {
			svc := serviceID
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < perService; i++ {
					stream, err := testutil.MustServiceMux(t, server, clientKey.Public).AcceptStream(svc)
					if err != nil {
						errCh <- err
						return
					}
					buf := make([]byte, 64)
					n, err := stream.Read(buf)
					_ = stream.Close()
					if err != nil {
						errCh <- err
						return
					}
					recvCh <- fmt.Sprintf("%d:%s", svc, string(buf[:n]))
				}
			}()
		}

		time.Sleep(20 * time.Millisecond)

		for _, serviceID := range services {
			for i := 0; i < perService; i++ {
				payload := []byte(fmt.Sprintf("round-%d-service-%d-stream-%d", round, serviceID, i))
				expected[fmt.Sprintf("%d:%s", serviceID, string(payload))] = struct{}{}

				wg.Add(1)
				go func(service uint64, msg []byte) {
					defer wg.Done()
					clientStream, err := testutil.MustServiceMux(t, client, serverKey.Public).OpenStream(service)
					if err != nil {
						errCh <- err
						return
					}
					if _, err := clientStream.Write(msg); err != nil {
						errCh <- err
					}
				}(serviceID, payload)
			}
		}

		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Fatal(err)
			}
		}
		close(recvCh)
		for got := range recvCh {
			delete(expected, got)
		}
		if len(expected) != 0 {
			t.Fatalf("missing payloads after round %d: %v", round, expected)
		}
	}
}

// TestIntegration_KCPService0AndNonZeroConcurrentActivity verifies that
// service 0 and a non-zero service can be active concurrently on the same peer
// without cross-talk or blocking each other.
func TestIntegration_KCPService0AndNonZeroConcurrentActivity(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()
	client := testutil.NewUDPNode(t, clientKey)
	defer client.Close()

	testutil.ConnectNodes(t, client, clientKey, server, serverKey)

	errCh := make(chan error, 4)
	done := make(chan string, 2)
	svc0RespRead := make(chan struct{})
	svc7RespRead := make(chan struct{})
	var clientWG sync.WaitGroup

	go func() {
		stream, err := testutil.MustServiceMux(t, server, clientKey.Public).AcceptStream(0)
		if err != nil {
			errCh <- err
			return
		}
		defer stream.Close()

		req := []byte(`{"method":"ping"}`)
		if got := testutil.ReadExactWithTimeout(t, stream, len(req), 5*time.Second); !bytes.Equal(got, req) {
			errCh <- fmt.Errorf("service 0 req mismatch: got=%q want=%q", got, req)
			return
		}
		resp := []byte(`{"ok":true}`)
		if _, err := stream.Write(resp); err != nil {
			errCh <- err
			return
		}
		<-svc0RespRead
		done <- "svc0"
	}()

	go func() {
		stream, err := testutil.MustServiceMux(t, server, clientKey.Public).AcceptStream(7)
		if err != nil {
			errCh <- err
			return
		}
		defer stream.Close()

		req := []byte("nonzero-request")
		if got := testutil.ReadExactWithTimeout(t, stream, len(req), 5*time.Second); !bytes.Equal(got, req) {
			errCh <- fmt.Errorf("service 7 req mismatch: got=%q want=%q", got, req)
			return
		}
		resp := []byte("nonzero-response")
		if _, err := stream.Write(resp); err != nil {
			errCh <- err
			return
		}
		<-svc7RespRead
		done <- "svc7"
	}()

	time.Sleep(20 * time.Millisecond)

	clientWG.Add(1)
	go func() {
		defer clientWG.Done()
		stream, err := testutil.MustServiceMux(t, client, serverKey.Public).OpenStream(0)
		if err != nil {
			errCh <- err
			return
		}
		defer stream.Close()

		req := []byte(`{"method":"ping"}`)
		if _, err := stream.Write(req); err != nil {
			errCh <- err
			return
		}
		resp := []byte(`{"ok":true}`)
		if got := testutil.ReadExactWithTimeout(t, stream, len(resp), 5*time.Second); !bytes.Equal(got, resp) {
			errCh <- fmt.Errorf("service 0 resp mismatch: got=%q want=%q", got, resp)
			return
		}
		close(svc0RespRead)
	}()

	clientWG.Add(1)
	go func() {
		defer clientWG.Done()
		stream, err := testutil.MustServiceMux(t, client, serverKey.Public).OpenStream(7)
		if err != nil {
			errCh <- err
			return
		}
		defer stream.Close()

		req := []byte("nonzero-request")
		if _, err := stream.Write(req); err != nil {
			errCh <- err
			return
		}
		resp := []byte("nonzero-response")
		if got := testutil.ReadExactWithTimeout(t, stream, len(resp), 5*time.Second); !bytes.Equal(got, resp) {
			errCh <- fmt.Errorf("service 7 resp mismatch: got=%q want=%q", got, resp)
			return
		}
		close(svc7RespRead)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case err := <-errCh:
			t.Fatal(err)
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent service activity timeout")
		}
	}

	clientWG.Wait()

	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}
}

// TestIntegration_KCPNonZeroServiceStream tests that non-zero service streams
// can be opened and accepted end-to-end through the foundation layer.
func TestIntegration_KCPNonZeroServiceStream(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()
	client := testutil.NewUDPNode(t, clientKey)
	defer client.Close()

	testutil.ConnectNodes(t, client, clientKey, server, serverKey)

	acceptCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		accepted, err := testutil.MustServiceMux(t, server, clientKey.Public).AcceptStream(7)
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- accepted
	}()

	time.Sleep(50 * time.Millisecond)

	stream, err := testutil.MustServiceMux(t, client, serverKey.Public).OpenStream(7)
	if err != nil {
		t.Fatalf("OpenStream(service=7) err=%v", err)
	}
	defer stream.Close()

	msg := []byte("non-zero-service")
	if _, err := stream.Write(msg); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	var accepted net.Conn
	select {
	case accepted = <-acceptCh:
	case err := <-errCh:
		t.Fatalf("AcceptStream(7) err=%v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptStream(7) timeout")
	}
	defer accepted.Close()

	got := testutil.ReadExactWithTimeout(t, accepted, len(msg), 5*time.Second)
	if !bytes.Equal(got, msg) {
		t.Fatalf("payload mismatch: got=%q want=%q", got, msg)
	}
}

// TestIntegration_RPCBidirectionalOverKCPStream verifies bidirectional RPC
// request/response: one call from A->B and one from B->A both complete.
func TestIntegration_RPCBidirectionalOverKCPStream(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()
	client := testutil.NewUDPNode(t, clientKey)
	defer client.Close()

	testutil.ConnectNodes(t, client, clientKey, server, serverKey)

	assertRPC := func(caller, callee *core.UDP, callerPK, calleePK noise.PublicKey, req, resp []byte) {
		t.Helper()

		acceptCh := make(chan net.Conn, 1)
		errCh := make(chan error, 1)
		go func() {
			stream, err := testutil.MustServiceMux(t, callee, callerPK).AcceptStream(0)
			if err != nil {
				errCh <- err
				return
			}
			acceptCh <- stream
		}()

		callerStream, err := testutil.MustServiceMux(t, caller, calleePK).OpenStream(0)
		if err != nil {
			t.Fatalf("OpenStream failed: %v", err)
		}
		defer callerStream.Close()

		if _, err := callerStream.Write(req); err != nil {
			t.Fatalf("caller write req failed: %v", err)
		}

		var calleeStream net.Conn
		select {
		case calleeStream = <-acceptCh:
			defer calleeStream.Close()
		case err := <-errCh:
			t.Fatalf("AcceptStream failed: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("AcceptStream timeout")
		}

		if got := testutil.ReadExactWithTimeout(t, calleeStream, len(req), 5*time.Second); !bytes.Equal(got, req) {
			t.Fatalf("callee read req mismatch: got=%q want=%q", string(got), string(req))
		}

		if _, err := calleeStream.Write(resp); err != nil {
			t.Fatalf("callee write resp failed: %v", err)
		}
		if got := testutil.ReadExactWithTimeout(t, callerStream, len(resp), 5*time.Second); !bytes.Equal(got, resp) {
			t.Fatalf("caller read resp mismatch: got=%q want=%q", string(got), string(resp))
		}
	}

	assertRPC(client, server, clientKey.Public, serverKey.Public, []byte(`{"method":"ping"}`), []byte(`{"ok":true}`))
	assertRPC(server, client, serverKey.Public, clientKey.Public, []byte(`{"method":"echo"}`), []byte(`{"msg":"ok"}`))
}

// TestIntegration_EVENTFireAndForgetBidirectional verifies bidirectional
// fire-and-forget EVENT delivery: 64 events each way without blocking.
func TestIntegration_EVENTFireAndForgetBidirectional(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()
	client := testutil.NewUDPNode(t, clientKey)
	defer client.Close()

	testutil.ConnectNodes(t, client, clientKey, server, serverKey)

	const burst = 64

	start := time.Now()
	for i := 0; i < burst; i++ {
		msg := []byte(fmt.Sprintf("event-c2s-%02d", i))
		if _, err := testutil.MustServiceMux(t, client, serverKey.Public).Write(core.ProtocolEVENT, msg); err != nil {
			t.Fatalf("client EVENT write[%d] failed: %v", i, err)
		}
	}
	for i := 0; i < burst; i++ {
		msg := []byte(fmt.Sprintf("event-s2c-%02d", i))
		if _, err := testutil.MustServiceMux(t, server, clientKey.Public).Write(core.ProtocolEVENT, msg); err != nil {
			t.Fatalf("server EVENT write[%d] failed: %v", i, err)
		}
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("EVENT fire-and-forget writes took too long: %s", took)
	}

	for i := 0; i < burst; i++ {
		want := []byte(fmt.Sprintf("event-c2s-%02d", i))
		proto, got := testutil.ReadFromPeerWithTimeout(t, server, clientKey.Public, 3*time.Second)
		if proto != core.ProtocolEVENT {
			t.Fatalf("server EVENT proto[%d]=%d, want %d", i, proto, core.ProtocolEVENT)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("server EVENT payload[%d] mismatch: got=%q want=%q", i, string(got), string(want))
		}
	}

	for i := 0; i < burst; i++ {
		want := []byte(fmt.Sprintf("event-s2c-%02d", i))
		proto, got := testutil.ReadFromPeerWithTimeout(t, client, serverKey.Public, 3*time.Second)
		if proto != core.ProtocolEVENT {
			t.Fatalf("client EVENT proto[%d]=%d, want %d", i, proto, core.ProtocolEVENT)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("client EVENT payload[%d] mismatch: got=%q want=%q", i, string(got), string(want))
		}
	}
}

// TestIntegration_OPUSFramesOrdered verifies that 40 consecutive OPUS frames
// are received in order with correct protocol identification.
func TestIntegration_OPUSFramesOrdered(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()
	client := testutil.NewUDPNode(t, clientKey)
	defer client.Close()

	testutil.ConnectNodes(t, client, clientKey, server, serverKey)

	const frames = 40
	for i := 0; i < frames; i++ {
		frame := []byte(fmt.Sprintf("opus-frame-%03d", i))
		if _, err := testutil.MustServiceMux(t, client, serverKey.Public).Write(core.ProtocolOPUS, frame); err != nil {
			t.Fatalf("client OPUS write[%d] failed: %v", i, err)
		}
	}

	for i := 0; i < frames; i++ {
		want := []byte(fmt.Sprintf("opus-frame-%03d", i))
		proto, got := testutil.ReadFromPeerWithTimeout(t, server, clientKey.Public, 3*time.Second)
		if proto != core.ProtocolOPUS {
			t.Fatalf("server OPUS proto[%d]=%d, want %d", i, proto, core.ProtocolOPUS)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("server OPUS payload[%d] mismatch: got=%q want=%q", i, string(got), string(want))
		}
	}
}

// TestIntegration_WriteValidationPrecedesPeerLookup verifies that Write
// rejects RPC datagrams and unsupported protocols before looking up the peer.
func TestIntegration_WriteValidationPrecedesPeerLookup(t *testing.T) {
	localKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate local key failed: %v", err)
	}
	remoteKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate remote key failed: %v", err)
	}

	u := testutil.NewUDPNode(t, localKey)
	defer u.Close()

	smux := core.NewServiceMux(remoteKey.Public, core.ServiceMuxConfig{})
	if _, err := smux.Write(core.ProtocolRPC, []byte("rpc-over-datagram")); err != core.ErrRPCMustUseStream {
		t.Fatalf("Write(RPC datagram) err=%v, want %v", err, core.ErrRPCMustUseStream)
	}

	if _, err := smux.Write(0x7f, []byte("unsupported")); err != core.ErrUnsupportedProtocol {
		t.Fatalf("Write(unsupported protocol) err=%v, want %v", err, core.ErrUnsupportedProtocol)
	}
}

// TestIntegration_UnknownPeerOperations verifies that Write/Read/OpenStream/
// AcceptStream return ErrPeerNotFound for an unknown peer.
func TestIntegration_UnknownPeerOperations(t *testing.T) {
	localKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate local key failed: %v", err)
	}
	unknownKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate unknown key failed: %v", err)
	}

	u := testutil.NewUDPNode(t, localKey)
	defer u.Close()

	if _, err := u.GetServiceMux(unknownKey.Public); err != core.ErrPeerNotFound {
		t.Fatalf("GetServiceMux(unknown peer) err=%v, want %v", err, core.ErrPeerNotFound)
	}

}

// TestIntegration_StreamBeforeSession verifies that stream operations return
// ErrNoSession when the peer is registered but the handshake is not complete.
func TestIntegration_StreamBeforeSession(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()
	client := testutil.NewUDPNode(t, clientKey)
	defer client.Close()

	// Register endpoints only, without Connect (peer exists but no session).
	client.SetPeerEndpoint(serverKey.Public, server.HostInfo().Addr)
	server.SetPeerEndpoint(clientKey.Public, client.HostInfo().Addr)

	if _, err := client.GetServiceMux(serverKey.Public); err != core.ErrNoSession {
		t.Fatalf("OpenStream(before session) err=%v, want %v", err, core.ErrNoSession)
	}

}

// TestIntegration_ClosedNodeOperations verifies that Read/Write/OpenStream/
// AcceptStream consistently return ErrClosed after the node is closed.
func TestIntegration_ClosedNodeOperations(t *testing.T) {
	localKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate local key failed: %v", err)
	}
	peerKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate peer key failed: %v", err)
	}

	u := testutil.NewUDPNode(t, localKey)
	if err := u.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if _, err := u.GetServiceMux(peerKey.Public); err != core.ErrClosed {
		t.Fatalf("GetServiceMux(after close) err=%v, want %v", err, core.ErrClosed)
	}
}

// TestIntegration_ZeroLengthPayloads verifies that EVENT/OPUS support
// zero-length payloads and the receiver correctly identifies the protocol.
func TestIntegration_ZeroLengthPayloads(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()
	client := testutil.NewUDPNode(t, clientKey)
	defer client.Close()

	testutil.ConnectNodes(t, client, clientKey, server, serverKey)

	tests := []struct {
		name  string
		proto byte
	}{
		{name: "EVENT empty payload", proto: core.ProtocolEVENT},
		{name: "OPUS empty payload", proto: core.ProtocolOPUS},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, err := testutil.MustServiceMux(t, client, serverKey.Public).Write(tc.proto, nil)
			if err != nil {
				t.Fatalf("Write(empty payload) failed: %v", err)
			}
			if n != 0 {
				t.Fatalf("Write(empty payload) bytes=%d, want 0", n)
			}

			proto, got := testutil.ReadFromPeerWithTimeout(t, server, clientKey.Public, 3*time.Second)
			if proto != tc.proto {
				t.Fatalf("Read proto=%d, want %d", proto, tc.proto)
			}
			if len(got) != 0 {
				t.Fatalf("Read payload len=%d, want 0", len(got))
			}
		})
	}
}
