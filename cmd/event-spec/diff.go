package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
	"github.com/spf13/cobra"
)

// exitCodeError carries a specific OS exit code for the diff command.
// A code of 1 signals version inconsistency; code 2 signals a spec or parse error.
type exitCodeError struct {
	code int
	err  error
}

func (e *exitCodeError) Error() string { return e.err.Error() }
func (e *exitCodeError) Unwrap() error { return e.err }

// diffResult holds the output of a single event diff operation.
type diffResult struct {
	Namespace   string
	Name        string
	FromVersion string
	ToVersion   string
	Changes     []spec.Change
	BumpErr     error
	Suggested   spec.SchemaVer
}

func newDiffCmd() *cobra.Command {
	var (
		breakingOnly bool
		format       string
		sourceName   string
	)

	cmd := &cobra.Command{
		Use:   "diff [from.yaml to.yaml | ns/name [from-ver to-ver]]",
		Short: "Show changes between two event spec versions and validate the version bump",
		Long: `Show changes between two event spec versions and validate that the declared
version bump is consistent with the detected changes.

Mode 1 — explicit file paths (no registry required):
  event-spec diff ./specs/ecommerce/product_viewed/1-0-0.yaml \
                  ./specs/ecommerce/product_viewed/1-2-0.yaml

Mode 2 — registry-aware with explicit versions:
  event-spec diff ecommerce/product_viewed 1-0-0 1-2-0

Mode 3 — defaults to latest two active versions:
  event-spec diff ecommerce/product_viewed

Mode 4 — inherits event list from workspace source config:
  event-spec diff [--source web-app]`,
		Args: cobra.RangeArgs(0, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch len(args) {
			case 2:
				return runDiffFiles(cmd, args[0], args[1], breakingOnly, format)
			case 3:
				return runDiffNSNameVersions(cmd, args[0], args[1], args[2], breakingOnly, format)
			case 1:
				return runDiffNSNameLatest(cmd, args[0], breakingOnly, format)
			default:
				return runDiffAll(cmd, sourceName, breakingOnly, format)
			}
		},
	}

	cmd.Flags().BoolVar(&breakingOnly, "breaking", false, "only display breaking changes")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text | json")
	cmd.Flags().StringVar(&sourceName, "source", "", "source name to resolve event list from (mode 4)")

	return cmd
}

// runDiffFiles implements Mode 1: diff two explicit YAML file paths.
func runDiffFiles(cmd *cobra.Command, fromPath, toPath string, breakingOnly bool, format string) error {
	from, err := spec.LoadEventDef(fromPath)
	if err != nil {
		return &exitCodeError{code: 2, err: fmt.Errorf("load %s: %w", fromPath, err)}
	}
	to, err := spec.LoadEventDef(toPath)
	if err != nil {
		return &exitCodeError{code: 2, err: fmt.Errorf("load %s: %w", toPath, err)}
	}
	return renderSingleDiff(cmd, from, to, breakingOnly, format)
}

// runDiffNSNameVersions implements Mode 2: diff by ns/name and explicit versions via registry.
func runDiffNSNameVersions(cmd *cobra.Command, nsName, fromVer, toVer string, breakingOnly bool, format string) error {
	reg, err := openDiffRegistry()
	if err != nil {
		return &exitCodeError{code: 2, err: err}
	}
	ns, name, err := splitNSName(nsName)
	if err != nil {
		return &exitCodeError{code: 2, err: err}
	}
	ctx := context.Background()
	from, err := reg.GetEvent(ctx, ns, name, fromVer)
	if err != nil {
		return &exitCodeError{code: 2, err: fmt.Errorf("get event %s@%s: %w", nsName, fromVer, err)}
	}
	to, err := reg.GetEvent(ctx, ns, name, toVer)
	if err != nil {
		return &exitCodeError{code: 2, err: fmt.Errorf("get event %s@%s: %w", nsName, toVer, err)}
	}
	return renderSingleDiff(cmd, from, to, breakingOnly, format)
}

