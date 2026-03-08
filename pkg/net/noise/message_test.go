package noise

import (
	"bytes"
	"testing"
)

func TestEncodeDecodePayload_ProtocolPayloadOnly(t *testing.T) {
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

func TestDecodePayload_Empty(t *testing.T) {
	_, _, err := DecodePayload(nil)
	if err != ErrMessageTooShort {
		t.Fatalf("DecodePayload(nil) err = %v, want %v", err, ErrMessageTooShort)
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

func TestParseTransportMessageRejectsInvalidInput(t *testing.T) {
	short := make([]byte, TransportHeaderSize+TagSize-1)
	if _, err := ParseTransportMessage(short); err != ErrMessageTooShort {
		t.Fatalf("ParseTransportMessage(short) err=%v, want %v", err, ErrMessageTooShort)
	}

	wire := BuildTransportMessage(1, 2, []byte("ciphertext-with-tag"))
	wire[0] = MessageTypeHandshakeInit
	if _, err := ParseTransportMessage(wire); err != ErrInvalidMessageType {
		t.Fatalf("ParseTransportMessage(wrong type) err=%v, want %v", err, ErrInvalidMessageType)
	}
}

func TestHandshakeInitRoundTrip(t *testing.T) {
	var ephemeral Key
	for i := range ephemeral {
		ephemeral[i] = byte(i)
	}
	staticEnc := bytes.Repeat([]byte{0xAB}, 48)

	wire := BuildHandshakeInit(7, ephemeral, staticEnc)
	parsed, err := ParseHandshakeInit(wire)
	if err != nil {
		t.Fatalf("ParseHandshakeInit() error = %v", err)
	}

	if parsed.SenderIndex != 7 {
		t.Fatalf("SenderIndex = %d, want 7", parsed.SenderIndex)
	}
	if parsed.Ephemeral != ephemeral {
		t.Fatal("Ephemeral mismatch")
	}
	if !bytes.Equal(parsed.Static, staticEnc) {
		t.Fatal("Static mismatch")
	}
}

func TestParseHandshakeInitRejectsInvalidInput(t *testing.T) {
	if _, err := ParseHandshakeInit(make([]byte, HandshakeInitSize-1)); err != ErrMessageTooShort {
		t.Fatalf("ParseHandshakeInit(short) err=%v, want %v", err, ErrMessageTooShort)
	}

	wire := make([]byte, HandshakeInitSize)
	wire[0] = MessageTypeTransport
	if _, err := ParseHandshakeInit(wire); err != ErrInvalidMessageType {
		t.Fatalf("ParseHandshakeInit(wrong type) err=%v, want %v", err, ErrInvalidMessageType)
	}
}

func TestHandshakeRespRoundTrip(t *testing.T) {
	var ephemeral Key
	for i := range ephemeral {
		ephemeral[i] = byte(255 - i)
	}
	empty := bytes.Repeat([]byte{0xCD}, 16)

	wire := BuildHandshakeResp(3, 9, ephemeral, empty)
	parsed, err := ParseHandshakeResp(wire)
	if err != nil {
		t.Fatalf("ParseHandshakeResp() error = %v", err)
	}

	if parsed.SenderIndex != 3 || parsed.ReceiverIndex != 9 {
		t.Fatalf("indexes = (%d,%d), want (3,9)", parsed.SenderIndex, parsed.ReceiverIndex)
	}
	if parsed.Ephemeral != ephemeral {
		t.Fatal("Ephemeral mismatch")
	}
	if !bytes.Equal(parsed.Empty, empty) {
		t.Fatal("Empty mismatch")
	}
}

func TestParseHandshakeRespRejectsInvalidInput(t *testing.T) {
	if _, err := ParseHandshakeResp(make([]byte, HandshakeRespSize-1)); err != ErrMessageTooShort {
		t.Fatalf("ParseHandshakeResp(short) err=%v, want %v", err, ErrMessageTooShort)
	}

	wire := make([]byte, HandshakeRespSize)
	wire[0] = MessageTypeCookieReply
	if _, err := ParseHandshakeResp(wire); err != ErrInvalidMessageType {
		t.Fatalf("ParseHandshakeResp(wrong type) err=%v, want %v", err, ErrInvalidMessageType)
	}
}

func TestGetMessageType(t *testing.T) {
	if _, err := GetMessageType(nil); err != ErrMessageTooShort {
		t.Fatalf("GetMessageType(nil) err=%v, want %v", err, ErrMessageTooShort)
	}
	if got, err := GetMessageType([]byte{MessageTypeTransport}); err != nil || got != MessageTypeTransport {
		t.Fatalf("GetMessageType(valid) = (%d,%v), want (%d,nil)", got, err, MessageTypeTransport)
	}
}
