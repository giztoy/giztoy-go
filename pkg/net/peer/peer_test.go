package peer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/pkg/net/core"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

func TestNilListenerGuard(t *testing.T) {
	var l *Listener

	if _, err := l.Accept(); !errors.Is(err, ErrNilListener) {
		t.Fatalf("Accept(nil listener) err=%v, want %v", err, ErrNilListener)
	}

	if _, err := l.Peer(noise.PublicKey{}); !errors.Is(err, ErrNilListener) {
		t.Fatalf("Peer(nil listener) err=%v, want %v", err, ErrNilListener)
	}

	if err := l.Close(); !errors.Is(err, ErrNilListener) {
		t.Fatalf("Close(nil listener) err=%v, want %v", err, ErrNilListener)
	}
}

func TestNilConnGuard(t *testing.T) {
	var c *Conn

	if _, err := c.OpenRPC(); !errors.Is(err, ErrNilConn) {
		t.Fatalf("OpenRPC(nil conn) err=%v, want %v", err, ErrNilConn)
	}
	if _, err := c.AcceptRPC(); !errors.Is(err, ErrNilConn) {
		t.Fatalf("AcceptRPC(nil conn) err=%v, want %v", err, ErrNilConn)
	}
	if err := c.SendEvent(Event{V: 1, Name: "x"}); !errors.Is(err, ErrNilConn) {
		t.Fatalf("SendEvent(nil conn) err=%v, want %v", err, ErrNilConn)
	}
	if _, err := c.ReadEvent(); !errors.Is(err, ErrNilConn) {
		t.Fatalf("ReadEvent(nil conn) err=%v, want %v", err, ErrNilConn)
	}
	if err := c.SendOpusFrame(nil); !errors.Is(err, ErrNilConn) {
		t.Fatalf("SendOpusFrame(nil conn) err=%v, want %v", err, ErrNilConn)
	}
	if _, err := c.ReadOpusFrame(); !errors.Is(err, ErrNilConn) {
		t.Fatalf("ReadOpusFrame(nil conn) err=%v, want %v", err, ErrNilConn)
	}
	if err := c.Close(); !errors.Is(err, ErrNilConn) {
		t.Fatalf("Close(nil conn) err=%v, want %v", err, ErrNilConn)
	}
	if got := c.PublicKey(); got != (noise.PublicKey{}) {
		t.Fatalf("PublicKey(nil conn) = %v, want zero key", got)
	}
}

func TestListenAndCloseOwnedListener(t *testing.T) {
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	l, err := Listen(key, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	if err := l.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if _, err := l.Accept(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Accept after Close err=%v, want %v", err, ErrClosed)
	}
}

func TestListenerPeerErrors(t *testing.T) {
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	l, err := Listen(key, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	unknown, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate unknown key failed: %v", err)
	}

	if _, err := l.Peer(unknown.Public); !errors.Is(err, core.ErrPeerNotFound) {
		t.Fatalf("Peer(unknown) err=%v, want %v", err, core.ErrPeerNotFound)
	}

	l.SetPeerEndpoint(unknown.Public, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 54321})
	if _, err := l.Peer(unknown.Public); !errors.Is(err, core.ErrNoSession) {
		t.Fatalf("Peer(no session) err=%v, want %v", err, core.ErrNoSession)
	}
}

