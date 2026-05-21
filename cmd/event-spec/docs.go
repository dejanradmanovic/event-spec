package main

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/dejanradmanovic/event-spec/spec"
	"github.com/spf13/cobra"
)

func newDocsCmd() *cobra.Command {
	var (
		format    string
		out       string
		formatSet bool
		outSet    bool
	)

	cmd := &cobra.Command{
		Use:   "docs [spec-dir]",
		Short: "Generate an HTML or Markdown event catalog from the spec registry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cfgErr := spec.LoadWorkspaceConfig("event-spec.yaml")

			if cfgErr == nil {
				if !formatSet && cfg.Docs.Format != "" {
					format = cfg.Docs.Format
				}
				if !outSet && cfg.Docs.Out != "" {
					out = cfg.Docs.Out
				}
			}
			if format == "" {
				format = "html"
			}
			if out == "" {
				out = "./docs"
			}

			switch format {
			case "html", "markdown":
			default:
				return fmt.Errorf("--format must be one of: html, markdown")
			}

			var defs []*spec.EventDef

			switch {
			case len(args) > 0:
				var walkErrs []error
				defs, walkErrs = spec.WalkEventDefs(args[0])
				for _, e := range walkErrs {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", e)
				}
			case cfgErr == nil:
				reg, err := openRegistry(cfg)
				if err != nil {
					return err
				}
				all, err := listAllEvents(context.Background(), reg)
				if err != nil {
					return fmt.Errorf("list events: %w", err)
				}
				defs = applySourceConfig(all, nil)
			default:
				var walkErrs []error
				defs, walkErrs = spec.WalkEventDefs("./specs")
				for _, e := range walkErrs {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", e)
				}
			}

			if len(defs) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no event specs found; nothing to generate")
				return nil
			}

			sort.Slice(defs, func(i, j int) bool {
				if defs[i].Namespace != defs[j].Namespace {
					return defs[i].Namespace < defs[j].Namespace
				}
				return defs[i].Name < defs[j].Name
			})

			if err := os.MkdirAll(out, 0o755); err != nil {
				return fmt.Errorf("create output dir: %w", err)
			}

			ext := "html"
			if format == "markdown" {
				ext = "md"
			}

			if err := generateDocs(defs, format, ext, out); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"generated %d event page(s) to %s (format: %s)\n",
				len(defs), out, format)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"index: %s\n", filepath.Join(out, "index."+ext))
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "", "output format: html | markdown (default: docs.format from event-spec.yaml, or html)")
	cmd.Flags().StringVar(&out, "out", "", "output directory (default: docs.out from event-spec.yaml, or ./docs)")

	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		formatSet = cmd.Flags().Changed("format")
		outSet = cmd.Flags().Changed("out")
	}

	return cmd
}

// docEventData is the template data model for a single event page.
type docEventData struct {
	DisplayName string
	Name        string
	Namespace   string
	Version     string
	Status      string
	EventType   string
	Owner       string
	Tags        []string
	Description string
	Changelog   string
	Props       []docProp
	IndexLink   string
	GeneratedAt time.Time
}

// docProp is a single property entry in the event page template.
type docProp struct {
	Name        string
	PropType    string
	Required    bool
	Description string
	Enum        []string
	Pattern     string
	HasMinimum  bool
	Minimum     float64
	HasMaximum  bool
	Maximum     float64
}

// docIndexData is the template data model for the catalog index page.
type docIndexData struct {
	Namespaces  []docNS
	GeneratedAt time.Time
}

// docNS groups events under a single namespace for the index.
type docNS struct {
	Name   string
	Events []docNSEntry
}

// docNSEntry is a single event row in the index.
type docNSEntry struct {
	DisplayName string
	Version     string
	Status      string
	Description string
	Link        string
}

