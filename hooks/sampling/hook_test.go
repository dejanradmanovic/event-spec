package sampling_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"event-spec/hooks"
	"event-spec/hooks/sampling"
	"event-spec/spec"
)

func TestSamplingHook_Before_noSamplingConfig_passesThrough(t *testing.T) {
	h := sampling.New(lookupFor(eventWithoutSampling()))
	_, err := h.Before(context.Background(), trackHC("Event A", "user-1"), nil)
	if err != nil {
		t.Fatalf("unexpected error for event with no sampling config: %v", err)
	}
}

func TestSamplingHook_Before_strategyNone_passesThrough(t *testing.T) {
	h := sampling.New(lookupFor(eventWithSampling(spec.SamplingNone, 0.5)))
	_, err := h.Before(context.Background(), trackHC("Event A", "user-1"), nil)
	if err != nil {
		t.Fatalf("strategy=none should pass through, got: %v", err)
	}
}

func TestSamplingHook_Before_unknownEvent_passesThrough(t *testing.T) {
	h := sampling.New(func(_ string) (*spec.EventDef, bool) { return nil, false })
	_, err := h.Before(context.Background(), trackHC("Unknown Event", "user-1"), nil)
	if err != nil {
		t.Fatalf("unknown event should pass through, got: %v", err)
	}
}

func TestSamplingHook_Before_hashStrategy_deterministic(t *testing.T) {
	h := sampling.New(lookupFor(eventWithSampling(spec.SamplingUserIDHash, 0.5)))
	hc := trackHC("Event A", "user-deterministic-42")

	_, firstErr := h.Before(context.Background(), hc, nil)
	for i := 0; i < 20; i++ {
		_, err := h.Before(context.Background(), hc, nil)
		if errors.Is(err, sampling.ErrSampled) != errors.Is(firstErr, sampling.ErrSampled) {
			t.Fatalf("call %d: non-deterministic result for same user_id", i+1)
		}
	}
}

func TestSamplingHook_Before_hashStrategy_zeroRate_dropsAll(t *testing.T) {
	h := sampling.New(lookupFor(eventWithSampling(spec.SamplingUserIDHash, 0.0)))
	for i := 0; i < 50; i++ {
		_, err := h.Before(context.Background(), trackHC("Event A", fmt.Sprintf("user-%d", i)), nil)
		if !errors.Is(err, sampling.ErrSampled) {
			t.Fatalf("rate=0.0 should drop all events, user-%d was kept", i)
		}
	}
}

func TestSamplingHook_Before_hashStrategy_fullRate_keepsAll(t *testing.T) {
	h := sampling.New(lookupFor(eventWithSampling(spec.SamplingUserIDHash, 1.0)))
	for i := 0; i < 50; i++ {
		_, err := h.Before(context.Background(), trackHC("Event A", fmt.Sprintf("user-%d", i)), nil)
		if errors.Is(err, sampling.ErrSampled) {
			t.Fatalf("rate=1.0 should keep all events, user-%d was dropped", i)
		}
	}
}

func TestSamplingHook_Before_hashStrategy_distribution(t *testing.T) {
	h := sampling.New(lookupFor(eventWithSampling(spec.SamplingUserIDHash, 0.5)))

	const trials = 2000
	kept := 0
	for i := 0; i < trials; i++ {
		_, err := h.Before(context.Background(), trackHC("Event A", fmt.Sprintf("user-%d", i)), nil)
		if !errors.Is(err, sampling.ErrSampled) {
			kept++
		}
	}

	// At 50% rate, expect 40–60% kept (wide tolerance for hash distribution).
	rate := float64(kept) / float64(trials)
	if rate < 0.40 || rate > 0.60 {
		t.Errorf("hash sampling at 50%% rate: %.1f%% kept over %d trials, want 40–60%%", rate*100, trials)
	}
}

func TestSamplingHook_Before_randomStrategy_distribution(t *testing.T) {
	h := sampling.New(lookupFor(eventWithSampling(spec.SamplingRandom, 0.1)))

	const trials = 10000
	kept := 0
	for i := 0; i < trials; i++ {
		_, err := h.Before(context.Background(), trackHC("Event A", fmt.Sprintf("user-%d", i)), nil)
		if !errors.Is(err, sampling.ErrSampled) {
			kept++
		}
	}

	// At 10% rate, expect 8–12% kept (±2% absolute tolerance over 10k trials).
	rate := float64(kept) / float64(trials)
	if rate < 0.08 || rate > 0.12 {
		t.Errorf("random sampling at 10%% rate: %.2f%% kept over %d trials, want 8–12%%", rate*100, trials)
	}
}

func TestSamplingHook_Before_droppedEvent_returnsSampledError(t *testing.T) {
	h := sampling.New(lookupFor(eventWithSampling(spec.SamplingUserIDHash, 0.0)))
	_, err := h.Before(context.Background(), trackHC("Event A", "any-user"), nil)
	if !errors.Is(err, sampling.ErrSampled) {
		t.Errorf("expected ErrSampled, got %v", err)
	}
}

// ---- helpers ----

func trackHC(eventName, userID string) hooks.HookContext {
	return hooks.HookContext{
		Operation: "track",
		EventName: eventName,
		Context:   hooks.AnalyticsContext{UserID: userID},
		Message:   &hooks.EventEnvelope{EventName: eventName},
	}
}

func lookupFor(def *spec.EventDef) sampling.LookupFunc {
	return func(eventName string) (*spec.EventDef, bool) {
		if eventName == def.EventName {
			return def, true
		}
		return nil, false
	}
}

func eventWithSampling(strategy spec.SamplingStrategy, rate float64) *spec.EventDef {
	return &spec.EventDef{
		Name:      "event_a",
		EventName: "Event A",
		Version:   "1-0-0",
		Status:    spec.StatusActive,
		Type:      spec.TypeTrack,
		Sampling:  &spec.SamplingConfig{Strategy: strategy, Rate: rate},
	}
}

func eventWithoutSampling() *spec.EventDef {
	return &spec.EventDef{
		Name:      "event_a",
		EventName: "Event A",
		Version:   "1-0-0",
		Status:    spec.StatusActive,
		Type:      spec.TypeTrack,
	}
}
