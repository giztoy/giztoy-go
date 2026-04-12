package playconfigcmd

import (
	"context"
	"encoding/json"

	"github.com/giztoy/giztoy-go/internal/client"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var ctxName string
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Fetch current device configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.DialFromContext(ctxName)
			if err != nil {
				return err
			}
			defer c.Close()
			cfg, err := c.GetConfig(context.Background())
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(cfg)
		},
	}
	cmd.Flags().StringVar(&ctxName, "context", "", "context name (default: current)")
	return cmd
}
