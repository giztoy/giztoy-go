package giznet_test

import (
	"io"
	"net"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/giznet"
)

const benchProtoEvent byte = 0x03

func BenchmarkPublicDatagramWrite(b *testing.B) {
	serverKey, err := giznet.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	clientKey, err := giznet.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	server := newBenchUDPNode(b, serverKey)
	client := newBenchUDPNode(b, clientKey)
	connectBenchNodes(b, client, clientKey, server, serverKey)

	payload := make([]byte, 256)
	for i := range payload {
		payload[i] = byte(i)
	}

	clientMux := mustPeerBenchMux(b, client, serverKey.Public)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := clientMux.Write(benchProtoEvent, payload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPublicDatagramRead(b *testing.B) {
	serverKey, err := giznet.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	clientKey, err := giznet.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	server := newBenchUDPNode(b, serverKey)
	client := newBenchUDPNode(b, clientKey)
	connectBenchNodes(b, client, clientKey, server, serverKey)

	serverMux := mustPeerBenchMux(b, server, clientKey.Public)
	clientMux := mustPeerBenchMux(b, client, serverKey.Public)

	payload := make([]byte, 256)
	for i := range payload {
		payload[i] = byte(i)
	}

	// Prime one read worth of data before timing.
	if _, err := clientMux.Write(benchProtoEvent, payload); err != nil {
		b.Fatal(err)
	}

	buf := make([]byte, 65535)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i > 0 {
			if _, err := clientMux.Write(benchProtoEvent, payload); err != nil {
				b.Fatal(err)
			}
		}
		proto, n, err := serverMux.Read(buf)
		if err != nil {
			b.Fatal(err)
		}
		if proto != benchProtoEvent || n != len(payload) {
			b.Fatalf("unexpected read proto=%d n=%d", proto, n)
		}
	}
}

func BenchmarkPublicStreamEcho(b *testing.B) {
	serverKey, err := giznet.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}
	clientKey, err := giznet.GenerateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	server := newBenchUDPNode(b, serverKey)
	client := newBenchUDPNode(b, clientKey)
	connectBenchNodes(b, client, clientKey, server, serverKey)

	serverMux := mustPeerBenchMux(b, server, clientKey.Public)
	acceptCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		c, err := serverMux.AcceptStream(0)
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- c
	}()

	clientStream, err := mustPeerBenchMux(b, client, serverKey.Public).OpenStream(0)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = clientStream.Close() })

	var serverStream net.Conn
	select {
	case serverStream = <-acceptCh:
	case err := <-errCh:
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = serverStream.Close() })

	payload := make([]byte, 512)
	for i := range payload {
		payload[i] = byte(i)
	}
	readBuf := make([]byte, len(payload))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := clientStream.Write(payload); err != nil {
			b.Fatal(err)
		}
		if _, err := io.ReadFull(serverStream, readBuf); err != nil {
			b.Fatal(err)
		}
		if _, err := serverStream.Write(readBuf); err != nil {
			b.Fatal(err)
		}
		if _, err := io.ReadFull(clientStream, readBuf); err != nil {
			b.Fatal(err)
		}
	}
}
