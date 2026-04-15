package giznet_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/audio/stampedopus"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
)

const (
	testServiceRPC   uint64 = 4
	testEventVersion int    = 1
)

var (
	errTestEventInvalidV    = errors.New("event: invalid version")
	errTestEventMissingName = errors.New("event: missing name")
)

type testEvent struct {
	Data *json.RawMessage `json:"data,omitempty"`
	Name string           `json:"name"`
	V    int              `json:"v"`
}

func (e testEvent) Validate() error {
	if e.V != testEventVersion {
		return errTestEventInvalidV
	}
	if strings.TrimSpace(e.Name) == "" {
		return errTestEventMissingName
	}
	return nil
}

// ConnectedPeerPair is two Listeners with an accepted server Conn and an
// established client Peer Conn (Listener / Conn public API).
type ConnectedPeerPair struct {
	ServerKey *giznet.KeyPair
	ClientKey *giznet.KeyPair

	ServerListener *giznet.Listener
	ClientListener *giznet.Listener

	ServerConn *giznet.Conn
	ClientConn *giznet.Conn
}

// NewConnectedPeerPair connects a client Listener to a server Listener and
// waits for server Accept plus client Peer.
func NewConnectedPeerPair(t *testing.T) *ConnectedPeerPair {
	t.Helper()

	serverKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	clientKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate client key failed: %v", err)
	}

	serverListener := NewTestListener(t, serverKey)
	clientListener := NewTestListener(t, clientKey)

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

	return &ConnectedPeerPair{
		ServerKey: serverKey,
		ClientKey: clientKey,

		ServerListener: serverListener,
		ClientListener: clientListener,

		ServerConn: serverConn,
		ClientConn: clientConn,
	}
}

// Close closes both listeners.
func (p *ConnectedPeerPair) Close() {
	if p == nil {
		return
	}
	if p.ServerListener != nil {
		_ = p.ServerListener.Close()
	}
	if p.ClientListener != nil {
		_ = p.ClientListener.Close()
	}
}

// NewTestListener returns a Listener on loopback with AllowUnknown and a UDP read loop.
func NewTestListener(t *testing.T, key *giznet.KeyPair) *giznet.Listener {
	t.Helper()
	l, err := giznet.Listen(key, giznet.WithBindAddr("127.0.0.1:0"), giznet.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	startReadLoop(l.UDP())
	return l
}

// ConnectTestListeners wires endpoints and runs client Connect to server.
func ConnectTestListeners(t *testing.T, client *giznet.Listener, clientKey *giznet.KeyPair, server *giznet.Listener, serverKey *giznet.KeyPair) {
	t.Helper()
	client.SetPeerEndpoint(serverKey.Public, server.HostInfo().Addr)
	server.SetPeerEndpoint(clientKey.Public, client.HostInfo().Addr)
	if err := client.Connect(serverKey.Public); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
}

func startReadLoop(u *giznet.UDP) {
	go func() {
		buf := make([]byte, 65535)
		for {
			if _, _, err := u.ReadFrom(buf); err != nil {
				return
			}
		}
	}()
}

// AcceptConnWithTimeout calls Listener.Accept with a timeout.
func AcceptConnWithTimeout(l *giznet.Listener, timeout time.Duration) (*giznet.Conn, error) {
	type result struct {
		conn *giznet.Conn
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

func writeTestEvent(c *giznet.Conn, evt testEvent) error {
	if err := evt.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	_, err = c.Write(testProtocolEvent, payload)
	return err
}

func decodeTestEvent(payload []byte) (testEvent, error) {
	var evt testEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return testEvent{}, err
	}
	if err := evt.Validate(); err != nil {
		return testEvent{}, err
	}
	return evt, nil
}

func readTestEvent(c *giznet.Conn) (testEvent, error) {
	proto, payload, err := readPacketWithTimeout(c, 5*time.Second)
	if err != nil {
		return testEvent{}, err
	}
	if proto != testProtocolEvent {
		return testEvent{}, fmt.Errorf("unexpected protocol %d", proto)
	}
	return decodeTestEvent(payload)
}

func writeTestOpusFrame(c *giznet.Conn, frame []byte) error {
	if _, _, ok := stampedopus.Unpack(frame); !ok {
		return errors.New("invalid stamped opus frame")
	}
	_, err := c.Write(testProtocolOpus, frame)
	return err
}

func readTestOpusFrame(c *giznet.Conn) (uint64, []byte, error) {
	proto, payload, err := readPacketWithTimeout(c, 5*time.Second)
	if err != nil {
		return 0, nil, err
	}
	if proto != testProtocolOpus {
		return 0, nil, fmt.Errorf("unexpected protocol %d", proto)
	}
	ts, frame, ok := stampedopus.Unpack(payload)
	if !ok {
		return 0, nil, errors.New("invalid stamped opus frame")
	}
	return ts, frame, nil
}

func readPacketWithTimeout(c *giznet.Conn, timeout time.Duration) (byte, []byte, error) {
	type result struct {
		proto byte
		data  []byte
		err   error
	}

	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, 65535)
		proto, n, err := c.Read(buf)
		if err != nil {
			ch <- result{err: err}
			return
		}
		payload := append([]byte(nil), buf[:n]...)
		ch <- result{proto: proto, data: payload}
	}()

	select {
	case r := <-ch:
		return r.proto, r.data, r.err
	case <-time.After(timeout):
		return 0, nil, errors.New("read packet timeout")
	}
}

