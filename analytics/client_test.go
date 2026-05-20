package analytics_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"event-spec/analytics"
	"event-spec/hooks"
	"event-spec/testutil"
)

// resetGlobal clears all global analytics state between tests.
func resetGlobal(t *testing.T) {
	t.Helper()
	analytics.SetGlobalContext(analytics.AnalyticsContext{})
	if err := analytics.SetGlobalProvider(); err != nil {
		t.Fatal(err)
	}
}

// ---- basic dispatch ----

func TestTrack_singleProvider(t *testing.T) {
	cap := testutil.NewCaptureProvider("test")
	client := analytics.NewClient(analytics.WithProviders(cap))

	err := client.Track(context.Background(), analytics.Event{
		Name:       "Button Clicked",
		Properties: map[string]any{"button": "submit"},
	})
	if err != nil {
		t.Fatalf("Track error: %v", err)
	}
	if len(cap.Tracks) != 1 {
		t.Fatalf("expected 1 track call, got %d", len(cap.Tracks))
	}
	if cap.Tracks[0].EventName != "Button Clicked" {
		t.Errorf("EventName: got %q, want %q", cap.Tracks[0].EventName, "Button Clicked")
	}
	if cap.Tracks[0].Properties["button"] != "submit" {
		t.Errorf("Properties[button]: got %v", cap.Tracks[0].Properties["button"])
	}
}

func TestTrack_multiProvider(t *testing.T) {
	a := testutil.NewCaptureProvider("alpha")
	b := testutil.NewCaptureProvider("beta")
	client := analytics.NewClient(analytics.WithProviders(a, b))

	if err := client.Track(context.Background(), analytics.Event{Name: "Ping"}); err != nil {
		t.Fatalf("Track error: %v", err)
	}
	if len(a.Tracks) != 1 {
		t.Errorf("alpha: expected 1 track, got %d", len(a.Tracks))
	}
	if len(b.Tracks) != 1 {
		t.Errorf("beta: expected 1 track, got %d", len(b.Tracks))
	}
}

func TestTrackDetailed_partialFailure(t *testing.T) {
	ok := testutil.NewCaptureProvider("ok")
	fail := testutil.NewCaptureProvider("fail")
	fail.TrackErr = errors.New("provider down")

	client := analytics.NewClient(analytics.WithProviders(ok, fail))
	result, err := client.TrackDetailed(context.Background(), analytics.Event{Name: "Ping"})
	if err != nil {
		t.Fatalf("unexpected pre-dispatch error: %v", err)
	}
	if !result.PartialSuccess {
		t.Error("expected PartialSuccess=true")
	}
	if len(result.Success) != 1 {
		t.Errorf("Success: got %d, want 1", len(result.Success))
	}
	if len(result.Failed) != 1 {
		t.Errorf("Failed: got %d, want 1", len(result.Failed))
	}
	if result.Failed[0].ProviderName != "fail" {
		t.Errorf("Failed provider: got %q, want %q", result.Failed[0].ProviderName, "fail")
	}
}

func TestTrackDetailed_allFail(t *testing.T) {
	p := testutil.NewCaptureProvider("bad")
	p.TrackErr = errors.New("boom")
	client := analytics.NewClient(analytics.WithProviders(p))

	result, err := client.TrackDetailed(context.Background(), analytics.Event{Name: "E"})
	if err != nil {
		t.Fatalf("unexpected pre-dispatch error: %v", err)
	}
	if result.PartialSuccess {
		t.Error("expected PartialSuccess=false when all providers fail")
	}
	if len(result.Failed) != 1 {
		t.Errorf("Failed: got %d, want 1", len(result.Failed))
	}
}

// ---- 16 combinations of 4-level context chain ----

// contextChainCombo describes one of the 16 combinations.
type contextChainCombo struct {
	setGlobal bool
	setTx     bool
	setClient bool
	setInvoc  bool
	wantUser  string
}

func buildCombos() []contextChainCombo {
	const (
		global = "global"
		tx     = "tx"
		client = "client"
		invoc  = "invocation"
	)
	var out []contextChainCombo
	for g := 0; g < 2; g++ {
		for t := 0; t < 2; t++ {
			for c := 0; c < 2; c++ {
				for i := 0; i < 2; i++ {
					combo := contextChainCombo{
						setGlobal: g == 1,
						setTx:     t == 1,
						setClient: c == 1,
						setInvoc:  i == 1,
					}
					switch {
					case i == 1:
						combo.wantUser = invoc
					case c == 1:
						combo.wantUser = client
					case t == 1:
						combo.wantUser = tx
					case g == 1:
						combo.wantUser = global
					}
					out = append(out, combo)
				}
			}
		}
	}
	return out
}

