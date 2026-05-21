// Package audit implements AST-based event usage scanning and coverage reporting.
package audit

import (
	"bufio"
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// Location identifies a specific point in source code.
type Location struct {
	File string
	Line int
}

// Matcher scans a single source file and returns method usages found.
type Matcher interface {
	// Language identifies the language this matcher handles.
	Language() string
	// FileExtensions returns the file suffixes to scan (e.g. ".go", ".ts").
	FileExtensions() []string
	// FindUsages parses content and returns a map of method name → locations
	// where EventSpec methods were called, and a separate map of raw event
	// names extracted from untyped Track calls (rogue candidates).
	FindUsages(path string, content []byte) (methodCalls map[string][]Location, rawTrackCalls map[string][]Location, err error)
}

// NewGoMatcher returns a Matcher for Go source files.
// It uses go/parser to detect SelectorExpr calls (typed EventSpec method calls)
// and also finds raw analytics.Event{Name: "..."} rogue calls.
func NewGoMatcher() Matcher { return &goMatcher{} }

// NewTypeScriptMatcher returns a Matcher for TypeScript/JavaScript source files.
func NewTypeScriptMatcher() Matcher { return &tsMatcher{} }

// NewSwiftMatcher returns a Matcher for Swift source files.
func NewSwiftMatcher() Matcher { return &swiftMatcher{} }

// ─── Go matcher ──────────────────────────────────────────────────────────────

type goMatcher struct{}

func (goMatcher) Language() string         { return "go" }
func (goMatcher) FileExtensions() []string { return []string{".go"} }

func (goMatcher) FindUsages(path string, content []byte) (map[string][]Location, map[string][]Location, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, content, 0)
	if err != nil {
		// Skip files that do not compile (e.g. build-tag excluded files).
		return nil, nil, nil
	}

	methods := make(map[string][]Location)
	rawTracks := make(map[string][]Location)

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		methodName := sel.Sel.Name
		pos := fset.Position(sel.Pos())
		loc := Location{File: path, Line: pos.Line}

		if methodName == "Track" {
			// Look for analytics.Event{Name: "<literal>"} in the args.
			for _, arg := range call.Args {
				if evName, ok := extractGoEventName(arg); ok {
					rawTracks[evName] = append(rawTracks[evName], loc)
				}
			}
			return true
		}

		methods[methodName] = append(methods[methodName], loc)
		return true
	})

	return methods, rawTracks, nil
}

// extractGoEventName attempts to extract the Name field from an analytics.Event
// composite literal, e.g. analytics.Event{Name: "Product Viewed"}.
func extractGoEventName(expr ast.Expr) (string, bool) {
	comp, ok := expr.(*ast.CompositeLit)
	if !ok {
		return "", false
	}
	for _, elt := range comp.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok || ident.Name != "Name" {
			continue
		}
		lit, ok := kv.Value.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		name := strings.Trim(lit.Value, `"`+"`")
		return name, true
	}
	return "", false
}

// ─── TypeScript matcher ───────────────────────────────────────────────────────

// tsMethodCallRe matches `.methodName(` with an optional space before the paren.
var tsMethodCallRe = regexp.MustCompile(`\.([a-zA-Z][a-zA-Z0-9_]*)\s*\(`)

// tsRawTrackRe matches `.track({...name: "EventName"...}` or `.track({ name: 'EventName'`.
var tsRawTrackRe = regexp.MustCompile(`\.track\s*\(\s*\{[^}]*?name\s*:\s*["']([^"']+)["']`)

type tsMatcher struct{}

func (tsMatcher) Language() string         { return "typescript" }
func (tsMatcher) FileExtensions() []string { return []string{".ts", ".tsx", ".js", ".jsx"} }

func (tsMatcher) FindUsages(path string, content []byte) (map[string][]Location, map[string][]Location, error) {
	methods := make(map[string][]Location)
	rawTracks := make(map[string][]Location)

	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip comment lines.
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		// Find raw .track({ name: ... }) calls first (before generic method scan).
		if matches := tsRawTrackRe.FindAllStringSubmatch(line, -1); len(matches) > 0 {
			for _, m := range matches {
				rawTracks[m[1]] = append(rawTracks[m[1]], Location{File: path, Line: lineNum})
			}
			continue
		}

		// Find all .methodName( calls.
		for _, m := range tsMethodCallRe.FindAllStringSubmatch(line, -1) {
			name := m[1]
			if name == "track" || name == "identify" || name == "group" || name == "page" || name == "alias" {
				// These are core analytics calls, not generated event methods.
				continue
			}
			methods[name] = append(methods[name], Location{File: path, Line: lineNum})
		}
	}
	return methods, rawTracks, scanner.Err()
}

// ─── Swift matcher ────────────────────────────────────────────────────────────

// swiftMethodCallRe matches `.methodName(` call sites.
var swiftMethodCallRe = regexp.MustCompile(`\.([a-zA-Z][a-zA-Z0-9_]*)\s*\(`)

// swiftRawTrackRe matches `.track(event: "EventName"` or `.track("EventName"`.
var swiftRawTrackRe = regexp.MustCompile(`\.track\s*\(\s*(?:event:\s*)?["']([^"']+)["']`)

type swiftMatcher struct{}

func (swiftMatcher) Language() string         { return "swift" }
func (swiftMatcher) FileExtensions() []string { return []string{".swift"} }

func (swiftMatcher) FindUsages(path string, content []byte) (map[string][]Location, map[string][]Location, error) {
	methods := make(map[string][]Location)
	rawTracks := make(map[string][]Location)

	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		if matches := swiftRawTrackRe.FindAllStringSubmatch(line, -1); len(matches) > 0 {
			for _, m := range matches {
				rawTracks[m[1]] = append(rawTracks[m[1]], Location{File: path, Line: lineNum})
			}
			continue
		}

		for _, m := range swiftMethodCallRe.FindAllStringSubmatch(line, -1) {
			name := m[1]
			if name == "track" || name == "identify" || name == "group" || name == "page" || name == "alias" {
				continue
			}
			methods[name] = append(methods[name], Location{File: path, Line: lineNum})
		}
	}
	return methods, rawTracks, scanner.Err()
}