func TestListenerAcceptAndConnEventOpus(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	serverListener := newTestListener(t, serverKey)
	defer serverListener.Close()
	clientListener := newTestListener(t, clientKey)
	defer clientListener.Close()

	acceptCh := make(chan *Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		c, err := serverListener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- c
	}()

	connectTestListeners(t, clientListener, clientKey, serverListener, serverKey)

	var serverConn *Conn
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

	evt := Event{V: PrologueVersion, Name: "hello"}
	if err := clientConn.SendEvent(evt); err != nil {
		t.Fatalf("SendEvent failed: %v", err)
	}

	gotEvent, err := serverConn.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent failed: %v", err)
	}
	if gotEvent.Name != evt.Name || gotEvent.V != PrologueVersion {
		t.Fatalf("event mismatch: got=%+v want=%+v", gotEvent, evt)
	}
	if gotPK := serverConn.PublicKey(); gotPK != clientKey.Public {
		t.Fatalf("serverConn.PublicKey() mismatch")
	}

	wantStamp := EpochMillis(1234567890123)
	wantRawFrame := []byte("opus-frame")
	frame := StampOpusFrame(wantRawFrame, wantStamp)
	if err := clientConn.SendOpusFrame(frame); err != nil {
		t.Fatalf("SendOpusFrame failed: %v", err)
	}

	gotFrame, err := serverConn.ReadOpusFrame()
	if err != nil {
		t.Fatalf("ReadOpusFrame failed: %v", err)
	}
	if gotFrame.Version() != OpusFrameVersion {
		t.Fatalf("opus frame version=%d, want %d", gotFrame.Version(), OpusFrameVersion)
	}
	if gotFrame.Stamp() != wantStamp {
		t.Fatalf("opus frame stamp=%d, want %d", gotFrame.Stamp(), wantStamp)
	}
	if !bytes.Equal(gotFrame.Frame(), wantRawFrame) {
		t.Fatalf("opus frame payload mismatch: got=%q want=%q", gotFrame.Frame(), wantRawFrame)
	}

	if err := clientConn.Close(); err != nil {
		t.Fatalf("clientConn.Close failed: %v", err)
	}
}

