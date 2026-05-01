package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/cmd/internal/service"
	"github.com/GizClaw/gizclaw-go/cmd/internal/storage"
	"github.com/GizClaw/gizclaw-go/cmd/internal/stores"
)

func TestPrepareWorkspaceConfigLoadsWorkspaceConfig(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, workspaceConfigFile), []byte(`
listen: "127.0.0.1:39001"
admin-public-key: "ABABABABABABABABABABABABABABABABABABABABABABABABABABABABABABABAB"
storage:
  memory:
    kind: keyvalue
    memory: {}
  local-files:
    kind: filesystem
    fs:
      dir: .
  firmware-depot:
    kind: depotstore
    depot-fs: {}
stores:
  gears:
    kind: keyvalue
    storage: memory
    prefix: gears
  credentials:
    kind: keyvalue
    storage: memory
    prefix: credentials
  minimax-tenants:
    kind: keyvalue
    storage: memory
    prefix: minimax-tenants
  voices:
    kind: keyvalue
    storage: memory
    prefix: voices
  workspaces:
    kind: keyvalue
    storage: memory
    prefix: workspaces
  workspace-templates:
    kind: keyvalue
    storage: memory
    prefix: workspace-templates
  firmware-depots:
    kind: keyvalue
    storage: memory
    prefix: firmware-depots
  firmware:
    kind: depotstore
    storage: firmware-depot
    depot-fs:
      filesystem:
        storage: local-files
        base-dir: firmware
gears:
  store: gears
credentials:
  store: credentials
minimax:
  tenants-store: minimax-tenants
  voices-store: voices
  credentials-store: credentials
workspaces:
  store: workspaces
  templates-store: workspace-templates
workspace-templates:
  store: workspace-templates
depots:
  store: firmware
  metadata-store: firmware-depots
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
	if cfg.AdminPublicKey != strings.Repeat("ab", 32) {
		t.Fatalf("AdminPublicKey = %q", cfg.AdminPublicKey)
	}
	if got := cfg.Storage["memory"].Dir; got != "" {
		t.Fatalf("memory store dir = %q", got)
	}
	if got := cfg.Storage["local-files"].FS.Dir; got != workspace {
		t.Fatalf("local-files dir = %q", got)
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
storage:
  memory:
    kind: keyvalue
    memory: {}
  fw-files:
    kind: filesystem
    fs:
      dir: .
  fw:
    kind: depotstore
    depot-fs: {}
stores:
  fw-meta:
    kind: keyvalue
    storage: memory
    prefix: firmware-depots
  fw:
    kind: depotstore
    storage: fw
    depot-fs:
      filesystem:
        storage: fw-files
        base-dir: firmware
gears:
  store: fw
credentials:
  store: fw
minimax:
  tenants-store: fw
  voices-store: fw
  credentials-store: fw
workspaces:
  store: fw
  templates-store: fw
workspace-templates:
  store: fw
depots:
  store: fw
  metadata-store: fw-meta
`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	cfg, err := prepareWorkspaceConfig(workspace)
	if err != nil {
		t.Fatalf("prepareWorkspaceConfig error = %v", err)
	}
	if got := cfg.Storage["fw-files"].FS.Dir; got != workspace {
		t.Fatalf("fw dir = %q", got)
	}
	if got := cfg.Stores["fw"].DepotFS.Filesystem.BaseDir; got != "firmware" {
		t.Fatalf("fw base-dir = %q", got)
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

	gotStorage := resolveWorkspaceStorageConfigs(root, map[string]storage.Config{
		"fw": {
			Kind: storage.KindFilesystem,
			FS:   &storage.FSConfig{Dir: absoluteDir},
		},
	})
	if gotStorage["fw"].FS.Dir != absoluteDir {
		t.Fatalf("fw storage dir = %q, want %q", gotStorage["fw"].FS.Dir, absoluteDir)
	}

	gotStores := resolveWorkspaceStoreConfigs(root, map[string]stores.Config{
		"fw": {
			Kind:    stores.KindFS,
			Backend: "filesystem",
			Dir:     absoluteDir,
		},
	})
	if gotStores["fw"].Dir != absoluteDir {
		t.Fatalf("fw legacy store dir = %q, want %q", gotStores["fw"].Dir, absoluteDir)
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

func TestServeContextClosesStoresWhenPIDAcquireFails(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, workspaceConfigFile), []byte(`
storage:
  main-kv:
    kind: keyvalue
    badger:
      dir: data/kv
  local-files:
    kind: filesystem
    fs:
      dir: .
  firmware-depot:
    kind: depotstore
    depot-fs: {}
stores:
  gears:
    kind: keyvalue
    storage: main-kv
    prefix: gears
  credentials:
    kind: keyvalue
    storage: main-kv
    prefix: credentials
  minimax-tenants:
    kind: keyvalue
    storage: main-kv
    prefix: minimax-tenants
  voices:
    kind: keyvalue
    storage: main-kv
    prefix: voices
  workspaces:
    kind: keyvalue
    storage: main-kv
    prefix: workspaces
  workspace-templates:
    kind: keyvalue
    storage: main-kv
    prefix: workspace-templates
  firmware-depots:
    kind: keyvalue
    storage: main-kv
    prefix: firmware-depots
  firmware:
    kind: depotstore
    storage: firmware-depot
    depot-fs:
      filesystem:
        storage: local-files
        base-dir: firmware
gears:
  store: gears
credentials:
  store: credentials
minimax:
  tenants-store: minimax-tenants
  voices-store: voices
  credentials-store: credentials
workspaces:
  store: workspaces
  templates-store: workspace-templates
workspace-templates:
  store: workspace-templates
depots:
  store: firmware
  metadata-store: firmware-depots
`), 0o644); err != nil {
		t.Fatalf("WriteFile config error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, workspacePIDFile), []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644); err != nil {
		t.Fatalf("WriteFile pid error = %v", err)
	}

	err := ServeContext(context.Background(), workspace, ServeOptions{})
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("ServeContext() err = %v", err)
	}

	reopened, err := storage.New(map[string]storage.Config{
		"main-kv": {Kind: storage.KindKeyValue, Badger: &storage.BadgerConfig{Dir: filepath.Join(workspace, "data", "kv")}},
	})
	if err != nil {
		t.Fatalf("storage should be closed after PID error, reopen: %v", err)
	}
	defer reopened.Close()
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

func TestAcquireWorkspacePIDRejectsRunningPID(t *testing.T) {
	workspace := t.TempDir()
	pidPath := filepath.Join(workspace, workspacePIDFile)
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	_, err := acquireWorkspacePID(workspace, false)
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("acquireWorkspacePID() err = %v", err)
	}
}

func TestHandleExistingWorkspacePIDForceRemovesUnreadablePID(t *testing.T) {
	workspace := t.TempDir()
	pidPath := filepath.Join(workspace, workspacePIDFile)
	if err := os.WriteFile(pidPath, []byte("not-a-pid\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	if err := handleExistingWorkspacePID(pidPath, true); err != nil {
		t.Fatalf("handleExistingWorkspacePID(force invalid) error = %v", err)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("pid file should be removed, stat err = %v", err)
	}
}

func TestReadWorkspacePIDRejectsInvalidPID(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), workspacePIDFile)
	if err := os.WriteFile(pidPath, []byte("0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if _, err := readWorkspacePID(pidPath); err == nil {
		t.Fatal("readWorkspacePID invalid pid error = nil")
	}
}

func TestProcessRunningAndWaitForProcessExitForMissingPID(t *testing.T) {
	if processRunning(0) {
		t.Fatal("processRunning(0) = true")
	}
	if err := waitForProcessExit(999999, time.Millisecond); err != nil {
		t.Fatalf("waitForProcessExit(missing) error = %v", err)
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
