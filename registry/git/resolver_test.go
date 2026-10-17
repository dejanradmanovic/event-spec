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

// refRemote is a fixture remote for the ref-resolution tests: a bare repo
// holding one commit on main, tagged, plus the working tree it was pushed
// from so a test can move the remote on afterwards.
type refRemote struct {
	bare string
	work string
	tag  string
	// first is the SHA of the commit the tag points at, which is also the
	// only commit on the remote until push is called.
	first string
}

// newRefRemote builds the fixture in the style of TestPull_fetchUpdatesIndex:
// init -b main with a local identity, one commit, a tag, then a bare clone.
func newRefRemote(t *testing.T) *refRemote {
	t.Helper()
	root := t.TempDir()
	r := &refRemote{
		bare: filepath.Join(root, "remote.git"),
		work: filepath.Join(root, "work"),
		tag:  "v1",
	}
	if err := os.MkdirAll(r.work, 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, r.work, "init", "-b", "main")
	testutil.RunGit(t, r.work, "config", "--local", "user.email", "test@test.com")
	testutil.RunGit(t, r.work, "config", "--local", "user.name", "Test")

	r.first = r.commit(t, "ecommerce", "product_viewed")
	testutil.RunGit(t, r.work, "tag", r.tag)
	testutil.RunGit(t, root, "clone", "--bare", r.work, r.bare)
	return r
}

// commit writes one event spec into the working tree, commits it, and returns
// the new SHA.
func (r *refRemote) commit(t *testing.T, namespace, name string) string {
	t.Helper()
	full := filepath.Join(r.work, "specs", namespace, name, "1-0-0.yaml")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	content := eventYAML(namespace, name, "1-0-0", "active")
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, r.work, "add", ".")
	testutil.RunGit(t, r.work, "commit", "-m", namespace+"/"+name)
	return headSHA(t, r.work)
}

// push advances main on the bare remote to the working tree's HEAD.
func (r *refRemote) push(t *testing.T) {
	t.Helper()
	testutil.RunGit(t, r.work, "push", r.bare, "main")
}

// url is the Remote the resolver is configured with. It is a file:// URL, not
// the bare path: git ignores --depth for a plain local path ("warning: --depth
// is ignored in local clones"), which would make a shallow clone a full one
// and quietly rob the pin-older-than-the-clone case of its point.
func (r *refRemote) url() string {
	path := filepath.ToSlash(r.bare)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path // a Windows path starts at the drive letter
	}
	return "file://" + path
}

