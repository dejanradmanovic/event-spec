package main

import (
	"flag"
	"fmt"
	"os"

	"event-spec/codegen"
	_ "event-spec/codegen/golang"
	_ "event-spec/codegen/typescript"
	"event-spec/spec"
)

func runGenerate(args []string) {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	lang := fs.String("lang", "", "target language: go, typescript")
	specsDir := fs.String("specs-dir", "./specs", "directory containing event spec YAML files")
	out := fs.String("out", "./generated", "output directory for generated files")
	_ = fs.Parse(args)

	if *lang == "" {
		fmt.Fprintln(os.Stderr, "error: --lang is required (go, typescript)")
		os.Exit(1)
	}

	defs, errs := spec.WalkEventDefs(*specsDir)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "error: %v\n", e)
		}
		os.Exit(1)
	}
	if len(defs) == 0 {
		fmt.Fprintf(os.Stderr, "no event specs found in %s\n", *specsDir)
		os.Exit(1)
	}

	if err := codegen.Run(defs, *lang, *out, "", ""); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("generated %d event(s) to %s\n", len(defs), *out)
}
