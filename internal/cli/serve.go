package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/giztoy/giztoy-go/internal/server"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve <dir>",
		Short: "Start the Giztoy server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := prepareServeWorkspace(args[0])
			if err != nil {
				return err
			}
			srv, err := server.New(cfg)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return srv.Run(ctx)
		},
	}

	return cmd
}

func prepareServeWorkspace(workspace string) (server.Config, error) {
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
	return workspaceServerConfig(), nil
}

func workspaceServerConfig() server.Config {
	return server.Config{
		DataDir:    ".",
		ConfigPath: "config.yaml",
	}
}
