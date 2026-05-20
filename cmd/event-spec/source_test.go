package main

import (
	"testing"

	"github.com/dejanradmanovic/event-spec/spec"
)

// --- matchesEventPattern ---

func TestMatchesEventPattern(t *testing.T) {
	cases := []struct {
		pattern   string
		namespace string
		name      string
		want      bool
	}{
		// exact match
		{"auth/user_signed_up", "auth", "user_signed_up", true},
		{"auth/user_signed_up", "auth", "user_login", false},
		// double-star catches all
		{"**", "anything", "goes", true},
		// namespace wildcard
		{"ecommerce/**", "ecommerce", "product_viewed", true},
		{"ecommerce/**", "ecommerce", "order_completed", true},
		{"ecommerce/**", "auth", "user_signed_up", false},
		// single-star wildcard within namespace
		{"ecommerce/*", "ecommerce", "product_viewed", true},
		{"ecommerce/*", "auth", "product_viewed", false},
		// no false positive on partial prefix
		{"eco/**", "ecommerce", "product_viewed", false},
	}

	for _, tc := range cases {
		got := matchesEventPattern(tc.pattern, tc.namespace, tc.name)
		if got != tc.want {
			t.Errorf("matchesEventPattern(%q, %q, %q) = %v, want %v",
				tc.pattern, tc.namespace, tc.name, got, tc.want)
		}
	}
}

// --- selectVersions ---

func makeEvent(ns, name, version string, status spec.EventStatus) *spec.EventDef {
	return &spec.EventDef{
		Namespace: ns,
		Name:      name,
		Version:   version,
		Status:    status,
		EventName: name,
	}
}

func TestSelectVersions_LatestActive(t *testing.T) {
	defs := []*spec.EventDef{
		makeEvent("ec", "pv", "1-0-0", spec.StatusActive),
		makeEvent("ec", "pv", "1-2-0", spec.StatusActive),
		makeEvent("ec", "pv", "2-0-0", spec.StatusDeprecated),
	}
	got := selectVersions(defs, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].Version != "1-2-0" {
		t.Errorf("expected latest active 1-2-0, got %s", got[0].Version)
	}
}

func TestSelectVersions_PinnedVersion(t *testing.T) {
	defs := []*spec.EventDef{
		makeEvent("ec", "pv", "1-0-0", spec.StatusActive),
		makeEvent("ec", "pv", "1-2-0", spec.StatusActive),
	}
	pinning := map[string]string{"ec/pv": "1-0-0"}
	got := selectVersions(defs, pinning)
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].Version != "1-0-0" {
		t.Errorf("expected pinned 1-0-0, got %s", got[0].Version)
	}
}

func TestSelectVersions_SkipsIfPinnedVersionMissing(t *testing.T) {
	defs := []*spec.EventDef{
		makeEvent("ec", "pv", "1-0-0", spec.StatusActive),
	}
	// Pinned to a version that doesn't exist in the registry.
	pinning := map[string]string{"ec/pv": "9-9-9"}
	got := selectVersions(defs, pinning)
	if len(got) != 0 {
		t.Errorf("expected 0 events when pinned version is missing, got %d", len(got))
	}
}

func TestSelectVersions_SkipsInactiveWhenNoPin(t *testing.T) {
	defs := []*spec.EventDef{
		makeEvent("ec", "pv", "1-0-0", spec.StatusDeprecated),
		makeEvent("ec", "pv", "2-0-0", spec.StatusDraft),
	}
	got := selectVersions(defs, nil)
	if len(got) != 0 {
		t.Errorf("expected 0 events (all inactive), got %d", len(got))
	}
}

func TestSelectVersions_MultipleEvents(t *testing.T) {
	defs := []*spec.EventDef{
		makeEvent("ec", "pv", "1-0-0", spec.StatusActive),
		makeEvent("ec", "pv", "1-2-0", spec.StatusActive),
		makeEvent("auth", "signup", "1-0-0", spec.StatusActive),
	}
	got := selectVersions(defs, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2 distinct events, got %d", len(got))
	}
}

// --- applySourceConfig ---

func TestApplySourceConfig_NilSrc_DeduplicatesVersions(t *testing.T) {
	defs := []*spec.EventDef{
		makeEvent("ec", "pv", "1-0-0", spec.StatusActive),
		makeEvent("ec", "pv", "1-2-0", spec.StatusActive),
	}
	got := applySourceConfig(defs, nil)
	if len(got) != 1 || got[0].Version != "1-2-0" {
		t.Errorf("expected single latest-active event, got %v", got)
	}
}

func TestApplySourceConfig_FiltersAndPins(t *testing.T) {
	defs := []*spec.EventDef{
		makeEvent("ecommerce", "product_viewed", "1-0-0", spec.StatusActive),
		makeEvent("ecommerce", "product_viewed", "1-2-0", spec.StatusActive),
		makeEvent("auth", "user_signed_up", "1-0-0", spec.StatusActive),
		makeEvent("internal", "debug_event", "1-0-0", spec.StatusActive),
	}
	src := &spec.SourceDef{
		Events: []string{"ecommerce/**", "auth/user_signed_up"},
		VersionPinning: map[string]string{
			"ecommerce/product_viewed": "1-0-0",
		},
	}
	got := applySourceConfig(defs, src)

	// Expect: product_viewed@1-0-0 (pinned) + user_signed_up@1-0-0; internal excluded.
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	byKey := map[string]string{}
	for _, d := range got {
		byKey[d.Namespace+"/"+d.Name] = d.Version
	}
	if byKey["ecommerce/product_viewed"] != "1-0-0" {
		t.Errorf("expected pinned 1-0-0 for product_viewed, got %s", byKey["ecommerce/product_viewed"])
	}
	if byKey["auth/user_signed_up"] != "1-0-0" {
		t.Errorf("expected 1-0-0 for user_signed_up, got %s", byKey["auth/user_signed_up"])
	}
	if _, found := byKey["internal/debug_event"]; found {
		t.Error("internal/debug_event should be excluded by source event patterns")
	}
}

func TestApplySourceConfig_EmptyPatterns_IncludesAll(t *testing.T) {
	defs := []*spec.EventDef{
		makeEvent("ec", "pv", "1-0-0", spec.StatusActive),
		makeEvent("auth", "signup", "1-0-0", spec.StatusActive),
	}
	src := &spec.SourceDef{Events: []string{}} // no filter
	got := applySourceConfig(defs, src)
	if len(got) != 2 {
		t.Errorf("empty events list should include all events, got %d", len(got))
	}
}