// runDiffNSNameLatest implements Mode 3: diff the two latest active versions of an event.
func runDiffNSNameLatest(cmd *cobra.Command, nsName string, breakingOnly bool, format string) error {
	reg, err := openDiffRegistry()
	if err != nil {
		return &exitCodeError{code: 2, err: err}
	}
	ns, name, err := splitNSName(nsName)
	if err != nil {
		return &exitCodeError{code: 2, err: err}
	}
	from, to, err := findLatestTwo(reg, ns, name)
	if err != nil {
		return &exitCodeError{code: 2, err: err}
	}
	return renderSingleDiff(cmd, from, to, breakingOnly, format)
}

// runDiffAll implements Mode 4: diff events from one or all workspace sources.
func runDiffAll(cmd *cobra.Command, sourceName string, breakingOnly bool, format string) error {
	cfg, err := spec.LoadWorkspaceConfig("event-spec.yaml")
	if err != nil {
		return &exitCodeError{code: 2, err: fmt.Errorf("load event-spec.yaml: %w", err)}
	}

	reg, err := openRegistry(cfg)
	if err != nil {
		return &exitCodeError{code: 2, err: err}
	}

	sourcesDir := cfg.SourcesDir
	if sourcesDir == "" {
		sourcesDir = "./sources"
	}

	var sources []*spec.SourceDef
	if sourceName != "" {
		src, loadErr := spec.LoadSourceDef(filepath.Join(sourcesDir, sourceName+".yaml"))
		if loadErr != nil {
			return &exitCodeError{code: 2, err: fmt.Errorf("load source %q: %w", sourceName, loadErr)}
		}
		sources = []*spec.SourceDef{src}
	} else {
		all, errs := spec.WalkSourceDefs(sourcesDir)
		if len(errs) > 0 {
			return &exitCodeError{code: 2, err: errs[0]}
		}
		if len(all) == 0 {
			return &exitCodeError{code: 2, err: fmt.Errorf("no sources found in %s", sourcesDir)}
		}
		sources = all
	}

	// Collect unique (ns, name) pairs across all sources (deduped).
	type eventKey struct{ ns, name string }
	type pinnedEvent struct {
		ns, name   string
		pinnedFrom string // empty if no pinning
	}
	seen := map[eventKey]bool{}
	var events []pinnedEvent

	allDefs, err := reg.ListEvents(context.Background(), registry.ListFilter{})
	if err != nil {
		return &exitCodeError{code: 2, err: fmt.Errorf("list events: %w", err)}
	}

	for _, src := range sources {
		filtered := applySourceConfig(ptrSlice(allDefs), src)
		for _, def := range filtered {
			k := eventKey{def.Namespace, def.Name}
			if seen[k] {
				continue
			}
			seen[k] = true
			pe := pinnedEvent{ns: def.Namespace, name: def.Name}
			// If version_pinning is set for this event, use pinned as from.
			if src.VersionPinning != nil {
				pe.pinnedFrom = src.VersionPinning[def.Namespace+"/"+def.Name]
			}
			events = append(events, pe)
		}
	}

	if len(events) == 0 {
		return &exitCodeError{code: 2, err: fmt.Errorf("no events found in sources")}
	}

	var results []diffResult
	for _, pe := range events {
		var from, to *spec.EventDef
		if pe.pinnedFrom != "" {
			var ferr, terr error
			from, ferr = reg.GetEvent(context.Background(), pe.ns, pe.name, pe.pinnedFrom)
			to, terr = reg.GetEvent(context.Background(), pe.ns, pe.name, "")
			if ferr != nil || terr != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s/%s: skip (%v / %v)\n", pe.ns, pe.name, ferr, terr)
				continue
			}
		} else {
			var ferr error
			from, to, ferr = findLatestTwo(reg, pe.ns, pe.name)
			if ferr != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s/%s: %v\n", pe.ns, pe.name, ferr)
				continue
			}
		}
		results = append(results, buildDiffResult(from, to))
	}

	return renderResults(cmd, results, breakingOnly, format)
}

