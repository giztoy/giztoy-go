package client

import (
	"strings"
	"testing"

	"github.com/GizClaw/gizclaw-go/cmd/internal/clicontext"
)

func TestDialFromContextNoActiveContext(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, _, _, err := DialFromContext("")
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

	_, _, _, err = DialFromContext("local")
	if err == nil {
		t.Fatal("DialFromContext should fail on invalid server public key")
	}
	if !strings.Contains(err.Error(), "invalid server public key") {
		t.Fatalf("DialFromContext error = %v", err)
	}
}

func TestDialFromContextMissingNamedContext(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, _, _, err := DialFromContext("missing")
	if err == nil {
		t.Fatal("DialFromContext should fail for a missing named context")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("DialFromContext error = %v", err)
	}
}
