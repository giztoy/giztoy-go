package cli

import "github.com/spf13/cobra"

func newAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Admin control-plane commands",
	}
	cmd.AddCommand(
		newAdminGearsCmd(),
		newAdminFirmwareCmd(),
	)
	return cmd
}
