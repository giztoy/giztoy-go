package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GizClaw/gizclaw-go/cmd/internal/stores"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ListenAddr != ":9820" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
}

func TestNewWithPreparedConfig(t *testing.T) {
	dir := t.TempDir()
	srv, err := New(Config{
		ListenAddr: ":1234",
		Stores: map[string]stores.Config{
			"mem": {Kind: stores.KindKeyValue, Backend: "memory"},
			"fw":  {Kind: stores.KindFS, Backend: "filesystem", Dir: filepath.Join(dir, "firmware")},
		},
		Gears: GearsConfig{
			Store: "mem",
			RegistrationTokens: map[string]RegistrationTokenConfig{
				"admin_default": {Role: gearservice.GearRoleAdmin},
			},
		},
		Depots: DepotsConfig{Store: "fw"},
	})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	if srv.GearStore == nil {
		t.Fatal("GearStore is nil")
	}
	if srv.DepotStore == nil {
		t.Fatal("DepotStore is nil")
	}
	if srv.PublicKey().String() == "" {
		t.Fatal("PublicKey should not be empty")
	}
}

func TestConfigValidateRequiresStores(t *testing.T) {
	cfg := Config{}
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
			RegistrationTokens: map[string]RegistrationTokenConfig{
				"runtime": {Role: gearservice.GearRoleAdmin},
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
			RegistrationTokens: map[string]RegistrationTokenConfig{
				"file": {Role: gearservice.GearRoleAdmin},
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
	if len(merged.Gears.RegistrationTokens) != 1 || merged.Gears.RegistrationTokens["runtime"].Role != gearservice.GearRoleAdmin {
		t.Fatalf("RegistrationTokens = %+v", merged.Gears.RegistrationTokens)
	}
	if merged.Depots.Store != "runtime-depots" {
		t.Fatalf("Depots.Store = %q", merged.Depots.Store)
	}
}

func TestValidateReportsSpecificMissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "missing gears store",
			cfg:  Config{Depots: DepotsConfig{Store: "d"}},
			want: "server: gears.store is required",
		},
		{
			name: "missing depots store",
			cfg:  Config{Gears: GearsConfig{Store: "g"}},
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

func TestPrepareConfigGeneratesKeyPairAndDefaultListenAddr(t *testing.T) {
	cfg, err := prepareConfig(Config{
		Gears:  GearsConfig{Store: "g"},
		Depots: DepotsConfig{Store: "d"},
	})
	if err != nil {
		t.Fatalf("prepareConfig error = %v", err)
	}
	if cfg.KeyPair == nil {
		t.Fatal("KeyPair should be generated")
	}
	if cfg.ListenAddr != DefaultConfig().ListenAddr {
		t.Fatalf("ListenAddr = %q, want %q", cfg.ListenAddr, DefaultConfig().ListenAddr)
	}
}

func TestNewRejectsUnknownStores(t *testing.T) {
	_, err := New(Config{
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
		Stores: map[string]stores.Config{
			"fw": {Kind: "filestore", Backend: "filesystem", Dir: filepath.Join(dir, "firmware")},
		},
		Gears:  GearsConfig{Store: "missing"},
		Depots: DepotsConfig{Store: "fw"},
	})
	if err == nil || !strings.Contains(err.Error(), "server: gears store:") {
		t.Fatalf("New error = %v", err)
	}

	_, err = New(Config{
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
	service := &gear.Server{Store: kv.NewMemory(nil)}
	if _, err := service.SaveGear(context.Background(), gearservice.Gear{
		PublicKey:     keyPair.Public.String(),
		Role:          gearservice.GearRoleAdmin,
		Status:        gearservice.GearStatusActive,
		Device:        gearservice.DeviceInfo{},
		Configuration: gearservice.Configuration{},
	}); err != nil {
		t.Fatalf("SaveGear error = %v", err)
	}

	policy := gizclaw.GearsSecurityPolicy{Gears: service}
	if !policy.AllowPeerService(keyPair.Public, gizclaw.ServiceAdmin) {
		t.Fatal("admin policy should allow admin service")
	}
}