func generateDocs(defs []*spec.EventDef, format, ext, out string) error {
	now := time.Now().UTC()

	funcs := template.FuncMap{
		"join": strings.Join,
		"firstLine": func(s string) string {
			s = strings.TrimSpace(s)
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[:i]
			}
			if len([]rune(s)) > 80 {
				return string([]rune(s)[:80]) + "…"
			}
			return s
		},
		"formatTime": func(t time.Time) string { return t.Format("2006-01-02") },
		"escape":     html.EscapeString,
	}

	// Group events by namespace for the index, preserving sort order.
	nsMap := map[string][]docNSEntry{}
	var nsOrder []string
	for _, def := range defs {
		displayName := def.DisplayName
		if displayName == "" {
			displayName = def.Name
		}
		entry := docNSEntry{
			DisplayName: displayName,
			Version:     def.Version,
			Status:      string(def.Status),
			Description: def.Description,
			Link:        def.Namespace + "/" + def.Name + "." + ext,
		}
		if _, seen := nsMap[def.Namespace]; !seen {
			nsOrder = append(nsOrder, def.Namespace)
		}
		nsMap[def.Namespace] = append(nsMap[def.Namespace], entry)
	}
	sort.Strings(nsOrder)

	nsList := make([]docNS, 0, len(nsOrder))
	for _, ns := range nsOrder {
		nsList = append(nsList, docNS{Name: ns, Events: nsMap[ns]})
	}

	indexSrc, eventSrc := mdIndexTmpl, mdEventTmpl
	if format == "html" {
		indexSrc, eventSrc = htmlIndexTmpl, htmlEventTmpl
	}

	indexTmpl, err := template.New("index").Funcs(funcs).Parse(indexSrc)
	if err != nil {
		return fmt.Errorf("parse index template: %w", err)
	}
	eventTmpl, err := template.New("event").Funcs(funcs).Parse(eventSrc)
	if err != nil {
		return fmt.Errorf("parse event template: %w", err)
	}

	var buf bytes.Buffer
	if err := indexTmpl.Execute(&buf, docIndexData{Namespaces: nsList, GeneratedAt: now}); err != nil {
		return fmt.Errorf("render index: %w", err)
	}
	if err := os.WriteFile(filepath.Join(out, "index."+ext), buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	for _, def := range defs {
		if err := writeEventPage(def, eventTmpl, ext, out, now); err != nil {
			return err
		}
	}
	return nil
}

