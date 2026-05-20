package main

import (
	"path"
	"strings"

	"event-spec/spec"
)

// applySourceConfig filters defs by the event glob patterns declared in src,
// then deduplicates to one version per event (pinned or latest active).
// When src is nil (no source arg given), only deduplication is applied.
func applySourceConfig(defs []*spec.EventDef, src *spec.SourceDef) []*spec.EventDef {
	filtered := defs
	if src != nil && len(src.Events) > 0 {
		filtered = nil
		for _, def := range defs {
			for _, pattern := range src.Events {
				if matchesEventPattern(pattern, def.Namespace, def.Name) {
					filtered = append(filtered, def)
					break
				}
			}
		}
	}

	var pinning map[string]string
	if src != nil {
		pinning = src.VersionPinning
	}
	return selectVersions(filtered, pinning)
}

// matchesEventPattern reports whether an event identified by namespace/name
// matches the given source event pattern. Supported forms:
//
//	"**"              — matches every event
//	"ecommerce/**"    — matches any event whose namespace is "ecommerce"
//	"ecommerce/*"     — single-level wildcard within the namespace segment
//	"auth/user_login" — exact namespace/name match
func matchesEventPattern(pattern, namespace, name string) bool {
	if pattern == "**" {
		return true
	}
	eventPath := namespace + "/" + name
	if pattern == eventPath {
		return true
	}
	// "ns/**" — match any event in that namespace (or nested namespace).
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return namespace == prefix || strings.HasPrefix(namespace, prefix+"/")
	}
	// Standard single-level glob via path.Match (uses forward-slash semantics).
	ok, err := path.Match(pattern, eventPath)
	return err == nil && ok
}

// selectVersions deduplicates defs to one EventDef per (namespace, name) pair.
// When pinning maps "namespace/name" to a version string, that exact version is
// used. Otherwise the highest active SchemaVer is selected.
func selectVersions(defs []*spec.EventDef, pinning map[string]string) []*spec.EventDef {
	type key struct{ ns, name string }
	groups := map[key][]*spec.EventDef{}
	// Preserve input order for deterministic output.
	var order []key
	for _, def := range defs {
		k := key{def.Namespace, def.Name}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], def)
	}

	out := make([]*spec.EventDef, 0, len(order))
	for _, k := range order {
		versions := groups[k]
		eventKey := k.ns + "/" + k.name

		if pinnedVer, ok := pinning[eventKey]; ok {
			for _, def := range versions {
				if def.Version == pinnedVer {
					out = append(out, def)
					break
				}
			}
			continue
		}

		// No pin: pick the highest active version.
		var best *spec.EventDef
		var bestVer spec.SchemaVer
		for _, def := range versions {
			if def.Status != spec.StatusActive {
				continue
			}
			sv, err := spec.ParseSchemaVer(def.Version)
			if err != nil {
				continue
			}
			if best == nil || spec.CompareSchemaVer(sv, bestVer) > 0 {
				best = def
				bestVer = sv
			}
		}
		if best != nil {
			out = append(out, best)
		}
	}
	return out
}
