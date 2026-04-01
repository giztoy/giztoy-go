package core

import (
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

// TestUDPStreamThroughput measures async throughput through a KCP stream over
// the full UDP transport layer (Noise encryption + real UDP sockets).
// Run with: -test.run=TestUDPStreamThroughput (skipped in short mode)
func TestUDPStreamThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping throughput test in short mode")
	}
	const totalSize = 32 * 1024 * 1024 // 32 MB
	const chunkSize = 8 * 1024         // 8 KB

	clientKey, _ := noise.GenerateKeyPair()
	serverKey, _ := noise.GenerateKeyPair()

	client, err := NewUDP(clientKey, WithBindAddr("127.0.0.1:0"), WithAllowUnknown(true))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	server, err := NewUDP(serverKey, WithBindAddr("127.0.0.1:0"), WithAllowUnknown(true))
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	client.SetPeerEndpoint(serverKey.Public, server.HostInfo().Addr)
	server.SetPeerEndpoint(clientKey.Public, client.HostInfo().Addr)

	// Drain non-KCP packets (required for UDP to process handshake/KCP)
	go func() {
		buf := make([]byte, 65535)
		for {
			if _, _, err := client.ReadFrom(buf); err != nil {
				return
			}
		}
	}()
	go func() {
		buf := make([]byte, 65535)
		for {
			if _, _, err := server.ReadFrom(buf); err != nil {
				return
			}
		}
	}()

	if err := client.Connect(serverKey.Public); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	chunk := make([]byte, chunkSize)
	for i := range chunk {
		chunk[i] = byte(i & 0xFF)
	}

	serverStreamCh := make(chan io.ReadWriteCloser, 1)
	serverErrCh := make(chan error, 1)
	go func() {
		s, err := mustServiceMux(t, server, clientKey.Public).AcceptStream(0)
		if err != nil {
			serverErrCh <- err
			return
		}
		serverStreamCh <- s
	}()

	time.Sleep(50 * time.Millisecond)

	clientStream, err := mustServiceMux(t, client, serverKey.Public).OpenStream(0)
	if err != nil {
		t.Fatal(err)
	}

	written := 0
	n, err := clientStream.Write(chunk)
	if err != nil {
		t.Fatalf("initial write error: %v", err)
	}
	written += n

	var serverStream io.ReadWriteCloser
	select {
	case serverStream = <-serverStreamCh:
	case err := <-serverErrCh:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for AcceptStream")
	}

	var wg sync.WaitGroup
	var readBytes int

	// Reader
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 64*1024)
		for readBytes < totalSize {
			n, err := serverStream.Read(buf)
			if err != nil {
				t.Errorf("read error: %v", err)
				return
			}
			readBytes += n
		}
	}()

	// Writer
	start := time.Now()
	for written < totalSize {
		n, err := clientStream.Write(chunk)
		if err != nil {
			t.Fatalf("write error at %d: %v", written, err)
		}
		written += n
	}

	// Wait for reader
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatalf("timeout: wrote %d, read %d", written, readBytes)
	}

	elapsed := time.Since(start)
	mbps := float64(readBytes) / elapsed.Seconds() / (1024 * 1024)
	t.Logf("Layer 2 (UDP + Noise): %d bytes in %s = %.1f MB/s", readBytes, elapsed.Round(time.Millisecond), mbps)
	fmt.Printf("THROUGHPUT_UDP=%.1f\n", mbps)
}
