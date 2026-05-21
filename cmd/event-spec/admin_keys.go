package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	serverclient "github.com/dejanradmanovic/event-spec/registry/server/client"
	"github.com/spf13/cobra"
)

func newAdminKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage API keys on the registry server",
	}
	cmd.AddCommand(newAdminKeysCreateCmd())
	cmd.AddCommand(newAdminKeysListCmd())
	cmd.AddCommand(newAdminKeysRevokeCmd())
	return cmd
}

func newAdminKeysCreateCmd() *cobra.Command {
	var (
		role      string
		name      string
		expiresIn string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new API key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if role == "" {
				return fmt.Errorf("--role is required: viewer, publisher, or admin")
			}
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			result, err := c.CreateAPIKey(context.Background(), role, name, expiresIn)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ID:   %d\n", result.ID)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Role: %s\n", result.Role)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Key:  %s\n", result.Key)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nStore this key securely — it will not be shown again.")
			return nil
		},
	}

	cmd.Flags().StringVar(&role, "role", "", "role for the key: viewer, publisher, or admin (required)")
	cmd.Flags().StringVar(&name, "name", "", "optional label for this key")
	cmd.Flags().StringVar(&expiresIn, "expires", "", "expiry duration: 90d, 1y, 720h, etc.")
	return cmd
}

func newAdminKeysListCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			records, err := c.ListAPIKeys(context.Background())
			if err != nil {
				return err
			}
			return printAPIKeys(cmd, records, format)
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "output format: text | json")
	return cmd
}

func newAdminKeysRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <key-id>",
		Short: "Revoke an API key by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid key id %q", args[0])
			}
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			if err := c.RevokeAPIKey(context.Background(), id); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "revoked key %d\n", id)
			return nil
		},
	}
}

func printAPIKeys(cmd *cobra.Command, records []serverclient.APIKeyRecord, format string) error {
	if format == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(records)
	}
	if len(records) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no API keys")
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-6s  %-12s  %-20s  %-16s  %s\n", "ID", "ROLE", "NAME", "CREATED BY", "EXPIRES")
	for _, r := range records {
		expires := "never"
		if r.ExpiresAt != nil {
			expires = r.ExpiresAt.Format("2006-01-02")
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-6d  %-12s  %-20s  %-16s  %s\n",
			r.ID, r.Role, r.Name, r.CreatedBy, expires)
	}
	return nil
}
