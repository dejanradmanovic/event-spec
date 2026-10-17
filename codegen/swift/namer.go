package swift

import "github.com/dejanradmanovic/event-spec/codegen"

// CamelCaseNamer uses camelCase for methods and fields, PascalCase for types (Swift convention).
type CamelCaseNamer struct{}

// MethodName returns the camelCase method name for raw.
func (CamelCaseNamer) MethodName(raw string) string { return codegen.ToCamelCase(raw) }

// TypeName returns the PascalCase type name for raw.
func (CamelCaseNamer) TypeName(raw string) string { return codegen.ToPascalCase(raw) }

// FieldName returns the camelCase field name for raw, backtick-escaped when it
// collides with a Swift reserved word.
func (CamelCaseNamer) FieldName(raw string) string {
	return escapeKeyword(codegen.ToCamelCase(raw))
}

// swiftKeywords are the Swift reserved words that cannot appear as a bare
// identifier in generated declarations.
var swiftKeywords = map[string]bool{
	"any": true, "as": true, "associatedtype": true, "async": true, "await": true,
	"break": true, "case": true, "catch": true, "class": true, "continue": true,
	"default": true, "defer": true, "deinit": true, "else": true, "enum": true,
	"extension": true, "false": true, "for": true, "func": true, "guard": true,
	"if": true, "import": true, "in": true, "init": true, "internal": true,
	"is": true, "let": true, "nil": true, "operator": true, "protocol": true,
	"public": true, "repeat": true, "return": true, "self": true, "some": true,
	"static": true, "struct": true, "subscript": true, "switch": true, "throw": true,
	"true": true, "try": true, "typealias": true, "var": true, "where": true,
	"while": true,
}

// escapeKeyword wraps s in backticks when it is a Swift reserved word, which is
// how Swift spells an identifier that shadows a keyword.
func escapeKeyword(s string) string {
	if swiftKeywords[s] {
		return "`" + s + "`"
	}
	return s
}