func TestConnOpenAcceptRPC(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	serverListener := newTestListener(t, serverKey)
	defer serverListener.Close()
	clientListener := newTestListener(t, clientKey)
	defer clientListener.Close()

	acceptConnCh := make(chan *Conn, 1)
	acceptErrCh := make(chan error, 1)
	go func() {
		c, err := serverListener.Accept()
		if err != nil {
			acceptErrCh <- err
			return
		}
		acceptConnCh <- c
	}()

	connectTestListeners(t, clientListener, clientKey, serverListener, serverKey)

	var serverConn *Conn
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
	go func() {
		s, err := serverConn.AcceptRPC()
		if err != nil {
			rpcErrCh <- err
			return
		}
		rpcAcceptCh <- s
	}()

	clientStream, err := clientConn.OpenRPC()
	if err != nil {
		t.Fatalf("OpenRPC failed: %v", err)
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
		t.Fatalf("AcceptRPC failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptRPC timeout")
	}

	if got := readExactWithTimeout(t, serverStream, len(req), 5*time.Second); !bytes.Equal(got, req) {
		t.Fatalf("server stream request mismatch: got=%q want=%q", got, req)
	}

	resp := []byte(`{"ok":true}`)
	if _, err := serverStream.Write(resp); err != nil {
		t.Fatalf("server stream write resp failed: %v", err)
	}
	if got := readExactWithTimeout(t, clientStream, len(resp), 5*time.Second); !bytes.Equal(got, resp) {
		t.Fatalf("client stream response mismatch: got=%q want=%q", got, resp)
	}

	// Reverse direction: server -> client
	clientRPCAcceptCh := make(chan net.Conn, 1)
	clientRPCErrCh := make(chan error, 1)
	go func() {
		s, err := clientConn.AcceptRPC()
		if err != nil {
			clientRPCErrCh <- err
			return
		}
		clientRPCAcceptCh <- s
	}()

	serverStream2, err := serverConn.OpenRPC()
	if err != nil {
		t.Fatalf("server OpenRPC failed: %v", err)
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
		t.Fatalf("client AcceptRPC failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("client AcceptRPC timeout")
	}

	if got := readExactWithTimeout(t, clientStream2, len(revReq), 5*time.Second); !bytes.Equal(got, revReq) {
		t.Fatalf("client stream reverse request mismatch: got=%q want=%q", got, revReq)
	}

	revResp := []byte(`{"ok":false}`)
	if _, err := clientStream2.Write(revResp); err != nil {
		t.Fatalf("client stream write reverse resp failed: %v", err)
	}
	if got := readExactWithTimeout(t, serverStream2, len(revResp), 5*time.Second); !bytes.Equal(got, revResp) {
		t.Fatalf("server stream reverse response mismatch: got=%q want=%q", got, revResp)
	}
}

func TestConnValidationAndPerProtocolReads(t *testing.T) {
	pair := newConnectedPeerPair(t)
	defer pair.Close()

	if err := pair.clientConn.SendEvent(Event{V: PrologueVersion, Name: "   "}); !errors.Is(err, ErrMissingName) {
		t.Fatalf("SendEvent(blank name) err=%v, want %v", err, ErrMissingName)
	}

	if err := pair.clientConn.SendOpusFrame(StampedOpusFrame{1, 2, 3}); !errors.Is(err, ErrOpusFrameTooShort) {
		t.Fatalf("SendOpusFrame(short frame) err=%v, want %v", err, ErrOpusFrameTooShort)
	}

	if err := pair.clientConn.SendOpusFrame(StampedOpusFrame{2, 0, 0, 0, 0, 0, 0, 0, 0xF8}); !errors.Is(err, ErrInvalidOpusFrameVersion) {
		t.Fatalf("SendOpusFrame(invalid version) err=%v, want %v", err, ErrInvalidOpusFrameVersion)
	}

	if err := pair.clientConn.SendEvent(Event{V: PrologueVersion, Name: "event-as-opus"}); err != nil {
		t.Fatalf("SendEvent failed: %v", err)
	}
	opusCh := make(chan StampedOpusFrame, 1)
	opusErrCh := make(chan error, 1)
	go func() {
		frame, err := pair.serverConn.ReadOpusFrame()
		if err != nil {
			opusErrCh <- err
			return
		}
		opusCh <- frame
	}()

	select {
	case err := <-opusErrCh:
		t.Fatalf("ReadOpusFrame(event queued first) err=%v", err)
	case <-opusCh:
		t.Fatal("ReadOpusFrame should not consume EVENT packets")
	case <-time.After(150 * time.Millisecond):
	}

	if err := pair.clientConn.SendOpusFrame(StampOpusFrame([]byte{0xF8}, 100)); err != nil {
		t.Fatalf("SendOpusFrame(valid) failed: %v", err)
	}

	select {
	case err := <-opusErrCh:
		t.Fatalf("ReadOpusFrame(valid opus) err=%v", err)
	case gotFrame := <-opusCh:
		if gotFrame.Stamp() != 100 {
			t.Fatalf("ReadOpusFrame stamp=%d, want 100", gotFrame.Stamp())
		}
		if !bytes.Equal(gotFrame.Frame(), []byte{0xF8}) {
			t.Fatalf("ReadOpusFrame payload=%v, want %v", gotFrame.Frame(), []byte{0xF8})
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ReadOpusFrame(valid opus) timeout")
	}

	gotEvent, err := pair.serverConn.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent(queued event) err=%v", err)
	}
	if gotEvent.Name != "event-as-opus" {
		t.Fatalf("ReadEvent name=%q, want %q", gotEvent.Name, "event-as-opus")
	}

	if _, err := (&Conn{pk: pair.serverKey.Public}).OpenRPC(); !errors.Is(err, ErrNilConn) {
		t.Fatalf("OpenRPC(nil udp) err=%v, want %v", err, ErrNilConn)
	}

	acceptCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		svcStream, err := pair.serverConn.AcceptService(1)
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- svcStream
	}()

	time.Sleep(50 * time.Millisecond)

	clientStream, err := pair.clientConn.OpenService(1)
	if err != nil {
		t.Fatalf("OpenService(1) failed: %v", err)
	}
	defer clientStream.Close()

	var svcStream net.Conn
	select {
	case svcStream = <-acceptCh:
	case err := <-errCh:
		t.Fatalf("AcceptService(1) err=%v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("AcceptService(1) timeout")
	}
	_ = svcStream.Close()
}

func TestConnEventConcurrentDelivery(t *testing.T) {
	pair := newConnectedPeerPair(t)
	defer pair.Close()

	const total = 32

	var wg sync.WaitGroup
	for i := range total {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			e := Event{V: PrologueVersion, Name: "evt-concurrent"}
			e.Data = []byte(fmt.Sprintf(`{"i":%d}`, idx))
			if err := pair.clientConn.SendEvent(e); err != nil {
				t.Errorf("SendEvent(%d) failed: %v", idx, err)
			}
		}()
	}
	wg.Wait()

	for i := 0; i < total; i++ {
		evt, err := readEventWithTimeout(pair.serverConn, 5*time.Second)
		if err != nil {
			t.Fatalf("ReadEvent(%d) failed: %v", i, err)
		}
		if evt.Name != "evt-concurrent" {
			t.Fatalf("event name mismatch: got=%q", evt.Name)
		}
	}
}

