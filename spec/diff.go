package spec

import (
	"fmt"
	"sort"
	"strings"
)

// ChangeKind classifies the type of difference detected between two event versions.
type ChangeKind string

// Valid ChangeKind values and their SchemaVer impact (MAJOR, MINOR, or PATCH).
const (
	ChangeAddRequiredProp ChangeKind = "add_required_prop" // MAJOR
	ChangeRemoveProp      ChangeKind = "remove_prop"       // MAJOR
	ChangeRenameProp      ChangeKind = "rename_prop"       // MAJOR
	ChangeTypeChanged     ChangeKind = "type_changed"      // MAJOR
	ChangeMakeRequired    ChangeKind = "make_required"     // MAJOR
	ChangeRemoveEnumValue ChangeKind = "remove_enum_value" // MAJOR
	ChangeRenameEvent     ChangeKind = "rename_event"      // MAJOR — event_name or name changed
	ChangeMakeOptional    ChangeKind = "make_optional"     // MINOR
	ChangeAddOptionalProp ChangeKind = "add_optional_prop" // MINOR
	ChangeAddEnumValue    ChangeKind = "add_enum_value"    // MINOR
	ChangeDescriptionOnly ChangeKind = "description_only"  // PATCH — description field changed only
)

// Change describes a single detected difference between two event spec versions.
type Change struct {
	Kind       ChangeKind
	Property   string
	Breaking   bool
	From, To   string
	Suggestion string // suggested new SchemaVer consistent with the detected change
}

// Diff compares two EventDef versions and returns all detected changes.
// from is the older version; to is the newer candidate.
func Diff(from, to *EventDef) []Change {
	var changes []Change

	// Detect event rename: either the internal name or the canonical event_name changed.
	if from.Name != to.Name || from.EventName != to.EventName {
		changes = append(changes, Change{
			Kind:     ChangeRenameEvent,
			Breaking: true,
			From:     from.EventName,
			To:       to.EventName,
		})
	}

	fromProps := from.Properties
	if fromProps == nil {
		fromProps = map[string]PropertyDef{}
	}
	toProps := to.Properties
	if toProps == nil {
		toProps = map[string]PropertyDef{}
	}

	// Build sorted key lists for deterministic output.
	fromNames := sortedPropKeys(fromProps)
	toNames := sortedPropKeys(toProps)

	removedNames := sliceDiff(fromNames, toNames) // in from but not in to
	addedNames := sliceDiff(toNames, fromNames)   // in to but not in from

	// Rename detection: pair each removed property with the first unmatched added
	// property that shares the same type.
	renamedFrom := map[string]bool{}
	renamedTo := map[string]bool{}
	for _, remName := range removedNames {
		remProp := fromProps[remName]
		for _, addName := range addedNames {
			if renamedTo[addName] {
				continue
			}
			if fromProps[remName].Type == toProps[addName].Type {
				_ = remProp // used for type comparison above
				renamedFrom[remName] = true
				renamedTo[addName] = true
				changes = append(changes, Change{
					Kind:     ChangeRenameProp,
					Property: remName,
					Breaking: true,
					From:     remName,
					To:       addName,
				})
				break
			}
		}
	}

	// Removed properties not consumed by rename matching.
	for _, name := range removedNames {
		if renamedFrom[name] {
			continue
		}
		changes = append(changes, Change{
			Kind:     ChangeRemoveProp,
			Property: name,
			Breaking: true,
		})
	}

	// Added properties not consumed by rename matching.
	for _, name := range addedNames {
		if renamedTo[name] {
			continue
		}
		prop := toProps[name]
		if prop.Required {
			changes = append(changes, Change{
				Kind:     ChangeAddRequiredProp,
				Property: name,
				Breaking: true,
			})
		} else {
			changes = append(changes, Change{
				Kind:     ChangeAddOptionalProp,
				Property: name,
				Breaking: false,
			})
		}
	}

	// Per-property changes for properties present in both from and to.
	for _, name := range fromNames {
		if _, inTo := toProps[name]; !inTo {
			continue // handled above
		}
		changes = append(changes, diffProp(name, fromProps[name], toProps[name])...)
	}

	return changes
}

