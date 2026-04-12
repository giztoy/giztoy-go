package scenarios

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"
)

// BenchmarkConnPingPongRTT benchmarks request/response RTT on reliable streams.
func BenchmarkConnPingPongRTT(b *testing.B, payload []byte, client io.ReadWriter, server io.ReadWriter) {
	b.Helper()

	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, len(payload))
		for i := 0; i < b.N; i++ {
			if _, err := io.ReadFull(server, buf); err != nil {
				errCh <- fmt.Errorf("server read[%d]: %w", i, err)
				return
			}
			if !bytes.Equal(buf, payload) {
				errCh <- fmt.Errorf("server read[%d] payload mismatch", i)
				return
			}
			if _, err := server.Write(payload); err != nil {
				errCh <- fmt.Errorf("server write[%d]: %w", i, err)
				return
			}
		}
		errCh <- nil
	}()

	recv := make([]byte, len(payload))
	b.SetBytes(int64(len(payload) * 2)) // req + resp
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.Write(payload); err != nil {
			b.Fatalf("client write[%d]: %v", i, err)
		}
		if _, err := io.ReadFull(client, recv); err != nil {
			b.Fatalf("client read[%d]: %v", i, err)
		}
		if !bytes.Equal(recv, payload) {
			b.Fatalf("client read[%d] payload mismatch", i)
		}
	}
	b.StopTimer()

	if err := <-errCh; err != nil {
		b.Fatal(err)
	}
}

// BenchmarkDatagramPingPongRTT benchmarks datagram RTT.
//
// This scenario assumes no/little loss; high loss can introduce retransmit/
// retry concerns that are outside simple ping-pong semantics.
func BenchmarkDatagramPingPongRTT(
	b *testing.B,
	payload []byte,
	clientSend DatagramSendFunc,
	clientRecv DatagramRecvFunc,
	serverSend DatagramSendFunc,
	serverRecv DatagramRecvFunc,
) {
	b.Helper()

	errCh := make(chan error, 1)
	go func() {
		for i := 0; i < b.N; i++ {
			pkt, err := serverRecv(5 * time.Second)
			if err != nil {
				errCh <- fmt.Errorf("server recv[%d]: %w", i, err)
				return
			}
			if !bytes.Equal(pkt, payload) {
				errCh <- fmt.Errorf("server recv[%d] payload mismatch", i)
				return
			}
			if err := serverSend(payload); err != nil {
				errCh <- fmt.Errorf("server send[%d]: %w", i, err)
				return
			}
		}
		errCh <- nil
	}()

	b.SetBytes(int64(len(payload) * 2))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := clientSend(payload); err != nil {
			b.Fatalf("client send[%d]: %v", i, err)
		}
		pkt, err := clientRecv(5 * time.Second)
		if err != nil {
			b.Fatalf("client recv[%d]: %v", i, err)
		}
		if !bytes.Equal(pkt, payload) {
			b.Fatalf("client recv[%d] payload mismatch", i)
		}
	}
	b.StopTimer()

	if err := <-errCh; err != nil {
		b.Fatal(err)
	}
}
