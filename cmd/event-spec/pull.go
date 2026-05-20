package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dejanradmanovic/event-spec/registry"
	gitregistry "github.com/dejanradmanovic/event-spec/registry/git"
	"github.com/dejanradmanovic/event-spec/spec"
	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	var (
		force bool
		ref   string
	)

	cmd := &cobra.Command{
		Use:   "pull [source-name]",
		Short: "Clone or fetch specs from a git registry into the local cache",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := spec.LoadWorkspaceConfig("event-spec.yaml")
			if err != nil {
				return fmt.Errorf("read event-spec.yaml: %w", err)
			}
			if cfg.Registry.Mode != spec.RegistryModeGit {
				return fmt.Errorf("event-spec pull requires registry.mode: git; got %q", cfg.Registry.Mode)
			}
			if len(args) > 0 && args[0] != cfg.Workspace {
				return fmt.Errorf("source name %q does not match workspace %q", args[0], cfg.Workspace)
			}

			gitCfg := gitregistry.Config{
				Remote:          cfg.Registry.Remote,
				Branch:          cfg.Registry.Branch,
				Ref:             cfg.Registry.Ref,
				CacheDir:        cfg.Registry.CacheDir,
				SpecsDir:        cfg.Registry.SpecsDir,
				SourcesDir:      cfg.SourcesDir,
				DestinationsDir: cfg.DestinationsDir,
				Force:           force,
			}
			if ref != "" {
				gitCfg.Ref = ref
			}

			cacheDir := gitCfg.CacheDir
			if cacheDir == "" {
				home, _ := os.UserHomeDir()
				h := sha256.Sum256([]byte(gitCfg.Remote))
				cacheDir = filepath.Join(home, ".event-spec", "cache", fmt.Sprintf("%x", h[:6]))
			}
			branch := gitCfg.Branch
			if branch == "" {
				branch = "main"
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pulling %s@%s into %s…\n", gitCfg.Remote, branch, cacheDir)

			r, err := gitregistry.NewWithPull(context.Background(), gitCfg)
			if err != nil {
				return err
			}

			events, err := r.ListEvents(context.Background(), registry.ListFilter{})
			if err != nil {
				return fmt.Errorf("index events: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Indexed %d event(s).\n", len(events))
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "delete the cache and re-clone")
	cmd.Flags().StringVar(&ref, "ref", "", "override the branch/tag/SHA from event-spec.yaml")

	return cmd
}
