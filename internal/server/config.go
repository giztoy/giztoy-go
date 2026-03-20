package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/haivivi/giztoy/go/pkg/firmware"
	"github.com/haivivi/giztoy/go/pkg/gears"
	"github.com/haivivi/giztoy/go/pkg/kv"
)

// StoreConfig defines a named store in the config file.
//
// Kind is the interface type ("keyvalue", "sql", or "filestore").
// Backend is the concrete implementation ("memory", "badger", "sqlite", "postgres", "filesystem").
// Dir is required for backends that need disk storage (badger, sqlite, filesystem).
// DSN is the connection string for remote backends (postgres).
type StoreConfig struct {
	Kind    string `yaml:"kind"`
	Backend string `yaml:"backend"`
	Dir     string `yaml:"dir"`
	DSN     string `yaml:"dsn"`
}

type GearsConfig struct {
	Store              string                             `yaml:"store"`
	RegistrationTokens map[string]gears.RegistrationToken `yaml:"registration-tokens"`
}

type DepotsConfig struct {
	Store string `yaml:"store"`
}

type FileConfig struct {
	ListenAddr string                 `yaml:"listen"`
	Stores     map[string]StoreConfig `yaml:"stores"`
	Gears      GearsConfig            `yaml:"gears"`
	Depots     DepotsConfig           `yaml:"depots"`
}

func LoadConfig(path string) (FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileConfig{}, err
	}
	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return FileConfig{}, err
	}
	return cfg, nil
}

func mergeFileConfig(cfg Config, fileCfg FileConfig) Config {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = fileCfg.ListenAddr
	}
	if len(cfg.Stores) == 0 {
		cfg.Stores = fileCfg.Stores
	}
	cfg.Gears = mergeGearsConfig(cfg.Gears, fileCfg.Gears)
	cfg.Depots = mergeDepotsConfig(cfg.Depots, fileCfg.Depots)
	return cfg
}

func mergeGearsConfig(runtime GearsConfig, file GearsConfig) GearsConfig {
	if runtime.Store == "" {
		runtime.Store = file.Store
	}
	if len(runtime.RegistrationTokens) == 0 {
		runtime.RegistrationTokens = file.RegistrationTokens
	}
	return runtime
}

func mergeDepotsConfig(runtime DepotsConfig, file DepotsConfig) DepotsConfig {
	if runtime.Store == "" {
		runtime.Store = file.Store
	}
	return runtime
}

func (cfg Config) effectiveListenAddr() string {
	if cfg.ListenAddr != "" {
		return cfg.ListenAddr
	}
	return ":9820"
}

// kvCompatibleKind reports whether the store kind can serve as a kv.Store.
func kvCompatibleKind(kind string) bool {
	return kind == "keyvalue" || kind == "sql"
}

