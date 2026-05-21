package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeGitWorkspaceConfig writes an event-spec.yaml with registry mode "git"
// and the given cache_dir into dir.
func writeGitWorkspaceConfig(t *testing.T, dir, cacheDir string) {
	t.Helper()
	content := fmt.Sprintf(`version: 1
workspace: test-workspace
registry:
  mode: git
  remote: https://github.com/example/tracking-plan.git
  cache_dir: %s
  specs_dir: specs
sources_dir: sources
destinations_dir: destinations
`, cacheDir)
	if err := os.WriteFile(filepath.Join(dir, "event-spec.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// withWorkDir changes the working directory to dir for the duration of the test
// and restores the original via t.Cleanup.
func withWorkDir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
}

// writeWorkspaceConfig writes a minimal event-spec.yaml into dir with the given specs_dir.
func writeWorkspaceConfig(t *testing.T, dir, specsDir string) {
	t.Helper()
	content := fmt.Sprintf(`version: 1
workspace: test-workspace
registry:
  mode: local
specs_dir: %s
sources_dir: sources
destinations_dir: destinations
`, specsDir)
	if err := os.WriteFile(filepath.Join(dir, "event-spec.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeSourceDef writes sources/<name>.yaml with the given language and output path.
func writeSourceDef(t *testing.T, sourcesDir, name, lang, outPath string) {
	t.Helper()
	if err := os.MkdirAll(sourcesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf(`name: %s
language: %s
events:
  - "**"
output:
  path: %s
`, name, lang, outPath)
	if err := os.WriteFile(filepath.Join(sourcesDir, name+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupWorkspaceRoot creates a temp root with event-spec.yaml, specs/, and sources/.
// It writes one valid spec and one source def, then returns the root path.
func setupWorkspaceRoot(t *testing.T, lang, outPath string) string {
	t.Helper()
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, specsDir, "test_event.yaml", validSpecYAML)
	writeWorkspaceConfig(t, root, "specs")
	writeSourceDef(t, filepath.Join(root, "sources"), "web-app", lang, outPath)
	return root
}

// --- Flags-only mode (specs_dir from event-spec.yaml, no source arg) ---

func TestGenerateCmd_FlagsOnly_Go(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, specsDir, "test_event.yaml", validSpecYAML)
	writeWorkspaceConfig(t, root, "specs")
	withWorkDir(t, root)

	outDir := t.TempDir()
	cmd := newGenerateCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--lang", "go", "--out", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "generated 1 event(s)") {
		t.Errorf("unexpected stdout: %s", stdout.String())
	}
	for _, f := range []string{"eventspec.go", "test_event.go"} {
		if _, err := os.Stat(filepath.Join(outDir, f)); err != nil {
			t.Errorf("expected %s to be generated: %v", f, err)
		}
	}
}

func TestGenerateCmd_FlagsOnly_TypeScript(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, specsDir, "test_event.yaml", validSpecYAML)
	writeWorkspaceConfig(t, root, "specs")
	withWorkDir(t, root)

	outDir := t.TempDir()
	cmd := newGenerateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--lang", "typescript", "--out", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range []string{"index.ts", "test_event.ts"} {
		if _, err := os.Stat(filepath.Join(outDir, f)); err != nil {
			t.Errorf("expected %s to be generated: %v", f, err)
		}
	}
}

// TestGenerateCmd_NoWorkspaceFallback — no event-spec.yaml; falls back to ./specs.
func TestGenerateCmd_NoWorkspaceFallback(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, specsDir, "test_event.yaml", validSpecYAML)
	withWorkDir(t, root) // no event-spec.yaml

	outDir := t.TempDir()
	cmd := newGenerateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--lang", "go", "--out", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("should fall back to ./specs when event-spec.yaml absent, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "eventspec.go")); err != nil {
		t.Errorf("expected eventspec.go: %v", err)
	}
}

func TestGenerateCmd_MissingLang(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, specsDir, "test_event.yaml", validSpecYAML)
	writeWorkspaceConfig(t, root, "specs")
	withWorkDir(t, root)

	cmd := newGenerateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --lang is missing with no source arg")
	}
}

func TestGenerateCmd_EmptySpecsDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceConfig(t, root, "specs") // specs dir exists but is empty
	withWorkDir(t, root)

	cmd := newGenerateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--lang", "go"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for empty specs dir")
	}
}

func TestGenerateCmd_UnsupportedLang(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, specsDir, "test_event.yaml", validSpecYAML)
	writeWorkspaceConfig(t, root, "specs")
	withWorkDir(t, root)

	cmd := newGenerateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--lang", "cobol", "--out", t.TempDir()})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

// --- Source mode (lang and out derived from source config) ---

func TestGenerateCmd_Source_DerivesLangAndOut(t *testing.T) {
	root := setupWorkspaceRoot(t, "go", "./out")
	withWorkDir(t, root)

	cmd := newGenerateCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"web-app"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "out", "eventspec.go")); err != nil {
		t.Errorf("expected eventspec.go in derived output dir: %v", err)
	}
}

func TestGenerateCmd_Source_FlagOverridesLang(t *testing.T) {
	// Source config says "typescript"; --lang go must win.
	root := setupWorkspaceRoot(t, "typescript", "./out")
	withWorkDir(t, root)

	cmd := newGenerateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--lang", "go", "web-app"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "out", "eventspec.go")); err != nil {
		t.Errorf("expected Go output when --lang overrides source config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "out", "index.ts")); err == nil {
		t.Error("expected no TypeScript output when --lang go overrides source config")
	}
}

func TestGenerateCmd_Source_MissingConfig(t *testing.T) {
	withWorkDir(t, t.TempDir()) // empty dir — no event-spec.yaml

	cmd := newGenerateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"web-app"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when event-spec.yaml is absent with source arg")
	}
}

func TestGenerateCmd_Source_MissingSourceFile(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceConfig(t, root, "specs")
	withWorkDir(t, root)

	cmd := newGenerateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"nonexistent"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when source file does not exist")
	}
}

// --- Registry mode tests ---

// TestGenerateCmd_GitMode_CacheNotFound — git mode with no local cache should
// tell the user to run 'event-spec pull' first.
func TestGenerateCmd_GitMode_CacheNotFound(t *testing.T) {
	root := t.TempDir()
	writeGitWorkspaceConfig(t, root, filepath.Join(root, "nonexistent-cache"))
	withWorkDir(t, root)

	cmd := newGenerateCmd()
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--lang", "go"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when git cache is absent")
	}
	if !strings.Contains(err.Error(), "pull") {
		t.Errorf("error should mention 'pull', got: %v", err)
	}
}

// TestGenerateCmd_ServerMode verifies that server mode connects to the registry
// HTTP client and returns a clear error when the server reports no events.
func TestGenerateCmd_ServerMode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, "[]")
	}))
	defer ts.Close()

	root := t.TempDir()
	content := fmt.Sprintf(`version: 1
workspace: test-workspace
registry:
  mode: server
  url: %s
sources_dir: sources
destinations_dir: destinations
`, ts.URL)
	if err := os.WriteFile(filepath.Join(root, "event-spec.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	withWorkDir(t, root)

	cmd := newGenerateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--lang", "go"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when server returns no events")
	}
	if !strings.Contains(err.Error(), "no event specs found") {
		t.Errorf("expected 'no event specs found', got: %v", err)
	}
}
