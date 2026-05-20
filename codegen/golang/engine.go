package golang

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/dejanradmanovic/event-spec/codegen"
)

//go:embed templates/event.go.tmpl
var eventTmpl string

//go:embed templates/eventspec.go.tmpl
var eventSpecTmpl string

var funcMap = template.FuncMap{
	"toPascal":   codegen.ToPascalCase,
	"joinQuoted": codegen.JoinQuoted,
}

func init() {
	codegen.Register(
		codegen.LangConfig{
			ID:         "go",
			Namer:      GoNamer{},
			TypeMapper: GoTypeMapper{},
			FileExt:    ".go",
		},
		&Engine{},
	)
}

// Engine generates Go source files from TemplateData.
type Engine struct{}

// Generate writes one .go file per event plus eventspec.go into outDir.
func (e *Engine) Generate(td codegen.TemplateData, outDir string) error {
	evtTmpl, err := template.New("event").Funcs(funcMap).Parse(eventTmpl)
	if err != nil {
		return fmt.Errorf("parse go event template: %w", err)
	}

	for _, ev := range td.Events {
		if err := codegen.RenderGoFile(evtTmpl, ev, filepath.Join(outDir, ev.NameRaw+".go")); err != nil {
			return fmt.Errorf("render go event %s: %w", ev.NameRaw, err)
		}
	}

	specTmpl, err := template.New("eventspec").Funcs(funcMap).Parse(eventSpecTmpl)
	if err != nil {
		return fmt.Errorf("parse go eventspec template: %w", err)
	}
	return codegen.RenderGoFile(specTmpl, td, filepath.Join(outDir, "eventspec.go"))
}
