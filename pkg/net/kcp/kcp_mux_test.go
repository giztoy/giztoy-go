package kcp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func kcpMuxPair(t *testing.T, cfg KcpMuxConfig) (*KcpMux, *KcpMux) {
	t.Helper()

	var clientMux, serverMux *KcpMux
	clientMux = NewKcpMux(0, true, cfg, func(service uint64, data []byte) error {
		return serverMux.Input(data)
	}, nil)
	serverMux = NewKcpMux(0, false, cfg, func(service uint64, data []byte) error {
		return clientMux.Input(data)
	}, nil)
	return clientMux, serverMux
}

func acceptWithTimeout(t *testing.T, mux *KcpMux, timeout time.Duration) net.Conn {
	t.Helper()
	result := make(chan struct {
		conn net.Conn
		err  error
	}, 1)
	go func() {
		conn, err := mux.Accept()
		result <- struct {
			conn net.Conn
			err  error
		}{conn: conn, err: err}
	}()

	select {
	case got := <-result:
		if got.err != nil {
			t.Fatalf("Accept failed: %v", got.err)
		}
		return got.conn
	case <-time.After(timeout):
		t.Fatalf("Accept timed out after %s", timeout)
		return nil
	}
}

func TestKcpMux_OpenCreatesDistinctStreams(t *testing.T) {
	client, server := kcpMuxPair(t, KcpMuxConfig{})
	defer client.Close()
	defer server.Close()

	stream1, err := client.Open()
	if err != nil {
		t.Fatalf("Open(1) failed: %v", err)
	}
	defer stream1.Close()

	stream2, err := client.Open()
	if err != nil {
		t.Fatalf("Open(2) failed: %v", err)
	}
	defer stream2.Close()

	if stream1 == stream2 {
		t.Fatal("expected distinct streams")
	}
	if got := client.NumStreams(); got != 2 {
		t.Fatalf("NumStreams=%d, want 2", got)
	}

	serverStream1 := acceptWithTimeout(t, server, 2*time.Second)
	defer serverStream1.Close()
	serverStream2 := acceptWithTimeout(t, server, 2*time.Second)
	defer serverStream2.Close()
	if serverStream1 == serverStream2 {
		t.Fatal("expected distinct accepted streams")
	}
}

func TestKcpMux_OpenThenDataBidirectional(t *testing.T) {
	client, server := kcpMuxPair(t, KcpMuxConfig{})
	defer client.Close()
	defer server.Close()

	clientStream, err := client.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer clientStream.Close()
	serverStream := acceptWithTimeout(t, server, 2*time.Second)
	defer serverStream.Close()

	request := []byte("hello")
	if _, err := clientStream.Write(request); err != nil {
		t.Fatalf("Write(request) failed: %v", err)
	}
	if got := readExactWithTimeout(t, serverStream, len(request), 5*time.Second); !bytes.Equal(got, request) {
		t.Fatalf("request mismatch: got=%q want=%q", got, request)
	}

	response := []byte("world")
	if _, err := serverStream.Write(response); err != nil {
		t.Fatalf("Write(response) failed: %v", err)
	}
	if got := readExactWithTimeout(t, clientStream, len(response), 5*time.Second); !bytes.Equal(got, response) {
		t.Fatalf("response mismatch: got=%q want=%q", got, response)
	}
}

func TestKcpMux_CloseOneStreamDoesNotAffectOthers(t *testing.T) {
	client, server := kcpMuxPair(t, KcpMuxConfig{})
	defer client.Close()
	defer server.Close()

	streamA, err := client.Open()
	if err != nil {
		t.Fatalf("Open(A) failed: %v", err)
	}
	defer streamA.Close()
	streamB, err := client.Open()
	if err != nil {
		t.Fatalf("Open(B) failed: %v", err)
	}
	defer streamB.Close()

	serverA := acceptWithTimeout(t, server, 2*time.Second)
	defer serverA.Close()
	serverB := acceptWithTimeout(t, server, 2*time.Second)
	defer serverB.Close()

	if err := streamA.Close(); err != nil {
		t.Fatalf("streamA.Close failed: %v", err)
	}

	payload := []byte("still-alive")
	if _, err := streamB.Write(payload); err != nil {
		t.Fatalf("streamB.Write failed: %v", err)
	}

	got := readExactWithTimeout(t, serverB, len(payload), 5*time.Second)
	if !bytes.Equal(got, payload) {
		t.Fatalf("streamB payload mismatch: got=%q want=%q", got, payload)
	}
}

func TestKcpMux_OpenFramesAreIdempotent(t *testing.T) {
	server := NewKcpMux(0, false, KcpMuxConfig{}, nil, nil)
	defer server.Close()

	frame := binary.AppendUvarint(nil, 1)
	frame = append(frame, kcpMuxFrameOpen)

	if err := server.Input(frame); err != nil {
		t.Fatalf("first Input failed: %v", err)
	}
	if err := server.Input(frame); err != nil {
		t.Fatalf("second Input failed: %v", err)
	}
	if got := server.NumStreams(); got != 1 {
		t.Fatalf("NumStreams=%d, want 1", got)
	}
}

func TestKcpMux_CloseAckTimeoutForcesCleanup(t *testing.T) {
	client := NewKcpMux(0, true, KcpMuxConfig{CloseAckTimeout: 50 * time.Millisecond}, func(service uint64, data []byte) error {
		return nil
	}, nil)
	defer client.Close()

	stream, err := client.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if err := stream.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for client.NumStreams() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := client.NumStreams(); got != 0 {
		t.Fatalf("NumStreams=%d, want 0", got)
	}
}

func TestKcpMux_IdleTimeoutCleansInactiveStream(t *testing.T) {
	client, server := kcpMuxPair(t, KcpMuxConfig{IdleStreamTimeout: 50 * time.Millisecond})
	defer client.Close()
	defer server.Close()

	stream, err := client.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer stream.Close()
	accepted := acceptWithTimeout(t, server, 2*time.Second)
	defer accepted.Close()

	deadline := time.Now().Add(500 * time.Millisecond)
	for client.NumStreams() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := client.NumStreams(); got != 0 {
		t.Fatalf("client NumStreams=%d, want 0", got)
	}
	if got := server.NumStreams(); got != 0 {
		t.Fatalf("server NumStreams=%d, want 0", got)
	}
}

func TestKcpMux_InvalidFrameClosesStreamWithInvalidReason(t *testing.T) {
	client, server := kcpMuxPair(t, KcpMuxConfig{})
	defer client.Close()
	defer server.Close()

	stream, err := client.Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer stream.Close()
	serverStream := acceptWithTimeout(t, server, 2*time.Second)
	defer serverStream.Close()

	frame := binary.AppendUvarint(nil, 1)
	frame = append(frame, byte(99))
	if err := server.Input(frame); err != nil {
		t.Fatalf("Input failed: %v", err)
	}

	buf := make([]byte, 1)
	_ = stream.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = stream.Read(buf)
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("expected invalid-close error, got %v", err)
	}
}
