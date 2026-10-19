package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dejanradmanovic/event-spec/testutil"
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
	writePullWorkspaceConfig(t, root, remote, cacheDir, "")
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
	// With no registry.ref the branch is what the pull follows, so that is
	// what the line must name.
	if !strings.Contains(out, "@main") {
		t.Errorf("expected the progress line to name the branch, got: %s", out)
	}
	if !strings.Contains(out, "Indexed 1 event(s)") {
		t.Errorf("expected index count, got: %s", out)
	}
}

// TestPullCmd_ProgressLineNamesRef pins the progress line to what the pull
// actually checks out: registry.ref overrides registry.branch, so printing
// the branch under a pin named a ref the cache never lands on.
func TestPullCmd_ProgressLineNamesRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	remote := pullTestBareRepo(t, map[string]string{
		"ecommerce/product_viewed/1-0-0.yaml": pullEventYAML("ecommerce", "product_viewed", "1-0-0", "active"),
	})

	root := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	writePullWorkspaceConfig(t, root, remote, cacheDir, pullTestTag)
	withWorkDir(t, root)

	cmd := newPullCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("pull: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "@"+pullTestTag) {
		t.Errorf("expected the progress line to name the pinned ref %q, got: %s", pullTestTag, out)
	}
	if strings.Contains(out, "@main") {
		t.Errorf("progress line names the branch instead of the pin, got: %s", out)
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
	writePullWorkspaceConfig(t, root, remote, cacheDir, "")
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

// pullTestTag is the tag pullTestBareRepo puts on its single commit, so a
// test can pin registry.ref to something that is not a branch name.
const pullTestTag = "v1"

func pullTestBareRepo(t *testing.T, events map[string]string) string {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	bare := filepath.Join(root, "remote.git")

	mustRun := func(dir string, args ...string) {
		t.Helper()
		testutil.RunGit(t, dir, args...)
	}

	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	mustRun(work, "init", "-b", "main")
	mustRun(work, "config", "--local", "user.email", "test@test.com")
	mustRun(work, "config", "--local", "user.name", "Test")

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
	mustRun(work, "tag", pullTestTag)
	mustRun(root, "clone", "--bare", work, bare)
	return bare
}

// writePullWorkspaceConfig writes a git-mode workspace config. ref becomes
// registry.ref when non-empty; empty leaves the branch to be followed.
func writePullWorkspaceConfig(t *testing.T, dir, remote, cacheDir, ref string) {
	t.Helper()
	registryRef := ""
	if ref != "" {
		registryRef = "  ref: " + ref + "\n"
	}
	content := "version: 1\nworkspace: test-workspace\nregistry:\n  mode: git\n  remote: " + remote +
		"\n  branch: main\n" + registryRef + "  cache_dir: " + cacheDir +
		"\n  specs_dir: specs\nsources_dir: sources\ndestinations_dir: destinations\n"
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
