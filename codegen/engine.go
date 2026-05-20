// Package codegen transforms event specs into typed language wrappers using text/template.
package codegen

import (
	"fmt"
	"os"

	"event-spec/spec"
)

// Engine renders TemplateData into language-specific source files.
type Engine interface {
	Generate(td TemplateData, outDir string) error
}

// Run resolves the engine for lang, builds template data, and generates output files into outDir.
// workspace and source are metadata embedded in generated file headers.
func Run(events []*spec.EventDef, lang, outDir, workspace, source string) error {
	entry, ok := registry[lang]
	if !ok {
		return fmt.Errorf("unsupported language: %q", lang)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	td := buildTemplateData(events, entry.config, workspace, source)
	return entry.engine.Generate(td, outDir)
}
