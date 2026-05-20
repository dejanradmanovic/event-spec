package main

import (
	"errors"
	"fmt"
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
	return root
}

// errorf is a helper that formats and writes to stderr, then returns an error.
// Used by commands that manage their own error output.
func errorf(cmd *cobra.Command, format string, a ...any) {
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: "+format+"\n", a...)
}
