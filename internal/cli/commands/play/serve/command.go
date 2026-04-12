package playservecmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/giztoy/giztoy-go/internal/client"
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

func NewCmd() *cobra.Command {
	var ctxName string
	var provider staticDeviceProvider

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve reverse device API on the current connection",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.DialFromContext(ctxName)
			if err != nil {
				return err
			}
			defer c.Close()

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return c.ServePeerPublic(ctx, provider)
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
