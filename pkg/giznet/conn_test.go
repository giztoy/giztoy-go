package giznet

import (
	"errors"
	"testing"
)

// TestConnDialNilUDPHandle asserts Dial on a Conn with only pk set (no UDP)
// returns ErrNilConn. Constructing that Conn requires unexported fields.
func TestConnDialNilUDPHandle(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	if _, err := (&Conn{pk: key.Public}).Dial(4); !errors.Is(err, ErrNilConn) {
		t.Fatalf("Dial(rpc, nil udp) err=%v, want %v", err, ErrNilConn)
	}
}
