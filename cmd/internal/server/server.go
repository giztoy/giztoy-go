package server

import (
	"fmt"
	"os"

	"github.com/GizClaw/gizclaw-go/cmd/internal/stores"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
)

var BuildCommit = "dev"

// New wires an already prepared in-memory config into a gizclaw.Server.
func New(cfg Config) (*gizclaw.Server, error) {
	cfg, err := prepareConfig(cfg)
	if err != nil {
		return nil, err
	}
	ss, err := stores.New(cfg.Stores)
	if err != nil {
		return nil, fmt.Errorf("server: stores: %w", err)
	}

	gearsKV, err := ss.KV(cfg.Gears.Store)
	if err != nil {
		_ = ss.Close()
		return nil, fmt.Errorf("server: gears store: %w", err)
	}

	fwStore, err := ss.FS(cfg.Depots.Store)
	if err != nil {
		_ = ss.Close()
		return nil, fmt.Errorf("server: firmware store: %w", err)
	}
	if err := os.MkdirAll(string(fwStore), 0o755); err != nil {
		_ = ss.Close()
		return nil, fmt.Errorf("server: firmware dir: %w", err)
	}

	return &gizclaw.Server{
		KeyPair:            cfg.KeyPair,
		GearStore:          gearsKV,
		RegistrationTokens: registrationTokenRoles(cfg.Gears.RegistrationTokens),
		BuildCommit:        BuildCommit,
		ServerPublicKey:    cfg.KeyPair.Public.String(),
		DepotStore:         fwStore,
	}, nil
}

func registrationTokenRoles(tokens map[string]RegistrationTokenConfig) map[string]gearservice.GearRole {
	if len(tokens) == 0 {
		return nil
	}
	out := make(map[string]gearservice.GearRole, len(tokens))
	for name, token := range tokens {
		out[name] = token.Role
	}
	return out
}
