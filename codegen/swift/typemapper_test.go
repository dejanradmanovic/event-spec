package swift

import (
	"testing"

	"github.com/dejanradmanovic/event-spec/spec"
)

func TestSwiftTypeMapper_NativeAndOptional(t *testing.T) {
	mapper := SwiftTypeMapper{}

	tests := []struct {
		name     string
		prop     spec.PropertyDef
		native   string
		optional string
	}{
		{
			name:     "string",
			prop:     spec.PropertyDef{Type: spec.PropertyTypeString},
			native:   "String",
			optional: "String?",
		},
		{
			name:     "number",
			prop:     spec.PropertyDef{Type: spec.PropertyTypeNumber},
			native:   "Double",
			optional: "Double?",
		},
		{
			name:     "integer",
			prop:     spec.PropertyDef{Type: spec.PropertyTypeInteger},
			native:   "Int",
			optional: "Int?",
		},
		{
			name:     "boolean",
			prop:     spec.PropertyDef{Type: spec.PropertyTypeBoolean},
			native:   "Bool",
			optional: "Bool?",
		},
		{
			name:     "object",
			prop:     spec.PropertyDef{Type: spec.PropertyTypeObject},
			native:   "[String: Any?]",
			optional: "[String: Any?]?",
		},
		{
			name:     "array",
			prop:     spec.PropertyDef{Type: spec.PropertyTypeArray},
			native:   "[Any?]",
			optional: "[Any?]?",
		},
		{
			name:     "unknown type falls back to Any? and is not double-wrapped",
			prop:     spec.PropertyDef{Type: spec.PropertyType("widget")},
			native:   "Any?",
			optional: "Any?",
		},
		{
			name:     "empty type falls back to Any? and is not double-wrapped",
			prop:     spec.PropertyDef{},
			native:   "Any?",
			optional: "Any?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			native := mapper.NativeType(tt.prop)
			if native != tt.native {
				t.Errorf("NativeType(%q) = %q, want %q", tt.prop.Type, native, tt.native)
			}
			if got := mapper.OptionalType(native); got != tt.optional {
				t.Errorf("OptionalType(%q) = %q, want %q", native, got, tt.optional)
			}
		})
	}
}
