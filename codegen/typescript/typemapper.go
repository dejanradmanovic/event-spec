package typescript

import "event-spec/spec"

// TSTypeMapper maps spec property types to TypeScript-native type strings.
type TSTypeMapper struct{}

// NativeType returns the TypeScript type string for the given property.
func (TSTypeMapper) NativeType(prop spec.PropertyDef) string {
	switch prop.Type {
	case spec.PropertyTypeString:
		return "string"
	case spec.PropertyTypeNumber, spec.PropertyTypeInteger:
		return "number"
	case spec.PropertyTypeBoolean:
		return "boolean"
	case spec.PropertyTypeObject:
		return "Record<string, unknown>"
	case spec.PropertyTypeArray:
		return "unknown[]"
	default:
		return "unknown"
	}
}

// OptionalType wraps a TypeScript type as a union with undefined.
func (TSTypeMapper) OptionalType(native string) string { return native + " | undefined" }