func TestPeerMultipleConcurrentConnections(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}

	serverListener := newTestListener(t, serverKey)
	defer serverListener.Close()

	const peers = 3
	type clientNode struct {
		key      *noise.KeyPair
		listener *Listener
	}
	clients := make([]clientNode, 0, peers)
	for i := 0; i < peers; i++ {
		k, err := noise.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Generate client key %d failed: %v", i, err)
		}
		cl := newTestListener(t, k)
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

	accepted := make(map[noise.PublicKey]struct{})
	for i := 0; i < peers; i++ {
		conn, err := acceptConnWithTimeout(serverListener, 5*time.Second)
		if err != nil {
			t.Fatalf("Accept(%d) failed: %v", i, err)
		}
		accepted[conn.PublicKey()] = struct{}{}
	}

	if len(accepted) != peers {
		t.Fatalf("accepted peers=%d, want %d", len(accepted), peers)
	}
}

func TestConnUnderlyingErrorPropagation(t *testing.T) {
	pair := newConnectedPeerPair(t)
	defer pair.Close()

	_ = pair.clientListener.UDP().Close()

	if _, err := pair.clientConn.OpenRPC(); !errors.Is(err, core.ErrClosed) {
		t.Fatalf("OpenRPC(after close) err=%v, want %v", err, core.ErrClosed)
	}
	if err := pair.clientConn.SendEvent(Event{V: PrologueVersion, Name: "x"}); !errors.Is(err, core.ErrClosed) {
		t.Fatalf("SendEvent(after close) err=%v, want %v", err, core.ErrClosed)
	}
}

func TestConnCloseIsHandleLocal(t *testing.T) {
	pair := newConnectedPeerPair(t)
	defer pair.Close()

	secondHandle, err := pair.clientListener.Peer(pair.serverKey.Public)
	if err != nil {
		t.Fatalf("clientListener.Peer failed: %v", err)
	}

	if err := pair.clientConn.Close(); err != nil {
		t.Fatalf("clientConn.Close failed: %v", err)
	}
	if _, err := pair.clientConn.OpenRPC(); !errors.Is(err, ErrConnClosed) {
		t.Fatalf("OpenRPC(after Conn.Close) err=%v, want %v", err, ErrConnClosed)
	}
	if err := pair.clientConn.SendEvent(Event{V: PrologueVersion, Name: "x"}); !errors.Is(err, ErrConnClosed) {
		t.Fatalf("SendEvent(after Conn.Close) err=%v, want %v", err, ErrConnClosed)
	}

	evt := Event{V: PrologueVersion, Name: "still-open"}
	if err := secondHandle.SendEvent(evt); err != nil {
		t.Fatalf("second handle SendEvent failed: %v", err)
	}
	gotEvent, err := pair.serverConn.ReadEvent()
	if err != nil {
		t.Fatalf("server ReadEvent failed: %v", err)
	}
	if gotEvent.Name != evt.Name {
		t.Fatalf("ReadEvent name=%q, want %q", gotEvent.Name, evt.Name)
	}
}

func TestConnCloseDoesNotAffectOpenRPCStreams(t *testing.T) {
	pair := newConnectedPeerPair(t)
	defer pair.Close()

	acceptCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		stream, err := pair.serverConn.AcceptRPC()
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- stream
	}()

	clientStream, err := pair.clientConn.OpenRPC()
	if err != nil {
		t.Fatalf("client OpenRPC failed: %v", err)
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
		t.Fatalf("server AcceptRPC failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server AcceptRPC timeout")
	}

	if got := readExactWithTimeout(t, serverStream, 1, 5*time.Second); !bytes.Equal(got, []byte("x")) {
		t.Fatalf("server stream priming payload mismatch: got=%q want=%q", string(got), "x")
	}

	if err := pair.clientConn.Close(); err != nil {
		t.Fatalf("clientConn.Close failed: %v", err)
	}

	if _, err := clientStream.Write([]byte("y")); err != nil {
		t.Fatalf("client stream write after Conn.Close failed: %v", err)
	}
	if got := readExactWithTimeout(t, serverStream, 1, 5*time.Second); !bytes.Equal(got, []byte("y")) {
		t.Fatalf("server stream payload after Conn.Close mismatch: got=%q want=%q", string(got), "y")
	}
}

