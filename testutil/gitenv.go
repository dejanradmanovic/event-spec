package testutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitEnvPrefix is the prefix shared by every environment variable git reads
// for its own configuration, including the ones it exports into hooks.
const gitEnvPrefix = "GIT_"

// GitEnv returns a hermetic environment for exec'ing git: os.Environ() with
// every GIT_* variable removed, plus a fixed identity and redirected config
// files.
//
// Dropping GIT_* is what makes an exec'd git safe inside a git hook. Git
// exports GIT_DIR, GIT_WORK_TREE and GIT_INDEX_FILE into every hook's
// environment, and GIT_DIR overrides both cmd.Dir and -C, so a test fixture
// that runs "init", "add ." and "commit" against a temp directory would
// otherwise operate on the contributor's real repository.
//
// The filter is deliberately broad, so it also removes transport variables
// such as GIT_SSH_COMMAND, GIT_ASKPASS and GIT_EXEC_PATH. No test in this
// repository clones over the network — every fixture remote is a local path —
// so nothing here depends on them.
//
// GIT_CONFIG_GLOBAL and GIT_CONFIG_SYSTEM keep a "git config" that forgot
// --local from escaping into the contributor's ~/.gitconfig. They require
// git >= 2.32.
func GitEnv() []string {
	base := os.Environ()
	env := make([]string, 0, len(base)+8)
	for _, kv := range base {
		if key, _, ok := strings.Cut(kv, "="); ok && isGitEnvKey(key) {
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+filepath.Join(os.TempDir(), "event-spec-test-gitconfig-global"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(os.TempDir(), "event-spec-test-gitconfig-system"),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
}

// isGitEnvKey reports whether key names a git environment variable. Windows
// environment variable names are case-insensitive, so the prefix match is too.
func isGitEnvKey(key string) bool {
	return strings.HasPrefix(strings.ToUpper(key), gitEnvPrefix)
}

// GitCommand returns an *exec.Cmd running "git args..." in dir with GitEnv
// applied. Every test that execs git must go through this helper; the
// TestNoUnguardedGitExec guard test enforces it.
func GitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = GitEnv()
	return cmd
}

// RunGit runs "git args..." in dir through GitCommand and calls tb.Fatalf with
// the combined output if git exits non-zero.
func RunGit(tb testing.TB, dir string, args ...string) {
	tb.Helper()
	out, err := GitCommand(context.Background(), dir, args...).CombinedOutput()
	if err != nil {
		tb.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
