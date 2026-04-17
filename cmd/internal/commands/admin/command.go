package admincmd

import (
	"strings"

	"github.com/GizClaw/gizclaw-go/cmd/internal/client"
	firmwarecmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/admin/firmware"
	gearscmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/admin/gears"
	"github.com/spf13/cobra"
)

var listenAndServeAdminUI = client.ListenAndServeAdminUI

func NewCmd() *cobra.Command {
	var ctxName string
	var listenAddr string
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Admin control-plane commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(listenAddr) == "" {
				return cmd.Help()
			}
			return listenAndServeAdminUI(ctxName, listenAddr, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&ctxName, "context", "", "context name (default: current)")
	cmd.Flags().StringVar(&listenAddr, "listen", "", "listen address or port for the admin web UI")
	cmd.AddCommand(
		gearscmd.NewCmd(),
		firmwarecmd.NewCmd(),
	)
	return cmd
}
