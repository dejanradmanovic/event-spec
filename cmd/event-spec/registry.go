package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dejanradmanovic/event-spec/registry"
	gitregistry "github.com/dejanradmanovic/event-spec/registry/git"
	"github.com/dejanradmanovic/event-spec/registry/local"
	serverclient "github.com/dejanradmanovic/event-spec/registry/server/client"
	"github.com/dejanradmanovic/event-spec/spec"
)

// openRegistry constructs a Registry from the workspace config.
// For git mode the local cache must already exist — run 'event-spec pull' first.
// For server mode an error is returned; the server client is not yet implemented.
func openRegistry(cfg *spec.WorkspaceConfig) (registry.Registry, error) {
	switch cfg.Registry.Mode {
	case spec.RegistryModeGit:
		return gitregistry.New(gitregistry.Config{
			Remote:          cfg.Registry.Remote,
			Branch:          cfg.Registry.Branch,
			Ref:             cfg.Registry.Ref,
			CacheDir:        cfg.Registry.CacheDir,
			SpecsDir:        cfg.Registry.SpecsDir,
			SourcesDir:      cfg.SourcesDir,
			DestinationsDir: cfg.DestinationsDir,
		})
	case spec.RegistryModeServer:
		if cfg.Registry.URL == "" {
			return nil, fmt.Errorf("registry mode %q: registry.url must be set in event-spec.yaml", cfg.Registry.Mode)
		}
		var apiKey string
		if cfg.Registry.APIKey != "" {
			var err error
			apiKey, err = spec.ResolveSecret(cfg.Registry.APIKey, cfg.Registry.APIKeySecretType)
			if err != nil {
				return nil, fmt.Errorf("resolve api key: %w", err)
			}
		}
		return serverclient.New(serverclient.Config{BaseURL: cfg.Registry.URL, APIKey: apiKey}), nil
	default: // local or empty
		specsDir := cfg.SpecsDir
		if specsDir == "" {
			specsDir = "./specs"
		}
		return local.New(local.Config{
			SpecsDir:        specsDir,
			SourcesDir:      cfg.SourcesDir,
			DestinationsDir: cfg.DestinationsDir,
		})
	}
}

// listAllEvents returns every event version in the registry as a pointer slice,
// suitable for applySourceConfig (which handles version pinning) and codegen.Run.
func listAllEvents(ctx context.Context, reg registry.Registry) ([]*spec.EventDef, error) {
	defs, err := reg.ListAllEvents(ctx, registry.ListFilter{})
	if err != nil {
		return nil, err
	}
	ptrs := make([]*spec.EventDef, len(defs))
	for i := range defs {
		ptrs[i] = &defs[i]
	}
	return ptrs, nil
}

// resolveSpecsPath returns the local filesystem path of the specs directory
// for the given workspace config. It is used by validate, which must walk the
// directory directly to collect every parse error (the registry stops at the
// first error, which would hide subsequent problems).
//
// For git mode the cache must already exist; run 'event-spec pull' first.
// For server mode an error is returned (not yet implemented).
func resolveSpecsPath(cfg *spec.WorkspaceConfig) (string, error) {
	switch cfg.Registry.Mode {
	case spec.RegistryModeGit:
		cacheDir := cfg.Registry.CacheDir
		if cacheDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve home dir: %w", err)
			}
			h := sha256.Sum256([]byte(cfg.Registry.Remote))
			cacheDir = filepath.Join(home, ".event-spec", "cache", fmt.Sprintf("%x", h[:6]))
		}
		if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
			return "", fmt.Errorf("git cache not found at %s: run 'event-spec pull' first", cacheDir)
		}
		specsDir := cfg.Registry.SpecsDir
		if specsDir == "" {
			specsDir = "specs"
		}
		return filepath.Join(cacheDir, specsDir), nil
	case spec.RegistryModeServer:
		return "", fmt.Errorf("registry mode %q: specs are fetched from the server; use 'event-spec generate' or 'event-spec diff' instead", cfg.Registry.Mode)
	default: // local or empty
		specsDir := cfg.SpecsDir
		if specsDir == "" {
			specsDir = "./specs"
		}
		return specsDir, nil
	}
}
