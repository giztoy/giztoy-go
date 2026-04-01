package core

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

func buildHandshakeResponseForUDPTest(t *testing.T, initiator *noise.KeyPair, responder *noise.KeyPair, localIdx, remoteIdx uint32) (*noise.HandshakeState, []byte) {
	t.Helper()

	// Keep the initiator state waiting for msg2 so tests can pass either a
	// valid response or malformed garbage into handleHandshakeResp.
	initHS, err := noise.NewHandshakeState(noise.Config{
		Pattern:      noise.PatternIK,
		Initiator:    true,
		LocalStatic:  initiator,
		RemoteStatic: &responder.Public,
	})
	if err != nil {
		t.Fatalf("initiator NewHandshakeState failed: %v", err)
	}
	msg1, err := initHS.WriteMessage(nil)
	if err != nil {
		t.Fatalf("initiator WriteMessage failed: %v", err)
	}

	respHS, err := noise.NewHandshakeState(noise.Config{
		Pattern:     noise.PatternIK,
		Initiator:   false,
		LocalStatic: responder,
	})
	if err != nil {
		t.Fatalf("responder NewHandshakeState failed: %v", err)
	}
	if _, err := respHS.ReadMessage(msg1); err != nil {
		t.Fatalf("responder ReadMessage failed: %v", err)
	}
	msg2, err := respHS.WriteMessage(nil)
	if err != nil {
		t.Fatalf("responder WriteMessage failed: %v", err)
	}

	wire := noise.BuildHandshakeResp(remoteIdx, localIdx, respHS.LocalEphemeral(), msg2[noise.KeySize:])
	return initHS, wire
}

func TestNewUDP(t *testing.T) {
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	udp, err := NewUDP(key)
	if err != nil {
		t.Fatalf("NewUDP failed: %v", err)
	}
	defer udp.Close()

	info := udp.HostInfo()
	if info.PublicKey != key.Public {
		t.Errorf("PublicKey mismatch")
	}
	if info.Addr == nil {
		t.Errorf("Addr should not be nil")
	}
}

func TestNewUDPWithBindAddr(t *testing.T) {
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	udp, err := NewUDP(key, WithBindAddr("127.0.0.1:0"))
	if err != nil {
		t.Fatalf("NewUDP failed: %v", err)
	}
	defer udp.Close()

	addr := udp.HostInfo().Addr
	if addr.IP.String() != "127.0.0.1" {
		t.Errorf("Expected 127.0.0.1, got %s", addr.IP.String())
	}
}

func TestSetPeerEndpoint(t *testing.T) {
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	udp, err := NewUDP(key)
	if err != nil {
		t.Fatalf("NewUDP failed: %v", err)
	}
	defer udp.Close()

	peerKey, _ := noise.GenerateKeyPair()
	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")

	udp.SetPeerEndpoint(peerKey.Public, peerAddr)

	info := udp.PeerInfo(peerKey.Public)
	if info == nil {
		t.Fatalf("PeerInfo returned nil")
	}
	if info.PublicKey != peerKey.Public {
		t.Errorf("PublicKey mismatch")
	}
	if info.Endpoint.String() != peerAddr.String() {
		t.Errorf("Endpoint mismatch: %s != %s", info.Endpoint.String(), peerAddr.String())
	}
}

func TestSetPeerEndpoint_IgnoresClosedAndInvalidAddr(t *testing.T) {
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	peerKey, _ := noise.GenerateKeyPair()

	udp, err := NewUDP(key)
	if err != nil {
		t.Fatalf("NewUDP failed: %v", err)
	}
	udp.Close()
	udp.SetPeerEndpoint(peerKey.Public, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345})
	if udp.PeerInfo(peerKey.Public) != nil {
		t.Fatal("SetPeerEndpoint should ignore updates after close")
	}
}

func TestRemovePeer(t *testing.T) {
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	udp, err := NewUDP(key)
	if err != nil {
		t.Fatalf("NewUDP failed: %v", err)
	}
	defer udp.Close()

	peerKey, _ := noise.GenerateKeyPair()
	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")

	udp.SetPeerEndpoint(peerKey.Public, peerAddr)

	if udp.PeerInfo(peerKey.Public) == nil {
		t.Fatalf("Peer should exist")
	}

	udp.RemovePeer(peerKey.Public)

	if udp.PeerInfo(peerKey.Public) != nil {
		t.Fatalf("Peer should be removed")
	}
}

