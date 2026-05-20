// Package golang provides the Go codegen engine.
package golang

import "github.com/dejanradmanovic/event-spec/codegen"

// GoNamer converts all identifiers to PascalCase (Go convention).
type GoNamer struct{}

// MethodName returns the PascalCase method name for raw.
func (GoNamer) MethodName(raw string) string { return codegen.ToPascalCase(raw) }

// TypeName returns the PascalCase type name for raw.
func (GoNamer) TypeName(raw string) string { return codegen.ToPascalCase(raw) }

// FieldName returns the PascalCase field name for raw.
func (GoNamer) FieldName(raw string) string { return codegen.ToPascalCase(raw) }
