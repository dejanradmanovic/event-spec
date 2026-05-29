// Package kotlin provides the Kotlin codegen engine.
package kotlin

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/dejanradmanovic/event-spec/codegen"
)

//go:embed templates/event.kt.tmpl
var eventTmpl string

//go:embed templates/eventspec.kt.tmpl
var eventSpecTmpl string

// toUpperSnake converts a snake_case or space-separated value to UPPER_SNAKE_CASE
// for use as a Kotlin enum constant name.
func toUpperSnake(s string) string {
	words := codegen.SplitWords(s)
	upper := make([]string, len(words))
	for i, w := range words {
		upper[i] = strings.ToUpper(w)
	}
	return strings.Join(upper, "_")
}

var funcMap = template.FuncMap{
	"joinQuoted":   codegen.JoinQuoted,
	"toUpperSnake": toUpperSnake,
}

func init() {
	codegen.Register(
		codegen.LangConfig{
			ID:         "kotlin",
			Namer:      CamelCaseNamer{},
			TypeMapper: KotlinTypeMapper{},
			FileExt:    ".kt",
		},
		&Engine{},
	)
}

// Engine generates Kotlin source files from TemplateData.
type Engine struct{}

// Generate writes one .kt file per event plus EventSpec.kt into outDir.
func (e *Engine) Generate(td codegen.TemplateData, outDir string) error {
	evtTmpl, err := template.New("event").Funcs(funcMap).Parse(eventTmpl)
	if err != nil {
		return fmt.Errorf("parse kotlin event template: %w", err)
	}

	for _, ev := range td.Events {
		if err := codegen.RenderFile(evtTmpl, ev, filepath.Join(outDir, ev.ClassName+".kt")); err != nil {
			return fmt.Errorf("render kotlin event %s: %w", ev.NameRaw, err)
		}
	}

	specTmpl, err := template.New("eventspec").Funcs(funcMap).Parse(eventSpecTmpl)
	if err != nil {
		return fmt.Errorf("parse kotlin eventspec template: %w", err)
	}
	return codegen.RenderFile(specTmpl, td, filepath.Join(outDir, "EventSpec.kt"))
}
