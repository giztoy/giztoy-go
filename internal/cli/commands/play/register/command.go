package playregistercmd

import (
	"context"
	"encoding/json"

	"github.com/giztoy/giztoy-go/internal/client"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var ctxName string
	var req gears.RegistrationRequest

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register the current device",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.DialFromContext(ctxName)
			if err != nil {
				return err
			}
			defer c.Close()
			result, err := c.Register(context.Background(), req)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
	cmd.Flags().StringVar(&ctxName, "context", "", "context name (default: current)")
	cmd.Flags().StringVar(&req.Device.Name, "name", "", "device name")
	cmd.Flags().StringVar(&req.Device.SN, "sn", "", "serial number")
	cmd.Flags().StringVar(&req.Device.Hardware.Manufacturer, "manufacturer", "", "manufacturer")
	cmd.Flags().StringVar(&req.Device.Hardware.Model, "model", "", "model")
	cmd.Flags().StringVar(&req.Device.Hardware.HardwareRevision, "hardware-revision", "", "hardware revision")
	cmd.Flags().StringVar(&req.Device.Hardware.Depot, "depot", "", "depot")
	cmd.Flags().StringVar(&req.Device.Hardware.FirmwareSemVer, "firmware-semver", "", "firmware semver")
	cmd.Flags().StringVar(&req.RegistrationToken, "token", "", "registration token")
	return cmd
}
