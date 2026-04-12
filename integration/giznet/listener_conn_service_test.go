package giznet_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/pkg/audio/stampedopus"
	"github.com/giztoy/giztoy-go/pkg/giznet"
)

func TestNilListenerGuard(t *testing.T) {
	var l *giznet.Listener

	if _, err := l.Accept(); !errors.Is(err, giznet.ErrNilListener) {
		t.Fatalf("Accept(nil listener) err=%v, want %v", err, giznet.ErrNilListener)
	}

	if _, err := l.Peer(giznet.PublicKey{}); !errors.Is(err, giznet.ErrNilListener) {
		t.Fatalf("Peer(nil listener) err=%v, want %v", err, giznet.ErrNilListener)
	}

	if err := l.Close(); !errors.Is(err, giznet.ErrNilListener) {
		t.Fatalf("Close(nil listener) err=%v, want %v", err, giznet.ErrNilListener)
	}
}

func TestListenAndCloseOwnedListener(t *testing.T) {
	key, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	l, err := giznet.Listen(key, giznet.WithBindAddr("127.0.0.1:0"), giznet.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	if err := l.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if _, err := l.Accept(); !errors.Is(err, giznet.ErrClosed) {
		t.Fatalf("Accept after Close err=%v, want %v", err, giznet.ErrClosed)
	}
}

func TestListenerPeerErrors(t *testing.T) {
	key, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	l, err := giznet.Listen(key, giznet.WithBindAddr("127.0.0.1:0"), giznet.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	unknown, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate unknown key failed: %v", err)
	}

	if _, err := l.Peer(unknown.Public); !errors.Is(err, giznet.ErrPeerNotFound) {
		t.Fatalf("Peer(unknown) err=%v, want %v", err, giznet.ErrPeerNotFound)
	}

	l.SetPeerEndpoint(unknown.Public, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 54321})
	if _, err := l.Peer(unknown.Public); !errors.Is(err, giznet.ErrNoSession) {
		t.Fatalf("Peer(no session) err=%v, want %v", err, giznet.ErrNoSession)
	}
}

func TestListenerDoesNotAcceptSamePeerAgainOnReconnect(t *testing.T) {
	pair := NewConnectedPeerPair(t)
	defer pair.Close()

	acceptCh := make(chan *giznet.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := pair.ServerListener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- conn
	}()

	if err := pair.ClientListener.Connect(pair.ServerKey.Public); err != nil {
		t.Fatalf("Reconnect failed: %v", err)
	}

	select {
	case conn := <-acceptCh:
		t.Fatalf("Listener.Accept unexpectedly returned duplicate peer %v after reconnect", conn.PublicKey())
	case err := <-errCh:
		t.Fatalf("Listener.Accept failed during reconnect: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestPeerMultipleConcurrentConnections(t *testing.T) {
	serverKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}

	serverListener := NewTestListener(t, serverKey)
	defer serverListener.Close()

	const peers = 3
	type clientNode struct {
		key      *giznet.KeyPair
		listener *giznet.Listener
	}
	clients := make([]clientNode, 0, peers)
	for i := 0; i < peers; i++ {
		k, err := giznet.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Generate client key %d failed: %v", i, err)
		}
		cl := NewTestListener(t, k)
		clients = append(clients, clientNode{key: k, listener: cl})
	}
	defer func() {
		for _, c := range clients {
			_ = c.listener.Close()
		}
	}()

	var connectWG sync.WaitGroup
	for _, c := range clients {
		connectWG.Add(1)
		client := c
		go func() {
			defer connectWG.Done()
			client.listener.SetPeerEndpoint(serverKey.Public, serverListener.HostInfo().Addr)
			serverListener.SetPeerEndpoint(client.key.Public, client.listener.HostInfo().Addr)
			if err := client.listener.Connect(serverKey.Public); err != nil {
				t.Errorf("Connect failed: %v", err)
			}
		}()
	}
	connectWG.Wait()

	accepted := make(map[giznet.PublicKey]struct{})
	for i := 0; i < peers; i++ {
		conn, err := AcceptConnWithTimeout(serverListener, 5*time.Second)
		if err != nil {
			t.Fatalf("Accept(%d) failed: %v", i, err)
		}
		accepted[conn.PublicKey()] = struct{}{}
	}

	if len(accepted) != peers {
		t.Fatalf("accepted peers=%d, want %d", len(accepted), peers)
	}
}