func TestListenerDoesNotAcceptSamePeerAgainOnReconnect(t *testing.T) {
	pair := newConnectedPeerPair(t)
	defer pair.Close()

	acceptCh := make(chan *Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := pair.serverListener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- conn
	}()

	if err := pair.clientListener.Connect(pair.serverKey.Public); err != nil {
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

func newTestListener(t *testing.T, key *noise.KeyPair) *Listener {
	t.Helper()
	l, err := Listen(key, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	startReadLoop(l.UDP())
	return l
}

func connectTestListeners(t *testing.T, client *Listener, clientKey *noise.KeyPair, server *Listener, serverKey *noise.KeyPair) {
	t.Helper()
	client.SetPeerEndpoint(serverKey.Public, server.HostInfo().Addr)
	server.SetPeerEndpoint(clientKey.Public, client.HostInfo().Addr)
	if err := client.Connect(serverKey.Public); err != nil {
		t.Fatalf("Connect failed: %v", err)
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

func readExactWithTimeout(t *testing.T, r io.Reader, n int, timeout time.Duration) []byte {
	t.Helper()

	errCh := make(chan error, 1)
	buf := make([]byte, n)
	go func() {
		_, err := io.ReadFull(r, buf)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ReadFull failed: %v", err)
		}
		return buf
	case <-time.After(timeout):
		t.Fatalf("ReadFull timeout after %s", timeout)
		return nil
	}
}

type connectedPeerPair struct {
	serverKey *noise.KeyPair
	clientKey *noise.KeyPair

	serverListener *Listener
	clientListener *Listener

	serverConn *Conn
	clientConn *Conn
}

func newConnectedPeerPair(t *testing.T) *connectedPeerPair {
	t.Helper()

	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	serverListener := newTestListener(t, serverKey)
	clientListener := newTestListener(t, clientKey)

	acceptCh := make(chan *Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		c, err := serverListener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- c
	}()

	connectTestListeners(t, clientListener, clientKey, serverListener, serverKey)

	var serverConn *Conn
	select {
	case serverConn = <-acceptCh:
	case err := <-errCh:
		_ = serverListener.Close()
		_ = clientListener.Close()
		t.Fatalf("Listener.Accept failed: %v", err)
	case <-time.After(3 * time.Second):
		_ = serverListener.Close()
		_ = clientListener.Close()
		t.Fatal("Listener.Accept timeout")
	}

	clientConn, err := clientListener.Peer(serverKey.Public)
	if err != nil {
		_ = serverListener.Close()
		_ = clientListener.Close()
		t.Fatalf("clientListener.Peer failed: %v", err)
	}

	return &connectedPeerPair{
		serverKey: serverKey,
		clientKey: clientKey,

		serverListener: serverListener,
		clientListener: clientListener,

		serverConn: serverConn,
		clientConn: clientConn,
	}
}

func (p *connectedPeerPair) Close() {
	if p == nil {
		return
	}
	if p.serverListener != nil {
		_ = p.serverListener.Close()
	}
	if p.clientListener != nil {
		_ = p.clientListener.Close()
	}
}

func readEventWithTimeout(c *Conn, timeout time.Duration) (Event, error) {
	type result struct {
		evt Event
		err error
	}

	ch := make(chan result, 1)
	go func() {
		evt, err := c.ReadEvent()
		ch <- result{evt: evt, err: err}
	}()

	select {
	case r := <-ch:
		return r.evt, r.err
	case <-time.After(timeout):
		return Event{}, errors.New("read event timeout")
	}
}

func acceptConnWithTimeout(l *Listener, timeout time.Duration) (*Conn, error) {
	type result struct {
		conn *Conn
		err  error
	}

	ch := make(chan result, 1)
	go func() {
		conn, err := l.Accept()
		ch <- result{conn: conn, err: err}
	}()

	select {
	case r := <-ch:
		return r.conn, r.err
	case <-time.After(timeout):
		return nil, errors.New("accept timeout")
	}
}
