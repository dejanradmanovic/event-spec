package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPullCmd_WrongMode(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceConfig(t, root, "specs") // local mode
	withWorkDir(t, root)

	cmd := newPullCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-git mode")
	}
	if !strings.Contains(err.Error(), "git") {
		t.Errorf("expected error to mention 'git', got: %v", err)
	}
}

func TestPullCmd_NoConfig(t *testing.T) {
	withWorkDir(t, t.TempDir())

	cmd := newPullCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when event-spec.yaml is absent")
	}
}

func TestPullCmd_SourceNameMismatch(t *testing.T) {
	root := t.TempDir()
	writeGitWorkspaceConfig(t, root, filepath.Join(t.TempDir(), "cache"))
	withWorkDir(t, root)

	cmd := newPullCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"wrong-source"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for mismatched source name")
	}
	if !strings.Contains(err.Error(), "workspace") {
		t.Errorf("expected error to mention 'workspace', got: %v", err)
	}
}

func TestPullCmd_ClonesAndIndexes(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	remote := pullTestBareRepo(t, map[string]string{
		"ecommerce/product_viewed/1-0-0.yaml": pullEventYAML("ecommerce", "product_viewed", "1-0-0", "active"),
	})

	root := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	writePullWorkspaceConfig(t, root, remote, cacheDir)
	withWorkDir(t, root)

	cmd := newPullCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("pull: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Pulling") {
		t.Errorf("expected progress line, got: %s", out)
	}
	if !strings.Contains(out, "Indexed 1 event(s)") {
		t.Errorf("expected index count, got: %s", out)
	}
}

func TestPullCmd_Force(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	remote := pullTestBareRepo(t, map[string]string{
		"ecommerce/product_viewed/1-0-0.yaml": pullEventYAML("ecommerce", "product_viewed", "1-0-0", "active"),
	})

	root := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	writePullWorkspaceConfig(t, root, remote, cacheDir)
	withWorkDir(t, root)

	// Initial pull to populate the cache.
	cmd := newPullCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("initial pull: %v", err)
	}

	// Corrupt the cache so a normal pull would fail.
	if err := os.RemoveAll(filepath.Join(cacheDir, ".git")); err != nil {
		t.Fatal(err)
	}

	// --force should wipe the corrupted cache and re-clone cleanly.
	cmd = newPullCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("force pull: %v", err)
	}
	if !strings.Contains(stdout.String(), "Indexed 1 event(s)") {
		t.Errorf("expected index count after --force, got: %s", stdout.String())
	}
}

// --- local helpers ---

func pullTestBareRepo(t *testing.T, events map[string]string) string {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	bare := filepath.Join(root, "remote.git")

	mustRun := func(dir string, args ...string) {
		t.Helper()
		c := exec.CommandContext(context.Background(), "git", args...)
		c.Dir = dir
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	mustRun(work, "init", "-b", "main")
	mustRun(work, "config", "user.email", "test@test.com")
	mustRun(work, "config", "user.name", "Test")

	for relPath, content := range events {
		full := filepath.Join(work, "specs", relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustRun(work, "add", ".")
	mustRun(work, "commit", "-m", "init")
	mustRun(root, "clone", "--bare", work, bare)
	return bare
}

func writePullWorkspaceConfig(t *testing.T, dir, remote, cacheDir string) {
	t.Helper()
	content := "version: 1\nworkspace: test-workspace\nregistry:\n  mode: git\n  remote: " + remote +
		"\n  branch: main\n  cache_dir: " + cacheDir + "\n  specs_dir: specs\nsources_dir: sources\ndestinations_dir: destinations\n"
	if err := os.WriteFile(filepath.Join(dir, "event-spec.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func pullEventYAML(namespace, name, version, status string) string {
	return `$schema: "https://event-spec.io/schemas/event/v1"` + "\n" +
		"name: " + name + "\n" +
		"namespace: " + namespace + "\n" +
		"version: " + version + "\n" +
		"status: " + status + "\n" +
		"event_name: " + name + "\n" +
		"type: track\n" +
		"properties: {}\n"
}
