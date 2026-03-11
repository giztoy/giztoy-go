package cli

import "github.com/spf13/cobra"

func newPlayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "play",
		Short: "Device-side commands and reverse API provider",
	}
	cmd.AddCommand(
		newPlayServeCmd(),
		newPlayRegisterCmd(),
		newPlayConfigCmd(),
		newPlayOTACmd(),
	)
	return cmd
}
