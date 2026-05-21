package audit

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"sort"
	"strings"

	"github.com/dejanradmanovic/event-spec/codegen"
	"github.com/dejanradmanovic/event-spec/spec"
)

// UsedEvent describes a spec event that was found in the codebase.
type UsedEvent struct {
	EventKey  string // "namespace/name"
	Version   string // spec version
	SpecFile  string // path to the spec YAML
	Method    string // generated method name
	Locations []Location
}

// UnusedEvent describes a spec event that was NOT found in the codebase.
type UnusedEvent struct {
	EventKey string // "namespace/name"
	Version  string
	SpecFile string
	Required bool
}

// RogueEvent describes a call found in code that has no matching spec event.
type RogueEvent struct {
	EventName string // as it appears in code (method name or raw Track name)
	Locations []Location
}

// Report is the full audit coverage report.
type Report struct {
	Source       string
	Language     string
	ScannedFiles int
	Used         []UsedEvent
	Unused       []UnusedEvent
	Rogue        []RogueEvent
	TotalDefined int
	CoveragePct  float64
}

// BuildReport compares spec event definitions against the scanner results to
// produce a coverage Report. methodName converts an event name to the
// language-specific generated method name (e.g. PascalCase for Go, camelCase
// for TypeScript and Swift).
func BuildReport(source, language string, defs []*spec.EventDef, result *ScanResult) *Report {
	r := &Report{
		Source:       source,
		Language:     language,
		ScannedFiles: result.ScannedFiles,
		TotalDefined: len(defs),
	}

	// Build the set of expected method names from the spec.
	type eventEntry struct {
		def    *spec.EventDef
		method string
		key    string
	}
	specEvents := make(map[string]eventEntry, len(defs))
	for _, def := range defs {
		method := methodNameForLanguage(language, def.Name)
		key := def.Namespace + "/" + def.Name
		specEvents[method] = eventEntry{def: def, method: method, key: key}
	}

	// Determine which spec events were used.
	usedMethods := make(map[string]bool)
	for methodName, entry := range specEvents {
		locs, found := result.MethodCalls[methodName]
		if found && len(locs) > 0 {
			usedMethods[methodName] = true
			r.Used = append(r.Used, UsedEvent{
				EventKey:  entry.key,
				Version:   entry.def.Version,
				SpecFile:  entry.def.SourcePath,
				Method:    methodName,
				Locations: locs,
			})
		} else {
			r.Unused = append(r.Unused, UnusedEvent{
				EventKey: entry.key,
				Version:  entry.def.Version,
				SpecFile: entry.def.SourcePath,
				Required: entry.def.Required,
			})
		}
	}

	// Any method call in the scan result not matching a spec event is rogue.
	for methodName, locs := range result.MethodCalls {
		if _, inSpec := specEvents[methodName]; !inSpec {
			r.Rogue = append(r.Rogue, RogueEvent{EventName: methodName, Locations: locs})
		}
	}
	// Raw Track calls are always rogue (bypass typed API).
	for eventName, locs := range result.RawTrackCalls {
		r.Rogue = append(r.Rogue, RogueEvent{EventName: eventName, Locations: locs})
	}

	// Compute coverage.
	if r.TotalDefined > 0 {
		r.CoveragePct = float64(len(r.Used)) / float64(r.TotalDefined) * 100
	}

	// Sort for deterministic output.
	sort.Slice(r.Used, func(i, j int) bool { return r.Used[i].EventKey < r.Used[j].EventKey })
	sort.Slice(r.Unused, func(i, j int) bool { return r.Unused[i].EventKey < r.Unused[j].EventKey })
	sort.Slice(r.Rogue, func(i, j int) bool { return r.Rogue[i].EventName < r.Rogue[j].EventName })

	return r
}

// methodNameForLanguage converts a snake_case event name to the generated
// method name convention for the target language.
func methodNameForLanguage(language, rawName string) string {
	switch strings.ToLower(language) {
	case "go":
		return codegen.ToPascalCase(rawName)
	default: // typescript, swift, kotlin, python → camelCase
		return codegen.ToCamelCase(rawName)
	}
}

// ─── Report rendering ─────────────────────────────────────────────────────────

