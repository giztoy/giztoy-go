package servecmd

import (
	"github.com/GizClaw/gizclaw-go/cmd/internal/server"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var force bool
	var serviceManaged bool
	cmd := &cobra.Command{
		Use:   "serve <dir>",
		Short: "Start the GizClaw server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return server.ServeWithOptions(args[0], server.ServeOptions{
				Force:          force,
				ServiceManaged: serviceManaged,
			})
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "stop the previous server for this workspace before starting")
	cmd.Flags().BoolVar(&serviceManaged, "service-managed", false, "internal flag for service-managed foreground runs")
	_ = cmd.Flags().MarkHidden("service-managed")
	return cmd
}
