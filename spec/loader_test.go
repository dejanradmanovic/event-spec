package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEventDef_valid(t *testing.T) {
	path := filepath.Join("testdata", "specs", "ecommerce", "product_viewed", "1-2-0.yaml")
	def, err := LoadEventDef(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.Name != "product_viewed" {
		t.Errorf("Name = %q, want %q", def.Name, "product_viewed")
	}
	if def.Version != "1-2-0" {
		t.Errorf("Version = %q, want %q", def.Version, "1-2-0")
	}
	if def.Namespace != "ecommerce" {
		t.Errorf("Namespace = %q, want %q", def.Namespace, "ecommerce")
	}
	if def.Status != StatusActive {
		t.Errorf("Status = %q, want %q", def.Status, StatusActive)
	}
	if def.Type != TypeTrack {
		t.Errorf("Type = %q, want %q", def.Type, TypeTrack)
	}
	if def.EventName != "Product Viewed" {
		t.Errorf("EventName = %q, want %q", def.EventName, "Product Viewed")
	}
	if def.SourcePath != path {
		t.Errorf("SourcePath = %q, want %q", def.SourcePath, path)
	}
	if len(def.Properties) != 6 {
		t.Errorf("len(Properties) = %d, want 6", len(def.Properties))
	}

	productID, ok := def.Properties["product_id"]
	if !ok {
		t.Fatal("Properties missing product_id")
	}
	if productID.Type != PropertyTypeString {
		t.Errorf("product_id.Type = %q, want %q", productID.Type, PropertyTypeString)
	}
	if !productID.Required {
		t.Error("product_id.Required = false, want true")
	}

	category := def.Properties["category"]
	if len(category.Enum) != 6 {
		t.Errorf("category.Enum len = %d, want 6", len(category.Enum))
	}

	price := def.Properties["price"]
	if price.Minimum == nil || *price.Minimum != 0 {
		t.Errorf("price.Minimum = %v, want 0", price.Minimum)
	}

	if def.Sampling == nil {
		t.Fatal("Sampling is nil")
	}
	if def.Sampling.Strategy != SamplingUserIDHash {
		t.Errorf("Sampling.Strategy = %q, want %q", def.Sampling.Strategy, SamplingUserIDHash)
	}
	if def.Sampling.Rate != 0.1 {
		t.Errorf("Sampling.Rate = %g, want 0.1", def.Sampling.Rate)
	}
	if def.PropertyPriority != PriorityEventWins {
		t.Errorf("PropertyPriority = %q, want %q", def.PropertyPriority, PriorityEventWins)
	}

	ga4, ok := def.ProviderOverrides["ga4"]
	if !ok {
		t.Fatal("ProviderOverrides missing ga4")
	}
	if ga4.EventName != "view_item" {
		t.Errorf("ga4.EventName = %q, want %q", ga4.EventName, "view_item")
	}
	if ga4.PropertyMap["product_id"] != "item_id" {
		t.Errorf("ga4.PropertyMap[product_id] = %q, want %q", ga4.PropertyMap["product_id"], "item_id")
	}
}

func TestLoadEventDef_notFound(t *testing.T) {
	_, err := LoadEventDef("testdata/nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadEventDef_invalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("name: [\ninvalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadEventDef(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadSourceDef(t *testing.T) {
	def, err := LoadSourceDef(filepath.Join("testdata", "sources", "web-app.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.Name != "web-app" {
		t.Errorf("Name = %q, want %q", def.Name, "web-app")
	}
	if def.Language != "typescript" {
		t.Errorf("Language = %q, want %q", def.Language, "typescript")
	}
	if len(def.Events) != 2 {
		t.Errorf("len(Events) = %d, want 2", len(def.Events))
	}
	if def.VersionPinning["ecommerce/product_viewed"] != "1-2-0" {
		t.Errorf("VersionPinning[ecommerce/product_viewed] = %q, want %q",
			def.VersionPinning["ecommerce/product_viewed"], "1-2-0")
	}
}

func TestLoadDestinationDef(t *testing.T) {
	def, err := LoadDestinationDef(filepath.Join("testdata", "destinations", "amplitude.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.Name != "amplitude" {
		t.Errorf("Name = %q, want %q", def.Name, "amplitude")
	}
	if def.Provider != "amplitude" {
		t.Errorf("Provider = %q, want %q", def.Provider, "amplitude")
	}
	if def.Config["secret_type"] != "env_var" {
		t.Errorf("Config[secret_type] = %v, want %q", def.Config["secret_type"], "env_var")
	}
}

func TestWalkEventDefs(t *testing.T) {
	defs, errs := WalkEventDefs(filepath.Join("testdata", "specs"))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(defs) != 1 {
		t.Fatalf("len(defs) = %d, want 1", len(defs))
	}
	if defs[0].Name != "product_viewed" {
		t.Errorf("defs[0].Name = %q, want %q", defs[0].Name, "product_viewed")
	}
}

func TestWalkEventDefs_skipsNonSpecFiles(t *testing.T) {
	// Sources and destinations don't have $schema — WalkEventDefs should skip them.
	defs, errs := WalkEventDefs(filepath.Join("testdata", "sources"))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(defs) != 0 {
		t.Errorf("expected 0 defs from sources dir, got %d", len(defs))
	}
}

func TestParseSchemaVer(t *testing.T) {
	tests := []struct {
		input   string
		major   int
		minor   int
		patch   int
		wantErr bool
	}{
		{"1-2-0", 1, 2, 0, false},
		{"0-0-0", 0, 0, 0, false},
		{"10-20-30", 10, 20, 30, false},
		{"1-2", 0, 0, 0, true},
		{"1-2-0-extra", 0, 0, 0, true},
		{"a-b-c", 0, 0, 0, true},
		{"1.2.0", 0, 0, 0, true},
		{"", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			sv, err := ParseSchemaVer(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSchemaVer(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr {
				if sv.Major != tt.major || sv.Minor != tt.minor || sv.Patch != tt.patch {
					t.Errorf("got %d-%d-%d, want %d-%d-%d", sv.Major, sv.Minor, sv.Patch, tt.major, tt.minor, tt.patch)
				}
				if sv.Raw != tt.input {
					t.Errorf("Raw = %q, want %q", sv.Raw, tt.input)
				}
			}
		})
	}
}

func TestCompareSchemaVer(t *testing.T) {
	parse := func(s string) SchemaVer {
		sv, err := ParseSchemaVer(s)
		if err != nil {
			t.Fatalf("ParseSchemaVer(%q): %v", s, err)
		}
		return sv
	}

	tests := []struct {
		a, b string
		want int
	}{
		{"1-0-0", "1-0-0", 0},
		{"2-0-0", "1-0-0", 1},
		{"1-0-0", "2-0-0", -1},
		{"1-2-0", "1-1-0", 1},
		{"1-0-0", "1-0-1", -1},
		{"1-2-3", "1-2-3", 0},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := CompareSchemaVer(parse(tt.a), parse(tt.b))
			if got != tt.want {
				t.Errorf("CompareSchemaVer(%s, %s) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}