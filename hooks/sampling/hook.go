package sampling

import (
	"context"
	"errors"
	"hash/fnv"
	"math/rand"

	"event-spec/hooks"
	"event-spec/spec"
)

// ErrSampled is returned by Before when the event is dropped by the sampling policy.
// The dispatch runtime converts this to a Dropped delivery state rather than a failure.
var ErrSampled = errors.New("event dropped by sampling policy")

// LookupFunc resolves an event name to its spec definition.
// Return (nil, false) for events without a declared sampling policy; those pass through unchanged.
type LookupFunc func(eventName string) (*spec.EventDef, bool)

// Hook applies the sampling policy declared in the event spec during the Before stage.
// Sampled-out events return ErrSampled; the dispatch runtime records these as Dropped.
type Hook struct {
	hooks.UnimplementedHook
	lookup LookupFunc
}

// New creates a Hook that resolves event specs via lookup.
func New(lookup LookupFunc) *Hook {
	return &Hook{lookup: lookup}
}

// Before applies the event's declared sampling strategy.
// Returns (nil, ErrSampled) when the event is dropped; (nil, nil) to pass it through.
func (h *Hook) Before(_ context.Context, hc hooks.HookContext, _ hooks.HookHints) (*hooks.EventEnvelope, error) {
	def, ok := h.lookup(hc.EventName)
	if !ok {
		return nil, nil
	}

	cfg := def.Sampling
	if cfg == nil || cfg.Strategy == spec.SamplingNone || cfg.Strategy == "" {
		return nil, nil
	}

	var keep bool
	switch cfg.Strategy {
	case spec.SamplingUserIDHash:
		keep = hashKeep(hc.Context.UserID, cfg.Rate)
	case spec.SamplingRandom:
		keep = rand.Float64() < cfg.Rate
	default:
		return nil, nil
	}

	if !keep {
		return nil, ErrSampled
	}
	return nil, nil
}

// hashKeep returns true when userID hashes into the kept fraction [0, rate).
// The decision is deterministic: the same userID always yields the same result.
func hashKeep(userID string, rate float64) bool {
	h := fnv.New32a()
	_, _ = h.Write([]byte(userID))
	norm := float64(h.Sum32()) / float64(1<<32)
	return norm < rate
}
