// Package testutil 提供集成测试的公共辅助函数
package testutil

import (
	"io"
	"net"
	"testing"
	"time"

	gnet "github.com/giztoy/giztoy-go/pkg/net/core"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

// NewUDPNode 创建一个新的 UDP 节点用于测试
func NewUDPNode(t *testing.T, key *noise.KeyPair) *gnet.UDP {
	t.Helper()

	u, err := gnet.NewUDP(
		key,
		gnet.WithBindAddr("127.0.0.1:0"),
		gnet.WithAllowUnknown(true),
		gnet.WithDecryptWorkers(1),
	)
	if err != nil {
		t.Fatalf("NewUDP failed: %v", err)
	}
	t.Cleanup(func() { u.Close() })

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

// NewListenerNode creates a peer.Listener for testing. A background read loop
// is started on the underlying UDP so internal buffers don't fill up.
func NewListenerNode(t *testing.T, key *noise.KeyPair, opts ...gnet.Option) *peer.Listener {
	t.Helper()

	defaults := []gnet.Option{
		gnet.WithBindAddr("127.0.0.1:0"),
		gnet.WithAllowUnknown(true),
		gnet.WithDecryptWorkers(1),
	}
	l, err := peer.Listen(key, append(defaults, opts...)...)
	if err != nil {
		t.Fatalf("peer.Listen failed: %v", err)
	}
	t.Cleanup(func() { l.Close() })

	u := l.UDP()
	go func() {
		buf := make([]byte, 65535)
		for {
			if _, _, err := u.ReadFrom(buf); err != nil {
				return
			}
		}
	}()

	return l
}

// ConnectListenerNodes establishes a connection between two Listener nodes.
func ConnectListenerNodes(t *testing.T, client *peer.Listener, clientKey *noise.KeyPair, server *peer.Listener, serverKey *noise.KeyPair) {
	t.Helper()

	client.SetPeerEndpoint(serverKey.Public, server.HostInfo().Addr)
	server.SetPeerEndpoint(clientKey.Public, client.HostInfo().Addr)

	if err := client.Connect(serverKey.Public); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	waitForPeerEstablished(t, client.UDP(), serverKey.Public)
	waitForPeerEstablished(t, server.UDP(), clientKey.Public)
}

// ConnectNodes 建立两个 UDP 节点之间的连接
func ConnectNodes(t *testing.T, client *gnet.UDP, clientKey *noise.KeyPair, server *gnet.UDP, serverKey *noise.KeyPair) {
	t.Helper()

	client.SetPeerEndpoint(serverKey.Public, server.HostInfo().Addr)
	server.SetPeerEndpoint(clientKey.Public, client.HostInfo().Addr)

	if err := client.Connect(serverKey.Public); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	waitForPeerEstablished(t, client, serverKey.Public)
	waitForPeerEstablished(t, server, clientKey.Public)
}

func waitForPeerEstablished(t *testing.T, u *gnet.UDP, pk noise.PublicKey) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		info := u.PeerInfo(pk)
		if info != nil && info.State == gnet.PeerStateEstablished {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	info := u.PeerInfo(pk)
	if info == nil {
		t.Fatalf("peer %x was not registered before timeout", pk)
	}
	t.Fatalf("peer %x state=%v, want %v", pk, info.State, gnet.PeerStateEstablished)
}

func MustServiceMux(t *testing.T, u *gnet.UDP, pk noise.PublicKey) *gnet.ServiceMux {
	t.Helper()

	smux, err := u.GetServiceMux(pk)
	if err != nil {
		t.Fatalf("GetServiceMux failed: %v", err)
	}
	return smux
}

// ReadFromPeerWithTimeout 从指定 peer 读取数据（带超时）
func ReadFromPeerWithTimeout(t *testing.T, u *gnet.UDP, pk noise.PublicKey, timeout time.Duration) (byte, []byte) {
	t.Helper()

	type result struct {
		proto   byte
		payload []byte
		err     error
	}

	smux := MustServiceMux(t, u, pk)
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

func OpenStream(t *testing.T, u *gnet.UDP, pk noise.PublicKey, service uint64) net.Conn {
	t.Helper()
	stream, err := MustServiceMux(t, u, pk).OpenStream(service)
	if err != nil {
		t.Fatalf("OpenStream(service=%d) failed: %v", service, err)
	}
	return stream
}

func AcceptStream(t *testing.T, u *gnet.UDP, pk noise.PublicKey, service uint64) net.Conn {
	t.Helper()
	stream, err := MustServiceMux(t, u, pk).AcceptStream(service)
	if err != nil {
		t.Fatalf("AcceptStream(service=%d) failed: %v", service, err)
	}
	return stream
}

// ReadExactWithTimeout 从 reader 读取指定字节数（带超时）
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
