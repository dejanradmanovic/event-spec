package audit_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dejanradmanovic/event-spec/codegen/audit"
	"github.com/dejanradmanovic/event-spec/spec"
)

// ─── Matcher tests ────────────────────────────────────────────────────────────

func TestGoMatcher_FindUsages(t *testing.T) {
	src := []byte(`package main

import (
	"context"
	analytics "github.com/example/analytics"
)

func main() {
	es.ProductViewed(ctx, analytics.ProductViewedProperties{})
	es.CheckoutStarted(ctx, analytics.CheckoutStartedProperties{})
	client.Track(ctx, analytics.Event{Name: "legacy_event"})
}
`)
	m := audit.NewGoMatcher()
	methods, rawTracks, err := m.FindUsages("main.go", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := methods["ProductViewed"]; !ok {
		t.Error("expected ProductViewed in methods")
	}
	if _, ok := methods["CheckoutStarted"]; !ok {
		t.Error("expected CheckoutStarted in methods")
	}
	if _, ok := rawTracks["legacy_event"]; !ok {
		t.Error("expected legacy_event in rawTracks")
	}
}

func TestTypeScriptMatcher_FindUsages(t *testing.T) {
	src := []byte(`
import { EventSpec } from './generated';

const es = new EventSpec(client);
es.productViewed({ productId: '123' });
es.checkoutStarted({ cartValue: 99 });
client.track({ name: 'rogueEvent', properties: {} });
// es.commented();
`)
	m := audit.NewTypeScriptMatcher()
	methods, rawTracks, err := m.FindUsages("app.ts", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := methods["productViewed"]; !ok {
		t.Error("expected productViewed in methods")
	}
	if _, ok := methods["checkoutStarted"]; !ok {
		t.Error("expected checkoutStarted in methods")
	}
	if _, ok := rawTracks["rogueEvent"]; !ok {
		t.Error("expected rogueEvent in rawTracks")
	}
	if _, ok := methods["commented"]; ok {
		t.Error("should not find commented-out calls")
	}
}

func TestSwiftMatcher_FindUsages(t *testing.T) {
	src := []byte(`
import EventSpec

let es = EventSpec(client: client)
es.productViewed(ProductViewedProperties(productId: "123"))
es.checkoutStarted(CheckoutStartedProperties())
client.track(event: "rawEvent")
`)
	m := audit.NewSwiftMatcher()
	methods, rawTracks, err := m.FindUsages("App.swift", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := methods["productViewed"]; !ok {
		t.Error("expected productViewed in methods")
	}
	if _, ok := methods["checkoutStarted"]; !ok {
		t.Error("expected checkoutStarted in methods")
	}
	if _, ok := rawTracks["rawEvent"]; !ok {
		t.Error("expected rawEvent in rawTracks")
	}
}

// ─── Scanner tests ────────────────────────────────────────────────────────────

func TestNewScanner_UnsupportedLanguage(t *testing.T) {
	_, err := audit.NewScanner("cobol")
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

func TestScanner_ScanDir_Go(t *testing.T) {
	dir := t.TempDir()

	content := `package analytics

func use(es EventSpec) {
	es.ProductViewed(nil)
	es.OrderCompleted(nil)
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	scanner, err := audit.NewScanner("go")
	if err != nil {
		t.Fatal(err)
	}
	result, err := scanner.ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ScannedFiles != 1 {
		t.Errorf("expected 1 scanned file, got %d", result.ScannedFiles)
	}
	if _, ok := result.MethodCalls["ProductViewed"]; !ok {
		t.Error("expected ProductViewed in MethodCalls")
	}
	if _, ok := result.MethodCalls["OrderCompleted"]; !ok {
		t.Error("expected OrderCompleted in MethodCalls")
	}
}

func TestScanner_SkipsVendorDir(t *testing.T) {
	dir := t.TempDir()

	// Create a vendor directory with a Go file.
	vendorDir := filepath.Join(dir, "vendor", "pkg")
	if err := os.MkdirAll(vendorDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "file.go"), []byte(`package pkg
func f(es ES) { es.ShouldNotBeSeen(nil) }
`), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a file in the root.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main
func main() { es.ProductViewed(nil) }
`), 0600); err != nil {
		t.Fatal(err)
	}

	scanner, _ := audit.NewScanner("go")
	result, err := scanner.ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := result.MethodCalls["ShouldNotBeSeen"]; ok {
		t.Error("vendor directory should be skipped")
	}
	if _, ok := result.MethodCalls["ProductViewed"]; !ok {
		t.Error("root file should be scanned")
	}
}

// ─── Coverage / Report tests ─────────────────────────────────────────────────

func TestBuildReport_UsedUnusedRogue(t *testing.T) {
	defs := []*spec.EventDef{
		{Name: "product_viewed", Namespace: "ecommerce", Version: "1-0-0", SourcePath: "specs/ecommerce/product_viewed/1-0-0.yaml"},
		{Name: "order_completed", Namespace: "ecommerce", Version: "1-0-0", SourcePath: "specs/ecommerce/order_completed/1-0-0.yaml", Required: true},
	}

	result := &audit.ScanResult{
		ScannedFiles: 5,
		MethodCalls: map[string][]audit.Location{
			"ProductViewed": {{File: "app.go", Line: 10}},
			"RogueMethod":   {{File: "app.go", Line: 20}},
		},
		RawTrackCalls: map[string][]audit.Location{},
	}

	report := audit.BuildReport("test-source", "go", defs, result)

	if len(report.Used) != 1 || report.Used[0].EventKey != "ecommerce/product_viewed" {
		t.Errorf("expected 1 used event, got %+v", report.Used)
	}
	if len(report.Unused) != 1 || report.Unused[0].EventKey != "ecommerce/order_completed" {
		t.Errorf("expected 1 unused event, got %+v", report.Unused)
	}
	if !report.Unused[0].Required {
		t.Error("order_completed should be marked required")
	}
	if len(report.Rogue) != 1 || report.Rogue[0].EventName != "RogueMethod" {
		t.Errorf("expected 1 rogue event, got %+v", report.Rogue)
	}

	expected := 50.0
	if report.CoveragePct != expected {
		t.Errorf("expected %.1f%% coverage, got %.1f%%", expected, report.CoveragePct)
	}
}

func TestBuildReport_CoverageFullyUsed(t *testing.T) {
	defs := []*spec.EventDef{
		{Name: "page_viewed", Namespace: "app", Version: "1-0-0"},
	}
	result := &audit.ScanResult{
		ScannedFiles:  3,
		MethodCalls:   map[string][]audit.Location{"PageViewed": {{File: "x.go", Line: 1}}},
		RawTrackCalls: map[string][]audit.Location{},
	}
	report := audit.BuildReport("src", "go", defs, result)

	if report.CoveragePct != 100.0 {
		t.Errorf("expected 100%% coverage, got %.1f%%", report.CoveragePct)
	}
	if len(report.Unused) != 0 {
		t.Errorf("expected no unused events")
	}
}

func TestReport_WriteText(t *testing.T) {
	defs := []*spec.EventDef{
		{Name: "product_viewed", Namespace: "ecommerce", Version: "1-0-0"},
	}
	result := &audit.ScanResult{
		ScannedFiles:  2,
		MethodCalls:   map[string][]audit.Location{"ProductViewed": {{File: "app.go", Line: 5}}},
		RawTrackCalls: map[string][]audit.Location{},
	}
	report := audit.BuildReport("my-app", "go", defs, result)

	var buf bytes.Buffer
	report.WriteText(&buf)
	out := buf.String()

	if !strings.Contains(out, "my-app") {
		t.Error("text report should contain source name")
	}
	if !strings.Contains(out, "ecommerce/product_viewed") {
		t.Error("text report should contain event key")
	}
	if !strings.Contains(out, "100.0%") {
		t.Error("text report should show 100% coverage")
	}
}

func TestReport_WriteJSON(t *testing.T) {
	defs := []*spec.EventDef{
		{Name: "product_viewed", Namespace: "ecommerce", Version: "1-0-0"},
	}
	result := &audit.ScanResult{
		ScannedFiles:  1,
		MethodCalls:   map[string][]audit.Location{},
		RawTrackCalls: map[string][]audit.Location{},
	}
	report := audit.BuildReport("my-app", "go", defs, result)

	var buf bytes.Buffer
	if err := report.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, `"source"`) {
		t.Error("JSON report should have source field")
	}
	if !strings.Contains(out, `"coverage_pct"`) {
		t.Error("JSON report should have coverage_pct field")
	}
}

func TestReport_WriteHTML(t *testing.T) {
	defs := []*spec.EventDef{
		{Name: "product_viewed", Namespace: "ecommerce", Version: "1-0-0"},
	}
	result := &audit.ScanResult{
		ScannedFiles:  1,
		MethodCalls:   map[string][]audit.Location{"ProductViewed": {{File: "app.go", Line: 1}}},
		RawTrackCalls: map[string][]audit.Location{},
	}
	report := audit.BuildReport("my-app", "go", defs, result)

	var buf bytes.Buffer
	report.WriteHTML(&buf)
	out := buf.String()

	if !strings.Contains(out, "<!DOCTYPE html>") {
		t.Error("HTML report should be a valid HTML document")
	}
	if !strings.Contains(out, "my-app") {
		t.Error("HTML report should contain source name")
	}
}