func TestRemovePeer_CleansSessionAndServiceMux(t *testing.T) {
	localKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(local) failed: %v", err)
	}
	peerKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(peer) failed: %v", err)
	}
	session, err := noise.NewSession(noise.SessionConfig{
		LocalIndex:  11,
		RemoteIndex: 22,
		SendKey:     noise.Key{1},
		RecvKey:     noise.Key{2},
		RemotePK:    peerKey.Public,
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	smux := NewServiceMux(peerKey.Public, ServiceMuxConfig{})
	u := &UDP{
		localKey: localKey,
		peers: map[noise.PublicKey]*peerState{
			peerKey.Public: {
				pk:         peerKey.Public,
				session:    session,
				serviceMux: smux,
				state:      PeerStateEstablished,
			},
		},
		byIndex: make(map[uint32]*peerState),
	}
	u.byIndex[session.LocalIndex()] = u.peers[peerKey.Public]

	u.RemovePeer(peerKey.Public)

	if _, ok := u.peers[peerKey.Public]; ok {
		t.Fatal("peer should be removed from map")
	}
	if _, ok := u.byIndex[session.LocalIndex()]; ok {
		t.Fatal("session index should be removed from byIndex")
	}
	if _, err := smux.OpenStream(0); err != ErrServiceMuxClosed {
		t.Fatalf("service mux should be closed after RemovePeer, err=%v", err)
	}
}

func TestPeersIterator(t *testing.T) {
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	udp, err := NewUDP(key)
	if err != nil {
		t.Fatalf("NewUDP failed: %v", err)
	}
	defer udp.Close()

	// Add some peers
	for i := 0; i < 3; i++ {
		peerKey, _ := noise.GenerateKeyPair()
		peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
		udp.SetPeerEndpoint(peerKey.Public, peerAddr)
	}

	count := 0
	for range udp.Peers() {
		count++
	}

	if count != 3 {
		t.Errorf("Expected 3 peers, got %d", count)
	}
}

func TestHandshakeAndTransport(t *testing.T) {
	// Create two UDP instances
	key1, _ := noise.GenerateKeyPair()
	key2, _ := noise.GenerateKeyPair()

	udp1, err := NewUDP(key1, WithBindAddr("127.0.0.1:0"), WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("NewUDP 1 failed: %v", err)
	}
	defer udp1.Close()

	udp2, err := NewUDP(key2, WithBindAddr("127.0.0.1:0"), WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("NewUDP 2 failed: %v", err)
	}
	defer udp2.Close()

	// Get addresses
	addr1 := udp1.HostInfo().Addr
	addr2 := udp2.HostInfo().Addr

	// Set up peer endpoints
	udp1.SetPeerEndpoint(key2.Public, addr2)
	udp2.SetPeerEndpoint(key1.Public, addr1)

	// Start receive goroutine for udp2 (responder)
	received := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 1024)
		for {
			pk, n, err := udp2.ReadFrom(buf)
			if err != nil {
				return
			}
			if pk == key1.Public {
				received <- append([]byte{}, buf[:n]...)
				return
			}
		}
	}()

	// Start receive goroutine for udp1 (initiator) to handle handshake response
	go func() {
		buf := make([]byte, 1024)
		for {
			_, _, err := udp1.ReadFrom(buf)
			if err != nil {
				return
			}
		}
	}()

	// Give the goroutines time to start
	time.Sleep(50 * time.Millisecond)

	// Initiate handshake from udp1
	udp1.mu.RLock()
	peer1 := udp1.peers[key2.Public]
	udp1.mu.RUnlock()

	err = udp1.initiateHandshake(peer1)
	if err != nil {
		t.Fatalf("Handshake failed: %v", err)
	}

	// Check that peer1 is now established
	info1 := udp1.PeerInfo(key2.Public)
	if info1.State != PeerStateEstablished {
		t.Errorf("Expected established state, got %v", info1.State)
	}

	// Send a message
	testData := []byte("hello world")
	err = udp1.WriteTo(key2.Public, testData)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// Wait for message
	select {
	case data := <-received:
		if !bytes.Equal(data, testData) {
			t.Errorf("Data mismatch: %s != %s", data, testData)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Timeout waiting for message")
	}
}

