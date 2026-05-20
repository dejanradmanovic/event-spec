package codegen

import "fmt"

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

// LookupLang returns the LangConfig for the given language ID.
func LookupLang(id string) (LangConfig, error) {
	e, ok := registry[id]
	if !ok {
		return LangConfig{}, fmt.Errorf("unsupported language: %q", id)
	}
	return e.config, nil
}
