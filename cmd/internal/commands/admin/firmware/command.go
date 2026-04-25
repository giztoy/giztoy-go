package firmwarecmd

import (
	"context"
	"encoding/json"
	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"os"

	"github.com/GizClaw/gizclaw-go/cmd/internal/client"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var ctxName string
	cmd := &cobra.Command{
		Use:   "firmware",
		Short: "Manage firmware depots",
	}
	cmd.PersistentFlags().StringVar(&ctxName, "context", "", "context name (default: current)")
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List firmware depots",
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				items, err := client.ListFirmwares(context.Background(), c)
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(items)
			},
		},
		&cobra.Command{
			Use:   "get <depot>",
			Short: "Get depot snapshot",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.GetFirmwareDepot(context.Background(), c, args[0])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
		&cobra.Command{
			Use:   "get-channel <depot> <channel>",
			Short: "Get active channel release",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.GetFirmwareChannel(context.Background(), c, args[0], adminservice.Channel(args[1]))
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
		newPutInfoCmd(&ctxName),
		newUploadCmd(&ctxName),
		&cobra.Command{
			Use:   "rollback <depot>",
			Short: "Promote rollback to stable",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.RollbackFirmware(context.Background(), c, args[0])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
		&cobra.Command{
			Use:   "release <depot>",
			Short: "Promote testing -> beta -> stable -> rollback",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := client.ConnectFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := client.ReleaseFirmware(context.Background(), c, args[0])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
	)
	return cmd
}

func newPutInfoCmd(ctxName *string) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "put-info <depot>",
		Short: "Write depot info.json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			var info apitypes.DepotInfo
			if err := json.Unmarshal(data, &info); err != nil {
				return err
			}
			c, err := client.ConnectFromContext(*ctxName)
			if err != nil {
				return err
			}
			defer c.Close()
			item, err := client.PutFirmwareInfo(context.Background(), c, args[0], info)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to info.json")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newUploadCmd(ctxName *string) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "upload <depot> <channel>",
		Short: "Upload a release tarball",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			c, err := client.ConnectFromContext(*ctxName)
			if err != nil {
				return err
			}
			defer c.Close()
			item, err := client.UploadFirmware(context.Background(), c, args[0], adminservice.Channel(args[1]), data)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to release tar")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}
