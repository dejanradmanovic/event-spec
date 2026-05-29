package kotlin

import "github.com/dejanradmanovic/event-spec/codegen"

// CamelCaseNamer uses camelCase for methods and fields, PascalCase for types (Kotlin convention).
type CamelCaseNamer struct{}

func (CamelCaseNamer) MethodName(raw string) string { return codegen.ToCamelCase(raw) }
func (CamelCaseNamer) TypeName(raw string) string   { return codegen.ToPascalCase(raw) }
func (CamelCaseNamer) FieldName(raw string) string  { return codegen.ToCamelCase(raw) }
