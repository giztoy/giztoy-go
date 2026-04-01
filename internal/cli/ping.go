package cli

import (
	"fmt"
	"time"

	"github.com/giztoy/giztoy-go/internal/client"
	gctx "github.com/giztoy/giztoy-go/internal/context"
	"github.com/spf13/cobra"
)

func newPingCmd() *cobra.Command {
	var ctxName string

	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Ping the server (peer-layer time sync)",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := gctx.DefaultStore()
			if err != nil {
				return err
			}

			var ctx *gctx.Context
			if ctxName != "" {
				ctx, err = store.LoadByName(ctxName)
			} else {
				ctx, err = store.Current()
			}
			if err != nil {
				return err
			}
			if ctx == nil {
				return fmt.Errorf("no active context; run 'giztoy context create' first")
			}

			serverPK, err := ctx.ServerPublicKey()
			if err != nil {
				return fmt.Errorf("invalid server public key: %w", err)
			}

			c, err := client.Dial(ctx.KeyPair, ctx.Config.Server.Address, serverPK)
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