// TestContextChain_16Combinations verifies all 16 combinations of the 4-level precedence chain.
// Higher-priority levels must override lower-priority levels for UserID.
func TestContextChain_16Combinations(t *testing.T) {
	const (
		valGlobal = "global"
		valTx     = "tx"
		valClient = "client"
		valInvoc  = "invocation"
	)

	for _, tc := range buildCombos() {
		name := fmt.Sprintf("g=%v_tx=%v_cl=%v_inv=%v", tc.setGlobal, tc.setTx, tc.setClient, tc.setInvoc)
		t.Run(name, func(t *testing.T) {
			resetGlobal(t)

			// Level 1: global
			if tc.setGlobal {
				analytics.SetGlobalContext(analytics.AnalyticsContext{UserID: valGlobal})
			}

			// Level 3: client
			var clientOpts []analytics.ClientOption
			if tc.setClient {
				clientOpts = append(clientOpts, analytics.WithContext(analytics.AnalyticsContext{UserID: valClient}))
			}
			cap := testutil.NewCaptureProvider("cap")
			clientOpts = append(clientOpts, analytics.WithProviders(cap))
			client := analytics.NewClient(clientOpts...)

			// Level 2: transaction (context.Context)
			ctx := context.Background()
			if tc.setTx {
				ctx = analytics.WithAnalyticsContext(ctx, analytics.TransactionContext{UserID: valTx})
			}

			// Level 4: invocation override
			var trackOpts []analytics.TrackOption
			if tc.setInvoc {
				trackOpts = append(trackOpts, analytics.WithContextOverride(analytics.AnalyticsContext{UserID: valInvoc}))
			}

			if err := client.Track(ctx, analytics.Event{Name: "E"}, trackOpts...); err != nil {
				t.Fatalf("Track: %v", err)
			}
			if len(cap.Tracks) != 1 {
				t.Fatalf("expected 1 track call, got %d", len(cap.Tracks))
			}
			gotUser := cap.Tracks[0].UserID
			if gotUser != tc.wantUser {
				t.Errorf("UserID: got %q, want %q", gotUser, tc.wantUser)
			}
		})
	}
}

// ---- hook chain ----

// orderHook records which stage was called and in what order.
type orderHook struct {
	hooks.UnimplementedHook
	name   string
	events *[]string
}

func (h *orderHook) Before(_ context.Context, _ hooks.HookContext, _ hooks.HookHints) (*hooks.EventEnvelope, error) {
	*h.events = append(*h.events, h.name+":before")
	return nil, nil
}

func (h *orderHook) After(_ context.Context, _ hooks.HookContext, _ hooks.HookResult, _ hooks.HookHints) error {
	*h.events = append(*h.events, h.name+":after")
	return nil
}

func (h *orderHook) Finally(_ context.Context, _ hooks.HookContext, _ hooks.HookResult, _ hooks.HookHints) {
	*h.events = append(*h.events, h.name+":finally")
}

func TestHookChain_beforeOrder(t *testing.T) {
	resetGlobal(t)

	var events []string
	h1 := &orderHook{name: "h1", events: &events}
	h2 := &orderHook{name: "h2", events: &events}

	cap := testutil.NewCaptureProvider("cap")
	client := analytics.NewClient(
		analytics.WithHooks(h1, h2),
		analytics.WithProviders(cap),
	)

	if err := client.Track(context.Background(), analytics.Event{Name: "E"}); err != nil {
		t.Fatalf("Track: %v", err)
	}

	// Before hooks run in registration order.
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %v", events)
	}
	if events[0] != "h1:before" {
		t.Errorf("first event: got %q, want %q", events[0], "h1:before")
	}
	if events[1] != "h2:before" {
		t.Errorf("second event: got %q, want %q", events[1], "h2:before")
	}
}

func TestHookChain_afterReverseOrder(t *testing.T) {
	resetGlobal(t)

	var events []string
	h1 := &orderHook{name: "h1", events: &events}
	h2 := &orderHook{name: "h2", events: &events}

	cap := testutil.NewCaptureProvider("cap")
	client := analytics.NewClient(
		analytics.WithHooks(h1, h2),
		analytics.WithProviders(cap),
	)

	if err := client.Track(context.Background(), analytics.Event{Name: "E"}); err != nil {
		t.Fatalf("Track: %v", err)
	}

	// After hooks run in reverse order: h2, then h1.
	afterEvents := []string{}
	for _, e := range events {
		if e == "h2:after" || e == "h1:after" {
			afterEvents = append(afterEvents, e)
		}
	}
	if len(afterEvents) < 2 {
		t.Fatalf("expected 2 after events, got %v", afterEvents)
	}
	if afterEvents[0] != "h2:after" {
		t.Errorf("first after: got %q, want %q", afterEvents[0], "h2:after")
	}
	if afterEvents[1] != "h1:after" {
		t.Errorf("second after: got %q, want %q", afterEvents[1], "h1:after")
	}
}

