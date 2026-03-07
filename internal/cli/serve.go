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
	cfg := server.DefaultConfig()

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Giztoy server",
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := server.New(cfg)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return srv.Run(ctx)
		},
	}

	cmd.Flags().StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "server data directory")
	cmd.Flags().StringVar(&cfg.ListenAddr, "listen", cfg.ListenAddr, "UDP listen address")

	return cmd
}