func writeEventPage(def *spec.EventDef, tmpl *template.Template, ext, out string, now time.Time) error {
	displayName := def.DisplayName
	if displayName == "" {
		displayName = def.Name
	}

	names := make([]string, 0, len(def.Properties))
	for n := range def.Properties {
		names = append(names, n)
	}
	sort.Strings(names)

	props := make([]docProp, 0, len(names))
	for _, n := range names {
		p := def.Properties[n]
		dp := docProp{
			Name:        n,
			PropType:    string(p.Type),
			Required:    p.Required,
			Description: p.Description,
			Enum:        p.Enum,
			Pattern:     p.Pattern,
		}
		if p.Minimum != nil {
			dp.HasMinimum = true
			dp.Minimum = *p.Minimum
		}
		if p.Maximum != nil {
			dp.HasMaximum = true
			dp.Maximum = *p.Maximum
		}
		props = append(props, dp)
	}

	data := docEventData{
		DisplayName: displayName,
		Name:        def.Name,
		Namespace:   def.Namespace,
		Version:     def.Version,
		Status:      string(def.Status),
		EventType:   string(def.Type),
		Owner:       def.Owner,
		Tags:        def.Tags,
		Description: def.Description,
		Changelog:   def.Changelog,
		Props:       props,
		IndexLink:   "../index." + ext,
		GeneratedAt: now,
	}

	nsDir := filepath.Join(out, def.Namespace)
	if err := os.MkdirAll(nsDir, 0o755); err != nil {
		return fmt.Errorf("create namespace dir %s: %w", nsDir, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("render event %s/%s: %w", def.Namespace, def.Name, err)
	}
	return os.WriteFile(filepath.Join(nsDir, def.Name+"."+ext), buf.Bytes(), 0o644)
}

const mdIndexTmpl = `# Event Catalog
{{range .Namespaces}}
## {{.Name}}

| Event | Version | Status | Description |
|-------|---------|--------|-------------|
{{- range .Events}}
| [{{.DisplayName}}]({{.Link}}) | ` + "`" + `{{.Version}}` + "`" + ` | {{.Status}} | {{firstLine .Description}} |
{{- end}}
{{end}}
---

_Generated by event-spec on {{formatTime .GeneratedAt}}_
`

const mdEventTmpl = `# {{.DisplayName}}
{{if .Description}}
{{.Description}}
{{end}}
| Field | Value |
|-------|-------|
| Version | ` + "`" + `{{.Version}}` + "`" + ` |
| Status | {{.Status}} |
| Type | {{.EventType}} |
| Namespace | {{.Namespace}} |
{{- if .Owner}}
| Owner | {{.Owner}} |
{{- end}}
{{- if .Tags}}
| Tags | {{join .Tags ", "}} |
{{- end}}

## Properties
{{if .Props}}
| Property | Type | Required | Description |
|----------|------|----------|-------------|
{{- range .Props}}
| ` + "`" + `{{.Name}}` + "`" + ` | {{.PropType}}{{if .Enum}} (enum){{end}} | {{if .Required}}✅{{else}}—{{end}} | {{.Description}}{{if .Enum}} Values: {{join .Enum ", "}}{{end}}{{if .Pattern}} Pattern: ` + "`" + `{{.Pattern}}` + "`" + `{{end}}{{if .HasMinimum}} Min: {{.Minimum}}{{end}}{{if .HasMaximum}} Max: {{.Maximum}}{{end}} |
{{- end}}
{{else}}
_No properties defined._
{{end}}
{{- if .Changelog}}
## Changelog

{{.Changelog}}
{{end}}
---

[← Back to Index]({{.IndexLink}})

_Generated by event-spec on {{formatTime .GeneratedAt}}_
`

const docCSS = `body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;max-width:960px;margin:0 auto;padding:2rem;color:#24292f;line-height:1.5}
h1{border-bottom:1px solid #d0d7de;padding-bottom:.3em}
h2{border-bottom:1px solid #d0d7de;padding-bottom:.3em;margin-top:2rem}
table{border-collapse:collapse;width:100%;margin:1rem 0}
th,td{padding:.5rem 1rem;border:1px solid #d0d7de;text-align:left;vertical-align:top}
th{background:#f6f8fa;font-weight:600}
tr:nth-child(even){background:#f9fafb}
a{color:#0969da;text-decoration:none}
a:hover{text-decoration:underline}
code{background:#eef0f2;padding:.2em .4em;border-radius:3px;font-size:.9em}
.status-active{color:#1a7f37;font-weight:600}
.status-deprecated{color:#9a6700;font-weight:600}
.status-draft{color:#0969da;font-weight:600}
.status-deleted{color:#cf222e;font-weight:600}
.required{color:#1a7f37}
.optional{color:#656d76}
.nav{margin-bottom:1.5rem}
.footer{margin-top:3rem;padding-top:1rem;border-top:1px solid #d0d7de;color:#656d76;font-size:.875rem}`

const htmlIndexTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Event Catalog</title>
<style>` + docCSS + `</style>
</head>
<body>
<h1>Event Catalog</h1>
{{range .Namespaces}}<h2>{{escape .Name}}</h2>
<table>
<thead><tr><th>Event</th><th>Version</th><th>Status</th><th>Description</th></tr></thead>
<tbody>
{{- range .Events}}
<tr>
<td><a href="{{.Link}}">{{escape .DisplayName}}</a></td>
<td><code>{{escape .Version}}</code></td>
<td><span class="status-{{escape .Status}}">{{escape .Status}}</span></td>
<td>{{escape (firstLine .Description)}}</td>
</tr>
{{- end}}
</tbody>
</table>
{{end}}<div class="footer">Generated by event-spec on {{formatTime .GeneratedAt}}</div>
</body>
</html>
`

const htmlEventTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{escape .DisplayName}} — Event Catalog</title>
<style>` + docCSS + `</style>
</head>
<body>
<div class="nav"><a href="{{.IndexLink}}">&#8592; Back to Index</a></div>
<h1>{{escape .DisplayName}}</h1>
{{if .Description}}<p>{{escape .Description}}</p>
{{end}}<table>
<tbody>
<tr><th>Version</th><td><code>{{escape .Version}}</code></td></tr>
<tr><th>Status</th><td><span class="status-{{escape .Status}}">{{escape .Status}}</span></td></tr>
<tr><th>Type</th><td>{{escape .EventType}}</td></tr>
<tr><th>Namespace</th><td>{{escape .Namespace}}</td></tr>
{{if .Owner}}<tr><th>Owner</th><td>{{escape .Owner}}</td></tr>
{{end}}{{if .Tags}}<tr><th>Tags</th><td>{{escape (join .Tags ", ")}}</td></tr>
{{end}}</tbody>
</table>
<h2>Properties</h2>
{{if .Props}}<table>
<thead><tr><th>Property</th><th>Type</th><th>Required</th><th>Description</th></tr></thead>
<tbody>
{{- range .Props}}
<tr>
<td><code>{{escape .Name}}</code></td>
<td>{{escape .PropType}}{{if .Enum}} <em>(enum)</em>{{end}}</td>
<td>{{if .Required}}<span class="required">&#10003; required</span>{{else}}<span class="optional">optional</span>{{end}}</td>
<td>{{escape .Description}}{{if .Enum}}<br><small>Values: <code>{{escape (join .Enum ", ")}}</code></small>{{end}}{{if .Pattern}}<br><small>Pattern: <code>{{escape .Pattern}}</code></small>{{end}}{{if .HasMinimum}}<br><small>Min: {{.Minimum}}</small>{{end}}{{if .HasMaximum}}<br><small>Max: {{.Maximum}}</small>{{end}}</td>
</tr>
{{- end}}
</tbody>
</table>
{{else}}<p><em>No properties defined.</em></p>
{{end}}{{if .Changelog}}<h2>Changelog</h2>
<p>{{escape .Changelog}}</p>
{{end}}<div class="footer">Generated by event-spec on {{formatTime .GeneratedAt}}</div>
</body>
</html>
`