// resolveKVStore creates a kv.Store from a named store config entry.
// The referenced store must have kind "keyvalue" or "sql".
func (cfg Config) resolveKVStore(name string) (kv.Store, error) {
	sc, ok := cfg.Stores[name]
	if !ok {
		return nil, fmt.Errorf("server: store %q not found", name)
	}
	if !kvCompatibleKind(sc.Kind) {
		return nil, fmt.Errorf("server: store %q is kind %q, need keyvalue or sql", name, sc.Kind)
	}
	switch sc.Backend {
	case "memory":
		return kv.NewMemory(nil), nil
	case "badger":
		dir, err := cfg.workspacePath(sc.Dir)
		if err != nil {
			return nil, fmt.Errorf("server: store %q: %w", name, err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("server: store %q dir: %w", name, err)
		}
		return kv.NewBadger(dir, nil)
	case "sqlite":
		dir, err := cfg.workspacePath(sc.Dir)
		if err != nil {
			return nil, fmt.Errorf("server: store %q: %w", name, err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("server: store %q dir: %w", name, err)
		}
		return kv.NewSQLite(dir, nil)
	case "postgres":
		return kv.NewPostgres(sc.DSN, nil)
	default:
		return nil, fmt.Errorf("server: store %q has unsupported backend %q for kind %q", name, sc.Backend, sc.Kind)
	}
}

// resolveFileStore creates a firmware.Store from a named store config entry.
// The referenced store must have kind "filestore".
func (cfg Config) resolveFileStore(name string) (*firmware.Store, error) {
	sc, ok := cfg.Stores[name]
	if !ok {
		return nil, fmt.Errorf("server: store %q not found", name)
	}
	if sc.Kind != "filestore" {
		return nil, fmt.Errorf("server: store %q is kind %q, need filestore", name, sc.Kind)
	}
	if sc.Backend != "filesystem" {
		return nil, fmt.Errorf("server: store %q has unsupported filestore backend %q", name, sc.Backend)
	}
	dir, err := cfg.workspacePath(sc.Dir)
	if err != nil {
		return nil, fmt.Errorf("server: store %q: %w", name, err)
	}
	return firmware.NewStore(dir), nil
}

func (cfg Config) gearsStore() (kv.Store, error) {
	if cfg.KVStore != nil {
		return cfg.KVStore, nil
	}
	if cfg.Gears.Store != "" {
		return cfg.resolveKVStore(cfg.Gears.Store)
	}
	return kv.NewMemory(nil), nil
}

func (cfg Config) firmwareStore() (*firmware.Store, error) {
	if cfg.FirmwareStore != nil {
		return cfg.FirmwareStore, nil
	}
	if cfg.Depots.Store != "" {
		return cfg.resolveFileStore(cfg.Depots.Store)
	}
	root := cfg.FirmwareRoot
	if root == "" {
		root = filepath.Join(cfg.DataDir, "firmware")
	}
	return firmware.NewStore(root), nil
}

// needsDir returns true for backends that require a local directory path.
// postgres uses DSN instead of Dir, so it is not included.
func needsDir(backend string) bool {
	switch backend {
	case "badger", "sqlite", "filesystem":
		return true
	}
	return false
}

func (cfg Config) validate() error {
	if cfg.DataDir == "" {
		return fmt.Errorf("server: empty data dir")
	}
	for name, sc := range cfg.Stores {
		if err := validateStoreConfig(name, sc); err != nil {
			return err
		}
		if needsDir(sc.Backend) {
			if _, err := cfg.workspacePath(sc.Dir); err != nil {
				return fmt.Errorf("server: store %q: %w", name, err)
			}
		}
	}
	if cfg.Gears.Store != "" {
		sc, ok := cfg.Stores[cfg.Gears.Store]
		if !ok {
			return fmt.Errorf("server: gears.store %q not found", cfg.Gears.Store)
		}
		if !kvCompatibleKind(sc.Kind) {
			return fmt.Errorf("server: gears.store %q must be kind keyvalue or sql, got %q", cfg.Gears.Store, sc.Kind)
		}
	}
	if cfg.Depots.Store != "" {
		sc, ok := cfg.Stores[cfg.Depots.Store]
		if !ok {
			return fmt.Errorf("server: depots.store %q not found", cfg.Depots.Store)
		}
		if sc.Kind != "filestore" {
			return fmt.Errorf("server: depots.store %q must be kind filestore, got %q", cfg.Depots.Store, sc.Kind)
		}
	}
	return nil
}

var validBackends = map[string]map[string]bool{
	"keyvalue":  {"memory": true, "badger": true},
	"sql":       {"sqlite": true, "postgres": true},
	"filestore": {"filesystem": true},
}

func validateStoreConfig(name string, sc StoreConfig) error {
	if sc.Kind == "" {
		return fmt.Errorf("server: store %q missing kind", name)
	}
	backends, ok := validBackends[sc.Kind]
	if !ok {
		return fmt.Errorf("server: store %q has unknown kind %q", name, sc.Kind)
	}
	if sc.Backend == "" {
		return fmt.Errorf("server: store %q missing backend", name)
	}
	if !backends[sc.Backend] {
		return fmt.Errorf("server: store %q has invalid backend %q for kind %q", name, sc.Backend, sc.Kind)
	}
	if needsDir(sc.Backend) && sc.Dir == "" {
		return fmt.Errorf("server: store %q (backend %q) requires dir", name, sc.Backend)
	}
	if sc.Backend == "postgres" && sc.DSN == "" {
		return fmt.Errorf("server: store %q (backend %q) requires dsn", name, sc.Backend)
	}
	return nil
}

func (cfg Config) workspacePath(path string) (string, error) {
	if cfg.DataDir == "" {
		return "", fmt.Errorf("server: empty data dir")
	}

	if filepath.IsAbs(path) {
		resolved, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("server: resolve path %q: %w", path, err)
		}
		return resolved, nil
	}

	workspace, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return "", fmt.Errorf("server: resolve workspace %q: %w", cfg.DataDir, err)
	}

	resolved, err := filepath.Abs(filepath.Join(workspace, path))
	if err != nil {
		return "", fmt.Errorf("server: resolve path %q: %w", path, err)
	}
	return resolved, nil
}