// renderSingleDiff runs Diff + ValidateVersionBump on a single (from, to) pair
// and outputs results. Returns exitCodeError{1} on version inconsistency.
func renderSingleDiff(cmd *cobra.Command, from, to *spec.EventDef, breakingOnly bool, format string) error {
	res := buildDiffResult(from, to)
	if err := renderResults(cmd, []diffResult{res}, breakingOnly, format); err != nil {
		return err
	}
	return nil
}

// buildDiffResult runs Diff and ValidateVersionBump, returning a populated diffResult.
func buildDiffResult(from, to *spec.EventDef) diffResult {
	changes := spec.Diff(from, to)
	fromVer, _ := spec.ParseSchemaVer(from.Version)
	res := diffResult{
		Namespace:   from.Namespace,
		Name:        from.Name,
		FromVersion: from.Version,
		ToVersion:   to.Version,
		Changes:     changes,
		Suggested:   spec.SuggestVersion(fromVer, changes),
	}
	res.BumpErr = spec.ValidateVersionBump(from, to, changes)
	return res
}

// renderResults outputs one or more diffResult values and returns an exitCodeError
// with code 1 if any result has a version inconsistency.
func renderResults(cmd *cobra.Command, results []diffResult, breakingOnly bool, format string) error {
	hasInconsistency := false
	for _, r := range results {
		if r.BumpErr != nil {
			hasInconsistency = true
		}
	}

	switch format {
	case "json":
		if err := renderJSON(cmd, results, breakingOnly); err != nil {
			return &exitCodeError{code: 2, err: err}
		}
	default:
		renderText(cmd, results, breakingOnly)
	}

	if hasInconsistency {
		// Collect messages for the returned error.
		var msgs []string
		for _, r := range results {
			if r.BumpErr != nil {
				msgs = append(msgs, r.BumpErr.Error())
			}
		}
		return &exitCodeError{
			code: 1,
			err:  fmt.Errorf("%s", strings.Join(msgs, "; ")),
		}
	}
	return nil
}

// renderText writes human-readable diff output to cmd's stdout.
func renderText(cmd *cobra.Command, results []diffResult, breakingOnly bool) {
	out := cmd.OutOrStdout()
	multi := len(results) > 1

	for i, r := range results {
		if multi {
			if i > 0 {
				_, _ = fmt.Fprintln(out)
			}
			_, _ = fmt.Fprintf(out, "=== %s/%s: %s → %s ===\n", r.Namespace, r.Name, r.FromVersion, r.ToVersion)
		}

		displayed := r.Changes
		if breakingOnly {
			var breaking []spec.Change
			for _, c := range r.Changes {
				if c.Breaking {
					breaking = append(breaking, c)
				}
			}
			displayed = breaking
		}

		if len(displayed) == 0 && !breakingOnly {
			_, _ = fmt.Fprintln(out, "(no changes)")
		}

		for _, c := range displayed {
			status := changeStatus(c)
			prop := c.Property
			if prop == "" {
				prop = fmt.Sprintf("%q → %q", c.From, c.To)
			}
			_, _ = fmt.Fprintf(out, "%-10s %-22s %s\n", status, string(c.Kind), prop)
		}

		_, _ = fmt.Fprintln(out)
		if r.BumpErr != nil {
			_, _ = fmt.Fprintf(out, "Version: declared %s, required %s — ERROR\n", r.ToVersion, r.Suggested.Raw)
		} else if len(r.Changes) > 0 {
			_, _ = fmt.Fprintf(out, "Version: declared %s — ok\n", r.ToVersion)
		} else {
			_, _ = fmt.Fprintf(out, "Version: no changes\n")
		}
	}
}

// changeStatus returns the display label for a change.
func changeStatus(c spec.Change) string {
	if c.Breaking {
		return "BREAKING"
	}
	if c.Kind == spec.ChangeDescriptionOnly {
		return "INFO"
	}
	return "OK"
}

