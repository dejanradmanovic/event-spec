package testutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// gitEnvPrefix is the prefix shared by every environment variable git reads
// for its own configuration, including the ones it exports into hooks.
const gitEnvPrefix = "GIT_"

// gitConfigDir is a private, process-scoped directory holding the config files
// GIT_CONFIG_GLOBAL and GIT_CONFIG_SYSTEM point at. Git reads those files, so
// the path must not be guessable: a fixed name under os.TempDir() can be
// pre-created by any other local user on a shared host or self-hosted runner,
// and a planted "[core] hooksPath" would then execute their code during
// "git commit". os.MkdirTemp picks a random name and creates the directory
// 0700, which cannot be pre-created or symlinked into.
var gitConfigDir = sync.OnceValue(func() string {
	dir, err := os.MkdirTemp("", "event-spec-gitenv-")
	if err != nil {
		panic("testutil: create git config dir: " + err.Error())
	}
	return dir
})

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
// GIT_CONFIG_GLOBAL keeps a "git config" that forgot --local from escaping
// into the contributor's ~/.gitconfig; both config variables require
// git >= 2.32. GIT_PAGER and GIT_EDITOR close git's fallback to the
// non-prefixed PAGER/EDITOR/VISUAL variables, which this filter cannot see.
func GitEnv() []string {
	base := os.Environ()
	env := make([]string, 0, len(base)+10)
	for _, kv := range base {
		if key, _, ok := strings.Cut(kv, "="); ok && isGitEnvKey(key) {
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+filepath.Join(gitConfigDir(), "gitconfig-global"),
		// Belt and braces: GIT_CONFIG_NOSYSTEM already stops git reading the
		// system config, so this only matters if that variable is ever dropped.
		"GIT_CONFIG_SYSTEM="+filepath.Join(gitConfigDir(), "gitconfig-system"),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_PAGER=cat",
		"GIT_EDITOR=false",
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
}

// GitConfigDir returns the private directory GitEnv points GIT_CONFIG_GLOBAL
// and GIT_CONFIG_SYSTEM at. It is created on first use and is unique per
// process; tests assert its shape rather than hardcoding a path.
func GitConfigDir() string { return gitConfigDir() }

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
