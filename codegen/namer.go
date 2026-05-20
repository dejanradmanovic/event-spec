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

// ToPascalCase converts a snake_case or kebab-case string to PascalCase.
func ToPascalCase(s string) string {
	words := SplitWords(s)
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.Join(words, "")
}

// ToCamelCase converts a snake_case or kebab-case string to camelCase.
func ToCamelCase(s string) string {
	words := SplitWords(s)
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

// SplitWords splits s on underscores, hyphens, and whitespace.
func SplitWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || unicode.IsSpace(r)
	})
}
