package client

import (
	"strings"
	"testing"

	"github.com/giztoy/giztoy-go/internal/clicontext"
)

func TestDialFromContextNoActiveContext(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := DialFromContext("")
	if err == nil {
		t.Fatal("DialFromContext should fail without an active context")
	}
	if !strings.Contains(err.Error(), "no active context") {
		t.Fatalf("DialFromContext error = %v", err)
	}
}

func TestDialFromContextInvalidServerPublicKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	store, err := clicontext.DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore error = %v", err)
	}
	if err := store.Create("local", "127.0.0.1:9820", "not-hex"); err != nil {
		t.Fatalf("Create error = %v", err)
	}

	_, err = DialFromContext("local")
	if err == nil {
		t.Fatal("DialFromContext should fail on invalid server public key")
	}
	if !strings.Contains(err.Error(), "invalid server public key") {
		t.Fatalf("DialFromContext error = %v", err)
	}
}
