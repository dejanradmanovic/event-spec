package main

import (
	"fmt"
	"path/filepath"

	"event-spec/codegen"
	_ "event-spec/codegen/golang"
	_ "event-spec/codegen/typescript"
	"event-spec/spec"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	var (
		lang     string
		specsDir string
		out      string
	)

	cmd := &cobra.Command{
		Use:   "generate [source]",
		Short: "Generate typed event wrappers from spec files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var workspace, sourceName string

			if len(args) > 0 {
				sourceName = args[0]
				cfg, err := spec.LoadWorkspaceConfig("event-spec.yaml")
				if err != nil {
					return fmt.Errorf("read event-spec.yaml: %w", err)
				}
				workspace = cfg.Workspace

				if !cmd.Flags().Changed("specs-dir") && cfg.SpecsDir != "" {
					specsDir = cfg.SpecsDir
				}

				sourcesDir := cfg.SourcesDir
				if sourcesDir == "" {
					sourcesDir = "./sources"
				}
				src, err := spec.LoadSourceDef(filepath.Join(sourcesDir, sourceName+".yaml"))
				if err != nil {
					return fmt.Errorf("load source %q: %w", sourceName, err)
				}
				if !cmd.Flags().Changed("lang") {
					lang = src.Language
				}
				if !cmd.Flags().Changed("out") && src.Output.Path != "" {
					out = src.Output.Path
				}
			}

			if lang == "" {
				return fmt.Errorf("--lang is required when no source is specified")
			}
			if out == "" {
				out = "./generated"
			}

			defs, errs := spec.WalkEventDefs(specsDir)
			if len(errs) > 0 {
				for _, e := range errs {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", e)
				}
				return fmt.Errorf("failed to load specs from %s", specsDir)
			}
			if len(defs) == 0 {
				return fmt.Errorf("no event specs found in %s", specsDir)
			}

			if err := codegen.Run(defs, lang, out, workspace, sourceName); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "generated %d event(s) to %s\n", len(defs), out)
			return nil
		},
	}

	cmd.Flags().StringVar(&lang, "lang", "", "target language: go, typescript (overrides source config)")
	cmd.Flags().StringVar(&specsDir, "specs-dir", "./specs", "directory containing event spec YAML files")
	cmd.Flags().StringVar(&out, "out", "", "output directory for generated files (overrides source config)")

	return cmd
}