// TestPull_refResolution is the regression matrix for #211: a ref naming a
// branch used to check out the stale local branch the first clone created, so
// every later pull was a no-op, while a ref naming a commit was ignored by
// the clone path altogether.
func TestPull_refResolution(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tests := []struct {
		name string
		// ref is read after the fixture exists, so a case can pin to a SHA.
		ref func(r *refRemote) string
		// wantHead reports the SHA the cache must sit on after both pulls;
		// nil means the second commit's, i.e. the ref followed the remote.
		wantHead   func(r *refRemote, second string) string
		wantEvents int
	}{
		{
			name:       "branch-named ref follows the remote",
			ref:        func(*refRemote) string { return "main" },
			wantEvents: 2,
		},
		{
			name:       "empty ref follows the branch",
			ref:        func(*refRemote) string { return "" },
			wantEvents: 2,
		},
		{
			name:       "commit SHA pins",
			ref:        func(r *refRemote) string { return r.first },
			wantHead:   func(r *refRemote, _ string) string { return r.first },
			wantEvents: 1,
		},
		{
			name:       "tag pins",
			ref:        func(r *refRemote) string { return r.tag },
			wantHead:   func(r *refRemote, _ string) string { return r.first },
			wantEvents: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remote := newRefRemote(t)
			cacheDir := filepath.Join(t.TempDir(), "cache")
			resolver, err := git.NewWithPull(context.Background(), git.Config{
				Remote:   remote.url(),
				Branch:   "main",
				CacheDir: cacheDir,
				Ref:      tt.ref(remote),
			})
			if err != nil {
				t.Fatalf("first pull: %v", err)
			}

			second := remote.commit(t, "auth", "user_signed_up")
			remote.push(t)

			if err := resolver.Pull(context.Background()); err != nil {
				t.Fatalf("second pull: %v", err)
			}

			want := second
			if tt.wantHead != nil {
				want = tt.wantHead(remote, second)
			}
			if got := headSHA(t, cacheDir); got != want {
				t.Errorf("cache HEAD = %s, want %s", got, want)
			}

			events, err := resolver.ListEvents(context.Background(), registry.ListFilter{})
			if err != nil {
				t.Fatalf("ListEvents: %v", err)
			}
			if len(events) != tt.wantEvents {
				t.Errorf("want %d event(s), got %d", tt.wantEvents, len(events))
			}

			// The cache must never sit on a local branch: that is the shape
			// the stale-ref bug needed to exist at all.
			if branch, ok := headBranch(t, cacheDir); ok {
				t.Errorf("cache HEAD is on local branch %q, want detached", branch)
			}
		})
	}

	// A pin older than the tip proves the clone path honours Ref: under
	// --depth 1 the commit would not be in the cache at all.
	t.Run("pin older than the clone resolves on a fresh cache", func(t *testing.T) {
		remote := newRefRemote(t)
		remote.commit(t, "auth", "user_signed_up")
		remote.push(t)

		cacheDir := filepath.Join(t.TempDir(), "cache")
		resolver, err := git.NewWithPull(context.Background(), git.Config{
			Remote:   remote.url(),
			Branch:   "main",
			CacheDir: cacheDir,
			Ref:      remote.first,
		})
		if err != nil {
			t.Fatalf("first pull: %v", err)
		}
		if got := headSHA(t, cacheDir); got != remote.first {
			t.Errorf("cache HEAD = %s, want the pinned %s", got, remote.first)
		}
		events, err := resolver.ListEvents(context.Background(), registry.ListFilter{})
		if err != nil {
			t.Fatalf("ListEvents: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("want 1 event at the pinned commit, got %d", len(events))
		}
	})

	t.Run("unknown ref fails loudly", func(t *testing.T) {
		remote := newRefRemote(t)
		_, err := git.NewWithPull(context.Background(), git.Config{
			Remote:   remote.url(),
			Branch:   "main",
			CacheDir: filepath.Join(t.TempDir(), "cache"),
			Ref:      "no-such-thing",
		})
		if err == nil {
			t.Fatal("expected a pull pinned to a missing ref to fail")
		}
		for _, want := range []string{"registry.ref", "no-such-thing"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error does not mention %q: %v", want, err)
			}
		}
	})
}

// headSHA returns the commit dir's HEAD points at. Local to this file rather
// than in testutil: only the ref-resolution tests need it.
func headSHA(t *testing.T, dir string) string {
	t.Helper()
	out, err := testutil.GitCommand(context.Background(), dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD in %s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

// headBranch returns the branch dir's HEAD is attached to, if any.
// symbolic-ref exits non-zero on a detached HEAD, which is the state every
// successful pull must leave the cache in.
func headBranch(t *testing.T, dir string) (string, bool) {
	t.Helper()
	out, err := testutil.GitCommand(context.Background(), dir, "symbolic-ref", "-q", "HEAD").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
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
	// A blank line separates git's own message from event-spec's advice.
	if !strings.Contains(msg, "\n\n") {
		t.Errorf("no blank line between git's message and the hint: %s", msg)
	}
	// The embedded newlines must not cost the caller the wrapped error: the
	// hint is spliced in with %w, so the exit status is still reachable.
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("the hint wrap broke unwrapping: %T does not unwrap to *exec.ExitError", err)
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
