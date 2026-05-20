package codegen

import "event-spec/spec"

// NativeType returns the language-native type string for a property.
// If the property has enum values (string type only), callers should override
// TypeNative with the generated enum type name instead.
func NativeType(prop spec.PropertyDef, lang string) string {
	switch lang {
	case "go":
		return goNativeType(prop.Type)
	case "typescript":
		return tsNativeType(prop.Type)
	default:
		return string(prop.Type)
	}
}

// OptionalType wraps a native type in the language-idiomatic optional form.
func OptionalType(native, lang string) string {
	switch lang {
	case "go":
		return "*" + native
	case "typescript":
		return native + " | undefined"
	default:
		return native
	}
}

func goNativeType(t spec.PropertyType) string {
	switch t {
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

func tsNativeType(t spec.PropertyType) string {
	switch t {
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
