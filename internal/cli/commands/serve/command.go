package servecmd

import (
	"context"
	"fmt"
	"github.com/giztoy/giztoy-go/internal/server"
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve <dir>",
		Short: "Start the Giztoy server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := PrepareWorkspace(args[0])
			if err != nil {
				return err
			}
			srv, err := server.New(cfg)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			errCh := make(chan error, 1)
			go func() {
				errCh <- srv.ListenAndServe(giznet.WithBindAddr(cfg.ListenAddr))
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
		},
	}
}

func PrepareWorkspace(workspace string) (server.Config, error) {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return server.Config{}, fmt.Errorf("serve: resolve workspace %q: %w", workspace, err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return server.Config{}, fmt.Errorf("serve: create workspace %q: %w", root, err)
	}
	if err := os.Chdir(root); err != nil {
		return server.Config{}, fmt.Errorf("serve: chdir workspace %q: %w", root, err)
	}
	return server.Config{
		DataDir:    ".",
		ConfigPath: "config.yaml",
	}, nil
}
