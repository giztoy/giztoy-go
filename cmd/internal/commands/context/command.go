package contextcmd

import (
	"fmt"

	"github.com/GizClaw/gizclaw-go/cmd/internal/clicontext"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "context",
		Aliases: []string{"ctx"},
		Short:   "Manage server connection contexts",
	}

	cmd.AddCommand(
		newCreateCmd(),
		newUseCmd(),
		newListCmd(),
	)

	return cmd
}

func newCreateCmd() *cobra.Command {
	var serverAddr, pubkey string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := clicontext.DefaultStore()
			if err != nil {
				return err
			}
			name := args[0]
			if err := store.Create(name, serverAddr, pubkey); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Context %q created.\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&serverAddr, "server", "", "server address (host:port)")
	cmd.Flags().StringVar(&pubkey, "pubkey", "", "server public key (hex)")
	_ = cmd.MarkFlagRequired("server")
	_ = cmd.MarkFlagRequired("pubkey")

	return cmd
}

func newUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Switch to a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := clicontext.DefaultStore()
			if err != nil {
				return err
			}
			if err := store.Use(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Switched to context %q.\n", args[0])
			return nil
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := clicontext.DefaultStore()
			if err != nil {
				return err
			}
			names, current, err := store.List()
			if err != nil {
				return err
			}
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No contexts found.")
				return nil
			}
			for _, name := range names {
				marker := "  "
				if name == current {
					marker = "* "
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", marker, name)
			}
			return nil
		},
	}
}
