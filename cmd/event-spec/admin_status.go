package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newAdminStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check server health",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			s, err := c.Status(context.Background())
			if err != nil {
				return fmt.Errorf("server unreachable: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "status: %s\n", s.Status)
			if s.Uptime != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "uptime: %s\n", s.Uptime)
			}
			return nil
		},
	}
}
