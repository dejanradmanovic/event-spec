package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	serverclient "github.com/dejanradmanovic/event-spec/registry/server/client"
	"github.com/spf13/cobra"
)

func newAdminWebhooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhooks",
		Short: "Manage webhooks on the registry server",
	}
	cmd.AddCommand(newAdminWebhooksAddCmd())
	cmd.AddCommand(newAdminWebhooksListCmd())
	cmd.AddCommand(newAdminWebhooksRemoveCmd())
	return cmd
}

func newAdminWebhooksAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <url>",
		Short: "Register a webhook URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			webhookURL := args[0]
			// RegisterWebhook uses the existing POST /v1/webhooks endpoint via PublishEvent-style call.
			// We call the server directly via the client's post method exposed through PublishWebhook.
			if err := c.RegisterWebhook(context.Background(), webhookURL); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "registered webhook: %s\n", webhookURL)
			return nil
		},
	}
}

func newAdminWebhooksListCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered webhooks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			records, err := c.ListWebhooksAdmin(context.Background())
			if err != nil {
				return err
			}
			return printWebhooks(cmd, records, format)
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "output format: text | json")
	return cmd
}

func newAdminWebhooksRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <webhook-id>",
		Short: "Remove a webhook by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid webhook id %q", args[0])
			}
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			if err := c.RemoveWebhook(context.Background(), id); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "removed webhook %d\n", id)
			return nil
		},
	}
}

func printWebhooks(cmd *cobra.Command, records []serverclient.WebhookRecord, format string) error {
	if format == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(records)
	}
	if len(records) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no webhooks registered")
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-6s  %-40s  %-16s  %s\n", "ID", "URL", "CREATED BY", "CREATED AT")
	for _, r := range records {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-6d  %-40s  %-16s  %s\n",
			r.ID, r.URL, r.CreatedBy, r.CreatedAt.Format("2006-01-02"))
	}
	return nil
}
