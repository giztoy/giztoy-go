package server

import (
	"fmt"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/cmd/internal/storage"
	"github.com/GizClaw/gizclaw-go/cmd/internal/stores"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
)

var BuildCommit = "dev"

// New wires an already prepared in-memory config into a gizclaw.Server.
func New(cfg Config) (*gizclaw.Server, error) {
	cfg, err := prepareConfig(cfg)
	if err != nil {
		return nil, err
	}
	ss, err := newStoreRegistry(cfg)
	if err != nil {
		return nil, fmt.Errorf("server: stores: %w", err)
	}

	gearsKV, err := ss.KV(cfg.Gears.Store)
	if err != nil {
		_ = ss.Close()
		return nil, fmt.Errorf("server: gears store: %w", err)
	}

	fwStore, err := ss.DepotStore(cfg.Depots.Store)
	if err != nil {
		_ = ss.Close()
		return nil, fmt.Errorf("server: firmware store: %w", err)
	}

	srv := &gizclaw.Server{
		KeyPair:            cfg.KeyPair,
		GearStore:          gearsKV,
		RegistrationTokens: registrationTokenRoles(cfg.Gears.RegistrationTokens),
		BuildCommit:        BuildCommit,
		ServerPublicKey:    cfg.KeyPair.Public.String(),
		DepotStore:         fwStore,
		StoreCloser:        ss,
	}
	if len(cfg.Storage) > 0 {
		if srv.CredentialStore, err = ss.KV(cfg.Credentials.Store); err != nil {
			_ = ss.Close()
			return nil, fmt.Errorf("server: credentials store: %w", err)
		}
		if srv.MiniMaxCredentialStore, err = ss.KV(cfg.MiniMax.CredentialsStore); err != nil {
			_ = ss.Close()
			return nil, fmt.Errorf("server: minimax credentials store: %w", err)
		}
		if srv.MiniMaxTenantStore, err = ss.KV(cfg.MiniMax.TenantsStore); err != nil {
			_ = ss.Close()
			return nil, fmt.Errorf("server: minimax tenants store: %w", err)
		}
		if srv.VoiceStore, err = ss.KV(cfg.MiniMax.VoicesStore); err != nil {
			_ = ss.Close()
			return nil, fmt.Errorf("server: voices store: %w", err)
		}
		if srv.WorkspaceStore, err = ss.KV(cfg.Workspaces.Store); err != nil {
			_ = ss.Close()
			return nil, fmt.Errorf("server: workspaces store: %w", err)
		}
		if srv.WorkspaceTemplateStore, err = ss.KV(cfg.Workspaces.TemplatesStore); err != nil {
			_ = ss.Close()
			return nil, fmt.Errorf("server: workspace template reference store: %w", err)
		}
		if srv.TemplateStore, err = ss.KV(cfg.WorkspaceTemplates.Store); err != nil {
			_ = ss.Close()
			return nil, fmt.Errorf("server: workspace templates store: %w", err)
		}
	}
	return srv, nil
}

func newStoreRegistry(cfg Config) (*stores.Stores, error) {
	if len(cfg.Storage) == 0 {
		return stores.New(cfg.Stores)
	}
	physical, err := storage.New(cfg.Storage)
	if err != nil {
		return nil, err
	}
	ss, err := stores.NewWithOwnedStorage(physical, cfg.Stores)
	if err != nil {
		return nil, err
	}
	return ss, nil
}

func registrationTokenRoles(tokens map[string]RegistrationTokenConfig) map[string]apitypes.GearRole {
	if len(tokens) == 0 {
		return nil
	}
	out := make(map[string]apitypes.GearRole, len(tokens))
	for name, token := range tokens {
		out[name] = token.Role
	}
	return out
}
