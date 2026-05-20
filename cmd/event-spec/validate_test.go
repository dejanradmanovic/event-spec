package main

import (
	"bytes"
	"fmt"
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

// TestValidateCmd_ExplicitDir — spec-dir passed as positional arg (no event-spec.yaml needed).
func TestValidateCmd_ExplicitDir(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "test_event.yaml", validSpecYAML)

	cmd := newValidateCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "validated 1 event spec(s): ok") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

// TestValidateCmd_FromWorkspace — no positional arg; specs_dir read from event-spec.yaml.
func TestValidateCmd_FromWorkspace(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, specsDir, "test_event.yaml", validSpecYAML)
	writeWorkspaceConfig(t, root, "specs")

	withWorkDir(t, root)

	cmd := newValidateCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "validated 1 event spec(s): ok") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

// TestValidateCmd_NoWorkspaceDefaultsToSpecsDir — no event-spec.yaml; falls back to ./specs.
func TestValidateCmd_NoWorkspaceDefaultsToSpecsDir(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, specsDir, "test_event.yaml", validSpecYAML)

	withWorkDir(t, root) // no event-spec.yaml present

	cmd := newValidateCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "validated 1 event spec(s): ok") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

// TestValidateCmd_Invalid — invalid spec reports error lines and exits non-zero.
func TestValidateCmd_Invalid(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "bad.yaml", `$schema: "https://event-spec.io/schemas/event/v1"
name: bad_event
`)

	cmd := newValidateCmd()
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{dir})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for invalid spec")
	}
	if !strings.Contains(stderr.String(), "error:") {
		t.Errorf("expected error lines in stderr, got: %s", stderr.String())
	}
}

// TestValidateCmd_ReportsAllViolations — errors from every spec file are reported before exiting.
func TestValidateCmd_ReportsAllViolations(t *testing.T) {
	dir := t.TempDir()
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
	if !strings.Contains(stderr.String(), "a.yaml") || !strings.Contains(stderr.String(), "b.yaml") {
		t.Errorf("expected errors from both files, got: %s", stderr.String())
	}
}

// TestValidateCmd_NonStrictIgnoresDeprecated — deprecated events are warnings, not errors.
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
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("non-strict mode should not fail on deprecated events, got: %v", err)
	}
}

// TestValidateCmd_StrictFailsOnDeprecated — --strict escalates deprecated warnings to failure.
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

// TestValidateCmd_EmptyDir — zero specs is a valid outcome, not an error.
func TestValidateCmd_EmptyDir(t *testing.T) {
	cmd := newValidateCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{t.TempDir()})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("empty dir should succeed with 0 events, got: %v", err)
	}
	if !strings.Contains(stdout.String(), "validated 0 event spec(s): ok") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

// --- Registry mode tests ---

// TestValidateCmd_GitMode_CacheNotFound — git mode with no local cache should
// tell the user to run 'event-spec pull' first.
func TestValidateCmd_GitMode_CacheNotFound(t *testing.T) {
	root := t.TempDir()
	content := fmt.Sprintf(`version: 1
workspace: test-workspace
registry:
  mode: git
  remote: https://github.com/example/tracking-plan.git
  cache_dir: %s
  specs_dir: specs
sources_dir: sources
destinations_dir: destinations
`, filepath.Join(root, "nonexistent-cache"))
	if err := os.WriteFile(filepath.Join(root, "event-spec.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	withWorkDir(t, root)

	cmd := newValidateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when git cache is absent")
	}
	if !strings.Contains(err.Error(), "pull") {
		t.Errorf("error should mention 'pull', got: %v", err)
	}
}

// TestValidateCmd_ServerMode_NotImplemented — server mode should return a clear error.
func TestValidateCmd_ServerMode_NotImplemented(t *testing.T) {
	root := t.TempDir()
	content := `version: 1
workspace: test-workspace
registry:
  mode: server
  url: https://registry.example.com
sources_dir: sources
destinations_dir: destinations
`
	if err := os.WriteFile(filepath.Join(root, "event-spec.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	withWorkDir(t, root)

	cmd := newValidateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for server mode")
	}
	if !strings.Contains(err.Error(), "server") {
		t.Errorf("error should mention 'server', got: %v", err)
	}
}

// TestValidateCmd_GitMode_ExplicitDirBypasses — explicit spec-dir arg bypasses
// registry mode; useful when the cache is unavailable but a local copy exists.
func TestValidateCmd_GitMode_ExplicitDirBypasses(t *testing.T) {
	root := t.TempDir()
	// Write git-mode workspace config (cache doesn't exist).
	content := fmt.Sprintf(`version: 1
workspace: test-workspace
registry:
  mode: git
  remote: https://github.com/example/tracking-plan.git
  cache_dir: %s
  specs_dir: specs
sources_dir: sources
destinations_dir: destinations
`, filepath.Join(root, "nonexistent-cache"))
	if err := os.WriteFile(filepath.Join(root, "event-spec.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write a valid spec in a local directory.
	localDir := filepath.Join(root, "local-specs")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, localDir, "test_event.yaml", validSpecYAML)
	withWorkDir(t, root)

	// Passing the explicit dir should work even though git cache is absent.
	cmd := newValidateCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{localDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("explicit dir should bypass registry mode: %v", err)
	}
	if !strings.Contains(stdout.String(), "validated 1 event spec(s): ok") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}
