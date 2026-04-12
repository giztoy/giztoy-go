package playcmd

import (
	playconfigcmd "github.com/giztoy/giztoy-go/internal/cli/commands/play/config"
	playotacmd "github.com/giztoy/giztoy-go/internal/cli/commands/play/ota"
	playregistercmd "github.com/giztoy/giztoy-go/internal/cli/commands/play/register"
	playservecmd "github.com/giztoy/giztoy-go/internal/cli/commands/play/serve"
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
