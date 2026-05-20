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
	"sync"

	"event-spec/registry"
	"event-spec/registry/local"
	"event-spec/spec"
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

func (r *Resolver) runGit(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %v: %w", args[0], err)
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
