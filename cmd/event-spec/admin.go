package main

import (
	"fmt"

	serverclient "github.com/dejanradmanovic/event-spec/registry/server/client"
	"github.com/dejanradmanovic/event-spec/spec"
	"github.com/spf13/cobra"
)

func newAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Administer a running registry server",
	}
	cmd.AddCommand(newAdminStatusCmd())
	cmd.AddCommand(newAdminKeysCmd())
	cmd.AddCommand(newAdminAuditCmd())
	cmd.AddCommand(newAdminWebhooksCmd())
	cmd.AddCommand(newAdminConfigCmd())
	cmd.AddCommand(newAdminDestinationsCmd())
	return cmd
}

// openAdminClient resolves the server URL and API key from event-spec.yaml and
// returns a configured HTTP client. The API key is optional — if absent the
// client sends an empty Bearer token, which the server allows only for
// bootstrap key creation (zero existing keys).
func openAdminClient() (*serverclient.Client, error) {
	cfg, err := spec.LoadWorkspaceConfig("event-spec.yaml")
	if err != nil {
		return nil, fmt.Errorf("read event-spec.yaml: %w", err)
	}
	if cfg.Registry.Mode != spec.RegistryModeServer {
		return nil, fmt.Errorf("admin commands require registry.mode: server in event-spec.yaml")
	}
	if cfg.Registry.URL == "" {
		return nil, fmt.Errorf("registry.url must be set in event-spec.yaml")
	}
	var apiKey string
	if cfg.Registry.APIKey != "" {
		apiKey, err = spec.ResolveSecret(cfg.Registry.APIKey, cfg.Registry.APIKeySecretType)
		if err != nil {
			return nil, fmt.Errorf("resolve api key: %w", err)
		}
	}
	return serverclient.New(serverclient.Config{BaseURL: cfg.Registry.URL, APIKey: apiKey}), nil
}
