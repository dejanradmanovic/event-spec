package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/dejanradmanovic/event-spec/registry"
	serverclient "github.com/dejanradmanovic/event-spec/registry/server/client"
	"github.com/dejanradmanovic/event-spec/spec"
	"github.com/spf13/cobra"
)

func newPublishCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "publish <spec-file> [<spec-file>...]",
		Short: "Publish event spec files to the registry server",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := spec.LoadWorkspaceConfig("event-spec.yaml")
			if err != nil {
				return fmt.Errorf("read event-spec.yaml: %w", err)
			}
			if cfg.Registry.Mode != spec.RegistryModeServer {
				return fmt.Errorf("event-spec publish requires registry.mode: server in event-spec.yaml")
			}
			if cfg.Registry.URL == "" {
				return fmt.Errorf("registry.url must be set in event-spec.yaml")
			}
			if cfg.Registry.APIKey == "" {
				return fmt.Errorf("registry.api_key is required for publish; set it in event-spec.yaml")
			}
			apiKey, err := spec.ResolveSecret(cfg.Registry.APIKey, cfg.Registry.APIKeySecretType)
			if err != nil {
				return fmt.Errorf("resolve api key: %w", err)
			}

			c := serverclient.New(serverclient.Config{BaseURL: cfg.Registry.URL, APIKey: apiKey})
			ctx := context.Background()

			for _, file := range args {
				ev, err := spec.LoadEventDef(file)
				if err != nil {
					return fmt.Errorf("load %s: %w", file, err)
				}

				if dryRun {
					existing, getErr := c.GetEvent(ctx, ev.Namespace, ev.Name, ev.Version)
					switch {
					case getErr == nil:
						changes := spec.Diff(existing, ev)
						if len(changes) == 0 {
							_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s/%s@%s: no changes\n", ev.Namespace, ev.Name, ev.Version)
						} else {
							_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s/%s@%s: %d change(s)\n", ev.Namespace, ev.Name, ev.Version, len(changes))
							for _, ch := range changes {
								breaking := ""
								if ch.Breaking {
									breaking = " [BREAKING]"
								}
								_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-24s %s%s\n", string(ch.Kind), ch.Property, breaking)
							}
						}
					case errors.Is(getErr, registry.ErrNotFound):
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s/%s@%s: new (not yet published)\n", ev.Namespace, ev.Name, ev.Version)
					default:
						return fmt.Errorf("check %s/%s@%s: %w", ev.Namespace, ev.Name, ev.Version, getErr)
					}
					continue
				}

				if err := c.PublishEvent(ctx, *ev); err != nil {
					return fmt.Errorf("publish %s/%s@%s: %w", ev.Namespace, ev.Name, ev.Version, err)
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "published %s/%s@%s\n", ev.Namespace, ev.Name, ev.Version)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate and diff against server version without writing")
	return cmd
}
