package playcmd

import (
	playconfigcmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/play/config"
	playotacmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/play/ota"
	playregistercmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/play/register"
	playservecmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/play/serve"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "play",
		Short: "Device-side commands and reverse API provider",
	}
	cmd.AddCommand(
		playservecmd.NewCmd(),
		playregistercmd.NewCmd(),
		playconfigcmd.NewCmd(),
		playotacmd.NewCmd(),
	)
	return cmd
}