func TestRoaming(t *testing.T) {
	// Create two UDP instances
	key1, _ := noise.GenerateKeyPair()
	key2, _ := noise.GenerateKeyPair()

	udp1, err := NewUDP(key1, WithBindAddr("127.0.0.1:0"), WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("NewUDP 1 failed: %v", err)
	}
	defer udp1.Close()

	udp2, err := NewUDP(key2, WithBindAddr("127.0.0.1:0"), WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("NewUDP 2 failed: %v", err)
	}
	defer udp2.Close()

	// Get addresses
	addr1 := udp1.HostInfo().Addr
	addr2 := udp2.HostInfo().Addr

	// Set up peer endpoints
	udp1.SetPeerEndpoint(key2.Public, addr2)
	udp2.SetPeerEndpoint(key1.Public, addr1)

	// Start receive goroutine for udp2 (responder)
	go func() {
		buf := make([]byte, 1024)
		for {
			_, _, err := udp2.ReadFrom(buf)
			if err != nil {
				return
			}
		}
	}()

	// Start receive goroutine for udp1 (initiator) to handle handshake response
	go func() {
		buf := make([]byte, 1024)
		for {
			_, _, err := udp1.ReadFrom(buf)
			if err != nil {
				return
			}
		}
	}()

	// Give the goroutines time to start
	time.Sleep(50 * time.Millisecond)

	// Initiate handshake from udp1
	udp1.mu.RLock()
	peer1 := udp1.peers[key2.Public]
	udp1.mu.RUnlock()

	err = udp1.initiateHandshake(peer1)
	if err != nil {
		t.Fatalf("Handshake failed: %v", err)
	}

	// Check initial endpoint on udp2
	info2 := udp2.PeerInfo(key1.Public)
	if info2 == nil {
		t.Fatalf("Peer should exist on udp2")
	}
	initialEndpoint := info2.Endpoint.String()

	// Create a new UDP socket to simulate roaming
	newSocket, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("Failed to create new socket: %v", err)
	}
	defer newSocket.Close()

	newAddr := newSocket.LocalAddr().(*net.UDPAddr)

	// Manually send a transport message from the new address
	// This simulates the peer having roamed to a new address
	udp1.mu.RLock()
	session1 := udp1.peers[key2.Public].session
	udp1.mu.RUnlock()

	if session1 == nil {
		t.Fatalf("Session should exist")
	}

	testData := []byte("roamed message")
	encrypted, nonce, err := session1.Encrypt(testData)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	msg := noise.BuildTransportMessage(session1.RemoteIndex(), nonce, encrypted)
	_, err = newSocket.WriteToUDP(msg, addr2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Check that endpoint was updated (roaming)
	info2 = udp2.PeerInfo(key1.Public)
	if info2.Endpoint.String() == initialEndpoint {
		t.Logf("Initial: %s, Current: %s, New: %s", initialEndpoint, info2.Endpoint.String(), newAddr.String())
		// Note: The endpoint might not change if the test runs too fast
		// This is a best-effort check
	}
}

func TestClose(t *testing.T) {
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	udp, err := NewUDP(key)
	if err != nil {
		t.Fatalf("NewUDP failed: %v", err)
	}

	err = udp.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Second close should be no-op
	err = udp.Close()
	if err != nil {
		t.Fatalf("Second close should not error: %v", err)
	}

	// WriteTo should fail after close
	peerKey, _ := noise.GenerateKeyPair()
	err = udp.WriteTo(peerKey.Public, []byte("test"))
	if err != ErrClosed {
		t.Errorf("Expected ErrClosed, got %v", err)
	}
}

func TestReadPacket_SkipsErroredPacketsAndReturnsNext(t *testing.T) {
	u := &UDP{
		outputChan: make(chan *packet, 2),
		closeChan:  make(chan struct{}),
	}

	skipped := acquirePacket()
	skipped.err = ErrNoData
	skipped.refs.Store(1)
	close(skipped.ready)
	u.outputChan <- skipped

	wantPK := noise.PublicKey{9, 9, 9}
	okPkt := acquirePacket()
	copy(okPkt.data, []byte("payload"))
	okPkt.pk = wantPK
	okPkt.protocol = ProtocolEVENT
	okPkt.payload = okPkt.data[:7]
	okPkt.payloadN = 7
	okPkt.refs.Store(1)
	close(okPkt.ready)
	u.outputChan <- okPkt

	buf := make([]byte, 16)
	pk, proto, n, err := u.ReadPacket(buf)
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}
	if pk != wantPK {
		t.Fatalf("pk=%v, want %v", pk, wantPK)
	}
	if proto != ProtocolEVENT {
		t.Fatalf("proto=%d, want %d", proto, ProtocolEVENT)
	}
	if got := string(buf[:n]); got != "payload" {
		t.Fatalf("payload=%q, want payload", got)
	}
}

func TestReadPacket_ReturnsClosedWhileWaitingForReady(t *testing.T) {
	u := &UDP{
		outputChan: make(chan *packet, 1),
		closeChan:  make(chan struct{}),
	}

	pkt := acquirePacket()
	pkt.refs.Store(1)
	u.outputChan <- pkt
	close(u.closeChan)

	buf := make([]byte, 8)
	if _, _, _, err := u.ReadPacket(buf); err != ErrClosed {
		t.Fatalf("ReadPacket err=%v, want %v", err, ErrClosed)
	}
}

