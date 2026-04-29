package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/cmd/internal/storage"
	"github.com/GizClaw/gizclaw-go/cmd/internal/stores"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ListenAddr != ":9820" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
}

func TestNewWithLayeredStorageConfig(t *testing.T) {
	dir := t.TempDir()
	srv, err := New(validLayeredConfig(dir))
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	if srv.GearStore == nil || srv.CredentialStore == nil || srv.MiniMaxTenantStore == nil || srv.VoiceStore == nil || srv.WorkspaceStore == nil || srv.WorkspaceTemplateStore == nil || srv.TemplateStore == nil {
		t.Fatalf("module stores not wired: %+v", srv)
	}
	if srv.DepotStore == nil {
		t.Fatal("DepotStore is nil")
	}
}

func TestNewWithLayeredStorageReportsStoreErrors(t *testing.T) {
	dir := t.TempDir()

	storageErrCfg := validLayeredConfig(dir)
	storageErrCfg.Storage["memory"] = storage.Config{Kind: storage.KindKeyValue, Backend: "redis"}
	if _, err := New(storageErrCfg); err == nil || !strings.Contains(err.Error(), "server: stores:") {
		t.Fatalf("New(storage error) = %v", err)
	}

	logicalErrCfg := validLayeredConfig(dir)
	logicalErrCfg.Stores["credentials"] = stores.Config{Kind: stores.KindKeyValue, Storage: "memory", Prefix: "bad:prefix"}
	if _, err := New(logicalErrCfg); err == nil || !strings.Contains(err.Error(), "server: stores:") {
		t.Fatalf("New(logical store error) = %v", err)
	}

	missingCredentialCfg := validLayeredConfig(dir)
	delete(missingCredentialCfg.Stores, "credentials")
	if _, err := New(missingCredentialCfg); err == nil || !strings.Contains(err.Error(), "server: credentials store:") {
		t.Fatalf("New(missing credentials store) = %v", err)
	}

	missingMiniMaxCredentialCfg := validLayeredConfig(dir)
	missingMiniMaxCredentialCfg.MiniMax.CredentialsStore = "missing"
	if _, err := New(missingMiniMaxCredentialCfg); err == nil || !strings.Contains(err.Error(), "server: minimax credentials store:") {
		t.Fatalf("New(missing minimax credentials store) = %v", err)
	}

	missingTenantCfg := validLayeredConfig(dir)
	missingTenantCfg.MiniMax.TenantsStore = "missing"
	if _, err := New(missingTenantCfg); err == nil || !strings.Contains(err.Error(), "server: minimax tenants store:") {
		t.Fatalf("New(missing tenant store) = %v", err)
	}

	missingVoicesCfg := validLayeredConfig(dir)
	missingVoicesCfg.MiniMax.VoicesStore = "missing"
	if _, err := New(missingVoicesCfg); err == nil || !strings.Contains(err.Error(), "server: voices store:") {
		t.Fatalf("New(missing voices store) = %v", err)
	}

	missingWorkspacesCfg := validLayeredConfig(dir)
	missingWorkspacesCfg.Workspaces.Store = "missing"
	if _, err := New(missingWorkspacesCfg); err == nil || !strings.Contains(err.Error(), "server: workspaces store:") {
		t.Fatalf("New(missing workspaces store) = %v", err)
	}

	missingWorkspaceTemplateRefCfg := validLayeredConfig(dir)
	missingWorkspaceTemplateRefCfg.Workspaces.TemplatesStore = "missing"
	if _, err := New(missingWorkspaceTemplateRefCfg); err == nil || !strings.Contains(err.Error(), "server: workspace template reference store:") {
		t.Fatalf("New(missing workspace template reference store) = %v", err)
	}

	missingTemplatesCfg := validLayeredConfig(dir)
	missingTemplatesCfg.WorkspaceTemplates.Store = "missing"
	if _, err := New(missingTemplatesCfg); err == nil || !strings.Contains(err.Error(), "server: workspace templates store:") {
		t.Fatalf("New(missing templates store) = %v", err)
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
				"admin_default": {Role: apitypes.GearRoleAdmin},
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
		Storage: map[string]storage.Config{
			"runtime-storage": {Kind: "keyvalue", Backend: "memory"},
		},
		Stores: map[string]stores.Config{
			"runtime": {Kind: "keyvalue", Backend: "memory"},
		},
		Gears: GearsConfig{
			Store: "runtime-gears",
			RegistrationTokens: map[string]RegistrationTokenConfig{
				"runtime": {Role: apitypes.GearRoleAdmin},
			},
		},
		Credentials: CredentialsConfig{Store: "runtime-credentials"},
		MiniMax: MiniMaxConfig{
			TenantsStore:     "runtime-tenants",
			VoicesStore:      "runtime-voices",
			CredentialsStore: "runtime-credentials",
		},
		Workspaces:         WorkspacesConfig{Store: "runtime-workspaces", TemplatesStore: "runtime-templates"},
		WorkspaceTemplates: WorkspaceTemplatesConfig{Store: "runtime-templates"},
		Depots:             DepotsConfig{Store: "runtime-depots"},
	}
	fileCfg := ConfigFile{
		ListenAddr: ":1234",
		Storage: map[string]storage.Config{
			"file-storage": {Kind: "keyvalue", Backend: "memory"},
		},
		Stores: map[string]stores.Config{
			"file": {Kind: "keyvalue", Backend: "memory"},
		},
		Gears: GearsConfig{
			Store: "file-gears",
			RegistrationTokens: map[string]RegistrationTokenConfig{
				"file": {Role: apitypes.GearRoleAdmin},
			},
		},
		Credentials: CredentialsConfig{Store: "file-credentials"},
		MiniMax: MiniMaxConfig{
			TenantsStore:     "file-tenants",
			VoicesStore:      "file-voices",
			CredentialsStore: "file-credentials",
		},
		Workspaces:         WorkspacesConfig{Store: "file-workspaces", TemplatesStore: "file-templates"},
		WorkspaceTemplates: WorkspaceTemplatesConfig{Store: "file-templates"},
		Depots:             DepotsConfig{Store: "file-depots"},
	}

	merged := mergeFileConfig(runtimeCfg, fileCfg)
	if merged.ListenAddr != ":9999" {
		t.Fatalf("ListenAddr = %q", merged.ListenAddr)
	}
	if len(merged.Stores) != 1 || merged.Stores["runtime"].Backend != "memory" {
		t.Fatalf("Stores = %+v", merged.Stores)
	}
	if len(merged.Storage) != 1 || merged.Storage["runtime-storage"].Backend != "memory" {
		t.Fatalf("Storage = %+v", merged.Storage)
	}
	if merged.Gears.Store != "runtime-gears" {
		t.Fatalf("Gears.Store = %q", merged.Gears.Store)
	}
	if len(merged.Gears.RegistrationTokens) != 1 || merged.Gears.RegistrationTokens["runtime"].Role != apitypes.GearRoleAdmin {
		t.Fatalf("RegistrationTokens = %+v", merged.Gears.RegistrationTokens)
	}
	if merged.Depots.Store != "runtime-depots" {
		t.Fatalf("Depots.Store = %q", merged.Depots.Store)
	}
	if merged.Credentials.Store != "runtime-credentials" {
		t.Fatalf("Credentials.Store = %q", merged.Credentials.Store)
	}
	if merged.MiniMax.TenantsStore != "runtime-tenants" || merged.MiniMax.VoicesStore != "runtime-voices" || merged.MiniMax.CredentialsStore != "runtime-credentials" {
		t.Fatalf("MiniMax = %+v", merged.MiniMax)
	}
	if merged.Workspaces.Store != "runtime-workspaces" || merged.Workspaces.TemplatesStore != "runtime-templates" {
		t.Fatalf("Workspaces = %+v", merged.Workspaces)
	}
	if merged.WorkspaceTemplates.Store != "runtime-templates" {
		t.Fatalf("WorkspaceTemplates.Store = %q", merged.WorkspaceTemplates.Store)
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

func TestValidateReportsLayeredStorageMissingFields(t *testing.T) {
	base := Config{
		Storage:            map[string]storage.Config{"memory": {Kind: storage.KindKeyValue, Memory: &storage.MemoryConfig{}}},
		Gears:              GearsConfig{Store: "gears"},
		Credentials:        CredentialsConfig{Store: "credentials"},
		MiniMax:            MiniMaxConfig{TenantsStore: "minimax-tenants", VoicesStore: "voices", CredentialsStore: "credentials"},
		Workspaces:         WorkspacesConfig{Store: "workspaces", TemplatesStore: "workspace-templates"},
		WorkspaceTemplates: WorkspaceTemplatesConfig{Store: "workspace-templates"},
		Depots:             DepotsConfig{Store: "firmware"},
	}
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"missing credentials", func(c *Config) { c.Credentials.Store = "" }, "server: credentials.store is required"},
		{"missing minimax tenants", func(c *Config) { c.MiniMax.TenantsStore = "" }, "server: minimax.tenants-store is required"},
		{"missing minimax voices", func(c *Config) { c.MiniMax.VoicesStore = "" }, "server: minimax.voices-store is required"},
		{"missing minimax credentials", func(c *Config) { c.MiniMax.CredentialsStore = "" }, "server: minimax.credentials-store is required"},
		{"missing workspaces", func(c *Config) { c.Workspaces.Store = "" }, "server: workspaces.store is required"},
		{"missing workspace template reference", func(c *Config) { c.Workspaces.TemplatesStore = "" }, "server: workspaces.templates-store is required"},
		{"missing workspace templates", func(c *Config) { c.WorkspaceTemplates.Store = "" }, "server: workspace-templates.store is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.edit(&cfg)
			err := cfg.validate()
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
	service := &gear.Server{Store: mustBadgerInMemory(t, nil)}
	if _, err := service.SaveGear(context.Background(), apitypes.Gear{
		PublicKey:     keyPair.Public.String(),
		Role:          apitypes.GearRoleAdmin,
		Status:        apitypes.GearStatusActive,
		Device:        apitypes.DeviceInfo{},
		Configuration: apitypes.Configuration{},
	}); err != nil {
		t.Fatalf("SaveGear error = %v", err)
	}

	policy := gizclaw.GearsSecurityPolicy{Gears: service}
	if !policy.AllowPeerService(keyPair.Public, gizclaw.ServiceAdmin) {
		t.Fatal("admin policy should allow admin service")
	}
}

func validLayeredConfig(dir string) Config {
	return Config{
		ListenAddr: ":1234",
		Storage: map[string]storage.Config{
			"memory":      {Kind: storage.KindKeyValue, Memory: &storage.MemoryConfig{}},
			"local-files": {Kind: storage.KindFilesystem, FS: &storage.FSConfig{Dir: dir}},
			"firmware-depot": {
				Kind:    storage.KindDepotStore,
				DepotFS: &storage.DepotFSConfig{},
			},
		},
		Stores: map[string]stores.Config{
			"gears":               {Kind: stores.KindKeyValue, Storage: "memory", Prefix: "gears"},
			"credentials":         {Kind: stores.KindKeyValue, Storage: "memory", Prefix: "credentials"},
			"minimax-tenants":     {Kind: stores.KindKeyValue, Storage: "memory", Prefix: "minimax-tenants"},
			"voices":              {Kind: stores.KindKeyValue, Storage: "memory", Prefix: "voices"},
			"workspaces":          {Kind: stores.KindKeyValue, Storage: "memory", Prefix: "workspaces"},
			"workspace-templates": {Kind: stores.KindKeyValue, Storage: "memory", Prefix: "workspace-templates"},
			"firmware": {
				Kind:    stores.KindDepotStore,
				Storage: "firmware-depot",
				DepotFS: &stores.DepotFSRef{
					Filesystem: storage.FilesystemRef{Storage: "local-files", BaseDir: "firmware"},
				},
			},
		},
		Gears:       GearsConfig{Store: "gears"},
		Credentials: CredentialsConfig{Store: "credentials"},
		MiniMax: MiniMaxConfig{
			TenantsStore:     "minimax-tenants",
			VoicesStore:      "voices",
			CredentialsStore: "credentials",
		},
		Workspaces:         WorkspacesConfig{Store: "workspaces", TemplatesStore: "workspace-templates"},
		WorkspaceTemplates: WorkspaceTemplatesConfig{Store: "workspace-templates"},
		Depots:             DepotsConfig{Store: "firmware"},
	}
}
