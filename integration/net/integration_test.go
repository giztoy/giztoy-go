package net_test

import (
	"bytes"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/integration/testutil"
	gnet "github.com/haivivi/giztoy/go/pkg/net/core"
	"github.com/haivivi/giztoy/go/pkg/net/noise"
)

// TestIntegration_ConnectionPoolCapacity 测试连接池容量
// 验证服务器能同时处理 64 个并发 peer 连接
func TestIntegration_ConnectionPoolCapacity(t *testing.T) {
	const peerCount = 64

	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate server key failed: %v", err)
	}
	server := testutil.NewUDPNode(t, serverKey)
	defer server.Close()

	clients := make([]*gnet.UDP, 0, peerCount)
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

	for i := 0; i < peerCount; i++ {
		msg := []byte(fmt.Sprintf("peer-%02d", i))
		n, err := clients[i].Write(serverKey.Public, noise.ProtocolEVENT, msg)
		if err != nil {
			t.Fatalf("client[%d] Write failed: %v", i, err)
		}
		if n != len(msg) {
			t.Fatalf("client[%d] Write bytes=%d, want %d", i, n, len(msg))
		}

		proto, got := testutil.ReadFromPeerWithTimeout(t, server, clientKeys[i].Public, 3*time.Second)
		if proto != noise.ProtocolEVENT {
			t.Fatalf("server received proto=%d from client[%d], want %d", proto, i, noise.ProtocolEVENT)
		}
		if !bytes.Equal(got, msg) {
			t.Fatalf("server payload mismatch from client[%d]: got=%q want=%q", i, string(got), string(msg))
		}
	}
}

// TestIntegration_NetworkInterruptionReconnect 测试网络中断与重连
// 验证同 key 新 endpoint 能成功重连，服务器能更新 endpoint 信息
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
	if _, err := clientV2.Write(serverKey.Public, noise.ProtocolEVENT, msg); err != nil {
		t.Fatalf("clientV2 Write failed: %v", err)
	}

	proto, got := testutil.ReadFromPeerWithTimeout(t, server, clientKey.Public, 3*time.Second)
	if proto != noise.ProtocolEVENT {
		t.Fatalf("server proto after reconnect=%d, want %d", proto, noise.ProtocolEVENT)
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

// TestIntegration_KCPService0Stream 测试 KCP 基础 stream 功能
// 验证 service=0 时可以正常建立 stream 并进行双向通信
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
		stream, service, err := server.AcceptStream(clientKey.Public)
		if err != nil {
			errCh <- err
			return
		}
		if service != 0 {
			errCh <- gnet.ErrUnsupportedService
			_ = stream.Close()
			return
		}
		acceptCh <- stream
	}()

	clientStream, err := client.OpenStream(serverKey.Public, 0)
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

// TestIntegration_KCPStreamActiveClose 测试主动关闭快速感知
// 验证一端 Close 后，另一端阻塞中的 stream Read 能在短时间内失败返回
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
		stream, service, acceptErr := server.AcceptStream(clientKey.Public)
		if acceptErr != nil {
			errCh <- acceptErr
			return
		}
		if service != 0 {
			errCh <- gnet.ErrUnsupportedService
			_ = stream.Close()
			return
		}
		acceptCh <- stream
	}()

	clientStream, err := client.OpenStream(serverKey.Public, 0)
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
	case acceptErr := <-errCh:
		t.Fatalf("server AcceptStream failed: %v", acceptErr)
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

// TestIntegration_KCPRejectNonZeroService 测试 foundation 层拒绝非零 service
// 验证 service != 0 时返回 ErrUnsupportedService
func TestIntegration_KCPRejectNonZeroService(t *testing.T) {
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

	if _, err := client.OpenStream(serverKey.Public, 7); err != gnet.ErrUnsupportedService {
		t.Fatalf("OpenStream(non-zero service) err=%v, want %v", err, gnet.ErrUnsupportedService)
	}
}

