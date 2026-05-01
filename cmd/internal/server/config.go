package server

import (
	"fmt"
	"os"

	"github.com/GizClaw/gizclaw-go/cmd/internal/storage"
	"github.com/GizClaw/gizclaw-go/cmd/internal/stores"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/goccy/go-yaml"
)

type Config struct {
	KeyPair            *giznet.KeyPair
	ListenAddr         string
	AdminPublicKey     string
	Storage            map[string]storage.Config
	Stores             map[string]stores.Config
	Gears              GearsConfig
	Credentials        CredentialsConfig
	MiniMax            MiniMaxConfig
	Workspaces         WorkspacesConfig
	WorkspaceTemplates WorkspaceTemplatesConfig
	Depots             DepotsConfig
}

type GearsConfig struct {
	Store string `yaml:"store"`
}

type CredentialsConfig struct {
	Store string `yaml:"store"`
}

type MiniMaxConfig struct {
	TenantsStore     string `yaml:"tenants-store"`
	VoicesStore      string `yaml:"voices-store"`
	CredentialsStore string `yaml:"credentials-store"`
}

type WorkspacesConfig struct {
	Store          string `yaml:"store"`
	TemplatesStore string `yaml:"templates-store"`
}

type WorkspaceTemplatesConfig struct {
	Store string `yaml:"store"`
}

type DepotsConfig struct {
	Store         string `yaml:"store"`
	MetadataStore string `yaml:"metadata-store"`
}

type ConfigFile struct {
	ListenAddr         string                    `yaml:"listen"`
	AdminPublicKey     string                    `yaml:"admin-public-key"`
	Storage            map[string]storage.Config `yaml:"storage"`
	Stores             map[string]stores.Config  `yaml:"stores"`
	Gears              GearsConfig               `yaml:"gears"`
	Credentials        CredentialsConfig         `yaml:"credentials"`
	MiniMax            MiniMaxConfig             `yaml:"minimax"`
	Workspaces         WorkspacesConfig          `yaml:"workspaces"`
	WorkspaceTemplates WorkspaceTemplatesConfig  `yaml:"workspace-templates"`
	Depots             DepotsConfig              `yaml:"depots"`
}

func LoadConfig(path string) (ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ConfigFile{}, err
	}
	var cfg ConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ConfigFile{}, err
	}
	return cfg, nil
}

func DefaultConfig() Config {
	return Config{
		ListenAddr: ":9820",
	}
}

func mergeFileConfig(cfg Config, fileCfg ConfigFile) Config {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = fileCfg.ListenAddr
	}
	if cfg.AdminPublicKey == "" {
		cfg.AdminPublicKey = fileCfg.AdminPublicKey
	}
	if len(cfg.Stores) == 0 {
		cfg.Stores = fileCfg.Stores
	}
	if len(cfg.Storage) == 0 {
		cfg.Storage = fileCfg.Storage
	}
	cfg.Gears = mergeGearsConfig(cfg.Gears, fileCfg.Gears)
	cfg.Credentials = mergeCredentialsConfig(cfg.Credentials, fileCfg.Credentials)
	cfg.MiniMax = mergeMiniMaxConfig(cfg.MiniMax, fileCfg.MiniMax)
	cfg.Workspaces = mergeWorkspacesConfig(cfg.Workspaces, fileCfg.Workspaces)
	cfg.WorkspaceTemplates = mergeWorkspaceTemplatesConfig(cfg.WorkspaceTemplates, fileCfg.WorkspaceTemplates)
	cfg.Depots = mergeDepotsConfig(cfg.Depots, fileCfg.Depots)
	return cfg
}

func mergeGearsConfig(runtime GearsConfig, file GearsConfig) GearsConfig {
	if runtime.Store == "" {
		runtime.Store = file.Store
	}
	return runtime
}