// WriteText writes a human-readable text report to w.
func (r *Report) WriteText(w io.Writer) {
	sep := strings.Repeat("=", 80)
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "Event Coverage Report: %s\n", r.Source)
	fmt.Fprintln(w, sep)
	fmt.Fprintf(w, "Source:     %s (%s)\n", r.Source, r.Language)
	fmt.Fprintf(w, "Scanned:    %d files\n", r.ScannedFiles)
	fmt.Fprintf(w, "Events:     %d defined, %d used (%.1f%% coverage)\n",
		r.TotalDefined, len(r.Used), r.CoveragePct)
	fmt.Fprintln(w, "")

	if len(r.Used) > 0 {
		fmt.Fprintf(w, "✅ USED (%d events)\n", len(r.Used))
		for _, e := range r.Used {
			firstLoc := ""
			if len(e.Locations) > 0 {
				firstLoc = fmt.Sprintf("  %s:%d", e.Locations[0].File, e.Locations[0].Line)
			}
			fmt.Fprintf(w, "  %-50s%s\n", e.EventKey, firstLoc)
		}
		fmt.Fprintln(w, "")
	}

	if len(r.Unused) > 0 {
		fmt.Fprintf(w, "⚠️  UNUSED (%d events - defined in spec but not found in code)\n", len(r.Unused))
		for _, e := range r.Unused {
			req := ""
			if e.Required {
				req = " [required]"
			}
			fmt.Fprintf(w, "  %-50s Declared in %s%s\n", e.EventKey, e.SpecFile, req)
		}
		fmt.Fprintln(w, "")
	}

	if len(r.Rogue) > 0 {
		fmt.Fprintf(w, "❌ ROGUE EVENTS (%d - sent but not in spec)\n", len(r.Rogue))
		for _, e := range r.Rogue {
			firstLoc := ""
			if len(e.Locations) > 0 {
				firstLoc = fmt.Sprintf("  %s:%d", e.Locations[0].File, e.Locations[0].Line)
			}
			fmt.Fprintf(w, "  %-50s%s\n", e.EventName, firstLoc)
		}
		fmt.Fprintln(w, "")
	}

	if len(r.Rogue) == 0 && len(r.Unused) == 0 {
		fmt.Fprintf(w, "✅ All %d events are used. No rogue events detected.\n", r.TotalDefined)
	}
}

// jsonReport mirrors the JSON output shape described in ARCHITECTURE.md.
type jsonReport struct {
	Source       string       `json:"source"`
	Language     string       `json:"language"`
	ScannedFiles int          `json:"scanned_files"`
	Coverage     jsonCoverage `json:"coverage"`
	Used         []jsonUsed   `json:"used"`
	Unused       []jsonUnused `json:"unused"`
	Rogue        []jsonRogue  `json:"rogue"`
}

type jsonCoverage struct {
	TotalEvents  int     `json:"total_events"`
	UsedEvents   int     `json:"used_events"`
	UnusedEvents int     `json:"unused_events"`
	RogueEvents  int     `json:"rogue_events"`
	CoveragePct  float64 `json:"coverage_pct"`
}

type jsonUsed struct {
	Event     string         `json:"event"`
	Version   string         `json:"version"`
	Locations []jsonLocation `json:"locations"`
}

type jsonUnused struct {
	Event    string `json:"event"`
	Version  string `json:"version"`
	SpecFile string `json:"spec_file"`
	Required bool   `json:"required,omitempty"`
}

type jsonRogue struct {
	EventName string         `json:"event_name"`
	Locations []jsonLocation `json:"locations"`
}

type jsonLocation struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// WriteJSON writes a machine-readable JSON report to w.
func (r *Report) WriteJSON(w io.Writer) error {
	jr := jsonReport{
		Source:       r.Source,
		Language:     r.Language,
		ScannedFiles: r.ScannedFiles,
		Coverage: jsonCoverage{
			TotalEvents:  r.TotalDefined,
			UsedEvents:   len(r.Used),
			UnusedEvents: len(r.Unused),
			RogueEvents:  len(r.Rogue),
			CoveragePct:  r.CoveragePct,
		},
	}
	for _, e := range r.Used {
		jlocs := make([]jsonLocation, len(e.Locations))
		for i, l := range e.Locations {
			jlocs[i] = jsonLocation{File: l.File, Line: l.Line}
		}
		jr.Used = append(jr.Used, jsonUsed{Event: e.EventKey, Version: e.Version, Locations: jlocs})
	}
	for _, e := range r.Unused {
		jr.Unused = append(jr.Unused, jsonUnused{Event: e.EventKey, Version: e.Version, SpecFile: e.SpecFile, Required: e.Required})
	}
	for _, e := range r.Rogue {
		jlocs := make([]jsonLocation, len(e.Locations))
		for i, l := range e.Locations {
			jlocs[i] = jsonLocation{File: l.File, Line: l.Line}
		}
		jr.Rogue = append(jr.Rogue, jsonRogue{EventName: e.EventName, Locations: jlocs})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jr)
}

