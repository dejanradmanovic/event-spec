package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"strings"
	"text/template"
)

// JoinQuoted formats ss as `"a" | "b" | "c"` for use in enum type declarations.
func JoinQuoted(ss []string) string {
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = `"` + s + `"`
	}
	return strings.Join(quoted, " | ")
}

// RenderFile executes tmpl with data and writes the result to path.
func RenderFile(tmpl *template.Template, data any, path string) error {
	return renderFormatted(tmpl, data, path, false)
}

// RenderGoFile executes tmpl with data, formats the result with gofmt, and writes it to path.
func RenderGoFile(tmpl *template.Template, data any, path string) error {
	return renderFormatted(tmpl, data, path, true)
}

func renderFormatted(tmpl *template.Template, data any, path string, gofmt bool) error {
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