// cancelHook cancels the event by returning an error from Before.
type cancelHook struct {
	hooks.UnimplementedHook
}

func (cancelHook) Before(_ context.Context, _ hooks.HookContext, _ hooks.HookHints) (*hooks.EventEnvelope, error) {
	return nil, errors.New("consent denied")
}

func TestHookChain_beforeCancelStopsDispatch(t *testing.T) {
	cap := testutil.NewCaptureProvider("cap")
	client := analytics.NewClient(
		analytics.WithHooks(cancelHook{}),
		analytics.WithProviders(cap),
	)

	err := client.Track(context.Background(), analytics.Event{Name: "E"})
	if err == nil {
		t.Fatal("expected error from cancelled hook, got nil")
	}
	if len(cap.Tracks) != 0 {
		t.Errorf("provider should not receive event when hook cancels: got %d calls", len(cap.Tracks))
	}
}

// mutateHook replaces the event name via the EventEnvelope.
type mutateHook struct {
	hooks.UnimplementedHook
	newName string
}

func (h *mutateHook) Before(_ context.Context, hc hooks.HookContext, _ hooks.HookHints) (*hooks.EventEnvelope, error) {
	return &hooks.EventEnvelope{
		EventName:  h.newName,
		Properties: map[string]any{},
		Context:    hc.Context,
	}, nil
}

func TestHookChain_beforeMutatesEventName(t *testing.T) {
	cap := testutil.NewCaptureProvider("cap")
	client := analytics.NewClient(
		analytics.WithHooks(&mutateHook{newName: "Renamed Event"}),
		analytics.WithProviders(cap),
	)

	if err := client.Track(context.Background(), analytics.Event{Name: "Original"}); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if len(cap.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(cap.Tracks))
	}
	if cap.Tracks[0].EventName != "Renamed Event" {
		t.Errorf("EventName: got %q, want %q", cap.Tracks[0].EventName, "Renamed Event")
	}
}

// ---- other call types ----

func TestIdentify(t *testing.T) {
	cap := testutil.NewCaptureProvider("cap")
	client := analytics.NewClient(analytics.WithProviders(cap))

	err := client.Identify(context.Background(), "user-1", map[string]any{"plan": "pro"})
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if len(cap.Identifies) != 1 {
		t.Fatalf("expected 1 identify, got %d", len(cap.Identifies))
	}
	if cap.Identifies[0].UserID != "user-1" {
		t.Errorf("UserID: got %q, want %q", cap.Identifies[0].UserID, "user-1")
	}
	if cap.Identifies[0].Traits["plan"] != "pro" {
		t.Errorf("Traits[plan]: got %v", cap.Identifies[0].Traits["plan"])
	}
}

func TestGroup(t *testing.T) {
	cap := testutil.NewCaptureProvider("cap")
	client := analytics.NewClient(analytics.WithProviders(cap))

	err := client.Group(context.Background(), "group-1", map[string]any{"name": "Acme"})
	if err != nil {
		t.Fatalf("Group: %v", err)
	}
	if len(cap.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(cap.Groups))
	}
	if cap.Groups[0].GroupID != "group-1" {
		t.Errorf("GroupID: got %q, want %q", cap.Groups[0].GroupID, "group-1")
	}
}

func TestPage(t *testing.T) {
	cap := testutil.NewCaptureProvider("cap")
	client := analytics.NewClient(analytics.WithProviders(cap))

	err := client.Page(context.Background(), "Home", map[string]any{"referrer": "google"})
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	if len(cap.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(cap.Pages))
	}
	if cap.Pages[0].Name != "Home" {
		t.Errorf("Name: got %q, want %q", cap.Pages[0].Name, "Home")
	}
}

func TestAlias(t *testing.T) {
	cap := testutil.NewCaptureProvider("cap")
	client := analytics.NewClient(analytics.WithProviders(cap))

	err := client.Alias(context.Background(), "new-id", "old-id")
	if err != nil {
		t.Fatalf("Alias: %v", err)
	}
	if len(cap.Aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(cap.Aliases))
	}
	if cap.Aliases[0].UserID != "new-id" {
		t.Errorf("UserID: got %q, want %q", cap.Aliases[0].UserID, "new-id")
	}
	if cap.Aliases[0].PreviousID != "old-id" {
		t.Errorf("PreviousID: got %q, want %q", cap.Aliases[0].PreviousID, "old-id")
	}
}

