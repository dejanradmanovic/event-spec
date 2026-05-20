// Package local implements a local-filesystem Registry that walks a specs directory,
// builds an in-memory index, and supports fsnotify hot-reload for dev mode.
package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

// Config configures the git-backed registry paths.
type Config struct {
	SpecsDir        string
	SourcesDir      string
	DestinationsDir string
}

// Resolver is the git-backed Registry implementation.
// All index reads are protected by a read-write mutex so that the fsnotify
// watcher can trigger a full reload without blocking concurrent readers.
type Resolver struct {
	cfg Config

	mu     sync.RWMutex
	events map[string]*spec.EventDef       // "namespace/name/version" → EventDef
	srcs   map[string]*spec.SourceDef      // name → SourceDef
	dsts   map[string]*spec.DestinationDef // name → DestinationDef
}

// New creates a Resolver and performs the initial index load from cfg.SpecsDir.
func New(cfg Config) (*Resolver, error) {
	r := &Resolver{cfg: cfg}
	if err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

// reload rebuilds all three indexes atomically under the write lock.
// Called once on construction and again by the Watcher on every spec change.
func (r *Resolver) reload() error {
	events := make(map[string]*spec.EventDef)
	srcs := make(map[string]*spec.SourceDef)
	dsts := make(map[string]*spec.DestinationDef)

	if r.cfg.SpecsDir != "" {
		defs, errs := spec.WalkEventDefs(r.cfg.SpecsDir)
		if len(errs) > 0 {
			return fmt.Errorf("walking specs dir: %w", errs[0])
		}
		for _, def := range defs {
			events[eventKey(def.Namespace, def.Name, def.Version)] = def
		}
	}

	if err := walkSources(r.cfg.SourcesDir, srcs); err != nil {
		return err
	}
	if err := walkDestinations(r.cfg.DestinationsDir, dsts); err != nil {
		return err
	}

	r.mu.Lock()
	r.events = events
	r.srcs = srcs
	r.dsts = dsts
	r.mu.Unlock()
	return nil
}

// eventKey builds the canonical index key for an event.
func eventKey(namespace, name, version string) string {
	return namespace + "/" + name + "/" + version
}

// ListEvents returns all indexed events that match filter.
func (r *Resolver) ListEvents(_ context.Context, filter registry.ListFilter) ([]spec.EventDef, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []spec.EventDef
	for _, def := range r.events {
		if filter.Namespace != "" && def.Namespace != filter.Namespace {
			continue
		}
		if filter.Status != "" && def.Status != filter.Status {
			continue
		}
		if !containsAll(def.Tags, filter.Tags) {
			continue
		}
		out = append(out, *def)
	}
	return out, nil
}

// GetEvent looks up an event by namespace, name, and version.
// When version is empty it returns the highest SchemaVer with status active.
// Multiple active versions may coexist; callers (e.g. codegen) may request a
// specific version via SourceDef.VersionPinning to support gradual migrations.
func (r *Resolver) GetEvent(_ context.Context, namespace, name, version string) (*spec.EventDef, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if version != "" {
		def, ok := r.events[eventKey(namespace, name, version)]
		if !ok {
			return nil, fmt.Errorf("event %s/%s@%s: %w", namespace, name, version, registry.ErrNotFound)
		}
		return def, nil
	}

	// Find the highest active version.
	var best *spec.EventDef
	var bestVer spec.SchemaVer
	for _, def := range r.events {
		if def.Namespace != namespace || def.Name != name || def.Status != spec.StatusActive {
			continue
		}
		sv, err := spec.ParseSchemaVer(def.Version)
		if err != nil {
			continue
		}
		if best == nil || spec.CompareSchemaVer(sv, bestVer) > 0 {
			best = def
			bestVer = sv
		}
	}
	if best == nil {
		return nil, fmt.Errorf("event %s/%s (active): %w", namespace, name, registry.ErrNotFound)
	}
	return best, nil
}

// GetSource returns the source definition with the given name.
func (r *Resolver) GetSource(_ context.Context, name string) (*spec.SourceDef, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.srcs[name]
	if !ok {
		return nil, fmt.Errorf("source %q: %w", name, registry.ErrNotFound)
	}
	return def, nil
}

// GetDestination returns the destination definition with the given name.
func (r *Resolver) GetDestination(_ context.Context, name string) (*spec.DestinationDef, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.dsts[name]
	if !ok {
		return nil, fmt.Errorf("destination %q: %w", name, registry.ErrNotFound)
	}
	return def, nil
}

// PublishEvent always returns ErrReadOnly. In git mode, publish by committing
// a new YAML file under specs/ and pushing to the shared repository.
func (r *Resolver) PublishEvent(_ context.Context, _ spec.EventDef) error {
	return registry.ErrReadOnly
}

// Diff returns the detected changes between two versions of an event.
func (r *Resolver) Diff(_ context.Context, namespace, name, from, to string) ([]spec.Change, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fromDef, ok := r.events[eventKey(namespace, name, from)]
	if !ok {
		return nil, fmt.Errorf("event %s/%s@%s: %w", namespace, name, from, registry.ErrNotFound)
	}
	toDef, ok := r.events[eventKey(namespace, name, to)]
	if !ok {
		return nil, fmt.Errorf("event %s/%s@%s: %w", namespace, name, to, registry.ErrNotFound)
	}
	return spec.Diff(fromDef, toDef), nil
}

// containsAll reports whether tags contains every element of required.
func containsAll(tags, required []string) bool {
	if len(required) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		set[t] = struct{}{}
	}
	for _, req := range required {
		if _, ok := set[req]; !ok {
			return false
		}
	}
	return true
}

// walkSources loads all *.yaml files in dir as SourceDef and adds them to out.
// A missing or empty dir is silently ignored.
func walkSources(dir string, out map[string]*spec.SourceDef) error {
	if dir == "" {
		return nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		def, err := spec.LoadSourceDef(path)
		if err != nil {
			return fmt.Errorf("load source %s: %w", path, err)
		}
		out[def.Name] = def
		return nil
	})
}

// walkDestinations loads all *.yaml files in dir as DestinationDef and adds them to out.
// A missing or empty dir is silently ignored.
func walkDestinations(dir string, out map[string]*spec.DestinationDef) error {
	if dir == "" {
		return nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		def, err := spec.LoadDestinationDef(path)
		if err != nil {
			return fmt.Errorf("load destination %s: %w", path, err)
		}
		out[def.Name] = def
		return nil
	})
}
