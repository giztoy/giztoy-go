package gizclaw

import (
	"errors"
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"testing"
)

func TestServerListenAndServeRequiresSecurityPolicy(t *testing.T) {
	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error = %v", err)
	}

	server := &Server{KeyPair: keyPair}
	err = server.ListenAndServe()
	if !errors.Is(err, ErrNilSecurityPolicy) {
		t.Fatalf("ListenAndServe error = %v, want %v", err, ErrNilSecurityPolicy)
	}
}

func TestAllowAllAllowsPeerService(t *testing.T) {
	var policy SecurityPolicy = AllowAll{}
	if !policy.AllowPeerService(giznet.PublicKey{}, ServiceAdmin) {
		t.Fatal("AllowAll should allow admin service")
	}
	if !policy.AllowPeerService(giznet.PublicKey{}, 0xffff) {
		t.Fatal("AllowAll should allow arbitrary service")
	}
}
