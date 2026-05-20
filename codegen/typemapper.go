package codegen

import "github.com/dejanradmanovic/event-spec/spec"

// TypeMapper converts spec property definitions to language-native type strings.
type TypeMapper interface {
	NativeType(prop spec.PropertyDef) string
	OptionalType(native string) string
}
