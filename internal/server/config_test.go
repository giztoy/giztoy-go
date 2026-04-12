package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/giztoy/giztoy-go/internal/stores"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/gizclaw"
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"github.com/giztoy/giztoy-go/pkg/kv"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ListenAddr != ":9820" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.DataDir == "" {
		t.Fatal("DataDir is empty")
	}
}

func TestLoadConfigMergesIntoRuntimeConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
listen: ":1234"
stores:
  mem:
    kind: keyvalue
    backend: memory
  fw:
    kind: filestore
    backend: filesystem
    dir: firmware
gears:
  store: mem
  registration-tokens:
    admin_default:
      role: admin
depots:
  store: fw
`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	srv, err := New(Config{
		DataDir:    dir,
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	if srv.SecurityPolicy == nil {
		t.Fatal("SecurityPolicy is nil")
	}
	if srv.Manager == nil {
		t.Fatal("Manager is nil")
	}
	if srv.PeerServer == nil {
		t.Fatal("PeerServer is nil")
	}
	if srv.PublicKey().String() == "" {
		t.Fatal("PublicKey should not be empty")
	}
}

func TestConfigValidateRequiresStores(t *testing.T) {
	cfg := Config{DataDir: t.TempDir()}
	if err := cfg.validate(); err == nil {
		t.Fatal("validate should fail without required stores")
	}
}

func TestLoadConfigErrors(t *testing.T) {
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("LoadConfig should fail for a missing file")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("listen: ["), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("LoadConfig should fail for invalid yaml")
	}
}

func TestMergeFileConfigKeepsRuntimeOverrides(t *testing.T) {
	runtimeCfg := Config{
		ListenAddr: ":9999",
		Stores: map[string]stores.Config{
			"runtime": {Kind: "keyvalue", Backend: "memory"},
		},
		Gears: GearsConfig{
			Store: "runtime-gears",
			RegistrationTokens: map[string]gears.RegistrationToken{
				"runtime": {Role: gears.GearRoleAdmin},
			},
		},
		Depots: DepotsConfig{Store: "runtime-depots"},
	}
	fileCfg := ConfigFile{
		ListenAddr: ":1234",
		Stores: map[string]stores.Config{
			"file": {Kind: "keyvalue", Backend: "memory"},
		},
		Gears: GearsConfig{
			Store: "file-gears",
			RegistrationTokens: map[string]gears.RegistrationToken{
				"file": {Role: gears.GearRoleAdmin},
			},
		},
		Depots: DepotsConfig{Store: "file-depots"},
	}

	merged := mergeFileConfig(runtimeCfg, fileCfg)
	if merged.ListenAddr != ":9999" {
		t.Fatalf("ListenAddr = %q", merged.ListenAddr)
	}
	if len(merged.Stores) != 1 || merged.Stores["runtime"].Backend != "memory" {
		t.Fatalf("Stores = %+v", merged.Stores)
	}
	if merged.Gears.Store != "runtime-gears" {
		t.Fatalf("Gears.Store = %q", merged.Gears.Store)
	}
	if len(merged.Gears.RegistrationTokens) != 1 || merged.Gears.RegistrationTokens["runtime"].Role != gears.GearRoleAdmin {
		t.Fatalf("RegistrationTokens = %+v", merged.Gears.RegistrationTokens)
	}
	if merged.Depots.Store != "runtime-depots" {
		t.Fatalf("Depots.Store = %q", merged.Depots.Store)
	}
}

func TestEffectiveListenAddr(t *testing.T) {
	if got := (Config{}).effectiveListenAddr(); got != ":9820" {
		t.Fatalf("effectiveListenAddr() = %q", got)
	}
	if got := (Config{ListenAddr: ":7777"}).effectiveListenAddr(); got != ":7777" {
		t.Fatalf("effectiveListenAddr(runtime) = %q", got)
	}
}

func TestValidateReportsSpecificMissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "missing data dir",
			cfg:  Config{Gears: GearsConfig{Store: "g"}, Depots: DepotsConfig{Store: "d"}},
			want: "server: empty data dir",
		},
		{
			name: "missing gears store",
			cfg:  Config{DataDir: t.TempDir(), Depots: DepotsConfig{Store: "d"}},
			want: "server: gears.store is required",
		},
		{
			name: "missing depots store",
			cfg:  Config{DataDir: t.TempDir(), Gears: GearsConfig{Store: "g"}},
			want: "server: depots.store is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.validate()
			if err == nil || err.Error() != tc.want {
				t.Fatalf("validate error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestNewConfigLoadError(t *testing.T) {
	_, err := New(Config{ConfigPath: filepath.Join(t.TempDir(), "missing.yaml")})
	if err == nil || !strings.Contains(err.Error(), "server: load config:") {
		t.Fatalf("New error = %v", err)
	}
}

func TestNewRejectsUnknownStores(t *testing.T) {
	dir := t.TempDir()

	_, err := New(Config{
		DataDir: dir,
		Stores: map[string]stores.Config{
			"bad": {Kind: "keyvalue", Backend: "unknown"},
		},
		Gears:  GearsConfig{Store: "bad"},
		Depots: DepotsConfig{Store: "bad"},
	})
	if err == nil || !strings.Contains(err.Error(), "server: stores:") {
		t.Fatalf("New error = %v", err)
	}
}

func TestNewRejectsMissingNamedStores(t *testing.T) {
	dir := t.TempDir()

	_, err := New(Config{
		DataDir: dir,
		Stores: map[string]stores.Config{
			"fw": {Kind: "filestore", Backend: "filesystem", Dir: "firmware"},
		},
		Gears:  GearsConfig{Store: "missing"},
		Depots: DepotsConfig{Store: "fw"},
	})
	if err == nil || !strings.Contains(err.Error(), "server: gears store:") {
		t.Fatalf("New error = %v", err)
	}

	_, err = New(Config{
		DataDir: dir,
		Stores: map[string]stores.Config{
			"mem": {Kind: "keyvalue", Backend: "memory"},
		},
		Gears:  GearsConfig{Store: "mem"},
		Depots: DepotsConfig{Store: "missing"},
	})
	if err == nil || !strings.Contains(err.Error(), "server: firmware store:") {
		t.Fatalf("New error = %v", err)
	}
}

func TestSecurityPolicyAllowsAdminServicesForActiveAdmin(t *testing.T) {
	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error = %v", err)
	}
	service := gears.NewService(gears.NewStore(kv.NewMemory(nil)), map[string]gears.RegistrationToken{
		"admin_default": {Role: gears.GearRoleAdmin},
	})
	_, err = service.Register(context.Background(), gears.RegistrationRequest{
		PublicKey:         keyPair.Public.String(),
		RegistrationToken: "admin_default",
	})
	if err != nil {
		t.Fatalf("Register error = %v", err)
	}

	policy := gizclaw.GearsSecurityPolicy{Gears: service}
	if !policy.AllowPeerService(keyPair.Public, gizclaw.ServiceAdmin) {
		t.Fatal("admin policy should allow admin service")
	}
}
