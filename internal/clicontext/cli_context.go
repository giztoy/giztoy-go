package clicontext

import (
	"fmt"
	"github.com/giztoy/giztoy-go/internal/identity"
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"github.com/goccy/go-yaml"
	"os"
	"path/filepath"
)

// ServerConfig holds the connection info for a remote server.
type ServerConfig struct {
	Address   string `yaml:"address"`
	PublicKey string `yaml:"public-key"`
}

// Config is the per-cli-context configuration stored in config.yaml.
type Config struct {
	Server ServerConfig `yaml:"server"`
}

// CLIContext represents a loaded CLI context directory.
type CLIContext struct {
	Name    string
	Dir     string
	Config  Config
	KeyPair *giznet.KeyPair
}

// Load reads a CLI context from its directory.
func Load(dir string) (*CLIContext, error) {
	name := filepath.Base(dir)

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("clicontext: read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("clicontext: parse config: %w", err)
	}

	kp, err := identity.Load(filepath.Join(dir, "identity.key"))
	if err != nil {
		return nil, fmt.Errorf("clicontext: load identity: %w", err)
	}

	return &CLIContext{Name: name, Dir: dir, Config: cfg, KeyPair: kp}, nil
}

// ServerPublicKey parses and returns the server's public key.
func (c *CLIContext) ServerPublicKey() (giznet.PublicKey, error) {
	return giznet.KeyFromHex(c.Config.Server.PublicKey)
}
