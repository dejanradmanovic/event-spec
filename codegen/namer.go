package codegen

import (
	"strings"
	"unicode"
)

// Namer converts raw snake_case identifiers into language-idiomatic names.
type Namer interface {
	MethodName(raw string) string
	TypeName(raw string) string
	FieldName(raw string) string
}

// GoNamer converts all identifiers to PascalCase (Go convention).
type GoNamer struct{}

// MethodName returns the PascalCase method name for raw.
func (GoNamer) MethodName(raw string) string { return toPascalCase(raw) }

// TypeName returns the PascalCase type name for raw.
func (GoNamer) TypeName(raw string) string { return toPascalCase(raw) }

// FieldName returns the PascalCase field name for raw.
func (GoNamer) FieldName(raw string) string { return toPascalCase(raw) }

// CamelCaseNamer uses camelCase for methods and fields, PascalCase for types (TypeScript, Swift, Kotlin).
type CamelCaseNamer struct{}

// MethodName returns the camelCase method name for raw.
func (CamelCaseNamer) MethodName(raw string) string { return toCamelCase(raw) }

// TypeName returns the PascalCase type name for raw.
func (CamelCaseNamer) TypeName(raw string) string { return toPascalCase(raw) }

// FieldName returns the camelCase field name for raw.
func (CamelCaseNamer) FieldName(raw string) string { return toCamelCase(raw) }

func toPascalCase(s string) string {
	words := splitWords(s)
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.Join(words, "")
}

func toCamelCase(s string) string {
	words := splitWords(s)
	for i, w := range words {
		if w == "" {
			continue
		}
		if i == 0 {
			words[i] = strings.ToLower(w)
		} else {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, "")
}

func splitWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || unicode.IsSpace(r)
	})
}
