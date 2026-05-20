package codegen

import "fmt"

// LangConfig holds per-language codegen configuration.
type LangConfig struct {
	ID      string // "go", "typescript"
	Namer   Namer
	FileExt string // ".go", ".ts"
}

var languages = map[string]LangConfig{
	"go": {
		ID:      "go",
		Namer:   GoNamer{},
		FileExt: ".go",
	},
	"typescript": {
		ID:      "typescript",
		Namer:   CamelCaseNamer{},
		FileExt: ".ts",
	},
}

// LookupLang returns the LangConfig for a language ID, or an error if unsupported.
func LookupLang(id string) (LangConfig, error) {
	lc, ok := languages[id]
	if !ok {
		return LangConfig{}, fmt.Errorf("unsupported language %q: supported languages are go, typescript", id)
	}
	return lc, nil
}
