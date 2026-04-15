package giznet_test

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/giznet"
)

func TestPublicSmokeDatagramAndStream(t *testing.T) {
	serverKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	server := newBenchUDPNode(t, serverKey)
	client := newBenchUDPNode(t, clientKey)
	connectBenchNodes(t, client, clientKey, server, serverKey)

	msg := []byte("smoke-datagram")
	if _, err := mustPeerBenchMux(t, client, serverKey.Public).Write(benchProtoEvent, msg); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 4096)
	proto, n, err := mustPeerBenchMux(t, server, clientKey.Public).Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if proto != benchProtoEvent || !bytes.Equal(buf[:n], msg) {
		t.Fatalf("datagram mismatch proto=%d payload=%q", proto, buf[:n])
	}

	serverMux := mustPeerBenchMux(t, server, clientKey.Public)
	acceptCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		s, err := serverMux.AcceptStream(0)
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- s
	}()

	clientStream, err := mustPeerBenchMux(t, client, serverKey.Public).OpenStream(0)
	if err != nil {
		t.Fatal(err)
	}
	defer clientStream.Close()

	var serverStream net.Conn
	select {
	case serverStream = <-acceptCh:
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("AcceptStream timeout")
	}
	defer serverStream.Close()

	streamMsg := []byte("smoke-stream")
	if _, err := clientStream.Write(streamMsg); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(streamMsg))
	if _, err := io.ReadFull(serverStream, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, streamMsg) {
		t.Fatalf("stream payload mismatch: %q vs %q", got, streamMsg)
	}
}
