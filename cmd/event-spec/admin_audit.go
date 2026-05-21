package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	serverclient "github.com/dejanradmanovic/event-spec/registry/server/client"
	"github.com/spf13/cobra"
)

func newAdminAuditCmd() *cobra.Command {
	var (
		since  string
		until  string
		entity string
		user   string
		format string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Query the server audit log",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openAdminClient()
			if err != nil {
				return err
			}

			q := url.Values{}
			if since != "" {
				q.Set("since", since)
			}
			if until != "" {
				q.Set("until", until)
			}
			if entity != "" {
				q.Set("entity", entity)
			}
			if user != "" {
				q.Set("user", user)
			}
			if limit > 0 {
				q.Set("limit", fmt.Sprintf("%d", limit))
			}

			entries, err := c.ListAuditLog(context.Background(), q)
			if err != nil {
				return err
			}
			return printAuditEntries(cmd, entries, format)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "include entries at or after this RFC3339 timestamp")
	cmd.Flags().StringVar(&until, "until", "", "include entries at or before this RFC3339 timestamp")
	cmd.Flags().StringVar(&entity, "entity", "", "filter by entity type: event | source | destination")
	cmd.Flags().StringVar(&user, "user", "", "filter by user ID")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text | json")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum number of results")
	return cmd
}

func printAuditEntries(cmd *cobra.Command, entries []serverclient.AuditEntry, format string) error {
	if format == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	if len(entries) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no audit entries")
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-6s  %-8s  %-14s  %-10s  %-16s  %s\n",
		"ID", "ACTION", "ENTITY TYPE", "ENTITY ID", "USER", "TIMESTAMP")
	for _, e := range entries {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-6d  %-8s  %-14s  %-10d  %-16s  %s\n",
			e.ID, e.Action, e.EntityType, e.EntityID, e.UserID, e.Timestamp.Format("2006-01-02T15:04:05Z"))
	}
	return nil
}
