package main

import (
	"context"
	"encoding/json"
	"fmt"

	serverclient "github.com/dejanradmanovic/event-spec/registry/server/client"
	"github.com/spf13/cobra"
)

func newAdminConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View or update runtime server configuration",
	}
	cmd.AddCommand(newAdminConfigGetCmd())
	cmd.AddCommand(newAdminConfigSetCmd())
	return cmd
}

func newAdminConfigGetCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "List all runtime configuration settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			settings, err := c.GetConfig(context.Background())
			if err != nil {
				return err
			}
			return printSettings(cmd, settings, format)
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "output format: text | json")
	return cmd
}

func newAdminConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Update a runtime configuration setting",
		Long: `Update a runtime configuration setting on the server.

Supported keys:
  hooks_enabled   Enable or disable analytics relay hooks (true | false)`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			setting, err := c.SetConfig(context.Background(), key, value)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "set %s = %s\n", setting.Key, setting.Value)
			return nil
		},
	}
}

func printSettings(cmd *cobra.Command, settings []serverclient.ServerSetting, format string) error {
	if format == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(settings)
	}
	if len(settings) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no settings configured")
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-30s  %s\n", "KEY", "VALUE")
	for _, s := range settings {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-30s  %s\n", s.Key, s.Value)
	}
	return nil
}
