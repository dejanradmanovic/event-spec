package hooks_test

import (
	"context"
	"errors"
	"testing"

	"event-spec/hooks"
)

// recorder is a test Hook that appends "name:stage" to a shared event log for each stage called.
type recorder struct {
	hooks.UnimplementedHook
	name   string
	events *[]string
}

func (r *recorder) Before(_ context.Context, _ hooks.HookContext, _ hooks.HookHints) (*hooks.EventEnvelope, error) {
	*r.events = append(*r.events, r.name+":before")
	return nil, nil
}

func (r *recorder) After(_ context.Context, _ hooks.HookContext, _ hooks.HookResult, _ hooks.HookHints) error {
	*r.events = append(*r.events, r.name+":after")
	return nil
}

func (r *recorder) Error(_ context.Context, _ hooks.HookContext, _ error, _ hooks.HookHints) {
	*r.events = append(*r.events, r.name+":error")
}

func (r *recorder) Finally(_ context.Context, _ hooks.HookContext, _ hooks.HookResult, _ hooks.HookHints) {
	*r.events = append(*r.events, r.name+":finally")
}

func baseHC() hooks.HookContext {
	return hooks.HookContext{Operation: "track", EventName: "Test Event"}
}

// ---- Chain.Before ----

func TestChain_Before_forwardOrder(t *testing.T) {
	var events []string
	chain := hooks.Chain{
		&recorder{name: "h1", events: &events},
		&recorder{name: "h2", events: &events},
		&recorder{name: "h3", events: &events},
	}

	if _, err := chain.Before(context.Background(), baseHC(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"h1:before", "h2:before", "h3:before"}
	assertEvents(t, events, want)
}

func TestChain_Before_cancellationStopsChain(t *testing.T) {
	var events []string
	sentinel := errors.New("consent denied")

	chain := hooks.Chain{
		&cancelHook{err: sentinel},
		&recorder{name: "never", events: &events},
	}

	_, err := chain.Before(context.Background(), baseHC(), nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("error: got %v, want %v", err, sentinel)
	}
	if len(events) != 0 {
		t.Errorf("subsequent hooks must not run after cancellation: got %v", events)
	}
}

func TestChain_Before_mutationPropagatesEventName(t *testing.T) {
	var seen string
	chain := hooks.Chain{
		&renameHook{newName: "Renamed"},
		&eventNameObserver{seen: &seen},
	}

	if _, err := chain.Before(context.Background(), baseHC(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seen != "Renamed" {
		t.Errorf("second hook hc.EventName: got %q, want %q", seen, "Renamed")
	}
}

func TestChain_Before_returnsFinalEnvelope(t *testing.T) {
	chain := hooks.Chain{
		&renameHook{newName: "First"},
		&renameHook{newName: "Second"},
	}

	env, err := chain.Before(context.Background(), baseHC(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env == nil {
		t.Fatal("expected non-nil envelope")
	}
	if env.EventName != "Second" {
		t.Errorf("EventName: got %q, want %q", env.EventName, "Second")
	}
}

func TestChain_Before_noMutation_returnsNil(t *testing.T) {
	var events []string
	chain := hooks.Chain{
		&recorder{name: "h1", events: &events},
	}

	env, err := chain.Before(context.Background(), baseHC(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env != nil {
		t.Errorf("expected nil envelope when no hook mutates, got %+v", env)
	}
}

// ---- Chain.After ----

func TestChain_After_reverseOrder(t *testing.T) {
	var events []string
	chain := hooks.Chain{
		&recorder{name: "h1", events: &events},
		&recorder{name: "h2", events: &events},
		&recorder{name: "h3", events: &events},
	}

	if err := chain.After(context.Background(), baseHC(), hooks.HookResult{Delivered: true}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEvents(t, events, []string{"h3:after", "h2:after", "h1:after"})
}

// ---- Chain.Error ----

func TestChain_Error_reverseOrder(t *testing.T) {
	var events []string
	chain := hooks.Chain{
		&recorder{name: "h1", events: &events},
		&recorder{name: "h2", events: &events},
	}

	chain.Error(context.Background(), baseHC(), errors.New("provider down"), nil)
	assertEvents(t, events, []string{"h2:error", "h1:error"})
}

// ---- Chain.Finally ----

func TestChain_Finally_reverseOrder(t *testing.T) {
	var events []string
	chain := hooks.Chain{
		&recorder{name: "h1", events: &events},
		&recorder{name: "h2", events: &events},
	}

	chain.Finally(context.Background(), baseHC(), hooks.HookResult{Delivered: true}, nil)
	assertEvents(t, events, []string{"h2:finally", "h1:finally"})
}

// ---- helpers ----

func assertEvents(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("event count: got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("events[%d]: got %q, want %q", i, got[i], w)
		}
	}
}

// cancelHook cancels the Before chain by returning an error.
type cancelHook struct {
	hooks.UnimplementedHook
	err error
}

func (h *cancelHook) Before(_ context.Context, _ hooks.HookContext, _ hooks.HookHints) (*hooks.EventEnvelope, error) {
	return nil, h.err
}

// renameHook replaces EventName via the returned *EventEnvelope.
type renameHook struct {
	hooks.UnimplementedHook
	newName string
}

func (h *renameHook) Before(_ context.Context, hc hooks.HookContext, _ hooks.HookHints) (*hooks.EventEnvelope, error) {
	return &hooks.EventEnvelope{
		EventName:  h.newName,
		Context:    hc.Context,
		Properties: map[string]any{},
	}, nil
}

// eventNameObserver records the EventName from HookContext that it receives.
type eventNameObserver struct {
	hooks.UnimplementedHook
	seen *string
}

func (h *eventNameObserver) Before(_ context.Context, hc hooks.HookContext, _ hooks.HookHints) (*hooks.EventEnvelope, error) {
	*h.seen = hc.EventName
	return nil, nil
}