// TestIntegration_RPCBidirectionalOverKCPStream 测试 RPC 双向请求响应
// 验证 A->B 与 B->A 各一次调用都能正常完成 req/resp
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

	assertRPC := func(caller, callee *gnet.UDP, callerPK, calleePK noise.PublicKey, req, resp []byte) {
		t.Helper()

		acceptCh := make(chan net.Conn, 1)
		errCh := make(chan error, 1)
		go func() {
			stream, service, err := callee.AcceptStream(callerPK)
			if err != nil {
				errCh <- err
				return
			}
			if service != 0 {
				errCh <- gnet.ErrUnsupportedService
				_ = stream.Close()
				return
			}
			acceptCh <- stream
		}()

		callerStream, err := caller.OpenStream(calleePK, 0)
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

// TestIntegration_EVENTFireAndForgetBidirectional 测试 EVENT fire-and-forget 双向突发收发
// 验证双向各发 64 条事件不阻塞主链路，无应答依赖
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
		if _, err := client.Write(serverKey.Public, noise.ProtocolEVENT, msg); err != nil {
			t.Fatalf("client EVENT write[%d] failed: %v", i, err)
		}
	}
	for i := 0; i < burst; i++ {
		msg := []byte(fmt.Sprintf("event-s2c-%02d", i))
		if _, err := server.Write(clientKey.Public, noise.ProtocolEVENT, msg); err != nil {
			t.Fatalf("server EVENT write[%d] failed: %v", i, err)
		}
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("EVENT fire-and-forget writes took too long: %s", took)
	}

	for i := 0; i < burst; i++ {
		want := []byte(fmt.Sprintf("event-c2s-%02d", i))
		proto, got := testutil.ReadFromPeerWithTimeout(t, server, clientKey.Public, 3*time.Second)
		if proto != noise.ProtocolEVENT {
			t.Fatalf("server EVENT proto[%d]=%d, want %d", i, proto, noise.ProtocolEVENT)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("server EVENT payload[%d] mismatch: got=%q want=%q", i, string(got), string(want))
		}
	}

	for i := 0; i < burst; i++ {
		want := []byte(fmt.Sprintf("event-s2c-%02d", i))
		proto, got := testutil.ReadFromPeerWithTimeout(t, client, serverKey.Public, 3*time.Second)
		if proto != noise.ProtocolEVENT {
			t.Fatalf("client EVENT proto[%d]=%d, want %d", i, proto, noise.ProtocolEVENT)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("client EVENT payload[%d] mismatch: got=%q want=%q", i, string(got), string(want))
		}
	}
}

// TestIntegration_OPUSFramesOrdered 测试 OPUS 连续帧顺序性
// 验证连续发送 40 帧 OPUS 数据，接收端顺序可读，无协议误判
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
		if _, err := client.Write(serverKey.Public, noise.ProtocolOPUS, frame); err != nil {
			t.Fatalf("client OPUS write[%d] failed: %v", i, err)
		}
	}

	for i := 0; i < frames; i++ {
		want := []byte(fmt.Sprintf("opus-frame-%03d", i))
		proto, got := testutil.ReadFromPeerWithTimeout(t, server, clientKey.Public, 3*time.Second)
		if proto != noise.ProtocolOPUS {
			t.Fatalf("server OPUS proto[%d]=%d, want %d", i, proto, noise.ProtocolOPUS)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("server OPUS payload[%d] mismatch: got=%q want=%q", i, string(got), string(want))
		}
	}
}

// TestIntegration_WriteValidationPrecedesPeerLookup 测试 Write 的边界校验顺序
// 验证 RPC datagram/非法协议在查找 peer 之前就会被拒绝
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

	if _, err := u.Write(remoteKey.Public, noise.ProtocolRPC, []byte("rpc-over-datagram")); err != gnet.ErrRPCMustUseStream {
		t.Fatalf("Write(RPC datagram) err=%v, want %v", err, gnet.ErrRPCMustUseStream)
	}

	if _, err := u.Write(remoteKey.Public, 0x7f, []byte("unsupported")); err != gnet.ErrUnsupportedProtocol {
		t.Fatalf("Write(unsupported protocol) err=%v, want %v", err, gnet.ErrUnsupportedProtocol)
	}
}

