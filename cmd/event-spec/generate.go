package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dejanradmanovic/event-spec/codegen"
	_ "github.com/dejanradmanovic/event-spec/codegen/golang"
	_ "github.com/dejanradmanovic/event-spec/codegen/kotlin"
	_ "github.com/dejanradmanovic/event-spec/codegen/typescript"
	"github.com/dejanradmanovic/event-spec/registry/local"
	"github.com/dejanradmanovic/event-spec/spec"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	var (
		lang string
		out  string
	)

	cmd := &cobra.Command{
		Use:   "generate [source]",
		Short: "Generate typed event wrappers from spec files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cfgErr := spec.LoadWorkspaceConfig("event-spec.yaml")

			// Open the registry for the configured mode (local / git / server).
			// Fall back to a local registry on ./specs when no workspace config exists.
			var workspace string
			var allDefs []*spec.EventDef

			if cfgErr == nil {
				workspace = cfg.Workspace
				reg, err := openRegistry(cfg)
				if err != nil {
					return err
				}
				allDefs, err = listAllEvents(context.Background(), reg)
				if err != nil {
					return fmt.Errorf("list events: %w", err)
				}
			} else {
				reg, err := local.New(local.Config{SpecsDir: "./specs"})
				if err != nil {
					return fmt.Errorf("open registry: %w", err)
				}
				allDefs, err = listAllEvents(context.Background(), reg)
				if err != nil {
					return fmt.Errorf("list events: %w", err)
				}
			}

			// Resolve lang, out, and pkg from the source config when a source arg is
			// given, then apply event-pattern filtering and version pinning.
			var sourceName, pkg string
			var src *spec.SourceDef

			if len(args) > 0 {
				if cfgErr != nil {
					return fmt.Errorf("read event-spec.yaml: %w", cfgErr)
				}
				sourceName = args[0]
				sourcesDir := cfg.SourcesDir
				if sourcesDir == "" {
					sourcesDir = "./sources"
				}
				var err error
				src, err = spec.LoadSourceDef(filepath.Join(sourcesDir, sourceName+".yaml"))
				if err != nil {
					return fmt.Errorf("load source %q: %w", sourceName, err)
				}
				if !cmd.Flags().Changed("lang") {
					lang = src.Language
				}
				if !cmd.Flags().Changed("out") && src.Output.Path != "" {
					out = src.Output.Path
				}
				pkg = src.Output.Package
			}

			if lang == "" {
				return fmt.Errorf("--lang is required when no source is specified")
			}
			if out == "" {
				out = "./generated"
			}

			// Filter by source event patterns and select one version per event.
			// When no source is given, this still deduplicates multi-version registries.
			defs := applySourceConfig(allDefs, src)
			if len(defs) == 0 {
				return fmt.Errorf("no event specs found")
			}

			if err := codegen.Run(defs, lang, out, workspace, sourceName, pkg); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "generated %d event(s) to %s\n", len(defs), out)
			return nil
		},
	}

	cmd.Flags().StringVar(&lang, "lang", "", "target language: go, typescript (overrides source config)")
	cmd.Flags().StringVar(&out, "out", "", "output directory for generated files (overrides source config)")

	return cmd
}
