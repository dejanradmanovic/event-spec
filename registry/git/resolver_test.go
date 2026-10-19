package git_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
// holding one commit on main, tagged twice, plus the working tree it was
// pushed from so a test can move the remote on afterwards.
type refRemote struct {
	bare string
	work string
	tag  string
	// annotatedTag names the same commit as tag, through a tag *object*
	// whose own SHA is not the commit SHA. Resolution has to peel it.
	annotatedTag string
	// first is the SHA of the commit the tags point at, which is also the
	// only commit on the remote until push is called.
	first string
}

// newRefRemote builds the fixture in the style of TestPull_fetchUpdatesIndex:
// init -b main with a local identity, one commit, two tags, then a bare clone.
func newRefRemote(t *testing.T) *refRemote {
	t.Helper()
	root := t.TempDir()
	r := &refRemote{
		bare:         filepath.Join(root, "remote.git"),
		work:         filepath.Join(root, "work"),
		tag:          "v1",
		annotatedTag: "v1-annotated",
	}
	if err := os.MkdirAll(r.work, 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, r.work, "init", "-b", "main")
	testutil.RunGit(t, r.work, "config", "--local", "user.email", "test@test.com")
	testutil.RunGit(t, r.work, "config", "--local", "user.name", "Test")

	r.first = r.commit(t, "ecommerce", "product_viewed")
	testutil.RunGit(t, r.work, "tag", r.tag)
	testutil.RunGit(t, r.work, "tag", "-a", r.annotatedTag, "-m", "release")
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

// commitOnBranch commits on a side branch and returns to main, so the commit
// is reachable from nothing the bare remote tracks. A tag on such a commit is
// the one a plain fetch will not auto-follow.
func (r *refRemote) commitOnBranch(t *testing.T, branch, namespace, name string) string {
	t.Helper()
	testutil.RunGit(t, r.work, "checkout", "-q", "-b", branch)
	sha := r.commit(t, namespace, name)
	testutil.RunGit(t, r.work, "checkout", "-q", "main")
	return sha
}

// push advances main on the bare remote to the working tree's HEAD.
func (r *refRemote) push(t *testing.T) {
	t.Helper()
	testutil.RunGit(t, r.work, "push", r.bare, "main")
}

// pushBranch creates a branch of the given name on the bare remote at sha. It
// is how the fixture plays the shadowing push: a branch named after a tag, or
// after a pinned commit's own SHA.
func (r *refRemote) pushBranch(t *testing.T, sha, name string) {
	t.Helper()
	testutil.RunGit(t, r.work, "push", r.bare, sha+":refs/heads/"+name)
}

// pushTagOnly puts a tag on the bare remote without any branch reaching the
// commit it points at.
func (r *refRemote) pushTagOnly(t *testing.T, sha, name string) {
	t.Helper()
	testutil.RunGit(t, r.work, "tag", name, sha)
	testutil.RunGit(t, r.work, "push", r.bare, "refs/tags/"+name)
}

// moveTag retargets an existing tag on the bare remote, which is what git
// refuses to replicate into a cache that already has it. Force is needed on
// the push only: the fixture is playing the upstream, not the cache.
func (r *refRemote) moveTag(t *testing.T, sha, name string) {
	t.Helper()
	testutil.RunGit(t, r.work, "tag", "-f", name, sha)
	testutil.RunGit(t, r.work, "push", "-f", r.bare, "refs/tags/"+name)
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
		// The remote was asked and has no such name, so a re-clone provably
		// cannot help and the message must not send the user through one.
		if strings.Contains(err.Error(), "--force") {
			t.Errorf("error promises --force for a name the remote lacks:\n%v", err)
		}
		if !strings.Contains(err.Error(), "advertises no tag or branch of that name") {
			t.Errorf("error does not say the remote lacks the name:\n%v", err)
		}
	})

	// The ux finding round 2 left open: every cache an ordinary ref-less pull
	// creates is shallow, so gating the remedy on shallowness alone promised
	// a re-clone for a plain typo — and a re-clone landed on the same error.
	// Nothing about the cache can tell those apart, so the remote is asked.
	for _, tt := range []struct {
		name  string
		typo  func(cfg *git.Config)
		wants []string
	}{
		{
			name:  "ref",
			typo:  func(cfg *git.Config) { cfg.Ref = "typo-on-a-shallow-cache" },
			wants: []string{"registry.ref", "typo-on-a-shallow-cache"},
		},
		{
			name:  "branch",
			typo:  func(cfg *git.Config) { cfg.Branch = "maiin-typo" },
			wants: []string{"registry.branch", "maiin-typo"},
		},
	} {
		t.Run("a typo in "+tt.name+" on an ordinary shallow cache does not promise --force", func(t *testing.T) {
			remote := newRefRemote(t)
			cacheDir := filepath.Join(t.TempDir(), "cache")
			cfg := git.Config{Remote: remote.url(), Branch: "main", CacheDir: cacheDir}
			if _, err := git.NewWithPull(context.Background(), cfg); err != nil {
				t.Fatalf("first pull without a ref: %v", err)
			}
			if !isShallow(t, cacheDir) {
				t.Fatal("fixture precondition: an ordinary ref-less pull must leave a shallow cache")
			}

			tt.typo(&cfg)
			_, err := git.NewWithPull(context.Background(), cfg)
			if err == nil {
				t.Fatal("expected a typo to fail")
			}
			for _, want := range tt.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error does not mention %q:\n%v", want, err)
				}
			}
			if strings.Contains(err.Error(), "--force") {
				t.Errorf("error promises --force for a typo on a shallow cache:\n%v", err)
			}
			if !strings.Contains(err.Error(), "advertises no tag or branch of that name") {
				t.Errorf("error does not say the remote lacks the name:\n%v", err)
			}
		})
	}

	// A tag and a branch of one name at different commits is the shadowing
	// push: with the clone no longer shallow, a branch named after a tag is
	// fetched into the cache, and resolving the branch first would hand
	// anyone with branch-push access every consumer pinned to that tag.
	t.Run("a tag shadowed by a same-named branch is refused", func(t *testing.T) {
		remote := newRefRemote(t)
		second := remote.commit(t, "auth", "user_signed_up")
		remote.push(t)
		remote.pushBranch(t, second, remote.tag)

		_, err := git.NewWithPull(context.Background(), git.Config{
			Remote:   remote.url(),
			Branch:   "main",
			CacheDir: filepath.Join(t.TempDir(), "cache"),
			Ref:      remote.tag,
		})
		if err == nil {
			t.Fatalf("expected an ambiguous %q to be refused", remote.tag)
		}
		for _, want := range []string{"registry.ref", "ambiguous", remote.tag, remote.first, second} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error does not mention %q:\n%v", want, err)
			}
		}
	})

	// The round-2 HIGH: git accepts a branch whose name is a full 40-hex
	// string, and refs/remotes/origin/<Ref> is a candidate event-spec
	// invents, not one git would try for a bare SHA — so the branch used to
	// win over the commit and silently redirect the pin to the attacker's
	// commit. That is the pin the ambiguity error itself recommends, so it
	// has to be the one thing nothing on the remote can shadow.
	t.Run("a commit pin shadowed by a branch of that SHA is refused", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			ref  func(r *refRemote) string
		}{
			{name: "full SHA", ref: func(r *refRemote) string { return r.first }},
			{name: "abbreviated SHA", ref: func(r *refRemote) string { return r.first[:7] }},
		} {
			t.Run(tt.name, func(t *testing.T) {
				remote := newRefRemote(t)
				// The shadow commit stays off main, so main's tip is still the
				// pinned commit: the only way the cache can land on evil is by
				// resolving the branch named after the pin.
				evil := remote.commitOnBranch(t, "evil", "auth", "user_signed_up")
				pin := tt.ref(remote)
				remote.pushBranch(t, evil, pin)

				cacheDir := filepath.Join(t.TempDir(), "cache")
				_, err := git.NewWithPull(context.Background(), git.Config{
					Remote:   remote.url(),
					Branch:   "main",
					CacheDir: cacheDir,
					Ref:      pin,
				})
				if err == nil {
					t.Fatalf("expected a commit pin shadowed by branch %q to be refused", pin)
				}
				for _, want := range []string{"registry.ref", "ambiguous", remote.first, evil} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error does not mention %q:\n%v", want, err)
					}
				}
				// The refusal is the point: the attacker's commit must never
				// become the indexed one.
				if head := headSHA(t, cacheDir); head == evil {
					t.Errorf("cache HEAD is the shadow branch's commit %s", evil)
				}
			})
		}
	})

	// The same name at the *same* commit is not ambiguous and must not be
	// refused: a tag and a release branch on one commit is an ordinary thing
	// for a tracking-plan repo to carry. The annotated case is what makes the
	// ^{commit} peeling load-bearing — an annotated tag's own object SHA is
	// not the commit SHA, so comparing unpeeled would refuse this.
	for _, tt := range []struct {
		name string
		tag  func(r *refRemote) string
	}{
		{name: "lightweight tag", tag: func(r *refRemote) string { return r.tag }},
		{name: "annotated tag", tag: func(r *refRemote) string { return r.annotatedTag }},
	} {
		t.Run("a same-named branch at the same commit still resolves, "+tt.name, func(t *testing.T) {
			remote := newRefRemote(t)
			name := tt.tag(remote)
			remote.pushBranch(t, remote.first, name)

			cacheDir := filepath.Join(t.TempDir(), "cache")
			resolver, err := git.NewWithPull(context.Background(), git.Config{
				Remote:   remote.url(),
				Branch:   "main",
				CacheDir: cacheDir,
				Ref:      name,
			})
			if err != nil {
				t.Fatalf("pull: %v", err)
			}
			if got := headSHA(t, cacheDir); got != remote.first {
				t.Errorf("cache HEAD = %s, want %s", got, remote.first)
			}
			if _, err := resolver.ListEvents(context.Background(), registry.ListFilter{}); err != nil {
				t.Fatalf("ListEvents: %v", err)
			}
		})
	}

	// The one case where re-cloning is the fix, and the one the docs promise:
	// a cache cloned before the pin existed is shallow and a fetch never
	// deepens it. The pin is the middle commit, which no tag points at —
	// fetching tags brings their commits in, so a tagged commit would now
	// resolve here, which is a better outcome but not this test's subject.
	t.Run("a pin added to a shallow cache names --force", func(t *testing.T) {
		remote := newRefRemote(t)
		middle := remote.commit(t, "auth", "user_signed_up")
		remote.commit(t, "billing", "invoice_paid")
		remote.push(t)

		cacheDir := filepath.Join(t.TempDir(), "cache")
		cfg := git.Config{Remote: remote.url(), Branch: "main", CacheDir: cacheDir}
		if _, err := git.NewWithPull(context.Background(), cfg); err != nil {
			t.Fatalf("first pull without a ref: %v", err)
		}

		cfg.Ref = middle // older than the shallow cache's only commit, and untagged
		_, err := git.NewWithPull(context.Background(), cfg)
		if err == nil {
			t.Fatal("expected a pin unreachable in a shallow cache to fail")
		}
		for _, want := range []string{"registry.ref", "shallow", "event-spec pull --force"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error does not mention %q:\n%v", want, err)
			}
		}

		// The remedy it names has to work, which is the whole difference
		// between this message and round 1's.
		cfg.Force = true
		resolver, err := git.NewWithPull(context.Background(), cfg)
		if err != nil {
			t.Fatalf("the --force the error recommends still failed: %v", err)
		}
		if got := headSHA(t, cacheDir); got != middle {
			t.Errorf("after --force, cache HEAD = %s, want the pinned %s", got, middle)
		}
		if _, err := resolver.ListEvents(context.Background(), registry.ListFilter{}); err != nil {
			t.Fatalf("ListEvents: %v", err)
		}
	})

	// The empty-Ref error path, reachable in the same shallow cache: a
	// --depth 1 clone is single-branch, so a registry.branch changed
	// afterwards has no remote-tracking ref and no fetch will create one.
	// Here the remote does have the branch, so --force is the real fix.
	t.Run("a branch missing from a shallow cache names --force", func(t *testing.T) {
		remote := newRefRemote(t)
		cacheDir := filepath.Join(t.TempDir(), "cache")
		cfg := git.Config{Remote: remote.url(), Branch: "main", CacheDir: cacheDir}
		if _, err := git.NewWithPull(context.Background(), cfg); err != nil {
			t.Fatalf("first pull on main: %v", err)
		}

		remote.pushBranch(t, remote.first, "release")
		cfg.Branch = "release"
		_, err := git.NewWithPull(context.Background(), cfg)
		if err == nil {
			t.Fatal("expected a branch absent from a single-branch cache to fail")
		}
		for _, want := range []string{"registry.branch", "release", "event-spec pull --force", "refs/heads/release"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error does not mention %q:\n%v", want, err)
			}
		}
	})

	// A tag on a commit no branch reaches never arrives through a plain
	// fetch, so on a pre-existing cache the tag side of the comparison was
	// empty and a branch pushed under that tag's name won by default. The
	// fetch asks for tags now; this is the steady state, not a first clone.
	t.Run("a shadowing push is refused on a pre-existing cache too", func(t *testing.T) {
		remote := newRefRemote(t)
		cacheDir := filepath.Join(t.TempDir(), "cache")
		cfg := git.Config{Remote: remote.url(), Branch: "main", CacheDir: cacheDir, Ref: remote.tag}
		if _, err := git.NewWithPull(context.Background(), cfg); err != nil {
			t.Fatalf("first pull: %v", err)
		}

		// A release tag on a commit that no branch reaches: the branch it was
		// cut from is gone, which is why a fetch will not auto-follow it.
		release := remote.commitOnBranch(t, "release-1", "billing", "invoice_paid")
		remote.pushTagOnly(t, release, "v2")
		evil := remote.commit(t, "auth", "user_signed_up")
		remote.push(t)
		remote.pushBranch(t, evil, "v2")

		cfg.Ref = "v2"
		_, err := git.NewWithPull(context.Background(), cfg)
		if err == nil {
			t.Fatal("expected the shadowed tag to be refused on an existing cache")
		}
		for _, want := range []string{"registry.ref", "ambiguous", release, evil} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error does not mention %q:\n%v", want, err)
			}
		}
		if head := headSHA(t, cacheDir); head == evil {
			t.Errorf("cache HEAD is the shadow branch's commit %s", evil)
		}
	})
}