func readEventWithTimeout(c *giznet.Conn, timeout time.Duration) (testEvent, error) {
	type result struct {
		evt testEvent
		err error
	}

	ch := make(chan result, 1)
	go func() {
		evt, err := readTestEvent(c)
		ch <- result{evt: evt, err: err}
	}()

	select {
	case r := <-ch:
		return r.evt, r.err
	case <-time.After(timeout):
		return testEvent{}, errors.New("read event timeout")
	}
}

// peerMux captures the per-peer mux surface used by integration tests without
// referencing internal packages.
type peerMux interface {
	Write(protocol byte, data []byte) (n int, err error)
	Read(buf []byte) (protocol byte, n int, err error)
	OpenStream(service uint64) (net.Conn, error)
	AcceptStream(service uint64) (net.Conn, error)
}

// NewUDPNode returns a UDP node backed by giznet.Listen (public API).
func NewUDPNode(t *testing.T, key *giznet.KeyPair) *giznet.UDP {
	t.Helper()

	l, err := giznet.Listen(key,
		giznet.WithBindAddr("127.0.0.1:0"),
		giznet.WithAllowUnknown(true),
		giznet.WithDecryptWorkers(1),
	)
	if err != nil {
		t.Fatalf("giznet.Listen failed: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	u := l.UDP()
	go func() {
		buf := make([]byte, 65535)
		for {
			if _, _, err := u.ReadFrom(buf); err != nil {
				return
			}
		}
	}()

	return u
}

// ConnectNodes establishes a connection between two UDP nodes.
func ConnectNodes(t *testing.T, client *giznet.UDP, clientKey *giznet.KeyPair, server *giznet.UDP, serverKey *giznet.KeyPair) {
	t.Helper()

	client.SetPeerEndpoint(serverKey.Public, server.HostInfo().Addr)
	server.SetPeerEndpoint(clientKey.Public, client.HostInfo().Addr)

	if err := client.Connect(serverKey.Public); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	waitForPeerEstablished(t, client, serverKey.Public)
	waitForPeerEstablished(t, server, clientKey.Public)
}

func waitForPeerEstablished(t *testing.T, u *giznet.UDP, pk giznet.PublicKey) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		info := u.PeerInfo(pk)
		if info != nil && info.State == giznet.PeerStateEstablished {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	info := u.PeerInfo(pk)
	if info == nil {
		t.Fatalf("peer %x was not registered before timeout", pk)
	}
	t.Fatalf("peer %x state=%v, want %v", pk, info.State, giznet.PeerStateEstablished)
}

// MustPeerMux returns the service mux for an established peer session.
func MustPeerMux(t *testing.T, u *giznet.UDP, pk giznet.PublicKey) peerMux {
	t.Helper()

	smux, err := u.GetServiceMux(pk)
	if err != nil {
		t.Fatalf("GetServiceMux failed: %v", err)
	}
	return smux
}

// ReadFromPeerWithTimeout reads a datagram from the specified peer with timeout.
func ReadFromPeerWithTimeout(t *testing.T, u *giznet.UDP, pk giznet.PublicKey, timeout time.Duration) (byte, []byte) {
	t.Helper()

	type result struct {
		proto   byte
		payload []byte
		err     error
	}

	smux := MustPeerMux(t, u, pk)
	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, 65535)
		proto, n, err := smux.Read(buf)
		if err != nil {
			ch <- result{err: err}
			return
		}
		payload := make([]byte, n)
		copy(payload, buf[:n])
		ch <- result{proto: proto, payload: payload}
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

// ReadExactWithTimeout reads exactly n bytes with timeout.
func ReadExactWithTimeout(t *testing.T, r io.Reader, n int, timeout time.Duration) []byte {
	t.Helper()

	type result struct {
		buf []byte
		err error
	}

	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, n)
		_, err := io.ReadFull(r, buf)
		ch <- result{buf: buf, err: err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("ReadFull failed: %v", r.err)
		}
		return r.buf
	case <-time.After(timeout):
		t.Fatalf("ReadFull timeout after %s", timeout)
		return nil
	}
}
