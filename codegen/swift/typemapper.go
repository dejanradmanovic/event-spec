package swift

import (
	"strings"

	"github.com/dejanradmanovic/event-spec/spec"
)

// SwiftTypeMapper maps spec property types to Swift-native type strings.
type SwiftTypeMapper struct{}

// NativeType returns the Swift type string for the given property.
func (SwiftTypeMapper) NativeType(prop spec.PropertyDef) string {
	switch prop.Type {
	case spec.PropertyTypeString:
		return "String"
	case spec.PropertyTypeNumber:
		return "Double"
	case spec.PropertyTypeInteger:
		return "Int"
	case spec.PropertyTypeBoolean:
		return "Bool"
	case spec.PropertyTypeObject:
		return "[String: Any?]"
	case spec.PropertyTypeArray:
		return "[Any?]"
	default:
		return "Any?"
	}
}

// OptionalType wraps a Swift type as an optional. Already-optional types are
// returned unchanged so the unknown-type default "Any?" does not become "Any??".
func (SwiftTypeMapper) OptionalType(native string) string {
	if strings.HasSuffix(native, "?") {
		return native
	}
	return native + "?"
}
