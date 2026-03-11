package cli

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "giztoy",
		Short: "Giztoy — peer-to-peer toy network",
	}

	root.AddCommand(
		newServeCmd(),
		newContextCmd(),
		newPingCmd(),
		newAdminCmd(),
		newPlayCmd(),
	)

	return root
}
