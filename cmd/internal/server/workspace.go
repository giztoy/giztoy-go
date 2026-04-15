package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/GizClaw/gizclaw-go/cmd/internal/identity"
	"github.com/GizClaw/gizclaw-go/cmd/internal/stores"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
)

const workspaceConfigFile = "config.yaml"
const workspaceIdentityFile = "identity.key"

func prepareWorkspaceConfig(workspace string) (Config, error) {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return Config{}, fmt.Errorf("server: resolve workspace %q: %w", workspace, err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Config{}, fmt.Errorf("server: create workspace %q: %w", root, err)
	}
	fileCfg, err := LoadConfig(filepath.Join(root, workspaceConfigFile))
	if err != nil {
		return Config{}, fmt.Errorf("server: load config: %w", err)
	}
	keyPair, err := identity.LoadOrGenerate(filepath.Join(root, workspaceIdentityFile))
	if err != nil {
		return Config{}, fmt.Errorf("server: identity: %w", err)
	}

	cfg := mergeFileConfig(Config{
		KeyPair: keyPair,
	}, fileCfg)
	cfg.Stores = resolveWorkspaceStoreConfigs(root, cfg.Stores)
	return prepareConfig(cfg)
}

func resolveWorkspaceStoreConfigs(root string, cfgs map[string]stores.Config) map[string]stores.Config {
	if len(cfgs) == 0 {
		return nil
	}

	resolved := make(map[string]stores.Config, len(cfgs))
	for name, cfg := range cfgs {
		if cfg.Dir != "" && !filepath.IsAbs(cfg.Dir) {
			cfg.Dir = filepath.Join(root, cfg.Dir)
		}
		resolved[name] = cfg
	}
	return resolved
}

func Serve(workspace string) error {
	cfg, err := prepareWorkspaceConfig(workspace)
	if err != nil {
		return err
	}
	srv, err := New(cfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(nil, giznet.WithBindAddr(cfg.ListenAddr))
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		if err := srv.Close(); err != nil {
			return err
		}
		return <-errCh
	}
}
