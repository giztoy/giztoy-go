package identity

import (
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadOrGenerate_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity.key")

	kp, err := LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("LoadOrGenerate (new) err=%v", err)
	}
	if kp.Public.IsZero() {
		t.Fatal("generated key has zero public key")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile err=%v", err)
	}
	if len(data) != giznet.KeySize {
		t.Fatalf("key file size=%d, want %d", len(data), giznet.KeySize)
	}

	info, _ := os.Stat(path)
	perm := info.Mode().Perm()
	if runtime.GOOS != "windows" && perm != 0o600 {
		t.Fatalf("key file perm=%o, want 0600", perm)
	}
	if runtime.GOOS == "windows" && perm == 0 {
		t.Fatalf("key file perm=%o, want non-zero", perm)
	}
}

func TestLoadOrGenerate_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity.key")

	kp1, err := LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("first LoadOrGenerate err=%v", err)
	}

	kp2, err := LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("second LoadOrGenerate err=%v", err)
	}

	if !kp1.Public.Equal(kp2.Public) {
		t.Fatal("public keys differ after reload")
	}
}

func TestLoadOrGenerate_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "identity.key")

	kp, err := LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("LoadOrGenerate (nested) err=%v", err)
	}
	if kp.Public.IsZero() {
		t.Fatal("generated key has zero public key")
	}
}

func TestLoad_NotExist(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.key"))
	if err == nil {
		t.Fatal("Load(nonexistent) should fail")
	}
}

func TestLoad_InvalidSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.key")
	if err := os.WriteFile(path, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load(bad size) should fail")
	}
}

func TestLoadOrGenerate_InvalidExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.key")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadOrGenerate(path)
	if err == nil {
		t.Fatal("LoadOrGenerate(bad existing) should fail")
	}
}
