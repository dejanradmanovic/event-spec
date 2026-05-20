package spec

import (
	"path/filepath"
	"testing"
)

func validProductViewedDef() *EventDef {
	min := 0.0
	return &EventDef{
		Schema:    "https://event-spec.io/schemas/event/v1",
		Name:      "product_viewed",
		Version:   "1-2-0",
		Namespace: "ecommerce",
		EventName: "Product Viewed",
		Status:    StatusActive,
		Type:      TypeTrack,
		Properties: map[string]PropertyDef{
			"product_id":   {Type: PropertyTypeString, Required: true},
			"product_name": {Type: PropertyTypeString, Required: true},
			"category": {
				Type:     PropertyTypeString,
				Required: true,
				Enum:     []string{"clothing", "electronics", "books", "home", "sports", "other"},
			},
			"price":    {Type: PropertyTypeNumber, Required: true, Minimum: &min},
			"currency": {Type: PropertyTypeString, Required: false, Pattern: "^[A-Z]{3}$"},
		},
	}
}

// --- ValidateEventDef ---

func TestValidateEventDef_valid(t *testing.T) {
	errs := ValidateEventDef(validProductViewedDef())
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateEventDef_missingRequiredFields(t *testing.T) {
	def := &EventDef{} // all zero values
	errs := ValidateEventDef(def)

	required := map[string]bool{
		"name":       false,
		"version":    false,
		"namespace":  false,
		"event_name": false,
		"status":     false,
		"type":       false,
	}
	for _, e := range errs {
		required[e.Field] = true
	}
	for field, found := range required {
		if !found {
			t.Errorf("expected error for field %q, but none found", field)
		}
	}
}

func TestValidateEventDef_invalidStatus(t *testing.T) {
	def := validProductViewedDef()
	def.Status = "unknown"
	errs := ValidateEventDef(def)
	if !containsField(errs, "status") {
		t.Error("expected validation error for invalid status")
	}
}

func TestValidateEventDef_invalidType(t *testing.T) {
	def := validProductViewedDef()
	def.Type = "click"
	errs := ValidateEventDef(def)
	if !containsField(errs, "type") {
		t.Error("expected validation error for invalid event type")
	}
}

func TestValidateEventDef_invalidPropertyType(t *testing.T) {
	def := validProductViewedDef()
	def.Properties["bad_prop"] = PropertyDef{Type: "url"}
	errs := ValidateEventDef(def)
	if !containsField(errs, "properties.bad_prop.type") {
		t.Error("expected validation error for invalid property type")
	}
}

func TestValidateEventDef_invalidSchemaVer(t *testing.T) {
	def := validProductViewedDef()
	def.Version = "1.2.0"
	errs := ValidateEventDef(def)
	if !containsField(errs, "version") {
		t.Error("expected validation error for invalid SchemaVer format")
	}
}

func TestValidateEventDef_samplingOutOfRange(t *testing.T) {
	def := validProductViewedDef()
	def.Sampling = &SamplingConfig{Strategy: SamplingRandom, Rate: 1.5}
	errs := ValidateEventDef(def)
	if !containsField(errs, "sampling.rate") {
		t.Error("expected validation error for sampling.rate > 1")
	}
}

func TestValidateEventDef_invalidSamplingStrategy(t *testing.T) {
	def := validProductViewedDef()
	def.Sampling = &SamplingConfig{Strategy: "deterministic", Rate: 0.5}
	errs := ValidateEventDef(def)
	if !containsField(errs, "sampling.strategy") {
		t.Error("expected validation error for invalid sampling strategy")
	}
}

func TestValidateEventDef_invalidPropertyPriority(t *testing.T) {
	def := validProductViewedDef()
	def.PropertyPriority = "event_only"
	errs := ValidateEventDef(def)
	if !containsField(errs, "property_priority") {
		t.Error("expected validation error for invalid property_priority")
	}
}

func TestValidateEventDef_fromFile(t *testing.T) {
	path := filepath.Join("testdata", "specs", "ecommerce", "product_viewed", "1-2-0.yaml")
	def, err := LoadEventDef(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	errs := ValidateEventDef(def)
	if len(errs) != 0 {
		t.Errorf("testdata spec should be valid, got errors: %v", errs)
	}
}

// --- BuildJSONSchema ---

func TestBuildJSONSchema(t *testing.T) {
	def := validProductViewedDef()
	schema := BuildJSONSchema(def)

	if schema["$schema"] != "http://json-schema.org/draft-07/schema#" {
		t.Errorf("$schema = %v, want draft-07 URL", schema["$schema"])
	}
	if schema["type"] != "object" {
		t.Errorf("type = %v, want object", schema["type"])
	}
	if schema["title"] != "Product Viewed" {
		t.Errorf("title = %v, want %q", schema["title"], "Product Viewed")
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties is not map[string]any")
	}
	if _, ok := props["product_id"]; !ok {
		t.Error("properties missing product_id")
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required is not []string")
	}
	requiredSet := make(map[string]bool)
	for _, r := range required {
		requiredSet[r] = true
	}
	for _, name := range []string{"product_id", "product_name", "category", "price"} {
		if !requiredSet[name] {
			t.Errorf("required missing %q", name)
		}
	}
	if requiredSet["currency"] {
		t.Error("currency should not be in required (it is optional)")
	}
}

func TestBuildJSONSchema_enumConstraint(t *testing.T) {
	def := validProductViewedDef()
	schema := BuildJSONSchema(def)
	props := schema["properties"].(map[string]any)
	categorySchema := props["category"].(map[string]any)
	enum, ok := categorySchema["enum"].([]string)
	if !ok || len(enum) != 6 {
		t.Errorf("category enum = %v, want 6 values", categorySchema["enum"])
	}
}

func TestBuildJSONSchema_patternConstraint(t *testing.T) {
	def := validProductViewedDef()
	schema := BuildJSONSchema(def)
	props := schema["properties"].(map[string]any)
	currencySchema := props["currency"].(map[string]any)
	if currencySchema["pattern"] != "^[A-Z]{3}$" {
		t.Errorf("currency pattern = %v, want %q", currencySchema["pattern"], "^[A-Z]{3}$")
	}
}

// --- ValidateEventPayload ---

func TestValidateEventPayload_valid(t *testing.T) {
	def := validProductViewedDef()
	props := map[string]any{
		"product_id":   "SKU-123",
		"product_name": "Blue Widget",
		"category":     "electronics",
		"price":        49.99,
		"currency":     "USD",
	}
	errs := ValidateEventPayload(def, props)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid payload, got: %v", errs)
	}
}

func TestValidateEventPayload_missingRequired(t *testing.T) {
	def := validProductViewedDef()
	props := map[string]any{
		"product_id": "SKU-123",
		// product_name, category, price missing
	}
	errs := ValidateEventPayload(def, props)
	if len(errs) == 0 {
		t.Error("expected errors for missing required properties, got none")
	}
}

func TestValidateEventPayload_wrongType(t *testing.T) {
	def := validProductViewedDef()
	props := map[string]any{
		"product_id":   "SKU-123",
		"product_name": "Blue Widget",
		"category":     "electronics",
		"price":        "not-a-number", // wrong type
	}
	errs := ValidateEventPayload(def, props)
	if len(errs) == 0 {
		t.Error("expected type error for price, got none")
	}
}

func TestValidateEventPayload_enumViolation(t *testing.T) {
	def := validProductViewedDef()
	props := map[string]any{
		"product_id":   "SKU-123",
		"product_name": "Blue Widget",
		"category":     "furniture", // not in enum
		"price":        49.99,
	}
	errs := ValidateEventPayload(def, props)
	if len(errs) == 0 {
		t.Error("expected enum error for invalid category, got none")
	}
}

func TestValidateEventPayload_patternViolation(t *testing.T) {
	def := validProductViewedDef()
	props := map[string]any{
		"product_id":   "SKU-123",
		"product_name": "Blue Widget",
		"category":     "electronics",
		"price":        49.99,
		"currency":     "usd", // lowercase, violates ^[A-Z]{3}$
	}
	errs := ValidateEventPayload(def, props)
	if len(errs) == 0 {
		t.Error("expected pattern error for lowercase currency, got none")
	}
}

func TestValidateEventPayload_optionalFieldAbsent(t *testing.T) {
	def := validProductViewedDef()
	props := map[string]any{
		"product_id":   "SKU-123",
		"product_name": "Blue Widget",
		"category":     "electronics",
		"price":        49.99,
		// currency absent — optional, should pass
	}
	errs := ValidateEventPayload(def, props)
	if len(errs) != 0 {
		t.Errorf("expected no errors when optional field is absent, got: %v", errs)
	}
}

// --- ValidateWorkspaceConfig ---

func TestValidateWorkspaceConfig_valid(t *testing.T) {
	cfg := &WorkspaceConfig{
		Version:   1,
		Workspace: "my-company",
		Registry:  RegistryConfig{Mode: RegistryModeLocal},
	}
	if errs := ValidateWorkspaceConfig(cfg, "event-spec.yaml"); len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateWorkspaceConfig_missingVersionAndWorkspace(t *testing.T) {
	cfg := &WorkspaceConfig{}
	errs := ValidateWorkspaceConfig(cfg, "event-spec.yaml")
	if !containsField(errs, "version") {
		t.Error("expected error for missing version")
	}
	if !containsField(errs, "workspace") {
		t.Error("expected error for missing workspace")
	}
}

func TestValidateWorkspaceConfig_invalidRegistryMode(t *testing.T) {
	cfg := &WorkspaceConfig{Version: 1, Workspace: "w", Registry: RegistryConfig{Mode: "database"}}
	if !containsField(ValidateWorkspaceConfig(cfg, "f"), "registry.mode") {
		t.Error("expected error for invalid registry.mode")
	}
}

func TestValidateWorkspaceConfig_gitModeRequiresRemote(t *testing.T) {
	cfg := &WorkspaceConfig{Version: 1, Workspace: "w", Registry: RegistryConfig{Mode: RegistryModeGit}}
	if !containsField(ValidateWorkspaceConfig(cfg, "f"), "registry.remote") {
		t.Error("expected error for git mode without remote")
	}
}

func TestValidateWorkspaceConfig_serverModeRequiresURL(t *testing.T) {
	cfg := &WorkspaceConfig{Version: 1, Workspace: "w", Registry: RegistryConfig{Mode: RegistryModeServer}}
	if !containsField(ValidateWorkspaceConfig(cfg, "f"), "registry.url") {
		t.Error("expected error for server mode without url")
	}
}

// --- ValidateSourceDef ---

func validSourceDef() *SourceDef {
	return &SourceDef{
		Name:       "web-app",
		Language:   "typescript",
		Events:     []string{"ecommerce/**"},
		Output:     SourceOutput{Path: "./src/analytics/generated"},
		SourcePath: "sources/web-app.yaml",
	}
}

func TestValidateSourceDef_valid(t *testing.T) {
	if errs := ValidateSourceDef(validSourceDef()); len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateSourceDef_missingNameAndLanguage(t *testing.T) {
	def := &SourceDef{Output: SourceOutput{Path: "./out"}, SourcePath: "f"}
	errs := ValidateSourceDef(def)
	if !containsField(errs, "name") {
		t.Error("expected error for missing name")
	}
	if !containsField(errs, "language") {
		t.Error("expected error for missing language")
	}
}

func TestValidateSourceDef_missingOutputPath(t *testing.T) {
	def := &SourceDef{Name: "a", Language: "go", SourcePath: "f"}
	if !containsField(ValidateSourceDef(def), "output.path") {
		t.Error("expected error for missing output.path")
	}
}

func TestValidateSourceDef_unknownLanguage(t *testing.T) {
	def := validSourceDef()
	def.Language = "cobol"
	if !containsField(ValidateSourceDef(def), "language") {
		t.Error("expected error for unknown language")
	}
}

func TestValidateSourceDef_invalidMode(t *testing.T) {
	def := validSourceDef()
	def.Mode = "direct"
	if !containsField(ValidateSourceDef(def), "mode") {
		t.Error("expected error for invalid mode")
	}
}

func TestValidateSourceDef_serverProxiedRequiresEndpoint(t *testing.T) {
	def := validSourceDef()
	def.Mode = "server_proxied"
	// RuntimeEndpoint not set
	if !containsField(ValidateSourceDef(def), "runtime_endpoint") {
		t.Error("expected error for server_proxied without runtime_endpoint")
	}
}

func TestValidateSourceDef_serverProxiedWithEndpoint(t *testing.T) {
	def := validSourceDef()
	def.Mode = "server_proxied"
	def.RuntimeEndpoint = "https://analytics.example.com/v1/track"
	if errs := ValidateSourceDef(def); len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// --- ValidateDestinationDef ---

func validDestinationDef() *DestinationDef {
	return &DestinationDef{
		Name:       "amplitude",
		Provider:   "amplitude",
		SourcePath: "destinations/amplitude.yaml",
	}
}

func TestValidateDestinationDef_valid(t *testing.T) {
	if errs := ValidateDestinationDef(validDestinationDef()); len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateDestinationDef_missingName(t *testing.T) {
	def := &DestinationDef{Provider: "amplitude", SourcePath: "f"}
	if !containsField(ValidateDestinationDef(def), "name") {
		t.Error("expected error for missing name")
	}
}

func TestValidateDestinationDef_missingProvider(t *testing.T) {
	def := &DestinationDef{Name: "amp", SourcePath: "f"}
	if !containsField(ValidateDestinationDef(def), "provider") {
		t.Error("expected error for missing provider")
	}
}

// containsField returns true if any ValidationError has the given Field value.
func containsField(errs []ValidationError, field string) bool {
	for _, e := range errs {
		if e.Field == field {
			return true
		}
	}
	return false
}
