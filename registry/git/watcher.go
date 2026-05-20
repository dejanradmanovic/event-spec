package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches the registry directories for YAML file changes and triggers
// a full index rebuild on the backing Resolver. Intended for dev mode only;
// production deployments should restart the process or use a server registry.
type Watcher struct {
	r  *Resolver
	fw *fsnotify.Watcher
}

// NewWatcher creates a Watcher that monitors the Resolver's configured directories.
// It recursively adds every subdirectory under SpecsDir, SourcesDir, and
// DestinationsDir so that newly created subdirectories are picked up on the
// next file event in a parent directory.
func NewWatcher(r *Resolver) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}
	w := &Watcher{r: r, fw: fw}
	for _, dir := range []string{r.cfg.SpecsDir, r.cfg.SourcesDir, r.cfg.DestinationsDir} {
		if err := w.addDir(dir); err != nil {
			fw.Close()
			return nil, err
		}
	}
	return w, nil
}

// addDir recursively adds dir and all subdirectories to the fsnotify watcher.
// Missing or empty directories are silently ignored.
func (w *Watcher) addDir(dir string) error {
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
		if d.IsDir() {
			return w.fw.Add(path)
		}
		return nil
	})
}

// Start runs the watcher event loop, blocking until ctx is cancelled.
// It reloads the Resolver index on every write, create, or remove event
// affecting a .yaml file. Errors from fsnotify are discarded; a failed
// reload leaves the previous index intact.
func (w *Watcher) Start(ctx context.Context) error {
	for {
		select {
		case event, ok := <-w.fw.Events:
			if !ok {
				return nil
			}
			if strings.HasSuffix(event.Name, ".yaml") &&
				(event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove)) {
				_ = w.r.reload()
			}
		case _, ok := <-w.fw.Errors:
			if !ok {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Close releases the fsnotify watcher and its file descriptors.
func (w *Watcher) Close() error {
	return w.fw.Close()
}
