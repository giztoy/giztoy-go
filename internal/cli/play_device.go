package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/spf13/cobra"
)

type staticDeviceProvider struct {
	info        gears.RefreshInfo
	identifiers gears.RefreshIdentifiers
	version     gears.RefreshVersion
}

func (p staticDeviceProvider) Info(context.Context) (gears.RefreshInfo, error) {
	return p.info, nil
}

func (p staticDeviceProvider) Identifiers(context.Context) (gears.RefreshIdentifiers, error) {
	return p.identifiers, nil
}

func (p staticDeviceProvider) Version(context.Context) (gears.RefreshVersion, error) {
	return p.version, nil
}

func newPlayServeCmd() *cobra.Command {
	var ctxName string
	var provider staticDeviceProvider

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve reverse device API on the current connection",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := dialFromContext(ctxName)
			if err != nil {
				return err
			}
			defer c.Close()

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return c.ServeReverseHTTP(ctx, provider)
		},
	}
	cmd.Flags().StringVar(&ctxName, "context", "", "context name (default: current)")
	cmd.Flags().StringVar(&provider.info.Name, "name", "", "device name")
	cmd.Flags().StringVar(&provider.info.Manufacturer, "manufacturer", "", "manufacturer")
	cmd.Flags().StringVar(&provider.info.Model, "model", "", "model")
	cmd.Flags().StringVar(&provider.info.HardwareRevision, "hardware-revision", "", "hardware revision")
	cmd.Flags().StringVar(&provider.identifiers.SN, "sn", "", "serial number")
	cmd.Flags().StringVar(&provider.version.Depot, "depot", "", "firmware depot")
	cmd.Flags().StringVar(&provider.version.FirmwareSemVer, "firmware-semver", "", "firmware semver")
	return cmd
}

func newPlayRegisterCmd() *cobra.Command {
	var ctxName string
	var req gears.RegistrationRequest
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register the current device",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := dialFromContext(ctxName)
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

func newPlayConfigCmd() *cobra.Command {
	var ctxName string
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Fetch current device configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := dialFromContext(ctxName)
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

func newPlayOTACmd() *cobra.Command {
	var ctxName string
	cmd := &cobra.Command{
		Use:   "ota",
		Short: "Fetch current OTA summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := dialFromContext(ctxName)
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
