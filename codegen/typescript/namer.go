// Package typescript provides the TypeScript codegen engine.
package typescript

import "github.com/dejanradmanovic/event-spec/codegen"

// CamelCaseNamer uses camelCase for methods and fields, PascalCase for types (TypeScript convention).
type CamelCaseNamer struct{}

// MethodName returns the camelCase method name for raw.
func (CamelCaseNamer) MethodName(raw string) string { return codegen.ToCamelCase(raw) }

// TypeName returns the PascalCase type name for raw.
func (CamelCaseNamer) TypeName(raw string) string { return codegen.ToPascalCase(raw) }

// FieldName returns the camelCase field name for raw.
func (CamelCaseNamer) FieldName(raw string) string { return codegen.ToCamelCase(raw) }
