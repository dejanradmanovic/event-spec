package spec_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dejanradmanovic/event-spec/spec"
)

// TestScaffoldEventDef_ValidAndDefaultsApplied verifies that the scaffold output
// passes ValidateEventDef and that default values are applied correctly.
func TestScaffoldEventDef_ValidAndDefaultsApplied(t *testing.T) {
	content, err := spec.ScaffoldEventDef("ecommerce", "checkout_started", spec.ScaffoldOpts{
		Owner: "checkout-team@example.com",
	})
	if err != nil {
		t.Fatalf("ScaffoldEventDef: %v", err)
	}

	def, err := spec.ParseEventDefBytes(content, "scaffold")
	if err != nil {
		t.Fatalf("parse scaffold output: %v", err)
	}

	if errs := spec.ValidateEventDef(def); len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
	}

	if def.Namespace != "ecommerce" {
		t.Errorf("namespace = %q, want %q", def.Namespace, "ecommerce")
	}
	if def.Name != "checkout_started" {
		t.Errorf("name = %q, want %q", def.Name, "checkout_started")
	}
	if def.Version != "1-0-0" {
		t.Errorf("version = %q, want %q", def.Version, "1-0-0")
	}
	if def.Status != spec.StatusDraft {
		t.Errorf("status = %q, want %q", def.Status, spec.StatusDraft)
	}
	if def.Type != spec.TypeTrack {
		t.Errorf("type = %q, want %q", def.Type, spec.TypeTrack)
	}
	if def.DisplayName != "Checkout Started" {
		t.Errorf("display_name = %q, want %q", def.DisplayName, "Checkout Started")
	}
	if def.EventName != "Checkout Started" {
		t.Errorf("event_name = %q, want %q", def.EventName, "Checkout Started")
	}
	if def.Owner != "checkout-team@example.com" {
		t.Errorf("owner = %q", def.Owner)
	}
	if !strings.Contains(string(content), "event-spec.io/schemas/event/v1") {
		t.Error("YAML missing $schema header")
	}
}

// TestScaffoldEventDef_OptsReflected checks that non-default ScaffoldOpts are
// preserved in the generated YAML and still pass ValidateEventDef.
func TestScaffoldEventDef_OptsReflected(t *testing.T) {
	content, err := spec.ScaffoldEventDef("auth", "user_signed_up", spec.ScaffoldOpts{
		Type:        spec.TypePage,
		Status:      spec.StatusActive,
		Owner:       "auth-team@example.com",
		Description: "Fired when a user signs up.",
		DisplayName: "User Signed Up",
	})
	if err != nil {
		t.Fatalf("ScaffoldEventDef: %v", err)
	}

	def, err := spec.ParseEventDefBytes(content, "scaffold")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if errs := spec.ValidateEventDef(def); len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
	}

	if def.Type != spec.TypePage {
		t.Errorf("type = %q, want %q", def.Type, spec.TypePage)
	}
	if def.Status != spec.StatusActive {
		t.Errorf("status = %q, want %q", def.Status, spec.StatusActive)
	}
	if def.Description != "Fired when a user signs up." {
		t.Errorf("description = %q", def.Description)
	}
	if def.DisplayName != "User Signed Up" {
		t.Errorf("display_name = %q, want %q", def.DisplayName, "User Signed Up")
	}
}

// TestWriteScaffoldFile_ConflictDetected verifies that a second write to the same
// path returns an error without overwriting the existing file.
func TestWriteScaffoldFile_ConflictDetected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1-0-0.yaml")

	content, err := spec.ScaffoldEventDef("ecommerce", "checkout_started", spec.ScaffoldOpts{})
	if err != nil {
		t.Fatalf("ScaffoldEventDef: %v", err)
	}

	if err := spec.WriteScaffoldFile(path, content); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	if err := spec.WriteScaffoldFile(path, content); err == nil {
		t.Error("expected error on second write (conflict), got nil")
	}
}

// TestWriteScaffoldFile_CreatesDirectories verifies that intermediate directories
// are created automatically.
func TestWriteScaffoldFile_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ecommerce", "checkout_started", "1-0-0.yaml")

	content, err := spec.ScaffoldEventDef("ecommerce", "checkout_started", spec.ScaffoldOpts{})
	if err != nil {
		t.Fatalf("ScaffoldEventDef: %v", err)
	}

	if err := spec.WriteScaffoldFile(path, content); err != nil {
		t.Fatalf("WriteScaffoldFile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created at expected path: %v", err)
	}
}
