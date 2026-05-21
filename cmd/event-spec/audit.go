package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dejanradmanovic/event-spec/codegen/audit"
	"github.com/dejanradmanovic/event-spec/spec"
	"github.com/spf13/cobra"
)

func newAuditCmd() *cobra.Command {
	var (
		scanPath    string
		strict      bool
		coverageMin float64
		reportFmt   string
		// sentinels to detect whether a flag was explicitly set on the CLI
		pathSet        bool
		coverageMinSet bool
		reportFmtSet   bool
	)

	cmd := &cobra.Command{
		Use:   "audit [source]",
		Short: "Scan a codebase and report event usage against the spec registry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load workspace config (optional).
			cfg, cfgErr := spec.LoadWorkspaceConfig("event-spec.yaml")

			// Resolve defaults from workspace config → built-in fallback.
			if cfgErr == nil {
				if !pathSet && cfg.Audit.Path != "" {
					scanPath = cfg.Audit.Path
				}
				if !coverageMinSet && cfg.Audit.CoverageMin > 0 {
					coverageMin = cfg.Audit.CoverageMin
				}
				if !reportFmtSet && cfg.Audit.Report != "" {
					reportFmt = cfg.Audit.Report
				}
			}
			if scanPath == "" {
				scanPath = "."
			}
			if reportFmt == "" {
				reportFmt = "text"
			}

			switch reportFmt {
			case "json", "text", "html":
			default:
				return fmt.Errorf("--report must be one of: json, text, html")
			}

			// Resolve all event defs and source language from the registry.
			var sourceName, language string
			var defs []*spec.EventDef

			if len(args) > 0 {
				if cfgErr != nil {
					return fmt.Errorf("read event-spec.yaml: %w", cfgErr)
				}
				sourceName = args[0]
				sourcesDir := cfg.SourcesDir
				if sourcesDir == "" {
					sourcesDir = "./sources"
				}
				src, err := spec.LoadSourceDef(filepath.Join(sourcesDir, sourceName+".yaml"))
				if err != nil {
					return fmt.Errorf("load source %q: %w", sourceName, err)
				}
				language = src.Language

				reg, err := openRegistry(cfg)
				if err != nil {
					return err
				}
				allDefs, err := listAllEvents(context.Background(), reg)
				if err != nil {
					return fmt.Errorf("list events: %w", err)
				}
				defs = applySourceConfig(allDefs, src)
			} else {
				// No source arg: use local specs and attempt to infer language.
				var specsDir string
				if cfgErr == nil {
					var err error
					specsDir, err = resolveSpecsPath(cfg)
					if err != nil {
						return err
					}
				} else {
					specsDir = "./specs"
				}

				var walkErrs []error
				defs, walkErrs = spec.WalkEventDefs(specsDir)
				for _, e := range walkErrs {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", e)
				}

				// Infer language from the first source def found.
				sourcesDir := "./sources"
				if cfgErr == nil && cfg.SourcesDir != "" {
					sourcesDir = cfg.SourcesDir
				}
				if srcDefs, _ := spec.WalkSourceDefs(sourcesDir); len(srcDefs) > 0 {
					language = srcDefs[0].Language
				}
				if language == "" {
					language = "go" // default when not determinable
				}
			}

			if len(defs) == 0 {
				return fmt.Errorf("no event specs found")
			}

			// Scan the codebase.
			scanner, err := audit.NewScanner(language)
			if err != nil {
				return err
			}
			result, err := scanner.ScanDir(scanPath)
			if err != nil {
				return fmt.Errorf("scan %s: %w", scanPath, err)
			}

			// Build the coverage report.
			if sourceName == "" {
				sourceName = scanPath
			}
			report := audit.BuildReport(sourceName, language, defs, result)

			// Render the report.
			out := cmd.OutOrStdout()
			switch reportFmt {
			case "json":
				if err := report.WriteJSON(out); err != nil {
					return fmt.Errorf("write report: %w", err)
				}
			case "html":
				report.WriteHTML(out)
			default:
				report.WriteText(out)
			}

			// Enforcement checks.
			var violations []string

			if coverageMin > 0 && report.CoveragePct < coverageMin {
				violations = append(violations, fmt.Sprintf(
					"coverage %.1f%% is below the required minimum of %.1f%%",
					report.CoveragePct, coverageMin,
				))
			}

			if strict {
				for _, u := range report.Unused {
					if u.Required {
						violations = append(violations, fmt.Sprintf(
							"required event %q is unused (spec: %s)", u.EventKey, u.SpecFile,
						))
					}
				}
			}

			if len(violations) > 0 {
				for _, v := range violations {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", v)
				}
				return &exitCodeError{code: 1, err: fmt.Errorf("%d audit violation(s)", len(violations))}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&scanPath, "path", "", "path to scan (default: audit.path from event-spec.yaml, or current directory)")
	cmd.Flags().BoolVar(&strict, "strict", false, "fail if any required events are unused")
	cmd.Flags().Float64Var(&coverageMin, "coverage-min", 0, "minimum coverage % required (default: audit.coverage_min from event-spec.yaml, or 0)")
	cmd.Flags().StringVar(&reportFmt, "report", "", "output format: json | text | html (default: audit.report from event-spec.yaml, or text)")

	// Track which flags were explicitly provided on the command line.
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		pathSet = cmd.Flags().Changed("path")
		coverageMinSet = cmd.Flags().Changed("coverage-min")
		reportFmtSet = cmd.Flags().Changed("report")
	}

	return cmd
}
