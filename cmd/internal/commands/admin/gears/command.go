package gearscmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/GizClaw/gizclaw-go/cmd/internal/client"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/spf13/cobra"
)

type gearConfigClient interface {
	GetGearConfig(ctx context.Context, publicKey string) (gearservice.Configuration, error)
	PutGearConfig(ctx context.Context, publicKey string, cfg gearservice.Configuration) (gearservice.Configuration, error)
	Close() error
}

type gearConfigBridge struct {
	c *gizclaw.Client
}

func (g *gearConfigBridge) GetGearConfig(ctx context.Context, publicKey string) (gearservice.Configuration, error) {
	return client.GetGearConfig(ctx, g.c, publicKey)
}

func (g *gearConfigBridge) PutGearConfig(ctx context.Context, publicKey string, cfg gearservice.Configuration) (gearservice.Configuration, error) {
	return client.PutGearConfig(ctx, g.c, publicKey, cfg)
}

func (g *gearConfigBridge) Close() error {
	return g.c.Close()
}

var openGearConfigClient = func(ctxName string) (gearConfigClient, error) {
	c, err := client.ConnectFromContext(ctxName)
	if err != nil {
		return nil, err
	}
	return &gearConfigBridge{c: c}, nil
}

func NewCmd() *cobra.Command {
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				items, err := client.ListGears(context.Background(), c)
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.GetGear(context.Background(), c, args[0])
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				publicKey, err := client.ResolveGearBySN(context.Background(), c, args[0])
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				publicKey, err := client.ResolveGearByIMEI(context.Background(), c, args[0], args[1])
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.ApproveGear(context.Background(), c, args[0], gearservice.GearRole(args[1]))
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.BlockGear(context.Background(), c, args[0])
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.GetGearInfo(context.Background(), c, args[0])
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.GetGearConfig(context.Background(), c, args[0])
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
				if cfg.Firmware == nil {
					cfg.Firmware = &gearservice.FirmwareConfig{}
				}
				channel := gearservice.GearFirmwareChannel(args[1])
				cfg.Firmware.Channel = &channel

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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.GetGearRuntime(context.Background(), c, args[0])
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.GetGearOTA(context.Background(), c, args[0])
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				items, err := client.ListGearsByLabel(context.Background(), c, args[0], args[1])
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				items, err := client.ListGearsByCertification(context.Background(), c, gearservice.GearCertificationType(args[0]), gearservice.GearCertificationAuthority(args[1]), args[2])
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				items, err := client.ListGearsByFirmware(context.Background(), c, args[0], gearservice.GearFirmwareChannel(args[1]))
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.DeleteGear(context.Background(), c, args[0])
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
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.RefreshGear(context.Background(), c, args[0])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
	)
	return cmd
}
