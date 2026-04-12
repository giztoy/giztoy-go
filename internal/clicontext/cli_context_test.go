package clicontext

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreCreateAndLoad(t *testing.T) {
	s := &Store{Root: t.TempDir()}

	if err := s.Create("local", "127.0.0.1:9820", "aabbccdd"); err != nil {
		t.Fatalf("Create err=%v", err)
	}

	cliCtx, err := Load(filepath.Join(s.Root, "local"))
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cliCtx.Name != "local" {
		t.Fatalf("Name=%q, want local", cliCtx.Name)
	}
	if cliCtx.Config.Server.Address != "127.0.0.1:9820" {
		t.Fatalf("Address=%q", cliCtx.Config.Server.Address)
	}
	if cliCtx.Config.Server.PublicKey != "aabbccdd" {
		t.Fatalf("PublicKey=%q", cliCtx.Config.Server.PublicKey)
	}
	if cliCtx.KeyPair == nil || cliCtx.KeyPair.Public.IsZero() {
		t.Fatal("KeyPair not loaded")
	}
}

func TestStoreCreateDuplicate(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := s.Create("dup", "addr", "pk"); err != nil {
		t.Fatal(err)
	}
	if err := s.Create("dup", "addr", "pk"); err == nil {
		t.Fatal("duplicate Create should fail")
	}
}

func TestStoreCurrentAutoSet(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := s.Create("first", "addr", "pk"); err != nil {
		t.Fatal(err)
	}

	cliCtx, err := s.Current()
	if err != nil {
		t.Fatalf("Current err=%v", err)
	}
	if cliCtx == nil {
		t.Fatal("Current returned nil after first Create")
	}
	if cliCtx.Name != "first" {
		t.Fatalf("Current Name=%q, want first", cliCtx.Name)
	}
}

func TestStoreCurrentNone(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	cliCtx, err := s.Current()
	if err != nil {
		t.Fatalf("Current err=%v", err)
	}
	if cliCtx != nil {
		t.Fatal("Current should be nil when no context exists")
	}
}

func TestStoreUse(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := s.Create("a", "addr-a", "pk-a"); err != nil {
		t.Fatal(err)
	}
	if err := s.Create("b", "addr-b", "pk-b"); err != nil {
		t.Fatal(err)
	}

	if err := s.Use("b"); err != nil {
		t.Fatalf("Use err=%v", err)
	}

	cliCtx, err := s.Current()
	if err != nil {
		t.Fatalf("Current err=%v", err)
	}
	if cliCtx.Name != "b" {
		t.Fatalf("Current Name=%q, want b", cliCtx.Name)
	}
}

func TestStoreUseNonExistent(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := s.Use("nope"); err == nil {
		t.Fatal("Use(nonexistent) should fail")
	}
}

func TestStoreList(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := s.Create("beta", "addr", "pk"); err != nil {
		t.Fatal(err)
	}
	if err := s.Create("alpha", "addr", "pk"); err != nil {
		t.Fatal(err)
	}

	names, current, err := s.List()
	if err != nil {
		t.Fatalf("List err=%v", err)
	}
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("List names=%v, want [alpha beta]", names)
	}
	if current != "beta" {
		t.Fatalf("List current=%q, want beta", current)
	}
}

func TestStoreListEmpty(t *testing.T) {
	s := &Store{Root: filepath.Join(t.TempDir(), "nonexistent")}
	names, current, err := s.List()
	if err != nil {
		t.Fatalf("List err=%v", err)
	}
	if len(names) != 0 {
		t.Fatalf("List names=%v, want empty", names)
	}
	if current != "" {
		t.Fatalf("List current=%q, want empty", current)
	}
}

func TestServerPublicKey(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	pk := "0000000000000000000000000000000000000000000000000000000000000000"
	if err := s.Create("spk", "addr", pk); err != nil {
		t.Fatal(err)
	}
	cliCtx, err := Load(filepath.Join(s.Root, "spk"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := cliCtx.ServerPublicKey()
	if err != nil {
		t.Fatalf("ServerPublicKey err=%v", err)
	}
	if key.String() != pk {
		t.Fatalf("ServerPublicKey=%q, want %q", key.String(), pk)
	}
}

func TestServerPublicKeyInvalid(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := s.Create("badpk", "addr", "not-hex"); err != nil {
		t.Fatal(err)
	}
	cliCtx, err := Load(filepath.Join(s.Root, "badpk"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cliCtx.ServerPublicKey(); err == nil {
		t.Fatal("ServerPublicKey(invalid) should fail")
	}
}

func TestLoadMissingConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("Load(no config) should fail")
	}
}

func TestLoadBadYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(":::"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("Load(bad yaml) should fail")
	}
}

func TestStoreLoadByName(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := s.Create("myctx", "127.0.0.1:9820", "aabb"); err != nil {
		t.Fatal(err)
	}

	cliCtx, err := s.LoadByName("myctx")
	if err != nil {
		t.Fatalf("LoadByName err=%v", err)
	}
	if cliCtx.Name != "myctx" {
		t.Fatalf("Name=%q, want myctx", cliCtx.Name)
	}
	if cliCtx.Config.Server.Address != "127.0.0.1:9820" {
		t.Fatalf("Address=%q", cliCtx.Config.Server.Address)
	}
}

func TestStoreLoadByNameRejectsTraversal(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	for _, bad := range []string{"", "../escape", "a/b", ".", ".."} {
		if _, err := s.LoadByName(bad); err == nil {
			t.Fatalf("LoadByName(%q) should fail", bad)
		}
	}
}

func TestStoreLoadByNameNotExist(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if _, err := s.LoadByName("nope"); err == nil {
		t.Fatal("LoadByName(nonexistent) should fail")
	}
}

func TestStoreSymlinkIsRelative(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := s.Create("myctx", "addr", "pk"); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(s.Root, currentLink)
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink err=%v", err)
	}
	if filepath.IsAbs(target) {
		t.Fatalf("symlink target is absolute: %q", target)
	}
	if target != "myctx" {
		t.Fatalf("symlink target=%q, want myctx", target)
	}
}

func TestStoreListAbsoluteCurrentSymlink(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := s.Create("alpha", "addr", "pk"); err != nil {
		t.Fatal(err)
	}
	if err := s.Create("beta", "addr", "pk"); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(s.Root, currentLink)
	if err := os.Remove(link); err != nil {
		t.Fatalf("Remove current symlink error=%v", err)
	}
	if err := os.Symlink(filepath.Join(s.Root, "alpha"), link); err != nil {
		t.Fatalf("Symlink error=%v", err)
	}

	names, current, err := s.List()
	if err != nil {
		t.Fatalf("List err=%v", err)
	}
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("List names=%v", names)
	}
	if current != "alpha" {
		t.Fatalf("List current=%q, want alpha", current)
	}
}
