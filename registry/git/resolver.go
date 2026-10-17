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
	"regexp"
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
	// Branch to track. Defaults to "main". Each pull moves the cache to the
	// remote tip of this branch, never to a local branch of the same name.
	Branch string
	// Ref optionally overrides Branch with an exact commit SHA or tag to pin
	// to. It is resolved against the remote — in order, refs/tags/<Ref>,
	// refs/remotes/origin/<Ref>, then <Ref> as a commit SHA — so a Ref naming
	// a branch follows that branch instead of freezing on the local copy a
	// clone left behind (#211), while a tag still wins over a branch of the
	// same name, as it does in git. A tag and a branch of one name at
	// different commits is refused rather than guessed at. Setting Ref also
	// makes the first clone a full one, since a shallow clone holds neither an
	// older commit nor any tag.
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

// Pull clones the remote repository on first call, or fetches on subsequent
// calls, then checks the cache out at the configured branch or ref, resolved
// against the remote and always left detached. After a successful pull the
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
		if err := r.runGit(ctx, r.cloneArgs(cacheDir)...); err != nil {
			return fmt.Errorf("git clone: %w", err)
		}
	} else {
		if err := r.runGit(ctx, "-C", cacheDir, "fetch", "origin"); err != nil {
			return fmt.Errorf("git fetch: %w", err)
		}
	}

	// Both paths end the same way, so the cache is a detached snapshot of a
	// remote-resolved revision whether it was just cloned or just fetched.
	rev, err := r.resolveCheckoutRef(ctx, cacheDir)
	if err != nil {
		return err
	}
	// --detach is also what keeps git's detached-HEAD advice off the user's
	// terminal: git prints that paragraph when a plain checkout of a commit
	// leaves a branch behind, never when the detaching was asked for.
	if err := r.runGit(ctx, "-C", cacheDir, "checkout", "--detach", rev); err != nil {
		return fmt.Errorf("git checkout %s: %w", rev, err)
	}

	return r.loadFromCache()
}

// cloneArgs builds the argv for the first clone. --depth 1 is used only when
// no Ref is configured: a shallow clone contains just the tip of the tracked
// branch, which is exactly what a pin to an older commit or to a tag outside
// that branch is not, so a pinned config pays for a full clone instead of
// failing to resolve its own pin on a fresh cache.
func (r *Resolver) cloneArgs(cacheDir string) []string {
	args := []string{"clone"}
	if r.cfg.Ref == "" {
		args = append(args, "--depth", "1")
	}
	return append(args, "--branch", r.cfg.branch(), r.cfg.Remote, cacheDir)
}

// refCandidate is one revision resolveCheckoutRef probes for, together with
// what the cache resolved it to.
type refCandidate struct {
	// kind labels the candidate in an error: tag, branch or commit.
	kind string
	// rev is the revision handed to git, fully qualified wherever possible so
	// that a bare name cannot be re-interpreted on the way.
	rev string
	// sha is the commit rev resolved to, empty when it did not resolve.
	sha string
	// skip is why the candidate was not probed at all, empty when it was.
	skip string
}

// shaRe matches a full or abbreviated commit SHA, upper or lower case, both
// of which git resolves. A Ref is only tried as a commit when it looks like
// one: resolved bare against the cache, "HEAD" or "refs/heads/main" would
// name the stale local branch a clone leaves behind — the very ref #211 is
// about — through a spelling that never came from the remote.
var shaRe = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

// resolveCheckoutRef returns the revision Pull checks out, always a
// remote-resolved one. A bare local name must never be used: the local branch
// a clone creates is not advanced by a fetch, so checking out "main" pinned
// the cache to the first clone forever (#211).
//
// With no Ref the tracked branch's remote-tracking ref is used. With a Ref the
// candidates are refs/tags/<Ref>, then refs/remotes/origin/<Ref>, then <Ref>
// as a commit. The tag comes first because that is git's own precedence for a
// bare name (gitrevisions puts refs/tags before refs/heads and refs/remotes),
// which the bare "checkout <Ref>" this function replaced inherited; preferring
// the branch would let anyone able to push a branch named like a tag redirect
// every consumer pinned to that tag. All candidates are probed instead of
// stopping at the first hit, so a tag and a branch of one name pointing at
// different commits is reported rather than silently resolved.
func (r *Resolver) resolveCheckoutRef(ctx context.Context, cacheDir string) (string, error) {
	shallow := gitIsShallow(ctx, cacheDir)

	if r.cfg.Ref == "" {
		branch := refCandidate{kind: "branch", rev: "refs/remotes/origin/" + r.cfg.branch()}
		branch.sha, _ = gitResolveCommit(ctx, cacheDir, branch.rev)
		if branch.sha == "" {
			return "", unresolvedRefError{
				key:        "registry.branch",
				value:      r.cfg.branch(),
				remote:     r.cfg.Remote,
				candidates: []refCandidate{branch},
				shallow:    shallow,
			}
		}
		return branch.rev, nil
	}

	tag := refCandidate{kind: "tag", rev: "refs/tags/" + r.cfg.Ref}
	branch := refCandidate{kind: "branch", rev: "refs/remotes/origin/" + r.cfg.Ref}
	commit := refCandidate{kind: "commit", rev: r.cfg.Ref}
	if !shaRe.MatchString(r.cfg.Ref) {
		commit.skip = "not 7-40 hex digits, so not a commit SHA"
	}
	tag.sha, _ = gitResolveCommit(ctx, cacheDir, tag.rev)
	branch.sha, _ = gitResolveCommit(ctx, cacheDir, branch.rev)
	if commit.skip == "" {
		commit.sha, _ = gitResolveCommit(ctx, cacheDir, commit.rev)
	}

	// A tag and a branch of the same name at different commits make the pin
	// mean two things, and the one it would mean is decided by whoever pushed
	// last. git calls such a name ambiguous; event-spec refuses it.
	if tag.sha != "" && branch.sha != "" && tag.sha != branch.sha {
		return "", ambiguousRefError{ref: r.cfg.Ref, remote: r.cfg.Remote, tag: tag, branch: branch}
	}

	candidates := []refCandidate{tag, branch, commit}
	for _, c := range candidates {
		if c.sha != "" {
			return c.rev, nil
		}
	}
	return "", unresolvedRefError{
		key:        "registry.ref",
		value:      r.cfg.Ref,
		remote:     r.cfg.Remote,
		candidates: candidates,
		shallow:    shallow,
	}
}

