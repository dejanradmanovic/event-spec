package git_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/registry/git"
	"github.com/dejanradmanovic/event-spec/spec"
	"github.com/dejanradmanovic/event-spec/testutil"
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
		testutil.RunGit(t, dir, args...)
	}

	// Init a working tree, commit specs, then push to a bare clone.
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(work, "init", "-b", "main")
	mustGit(work, "config", "--local", "user.email", "test@test.com")
	mustGit(work, "config", "--local", "user.name", "Test")

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
		testutil.RunGit(t, dir, args...)
	}

	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(work, "init", "-b", "main")
	mustGit(work, "config", "--local", "user.email", "test@test.com")
	mustGit(work, "config", "--local", "user.name", "Test")

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

// TestNewBareRepo_IgnoresInheritedGitDir covers the whole stack under a hook's
// environment: the fixture helpers and the resolver's own runGit must both
// ignore an inherited GIT_DIR instead of cloning into it (#184).
func TestNewBareRepo_IgnoresInheritedGitDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	scratch := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, scratch, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(scratch, "file.txt"), []byte("scratch"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, scratch, "add", ".")
	testutil.RunGit(t, scratch, "commit", "-m", "scratch")

	headBefore := gitOutput(t, scratch, "rev-parse", "HEAD")
	configBefore, err := os.ReadFile(filepath.Join(scratch, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("GIT_DIR", filepath.Join(scratch, ".git"))

	remote := newBareRepo(t, map[string]string{
		"ecommerce/product_viewed/1-0-0.yaml": eventYAML("ecommerce", "product_viewed", "1-0-0", "active"),
	})
	resolver, err := git.NewWithPull(context.Background(), git.Config{
		Remote:   remote,
		Branch:   "main",
		CacheDir: filepath.Join(t.TempDir(), "cache"),
	})
	if err != nil {
		t.Fatalf("NewWithPull: %v", err)
	}
	// A second Pull takes the fetch/checkout path, where GIT_DIR would
	// override the resolver's own -C <cacheDir>.
	if err := resolver.Pull(context.Background()); err != nil {
		t.Fatalf("second pull: %v", err)
	}

	events, err := resolver.ListEvents(context.Background(), registry.ListFilter{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("want 1 event, got %d", len(events))
	}

	if head := gitOutput(t, scratch, "rev-parse", "HEAD"); head != headBefore {
		t.Errorf("scratch HEAD moved: %s -> %s", headBefore, head)
	}
	if status := gitOutput(t, scratch, "status", "--porcelain"); status != "" {
		t.Errorf("scratch working tree dirtied:\n%s", status)
	}
	configAfter, err := os.ReadFile(filepath.Join(scratch, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(configBefore, configAfter) {
		t.Errorf("scratch .git/config changed:\nbefore:\n%s\nafter:\n%s", configBefore, configAfter)
	}
}

// gitOutput runs git in dir through the isolated helper and returns trimmed
// combined output.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := testutil.GitCommand(context.Background(), dir, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestPull_FailureNamesStrippedExecVars covers the diagnosability half of the
// environment scrub: when the user had an env-var-based SSH setup, a failing
// pull must say event-spec dropped it and point at the git config equivalent,
// instead of leaving them with a bare "Permission denied (publickey)".
func TestPull_FailureNamesStrippedExecVars(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	const keyPath = "ssh -i /home/user/.ssh/deploy_key_secret"
	t.Setenv("GIT_SSH_COMMAND", keyPath)

	_, err := git.NewWithPull(context.Background(), git.Config{
		Remote:   filepath.Join(t.TempDir(), "does-not-exist.git"),
		Branch:   "main",
		CacheDir: filepath.Join(t.TempDir(), "cache"),
	})
	if err == nil {
		t.Fatal("expected the clone of a missing remote to fail")
	}

	msg := err.Error()
	for _, want := range []string{"GIT_SSH_COMMAND", "core.sshCommand", "event-spec deliberately removed"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not mention %q: %s", want, msg)
		}
	}
	if strings.Contains(msg, "deploy_key_secret") {
		t.Errorf("error leaks the variable's value: %s", msg)
	}
	if !strings.Contains(msg, "\n") {
		t.Errorf("hint is not broken across lines: %s", msg)
	}
	// The first line is git's own message and may embed a long temp path; the
	// hint that follows is hard-wrapped, and that is what 80 columns applies to.
	for i, line := range strings.Split(msg, "\n")[1:] {
		if n := utf8.RuneCountInString(line); n > 80 {
			t.Errorf("line %d of the error is %d runes, want <= 80:\n%s", i+2, n, line)
		}
	}
}

// TestPull_RejectsExtTransport checks the GIT_ALLOW_PROTOCOL default: Remote
// comes from event-spec.yaml and is not validated, and an "ext::" remote makes
// git run an arbitrary command. The sentinel proves nothing was executed.
func TestPull_RejectsExtTransport(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	sentinel := filepath.ToSlash(filepath.Join(dir, "executed"))
	script := filepath.Join(dir, "payload.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch \""+sentinel+"\"\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := git.NewWithPull(context.Background(), git.Config{
		Remote:   "ext::sh " + filepath.ToSlash(script),
		Branch:   "main",
		CacheDir: filepath.Join(dir, "cache"),
	})
	if err == nil {
		t.Fatal("expected an ext:: remote to be refused")
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatal("the ext:: payload executed; GIT_ALLOW_PROTOCOL did not block it")
	}
}
