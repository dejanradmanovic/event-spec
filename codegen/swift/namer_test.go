package swift

import "testing"

func TestSwiftNamer_EscapesKeywords(t *testing.T) {
	namer := CamelCaseNamer{}

	tests := []struct {
		name     string
		raw      string
		method   string
		typeName string
		field    string
		enumCase string
	}{
		{
			name:     "snake_case identifier",
			raw:      "product_id",
			method:   "productId",
			typeName: "ProductId",
			field:    "productId",
			enumCase: "productId",
		},
		{
			name:     "single word",
			raw:      "currency",
			method:   "currency",
			typeName: "Currency",
			field:    "currency",
			enumCase: "currency",
		},
		{
			name:     "reserved word default",
			raw:      "default",
			method:   "default",
			typeName: "Default",
			field:    "`default`",
			enumCase: "`default`",
		},
		{
			name:     "reserved word class",
			raw:      "class",
			method:   "class",
			typeName: "Class",
			field:    "`class`",
			enumCase: "`class`",
		},
		{
			name:     "reserved word in",
			raw:      "in",
			method:   "in",
			typeName: "In",
			field:    "`in`",
			enumCase: "`in`",
		},
		{
			name:     "reserved word associatedtype",
			raw:      "associatedtype",
			method:   "associatedtype",
			typeName: "Associatedtype",
			field:    "`associatedtype`",
			enumCase: "`associatedtype`",
		},
		{
			name:     "hyphenated identifier is camelCased and not reserved",
			raw:      "associated-type",
			method:   "associatedType",
			typeName: "AssociatedType",
			field:    "associatedType",
			enumCase: "associatedType",
		},
		{
			name:     "reserved word prefix is not reserved",
			raw:      "default_value",
			method:   "defaultValue",
			typeName: "DefaultValue",
			field:    "defaultValue",
			enumCase: "defaultValue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := namer.MethodName(tt.raw); got != tt.method {
				t.Errorf("MethodName(%q) = %q, want %q", tt.raw, got, tt.method)
			}
			if got := namer.TypeName(tt.raw); got != tt.typeName {
				t.Errorf("TypeName(%q) = %q, want %q", tt.raw, got, tt.typeName)
			}
			if got := namer.FieldName(tt.raw); got != tt.field {
				t.Errorf("FieldName(%q) = %q, want %q", tt.raw, got, tt.field)
			}
			if got := swiftCase(tt.raw); got != tt.enumCase {
				t.Errorf("swiftCase(%q) = %q, want %q", tt.raw, got, tt.enumCase)
			}
		})
	}
}
