package hooks

import "context"

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
