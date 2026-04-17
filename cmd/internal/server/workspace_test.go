package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GizClaw/gizclaw-go/cmd/internal/service"
	"github.com/GizClaw/gizclaw-go/cmd/internal/stores"
)

func TestPrepareWorkspaceConfigLoadsWorkspaceConfig(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, workspaceConfigFile), []byte(`
listen: "127.0.0.1:39001"
stores:
  mem:
    kind: keyvalue
    backend: memory
gears:
  store: mem
depots:
  store: mem
`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	cfg, err := prepareWorkspaceConfig(workspace)
	if err != nil {
		t.Fatalf("prepareWorkspaceConfig error = %v", err)
	}
	if cfg.KeyPair == nil {
		t.Fatal("KeyPair should not be nil")
	}
	if cfg.ListenAddr != "127.0.0.1:39001" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if got := cfg.Stores["mem"].Dir; got != "" {
		t.Fatalf("memory store dir = %q", got)
	}
}

func TestPrepareWorkspaceConfigUsesDefaultListenAddr(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, workspaceConfigFile), []byte(`
stores:
  mem:
    kind: keyvalue
    backend: memory
gears:
  store: mem
depots:
  store: mem
`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	cfg, err := prepareWorkspaceConfig(workspace)
	if err != nil {
		t.Fatalf("prepareWorkspaceConfig error = %v", err)
	}
	if cfg.ListenAddr != DefaultConfig().ListenAddr {
		t.Fatalf("ListenAddr = %q, want %q", cfg.ListenAddr, DefaultConfig().ListenAddr)
	}
}

func TestPrepareWorkspaceConfigLoadError(t *testing.T) {
	_, err := prepareWorkspaceConfig(t.TempDir())
	if err == nil {
		t.Fatal("prepareWorkspaceConfig should fail without config.yaml")
	}
}

func TestPrepareWorkspaceConfigResolvesRelativeStoreDirs(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, workspaceConfigFile), []byte(`
stores:
  fw:
    kind: filestore
    backend: filesystem
    dir: firmware
gears:
  store: fw
depots:
  store: fw
`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	cfg, err := prepareWorkspaceConfig(workspace)
	if err != nil {
		t.Fatalf("prepareWorkspaceConfig error = %v", err)
	}
	if got := cfg.Stores["fw"].Dir; got != filepath.Join(workspace, "firmware") {
		t.Fatalf("fw dir = %q", got)
	}
}

func TestPrepareWorkspaceConfigIdentityError(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, workspaceConfigFile), []byte(`
stores:
  mem:
    kind: keyvalue
    backend: memory
gears:
  store: mem
depots:
  store: mem
`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(workspace, workspaceIdentityFile), 0o755); err != nil {
		t.Fatalf("Mkdir error = %v", err)
	}

	_, err := prepareWorkspaceConfig(workspace)
	if err == nil {
		t.Fatal("prepareWorkspaceConfig should fail when identity.key is a directory")
	}
}

func TestResolveWorkspaceStoreConfigsPreservesAbsoluteDirs(t *testing.T) {
	root := t.TempDir()
	absoluteDir := filepath.Join(t.TempDir(), "firmware")

	got := resolveWorkspaceStoreConfigs(root, map[string]stores.Config{
		"fw": {
			Kind:    stores.KindFS,
			Backend: "filesystem",
			Dir:     absoluteDir,
		},
	})
	if got["fw"].Dir != absoluteDir {
		t.Fatalf("fw dir = %q, want %q", got["fw"].Dir, absoluteDir)
	}
}

func TestServeReturnsWorkspaceLoadError(t *testing.T) {
	err := Serve(t.TempDir())
	if err == nil {
		t.Fatal("Serve should fail without config.yaml")
	}
}

func TestServeReturnsServerBuildError(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, workspaceConfigFile), []byte(`
stores:
  bad:
    kind: keyvalue
    backend: unknown
gears:
  store: bad
depots:
  store: bad
`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	err := Serve(workspace)
	if err == nil {
		t.Fatal("Serve should fail when New cannot build stores")
	}
}

func TestServeRejectsServiceManagedWorkspace(t *testing.T) {
	workspace := t.TempDir()
	if err := service.WriteMarker(workspace); err != nil {
		t.Fatalf("WriteMarker error = %v", err)
	}

	err := Serve(workspace)
	if err == nil || !strings.Contains(err.Error(), "managed by gizclaw service") {
		t.Fatalf("Serve(service-managed) err = %v", err)
	}
}

func TestHandleExistingWorkspacePIDRejectsStaleWithoutForce(t *testing.T) {
	workspace := t.TempDir()
	pidPath := filepath.Join(workspace, workspacePIDFile)
	if err := os.WriteFile(pidPath, []byte("999999\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	err := handleExistingWorkspacePID(pidPath, false)
	if err == nil || !strings.Contains(err.Error(), "stale pid file") {
		t.Fatalf("handleExistingWorkspacePID() err = %v", err)
	}
}

func TestHandleExistingWorkspacePIDForceRemovesStale(t *testing.T) {
	workspace := t.TempDir()
	pidPath := filepath.Join(workspace, workspacePIDFile)
	if err := os.WriteFile(pidPath, []byte("999999\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	if err := handleExistingWorkspacePID(pidPath, true); err != nil {
		t.Fatalf("handleExistingWorkspacePID(force) error = %v", err)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("pid file should be removed, stat err = %v", err)
	}
}

func TestAcquireWorkspacePIDWritesAndRemovesCurrentPID(t *testing.T) {
	workspace := t.TempDir()

	release, err := acquireWorkspacePID(workspace, false)
	if err != nil {
		t.Fatalf("acquireWorkspacePID error = %v", err)
	}

	pidPath := filepath.Join(workspace, workspacePIDFile)
	pid, err := readWorkspacePID(pidPath)
	if err != nil {
		t.Fatalf("readWorkspacePID error = %v", err)
	}
	if pid != os.Getpid() {
		t.Fatalf("pid = %d, want %d", pid, os.Getpid())
	}

	release()
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("pid file should be removed, stat err = %v", err)
	}
}