// ---- WithTransaction ----

func TestWithTransaction(t *testing.T) {
	resetGlobal(t)

	cap := testutil.NewCaptureProvider("cap")
	base := analytics.NewClient(analytics.WithProviders(cap))
	scoped := base.WithTransaction(analytics.TransactionContext{UserID: "req-user", AnonymousID: "req-anon"})

	if err := scoped.Track(context.Background(), analytics.Event{Name: "E"}); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if cap.Tracks[0].UserID != "req-user" {
		t.Errorf("UserID: got %q, want %q", cap.Tracks[0].UserID, "req-user")
	}
	if cap.Tracks[0].AnonymousID != "req-anon" {
		t.Errorf("AnonymousID: got %q, want %q", cap.Tracks[0].AnonymousID, "req-anon")
	}
}

// ---- global API ----

func TestGlobalTrack(t *testing.T) {
	resetGlobal(t)

	cap := testutil.NewCaptureProvider("cap")
	if err := analytics.SetGlobalProvider(cap); err != nil {
		t.Fatal(err)
	}

	if err := analytics.Track(context.Background(), analytics.Event{Name: "Global Event"}); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if len(cap.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(cap.Tracks))
	}
}

// ---- MessageContext propagation ----

func TestTrack_messageContextFromAttributes(t *testing.T) {
	cap := testutil.NewCaptureProvider("test")
	client := analytics.NewClient(
		analytics.WithProviders(cap),
		analytics.WithContext(analytics.AnalyticsContext{
			UserID: "user-1",
			Attributes: map[string]any{
				"ip_address": "1.2.3.4",
				"user_agent": "Mozilla/5.0",
				"locale":     "en-US",
				"timezone":   "UTC",
				"session_id": "sess-abc",
			},
		}),
	)

	if err := client.Track(context.Background(), analytics.Event{Name: "Page Loaded"}); err != nil {
		t.Fatalf("Track: %v", err)
	}

	mc := cap.Tracks[0].MessageContext
	if mc.IPAddress != "1.2.3.4" {
		t.Errorf("IPAddress = %q, want 1.2.3.4", mc.IPAddress)
	}
	if mc.UserAgent != "Mozilla/5.0" {
		t.Errorf("UserAgent = %q, want Mozilla/5.0", mc.UserAgent)
	}
	if mc.Locale != "en-US" {
		t.Errorf("Locale = %q, want en-US", mc.Locale)
	}
	if mc.Timezone != "UTC" {
		t.Errorf("Timezone = %q, want UTC", mc.Timezone)
	}
	if mc.Extra["session_id"] != "sess-abc" {
		t.Errorf("Extra[session_id] = %v, want sess-abc", mc.Extra["session_id"])
	}
}

func TestIdentify_messageContextFromAttributes(t *testing.T) {
	cap := testutil.NewCaptureProvider("test")
	client := analytics.NewClient(
		analytics.WithProviders(cap),
		analytics.WithContext(analytics.AnalyticsContext{
			Attributes: map[string]any{"session_id": "sess-xyz", "ip_address": "9.8.7.6"},
		}),
	)

	if err := client.Identify(context.Background(), "user-2", nil); err != nil {
		t.Fatalf("Identify: %v", err)
	}

	mc := cap.Identifies[0].MessageContext
	if mc.IPAddress != "9.8.7.6" {
		t.Errorf("IPAddress = %q, want 9.8.7.6", mc.IPAddress)
	}
	if mc.Extra["session_id"] != "sess-xyz" {
		t.Errorf("Extra[session_id] = %v, want sess-xyz", mc.Extra["session_id"])
	}
}

func TestMessageContext_emptyAttributesProducesZeroValue(t *testing.T) {
	cap := testutil.NewCaptureProvider("test")
	client := analytics.NewClient(analytics.WithProviders(cap))

	if err := client.Track(context.Background(), analytics.Event{Name: "Minimal"}); err != nil {
		t.Fatalf("Track: %v", err)
	}

	mc := cap.Tracks[0].MessageContext
	if mc.IPAddress != "" || mc.Locale != "" || mc.Extra != nil {
		t.Errorf("expected zero-value MessageContext for empty attributes, got %+v", mc)
	}
}

// ---- Flush ----

func TestFlush(t *testing.T) {
	cap := testutil.NewCaptureProvider("cap")
	client := analytics.NewClient(analytics.WithProviders(cap))

	if err := client.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if cap.FlushCalls != 1 {
		t.Errorf("FlushCalls: got %d, want 1", cap.FlushCalls)
	}
}
