package commands

import (
	admincmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/admin"
	contextcmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/context"
	pingcmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/ping"
	playcmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/play"
	servecmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/serve"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	root := &cobra.Command{
		Use:   "gizclaw",
		Short: "GizClaw - peer-to-peer toy network",
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
