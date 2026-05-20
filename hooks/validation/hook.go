package validation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dejanradmanovic/event-spec/hooks"
	"github.com/dejanradmanovic/event-spec/spec"
)

// ErrDeletedEvent is returned by Before when the event's spec status is deleted.
// Dispatch is blocked entirely to prevent silent data loss when a retired event
// is still being called at a call site.
var ErrDeletedEvent = errors.New("event is deleted and cannot be dispatched")

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
// in the Before stage. It also gates dispatch on the event's lifecycle status:
// deleted events are rejected with ErrDeletedEvent; deprecated events emit a
// structured warning log but proceed normally.
//
// Events with no registered spec (dynamic or migrated events) pass through
// unchanged, so this hook works alongside codegen compile-time safety rather
// than replacing it.
type Hook struct {
	hooks.UnimplementedHook
	lookup LookupFunc
	logger *slog.Logger
}

// New creates a Hook that resolves event specs via lookup.
// logger is optional; if nil, deprecation warnings are silently skipped.
func New(lookup LookupFunc, logger *slog.Logger) *Hook {
	return &Hook{lookup: lookup, logger: logger}
}

// Before gates dispatch on the event's lifecycle status and validates its
// properties against the registered JSON Schema.
//
// Status behaviour:
//   - deleted    → returns ErrDeletedEvent, blocking dispatch entirely
//   - deprecated → emits a structured warning log (if logger is set) and proceeds
//   - draft      → passes through (runtime draft behaviour handled separately)
//   - active     → normal schema validation
//
// Returns a *ValidationError (cancelling dispatch) on schema violations.
// Returns (nil, nil) when the event has no registered spec or when the
// message carries no extractable properties.
func (h *Hook) Before(_ context.Context, hc hooks.HookContext, _ hooks.HookHints) (*hooks.EventEnvelope, error) {
	def, ok := h.lookup(hc.EventName)
	if !ok {
		return nil, nil
	}

	if def.Status == spec.StatusDeleted {
		return nil, fmt.Errorf("event %q: %w", hc.EventName, ErrDeletedEvent)
	}

	if def.Status == spec.StatusDeprecated && h.logger != nil {
		h.logger.Warn("event is deprecated — update call sites", "event", hc.EventName)
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