// TestIntegration_UnknownPeerOperations 测试未知 peer 的错误语义
// 验证 Write/Read/OpenStream/AcceptStream 对未知 peer 返回 ErrPeerNotFound
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

	if _, err := u.Write(unknownKey.Public, noise.ProtocolEVENT, []byte("x")); err != gnet.ErrPeerNotFound {
		t.Fatalf("Write(unknown peer) err=%v, want %v", err, gnet.ErrPeerNotFound)
	}

	buf := make([]byte, 8)
	if _, _, err := u.Read(unknownKey.Public, buf); err != gnet.ErrPeerNotFound {
		t.Fatalf("Read(unknown peer) err=%v, want %v", err, gnet.ErrPeerNotFound)
	}

	if _, err := u.OpenStream(unknownKey.Public, 0); err != gnet.ErrPeerNotFound {
		t.Fatalf("OpenStream(unknown peer) err=%v, want %v", err, gnet.ErrPeerNotFound)
	}

	if _, _, err := u.AcceptStream(unknownKey.Public); err != gnet.ErrPeerNotFound {
		t.Fatalf("AcceptStream(unknown peer) err=%v, want %v", err, gnet.ErrPeerNotFound)
	}
}

// TestIntegration_StreamBeforeSession 测试会话未建立时的 stream 行为
// 验证 peer 已注册但未握手建立 session 时返回 ErrNoSession
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

	// 仅注册 endpoint，不执行 Connect（因此 peer 存在但 session 未建立）
	client.SetPeerEndpoint(serverKey.Public, server.HostInfo().Addr)
	server.SetPeerEndpoint(clientKey.Public, client.HostInfo().Addr)

	if _, err := client.OpenStream(serverKey.Public, 0); err != gnet.ErrNoSession {
		t.Fatalf("OpenStream(before session) err=%v, want %v", err, gnet.ErrNoSession)
	}

	if _, _, err := server.AcceptStream(clientKey.Public); err != gnet.ErrNoSession {
		t.Fatalf("AcceptStream(before session) err=%v, want %v", err, gnet.ErrNoSession)
	}
}

// TestIntegration_ClosedNodeOperations 测试关闭节点后的 API 行为
// 验证 Read/Write/OpenStream/AcceptStream 一致返回 ErrClosed
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

	if _, err := u.Write(peerKey.Public, noise.ProtocolEVENT, []byte("x")); err != gnet.ErrClosed {
		t.Fatalf("Write(after close) err=%v, want %v", err, gnet.ErrClosed)
	}

	if _, err := u.OpenStream(peerKey.Public, 0); err != gnet.ErrClosed {
		t.Fatalf("OpenStream(after close) err=%v, want %v", err, gnet.ErrClosed)
	}

	if _, _, err := u.AcceptStream(peerKey.Public); err != gnet.ErrClosed {
		t.Fatalf("AcceptStream(after close) err=%v, want %v", err, gnet.ErrClosed)
	}

	buf := make([]byte, 8)
	if _, _, err := u.Read(peerKey.Public, buf); err != gnet.ErrClosed {
		t.Fatalf("Read(after close) err=%v, want %v", err, gnet.ErrClosed)
	}
}

// TestIntegration_ZeroLengthPayloads 测试零长度 payload 边界
// 验证 EVENT/OPUS 支持空 payload 且接收侧可正确识别协议号
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
		{name: "EVENT empty payload", proto: noise.ProtocolEVENT},
		{name: "OPUS empty payload", proto: noise.ProtocolOPUS},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, err := client.Write(serverKey.Public, tc.proto, nil)
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
