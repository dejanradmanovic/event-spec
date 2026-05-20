package golang

import "event-spec/spec"

// GoTypeMapper maps spec property types to Go-native type strings.
type GoTypeMapper struct{}

// NativeType returns the Go type string for the given property.
func (GoTypeMapper) NativeType(prop spec.PropertyDef) string {
	switch prop.Type {
	case spec.PropertyTypeString:
		return "string"
	case spec.PropertyTypeNumber:
		return "float64"
	case spec.PropertyTypeInteger:
		return "int64"
	case spec.PropertyTypeBoolean:
		return "bool"
	case spec.PropertyTypeObject:
		return "map[string]any"
	case spec.PropertyTypeArray:
		return "[]any"
	default:
		return "any"
	}
}

// OptionalType wraps a Go type as a pointer (*T).
func (GoTypeMapper) OptionalType(native string) string { return "*" + native }