func mergeDepotsConfig(runtime DepotsConfig, file DepotsConfig) DepotsConfig {
	if runtime.Store == "" {
		runtime.Store = file.Store
	}
	if runtime.MetadataStore == "" {
		runtime.MetadataStore = file.MetadataStore
	}
	return runtime
}

func mergeCredentialsConfig(runtime CredentialsConfig, file CredentialsConfig) CredentialsConfig {
	if runtime.Store == "" {
		runtime.Store = file.Store
	}
	return runtime
}

func mergeMiniMaxConfig(runtime MiniMaxConfig, file MiniMaxConfig) MiniMaxConfig {
	if runtime.TenantsStore == "" {
		runtime.TenantsStore = file.TenantsStore
	}
	if runtime.VoicesStore == "" {
		runtime.VoicesStore = file.VoicesStore
	}
	if runtime.CredentialsStore == "" {
		runtime.CredentialsStore = file.CredentialsStore
	}
	return runtime
}

func mergeWorkspacesConfig(runtime WorkspacesConfig, file WorkspacesConfig) WorkspacesConfig {
	if runtime.Store == "" {
		runtime.Store = file.Store
	}
	if runtime.TemplatesStore == "" {
		runtime.TemplatesStore = file.TemplatesStore
	}
	return runtime
}

func mergeWorkspaceTemplatesConfig(runtime WorkspaceTemplatesConfig, file WorkspaceTemplatesConfig) WorkspaceTemplatesConfig {
	if runtime.Store == "" {
		runtime.Store = file.Store
	}
	return runtime
}

func prepareConfig(cfg Config) (Config, error) {
	defaults := DefaultConfig()
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaults.ListenAddr
	}
	adminPublicKey, err := normalizeAdminPublicKey(cfg.AdminPublicKey)
	if err != nil {
		return Config{}, err
	}
	cfg.AdminPublicKey = adminPublicKey
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	if cfg.KeyPair == nil {
		keyPair, err := giznet.GenerateKeyPair()
		if err != nil {
			return Config{}, fmt.Errorf("server: generate key pair: %w", err)
		}
		cfg.KeyPair = keyPair
	}
	return cfg, nil
}

func (cfg Config) validate() error {
	if cfg.Gears.Store == "" {
		return fmt.Errorf("server: gears.store is required")
	}
	if cfg.Depots.Store == "" {
		return fmt.Errorf("server: depots.store is required")
	}
	if _, err := normalizeAdminPublicKey(cfg.AdminPublicKey); err != nil {
		return err
	}
	if len(cfg.Storage) == 0 {
		return nil
	}
	if cfg.Credentials.Store == "" {
		return fmt.Errorf("server: credentials.store is required")
	}
	if cfg.MiniMax.TenantsStore == "" {
		return fmt.Errorf("server: minimax.tenants-store is required")
	}
	if cfg.MiniMax.VoicesStore == "" {
		return fmt.Errorf("server: minimax.voices-store is required")
	}
	if cfg.MiniMax.CredentialsStore == "" {
		return fmt.Errorf("server: minimax.credentials-store is required")
	}
	if cfg.Workspaces.Store == "" {
		return fmt.Errorf("server: workspaces.store is required")
	}
	if cfg.Workspaces.TemplatesStore == "" {
		return fmt.Errorf("server: workspaces.templates-store is required")
	}
	if cfg.WorkspaceTemplates.Store == "" {
		return fmt.Errorf("server: workspace-templates.store is required")
	}
	if cfg.Depots.MetadataStore == "" {
		return fmt.Errorf("server: depots.metadata-store is required")
	}
	return nil
}

func normalizeAdminPublicKey(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	adminPublicKey, err := giznet.KeyFromHex(value)
	if err != nil {
		return "", fmt.Errorf("server: invalid admin-public-key: %w", err)
	}
	if adminPublicKey.IsZero() {
		return "", fmt.Errorf("server: invalid admin-public-key: zero key")
	}
	return adminPublicKey.String(), nil
}
