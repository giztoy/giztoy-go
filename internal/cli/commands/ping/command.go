package pingcmd

import (
	"fmt"
	"time"

	"github.com/giztoy/giztoy-go/internal/client"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var ctxName string

	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Ping the server (peer-layer time sync)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.DialFromContext(ctxName)
			if err != nil {
				return err
			}
			defer c.Close()

			result, err := c.Ping()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Server Time: %s\n", result.ServerTime.Format(time.RFC3339Nano))
			fmt.Fprintf(out, "RTT:         %v\n", result.RTT.Round(time.Microsecond))
			fmt.Fprintf(out, "Clock Diff:  %v\n", result.ClockDiff.Round(time.Microsecond))
			return nil
		},
	}

	cmd.Flags().StringVar(&ctxName, "context", "", "context name (default: current)")
	return cmd
}
