package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/haivivi/giztoy/go/internal/server"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	defaults := server.DefaultConfig()
	var dataDir string
	var listenAddr string
	var configPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Giztoy server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := server.Config{ConfigPath: configPath}
			if cmd.Flags().Changed("data-dir") {
				cfg.DataDir = dataDir
			}
			if cmd.Flags().Changed("listen") {
				cfg.ListenAddr = listenAddr
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

	cmd.Flags().StringVar(&dataDir, "data-dir", defaults.DataDir, "server data directory")
	cmd.Flags().StringVar(&listenAddr, "listen", defaults.ListenAddr, "UDP listen address")
	cmd.Flags().StringVar(&configPath, "config", "", "server config file")

	return cmd
}
