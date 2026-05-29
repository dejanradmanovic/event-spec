package kotlin

import "github.com/dejanradmanovic/event-spec/spec"

// KotlinTypeMapper maps spec property types to Kotlin-native type strings.
type KotlinTypeMapper struct{}

// NativeType returns the Kotlin type string for the given property.
func (KotlinTypeMapper) NativeType(prop spec.PropertyDef) string {
	switch prop.Type {
	case spec.PropertyTypeString:
		return "String"
	case spec.PropertyTypeNumber:
		return "Double"
	case spec.PropertyTypeInteger:
		return "Long"
	case spec.PropertyTypeBoolean:
		return "Boolean"
	case spec.PropertyTypeObject:
		return "Map<String, Any?>"
	case spec.PropertyTypeArray:
		return "List<Any?>"
	default:
		return "Any?"
	}
}

// OptionalType wraps a Kotlin type as nullable.
func (KotlinTypeMapper) OptionalType(native string) string { return native + "?" }
