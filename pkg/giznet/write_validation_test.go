package giznet

import (
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/giznet/internal/core"
)

// TestIntegration_WriteValidationPrecedesPeerLookup verifies that Write
// rejects RPC datagrams and unsupported protocols before looking up the peer.
// This requires constructing a standalone ServiceMux via internal APIs.
func TestIntegration_WriteValidationPrecedesPeerLookup(t *testing.T) {
	localKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate local key failed: %v", err)
	}
	remoteKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Generate remote key failed: %v", err)
	}

	u, err := core.NewUDP(localKey,
		WithBindAddr("127.0.0.1:0"),
		WithAllowUnknown(true),
		WithDecryptWorkers(1),
	)
	if err != nil {
		t.Fatalf("NewUDP failed: %v", err)
	}
	t.Cleanup(func() { _ = u.Close() })

	go func() {
		buf := make([]byte, 65535)
		for {
			if _, _, err := u.ReadFrom(buf); err != nil {
				return
			}
		}
	}()

	smux := core.NewServiceMux(remoteKey.Public, ServiceMuxConfig{})
	if _, err := smux.Write(ProtocolKCP, []byte("rpc-over-datagram")); err != ErrKCPMustUseStream {
		t.Fatalf("Write(RPC datagram) err=%v, want %v", err, ErrKCPMustUseStream)
	}

	if _, err := smux.Write(0x7f, []byte("custom")); err != ErrNoSession {
		t.Fatalf("Write(custom protocol without Output) err=%v, want %v", err, ErrNoSession)
	}
}
