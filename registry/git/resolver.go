// Package git implements a Registry backed by a remote git repository.
// Pull clones or fetches the remote into a local cache; all subsequent
// registry operations read from that cache via the local package.
package git

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/registry/local"
	"github.com/dejanradmanovic/event-spec/spec"
)

// Config configures the remote git registry.
type Config struct {
	// Remote is the git clone URL of the shared tracking-plan repository.
	Remote string
	// Branch to track. Defaults to "main".
	Branch string
	// Ref is an optional commit SHA or tag to pin to. When empty the HEAD of
	// Branch is used. On each pull the cache is reset to this exact ref.
	Ref string
	// CacheDir is the local directory where the repo is cloned.
	// Defaults to ~/.event-spec/cache/<sha12-of-Remote>.
	CacheDir string
	// SpecsDir is the path to the specs directory within the remote repo.
	// Defaults to "specs".
	SpecsDir string
	// SourcesDir and DestinationsDir are always local to the consuming repo.
	SourcesDir      string
	DestinationsDir string
	// Force removes CacheDir before cloning, forcing a fresh clone.
	// Useful after a remote force-push or corrupted cache.
	Force bool
}

func (c Config) branch() string {
	if c.Branch != "" {
		return c.Branch
	}
	return "main"
}

func (c Config) specsDir() string {
	if c.SpecsDir != "" {
		return c.SpecsDir
	}
	return "specs"
}

func (c Config) cacheDir() string {
	if c.CacheDir != "" {
		return c.CacheDir
	}
	home, _ := os.UserHomeDir()
	h := sha256.Sum256([]byte(c.Remote))
	return filepath.Join(home, ".event-spec", "cache", fmt.Sprintf("%x", h[:6]))
}

// Resolver is the remote-git Registry implementation.
// It delegates all reads to an inner local.Resolver built from the cache.
// Pull must be called at least once before the first use.
type Resolver struct {
	cfg Config

	mu    sync.RWMutex
	inner *local.Resolver
}

// New creates a Resolver and loads the index from the local cache.
// Returns an error if the cache does not exist — run event-spec pull first.
func New(cfg Config) (*Resolver, error) {
	r := &Resolver{cfg: cfg}
	if err := r.loadFromCache(); err != nil {
		return nil, err
	}
	return r, nil
}

// NewWithPull creates a Resolver, calls Pull to clone or update the cache,
// then loads the index. Use this when you want a single call that handles
// first-time setup (e.g. in the event-spec pull CLI command).
func NewWithPull(ctx context.Context, cfg Config) (*Resolver, error) {
	r := &Resolver{cfg: cfg}
	if err := r.Pull(ctx); err != nil {
		return nil, err
	}
	return r, nil
}

// Pull clones the remote repository on first call, or fetches and resets to
// the configured branch/ref on subsequent calls. After a successful pull the
// in-memory index is rebuilt from the updated cache.
func (r *Resolver) Pull(ctx context.Context) error {
	cacheDir := r.cfg.cacheDir()

	if err := os.MkdirAll(filepath.Dir(cacheDir), 0o755); err != nil {
		return fmt.Errorf("create cache parent dir: %w", err)
	}

	if r.cfg.Force {
		if err := os.RemoveAll(cacheDir); err != nil {
			return fmt.Errorf("remove cache for force re-clone: %w", err)
		}
	}

	if _, err := os.Stat(filepath.Join(cacheDir, ".git")); os.IsNotExist(err) {
		if err := r.runGit(ctx, "clone", "--depth", "1", "--branch", r.cfg.branch(), r.cfg.Remote, cacheDir); err != nil {
			return fmt.Errorf("git clone: %w", err)
		}
	} else {
		if err := r.runGit(ctx, "-C", cacheDir, "fetch", "origin"); err != nil {
			return fmt.Errorf("git fetch: %w", err)
		}
		ref := r.cfg.Ref
		if ref == "" {
			ref = "origin/" + r.cfg.branch()
		}
		if err := r.runGit(ctx, "-C", cacheDir, "checkout", ref); err != nil {
			return fmt.Errorf("git checkout %s: %w", ref, err)
		}
	}

	return r.loadFromCache()
}

// loadFromCache builds the inner local.Resolver from the cache directory.
func (r *Resolver) loadFromCache() error {
	cacheDir := r.cfg.cacheDir()
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return fmt.Errorf("git cache not found at %s: run 'event-spec pull' first", cacheDir)
	}
	inner, err := local.New(local.Config{
		SpecsDir:        filepath.Join(cacheDir, r.cfg.specsDir()),
		SourcesDir:      r.cfg.SourcesDir,
		DestinationsDir: r.cfg.DestinationsDir,
	})
	if err != nil {
		return fmt.Errorf("load from cache: %w", err)
	}
	r.mu.Lock()
	r.inner = inner
	r.mu.Unlock()
	return nil
}

