package testutil_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/dejanradmanovic/event-spec/testutil"
)

// gitEnvFixed is the set of GIT_* keys GitEnv sets to a constant value.
var gitEnvFixed = map[string]string{
	"GIT_CONFIG_NOSYSTEM": "1",
	"GIT_TERMINAL_PROMPT": "0",
	"GIT_PAGER":           "cat",
	"GIT_EDITOR":          "false",
	"GIT_AUTHOR_NAME":     "Test",
	"GIT_AUTHOR_EMAIL":    "test@test.com",
	"GIT_COMMITTER_NAME":  "Test",
	"GIT_COMMITTER_EMAIL": "test@test.com",
}

// gitEnvConfigPaths are the GIT_* keys pointed at the private per-process
// config directory. Their values are random, so tests assert their shape.
var gitEnvConfigPaths = []string{"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM"}

func skipIfNoGit(tb testing.TB) {
	tb.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		tb.Skip("git not available")
	}
}

// gitOutput runs git through the helper under test and returns trimmed stdout.
func gitOutput(tb testing.TB, dir string, args ...string) string {
	tb.Helper()
	out, err := testutil.GitCommand(context.Background(), dir, args...).CombinedOutput()
	if err != nil {
		tb.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// initRepo creates a git repository with a single commit at dir.
func initRepo(tb testing.TB, dir, content string) {
	tb.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatal(err)
	}
	testutil.RunGit(tb, dir, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte(content), 0o644); err != nil {
		tb.Fatal(err)
	}
	testutil.RunGit(tb, dir, "add", ".")
	testutil.RunGit(tb, dir, "commit", "-m", content)
}

func TestGitEnv_DropsInheritedGitVariables(t *testing.T) {
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "evil", ".git"))
	t.Setenv("GIT_WORK_TREE", t.TempDir())
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "index"))
	t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(t.TempDir(), "objects"))
	t.Setenv("GIT_PREFIX", "sub/")

	got := map[string]string{}
	for _, kv := range testutil.GitEnv() {
		key, value, ok := strings.Cut(kv, "=")
		key = strings.ToUpper(key)
		if !ok || !strings.HasPrefix(key, "GIT_") {
			continue
		}
		_, fixed := gitEnvFixed[key]
		if !fixed && !slices.Contains(gitEnvConfigPaths, key) {
			// Print the key only: a leaked GIT_CONFIG_VALUE_n can hold a
			// credential, and the value is never needed for the diagnosis.
			t.Errorf("GitEnv leaked inherited variable %s", key)
			continue
		}
		got[key] = value
	}

	for key, want := range gitEnvFixed {
		if got[key] != want {
			t.Errorf("GitEnv()[%s] = %q, want %q", key, got[key], want)
		}
	}
	for _, key := range gitEnvConfigPaths {
		if dir := filepath.Dir(got[key]); dir != testutil.GitConfigDir() {
			t.Errorf("GitEnv()[%s] lives in %s, want it under %s", key, dir, testutil.GitConfigDir())
		}
	}
}

// TestGitEnv_ConfigDirIsPrivate guards the fix for the planted-config attack:
// the files GIT_CONFIG_GLOBAL/GIT_CONFIG_SYSTEM name must sit in a directory
// no other local user can pre-create, not at a fixed path under os.TempDir().
func TestGitEnv_ConfigDirIsPrivate(t *testing.T) {
	dir := testutil.GitConfigDir()

	if dir == os.TempDir() || filepath.Dir(dir) != filepath.Clean(os.TempDir()) {
		t.Errorf("config dir %s is not a private child of %s", dir, os.TempDir())
	}
	for _, stale := range []string{
		filepath.Join(os.TempDir(), "event-spec-test-gitconfig-global"),
		filepath.Join(os.TempDir(), "event-spec-test-gitconfig-system"),
	} {
		if strings.HasPrefix(gitEnvValue(t, "GIT_CONFIG_GLOBAL"), stale) {
			t.Errorf("GIT_CONFIG_GLOBAL still uses the predictable path %s", stale)
		}
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("config dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", dir)
	}
	// os.MkdirTemp creates 0700. Windows does not model POSIX bits, so the
	// assertion only runs where the mode is meaningful.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Errorf("config dir mode = %#o, want 0700", perm)
		}
	}
}

