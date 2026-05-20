// Package codegen transforms event specs into typed language wrappers using text/template.
package codegen

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"event-spec/spec"
)

//go:embed templates/go/event.go.tmpl
var goEventTmpl string

//go:embed templates/go/eventspec.go.tmpl
var goEventSpecTmpl string

//go:embed templates/typescript/event.ts.tmpl
var tsEventTmpl string

//go:embed templates/typescript/index.ts.tmpl
var tsIndexTmpl string

var funcMap = template.FuncMap{
	"toPascal": toPascalCase,
	"joinQuoted": func(ss []string) string {
		quoted := make([]string, len(ss))
		for i, s := range ss {
			quoted[i] = `"` + s + `"`
		}
		return strings.Join(quoted, " | ")
	},
}

// Engine renders event specs into language-specific source files using text/template.
type Engine struct{}

// Generate renders all events into outDir for the given language.
// workspace and source are metadata embedded in the generated output header.
func (e *Engine) Generate(events []*spec.EventDef, lang, outDir, workspace, source string) error {
	lc, err := LookupLang(lang)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	td := buildTemplateData(events, lc, workspace, source)

	switch lang {
	case "go":
		return e.generateGo(td, outDir)
	case "typescript":
		return e.generateTypeScript(td, outDir)
	default:
		return fmt.Errorf("unsupported language: %q", lang)
	}
}

func (e *Engine) generateGo(td TemplateData, outDir string) error {
	evtTmpl, err := template.New("event").Funcs(funcMap).Parse(goEventTmpl)
	if err != nil {
		return fmt.Errorf("parse go event template: %w", err)
	}

	for _, ev := range td.Events {
		if err := renderGoFile(evtTmpl, ev, filepath.Join(outDir, ev.NameRaw+".go")); err != nil {
			return fmt.Errorf("render go event %s: %w", ev.NameRaw, err)
		}
	}

	specTmpl, err := template.New("eventspec").Funcs(funcMap).Parse(goEventSpecTmpl)
	if err != nil {
		return fmt.Errorf("parse go eventspec template: %w", err)
	}
	return renderGoFile(specTmpl, td, filepath.Join(outDir, "eventspec.go"))
}

func (e *Engine) generateTypeScript(td TemplateData, outDir string) error {
	evtTmpl, err := template.New("event").Funcs(funcMap).Parse(tsEventTmpl)
	if err != nil {
		return fmt.Errorf("parse ts event template: %w", err)
	}

	for _, ev := range td.Events {
		if err := renderFile(evtTmpl, ev, filepath.Join(outDir, ev.NameRaw+".ts")); err != nil {
			return fmt.Errorf("render ts event %s: %w", ev.NameRaw, err)
		}
	}

	indexTmpl, err := template.New("index").Funcs(funcMap).Parse(tsIndexTmpl)
	if err != nil {
		return fmt.Errorf("parse ts index template: %w", err)
	}
	return renderFile(indexTmpl, td, filepath.Join(outDir, "index.ts"))
}

func renderFile(tmpl *template.Template, data any, path string) error {
	return renderFileFormatted(tmpl, data, path, false)
}

func renderGoFile(tmpl *template.Template, data any, path string) error {
	return renderFileFormatted(tmpl, data, path, true)
}

func renderFileFormatted(tmpl *template.Template, data any, path string, gofmt bool) error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}
	src := buf.Bytes()
	if gofmt {
		formatted, err := format.Source(src)
		if err != nil {
			return fmt.Errorf("format %s: %w\n--- source ---\n%s", path, err, src)
		}
		src = formatted
	}
	if err := os.WriteFile(path, src, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// buildTemplateData converts a slice of EventDef into template-ready data.
// Events are sorted by name for deterministic output.
func buildTemplateData(events []*spec.EventDef, lc LangConfig, workspace, source string) TemplateData {
	sorted := make([]*spec.EventDef, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	td := TemplateData{
		Workspace:   workspace,
		Source:      source,
		GeneratedAt: time.Now().UTC(),
		Lang:        lc,
	}
	for _, def := range sorted {
		td.Events = append(td.Events, buildEventData(def, lc))
	}
	return td
}

func buildEventData(def *spec.EventDef, lc LangConfig) EventTemplateData {
	namer := lc.Namer
	className := namer.TypeName(def.Name)

	displayName := def.DisplayName
	if displayName == "" {
		displayName = def.Name
	}
	eventName := def.EventName
	if eventName == "" {
		eventName = def.Name
	}

	ev := EventTemplateData{
		NameRaw:        def.Name,
		NameDisplay:    displayName,
		EventName:      eventName,
		Version:        def.Version,
		Description:    def.Description,
		MethodName:     namer.MethodName(def.Name),
		ClassName:      className,
		ParamsTypeName: className + "Properties",
	}

	for name, prop := range def.Properties {
		pd := buildPropData(name, prop, className, lc)
		if prop.Required {
			ev.RequiredProps = append(ev.RequiredProps, pd)
		} else {
			ev.OptionalProps = append(ev.OptionalProps, pd)
		}
	}

	sort.Slice(ev.RequiredProps, func(i, j int) bool { return ev.RequiredProps[i].NameRaw < ev.RequiredProps[j].NameRaw })
	sort.Slice(ev.OptionalProps, func(i, j int) bool { return ev.OptionalProps[i].NameRaw < ev.OptionalProps[j].NameRaw })

	ev.HasProps = len(ev.RequiredProps)+len(ev.OptionalProps) > 0
	return ev
}

func buildPropData(name string, prop spec.PropertyDef, className string, lc LangConfig) PropTemplateData {
	namer := lc.Namer
	isEnum := len(prop.Enum) > 0 && prop.Type == spec.PropertyTypeString
	enumTypeName := ""
	if isEnum {
		enumTypeName = className + namer.TypeName(name)
	}

	var nativeType string
	if isEnum {
		nativeType = enumTypeName
	} else {
		nativeType = NativeType(prop, lc.ID)
	}

	return PropTemplateData{
		NameRaw:      name,
		NameField:    namer.FieldName(name),
		TypeNative:   nativeType,
		TypeOptional: OptionalType(nativeType, lc.ID),
		Required:     prop.Required,
		Enum:         prop.Enum,
		IsEnum:       isEnum,
		EnumTypeName: enumTypeName,
		Description:  prop.Description,
	}
}
