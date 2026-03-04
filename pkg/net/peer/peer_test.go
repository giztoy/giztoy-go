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

	"github.com/haivivi/giztoy/go/pkg/net/core"
	"github.com/haivivi/giztoy/go/pkg/net/noise"
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

func TestWrapNilUDP(t *testing.T) {
	if _, err := Wrap(nil); !errors.Is(err, ErrNilUDP) {
		t.Fatalf("Wrap(nil) err=%v, want %v", err, ErrNilUDP)
	}
}

func TestListenerPeerErrors(t *testing.T) {
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	u, err := core.NewUDP(key, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("NewUDP failed: %v", err)
	}
	defer u.Close()

	l, err := Wrap(u)
	if err != nil {
		t.Fatalf("Wrap failed: %v", err)
	}
	defer l.Close()

	unknown, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate unknown key failed: %v", err)
	}

	if _, err := l.Peer(unknown.Public); !errors.Is(err, core.ErrPeerNotFound) {
		t.Fatalf("Peer(unknown) err=%v, want %v", err, core.ErrPeerNotFound)
	}

	u.SetPeerEndpoint(unknown.Public, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 54321})
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

	serverUDP, err := core.NewUDP(serverKey, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("NewUDP(server) failed: %v", err)
	}
	defer serverUDP.Close()

	clientUDP, err := core.NewUDP(clientKey, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("NewUDP(client) failed: %v", err)
	}
	defer clientUDP.Close()

	startReadLoop(serverUDP)
	startReadLoop(clientUDP)

	serverListener, err := Wrap(serverUDP)
	if err != nil {
		t.Fatalf("Wrap(server) failed: %v", err)
	}
	defer serverListener.Close()

	clientListener, err := Wrap(clientUDP)
	if err != nil {
		t.Fatalf("Wrap(client) failed: %v", err)
	}
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

	clientUDP.SetPeerEndpoint(serverKey.Public, serverUDP.HostInfo().Addr)
	serverUDP.SetPeerEndpoint(clientKey.Public, clientUDP.HostInfo().Addr)

	if err := clientUDP.Connect(serverKey.Public); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	waitEstablished(t, serverUDP, clientKey.Public)
	waitEstablished(t, clientUDP, serverKey.Public)

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

	serverUDP, err := core.NewUDP(serverKey, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("NewUDP(server) failed: %v", err)
	}
	defer serverUDP.Close()

	clientUDP, err := core.NewUDP(clientKey, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("NewUDP(client) failed: %v", err)
	}
	defer clientUDP.Close()

	startReadLoop(serverUDP)
	startReadLoop(clientUDP)

	serverListener, err := Wrap(serverUDP)
	if err != nil {
		t.Fatalf("Wrap(server) failed: %v", err)
	}
	defer serverListener.Close()

	clientListener, err := Wrap(clientUDP)
	if err != nil {
		t.Fatalf("Wrap(client) failed: %v", err)
	}
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

	clientUDP.SetPeerEndpoint(serverKey.Public, serverUDP.HostInfo().Addr)
	serverUDP.SetPeerEndpoint(clientKey.Public, clientUDP.HostInfo().Addr)

	if err := clientUDP.Connect(serverKey.Public); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	waitEstablished(t, serverUDP, clientKey.Public)
	waitEstablished(t, clientUDP, serverKey.Public)

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

func TestConnValidationAndProtocolErrorPaths(t *testing.T) {
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
	if _, err := pair.serverConn.ReadOpusFrame(); !errors.Is(err, ErrUnexpectedProtocol) {
		t.Fatalf("ReadOpusFrame(non-opus) err=%v, want %v", err, ErrUnexpectedProtocol)
	}

	if err := pair.clientConn.SendOpusFrame(StampOpusFrame([]byte{0xF8}, 100)); err != nil {
		t.Fatalf("SendOpusFrame(valid) failed: %v", err)
	}
	if _, err := pair.serverConn.ReadEvent(); !errors.Is(err, ErrUnexpectedProtocol) {
		t.Fatalf("ReadEvent(non-event) err=%v, want %v", err, ErrUnexpectedProtocol)
	}

	if _, err := (&Conn{pk: pair.serverKey.Public}).OpenRPC(); !errors.Is(err, ErrNilConn) {
		t.Fatalf("OpenRPC(nil udp) err=%v, want %v", err, ErrNilConn)
	}

	smux, err := pair.serverUDP.GetServiceMux(pair.clientKey.Public)
	if err != nil {
		t.Fatalf("server GetServiceMux failed: %v", err)
	}
	if err := smux.Input(1, []byte{0x00, 0x01, 0x02}); err != nil {
		t.Fatalf("smux.Input(service=1) failed: %v", err)
	}
	if _, err := pair.serverConn.AcceptRPC(); !errors.Is(err, core.ErrUnsupportedService) {
		t.Fatalf("AcceptRPC(service!=0) err=%v, want %v", err, core.ErrUnsupportedService)
	}
}

