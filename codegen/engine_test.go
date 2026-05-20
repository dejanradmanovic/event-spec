package codegen_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/dejanradmanovic/event-spec/codegen"
	_ "github.com/dejanradmanovic/event-spec/codegen/golang"
	_ "github.com/dejanradmanovic/event-spec/codegen/typescript"
	"github.com/dejanradmanovic/event-spec/spec"
)

var update = flag.Bool("update", false, "update golden files instead of comparing")

func TestGenerate_Go(t *testing.T) {
	events := testEvents()
	outDir := t.TempDir()
	if err := codegen.Run(events, "go", outDir, "test-workspace", "test-source", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	compareOrUpdate(t, outDir, filepath.Join("testdata", "golden", "go"))
}

func TestGenerate_TypeScript(t *testing.T) {
	events := testEvents()
	outDir := t.TempDir()
	if err := codegen.Run(events, "typescript", outDir, "test-workspace", "test-source", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	compareOrUpdate(t, outDir, filepath.Join("testdata", "golden", "typescript"))
}

func TestGenerate_NoPropsEventGoHasFile(t *testing.T) {
	events := []*spec.EventDef{testSessionStartedEvent()}
	outDir := t.TempDir()
	if err := codegen.Run(events, "go", outDir, "", "", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "session_started.go")); err != nil {
		t.Errorf("expected session_started.go to be generated: %v", err)
	}
}

func TestGenerate_NoPropsEventTSHasFile(t *testing.T) {
	events := []*spec.EventDef{testSessionStartedEvent()}
	outDir := t.TempDir()
	if err := codegen.Run(events, "typescript", outDir, "", "", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "session_started.ts")); err != nil {
		t.Errorf("expected session_started.ts to be generated: %v", err)
	}
}

func TestGenerate_UnsupportedLang(t *testing.T) {
	if err := codegen.Run(nil, "cobol", t.TempDir(), "", "", ""); err == nil {
		t.Error("expected error for unsupported language")
	}
}

// compareOrUpdate either writes generated files over golden files (when -update),
// or asserts that every golden file matches the corresponding generated file.
func compareOrUpdate(t *testing.T, gotDir, goldenDir string) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatalf("mkdir golden: %v", err)
		}
		entries, err := os.ReadDir(gotDir)
		if err != nil {
			t.Fatalf("read outDir: %v", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(gotDir, e.Name()))
			if err != nil {
				t.Fatalf("read %s: %v", e.Name(), err)
			}
			if err := os.WriteFile(filepath.Join(goldenDir, e.Name()), data, 0o644); err != nil {
				t.Fatalf("write golden %s: %v", e.Name(), err)
			}
		}
		return
	}

	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("read goldenDir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		want, err := os.ReadFile(filepath.Join(goldenDir, e.Name()))
		if err != nil {
			t.Fatalf("read golden %s: %v", e.Name(), err)
		}
		got, err := os.ReadFile(filepath.Join(gotDir, e.Name()))
		if err != nil {
			t.Fatalf("read generated %s: %v", e.Name(), err)
		}
		if string(got) != string(want) {
			t.Errorf("file %s does not match golden:\n--- want ---\n%s\n--- got ---\n%s", e.Name(), want, got)
		}
	}
}

func testEvents() []*spec.EventDef {
	return []*spec.EventDef{
		testProductViewedEvent(),
		testSessionStartedEvent(),
	}
}

func testProductViewedEvent() *spec.EventDef {
	return &spec.EventDef{
		Schema:      "https://event-spec.io/schemas/event/v1",
		Name:        "product_viewed",
		DisplayName: "Product Viewed",
		EventName:   "Product Viewed",
		Version:     "1-0-0",
		Status:      spec.StatusActive,
		Namespace:   "ecommerce",
		Type:        spec.TypeTrack,
		Properties: map[string]spec.PropertyDef{
			"product_id": {Type: spec.PropertyTypeString, Required: true},
			"category": {
				Type:     spec.PropertyTypeString,
				Required: true,
				Enum:     []string{"clothing", "electronics", "other"},
			},
			"currency": {Type: spec.PropertyTypeString, Required: false},
		},
	}
}

func testSessionStartedEvent() *spec.EventDef {
	return &spec.EventDef{
		Schema:      "https://event-spec.io/schemas/event/v1",
		Name:        "session_started",
		DisplayName: "Session Started",
		EventName:   "Session Started",
		Version:     "1-0-0",
		Status:      spec.StatusActive,
		Namespace:   "core",
		Type:        spec.TypeTrack,
		Properties:  map[string]spec.PropertyDef{},
	}
}
