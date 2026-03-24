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

type StoreConfig struct {
	Kind string `yaml:"kind"`
	Dir  string `yaml:"dir"`
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

func (cfg Config) gearsStore() (kv.Store, error) {
	if cfg.KVStore != nil {
		return cfg.KVStore, nil
	}
	if cfg.Gears.Store != "" {
		storeCfg, ok := cfg.Stores[cfg.Gears.Store]
		if !ok {
			return nil, fmt.Errorf("server: gears.store %q not found", cfg.Gears.Store)
		}
		switch storeCfg.Kind {
		case "memory":
			return kv.NewMemory(nil), nil
		case "badger":
			if storeCfg.Dir == "" {
				return nil, fmt.Errorf("server: gears store %q (badger) missing dir", cfg.Gears.Store)
			}
			dir, err := cfg.workspacePath(storeCfg.Dir)
			if err != nil {
				return nil, err
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("server: gears store dir: %w", err)
			}
			return kv.NewBadger(dir, nil)
		default:
			return nil, fmt.Errorf("server: unsupported gears store kind %q", storeCfg.Kind)
		}
	}
	return kv.NewMemory(nil), nil
}

func (cfg Config) firmwareStore() (*firmware.Store, error) {
	if cfg.FirmwareStore != nil {
		return cfg.FirmwareStore, nil
	}
	if cfg.Depots.Store != "" {
		storeCfg, ok := cfg.Stores[cfg.Depots.Store]
		if !ok {
			return nil, fmt.Errorf("server: depots.store %q not found", cfg.Depots.Store)
		}
		if storeCfg.Kind != "file" {
			return nil, fmt.Errorf("server: unsupported depots store kind %q", storeCfg.Kind)
		}
		if storeCfg.Dir == "" {
			return nil, fmt.Errorf("server: depots store %q missing dir", cfg.Depots.Store)
		}
		root, err := cfg.workspacePath(storeCfg.Dir)
		if err != nil {
			return nil, err
		}
		return firmware.NewStore(root), nil
	}
	root := cfg.FirmwareRoot
	if root == "" {
		root = filepath.Join(cfg.DataDir, "firmware")
	}
	return firmware.NewStore(root), nil
}

func (cfg Config) validate() error {
	if cfg.DataDir == "" {
		return fmt.Errorf("server: empty data dir")
	}
	if cfg.Gears.Store != "" {
		storeCfg, ok := cfg.Stores[cfg.Gears.Store]
		if !ok {
			return fmt.Errorf("server: gears.store %q not found", cfg.Gears.Store)
		}
		if storeCfg.Kind == "" {
			return fmt.Errorf("server: gears store %q missing kind", cfg.Gears.Store)
		}
		if storeCfg.Kind == "badger" {
			if storeCfg.Dir == "" {
				return fmt.Errorf("server: gears store %q (badger) missing dir", cfg.Gears.Store)
			}
			if _, err := cfg.workspacePath(storeCfg.Dir); err != nil {
				return err
			}
		}
	}
	if cfg.Depots.Store != "" {
		storeCfg, ok := cfg.Stores[cfg.Depots.Store]
		if !ok {
			return fmt.Errorf("server: depots.store %q not found", cfg.Depots.Store)
		}
		if storeCfg.Kind != "file" {
			return fmt.Errorf("server: depots store %q must use file kind", cfg.Depots.Store)
		}
		if storeCfg.Dir == "" {
			return fmt.Errorf("server: depots store %q missing dir", cfg.Depots.Store)
		}
		if _, err := cfg.workspacePath(storeCfg.Dir); err != nil {
			return err
		}
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
