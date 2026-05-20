package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		var ee *exitCodeError
		if errors.As(err, &ee) {
			os.Exit(ee.code)
		}
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "event-spec",
		Short:        "Provider-agnostic analytics codegen and governance CLI",
		SilenceUsage: true,
	}
	root.AddCommand(newGenerateCmd())
	root.AddCommand(newValidateCmd())
	root.AddCommand(newDiffCmd())
	root.AddCommand(newNewCmd())
	return root
}
