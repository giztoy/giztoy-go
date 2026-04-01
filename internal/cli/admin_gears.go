package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/spf13/cobra"
)

type gearConfigClient interface {
	GetGearConfig(ctx context.Context, publicKey string) (gears.Configuration, error)
	PutGearConfig(ctx context.Context, publicKey string, cfg gears.Configuration) (gears.Configuration, error)
	Close() error
}

var openGearConfigClient = func(ctxName string) (gearConfigClient, error) {
	return dialFromContext(ctxName)
}

func newAdminGearsCmd() *cobra.Command {
	var ctxName string
	cmd := &cobra.Command{
		Use:   "gears",
		Short: "Manage gears",
	}
	cmd.PersistentFlags().StringVar(&ctxName, "context", "", "context name (default: current)")
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List gears",
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				items, err := c.ListGears(context.Background())
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(items)
			},
		},
		&cobra.Command{
			Use:   "get <pubkey>",
			Short: "Get gear registration",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.GetGear(context.Background(), args[0])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
		&cobra.Command{
			Use:   "resolve-sn <sn>",
			Short: "Resolve public key by SN",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				publicKey, err := c.ResolveGearBySN(context.Background(), args[0])
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), publicKey)
				return nil
			},
		},
		&cobra.Command{
			Use:   "resolve-imei <tac> <serial>",
			Short: "Resolve public key by IMEI",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				publicKey, err := c.ResolveGearByIMEI(context.Background(), args[0], args[1])
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), publicKey)
				return nil
			},
		},
		&cobra.Command{
			Use:   "approve <pubkey> <role>",
			Short: "Approve gear role",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.ApproveGear(context.Background(), args[0], gears.GearRole(args[1]))
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), item.PublicKey, item.Role, item.Status)
				return nil
			},
		},
		&cobra.Command{
			Use:   "block <pubkey>",
			Short: "Block gear",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.BlockGear(context.Background(), args[0])
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), item.PublicKey, item.Status)
				return nil
			},
		},
		&cobra.Command{
			Use:   "info <pubkey>",
			Short: "Get gear info snapshot",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.GetGearInfo(context.Background(), args[0])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
		&cobra.Command{
			Use:   "config <pubkey>",
			Short: "Get gear config snapshot",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.GetGearConfig(context.Background(), args[0])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
		&cobra.Command{
			Use:   "put-config <pubkey> <channel>",
			Short: "Replace gear firmware config",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := openGearConfigClient(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()

				cfg, err := c.GetGearConfig(context.Background(), args[0])
				if err != nil {
					return err
				}
				cfg.Firmware.Channel = gears.GearFirmwareChannel(args[1])

				item, err := c.PutGearConfig(context.Background(), args[0], cfg)
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
		&cobra.Command{
			Use:   "runtime <pubkey>",
			Short: "Get gear runtime snapshot",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.GetGearRuntime(context.Background(), args[0])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
		&cobra.Command{
			Use:   "ota <pubkey>",
			Short: "Get gear OTA summary",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.GetGearOTA(context.Background(), args[0])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
		&cobra.Command{
			Use:   "list-by-label <key> <value>",
			Short: "List gears by label",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				items, err := c.ListGearsByLabel(context.Background(), args[0], args[1])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(items)
			},
		},
		&cobra.Command{
			Use:   "list-by-certification <type> <authority> <id>",
			Short: "List gears by certification",
			Args:  cobra.ExactArgs(3),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				items, err := c.ListGearsByCertification(context.Background(), gears.GearCertificationType(args[0]), gears.GearCertificationAuthority(args[1]), args[2])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(items)
			},
		},
		&cobra.Command{
			Use:   "list-by-firmware <depot> <channel>",
			Short: "List gears by firmware policy",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				items, err := c.ListGearsByFirmware(context.Background(), args[0], gears.GearFirmwareChannel(args[1]))
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(items)
			},
		},
		&cobra.Command{
			Use:   "delete <pubkey>",
			Short: "Reset gear registration",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.DeleteGear(context.Background(), args[0])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
		&cobra.Command{
			Use:   "refresh <pubkey>",
			Short: "Refresh gear from device-side API",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.RefreshGear(context.Background(), args[0])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
	)
	return cmd
}
