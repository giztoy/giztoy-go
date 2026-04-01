package server

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/giztoy/giztoy-go/internal/stores"
	"github.com/giztoy/giztoy-go/pkg/gears"
)

type GearsConfig struct {
	Store              string                             `yaml:"store"`
	RegistrationTokens map[string]gears.RegistrationToken `yaml:"registration-tokens"`
}

type DepotsConfig struct {
	Store string `yaml:"store"`
}

type FileConfig struct {
	ListenAddr string                  `yaml:"listen"`
	Stores     map[string]stores.Config `yaml:"stores"`
	Gears      GearsConfig             `yaml:"gears"`
	Depots     DepotsConfig            `yaml:"depots"`
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

func (cfg Config) validate() error {
	if cfg.DataDir == "" {
		return fmt.Errorf("server: empty data dir")
	}
	if cfg.Gears.Store == "" {
		return fmt.Errorf("server: gears.store is required")
	}
	if cfg.Depots.Store == "" {
		return fmt.Errorf("server: depots.store is required")
	}
	return nil
}