// execVarFix pairs a variable gitEnv removes with the git config setting that
// replaces it. It is the single source for both halves of the migration
// guidance: strippedExecVarsHint renders it into the terminal, and
// docs_test.go pins it against the table in docs/docs/registry/git.md, so
// neither side can drift from the other without failing the build.
type execVarFix struct {
	// name is the environment variable gitEnv removes.
	name string
	// config is the git config key that replaces it, empty when git has none.
	config string
	// value is a placeholder for the suggested command's argument. Only ever
	// a placeholder: the caller's own value may be a credential.
	value string
	// caveat is a second line qualifying config, or, when config is empty,
	// what to do instead of configuring anything.
	caveat string
	// docsPhrase is the wording the migration guide must carry for this
	// variable beyond the config key itself; empty when the key says it all.
	docsPhrase string
}

// execVarFixes are the variables gitEnv removes that let the environment
// choose a program for git to run, each with its replacement. They are listed
// separately from the rest of the GIT_* scrub so a failing git invocation can
// tell the user which one of theirs was dropped and what to use instead,
// rather than surfacing an unexplained "Permission denied (publickey)".
//
// The git config equivalents are git's own, from git-config(1): core.gitProxy
// is documented as overridden by GIT_PROXY_COMMAND but applies to the git://
// protocol only, and diff.external as overridden by GIT_EXTERNAL_DIFF.
// GIT_EXEC_PATH has no config key at all: git takes it only as the
// --exec-path option, which event-spec never passes, so its entry carries no
// config key and says as much.
var execVarFixes = []execVarFix{
	{
		name:   "GIT_SSH",
		config: "core.sshCommand",
		value:  `"ssh -i /path/to/key"`,
	},
	{
		name:   "GIT_SSH_COMMAND",
		config: "core.sshCommand",
		value:  `"ssh -i /path/to/key"`,
	},
	{
		name:   "GIT_ASKPASS",
		config: "core.askPass",
		value:  "/path/to/askpass",
	},
	{
		name:       "GIT_PROXY_COMMAND",
		config:     "core.gitProxy",
		value:      "/path/to/proxy",
		caveat:     "git:// remotes only; an HTTPS remote needs http.proxy",
		docsPhrase: "http.proxy",
	},
	{
		name:       "GIT_EXTERNAL_DIFF",
		config:     "diff.external",
		value:      "/path/to/diff",
		caveat:     "never affects a pull, which only runs clone, fetch and checkout",
		docsPhrase: "never affects a pull",
	},
	{
		name:       "GIT_EXEC_PATH",
		caveat:     "no git config equivalent: put your git first on PATH",
		docsPhrase: "no git config equivalent",
	},
	{
		name:       "GIT_CONFIG_COUNT",
		config:     "<key>",
		value:      "<value>",
		caveat:     "if <value> is a credential, use credential.helper instead",
		docsPhrase: "credential.helper",
	},
}

// strippedExecVars are the names in execVarFixes, in the same order. Derived
// rather than written out so the list the error names can never include a
// variable the error has no fix for.
var strippedExecVars = execVarFixNames()

func execVarFixNames() []string {
	names := make([]string, len(execVarFixes))
	for i, fix := range execVarFixes {
		names[i] = fix.name
	}
	return names
}

