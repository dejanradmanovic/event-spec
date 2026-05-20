package main

import (
	"fmt"

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
		Use:   "generate",
		Short: "Generate typed event wrappers from spec files",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			if err := codegen.Run(defs, lang, out, "", ""); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "generated %d event(s) to %s\n", len(defs), out)
			return nil
		},
	}

	cmd.Flags().StringVar(&lang, "lang", "", "target language: go, typescript")
	cmd.Flags().StringVar(&specsDir, "specs-dir", "./specs", "directory containing event spec YAML files")
	cmd.Flags().StringVar(&out, "out", "./generated", "output directory for generated files")
	_ = cmd.MarkFlagRequired("lang")

	return cmd
}
