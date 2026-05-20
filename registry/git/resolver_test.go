package git_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/registry/git"
	"github.com/dejanradmanovic/event-spec/spec"
)

const schemaHeader = `$schema: "https://event-spec.io/schemas/event/v1"` + "\n"

func eventYAML(namespace, name, version, status string) string {
	return schemaHeader +
		"name: " + name + "\n" +
		"namespace: " + namespace + "\n" +
		"version: " + version + "\n" +
		"status: " + status + "\n" +
		"event_name: " + name + "\n" +
		"type: track\n" +
		"properties: {}\n"
}

// newBareRepo creates a bare git repo at dir/remote.git, commits one event spec
// into it, and returns the path to be used as the Remote URL.
func newBareRepo(t *testing.T, events map[string]string) string {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	bare := filepath.Join(root, "remote.git")

	mustGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(context.Background(), "git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Init a working tree, commit specs, then push to a bare clone.
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(work, "init", "-b", "main")
	mustGit(work, "config", "user.email", "test@test.com")
	mustGit(work, "config", "user.name", "Test")

	specsDir := filepath.Join(work, "specs")
	for relPath, content := range events {
		full := filepath.Join(specsDir, relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustGit(work, "add", ".")
	mustGit(work, "commit", "-m", "init")

	mustGit(root, "clone", "--bare", work, bare)
	return bare
}

func TestNew_noCacheReturnsError(t *testing.T) {
	cfg := git.Config{
		Remote:   "https://example.com/repo.git",
		CacheDir: filepath.Join(t.TempDir(), "nonexistent"),
	}
	_, err := git.New(cfg)
	if err == nil {
		t.Fatal("expected error when cache missing, got nil")
	}
}

func TestPull_clonesRemoteAndIndexesEvents(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	remote := newBareRepo(t, map[string]string{
		"ecommerce/product_viewed/1-0-0.yaml": eventYAML("ecommerce", "product_viewed", "1-0-0", "active"),
	})

	cfg := git.Config{
		Remote:   remote,
		Branch:   "main",
		CacheDir: filepath.Join(t.TempDir(), "cache"),
	}

	// Pull must succeed without a pre-existing cache.
	resolver, err := git.NewWithPull(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewWithPull: %v", err)
	}

	events, err := resolver.ListEvents(context.Background(), registry.ListFilter{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 || events[0].Name != "product_viewed" {
		t.Errorf("want 1 event product_viewed, got %v", events)
	}
}

func TestPull_fetchUpdatesIndex(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Initial repo with one event.
	root := t.TempDir()
	work := filepath.Join(root, "work")
	bare := filepath.Join(root, "remote.git")

	mustGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(context.Background(), "git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(work, "init", "-b", "main")
	mustGit(work, "config", "user.email", "test@test.com")
	mustGit(work, "config", "user.name", "Test")

	writeSpec := func(relPath, content string) {
		full := filepath.Join(work, "specs", relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeSpec("ecommerce/product_viewed/1-0-0.yaml", eventYAML("ecommerce", "product_viewed", "1-0-0", "active"))
	mustGit(work, "add", ".")
	mustGit(work, "commit", "-m", "first event")
	mustGit(root, "clone", "--bare", work, bare)

	cacheDir := filepath.Join(root, "cache")
	cfg := git.Config{Remote: bare, Branch: "main", CacheDir: cacheDir}

	resolver, err := git.NewWithPull(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first pull: %v", err)
	}

	events, _ := resolver.ListEvents(context.Background(), registry.ListFilter{})
	if len(events) != 1 {
		t.Fatalf("after first pull: want 1 event, got %d", len(events))
	}

	// Push a second event to the remote.
	writeSpec("auth/user_signed_up/1-0-0.yaml", eventYAML("auth", "user_signed_up", "1-0-0", "active"))
	mustGit(work, "add", ".")
	mustGit(work, "commit", "-m", "second event")
	mustGit(work, "push", bare, "main")

	// Second pull should pick up the new event.
	if err := resolver.Pull(context.Background()); err != nil {
		t.Fatalf("second pull: %v", err)
	}

	events, _ = resolver.ListEvents(context.Background(), registry.ListFilter{})
	if len(events) != 2 {
		t.Errorf("after second pull: want 2 events, got %d", len(events))
	}
}

func TestPublishEvent_returnsErrReadOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	remote := newBareRepo(t, map[string]string{
		"ecommerce/product_viewed/1-0-0.yaml": eventYAML("ecommerce", "product_viewed", "1-0-0", "active"),
	})
	resolver, err := git.NewWithPull(context.Background(), git.Config{
		Remote:   remote,
		Branch:   "main",
		CacheDir: filepath.Join(t.TempDir(), "cache"),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = resolver.PublishEvent(context.Background(), spec.EventDef{})
	if !errors.Is(err, registry.ErrReadOnly) {
		t.Errorf("want ErrReadOnly, got %v", err)
	}
}