// TestPull_tagMovedUpstream is the case whose absence let round 3's
// "fetch origin --tags" past three review rounds: every tag fixture until now
// *added* a tag the cache did not have, and adding one always succeeds. Moving
// one the cache already holds is what git refuses — correctly, because that
// refusal is what stops an upstream tag move from retargeting a pin — but git
// reports the refusal as a failed ref update and exits non-zero, so fetching
// the whole tag namespace turned any cached tag moving anywhere on the remote
// into a hard failure of every later pull.
func TestPull_tagMovedUpstream(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// cacheWithTag leaves a cache that holds remote.tag at remote.first,
	// then moves that tag upstream to a second commit on main and returns
	// the new tip. A pin set beforehand makes the first clone a full one, so
	// the cache really does carry the tag before it moves.
	cacheWithTag := func(t *testing.T, remote *refRemote, cacheDir string) string {
		t.Helper()
		cfg := git.Config{Remote: remote.url(), Branch: "main", CacheDir: cacheDir, Ref: remote.tag}
		if _, err := git.NewWithPull(context.Background(), cfg); err != nil {
			t.Fatalf("first pull: %v", err)
		}
		if got := tagSHA(t, cacheDir, remote.tag); got != remote.first {
			t.Fatalf("fixture precondition: cache tag %s = %q, want %s", remote.tag, got, remote.first)
		}
		moved := remote.commit(t, "auth", "user_signed_up")
		remote.push(t)
		remote.moveTag(t, moved, remote.tag)
		return moved
	}

	// Each case starts from that cache and pulls again with its own config.
	// None of them names a tag that is supposed to move, and none of them
	// may fail.
	tests := []struct {
		name string
		// config is the second pull's configuration.
		config func(remote *refRemote, moved string) (ref string, branch string)
		// wantHead is the commit the second pull must land on.
		wantHead func(remote *refRemote, moved string) string
	}{
		{
			// The documented default, and the configuration the approver
			// reproduced the breakage on: nothing here names a tag at all.
			name:     "a ref-less branch-tracking pull",
			config:   func(*refRemote, string) (string, string) { return "", "main" },
			wantHead: func(_ *refRemote, moved string) string { return moved },
		},
		{
			// The configuration spec.md says every existing user has
			// committed, which security's one-line gate would still have
			// broken: Ref is set, so the gate lets the tag fetch run, and
			// only narrowing it to refs/tags/main keeps this working.
			name:     "a branch-named ref",
			config:   func(*refRemote, string) (string, string) { return "main", "main" },
			wantHead: func(_ *refRemote, moved string) string { return moved },
		},
		{
			// A commit pin, which must be unaffected by any tag anywhere.
			name: "a commit pin",
			config: func(remote *refRemote, _ string) (string, string) {
				return remote.first, "main"
			},
			wantHead: func(remote *refRemote, _ string) string { return remote.first },
		},
		{
			// The pin that names the tag that moved. The pull must succeed
			// and must keep the commit it was cached at: an upstream tag move
			// retargeting a cached pin is the thing the unforced refspec
			// exists to prevent.
			name: "a pin on the tag that moved",
			config: func(remote *refRemote, _ string) (string, string) {
				return remote.tag, "main"
			},
			wantHead: func(remote *refRemote, _ string) string { return remote.first },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remote := newRefRemote(t)
			cacheDir := filepath.Join(t.TempDir(), "cache")
			moved := cacheWithTag(t, remote, cacheDir)

			ref, branch := tt.config(remote, moved)
			resolver, err := git.NewWithPull(context.Background(), git.Config{
				Remote:   remote.url(),
				Branch:   branch,
				CacheDir: cacheDir,
				Ref:      ref,
			})
			if err != nil {
				t.Fatalf("second pull after an upstream tag move: %v", err)
			}
			if got := headSHA(t, cacheDir); got != tt.wantHead(remote, moved) {
				t.Errorf("cache HEAD = %s, want %s", got, tt.wantHead(remote, moved))
			}
			if _, err := resolver.ListEvents(context.Background(), registry.ListFilter{}); err != nil {
				t.Fatalf("ListEvents: %v", err)
			}
			// Whatever else happened, the cached tag must not have followed
			// the remote: that is the property the refspec stays unforced for.
			if got := tagSHA(t, cacheDir, remote.tag); got != remote.first {
				t.Errorf("cached tag %s = %s, want the commit it was cached at %s",
					remote.tag, got, remote.first)
			}
		})
	}
}