// jsonDiffResult is the JSON-serialisable form of a diffResult.
type jsonDiffResult struct {
	Namespace       string       `json:"namespace"`
	Name            string       `json:"name"`
	FromVersion     string       `json:"from_version"`
	ToVersion       string       `json:"to_version"`
	Changes         []jsonChange `json:"changes"`
	VersionValid    bool         `json:"version_valid"`
	RequiredVersion string       `json:"required_version,omitempty"`
	Error           string       `json:"error,omitempty"`
}

type jsonChange struct {
	Kind     string `json:"kind"`
	Property string `json:"property,omitempty"`
	Breaking bool   `json:"breaking"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
}

func renderJSON(cmd *cobra.Command, results []diffResult, breakingOnly bool) error {
	var out []jsonDiffResult
	for _, r := range results {
		var jcs []jsonChange
		for _, c := range r.Changes {
			if breakingOnly && !c.Breaking {
				continue
			}
			jcs = append(jcs, jsonChange{
				Kind:     string(c.Kind),
				Property: c.Property,
				Breaking: c.Breaking,
				From:     c.From,
				To:       c.To,
			})
		}
		jr := jsonDiffResult{
			Namespace:    r.Namespace,
			Name:         r.Name,
			FromVersion:  r.FromVersion,
			ToVersion:    r.ToVersion,
			Changes:      jcs,
			VersionValid: r.BumpErr == nil,
		}
		if r.BumpErr != nil {
			jr.RequiredVersion = r.Suggested.Raw
			jr.Error = r.BumpErr.Error()
		}
		out = append(out, jr)
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if len(out) == 1 {
		return enc.Encode(out[0])
	}
	return enc.Encode(out)
}

// openDiffRegistry opens a registry from the workspace config, falling back to
// a local registry on ./specs when no workspace config is present.
func openDiffRegistry() (interface {
	GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error)
	ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error)
}, error) {
	cfg, err := spec.LoadWorkspaceConfig("event-spec.yaml")
	if err != nil {
		// No workspace config: fall back to ./specs local registry.
		return openRegistry(&spec.WorkspaceConfig{Registry: spec.RegistryConfig{Mode: spec.RegistryModeLocal}, SpecsDir: "./specs"})
	}
	return openRegistry(cfg)
}

// findLatestTwo finds the two latest active versions of an event in the registry.
func findLatestTwo(reg interface {
	ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error)
}, namespace, name string) (*spec.EventDef, *spec.EventDef, error) {
	all, err := reg.ListEvents(context.Background(), registry.ListFilter{Namespace: namespace, Status: spec.StatusActive})
	if err != nil {
		return nil, nil, fmt.Errorf("list events: %w", err)
	}

	var matches []spec.EventDef
	for _, def := range all {
		if def.Name == name {
			matches = append(matches, def)
		}
	}

	if len(matches) < 2 {
		return nil, nil, fmt.Errorf("event %s/%s: need at least 2 active versions, found %d", namespace, name, len(matches))
	}

	sort.Slice(matches, func(i, j int) bool {
		vi, _ := spec.ParseSchemaVer(matches[i].Version)
		vj, _ := spec.ParseSchemaVer(matches[j].Version)
		return spec.CompareSchemaVer(vi, vj) < 0
	})

	n := len(matches)
	return &matches[n-2], &matches[n-1], nil
}

// splitNSName splits "namespace/name" into its two parts.
func splitNSName(nsName string) (namespace, name string, err error) {
	idx := strings.LastIndex(nsName, "/")
	if idx < 0 || idx == 0 || idx == len(nsName)-1 {
		return "", "", fmt.Errorf("invalid ns/name %q: expected format namespace/event_name", nsName)
	}
	return nsName[:idx], nsName[idx+1:], nil
}

// ptrSlice converts []EventDef to []*EventDef for use with applySourceConfig.
func ptrSlice(defs []spec.EventDef) []*spec.EventDef {
	out := make([]*spec.EventDef, len(defs))
	for i := range defs {
		out[i] = &defs[i]
	}
	return out
}