// WriteHTML writes an HTML report to w.
func (r *Report) WriteHTML(w io.Writer) {
	e := func(s string) string { return html.EscapeString(s) }

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Event Coverage Report: %s</title>
<style>
body{font-family:system-ui,sans-serif;max-width:960px;margin:2rem auto;padding:0 1rem;color:#222}
h1{border-bottom:2px solid #ddd;padding-bottom:.5rem}
.summary{background:#f8f8f8;border:1px solid #ddd;border-radius:4px;padding:1rem;margin-bottom:1.5rem}
.pct{font-size:2rem;font-weight:bold}
table{width:100%%;border-collapse:collapse;margin-bottom:2rem}
th{text-align:left;border-bottom:2px solid #ddd;padding:.4rem .6rem}
td{border-bottom:1px solid #eee;padding:.4rem .6rem;vertical-align:top}
.used td:first-child::before{content:"✅ "}
.unused td:first-child::before{content:"⚠️ "}
.rogue td:first-child::before{content:"❌ "}
.req{color:#c00;font-size:.8em}
</style>
</head>
<body>
<h1>Event Coverage Report: %s</h1>
<div class="summary">
<p>Source: <strong>%s</strong> (%s) &nbsp;|&nbsp; Scanned: <strong>%d</strong> files</p>
<p class="pct">%.1f%%</p>
<p>%d defined &nbsp;·&nbsp; <strong>%d used</strong> &nbsp;·&nbsp; %d unused &nbsp;·&nbsp; %d rogue</p>
</div>
`,
		e(r.Source),
		e(r.Source),
		e(r.Source), e(r.Language), r.ScannedFiles,
		r.CoveragePct,
		r.TotalDefined, len(r.Used), len(r.Unused), len(r.Rogue),
	)

	if len(r.Used) > 0 {
		fmt.Fprintln(w, "<h2>✅ Used Events</h2>")
		fmt.Fprintln(w, "<table><tr><th>Event</th><th>Version</th><th>Location</th></tr>")
		for _, ev := range r.Used {
			loc := ""
			if len(ev.Locations) > 0 {
				loc = fmt.Sprintf("%s:%d", e(ev.Locations[0].File), ev.Locations[0].Line)
			}
			fmt.Fprintf(w, "<tr class=\"used\"><td>%s</td><td>%s</td><td>%s</td></tr>\n",
				e(ev.EventKey), e(ev.Version), loc)
		}
		fmt.Fprintln(w, "</table>")
	}

	if len(r.Unused) > 0 {
		fmt.Fprintln(w, "<h2>⚠️ Unused Events</h2>")
		fmt.Fprintln(w, "<table><tr><th>Event</th><th>Version</th><th>Spec File</th></tr>")
		for _, ev := range r.Unused {
			req := ""
			if ev.Required {
				req = ` <span class="req">[required]</span>`
			}
			fmt.Fprintf(w, "<tr class=\"unused\"><td>%s%s</td><td>%s</td><td>%s</td></tr>\n",
				e(ev.EventKey), req, e(ev.Version), e(ev.SpecFile))
		}
		fmt.Fprintln(w, "</table>")
	}

	if len(r.Rogue) > 0 {
		fmt.Fprintln(w, "<h2>❌ Rogue Events</h2>")
		fmt.Fprintln(w, "<table><tr><th>Event</th><th>Location</th></tr>")
		for _, ev := range r.Rogue {
			loc := ""
			if len(ev.Locations) > 0 {
				loc = fmt.Sprintf("%s:%d", e(ev.Locations[0].File), ev.Locations[0].Line)
			}
			fmt.Fprintf(w, "<tr class=\"rogue\"><td>%s</td><td>%s</td></tr>\n",
				e(ev.EventName), loc)
		}
		fmt.Fprintln(w, "</table>")
	}

	fmt.Fprintln(w, "</body></html>")
}
