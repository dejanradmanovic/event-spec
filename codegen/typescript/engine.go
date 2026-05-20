package typescript

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/dejanradmanovic/event-spec/codegen"
)

//go:embed templates/event.ts.tmpl
var eventTmpl string

//go:embed templates/index.ts.tmpl
var indexTmpl string

var funcMap = template.FuncMap{
	"joinQuoted": codegen.JoinQuoted,
}

func init() {
	codegen.Register(
		codegen.LangConfig{
			ID:         "typescript",
			Namer:      CamelCaseNamer{},
			TypeMapper: TSTypeMapper{},
			FileExt:    ".ts",
		},
		&Engine{},
	)
}

// Engine generates TypeScript source files from TemplateData.
type Engine struct{}

// Generate writes one .ts file per event plus index.ts into outDir.
func (e *Engine) Generate(td codegen.TemplateData, outDir string) error {
	evtTmpl, err := template.New("event").Funcs(funcMap).Parse(eventTmpl)
	if err != nil {
		return fmt.Errorf("parse ts event template: %w", err)
	}

	for _, ev := range td.Events {
		if err := codegen.RenderFile(evtTmpl, ev, filepath.Join(outDir, ev.NameRaw+".ts")); err != nil {
			return fmt.Errorf("render ts event %s: %w", ev.NameRaw, err)
		}
	}

	idxTmpl, err := template.New("index").Funcs(funcMap).Parse(indexTmpl)
	if err != nil {
		return fmt.Errorf("parse ts index template: %w", err)
	}
	return codegen.RenderFile(idxTmpl, td, filepath.Join(outDir, "index.ts"))
}