func TestNilServiceListenerGuard(t *testing.T) {
	var conn *giznet.Conn

	if _, err := conn.Dial(1); !errors.Is(err, giznet.ErrNilConn) {
		t.Fatalf("Dial(1, nil conn) err=%v, want %v", err, giznet.ErrNilConn)
	}

	listener := conn.ListenService(1)
	if _, err := listener.Accept(); !errors.Is(err, giznet.ErrNilConn) {
		t.Fatalf("ListenService(1).Accept(nil conn) err=%v, want %v", err, giznet.ErrNilConn)
	}
	if err := listener.Close(); !errors.Is(err, giznet.ErrNilConn) {
		t.Fatalf("ListenService(1).Close(nil conn) err=%v, want %v", err, giznet.ErrNilConn)
	}
}

func TestServiceListenerAcceptAndClose(t *testing.T) {
	pair := NewConnectedPeerPair(t)
	defer pair.Close()

	listener := pair.ServerConn.ListenService(testServiceRPC)
	if listener.Service() != testServiceRPC {
		t.Fatalf("listener.Service()=%d, want %d", listener.Service(), testServiceRPC)
	}
	if listener.Addr().Network() != "kcp-service" {
		t.Fatalf("listener.Addr().Network()=%q", listener.Addr().Network())
	}

	acceptCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		stream, err := listener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- stream
	}()

	clientStream, err := pair.ClientConn.Dial(testServiceRPC)
	if err != nil {
		t.Fatalf("Dial(rpc) failed: %v", err)
	}
	defer clientStream.Close()

	select {
	case stream := <-acceptCh:
		_ = stream.Close()
	case err := <-errCh:
		t.Fatalf("listener.Accept failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("listener.Accept timeout")
	}

	done := make(chan error, 1)
	go func() {
		_, err := listener.Accept()
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)

	if err := listener.Close(); err != nil {
		t.Fatalf("listener.Close error: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, net.ErrClosed) {
			t.Fatalf("Accept after Close err=%v, want %v", err, net.ErrClosed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Accept did not unblock after listener.Close")
	}
}

func TestNilConnGuard(t *testing.T) {
	var c *giznet.Conn

	if _, err := c.Dial(testServiceRPC); !errors.Is(err, giznet.ErrNilConn) {
		t.Fatalf("Dial(rpc, nil conn) err=%v, want %v", err, giznet.ErrNilConn)
	}
	if _, err := c.ListenService(testServiceRPC).Accept(); !errors.Is(err, giznet.ErrNilConn) {
		t.Fatalf("ListenService(rpc).Accept(nil conn) err=%v, want %v", err, giznet.ErrNilConn)
	}
	if _, err := c.Write(testProtocolEvent, []byte("x")); !errors.Is(err, giznet.ErrNilConn) {
		t.Fatalf("Write(nil conn) err=%v, want %v", err, giznet.ErrNilConn)
	}
	if _, _, err := c.Read(make([]byte, 1)); !errors.Is(err, giznet.ErrNilConn) {
		t.Fatalf("Read(nil conn) err=%v, want %v", err, giznet.ErrNilConn)
	}
	if err := c.Close(); !errors.Is(err, giznet.ErrNilConn) {
		t.Fatalf("Close(nil conn) err=%v, want %v", err, giznet.ErrNilConn)
	}
	if got := c.PublicKey(); got != (giznet.PublicKey{}) {
		t.Fatalf("PublicKey(nil conn) = %v, want zero key", got)
	}
}

func TestListenerAcceptAndConnEventOpus(t *testing.T) {
	serverKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	serverListener := NewTestListener(t, serverKey)
	defer serverListener.Close()
	clientListener := NewTestListener(t, clientKey)
	defer clientListener.Close()

	acceptCh := make(chan *giznet.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		c, err := serverListener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- c
	}()

	ConnectTestListeners(t, clientListener, clientKey, serverListener, serverKey)

	var serverConn *giznet.Conn
	select {
	case serverConn = <-acceptCh:
	case err := <-errCh:
		t.Fatalf("Listener.Accept failed: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("Listener.Accept timeout")
	}

	clientConn, err := clientListener.Peer(serverKey.Public)
	if err != nil {
		t.Fatalf("clientListener.Peer failed: %v", err)
	}

	evt := testEvent{V: testEventVersion, Name: "hello"}
	if err := writeTestEvent(clientConn, evt); err != nil {
		t.Fatalf("writeTestEvent failed: %v", err)
	}

	gotEvent, err := readTestEvent(serverConn)
	if err != nil {
		t.Fatalf("readTestEvent failed: %v", err)
	}
	if gotEvent.Name != evt.Name || gotEvent.V != testEventVersion {
		t.Fatalf("event mismatch: got=%+v want=%+v", gotEvent, evt)
	}
	if gotPK := serverConn.PublicKey(); gotPK != clientKey.Public {
		t.Fatalf("serverConn.PublicKey() mismatch")
	}

	wantStamp := uint64(1234567890123)
	wantRawFrame := []byte("opus-frame")
	frame := stampedopus.Pack(wantStamp, wantRawFrame)
	if err := writeTestOpusFrame(clientConn, frame); err != nil {
		t.Fatalf("writeTestOpusFrame failed: %v", err)
	}

	gotStamp, gotFrame, err := readTestOpusFrame(serverConn)
	if err != nil {
		t.Fatalf("readTestOpusFrame failed: %v", err)
	}
	if gotStamp != wantStamp {
		t.Fatalf("opus frame stamp=%d, want %d", gotStamp, wantStamp)
	}
	if !bytes.Equal(gotFrame, wantRawFrame) {
		t.Fatalf("opus frame payload mismatch: got=%q want=%q", gotFrame, wantRawFrame)
	}

	if err := clientConn.Close(); err != nil {
		t.Fatalf("clientConn.Close failed: %v", err)
	}
}

func TestConnOpenAcceptRPC(t *testing.T) {
	serverKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	serverListener := NewTestListener(t, serverKey)
	defer serverListener.Close()
	clientListener := NewTestListener(t, clientKey)
	defer clientListener.Close()

	acceptConnCh := make(chan *giznet.Conn, 1)
	acceptErrCh := make(chan error, 1)
	go func() {
		c, err := serverListener.Accept()
		if err != nil {
			acceptErrCh <- err
			return
		}
		acceptConnCh <- c
	}()

	ConnectTestListeners(t, clientListener, clientKey, serverListener, serverKey)

	var serverConn *giznet.Conn
	select {
	case serverConn = <-acceptConnCh:
	case err := <-acceptErrCh:
		t.Fatalf("Listener.Accept failed: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("Listener.Accept timeout")
	}

	clientConn, err := clientListener.Peer(serverKey.Public)
	if err != nil {
		t.Fatalf("clientListener.Peer failed: %v", err)
	}

	rpcAcceptCh := make(chan net.Conn, 1)
	rpcErrCh := make(chan error, 1)
	rpcListener := serverConn.ListenService(testServiceRPC)
	defer rpcListener.Close()
	go func() {
		s, err := rpcListener.Accept()
		if err != nil {
			rpcErrCh <- err
			return
		}
		rpcAcceptCh <- s
	}()

	clientStream, err := clientConn.Dial(testServiceRPC)
	if err != nil {
		t.Fatalf("Dial(rpc) failed: %v", err)
	}
	defer clientStream.Close()

	req := []byte(`{"method":"ping"}`)
	if _, err := clientStream.Write(req); err != nil {
		t.Fatalf("client stream write req failed: %v", err)
	}

	var serverStream net.Conn
	select {
	case serverStream = <-rpcAcceptCh:
		defer serverStream.Close()
	case err := <-rpcErrCh:
		t.Fatalf("ListenService(rpc).Accept failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("ListenService(rpc).Accept timeout")
	}

	if got := ReadExactWithTimeout(t, serverStream, len(req), 5*time.Second); !bytes.Equal(got, req) {
		t.Fatalf("server stream request mismatch: got=%q want=%q", got, req)
	}

	resp := []byte(`{"ok":true}`)
	if _, err := serverStream.Write(resp); err != nil {
		t.Fatalf("server stream write resp failed: %v", err)
	}
	if got := ReadExactWithTimeout(t, clientStream, len(resp), 5*time.Second); !bytes.Equal(got, resp) {
		t.Fatalf("client stream response mismatch: got=%q want=%q", got, resp)
	}

	clientRPCAcceptCh := make(chan net.Conn, 1)
	clientRPCErrCh := make(chan error, 1)
	clientRPCListener := clientConn.ListenService(testServiceRPC)
	defer clientRPCListener.Close()
	go func() {
		s, err := clientRPCListener.Accept()
		if err != nil {
			clientRPCErrCh <- err
			return
		}
		clientRPCAcceptCh <- s
	}()

	serverStream2, err := serverConn.Dial(testServiceRPC)
	if err != nil {
		t.Fatalf("server Dial(rpc) failed: %v", err)
	}
	defer serverStream2.Close()

	revReq := []byte(`{"method":"pong"}`)
	if _, err := serverStream2.Write(revReq); err != nil {
		t.Fatalf("server stream write reverse req failed: %v", err)
	}

	var clientStream2 net.Conn
	select {
	case clientStream2 = <-clientRPCAcceptCh:
		defer clientStream2.Close()
	case err := <-clientRPCErrCh:
		t.Fatalf("client ListenService(rpc).Accept failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("client ListenService(rpc).Accept timeout")
	}

	if got := ReadExactWithTimeout(t, clientStream2, len(revReq), 5*time.Second); !bytes.Equal(got, revReq) {
		t.Fatalf("client stream reverse request mismatch: got=%q want=%q", got, revReq)
	}

	revResp := []byte(`{"ok":false}`)
	if _, err := clientStream2.Write(revResp); err != nil {
		t.Fatalf("client stream write reverse resp failed: %v", err)
	}
	if got := ReadExactWithTimeout(t, serverStream2, len(revResp), 5*time.Second); !bytes.Equal(got, revResp) {
		t.Fatalf("server stream reverse response mismatch: got=%q want=%q", got, revResp)
	}
}

func TestConnValidationAndPerProtocolReads(t *testing.T) {
	pair := NewConnectedPeerPair(t)
	defer pair.Close()

	if err := (testEvent{V: testEventVersion, Name: "   "}).Validate(); !errors.Is(err, errTestEventMissingName) {
		t.Fatalf("Event.Validate(blank name) err=%v, want %v", err, errTestEventMissingName)
	}

	if err := writeTestEvent(pair.ClientConn, testEvent{V: testEventVersion, Name: "event-before-opus"}); err != nil {
		t.Fatalf("writeTestEvent failed: %v", err)
	}
	if err := writeTestOpusFrame(pair.ClientConn, stampedopus.Pack(100, []byte{0xF8})); err != nil {
		t.Fatalf("writeTestOpusFrame(valid) failed: %v", err)
	}

	firstProto, firstPayload, err := readPacketWithTimeout(pair.ServerConn, 5*time.Second)
	if err != nil {
		t.Fatalf("read first packet err=%v", err)
	}

	secondProto, secondPayload, err := readPacketWithTimeout(pair.ServerConn, 5*time.Second)
	if err != nil {
		t.Fatalf("read second packet err=%v", err)
	}
	seenEvent := false
	seenOpus := false
	for _, pkt := range []struct {
		proto   byte
		payload []byte
	}{
		{proto: firstProto, payload: firstPayload},
		{proto: secondProto, payload: secondPayload},
	} {
		switch pkt.proto {
		case testProtocolEvent:
			gotEvent, err := decodeTestEvent(pkt.payload)
			if err != nil {
				t.Fatalf("decode event err=%v", err)
			}
			if gotEvent.Name != "event-before-opus" {
				t.Fatalf("event name=%q, want %q", gotEvent.Name, "event-before-opus")
			}
			seenEvent = true
		case testProtocolOpus:
			gotStamp, gotFrame, ok := stampedopus.Unpack(pkt.payload)
			if !ok {
				t.Fatal("stampedopus.Unpack failed")
			}
			if gotStamp != 100 {
				t.Fatalf("opus frame stamp=%d, want 100", gotStamp)
			}
			if !bytes.Equal(gotFrame, []byte{0xF8}) {
				t.Fatalf("opus frame payload=%v, want %v", gotFrame, []byte{0xF8})
			}
			seenOpus = true
		default:
			t.Fatalf("unexpected packet protocol=%d", pkt.proto)
		}
	}
	if !seenEvent || !seenOpus {
		t.Fatalf("seenEvent=%v seenOpus=%v, want both true", seenEvent, seenOpus)
	}

	acceptCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	serviceListener := pair.ServerConn.ListenService(1)
	defer serviceListener.Close()
	go func() {
		svcStream, err := serviceListener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- svcStream
	}()

	time.Sleep(50 * time.Millisecond)

	clientStream, err := pair.ClientConn.Dial(1)
	if err != nil {
		t.Fatalf("Dial(1) failed: %v", err)
	}
	defer clientStream.Close()

	var svcStream net.Conn
	select {
	case svcStream = <-acceptCh:
	case err := <-errCh:
		t.Fatalf("ListenService(1).Accept err=%v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("ListenService(1).Accept timeout")
	}
	_ = svcStream.Close()
}

func TestConnEventConcurrentDelivery(t *testing.T) {
	pair := NewConnectedPeerPair(t)
	defer pair.Close()

	const total = 32

	var wg sync.WaitGroup
	for i := range total {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			e := testEvent{V: testEventVersion, Name: "evt-concurrent"}
			raw := json.RawMessage(fmt.Sprintf(`{"i":%d}`, idx))
			e.Data = &raw
			if err := writeTestEvent(pair.ClientConn, e); err != nil {
				t.Errorf("writeTestEvent(%d) failed: %v", idx, err)
			}
		}()
	}
	wg.Wait()

	for i := 0; i < total; i++ {
		evt, err := readEventWithTimeout(pair.ServerConn, 5*time.Second)
		if err != nil {
			t.Fatalf("ReadEvent(%d) failed: %v", i, err)
		}
		if evt.Name != "evt-concurrent" {
			t.Fatalf("event name mismatch: got=%q", evt.Name)
		}
	}
}

func TestConnUnderlyingErrorPropagation(t *testing.T) {
	pair := NewConnectedPeerPair(t)
	defer pair.Close()

	_ = pair.ClientListener.UDP().Close()

	if _, err := pair.ClientConn.Dial(testServiceRPC); !errors.Is(err, giznet.ErrUDPClosed) {
		t.Fatalf("Dial(rpc, after close) err=%v, want %v", err, giznet.ErrUDPClosed)
	}
	if _, err := pair.ClientConn.Write(testProtocolEvent, []byte("x")); !errors.Is(err, giznet.ErrUDPClosed) {
		t.Fatalf("Write(after close) err=%v, want %v", err, giznet.ErrUDPClosed)
	}
}

func TestConnCloseIsHandleLocal(t *testing.T) {
	pair := NewConnectedPeerPair(t)
	defer pair.Close()

	secondHandle, err := pair.ClientListener.Peer(pair.ServerKey.Public)
	if err != nil {
		t.Fatalf("clientListener.Peer failed: %v", err)
	}

	if err := pair.ClientConn.Close(); err != nil {
		t.Fatalf("clientConn.Close failed: %v", err)
	}
	if _, err := pair.ClientConn.Dial(testServiceRPC); !errors.Is(err, giznet.ErrConnClosed) {
		t.Fatalf("Dial(rpc, after Conn.Close) err=%v, want %v", err, giznet.ErrConnClosed)
	}
	if _, err := pair.ClientConn.Write(testProtocolEvent, []byte("x")); !errors.Is(err, giznet.ErrConnClosed) {
		t.Fatalf("Write(after Conn.Close) err=%v, want %v", err, giznet.ErrConnClosed)
	}

	evt := testEvent{V: testEventVersion, Name: "still-open"}
	if err := writeTestEvent(secondHandle, evt); err != nil {
		t.Fatalf("second handle writeTestEvent failed: %v", err)
	}
	gotEvent, err := readTestEvent(pair.ServerConn)
	if err != nil {
		t.Fatalf("server readTestEvent failed: %v", err)
	}
	if gotEvent.Name != evt.Name {
		t.Fatalf("ReadEvent name=%q, want %q", gotEvent.Name, evt.Name)
	}
}

func TestConnCloseDoesNotAffectOpenRPCStreams(t *testing.T) {
	pair := NewConnectedPeerPair(t)
	defer pair.Close()

	acceptCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	rpcListener := pair.ServerConn.ListenService(testServiceRPC)
	defer rpcListener.Close()
	go func() {
		stream, err := rpcListener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- stream
	}()

	clientStream, err := pair.ClientConn.Dial(testServiceRPC)
	if err != nil {
		t.Fatalf("client Dial(rpc) failed: %v", err)
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
		t.Fatalf("server ListenService(rpc).Accept failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server ListenService(rpc).Accept timeout")
	}

	if got := ReadExactWithTimeout(t, serverStream, 1, 5*time.Second); !bytes.Equal(got, []byte("x")) {
		t.Fatalf("server stream priming payload mismatch: got=%q want=%q", string(got), "x")
	}

	if err := pair.ClientConn.Close(); err != nil {
		t.Fatalf("clientConn.Close failed: %v", err)
	}

	if _, err := clientStream.Write([]byte("y")); err != nil {
		t.Fatalf("client stream write after Conn.Close failed: %v", err)
	}
	if got := ReadExactWithTimeout(t, serverStream, 1, 5*time.Second); !bytes.Equal(got, []byte("y")) {
		t.Fatalf("server stream payload after Conn.Close mismatch: got=%q want=%q", string(got), "y")
	}
}
