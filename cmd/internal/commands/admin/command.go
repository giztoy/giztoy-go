package admincmd

import (
	firmwarecmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/admin/firmware"
	gearscmd "github.com/GizClaw/gizclaw-go/cmd/internal/commands/admin/gears"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Admin control-plane commands",
	}
	cmd.AddCommand(
		gearscmd.NewCmd(),
		firmwarecmd.NewCmd(),
	)
	return cmd
}
