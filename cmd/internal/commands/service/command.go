package servicecmd

import (
	"fmt"

	"github.com/GizClaw/gizclaw-go/cmd/internal/service"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the GizClaw server as a system service",
	}

	cmd.AddCommand(
		newInstallCmd(),
		newStatusCmd(),
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newUninstallCmd(),
	)

	return cmd
}

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <workspace>",
		Short: "Install a service definition for the workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.Install(args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Installed service for %s\n", args[0])
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the installed service status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := service.Status()
			if err != nil {
				return err
			}
			workspace := info.WorkspaceRoot
			if workspace == "" {
				workspace = "(unknown)"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "service: %s\n", info.ServiceName)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "installed: %t\n", info.Installed)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "running: %t\n", info.Running)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "state: %s\n", info.State)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "workspace: %s\n", workspace)
			return nil
		},
	}
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the installed service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.Start(); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Started service")
			return nil
		},
	}
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the installed service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.Stop(); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Stopped service")
			return nil
		},
	}
}

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the installed service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.Restart(); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Restarted service")
			return nil
		},
	}
}

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the service definition",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.Uninstall(); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Uninstalled service")
			return nil
		},
	}
}
