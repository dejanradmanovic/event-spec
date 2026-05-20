package hooks

import "context"

// AnalyticsContext carries identity and attribute state through the event pipeline.
// Defined here (not in analytics) so the hooks package remains a leaf with no
// project imports, breaking the potential hooks ↔ analytics circular dependency.
type AnalyticsContext struct {
	UserID      string
	AnonymousID string
	Attributes  map[string]any
}

// EventEnvelope is the mutable event representation passed through Before hooks.
// Return a non-nil *EventEnvelope from Before to replace the event for subsequent
// hooks and providers.
type EventEnvelope struct {
	EventName  string
	Properties map[string]any
	Context    AnalyticsContext
	Metadata   map[string]any // hook-private: routing hints, consent flags, enrichment data
}

// HookContext carries the event being processed through the hook chain.
type HookContext struct {
	Operation string           // "track" | "identify" | "group" | "page" | "alias"
	EventName string           // canonical event name from spec
	Context   AnalyticsContext // merged analytics context at this hook stage
	Message   any              // outbound message type (TrackMessage, IdentifyMessage, etc.)
	Provider  string           // set only in After/Error/Finally; empty in Before
}

// HookHints is an open-ended map for passing hints between the runtime and hooks.
type HookHints map[string]any

// HookResult is the outcome passed to After and Finally hooks.
type HookResult struct {
	Delivered bool
	Dropped   bool
	Error     error
}

// Hook is the middleware interface for the analytics event pipeline.
//
// Ordering is governance-first: api-hooks → client-hooks → invocation-hooks → provider-hooks
// for Before; the reverse for After/Error/Finally.
//
// Before runs once, gating all providers. After/Error/Finally fire once per provider result.
type Hook interface {
	// Before runs once before dispatch. Return a modified *EventEnvelope to replace the event
	// for subsequent hooks and providers. Return a non-nil error to cancel the event entirely.
	Before(ctx context.Context, hc HookContext, hints HookHints) (*EventEnvelope, error)

	// After is called per-provider on success, in reverse hook order.
	After(ctx context.Context, hc HookContext, result HookResult, hints HookHints) error

	// Error is called per-provider on failure, in reverse hook order. Must not return an error.
	Error(ctx context.Context, hc HookContext, err error, hints HookHints)

	// Finally is always called after After or Error (defer semantics), in reverse hook order.
	Finally(ctx context.Context, hc HookContext, result HookResult, hints HookHints)
}

// Chain is a slice of hooks that implements the hook chain executor.
//
// Before runs in forward (governance-first) order: each non-nil *EventEnvelope returned by a
// hook replaces the active event for all subsequent hooks. The first error cancels the chain.
// After, Error, and Finally run in reverse order (provider → invocation → client → api).
type Chain []Hook

// Before runs each hook's Before method in forward order, threading EventEnvelope mutations
// through by updating hc for subsequent hooks. Returns the final mutated *EventEnvelope, or
// nil if no hook mutated the event.
func (c Chain) Before(ctx context.Context, hc HookContext, hints HookHints) (*EventEnvelope, error) {
	var latest *EventEnvelope
	for _, h := range c {
		result, err := h.Before(ctx, hc, hints)
		if err != nil {
			return nil, err
		}
		if result != nil {
			latest = result
			hc.EventName = result.EventName
			hc.Context = result.Context
		}
	}
	return latest, nil
}

// After runs each hook's After method in reverse order (provider → client → api).
func (c Chain) After(ctx context.Context, hc HookContext, result HookResult, hints HookHints) error {
	for i := len(c) - 1; i >= 0; i-- {
		if err := c[i].After(ctx, hc, result, hints); err != nil {
			return err
		}
	}
	return nil
}

// Error runs each hook's Error method in reverse order (provider → client → api).
func (c Chain) Error(ctx context.Context, hc HookContext, err error, hints HookHints) {
	for i := len(c) - 1; i >= 0; i-- {
		c[i].Error(ctx, hc, err, hints)
	}
}

// Finally runs each hook's Finally method in reverse order (provider → client → api).
func (c Chain) Finally(ctx context.Context, hc HookContext, result HookResult, hints HookHints) {
	for i := len(c) - 1; i >= 0; i-- {
		c[i].Finally(ctx, hc, result, hints)
	}
}
