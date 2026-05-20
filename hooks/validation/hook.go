package validation

import (
	"context"
	"fmt"
	"strings"

	"event-spec/hooks"
	"event-spec/spec"
)

// LookupFunc resolves an event name to its spec definition.
// Return (nil, false) for events without a registered spec; those events skip validation.
type LookupFunc func(eventName string) (*spec.EventDef, bool)

// ValidationError is returned by Before when an event fails schema validation.
// It carries the full list of field-level violations so callers can surface
// exactly which properties failed and why.
type ValidationError struct {
	EventName  string
	Violations []spec.ValidationError
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		if v.Field != "" {
			msgs[i] = fmt.Sprintf("%s: %s", v.Field, v.Message)
		} else {
			msgs[i] = v.Message
		}
	}
	return fmt.Sprintf("event %q failed schema validation: %s", e.EventName, strings.Join(msgs, "; "))
}

// Hook validates event properties against the registered event spec JSON Schema
// in the Before stage. It cancels dispatch by returning a *ValidationError when
// the payload violates the schema.
//
// Events with no registered spec (dynamic or migrated events) pass through
// unchanged, so this hook works alongside codegen compile-time safety rather
// than replacing it.
type Hook struct {
	hooks.UnimplementedHook
	lookup LookupFunc
}

// New creates a Hook that resolves event specs via lookup.
func New(lookup LookupFunc) *Hook {
	return &Hook{lookup: lookup}
}

// Before validates the event properties against the event's JSON Schema.
// Returns a *ValidationError (cancelling dispatch) on schema violations.
// Returns (nil, nil) when the event has no registered spec or when the
// message carries no extractable properties.
func (h *Hook) Before(_ context.Context, hc hooks.HookContext, _ hooks.HookHints) (*hooks.EventEnvelope, error) {
	def, ok := h.lookup(hc.EventName)
	if !ok {
		return nil, nil
	}

	props := extractProperties(hc.Message)
	if props == nil {
		return nil, nil
	}

	errs := spec.ValidateEventPayload(def, props)
	if len(errs) == 0 {
		return nil, nil
	}
	return nil, &ValidationError{EventName: hc.EventName, Violations: errs}
}

// extractProperties retrieves the properties map from the hook message.
// Supports *hooks.EventEnvelope and map[string]any.
func extractProperties(msg any) map[string]any {
	if msg == nil {
		return nil
	}
	switch v := msg.(type) {
	case *hooks.EventEnvelope:
		return v.Properties
	case map[string]any:
		return v
	default:
		return nil
	}
}