// gitEnv returns os.Environ() with every GIT_* variable removed, plus two
// hardening defaults.
//
// Git exports GIT_DIR, GIT_WORK_TREE and GIT_INDEX_FILE into every hook's
// environment, and GIT_DIR overrides -C, so a pull triggered from inside a
// hook would otherwise clone into the caller's own repository instead of the
// cache directory. Variables must be dropped, not blanked: git treats
// GIT_DIR="" as set. testutil/gitenv.go holds the canonical version of this
// list for tests.
//
// The filter is an allowlist in the right direction: it also removes the
// variables that name a program for git to execute (GIT_SSH_COMMAND,
// GIT_ASKPASS, GIT_PROXY_COMMAND, GIT_EXTERNAL_DIFF, GIT_EXEC_PATH,
// GIT_CONFIG_KEY_n=core.hooksPath), which matters because event-spec runs
// inside hooks, make targets and CI steps, where the environment is often set
// by someone other than the user. The supported way to configure SSH and
// credentials is the user's own git config — core.sshCommand,
// credential.helper, url.*.insteadOf — which this function deliberately leaves
// alone by not redirecting GIT_CONFIG_GLOBAL. runGit names the dropped
// variables in its error when a command fails, and docs/docs/registry/git.md
// documents the migration.
//
// Two values are then added back: GIT_TERMINAL_PROMPT=0 so a pull fails fast
// instead of blocking on a credential prompt in a batch context, and a
// conservative GIT_ALLOW_PROTOCOL, which keeps a hardened caller's protocol
// restriction from being lost with the rest of the environment and blocks the
// code-executing ext:: transport for a Remote taken from event-spec.yaml.
func gitEnv() []string {
	base := os.Environ()
	env := make([]string, 0, len(base)+2)
	for _, kv := range base {
		if key, _, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(strings.ToUpper(key), "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ALLOW_PROTOCOL=file:git:http:https:ssh",
	)
}

// strippedExecVarsInEnv returns the names of the command-execution variables
// present in the caller's environment, which gitEnv has removed. Only names
// are returned: a value such as GIT_CONFIG_VALUE_n can hold a credential.
func strippedExecVarsInEnv() []string {
	var found []string
	for _, key := range strippedExecVars {
		if _, ok := os.LookupEnv(key); ok {
			found = append(found, key)
		}
	}
	return found
}

// gitDocsURL is the published form of docs/docs/registry/git.md, which holds
// the full migration table. docs_test.go pins the two together, so moving the
// page fails the build instead of leaving a dead pointer in a user's terminal.
const gitDocsURL = "https://event-spec.io/docs/registry/git"

// strippedExecVarsHint renders the diagnostic runGit appends to a failed git
// command when the caller had command-execution variables set. It is built one
// line at a time and joined with "\n" rather than wrapped at display time:
// event-spec has no way to know the console width, and a pull that fails in a
// git hook or a CI log is often read on something that does not wrap at all.
// Each line stays under 80 runes, which is why every variable gets its own
// lines: the worst case is all of execVarFixes set at once.
//
// The replacement shown is the one execVarFixes records for that variable, not
// a single stock example, because five of the seven have nothing to do with
// SSH and GIT_EXEC_PATH has no git config equivalent to offer at all. Names
// only, never values, for the same reason as strippedExecVarsInEnv.
func strippedExecVarsHint(stripped []string) string {
	lines := []string{
		"event-spec deliberately removed the following variables from git's",
		"environment, so a hook, make target or CI step cannot choose which",
		"program git runs, and what to use instead:",
	}
	for _, name := range stripped {
		lines = append(lines, "  "+name)
		fix, ok := lookupExecVarFix(name)
		if !ok {
			// Unreachable from strippedExecVarsInEnv, which only returns
			// names from execVarFixes; kept so a variable added to the scrub
			// without a fix degrades to the table rather than to silence.
			lines = append(lines, "    see "+gitDocsURL)
			continue
		}
		if fix.config != "" {
			lines = append(lines, "    git config --global "+fix.config+" "+fix.value)
		}
		if fix.caveat != "" {
			lines = append(lines, "    "+fix.caveat)
		}
	}
	return strings.Join(append(lines,
		"the full table, including the HTTPS cases, is at",
		"  "+gitDocsURL,
	), "\n")
}

// lookupExecVarFix returns the recorded fix for an environment variable name.
func lookupExecVarFix(name string) (execVarFix, bool) {
	for _, fix := range execVarFixes {
		if fix.name == name {
			return fix, true
		}
	}
	return execVarFix{}, false
}

func (r *Resolver) runGit(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = gitEnv()
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if stripped := strippedExecVarsInEnv(); len(stripped) > 0 {
			// The blank line is the seam between git's own output, which
			// cmd.Stderr has already written, and event-spec's advice; the
			// hint's opening clause names the voice from there on.
			return fmt.Errorf("%w\n\n%s", err, strippedExecVarsHint(stripped))
		}
		return err
	}
	return nil
}

func (r *Resolver) snapshot() *local.Resolver {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.inner
}

// ListEvents implements Registry by delegating to the cached local resolver.
func (r *Resolver) ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error) {
	return r.snapshot().ListEvents(ctx, filter)
}

// ListAllEvents implements Registry by delegating to the cached local resolver.
func (r *Resolver) ListAllEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error) {
	return r.snapshot().ListAllEvents(ctx, filter)
}

// GetEvent implements Registry by delegating to the cached local resolver.
func (r *Resolver) GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error) {
	return r.snapshot().GetEvent(ctx, namespace, name, version)
}

// GetSource implements Registry by delegating to the cached local resolver.
func (r *Resolver) GetSource(ctx context.Context, name string) (*spec.SourceDef, error) {
	return r.snapshot().GetSource(ctx, name)
}

// GetDestination implements Registry by delegating to the cached local resolver.
func (r *Resolver) GetDestination(ctx context.Context, name string) (*spec.DestinationDef, error) {
	return r.snapshot().GetDestination(ctx, name)
}

// PublishEvent always returns ErrReadOnly. Publish by committing to the shared
// tracking-plan repository and running event-spec pull to sync the cache.
func (r *Resolver) PublishEvent(_ context.Context, _ spec.EventDef) error {
	return registry.ErrReadOnly
}

// Diff implements Registry by delegating to the cached local resolver.
func (r *Resolver) Diff(ctx context.Context, namespace, name, from, to string) ([]spec.Change, error) {
	return r.snapshot().Diff(ctx, namespace, name, from, to)
}
