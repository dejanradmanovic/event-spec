package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScaffoldOpts configures the generated event spec template.
type ScaffoldOpts struct {
	Type        EventType
	Status      EventStatus
	Owner       string
	Description string
	DisplayName string
}

// ScaffoldEventDef generates a minimal, valid YAML event spec for namespace/name.
// The returned bytes pass ValidateEventDef immediately after creation.
// All ScaffoldOpts fields are optional; zero values get sensible defaults.
func ScaffoldEventDef(namespace, name string, opts ScaffoldOpts) ([]byte, error) {
	if opts.Type == "" {
		opts.Type = TypeTrack
	}
	if opts.Status == "" {
		opts.Status = StatusDraft
	}
	if opts.DisplayName == "" {
		opts.DisplayName = toDisplayName(name)
	}

	def := EventDef{
		Schema:      "https://event-spec.io/schemas/event/v1",
		Name:        name,
		DisplayName: opts.DisplayName,
		Description: opts.Description,
		Version:     "1-0-0",
		Changelog:   "Initial version",
		Status:      opts.Status,
		Namespace:   namespace,
		Owner:       opts.Owner,
		Type:        opts.Type,
		EventName:   opts.DisplayName,
		Properties:  map[string]PropertyDef{},
	}

	return yaml.Marshal(&def)
}

// WriteScaffoldFile writes content to path, creating parent directories as needed.
// Returns an error if path already exists.
func WriteScaffoldFile(path string, content []byte) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// toDisplayName converts a snake_case name to "Title Case" display name.
func toDisplayName(name string) string {
	words := strings.Split(name, "_")
	for i, w := range words {
		if len(w) == 0 {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
