package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScanResult holds the aggregated output of scanning a directory tree.
type ScanResult struct {
	// MethodCalls maps method name → all locations where that method was called.
	MethodCalls map[string][]Location
	// RawTrackCalls maps raw event name → locations of untyped Track calls.
	RawTrackCalls map[string][]Location
	// ScannedFiles is the count of source files examined.
	ScannedFiles int
}

// Scanner walks a directory tree and collects EventSpec method call sites
// using the Matcher registered for the target language.
type Scanner struct {
	matcher Matcher
}

// NewScanner returns a Scanner for the given language.
// Returns an error if the language is not supported.
func NewScanner(language string) (*Scanner, error) {
	m, ok := matcherForLanguage(language)
	if !ok {
		return nil, fmt.Errorf("audit: unsupported language %q; supported: go, typescript, swift", language)
	}
	return &Scanner{matcher: m}, nil
}

func matcherForLanguage(lang string) (Matcher, bool) {
	switch strings.ToLower(lang) {
	case "go":
		return NewGoMatcher(), true
	case "typescript", "javascript":
		return NewTypeScriptMatcher(), true
	case "swift":
		return NewSwiftMatcher(), true
	default:
		return nil, false
	}
}

// ScanDir walks dir recursively, scanning every source file whose extension
// matches the language-specific set. It returns the aggregated ScanResult.
func (s *Scanner) ScanDir(dir string) (*ScanResult, error) {
	result := &ScanResult{
		MethodCalls:   make(map[string][]Location),
		RawTrackCalls: make(map[string][]Location),
	}

	exts := extensionSet(s.matcher.FileExtensions())

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden directories and common vendor/dependency directories.
			name := d.Name()
			if name != "." && (strings.HasPrefix(name, ".") ||
				name == "node_modules" || name == "vendor" ||
				name == "dist" || name == "build" || name == ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		if !exts[filepath.Ext(path)] {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		methods, rawTracks, err := s.matcher.FindUsages(path, content)
		if err != nil {
			// Non-fatal: skip files that cannot be parsed.
			return nil
		}

		result.ScannedFiles++
		for name, locs := range methods {
			result.MethodCalls[name] = append(result.MethodCalls[name], locs...)
		}
		for name, locs := range rawTracks {
			result.RawTrackCalls[name] = append(result.RawTrackCalls[name], locs...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func extensionSet(exts []string) map[string]bool {
	m := make(map[string]bool, len(exts))
	for _, e := range exts {
		m[e] = true
	}
	return m
}
