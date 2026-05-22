package main

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/dejanradmanovic/event-spec/spec"
	"github.com/spf13/cobra"
)

func newAdminAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Manage apps (sources) on the registry server",
	}
	cmd.AddCommand(newAdminAppsListCmd())
	cmd.AddCommand(newAdminAppsGetCmd())
	cmd.AddCommand(newAdminAppsCreateCmd())
	cmd.AddCommand(newAdminAppsUpdateCmd())
	cmd.AddCommand(newAdminAppsDeleteCmd())
	return cmd
}

func newAdminAppsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all apps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			apps, err := c.ListApps(context.Background())
			if err != nil {
				return err
			}
			if len(apps) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no apps")
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-20s  %-10s  %-12s  %s\n", "NAME", "PLATFORM", "LANGUAGE", "MODE")
			for _, a := range apps {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-20s  %-10s  %-12s  %s\n", a.Name, a.Platform, a.Language, a.Mode)
			}
			return nil
		},
	}
}

func newAdminAppsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Print an app's YAML",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			app, err := c.GetApp(context.Background(), args[0])
			if err != nil {
				return err
			}
			out, err := yaml.Marshal(app)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}

func newAdminAppsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <file.yaml>",
		Short: "Create an app from a YAML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			var src spec.SourceDef
			if err := yaml.Unmarshal(data, &src); err != nil {
				return fmt.Errorf("parse yaml: %w", err)
			}
			if src.Name == "" || src.Language == "" {
				return fmt.Errorf("app YAML must have name and language fields")
			}
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			result, err := c.CreateApp(context.Background(), src)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "created app %q (language: %s)\n", result.Name, result.Language)
			return nil
		},
	}
}

func newAdminAppsUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <file.yaml>",
		Short: "Update an app from a YAML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			var src spec.SourceDef
			if err := yaml.Unmarshal(data, &src); err != nil {
				return fmt.Errorf("parse yaml: %w", err)
			}
			if src.Name == "" || src.Language == "" {
				return fmt.Errorf("app YAML must have name and language fields")
			}
			c, err := openAdminClient()
			if err != nil {
				return err
			}
			result, err := c.UpdateApp(context.Background(), src)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "updated app %q (language: %s)\n", result.Name, result.Language)
			return nil
		},
	}
}

func newAdminAppsDeleteCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an app by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !yes {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Delete app %q? [y/N] ", name)
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
			if err := c.DeleteApp(context.Background(), name); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "deleted app %q\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return cmd
}
