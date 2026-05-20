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
		Short: "Validate all event specs against their schema",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specsDir := "./specs"
			if len(args) > 0 {
				specsDir = args[0]
			}

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
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "fail on warnings (deprecated or deleted events)")
	return cmd
}
