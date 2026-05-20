package analytics_test

import (
	"context"
	"testing"

	"github.com/dejanradmanovic/event-spec/analytics"
)

// TestMerge verifies field-level and attribute-level semantics of Merge.
func TestMerge(t *testing.T) {
	t.Run("override UserID wins", func(t *testing.T) {
		got := analytics.Merge(
			analytics.AnalyticsContext{UserID: "base"},
			analytics.AnalyticsContext{UserID: "override"},
		)
		assertEqual(t, "UserID", "override", got.UserID)
	})

	t.Run("empty override does not clobber base", func(t *testing.T) {
		got := analytics.Merge(
			analytics.AnalyticsContext{UserID: "base", AnonymousID: "anon"},
			analytics.AnalyticsContext{},
		)
		assertEqual(t, "UserID", "base", got.UserID)
		assertEqual(t, "AnonymousID", "anon", got.AnonymousID)
	})

	t.Run("override AnonymousID wins", func(t *testing.T) {
		got := analytics.Merge(
			analytics.AnalyticsContext{AnonymousID: "base-anon"},
			analytics.AnalyticsContext{AnonymousID: "override-anon"},
		)
		assertEqual(t, "AnonymousID", "override-anon", got.AnonymousID)
	})

	t.Run("attributes merged key-by-key, override wins on collision", func(t *testing.T) {
		got := analytics.Merge(
			analytics.AnalyticsContext{Attributes: map[string]any{"a": 1, "b": 2}},
			analytics.AnalyticsContext{Attributes: map[string]any{"b": 99, "c": 3}},
		)
		if got.Attributes["a"] != 1 {
			t.Errorf("a: got %v, want 1", got.Attributes["a"])
		}
		if got.Attributes["b"] != 99 {
			t.Errorf("b: got %v, want 99 (override wins)", got.Attributes["b"])
		}
		if got.Attributes["c"] != 3 {
			t.Errorf("c: got %v, want 3", got.Attributes["c"])
		}
	})

	t.Run("nil attributes in both yields nil result", func(t *testing.T) {
		got := analytics.Merge(analytics.AnalyticsContext{}, analytics.AnalyticsContext{})
		if got.Attributes != nil {
			t.Error("expected nil Attributes when both sides are nil")
		}
	})

	t.Run("base attributes preserved when override has none", func(t *testing.T) {
		got := analytics.Merge(
			analytics.AnalyticsContext{Attributes: map[string]any{"x": "y"}},
			analytics.AnalyticsContext{},
		)
		if got.Attributes["x"] != "y" {
			t.Errorf("x: got %v, want %q", got.Attributes["x"], "y")
		}
	})
}

func TestWithAnalyticsContext_roundtrip(t *testing.T) {
	tx := analytics.TransactionContext{
		UserID:      "u1",
		AnonymousID: "a1",
		Attributes:  map[string]any{"key": "val"},
	}
	ctx := analytics.WithAnalyticsContext(context.Background(), tx)

	got, ok := analytics.TransactionContextFrom(ctx)
	if !ok {
		t.Fatal("TransactionContextFrom: expected ok=true")
	}
	assertEqual(t, "UserID", tx.UserID, got.UserID)
	assertEqual(t, "AnonymousID", tx.AnonymousID, got.AnonymousID)
	if got.Attributes["key"] != "val" {
		t.Errorf("Attributes[key]: got %v, want %q", got.Attributes["key"], "val")
	}
}

func TestTransactionContextFrom_missingReturnsNotOk(t *testing.T) {
	_, ok := analytics.TransactionContextFrom(context.Background())
	if ok {
		t.Error("expected ok=false on a plain context.Background()")
	}
}

func assertEqual(t *testing.T, field, want, got string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}