// TestPull_refLessPullLeavesCacheShallow pins the sentence the docs and the
// shallow-cache remedy both state: a fetch does not deepen the cache the
// documented ref-less default creates. Round 3's whole-namespace tag fetch
// dragged in the commits every tag pointed at and made both of them false.
func TestPull_refLessPullLeavesCacheShallow(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	remote := newRefRemote(t)
	// Tagged history off to the side, which a tag fetch would pull in: the
	// tag is on a commit no branch reaches, so nothing but an explicit tag
	// fetch can bring it or its commit into the cache.
	side := remote.commitOnBranch(t, "release-1", "billing", "invoice_paid")
	remote.pushTagOnly(t, side, "v2")
	remote.commit(t, "auth", "user_signed_up")
	remote.push(t)

	cacheDir := filepath.Join(t.TempDir(), "cache")
	cfg := git.Config{Remote: remote.url(), Branch: "main", CacheDir: cacheDir}
	if _, err := git.NewWithPull(context.Background(), cfg); err != nil {
		t.Fatalf("first pull: %v", err)
	}
	before := commitCount(t, cacheDir)

	// A second commit upstream, so the second pull has real work to do.
	remote.commit(t, "ecommerce", "cart_viewed")
	remote.push(t)
	resolver, err := git.NewWithPull(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second pull: %v", err)
	}
	if _, err := resolver.ListEvents(context.Background(), registry.ListFilter{}); err != nil {
		t.Fatalf("ListEvents: %v", err)
	}

	if !isShallow(t, cacheDir) {
		t.Error("a ref-less pull left the cache non-shallow")
	}
	// One new commit on main is the work this fetch legitimately had to do.
	// Anything beyond that is tagged history the cache was never asked for.
	if got, want := commitCount(t, cacheDir), before+1; got != want {
		t.Errorf("a ref-less fetch deepened the cache: %d commits, want %d", got, want)
	}
	if sha := tagSHA(t, cacheDir, "v2"); sha != "" {
		t.Errorf("a ref-less fetch brought tag v2 (%s) into the cache", sha)
	}
	if hasCommit(t, cacheDir, side) {
		t.Errorf("a ref-less fetch brought the tagged side commit %s into the cache", side)
	}
}

