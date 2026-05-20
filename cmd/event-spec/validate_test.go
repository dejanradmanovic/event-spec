package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validSpecYAML = `$schema: "https://event-spec.io/schemas/event/v1"
name: test_event
namespace: test
version: "1-0-0"
status: active
type: track
event_name: "Test Event"
properties:
  foo:
    type: string
    required: true
`

func writeSpec(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCmd_Valid(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "test_event.yaml", validSpecYAML)

	cmd := newValidateCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error, got: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "validated 1 event spec(s): ok") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

func TestValidateCmd_Invalid(t *testing.T) {
	dir := t.TempDir()
	// Missing required fields: version, namespace, status, type, event_name
	writeSpec(t, dir, "bad.yaml", `$schema: "https://event-spec.io/schemas/event/v1"
name: bad_event
`)

	cmd := newValidateCmd()
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{dir})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for invalid spec, got nil")
	}
	if !strings.Contains(stderr.String(), "error:") {
		t.Errorf("expected error lines in stderr, got: %s", stderr.String())
	}
}

func TestValidateCmd_ReportsAllViolations(t *testing.T) {
	dir := t.TempDir()
	// Two invalid specs — both should be reported.
	writeSpec(t, dir, "a.yaml", `$schema: "https://event-spec.io/schemas/event/v1"
name: a
`)
	writeSpec(t, dir, "b.yaml", `$schema: "https://event-spec.io/schemas/event/v1"
name: b
`)

	cmd := newValidateCmd()
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{dir})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error")
	}
	// Each missing field produces an error line; we should see errors from both files.
	if !strings.Contains(stderr.String(), "a.yaml") || !strings.Contains(stderr.String(), "b.yaml") {
		t.Errorf("expected errors from both files, got: %s", stderr.String())
	}
}

func TestValidateCmd_NonStrictIgnoresDeprecated(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "old.yaml", `$schema: "https://event-spec.io/schemas/event/v1"
name: old_event
namespace: test
version: "1-0-0"
status: deprecated
type: track
event_name: "Old Event"
`)

	cmd := newValidateCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("non-strict mode should not fail on deprecated events, got: %v", err)
	}
}

func TestValidateCmd_StrictFailsOnDeprecated(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "old.yaml", `$schema: "https://event-spec.io/schemas/event/v1"
name: old_event
namespace: test
version: "1-0-0"
status: deprecated
type: track
event_name: "Old Event"
`)

	cmd := newValidateCmd()
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--strict", dir})

	if err := cmd.Execute(); err == nil {
		t.Fatal("strict mode should fail on deprecated events")
	}
	if !strings.Contains(stderr.String(), "warning:") {
		t.Errorf("expected warning line in stderr, got: %s", stderr.String())
	}
}

func TestValidateCmd_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	cmd := newValidateCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("empty dir should succeed with 0 events, got: %v", err)
	}
	if !strings.Contains(stdout.String(), "validated 0 event spec(s): ok") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}
