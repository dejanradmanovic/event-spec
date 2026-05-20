package codegen

import "time"

// TemplateData is the top-level data passed to struct-declaration templates (eventspec.go, index.ts).
type TemplateData struct {
	Workspace   string
	Source      string
	GeneratedAt time.Time
	Events      []EventTemplateData
	Lang        LangConfig
}

// EventTemplateData is the per-event data passed to per-event templates.
type EventTemplateData struct {
	NameRaw        string // "product_viewed"
	NameDisplay    string // "Product Viewed"
	EventName      string // canonical event name sent to providers
	Version        string // "1-0-0"
	Description    string
	MethodName     string // language-adapted method name
	ClassName      string // language-adapted type name for the event
	ParamsTypeName string // "ProductViewedProperties"
	HasProps       bool
	RequiredProps  []PropTemplateData
	OptionalProps  []PropTemplateData
}

// PropTemplateData is the per-property data used in templates.
type PropTemplateData struct {
	NameRaw      string // "product_id"
	NameField    string // language-adapted field name: "productId", "ProductId"
	TypeNative   string // language-native base type (or enum type name if IsEnum)
	TypeOptional string // language-idiomatic optional wrapper: "*string", "string | undefined"
	Required     bool
	Enum         []string
	IsEnum       bool
	EnumTypeName string // e.g. "ProductViewedCategory"
	Description  string
}
