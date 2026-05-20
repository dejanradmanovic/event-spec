package main

import (
	"fmt"

	"event-spec/spec"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var strict bool

	cmd := &cobra.Command{
		Use:   "validate [spec-dir]",
		Short: "Validate event specs, sources, destinations, and workspace config",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Explicit spec-dir: only validate event specs (backward-compat).
			if len(args) > 0 {
				return validateEventSpecs(cmd, args[0], strict)
			}

			// No explicit arg: validate the whole workspace.
			cfg, cfgErr := spec.LoadWorkspaceConfig("event-spec.yaml")

			var specsDir, sourcesDir, destinationsDir string
			if cfgErr == nil {
				var err error
				specsDir, err = resolveSpecsPath(cfg)
				if err != nil {
					return err
				}
				sourcesDir = cfg.SourcesDir
				if sourcesDir == "" {
					sourcesDir = "./sources"
				}
				destinationsDir = cfg.DestinationsDir
				if destinationsDir == "" {
					destinationsDir = "./destinations"
				}
			} else {
				specsDir = "./specs"
				sourcesDir = "./sources"
				destinationsDir = "./destinations"
			}

			var errorCount, warnCount int
			report := func(format string, a ...any) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", a...)
			}

			// Validate workspace config when present.
			if cfgErr == nil {
				for _, verr := range spec.ValidateWorkspaceConfig(cfg, "event-spec.yaml") {
					report("error: %v", verr)
					errorCount++
				}
			}

			// Validate event specs.
			defs, walkErrs := spec.WalkEventDefs(specsDir)
			for _, err := range walkErrs {
				report("error: %v", err)
				errorCount++
			}
			for _, def := range defs {
				for _, verr := range spec.ValidateEventDef(def) {
					report("error: %v", verr)
					errorCount++
				}
				if def.Status == spec.StatusDeprecated || def.Status == spec.StatusDeleted {
					report("warning: %s: status %q", def.SourcePath, def.Status)
					warnCount++
				}
			}

			// Validate sources.
			srcDefs, srcErrs := spec.WalkSourceDefs(sourcesDir)
			for _, err := range srcErrs {
				report("error: %v", err)
				errorCount++
			}
			for _, def := range srcDefs {
				for _, verr := range spec.ValidateSourceDef(def) {
					report("error: %v", verr)
					errorCount++
				}
			}

			// Validate destinations.
			dstDefs, dstErrs := spec.WalkDestinationDefs(destinationsDir)
			for _, err := range dstErrs {
				report("error: %v", err)
				errorCount++
			}
			for _, def := range dstDefs {
				for _, verr := range spec.ValidateDestinationDef(def) {
					report("error: %v", verr)
					errorCount++
				}
			}

			if errorCount > 0 {
				return fmt.Errorf("%d error(s)", errorCount)
			}
			if strict && warnCount > 0 {
				return fmt.Errorf("%d warning(s) (strict mode)", warnCount)
			}

			msg := fmt.Sprintf("validated %d event spec(s)", len(defs))
			if len(srcDefs) > 0 {
				msg += fmt.Sprintf(", %d source(s)", len(srcDefs))
			}
			if len(dstDefs) > 0 {
				msg += fmt.Sprintf(", %d destination(s)", len(dstDefs))
			}
			msg += ": ok"
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), msg)
			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "fail on warnings (deprecated or deleted events)")
	return cmd
}

// validateEventSpecs validates only the event spec YAML files in specsDir.
// Used when an explicit spec-dir argument is provided.
func validateEventSpecs(cmd *cobra.Command, specsDir string, strict bool) error {
	defs, walkErrs := spec.WalkEventDefs(specsDir)

	var errorCount, warnCount int
	for _, err := range walkErrs {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", err)
		errorCount++
	}
	for _, def := range defs {
		for _, verr := range spec.ValidateEventDef(def) {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", verr)
			errorCount++
		}
		if def.Status == spec.StatusDeprecated || def.Status == spec.StatusDeleted {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: status %q\n", def.SourcePath, def.Status)
			warnCount++
		}
	}

	if errorCount > 0 {
		return fmt.Errorf("%d error(s)", errorCount)
	}
	if strict && warnCount > 0 {
		return fmt.Errorf("%d warning(s) (strict mode)", warnCount)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "validated %d event spec(s): ok\n", len(defs))
	return nil
}
