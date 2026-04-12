package giznet

import "testing"

// TestListenServiceAddrOnStandaloneConn covers ListenService on a Conn that
// only has a public key set (no UDP handle). This requires the unexported Conn
// literal and must stay in package giznet.
func TestListenServiceAddrOnStandaloneConn(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	conn := &Conn{pk: key.Public}
	listener := conn.ListenService(7)
	if got := listener.Addr().String(); got == "" {
		t.Fatal("listener.Addr().String() should not be empty")
	}
}