// TestGitEnv_ConfigPathIsUnpredictableAcrossProcesses re-execs this test
// binary and compares the config path the child computes with this process's.
// A constant path would make them equal, which is exactly what an attacker
// needs in order to plant the file ahead of time.
func TestGitEnv_ConfigPathIsUnpredictableAcrossProcesses(t *testing.T) {
	if os.Getenv(gitEnvChildEnv) == "1" {
		fmt.Println(gitEnvChildMarker + gitEnvValue(t, "GIT_CONFIG_GLOBAL"))
		return
	}

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
	cmd.Env = append(os.Environ(), gitEnvChildEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child test process: %v\n%s", err, out)
	}

	var child string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, gitEnvChildMarker); ok {
			child = after
			break
		}
	}
	if child == "" {
		t.Fatalf("child did not report a config path:\n%s", out)
	}
	if parent := gitEnvValue(t, "GIT_CONFIG_GLOBAL"); child == parent {
		t.Errorf("two processes computed the same config path %s; it must be unpredictable", child)
	}
}

const (
	gitEnvChildEnv    = "EVENT_SPEC_GITENV_CHILD"
	gitEnvChildMarker = "GITENV-CONFIG-GLOBAL="
)

// gitEnvValue returns the value GitEnv assigns to key.
func gitEnvValue(tb testing.TB, key string) string {
	tb.Helper()
	for _, kv := range testutil.GitEnv() {
		if k, v, ok := strings.Cut(kv, "="); ok && strings.EqualFold(k, key) {
			return v
		}
	}
	tb.Fatalf("GitEnv has no %s", key)
	return ""
}

// TestGitEnv_DropsLowercaseGitKeys covers the documented reason isGitEnvKey
// upper-cases before matching: on Windows "git_dir" and "GIT_DIR" are the same
// variable, so the filter must not be case-sensitive.
func TestGitEnv_DropsLowercaseGitKeys(t *testing.T) {
	t.Setenv("git_dir", filepath.Join(t.TempDir(), "lower", ".git"))
	t.Setenv("Git_Work_Tree", t.TempDir())

	for _, kv := range testutil.GitEnv() {
		key, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		switch strings.ToUpper(key) {
		case "GIT_DIR", "GIT_WORK_TREE":
			t.Errorf("GitEnv kept %s, the prefix match is case-sensitive", key)
		}
	}
}

func TestGitEnv_KeepsNonGitVariables(t *testing.T) {
	t.Setenv("EVENT_SPEC_GITENV_PROBE", "kept")

	if !slices.Contains(testutil.GitEnv(), "EVENT_SPEC_GITENV_PROBE=kept") {
		t.Error("GitEnv dropped a non-git variable")
	}
}

func TestGitCommand_SetsDirAndEnv(t *testing.T) {
	dir := t.TempDir()
	cmd := testutil.GitCommand(context.Background(), dir, "status")

	if cmd.Dir != dir {
		t.Errorf("cmd.Dir = %s, want %s", cmd.Dir, dir)
	}
	if cmd.Env == nil {
		t.Error("cmd.Env is nil, want the scrubbed environment")
	}
	if len(cmd.Args) != 2 || cmd.Args[1] != "status" {
		t.Errorf("cmd.Args = %v, want [git status]", cmd.Args)
	}
}