// TestPull_rejectsRefsThatCannotName covers the values docs/docs/registry/git.md
// promises are refused. Candidates 1 and 2 build a revision *expression*, so
// before this check "main~1" and friends resolved and quietly followed a
// moving branch. registry.branch goes through the same rule, since it reaches
// "clone --branch" and the ls-remote remedy probe. Rejected before any clone,
// so no fixture remote is needed.
func TestPull_rejectsRefsThatCannotName(t *testing.T) {
	tests := []struct {
		// key is the config field under test.
		key   string
		value string
		want  string
	}{
		{key: "registry.ref", value: "HEAD", want: "not a ref the"},
		{key: "registry.ref", value: "main~1", want: "revision-expression syntax"},
		{key: "registry.ref", value: "main^", want: "revision-expression syntax"},
		{key: "registry.ref", value: "v1.4.0^{}", want: "revision-expression syntax"},
		{key: "registry.ref", value: "main@{upstream}", want: "revision-expression syntax"},
		{key: "registry.ref", value: "v1..v2", want: "revision-expression syntax"},
		{key: "registry.ref", value: "a b", want: "revision-expression syntax"},
		{key: "registry.ref", value: "refs/heads/main", want: "without the refs/ prefix"},
		{key: "registry.ref", value: "refs/tags/v1", want: "without the refs/ prefix"},
		// A leading dash is an argument-injection guard, not revision syntax,
		// and says so rather than listing characters the value does not have.
		{key: "registry.ref", value: "--upload-pack=calc", want: "cannot begin with \"-\""},
		{key: "registry.ref", value: "-rf", want: "cannot begin with \"-\""},
		{key: "registry.branch", value: "--upload-pack=calc", want: "cannot begin with \"-\""},
		{key: "registry.branch", value: "main~1", want: "revision-expression syntax"},
		{key: "registry.branch", value: "refs/heads/main", want: "without the refs/ prefix"},
	}

	for _, tt := range tests {
		t.Run(tt.key+" "+tt.value, func(t *testing.T) {
			cacheDir := filepath.Join(t.TempDir(), "cache")
			cfg := git.Config{
				Remote:   "file:///does-not-matter",
				Branch:   "main",
				CacheDir: cacheDir,
			}
			if tt.key == "registry.branch" {
				cfg.Branch = tt.value
			} else {
				cfg.Ref = tt.value
			}
			_, err := git.NewWithPull(context.Background(), cfg)
			if err == nil {
				t.Fatalf("expected %s %q to be refused", tt.key, tt.value)
			}
			for _, want := range []string{tt.key, tt.value, tt.want} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error does not mention %q:\n%v", want, err)
				}
			}
			// Refused before the network, so nothing was cloned.
			if _, statErr := os.Stat(cacheDir); !os.IsNotExist(statErr) {
				t.Errorf("%s exists: an unusable value must be refused before any clone", cacheDir)
			}
		})
	}
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

