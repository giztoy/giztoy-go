package noise

import (
	"net"
	"testing"
	"time"
)

func TestWrapPacketConnNil(t *testing.T) {
	if WrapPacketConn(nil) != nil {
		t.Fatal("WrapPacketConn(nil) should return nil")
	}
}

func TestWrapPacketConnPassthrough(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket failed: %v", err)
	}
	defer pc.Close()

	transport := &packetConnTransport{PacketConn: pc}
	if WrapPacketConn(transport) != transport {
		t.Fatal("WrapPacketConn should return existing Transport as-is")
	}
}

func TestWrapPacketConnCompatibilityMethods(t *testing.T) {
	sender, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket sender failed: %v", err)
	}
	defer sender.Close()

	receiver, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket receiver failed: %v", err)
	}
	defer receiver.Close()

	wrappedSender := WrapPacketConn(sender)
	wrappedReceiver := WrapPacketConn(receiver)

	deadline := time.Now().Add(2 * time.Second)
	if err := wrappedReceiver.SetReadDeadline(deadline); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}

	msg := []byte("wrapped-packet")
	if err := wrappedSender.SendTo(msg, receiver.LocalAddr()); err != nil {
		t.Fatalf("SendTo failed: %v", err)
	}

	buf := make([]byte, 64)
	n, from, err := wrappedReceiver.RecvFrom(buf)
	if err != nil {
		t.Fatalf("RecvFrom failed: %v", err)
	}
	if from == nil {
		t.Fatal("RecvFrom returned nil address")
	}
	if got := string(buf[:n]); got != string(msg) {
		t.Fatalf("payload=%q, want %q", got, string(msg))
	}
}
