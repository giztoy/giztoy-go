package playcmd

import (
	"strings"

	"github.com/GizClaw/gizclaw-go/cmd/internal/client"
	playconfigcmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/play/config"
	playotacmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/play/ota"
	playregistercmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/play/register"
	playservecmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/play/serve"
	"github.com/spf13/cobra"
)

var listenAndServePlayUI = client.ListenAndServePlayUI

func NewCmd() *cobra.Command {
	var ctxName string
	var listenAddr string
	cmd := &cobra.Command{
		Use:   "play",
		Short: "Device-side commands and reverse API provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(listenAddr) == "" {
				return cmd.Help()
			}
			return listenAndServePlayUI(ctxName, listenAddr, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&ctxName, "context", "", "context name (default: current)")
	cmd.Flags().StringVar(&listenAddr, "listen", "", "listen address or port for the play web UI")
	cmd.AddCommand(
		playservecmd.NewCmd(),
		playregistercmd.NewCmd(),
		playconfigcmd.NewCmd(),
		playotacmd.NewCmd(),
	)
	return cmd
}
