package context

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/giztoy/giztoy-go/internal/identity"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

// ServerConfig holds the connection info for a remote server.
type ServerConfig struct {
	Address   string `yaml:"address"`
	PublicKey string `yaml:"public-key"`
}

// Config is the per-context configuration stored in config.yaml.
type Config struct {
	Server ServerConfig `yaml:"server"`
}

// Context represents a loaded context directory.
type Context struct {
	Name    string
	Dir     string
	Config  Config
	KeyPair *noise.KeyPair
}

// Load reads a context from its directory.
func Load(dir string) (*Context, error) {
	name := filepath.Base(dir)

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("context: read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("context: parse config: %w", err)
	}

	kp, err := identity.Load(filepath.Join(dir, "identity.key"))
	if err != nil {
		return nil, fmt.Errorf("context: load identity: %w", err)
	}

	return &Context{Name: name, Dir: dir, Config: cfg, KeyPair: kp}, nil
}

// ServerPublicKey parses and returns the server's public key.
func (c *Context) ServerPublicKey() (noise.PublicKey, error) {
	return noise.KeyFromHex(c.Config.Server.PublicKey)
}
