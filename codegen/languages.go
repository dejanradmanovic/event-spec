package codegen

import (
	"fmt"
	"sort"
	"strings"
)

// LangConfig holds the language-specific strategy objects for codegen.
type LangConfig struct {
	ID         string // "go", "typescript"
	Namer      Namer
	TypeMapper TypeMapper
	FileExt    string // ".go", ".ts"
}

type langEntry struct {
	config LangConfig
	engine Engine
}

var registry = map[string]langEntry{}

// Register registers a language engine and its configuration.
// Intended to be called from init() in language-specific packages.
func Register(config LangConfig, engine Engine) {
	registry[config.ID] = langEntry{config: config, engine: engine}
}

// RegisteredLangs returns the IDs of every registered language, sorted.
// Only languages whose package has been imported are present, so callers that
// need the full set must blank-import the engines (as cmd/event-spec does).
func RegisteredLangs() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// LookupLang returns the LangConfig for the given language ID.
// An unknown ID names the languages that are actually available, so the caller
// does not have to go and read the registry to find out what to pass.
func LookupLang(id string) (LangConfig, error) {
	e, ok := registry[id]
	if !ok {
		if ids := RegisteredLangs(); len(ids) > 0 {
			return LangConfig{}, fmt.Errorf("unsupported language: %q (supported: %s)", id, strings.Join(ids, ", "))
		}
		return LangConfig{}, fmt.Errorf("unsupported language: %q (no languages registered)", id)
	}
	return e.config, nil
}
