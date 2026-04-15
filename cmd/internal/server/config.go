package server

import (
	"fmt"
	"os"

	"github.com/GizClaw/gizclaw-go/cmd/internal/stores"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/goccy/go-yaml"
)

type Config struct {
	KeyPair    *giznet.KeyPair
	ListenAddr string
	Stores     map[string]stores.Config
	Gears      GearsConfig
	Depots     DepotsConfig
}

type GearsConfig struct {
	Store              string                             `yaml:"store"`
	RegistrationTokens map[string]RegistrationTokenConfig `yaml:"registration-tokens"`
}

type RegistrationTokenConfig struct {
	Role gearservice.GearRole `yaml:"role"`
}

type DepotsConfig struct {
	Store string `yaml:"store"`
}

type ConfigFile struct {
	ListenAddr string                   `yaml:"listen"`
	Stores     map[string]stores.Config `yaml:"stores"`
	Gears      GearsConfig              `yaml:"gears"`
	Depots     DepotsConfig             `yaml:"depots"`
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

func prepareConfig(cfg Config) (Config, error) {
	defaults := DefaultConfig()
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaults.ListenAddr
	}
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
	return nil
}
