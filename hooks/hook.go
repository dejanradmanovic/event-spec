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

// UnimplementedHook provides no-op implementations of all Hook methods.
// Embed in your hook struct to only override the stages you need.
type UnimplementedHook struct{}

// Before is a no-op that allows the event to proceed unchanged.
func (UnimplementedHook) Before(_ context.Context, _ HookContext, _ HookHints) (*EventEnvelope, error) {
	return nil, nil
}

// After is a no-op. Override in your hook to react to successful provider delivery.
func (UnimplementedHook) After(_ context.Context, _ HookContext, _ HookResult, _ HookHints) error {
	return nil
}

// Error is a no-op. Override in your hook to react to provider delivery failures.
func (UnimplementedHook) Error(_ context.Context, _ HookContext, _ error, _ HookHints) {}

// Finally is a no-op. Override in your hook for cleanup that always runs after dispatch.
func (UnimplementedHook) Finally(_ context.Context, _ HookContext, _ HookResult, _ HookHints) {}
