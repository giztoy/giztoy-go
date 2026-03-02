package noise

import (
	"bytes"
	"testing"
)

func TestProtocolConstants(t *testing.T) {
	if ProtocolRPC != 0x01 {
		t.Fatalf("ProtocolRPC = %d, want 1", ProtocolRPC)
	}
	if ProtocolEVENT != 0x03 {
		t.Fatalf("ProtocolEVENT = %d, want 3", ProtocolEVENT)
	}
	if ProtocolOPUS != 0x10 {
		t.Fatalf("ProtocolOPUS = %d, want 16", ProtocolOPUS)
	}
}

func TestEncodeDecodePayload_ProtocolPayloadOnly(t *testing.T) {
	payload := []byte(`{"method":"ping"}`)
	encoded := EncodePayload(ProtocolRPC, payload)

	if len(encoded) != 1+len(payload) {
		t.Fatalf("encoded length = %d, want %d", len(encoded), 1+len(payload))
	}

	protocol, decoded, err := DecodePayload(encoded)
	if err != nil {
		t.Fatalf("DecodePayload() error = %v", err)
	}
	if protocol != ProtocolRPC {
		t.Fatalf("protocol = %d, want %d", protocol, ProtocolRPC)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatalf("decoded payload mismatch")
	}
}

func TestDecodePayload_Empty(t *testing.T) {
	_, _, err := DecodePayload(nil)
	if err != ErrMessageTooShort {
		t.Fatalf("DecodePayload(nil) err = %v, want %v", err, ErrMessageTooShort)
	}
}

func TestIsFoundationProtocol(t *testing.T) {
	if !IsFoundationProtocol(ProtocolRPC) {
		t.Fatal("ProtocolRPC should be whitelisted")
	}
	if !IsFoundationProtocol(ProtocolEVENT) {
		t.Fatal("ProtocolEVENT should be whitelisted")
	}
	if !IsFoundationProtocol(ProtocolOPUS) {
		t.Fatal("ProtocolOPUS should be whitelisted")
	}
	if IsFoundationProtocol(0x40) {
		t.Fatal("0x40 should not be whitelisted")
	}
}

func TestTransportMessageRoundTrip(t *testing.T) {
	receiverIndex := uint32(42)
	counter := uint64(99)
	ciphertext := []byte("0123456789abcdefciphertext")

	wire := BuildTransportMessage(receiverIndex, counter, ciphertext)
	parsed, err := ParseTransportMessage(wire)
	if err != nil {
		t.Fatalf("ParseTransportMessage() error = %v", err)
	}

	if parsed.ReceiverIndex != receiverIndex {
		t.Fatalf("receiver index = %d, want %d", parsed.ReceiverIndex, receiverIndex)
	}
	if parsed.Counter != counter {
		t.Fatalf("counter = %d, want %d", parsed.Counter, counter)
	}
	if !bytes.Equal(parsed.Ciphertext, ciphertext) {
		t.Fatalf("ciphertext mismatch")
	}
}
