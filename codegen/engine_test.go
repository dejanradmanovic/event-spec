package codegen_test

import (
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dejanradmanovic/event-spec/codegen"
	_ "github.com/dejanradmanovic/event-spec/codegen/golang"
	_ "github.com/dejanradmanovic/event-spec/codegen/kotlin"
	_ "github.com/dejanradmanovic/event-spec/codegen/swift"
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

func TestGenerate_Kotlin(t *testing.T) {
	events := testEvents()
	outDir := t.TempDir()
	if err := codegen.Run(events, "kotlin", outDir, "test-workspace", "test-source", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	compareOrUpdate(t, outDir, filepath.Join("testdata", "golden", "kotlin"))
}

func TestGenerate_Kotlin_StatusAware(t *testing.T) {
	events := testStatusEvents()
	outDir := t.TempDir()
	if err := codegen.Run(events, "kotlin", outDir, "test-workspace", "test-source", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	compareOrUpdate(t, outDir, filepath.Join("testdata", "golden", "kotlin_status"))
}

func TestGenerate_DeletedEventAbsent_Kotlin(t *testing.T) {
	events := []*spec.EventDef{testDeletedEvent()}
	outDir := t.TempDir()
	if err := codegen.Run(events, "kotlin", outDir, "", "", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "LegacyPageview.kt")); !os.IsNotExist(err) {
		t.Error("deleted event should produce no Kotlin file")
	}
}

func TestGenerate_Swift(t *testing.T) {
	events := testEvents()
	outDir := t.TempDir()
	if err := codegen.Run(events, "swift", outDir, "test-workspace", "test-source", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	compareOrUpdate(t, outDir, filepath.Join("testdata", "golden", "swift"))
}

func TestGenerate_Swift_StatusAware(t *testing.T) {
	events := testStatusEvents()
	outDir := t.TempDir()
	if err := codegen.Run(events, "swift", outDir, "test-workspace", "test-source", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	compareOrUpdate(t, outDir, filepath.Join("testdata", "golden", "swift_status"))
}

func TestGenerate_NoPropsEventSwiftHasFile(t *testing.T) {
	events := []*spec.EventDef{testSessionStartedEvent()}
	outDir := t.TempDir()
	if err := codegen.Run(events, "swift", outDir, "", "", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "SessionStarted.swift")); err != nil {
		t.Errorf("expected SessionStarted.swift to be generated: %v", err)
	}
}

func TestGenerate_DeletedEventAbsent_Swift(t *testing.T) {
	events := []*spec.EventDef{testDeletedEvent()}
	outDir := t.TempDir()
	if err := codegen.Run(events, "swift", outDir, "", "", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "LegacyPageview.swift")); !os.IsNotExist(err) {
		t.Error("deleted event should produce no Swift file")
	}
}

func TestLookupLang_Swift(t *testing.T) {
	lc, err := codegen.LookupLang("swift")
	if err != nil {
		t.Fatalf("LookupLang: %v", err)
	}
	if lc.ID != "swift" {
		t.Errorf("ID = %q, want %q", lc.ID, "swift")
	}
	if lc.FileExt != ".swift" {
		t.Errorf("FileExt = %q, want %q", lc.FileExt, ".swift")
	}
}

func TestGenerate_UnsupportedLang(t *testing.T) {
	if err := codegen.Run(nil, "cobol", t.TempDir(), "", "", ""); err == nil {
		t.Error("expected error for unsupported language")
	}
}

// An unsupported language must name the ones that are available. Telling a user
// "cobol" is unsupported without saying what is supported makes them go read the
// registry, and the CLI's --lang help can drift from reality.
func TestLookupLang_UnsupportedNamesSupported(t *testing.T) {
	_, err := codegen.LookupLang("cobol")
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"cobol"`) {
		t.Errorf("error should quote the rejected id, got: %s", msg)
	}
	for _, want := range codegen.RegisteredLangs() {
		if !strings.Contains(msg, want) {
			t.Errorf("error should list supported language %q, got: %s", want, msg)
		}
	}
}

func TestRegisteredLangs_SortedAndComplete(t *testing.T) {
	got := codegen.RegisteredLangs()
	for _, want := range []string{"go", "kotlin", "swift", "typescript"} {
		if !slices.Contains(got, want) {
			t.Errorf("RegisteredLangs() missing %q: %v", want, got)
		}
	}
	if !slices.IsSorted(got) {
		t.Errorf("RegisteredLangs() not sorted: %v", got)
	}
}

func TestGenerate_Go_StatusAware(t *testing.T) {
	events := testStatusEvents()
	outDir := t.TempDir()
	if err := codegen.Run(events, "go", outDir, "test-workspace", "test-source", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	compareOrUpdate(t, outDir, filepath.Join("testdata", "golden", "go_status"))
}

func TestGenerate_TypeScript_StatusAware(t *testing.T) {
	events := testStatusEvents()
	outDir := t.TempDir()
	if err := codegen.Run(events, "typescript", outDir, "test-workspace", "test-source", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	compareOrUpdate(t, outDir, filepath.Join("testdata", "golden", "typescript_status"))
}

func TestGenerate_DeletedEventAbsent_Go(t *testing.T) {
	events := []*spec.EventDef{testDeletedEvent()}
	outDir := t.TempDir()
	if err := codegen.Run(events, "go", outDir, "", "", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "legacy_pageview.go")); !os.IsNotExist(err) {
		t.Error("deleted event should produce no Go file")
	}
}

func TestGenerate_DeletedEventAbsent_TypeScript(t *testing.T) {
	events := []*spec.EventDef{testDeletedEvent()}
	outDir := t.TempDir()
	if err := codegen.Run(events, "typescript", outDir, "", "", ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "legacy_pageview.ts")); !os.IsNotExist(err) {
		t.Error("deleted event should produce no TypeScript file")
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
	golden := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		golden[e.Name()] = true
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

	// Walking only goldenDir would silently ignore a generated file that has no
	// golden counterpart, which is how go_status/eventspec.go went unverified and
	// reappeared as an untracked file after every `-update`. Assert the sets match.
	gotEntries, err := os.ReadDir(gotDir)
	if err != nil {
		t.Fatalf("read outDir: %v", err)
	}
	for _, e := range gotEntries {
		if e.IsDir() {
			continue
		}
		if !golden[e.Name()] {
			t.Errorf("generated file %s has no golden in %s; run `go test ./codegen -update` and commit it",
				e.Name(), goldenDir)
		}
	}
}

func testEvents() []*spec.EventDef {
	return []*spec.EventDef{
		testProductViewedEvent(),
		testSessionStartedEvent(),
	}
}

func testStatusEvents() []*spec.EventDef {
	return []*spec.EventDef{
		testDeprecatedEvent(),
		testDraftEvent(),
		testDeletedEvent(),
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

func testDeprecatedEvent() *spec.EventDef {
	return &spec.EventDef{
		Schema:      "https://event-spec.io/schemas/event/v1",
		Name:        "checkout_abandoned",
		DisplayName: "Checkout Abandoned",
		EventName:   "Checkout Abandoned",
		Version:     "2-0-0",
		Status:      spec.StatusDeprecated,
		Description: "Use checkout_started instead.",
		Namespace:   "ecommerce",
		Type:        spec.TypeTrack,
		Properties: map[string]spec.PropertyDef{
			"cart_id": {Type: spec.PropertyTypeString, Required: true},
		},
	}
}

func testDraftEvent() *spec.EventDef {
	return &spec.EventDef{
		Schema:      "https://event-spec.io/schemas/event/v1",
		Name:        "wishlist_shared",
		DisplayName: "Wishlist Shared",
		EventName:   "Wishlist Shared",
		Version:     "1-0-0",
		Status:      spec.StatusDraft,
		Namespace:   "ecommerce",
		Type:        spec.TypeTrack,
		Properties: map[string]spec.PropertyDef{
			"wishlist_id": {Type: spec.PropertyTypeString, Required: true},
		},
	}
}

func testDeletedEvent() *spec.EventDef {
	return &spec.EventDef{
		Schema:      "https://event-spec.io/schemas/event/v1",
		Name:        "legacy_pageview",
		DisplayName: "Legacy Pageview",
		EventName:   "Legacy Pageview",
		Version:     "1-0-0",
		Status:      spec.StatusDeleted,
		Namespace:   "core",
		Type:        spec.TypePage,
		Properties:  map[string]spec.PropertyDef{},
	}
}