// unresolvedRefError is returned when none of the candidates for a configured
// registry.ref or registry.branch names a commit in the cache.
type unresolvedRefError struct {
	// key is the config key at fault, registry.ref or registry.branch.
	key string
	// value is the key's configured value.
	value string
	// remote is the clone URL, so the message says which remote was searched.
	remote string
	// candidates are the revisions that were looked for, in probe order.
	candidates []refCandidate
	// shallow records whether the cache is a shallow clone, which is the only
	// state in which re-cloning is the fix.
	shallow bool
}

// Error renders the message one line at a time, exactly as
// strippedExecVarsHint does and for the same reason: event-spec cannot know
// the console width, a pull that fails inside a git hook or a CI log is often
// read somewhere that does not wrap at all, and the one-sentence form of this
// message broke mid-word at 80 columns (#206). Every line of prose stays
// under 80 runes, and every piece of user data — the remote URL, the
// configured value, each ref form — sits at the end of its own line, so the
// only thing a narrow console can ever fold is the datum, never the sentence
// explaining it, and a URL stays in one piece to be copied.
//
// The remedy is the other half of the fix: "re-run with --force" is appended
// only when the cache really is shallow, because for the likeliest cause — a
// typo in the config — a full re-clone lands back on this same error having
// achieved nothing.
func (e unresolvedRefError) Error() string {
	lines := []string{
		"cannot resolve " + e.key + " against the remote",
		"  " + e.key + ": " + e.value,
		"  remote: " + e.remote,
	}
	if len(e.candidates) > 1 {
		lines = append(lines, "looked for, in this order:")
	} else {
		lines = append(lines, "looked for:")
	}
	for _, c := range e.candidates {
		if c.skip != "" {
			lines = append(lines, fmt.Sprintf("  %-7s not tried: %s", c.kind, c.skip))
			continue
		}
		lines = append(lines, fmt.Sprintf("  %-7s %s", c.kind, c.rev))
	}
	lines = append(lines, "check "+e.key+" against what the remote actually has")
	if e.shallow {
		lines = append(lines,
			"this cache is shallow and a fetch never deepens it, so it holds",
			"only the tip of the tracked branch: re-run 'event-spec pull --force'")
	}
	return strings.Join(lines, "\n")
}

// ambiguousRefError is returned when a registry.ref names both a tag and a
// remote branch and the two point at different commits.
type ambiguousRefError struct {
	ref    string
	remote string
	tag    refCandidate
	branch refCandidate
}

// Error is pre-wrapped line by line for the same reasons as
// unresolvedRefError.Error. Both SHAs are named in full so the user can paste
// the one they meant straight into registry.ref, which is also the only
// spelling of a pin that nothing pushed to the remote can shadow.
func (e ambiguousRefError) Error() string {
	return strings.Join([]string{
		"registry.ref is ambiguous on the remote",
		"  registry.ref: " + e.ref,
		"  remote: " + e.remote,
		"a tag and a branch of that name point at different commits:",
		"  tag    " + e.tag.sha,
		"    " + e.tag.rev,
		"  branch " + e.branch.sha,
		"    " + e.branch.rev,
		"event-spec resolves a tag before a branch, as git does, but will not",
		"guess when the two disagree: a branch pushed under a tag's name would",
		"redirect the pin. set registry.ref to the commit SHA you mean",
	}, "\n")
}

// gitProbe runs a read-only git query in the cache and returns its trimmed
// stdout. It is a probe rather than a command, so it deliberately does not go
// through runGit: a non-match is an expected step of resolveCheckoutRef's
// search, and runGit would print git's own complaint to the user's terminal
// and attach the GIT_* migration hint (#204) to something that never failed.
// Stderr is discarded for the same reason. The #184 environment scrub still
// applies, because a probe is a git invocation like any other.
func gitProbe(ctx context.Context, cacheDir string, args ...string) (string, bool) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", cacheDir}, args...)...)
	cmd.Env = gitEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// gitResolveCommit returns the commit SHA rev names in the cache, if any.
// --quiet keeps git silent on the expected non-match, and the ^{commit}
// suffix means a rev beginning with "-" can never be parsed as an option.
func gitResolveCommit(ctx context.Context, cacheDir, rev string) (string, bool) {
	return gitProbe(ctx, cacheDir, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
}

// gitIsShallow reports whether the cache is a shallow clone, which is what
// decides whether re-cloning can help: a fetch never deepens a shallow cache,
// so a commit or tag it never fetched can only be had by cloning again. A git
// too old for the query (pre-2.15) reports false, so the remedy is withheld
// rather than promised on a guess.
func gitIsShallow(ctx context.Context, cacheDir string) bool {
	out, ok := gitProbe(ctx, cacheDir, "rev-parse", "--is-shallow-repository")
	return ok && out == "true"
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
	// Distinctive enough to appear only where this fact is stated, since
	// docs_test.go looks for it anywhere on the page.
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
		docsPhrase: "use `credential.helper`",
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