// diffProp returns the changes between two versions of the same-named property.
// At most one change is returned per structural dimension; description-only is
// reported only when no other change is detected.
func diffProp(name string, from, to PropertyDef) []Change {
	// Type change is the most impactful; if present, skip all other checks.
	if from.Type != to.Type {
		return []Change{{
			Kind:     ChangeTypeChanged,
			Property: name,
			Breaking: true,
			From:     string(from.Type),
			To:       string(to.Type),
		}}
	}

	// Required status changes.
	if !from.Required && to.Required {
		return []Change{{Kind: ChangeMakeRequired, Property: name, Breaking: true}}
	}
	if from.Required && !to.Required {
		return []Change{{Kind: ChangeMakeOptional, Property: name, Breaking: false}}
	}

	// Enum value changes.
	removedEnums := enumDiff(from.Enum, to.Enum)
	addedEnums := enumDiff(to.Enum, from.Enum)
	if len(removedEnums) > 0 {
		return []Change{{
			Kind:     ChangeRemoveEnumValue,
			Property: name,
			Breaking: true,
			From:     strings.Join(removedEnums, ","),
		}}
	}
	if len(addedEnums) > 0 {
		return []Change{{
			Kind:     ChangeAddEnumValue,
			Property: name,
			Breaking: false,
			To:       strings.Join(addedEnums, ","),
		}}
	}

	// Description-only change: reported only when no structural change was found.
	if from.Description != to.Description {
		return []Change{{Kind: ChangeDescriptionOnly, Property: name, Breaking: false}}
	}

	return nil
}

// SuggestVersion returns the minimum valid SchemaVer for to given the
// detected changes and the from version. Returns from unchanged when
// changes is empty.
func SuggestVersion(from SchemaVer, changes []Change) SchemaVer {
	if len(changes) == 0 {
		return from
	}
	maxLevel := 0
	for _, c := range changes {
		if l := changeBumpLevel(c); l > maxLevel {
			maxLevel = l
		}
	}
	switch maxLevel {
	case 2:
		return SchemaVer{Major: from.Major + 1, Raw: fmt.Sprintf("%d-0-0", from.Major+1)}
	case 1:
		return SchemaVer{Major: from.Major, Minor: from.Minor + 1, Raw: fmt.Sprintf("%d-%d-0", from.Major, from.Minor+1)}
	default:
		return SchemaVer{Major: from.Major, Minor: from.Minor, Patch: from.Patch + 1,
			Raw: fmt.Sprintf("%d-%d-%d", from.Major, from.Minor, from.Patch+1)}
	}
}

// ValidateVersionBump returns a non-nil error if the declared version jump is
// inconsistent with the detected changes.
//
// Rules:
//   - to.Version must be strictly greater than from.Version (no downgrade)
//   - declared bump level must be >= minimum required by changes:
//     any Breaking change     → requires MAJOR bump
//     any MINOR change only   → requires at least MINOR bump
//     only PATCH changes      → requires at least PATCH bump
//   - over-bumping (MAJOR declared when only MINOR changes exist) is allowed
func ValidateVersionBump(from, to *EventDef, changes []Change) error {
	fromVer, err := ParseSchemaVer(from.Version)
	if err != nil {
		return fmt.Errorf("invalid from version %q: %w", from.Version, err)
	}
	toVer, err := ParseSchemaVer(to.Version)
	if err != nil {
		return fmt.Errorf("invalid to version %q: %w", to.Version, err)
	}
	if CompareSchemaVer(toVer, fromVer) <= 0 {
		return fmt.Errorf("version must strictly increase: %s is not greater than %s", to.Version, from.Version)
	}
	if len(changes) == 0 {
		return nil
	}

	minLevel := 0
	for _, c := range changes {
		if l := changeBumpLevel(c); l > minLevel {
			minLevel = l
		}
	}

	var declaredLevel int
	switch {
	case toVer.Major > fromVer.Major:
		declaredLevel = 2
	case toVer.Minor > fromVer.Minor:
		declaredLevel = 1
	default:
		declaredLevel = 0
	}

	if declaredLevel < minLevel {
		levelNames := []string{"PATCH", "MINOR", "MAJOR"}
		suggested := SuggestVersion(fromVer, changes)
		return fmt.Errorf("declared %s bump insufficient: changes require at least %s; suggested version %s",
			levelNames[declaredLevel], levelNames[minLevel], suggested.Raw)
	}
	return nil
}

// changeBumpLevel returns the minimum SchemaVer bump level required by a change:
// 0 = PATCH, 1 = MINOR, 2 = MAJOR.
func changeBumpLevel(c Change) int {
	if c.Breaking {
		return 2
	}
	if c.Kind == ChangeDescriptionOnly {
		return 0
	}
	return 1
}

// sortedPropKeys returns the keys of a property map in sorted order.
func sortedPropKeys(m map[string]PropertyDef) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sliceDiff returns elements present in a but absent from b, preserving a's order.
func sliceDiff(a, b []string) []string {
	bSet := make(map[string]bool, len(b))
	for _, s := range b {
		bSet[s] = true
	}
	var out []string
	for _, s := range a {
		if !bSet[s] {
			out = append(out, s)
		}
	}
	return out
}

// enumDiff returns enum values present in a but absent from b.
func enumDiff(a, b []string) []string {
	bSet := make(map[string]bool, len(b))
	for _, s := range b {
		bSet[s] = true
	}
	var out []string
	for _, s := range a {
		if !bSet[s] {
			out = append(out, s)
		}
	}
	return out
}
