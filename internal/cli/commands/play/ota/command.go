package playotacmd

import (
	"context"
	"encoding/json"

	"github.com/giztoy/giztoy-go/internal/client"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var ctxName string
	cmd := &cobra.Command{
		Use:   "ota",
		Short: "Fetch current OTA summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.DialFromContext(ctxName)
			if err != nil {
				return err
			}
			defer c.Close()
			ota, err := c.GetOTA(context.Background())
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(ota)
		},
	}
	cmd.Flags().StringVar(&ctxName, "context", "", "context name (default: current)")
	return cmd
}