// TestGitCommand_IgnoresInheritedGitDir is the regression test for #184: a
// fixture repository built through the helper must not touch the repository
// GIT_DIR points at, the way a git hook's environment points it at the
// contributor's own checkout.
func TestGitCommand_IgnoresInheritedGitDir(t *testing.T) {
	skipIfNoGit(t)

	scratch := filepath.Join(t.TempDir(), "scratch")
	initRepo(t, scratch, "scratch")

	scratchGitDir := filepath.Join(scratch, ".git")
	headBefore := gitOutput(t, scratch, "rev-parse", "HEAD")
	configBefore, err := os.ReadFile(filepath.Join(scratchGitDir, "config"))
	if err != nil {
		t.Fatal(err)
	}

	// Reproduce the hook environment.
	t.Setenv("GIT_DIR", scratchGitDir)

	fixture := filepath.Join(t.TempDir(), "fixture")
	initRepo(t, fixture, "fixture")
	// A "config --local" must stay local to the fixture, too.
	testutil.RunGit(t, fixture, "config", "--local", "user.email", "fixture@example.com")

	if head := gitOutput(t, scratch, "rev-parse", "HEAD"); head != headBefore {
		t.Errorf("scratch HEAD moved: %s -> %s", headBefore, head)
	}
	if status := gitOutput(t, scratch, "status", "--porcelain"); status != "" {
		t.Errorf("scratch working tree dirtied:\n%s", status)
	}
	configAfter, err := os.ReadFile(filepath.Join(scratchGitDir, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(configBefore, configAfter) {
		t.Errorf("scratch .git/config changed:\nbefore:\n%s\nafter:\n%s", configBefore, configAfter)
	}

	// The fixture repo is real, and separate.
	if head := gitOutput(t, fixture, "rev-parse", "HEAD"); head == headBefore {
		t.Error("fixture commit landed in the scratch repository")
	}
}

// TestGitCommand_IgnoresInheritedGitDirAndWorkTree repeats the regression with
// the variable pair git exports together for several hooks. It asserts the
// scratch repo is untouched; note that unscrubbed on Windows this fails safe
// (git refuses the commit) rather than reproducing the destructive outcome the
// issue saw, because the inherited GIT_DIR here is a backslash path built by
// filepath.Join. The scrub drops the variable by name either way.
func TestGitCommand_IgnoresInheritedGitDirAndWorkTree(t *testing.T) {
	skipIfNoGit(t)

	scratch := filepath.Join(t.TempDir(), "scratch")
	initRepo(t, scratch, "scratch")

	headBefore := gitOutput(t, scratch, "rev-parse", "HEAD")
	treeBefore := gitOutput(t, scratch, "ls-files")

	t.Setenv("GIT_DIR", filepath.Join(scratch, ".git"))
	t.Setenv("GIT_WORK_TREE", scratch)

	fixture := filepath.Join(t.TempDir(), "fixture")
	initRepo(t, fixture, "fixture")
	// A "config --local" must stay local to the fixture, too.
	testutil.RunGit(t, fixture, "config", "--local", "user.email", "fixture@example.com")

	if head := gitOutput(t, scratch, "rev-parse", "HEAD"); head != headBefore {
		t.Errorf("scratch HEAD moved: %s -> %s", headBefore, head)
	}
	if tree := gitOutput(t, scratch, "ls-files"); tree != treeBefore {
		t.Errorf("scratch tracked files changed: %q -> %q", treeBefore, tree)
	}
	if status := gitOutput(t, scratch, "status", "--porcelain"); status != "" {
		t.Errorf("scratch working tree dirtied:\n%s", status)
	}
}

// TestRunGit_CommitsWithoutConfiguredIdentity proves the identity comes from
// GitEnv: with GIT_CONFIG_NOSYSTEM/GLOBAL/SYSTEM redirected there is no
// user.name anywhere, and the commit must still succeed.
func TestRunGit_CommitsWithoutConfiguredIdentity(t *testing.T) {
	skipIfNoGit(t)

	dir := filepath.Join(t.TempDir(), "repo")
	initRepo(t, dir, "identity")

	// "config --get" exits 1 when the key is unset, so the error is expected
	// here; only the output matters.
	out, _ := testutil.GitCommand(context.Background(), dir, "config", "--get", "user.name").CombinedOutput()
	if name := strings.TrimSpace(string(out)); name != "" {
		t.Fatalf("expected no configured user.name, got %q", name)
	}
	if author := gitOutput(t, dir, "log", "-1", "--format=%an <%ae>"); author != "Test <test@test.com>" {
		t.Errorf("author = %q, want %q", author, "Test <test@test.com>")
	}
}

func TestRunGit_ReportsCombinedOutput(t *testing.T) {
	skipIfNoGit(t)

	tb := &recordingTB{}
	testutil.RunGit(tb, t.TempDir(), "rev-parse", "--verify", "definitely-not-a-ref")

	if !tb.failed {
		t.Fatal("RunGit did not fail on a failing git command")
	}
	if !strings.Contains(tb.msg, "rev-parse") {
		t.Errorf("failure message missing the arguments: %s", tb.msg)
	}
	if !strings.Contains(tb.msg, "definitely-not-a-ref") && !strings.Contains(tb.msg, "fatal") {
		t.Errorf("failure message missing git's combined output: %s", tb.msg)
	}
}

// recordingTB captures a single Fatalf instead of aborting the goroutine, so
// RunGit's failure path can be asserted. Only the methods RunGit calls are
// implemented; anything else panics on the nil embedded TB.
type recordingTB struct {
	testing.TB

	failed bool
	msg    string
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.failed = true
	r.msg = fmt.Sprintf(format, args...)
}
