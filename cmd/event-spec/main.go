package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
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
	return root
}
