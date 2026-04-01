package cli

import (
	"context"
	"encoding/json"
	"os"

	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/spf13/cobra"
)

func newAdminFirmwareCmd() *cobra.Command {
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
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				items, err := c.ListFirmwares(context.Background())
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
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.GetFirmwareDepot(context.Background(), args[0])
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
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.GetFirmwareChannel(context.Background(), args[0], firmware.Channel(args[1]))
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
		newFirmwarePutInfoCmd(&ctxName),
		newFirmwareUploadCmd(&ctxName),
		&cobra.Command{
			Use:   "rollback <depot>",
			Short: "Promote rollback to stable",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.RollbackFirmware(context.Background(), args[0])
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
				c, err := dialFromContext(ctxName)
				if err != nil {
					return err
				}
				defer c.Close()
				item, err := c.ReleaseFirmware(context.Background(), args[0])
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(item)
			},
		},
	)
	return cmd
}

func newFirmwarePutInfoCmd(ctxName *string) *cobra.Command {
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
			var info firmware.DepotInfo
			if err := json.Unmarshal(data, &info); err != nil {
				return err
			}
			c, err := dialFromContext(*ctxName)
			if err != nil {
				return err
			}
			defer c.Close()
			item, err := c.PutFirmwareInfo(context.Background(), args[0], info)
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

func newFirmwareUploadCmd(ctxName *string) *cobra.Command {
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
			c, err := dialFromContext(*ctxName)
			if err != nil {
				return err
			}
			defer c.Close()
			item, err := c.UploadFirmware(context.Background(), args[0], firmware.Channel(args[1]), data)
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