// isShallow reports whether dir is a shallow clone. The ordinary ref-less
// pull produces one, which is why shallowness cannot be the predicate for
// "a re-clone would help".
func isShallow(t *testing.T, dir string) bool {
	t.Helper()
	return gitOutput(t, dir, "rev-parse", "--is-shallow-repository") == "true"
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

// tagSHA returns the commit a tag in dir points at, peeled, or "" when the
// tag is not in dir at all. Used to assert that a cached tag did not follow
// an upstream move.
func tagSHA(t *testing.T, dir, name string) string {
	t.Helper()
	out, err := testutil.GitCommand(context.Background(), dir,
		"rev-parse", "--verify", "--quiet", "refs/tags/"+name+"^{commit}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// commitCount is how many commits dir holds, which is how a fetch that
// deepens a shallow cache shows up.
func commitCount(t *testing.T, dir string) int {
	t.Helper()
	n, err := strconv.Atoi(gitOutput(t, dir, "rev-list", "--count", "--all"))
	if err != nil {
		t.Fatalf("parse commit count in %s: %v", dir, err)
	}
	return n
}

// hasCommit reports whether dir holds the commit object sha.
func hasCommit(t *testing.T, dir, sha string) bool {
	t.Helper()
	return testutil.GitCommand(context.Background(), dir, "cat-file", "-e", sha+"^{commit}").Run() == nil
}