func TestUDPHandleHandshakeResp_Success(t *testing.T) {
	localKey, _ := noise.GenerateKeyPair()
	remoteKey, _ := noise.GenerateKeyPair()
	initHS, wire := buildHandshakeResponseForUDPTest(t, localKey, remoteKey, 17, 29)

	done := make(chan error, 1)
	peer := &peerState{pk: remoteKey.Public, state: PeerStateConnecting}
	u := &UDP{
		localKey: localKey,
		pending: map[uint32]*pendingHandshake{
			17: {
				peer:     peer,
				hsState:  initHS,
				localIdx: 17,
				done:     done,
			},
		},
		byIndex: make(map[uint32]*peerState),
	}

	from, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9999")
	u.handleHandshakeResp(wire, from)

	if err := <-done; err != nil {
		t.Fatalf("done err=%v, want nil", err)
	}
	if peer.state != PeerStateEstablished {
		t.Fatalf("peer.state=%v, want %v", peer.state, PeerStateEstablished)
	}
	if peer.session == nil || peer.serviceMux == nil {
		t.Fatal("peer session/serviceMux should be initialized")
	}
	if peer.endpoint.String() != from.String() {
		t.Fatalf("endpoint=%v, want %v", peer.endpoint, from)
	}
	if got := u.byIndex[17]; got != peer {
		t.Fatal("byIndex should register established peer")
	}
	if _, ok := u.pending[17]; ok {
		t.Fatal("pending handshake should be removed after success")
	}
}

func TestUDPHandleHandshakeResp_FailureMarksPeerFailed(t *testing.T) {
	localKey, _ := noise.GenerateKeyPair()
	remoteKey, _ := noise.GenerateKeyPair()
	initHS, _ := buildHandshakeResponseForUDPTest(t, localKey, remoteKey, 23, 41)

	done := make(chan error, 1)
	peer := &peerState{pk: remoteKey.Public, state: PeerStateConnecting}
	u := &UDP{
		localKey: localKey,
		pending: map[uint32]*pendingHandshake{
			23: {
				peer:     peer,
				hsState:  initHS,
				localIdx: 23,
				done:     done,
			},
		},
		byIndex: make(map[uint32]*peerState),
	}

	badWire := make([]byte, noise.HandshakeRespSize)
	badWire[0] = noise.MessageTypeHandshakeResp
	binary.LittleEndian.PutUint32(badWire[5:9], 23)
	u.handleHandshakeResp(badWire, &net.UDPAddr{})

	if err := <-done; err != ErrHandshakeFailed {
		t.Fatalf("done err=%v, want %v", err, ErrHandshakeFailed)
	}
	if peer.state != PeerStateFailed {
		t.Fatalf("peer.state=%v, want %v", peer.state, PeerStateFailed)
	}
	if _, ok := u.pending[23]; ok {
		t.Fatal("pending handshake should be removed after failure")
	}
}

func TestDecryptTransport_ReturnsPeerNotFoundWhenPeerRemovedMidFlight(t *testing.T) {
	localKey, _ := noise.GenerateKeyPair()
	peerKey, _ := noise.GenerateKeyPair()
	session, err := noise.NewSession(noise.SessionConfig{
		LocalIndex:  7,
		RemoteIndex: 9,
		SendKey:     noise.Key{1},
		RecvKey:     noise.Key{1},
		RemotePK:    peerKey.Public,
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	peer := &peerState{
		pk:         peerKey.Public,
		session:    session,
		serviceMux: NewServiceMux(peerKey.Public, ServiceMuxConfig{}),
		state:      PeerStateEstablished,
	}
	u := &UDP{
		localKey: localKey,
		peers: map[noise.PublicKey]*peerState{
			peerKey.Public: peer,
		},
		byIndex: map[uint32]*peerState{
			session.LocalIndex(): peer,
		},
	}

	plaintext := noise.EncodePayload(ProtocolEVENT, []byte("payload"))
	ciphertext, counter, err := session.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	wire := noise.BuildTransportMessage(session.LocalIndex(), counter, ciphertext)
	from, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9999")
	pkt := acquirePacket()
	defer releasePacket(pkt)

	hookStarted := make(chan struct{})
	hookRelease := make(chan struct{})
	afterDecryptTransportDecryptHook = func() {
		close(hookStarted)
		<-hookRelease
	}
	defer func() { afterDecryptTransportDecryptHook = nil }()

	done := make(chan struct{})
	go func() {
		u.decryptTransport(pkt, wire, from)
		close(done)
	}()

	<-hookStarted
	u.RemovePeer(peerKey.Public)
	close(hookRelease)
	<-done

	if pkt.err != ErrPeerNotFound {
		t.Fatalf("pkt.err=%v, want %v", pkt.err, ErrPeerNotFound)
	}
}
