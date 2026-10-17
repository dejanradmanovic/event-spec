// Package swift provides the Swift codegen engine.
package swift

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/dejanradmanovic/event-spec/codegen"
)

//go:embed templates/event.swift.tmpl
var eventTmpl string

//go:embed templates/eventspec.swift.tmpl
var eventSpecTmpl string

// swiftCase converts an enum value to a camelCase Swift enum case name,
// backtick-escaping it when it collides with a reserved word. The raw spec
// value is kept as the case's String raw value by the template.
func swiftCase(s string) string { return escapeKeyword(codegen.ToCamelCase(s)) }

var funcMap = template.FuncMap{
	"swiftCase": swiftCase,
}

func init() {
	codegen.Register(
		codegen.LangConfig{
			ID:         "swift",
			Namer:      CamelCaseNamer{},
			TypeMapper: SwiftTypeMapper{},
			FileExt:    ".swift",
		},
		&Engine{},
	)
}

// Engine generates Swift source files from TemplateData.
type Engine struct{}

// Generate writes one .swift file per event plus EventSpec.swift into outDir.
func (e *Engine) Generate(td codegen.TemplateData, outDir string) error {
	evtTmpl, err := template.New("event").Funcs(funcMap).Parse(eventTmpl)
	if err != nil {
		return fmt.Errorf("parse swift event template: %w", err)
	}

	for _, ev := range td.Events {
		if err := codegen.RenderFile(evtTmpl, ev, filepath.Join(outDir, ev.ClassName+".swift")); err != nil {
			return fmt.Errorf("render swift event %s: %w", ev.NameRaw, err)
		}
	}

	specTmpl, err := template.New("eventspec").Funcs(funcMap).Parse(eventSpecTmpl)
	if err != nil {
		return fmt.Errorf("parse swift eventspec template: %w", err)
	}
	return codegen.RenderFile(specTmpl, td, filepath.Join(outDir, "EventSpec.swift"))
}
