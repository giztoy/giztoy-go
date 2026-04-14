package core

import (
	"bytes"
	"testing"
)

func TestEncodeDecodePayload(t *testing.T) {
	payload := []byte(`{"method":"ping"}`)
	const protocolRPC byte = 0x81
	encoded := EncodePayload(protocolRPC, payload)

	if len(encoded) != 1+len(payload) {
		t.Fatalf("encoded length = %d, want %d", len(encoded), 1+len(payload))
	}

	protocol, decoded, err := DecodePayload(encoded)
	if err != nil {
		t.Fatalf("DecodePayload() error = %v", err)
	}
	if protocol != protocolRPC {
		t.Fatalf("protocol = %d, want %d", protocol, protocolRPC)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatalf("decoded payload mismatch")
	}
}

func TestDecodePayloadEmpty(t *testing.T) {
	_, _, err := DecodePayload(nil)
	if err != ErrPayloadTooShort {
		t.Fatalf("DecodePayload(nil) err = %v, want %v", err, ErrPayloadTooShort)
	}
}
