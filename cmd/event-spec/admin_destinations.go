package main

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/dejanradmanovic/event-spec/spec"
	"github.com/spf13/cobra"
)

func newAdminDestinationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destinations",
		Short: "Manage destinations on the registry server",
	}
	cmd.AddCommand(newAdminDestinationsListCmd())
	cmd.AddCommand(newAdminDestinationsGetCmd())
	cmd.AddCommand(newAdminDestinationsCreateCmd())
	cmd.AddCommand(newAdminDestinationsUpdateCmd())
	cmd.AddCommand(newAdminDestinationsDeleteCmd())
	return cmd
}

func newAdminDestinationsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all destinations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			dests, err := c.ListDestinations(context.Background())
			if err != nil {
				return err
			}
			if len(dests) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no destinations")
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-20s  %s\n", "NAME", "PROVIDER")
			for _, d := range dests {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-20s  %s\n", d.Name, d.Provider)
			}
			return nil
		},
	}
}

func newAdminDestinationsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Print a destination's YAML",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			dest, err := c.GetDestination(context.Background(), args[0])
			if err != nil {
				return err
			}
			out, err := yaml.Marshal(dest)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}

func newAdminDestinationsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <file.yaml>",
		Short: "Create a destination from a YAML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			var dest spec.DestinationDef
			if err := yaml.Unmarshal(data, &dest); err != nil {
				return fmt.Errorf("parse yaml: %w", err)
			}
			if dest.Name == "" || dest.Provider == "" {
				return fmt.Errorf("destination YAML must have name and provider fields")
			}
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			result, err := c.CreateDestination(context.Background(), dest)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "created destination %q (provider: %s)\n", result.Name, result.Provider)
			return nil
		},
	}
}

func newAdminDestinationsUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <file.yaml>",
		Short: "Update a destination from a YAML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			var dest spec.DestinationDef
			if err := yaml.Unmarshal(data, &dest); err != nil {
				return fmt.Errorf("parse yaml: %w", err)
			}
			if dest.Name == "" || dest.Provider == "" {
				return fmt.Errorf("destination YAML must have name and provider fields")
			}
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			result, err := c.UpdateDestination(context.Background(), dest)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "updated destination %q (provider: %s)\n", result.Name, result.Provider)
			return nil
		},
	}
}

func newAdminDestinationsDeleteCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a destination by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !yes {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Delete destination %q? [y/N] ", name)
				var answer string
				_, _ = fmt.Fscan(os.Stdin, &answer)
				if answer != "y" && answer != "Y" {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "aborted")
					return nil
				}
			}
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			if err := c.DeleteDestination(context.Background(), name); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "deleted destination %q\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return cmd
}
