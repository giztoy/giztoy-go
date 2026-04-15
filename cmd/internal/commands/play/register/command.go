package playregistercmd

import (
	"context"
	"encoding/json"

	"github.com/GizClaw/gizclaw-go/cmd/internal/client"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var ctxName string
	var req serverpublic.RegistrationRequest

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register the current device",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.ConnectFromContext(ctxName)
			if err != nil {
				return err
			}
			defer c.Close()
			result, err := client.Register(context.Background(), c, req)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
	cmd.Flags().StringVar(&ctxName, "context", "", "context name (default: current)")
	var name string
	var sn string
	var manufacturer string
	var model string
	var hardwareRevision string
	var depot string
	var firmwareSemver string
	var token string
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		req.Device = serverpublic.DeviceInfo{
			Name: optionalString(name),
			Sn:   optionalString(sn),
			Hardware: &serverpublic.HardwareInfo{
				Manufacturer:     optionalString(manufacturer),
				Model:            optionalString(model),
				HardwareRevision: optionalString(hardwareRevision),
				Depot:            optionalString(depot),
				FirmwareSemver:   optionalString(firmwareSemver),
			},
		}
		req.RegistrationToken = optionalString(token)
	}
	cmd.Flags().StringVar(&name, "name", "", "device name")
	cmd.Flags().StringVar(&sn, "sn", "", "serial number")
	cmd.Flags().StringVar(&manufacturer, "manufacturer", "", "manufacturer")
	cmd.Flags().StringVar(&model, "model", "", "model")
	cmd.Flags().StringVar(&hardwareRevision, "hardware-revision", "", "hardware revision")
	cmd.Flags().StringVar(&depot, "depot", "", "depot")
	cmd.Flags().StringVar(&firmwareSemver, "firmware-semver", "", "firmware semver")
	cmd.Flags().StringVar(&token, "token", "", "registration token")
	return cmd
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