func TestConnEventConcurrentDelivery(t *testing.T) {
	pair := newConnectedPeerPair(t)
	defer pair.Close()

	const total = 32

	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
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

	serverUDP, err := core.NewUDP(serverKey, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("NewUDP(server) failed: %v", err)
	}
	defer serverUDP.Close()
	startReadLoop(serverUDP)

	serverListener, err := Wrap(serverUDP)
	if err != nil {
		t.Fatalf("Wrap(server) failed: %v", err)
	}
	defer serverListener.Close()

	const peers = 3
	type clientNode struct {
		key *noise.KeyPair
		udp *core.UDP
	}
	clients := make([]clientNode, 0, peers)
	for i := 0; i < peers; i++ {
		k, err := noise.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Generate client key %d failed: %v", i, err)
		}
		u, err := core.NewUDP(k, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
		if err != nil {
			t.Fatalf("NewUDP(client %d) failed: %v", i, err)
		}
		startReadLoop(u)
		clients = append(clients, clientNode{key: k, udp: u})
	}
	defer func() {
		for _, c := range clients {
			_ = c.udp.Close()
		}
	}()

	var connectWG sync.WaitGroup
	for _, c := range clients {
		connectWG.Add(1)
		client := c
		go func() {
			defer connectWG.Done()
			client.udp.SetPeerEndpoint(serverKey.Public, serverUDP.HostInfo().Addr)
			serverUDP.SetPeerEndpoint(client.key.Public, client.udp.HostInfo().Addr)
			if err := client.udp.Connect(serverKey.Public); err != nil {
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

	_ = pair.clientUDP.Close()

	if _, err := pair.clientConn.OpenRPC(); !errors.Is(err, core.ErrClosed) {
		t.Fatalf("OpenRPC(after close) err=%v, want %v", err, core.ErrClosed)
	}
	if err := pair.clientConn.SendEvent(Event{V: PrologueVersion, Name: "x"}); !errors.Is(err, core.ErrClosed) {
		t.Fatalf("SendEvent(after close) err=%v, want %v", err, core.ErrClosed)
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

func waitEstablished(t *testing.T, u *core.UDP, pk noise.PublicKey) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if info := u.PeerInfo(pk); info != nil && info.State == core.PeerStateEstablished {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	info := u.PeerInfo(pk)
	if info == nil {
		t.Fatal("peer info is nil")
	}
	t.Fatalf("peer state not established: got=%s", info.State)
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

	serverUDP *core.UDP
	clientUDP *core.UDP

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

	serverUDP, err := core.NewUDP(serverKey, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("NewUDP(server) failed: %v", err)
	}

	clientUDP, err := core.NewUDP(clientKey, core.WithBindAddr("127.0.0.1:0"), core.WithAllowUnknown(true))
	if err != nil {
		_ = serverUDP.Close()
		t.Fatalf("NewUDP(client) failed: %v", err)
	}

	startReadLoop(serverUDP)
	startReadLoop(clientUDP)

	serverListener, err := Wrap(serverUDP)
	if err != nil {
		_ = serverUDP.Close()
		_ = clientUDP.Close()
		t.Fatalf("Wrap(server) failed: %v", err)
	}
	clientListener, err := Wrap(clientUDP)
	if err != nil {
		_ = serverListener.Close()
		_ = serverUDP.Close()
		_ = clientUDP.Close()
		t.Fatalf("Wrap(client) failed: %v", err)
	}

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

	clientUDP.SetPeerEndpoint(serverKey.Public, serverUDP.HostInfo().Addr)
	serverUDP.SetPeerEndpoint(clientKey.Public, clientUDP.HostInfo().Addr)

	if err := clientUDP.Connect(serverKey.Public); err != nil {
		_ = serverListener.Close()
		_ = clientListener.Close()
		_ = serverUDP.Close()
		_ = clientUDP.Close()
		t.Fatalf("Connect failed: %v", err)
	}

	waitEstablished(t, serverUDP, clientKey.Public)
	waitEstablished(t, clientUDP, serverKey.Public)

	var serverConn *Conn
	select {
	case serverConn = <-acceptCh:
	case err := <-errCh:
		_ = serverListener.Close()
		_ = clientListener.Close()
		_ = serverUDP.Close()
		_ = clientUDP.Close()
		t.Fatalf("Listener.Accept failed: %v", err)
	case <-time.After(3 * time.Second):
		_ = serverListener.Close()
		_ = clientListener.Close()
		_ = serverUDP.Close()
		_ = clientUDP.Close()
		t.Fatal("Listener.Accept timeout")
	}

	clientConn, err := clientListener.Peer(serverKey.Public)
	if err != nil {
		_ = serverListener.Close()
		_ = clientListener.Close()
		_ = serverUDP.Close()
		_ = clientUDP.Close()
		t.Fatalf("clientListener.Peer failed: %v", err)
	}

	return &connectedPeerPair{
		serverKey: serverKey,
		clientKey: clientKey,

		serverUDP: serverUDP,
		clientUDP: clientUDP,

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
	if p.serverUDP != nil {
		_ = p.serverUDP.Close()
	}
	if p.clientUDP != nil {
		_ = p.clientUDP.Close()
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
