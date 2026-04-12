package cli

import (
	admincmd "github.com/giztoy/giztoy-go/internal/cli/commands/admin"
	contextcmd "github.com/giztoy/giztoy-go/internal/cli/commands/context"
	pingcmd "github.com/giztoy/giztoy-go/internal/cli/commands/ping"
	playcmd "github.com/giztoy/giztoy-go/internal/cli/commands/play"
	servecmd "github.com/giztoy/giztoy-go/internal/cli/commands/serve"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "giztoy",
		Short: "Giztoy — peer-to-peer toy network",
	}

	root.AddCommand(
		servecmd.NewCmd(),
		contextcmd.NewCmd(),
		pingcmd.NewCmd(),
		admincmd.NewCmd(),
		playcmd.NewCmd(),
	)

	return root
}
