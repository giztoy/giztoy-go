package admincmd

import (
	firmwarecmd "github.com/giztoy/giztoy-go/internal/cli/commands/admin/firmware"
	gearscmd "github.com/giztoy/giztoy-go/internal/cli/commands/admin/gears"
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
