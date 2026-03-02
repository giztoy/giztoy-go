package transport

import (
	"net"
	"testing"
	"time"
)

func TestUDPAddrAndTransportBasic(t *testing.T) {
	a, err := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
	if err != nil {
		t.Fatalf("ResolveUDPAddr failed: %v", err)
	}
	wrapped := UDPAddrFromNetAddr(a)
	if wrapped.Network() != "udp" {
		t.Fatalf("Network=%s, want udp", wrapped.Network())
	}
	if wrapped.String() != a.String() {
		t.Fatalf("String=%s, want %s", wrapped.String(), a.String())
	}

	t1, err := NewUDPTransport("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewUDPTransport t1 failed: %v", err)
	}
	defer t1.Close()
	t2, err := NewUDPTransport("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewUDPTransport t2 failed: %v", err)
	}
	defer t2.Close()

	if t1.LocalAddr() == nil || t2.LocalAddr() == nil {
		t.Fatal("LocalAddr should not be nil")
	}
	if err := t1.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	if err := t1.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetWriteDeadline failed: %v", err)
	}
	_ = t1.SetReadDeadline(time.Time{})
	_ = t1.SetWriteDeadline(time.Time{})

	msg := []byte("udp-internal-transport")
	if err := t1.SendTo(msg, t2.LocalAddr()); err != nil {
		t.Fatalf("SendTo failed: %v", err)
	}

	buf := make([]byte, 256)
	if err := t2.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline recv side failed: %v", err)
	}
	n, from, err := t2.RecvFrom(buf)
	if err != nil {
		t.Fatalf("RecvFrom failed: %v", err)
	}
	if from == nil {
		t.Fatal("from should not be nil")
	}
	if got := string(buf[:n]); got != string(msg) {
		t.Fatalf("payload=%q, want %q", got, string(msg))
	}

	if err := t1.SendTo(msg, NewMockAddr("not-a-udp-address")); err == nil {
		t.Fatal("SendTo with invalid address should fail")
	}
}

func TestMockTransportBasic(t *testing.T) {
	t1 := NewMockTransport("peer1")
	t2 := NewMockTransport("peer2")
	t1.Connect(t2)

	msg := []byte("hello")
	if err := t1.SendTo(msg, t2.LocalAddr()); err != nil {
		t.Fatalf("SendTo failed: %v", err)
	}
	buf := make([]byte, 256)
	n, from, err := t2.RecvFrom(buf)
	if err != nil {
		t.Fatalf("RecvFrom failed: %v", err)
	}
	if from.String() != "peer1" {
		t.Fatalf("from=%s, want peer1", from.String())
	}
	if got := string(buf[:n]); got != "hello" {
		t.Fatalf("payload=%q, want hello", got)
	}

	if err := t2.InjectPacket([]byte("injected"), NewMockAddr("sender")); err != nil {
		t.Fatalf("InjectPacket failed: %v", err)
	}
	n, from, err = t2.RecvFrom(buf)
	if err != nil {
		t.Fatalf("RecvFrom injected failed: %v", err)
	}
	if from.String() != "sender" {
		t.Fatalf("from=%s, want sender", from.String())
	}
	if got := string(buf[:n]); got != "injected" {
		t.Fatalf("payload=%q, want injected", got)
	}

	if err := t1.SetReadDeadline(time.Now()); err != nil {
		t.Fatalf("SetReadDeadline no-op failed: %v", err)
	}
	if err := t1.SetWriteDeadline(time.Now()); err != nil {
		t.Fatalf("SetWriteDeadline no-op failed: %v", err)
	}

	t3 := NewMockTransport("isolated")
	if err := t3.SendTo([]byte("x"), NewMockAddr("peer")); err != ErrMockNoPeer {
		t.Fatalf("SendTo without peer err=%v, want %v", err, ErrMockNoPeer)
	}

	if err := t1.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := t1.Close(); err != nil {
		t.Fatalf("Close second time failed: %v", err)
	}
	if err := t1.SendTo([]byte("x"), NewMockAddr("peer")); err != ErrMockTransportClosed {
		t.Fatalf("SendTo after close err=%v, want %v", err, ErrMockTransportClosed)
	}
	if err := t3.Close(); err != nil {
		t.Fatalf("Close isolated failed: %v", err)
	}
}
