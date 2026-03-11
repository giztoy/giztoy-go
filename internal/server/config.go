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
		return firmware.NewStore(storeCfg.Dir), nil
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
	if cfg.AdminServiceID == 0 {
		cfg.AdminServiceID = 1
	}
	if cfg.ReverseServiceID == 0 {
		cfg.ReverseServiceID = 2
	}
	if cfg.Gears.Store != "" {
		storeCfg, ok := cfg.Stores[cfg.Gears.Store]
		if !ok {
			return fmt.Errorf("server: gears.store %q not found", cfg.Gears.Store)
		}
		if storeCfg.Kind == "" {
			return fmt.Errorf("server: gears store %q missing kind", cfg.Gears.Store)
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
	}
	return nil
}
