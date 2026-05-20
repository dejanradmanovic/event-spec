package amplitude_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dejanradmanovic/event-spec/provider"
	"github.com/dejanradmanovic/event-spec/provider/amplitude"
)

// compile-time check: *Provider satisfies provider.Provider.
var _ provider.Provider = (*amplitude.Provider)(nil)

// captureServer returns a test server and a pointer to the slice of decoded requests.
func captureServer(t *testing.T) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var mu sync.Mutex
	var reqs []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		mu.Lock()
		reqs = append(reqs, req)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs
}

// newProvider creates a test Provider pointing at srv with a large batch size so
// events are only sent when Flush is called explicitly.
func newProvider(t *testing.T, srv *httptest.Server, mutate ...func(*amplitude.Config)) *amplitude.Provider {
	t.Helper()
	cfg := amplitude.Config{
		ProviderConfig: provider.ProviderConfig{
			APIKey:        "test-api-key",
			SecretType:    provider.SecretInline,
			BatchSize:     1000,
			FlushInterval: time.Hour,
		},
		Endpoint: srv.URL,
	}
	for _, fn := range mutate {
		fn(&cfg)
	}
	p, err := amplitude.New(cfg)
	if err != nil {
		t.Fatalf("amplitude.New: %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
	return p
}

func firstEvent(t *testing.T, reqs *[]map[string]any) map[string]any {
	t.Helper()
	if len(*reqs) == 0 {
		t.Fatal("no requests captured by test server")
	}
	events, ok := (*reqs)[0]["events"].([]any)
	if !ok || len(events) == 0 {
		t.Fatalf("events field missing or empty in request: %v", (*reqs)[0])
	}
	ev, ok := events[0].(map[string]any)
	if !ok {
		t.Fatalf("event is not a map: %v", events[0])
	}
	return ev
}

// TestMetadata verifies provider name, version, and capability declarations.
func TestMetadata(t *testing.T) {
	srv, _ := captureServer(t)
	p := newProvider(t, srv)
	meta := p.Metadata()

	if meta.Name != "amplitude" {
		t.Errorf("Name = %q, want amplitude", meta.Name)
	}
	if meta.Version == "" {
		t.Error("Version is empty")
	}

	caps := meta.Capabilities
	if !caps.Track {
		t.Error("Capabilities.Track = false, want true")
	}
	if !caps.Identify {
		t.Error("Capabilities.Identify = false, want true")
	}
	if !caps.Group {
		t.Error("Capabilities.Group = false, want true")
	}
	if caps.Page {
		t.Error("Capabilities.Page = true, want false")
	}
	if !caps.Alias {
		t.Error("Capabilities.Alias = false, want true")
	}
}

// TestHooks verifies no provider-level hooks are returned.
func TestHooks(t *testing.T) {
	srv, _ := captureServer(t)
	p := newProvider(t, srv)
	if h := p.Hooks(); len(h) != 0 {
		t.Errorf("Hooks() len = %d, want 0", len(h))
	}
}

// TestTrackBatchPayload verifies the JSON batch format for a track event.
func TestTrackBatchPayload(t *testing.T) {
	srv, reqs := captureServer(t)
	p := newProvider(t, srv)

	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	msg := provider.TrackMessage{
		MessageID:   "msg-001",
		Timestamp:   ts,
		EventName:   "Button Clicked",
		UserID:      "user-42",
		AnonymousID: "anon-99",
		Properties: map[string]any{
			"button": "signup",
			"count":  3.0,
		},
	}
	if err := p.Track(context.Background(), msg); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if err := p.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if (*reqs)[0]["api_key"] != "test-api-key" {
		t.Errorf("api_key = %v, want test-api-key", (*reqs)[0]["api_key"])
	}

	ev := firstEvent(t, reqs)

	if ev["event_type"] != "Button Clicked" {
		t.Errorf("event_type = %v, want Button Clicked", ev["event_type"])
	}
	if ev["user_id"] != "user-42" {
		t.Errorf("user_id = %v, want user-42", ev["user_id"])
	}
	if ev["device_id"] != "anon-99" {
		t.Errorf("device_id = %v, want anon-99", ev["device_id"])
	}
	if ev["insert_id"] != "msg-001" {
		t.Errorf("insert_id = %v, want msg-001", ev["insert_id"])
	}
	if ev["time"] != float64(ts.UnixMilli()) {
		t.Errorf("time = %v, want %d", ev["time"], ts.UnixMilli())
	}

	props, ok := ev["event_properties"].(map[string]any)
	if !ok {
		t.Fatalf("event_properties missing or wrong type: %v", ev["event_properties"])
	}
	if props["button"] != "signup" {
		t.Errorf("event_properties.button = %v, want signup", props["button"])
	}
}

// TestIdentifyPayload verifies the $identify event format and that Identify sends
// synchronously â€” the request is captured without calling Flush.
func TestIdentifyPayload(t *testing.T) {
	srv, reqs := captureServer(t)
	p := newProvider(t, srv)

	msg := provider.IdentifyMessage{
		MessageID: "id-001",
		Timestamp: time.Now(),
		UserID:    "user-1",
		Traits:    map[string]any{"email": "user@example.com"},
	}
	if err := p.Identify(context.Background(), msg); err != nil {
		t.Fatalf("Identify: %v", err)
	}
	// No Flush â€” request must already be delivered synchronously.

	ev := firstEvent(t, reqs)

	if ev["event_type"] != "$identify" {
		t.Errorf("event_type = %v, want $identify", ev["event_type"])
	}
	if ev["user_id"] != "user-1" {
		t.Errorf("user_id = %v, want user-1", ev["user_id"])
	}

	userProps, ok := ev["user_properties"].(map[string]any)
	if !ok {
		t.Fatalf("user_properties missing: %v", ev["user_properties"])
	}
	setMap, ok := userProps["$set"].(map[string]any)
	if !ok {
		t.Fatalf("user_properties.$set missing: %v", userProps)
	}
	if setMap["email"] != "user@example.com" {
		t.Errorf("user_properties.$set.email = %v, want user@example.com", setMap["email"])
	}
}

// TestGroupPayload verifies the $groupidentify event format and that Group sends
// synchronously â€” the request is captured without calling Flush.
func TestGroupPayload(t *testing.T) {
	srv, reqs := captureServer(t)
	p := newProvider(t, srv)

	msg := provider.GroupMessage{
		MessageID: "grp-001",
		Timestamp: time.Now(),
		UserID:    "user-1",
		GroupID:   "acme-corp",
		Traits:    map[string]any{"plan": "enterprise"},
	}
	if err := p.Group(context.Background(), msg); err != nil {
		t.Fatalf("Group: %v", err)
	}
	// No Flush â€” request must already be delivered synchronously.

	ev := firstEvent(t, reqs)

	if ev["event_type"] != "$groupidentify" {
		t.Errorf("event_type = %v, want $groupidentify", ev["event_type"])
	}
	if ev["group_type"] != "group" {
		t.Errorf("group_type = %v, want group", ev["group_type"])
	}
	if ev["group_value"] != "acme-corp" {
		t.Errorf("group_value = %v, want acme-corp", ev["group_value"])
	}

	groupProps, ok := ev["group_properties"].(map[string]any)
	if !ok {
		t.Fatalf("group_properties missing: %v", ev["group_properties"])
	}
	setMap, ok := groupProps["$set"].(map[string]any)
	if !ok {
		t.Fatalf("group_properties.$set missing: %v", groupProps)
	}
	if setMap["plan"] != "enterprise" {
		t.Errorf("group_properties.$set.plan = %v, want enterprise", setMap["plan"])
	}
}

// TestAliasPayload verifies that Alias sends a $identify event linking
// PreviousID (device_id) to UserID synchronously â€” no Flush required.
func TestAliasPayload(t *testing.T) {
	srv, reqs := captureServer(t)
	p := newProvider(t, srv)

	msg := provider.AliasMessage{
		MessageID:  "alias-001",
		Timestamp:  time.Now(),
		UserID:     "user-new",
		PreviousID: "anon-old",
	}
	if err := p.Alias(context.Background(), msg); err != nil {
		t.Fatalf("Alias: %v", err)
	}
	// No Flush â€” request must already be delivered synchronously.

	ev := firstEvent(t, reqs)

	if ev["event_type"] != "$identify" {
		t.Errorf("event_type = %v, want $identify", ev["event_type"])
	}
	if ev["user_id"] != "user-new" {
		t.Errorf("user_id = %v, want user-new", ev["user_id"])
	}
	if ev["device_id"] != "anon-old" {
		t.Errorf("device_id = %v, want anon-old", ev["device_id"])
	}
}

// TestPageUnsupported verifies that Page returns ErrUnsupportedOperation.
func TestPageUnsupported(t *testing.T) {
	srv, _ := captureServer(t)
	p := newProvider(t, srv)
	err := p.Page(context.Background(), provider.PageMessage{})
	if !errors.Is(err, provider.ErrUnsupportedOperation) {
		t.Errorf("Page() error = %v, want ErrUnsupportedOperation", err)
	}
}

// TestContextPropertiesPassThrough verifies MessageContext.Extra appears in event_properties.
func TestContextPropertiesPassThrough(t *testing.T) {
	srv, reqs := captureServer(t)
	p := newProvider(t, srv)

	msg := provider.TrackMessage{
		EventName: "ctx_test",
		UserID:    "user-1",
		MessageContext: provider.MessageContext{
			IPAddress: "1.2.3.4",
			Locale:    "en-US",
			OS:        map[string]any{"name": "iOS"},
			App:       map[string]any{"platform": "mobile"},
			Extra:     map[string]any{"session_id": "sess-abc"},
		},
	}
	if err := p.Track(context.Background(), msg); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if err := p.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	ev := firstEvent(t, reqs)

	if ev["ip"] != "1.2.3.4" {
		t.Errorf("ip = %v, want 1.2.3.4", ev["ip"])
	}
	if ev["language"] != "en-US" {
		t.Errorf("language = %v, want en-US", ev["language"])
	}
	if ev["os_name"] != "iOS" {
		t.Errorf("os_name = %v, want iOS", ev["os_name"])
	}
	if ev["platform"] != "mobile" {
		t.Errorf("platform = %v, want mobile", ev["platform"])
	}

	props, ok := ev["event_properties"].(map[string]any)
	if !ok {
		t.Fatalf("event_properties missing: %v", ev["event_properties"])
	}
	if props["session_id"] != "sess-abc" {
		t.Errorf("event_properties.session_id = %v, want sess-abc", props["session_id"])
	}
}

// TestStringTruncation verifies that property strings over 1024 chars are truncated.
func TestStringTruncation(t *testing.T) {
	srv, reqs := captureServer(t)
	p := newProvider(t, srv)

	longVal := strings.Repeat("a", 2000)
	msg := provider.TrackMessage{
		EventName:  "trunc_test",
		UserID:     "user-1",
		Properties: map[string]any{"long": longVal},
	}
	if err := p.Track(context.Background(), msg); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if err := p.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	ev := firstEvent(t, reqs)
	props := ev["event_properties"].(map[string]any)
	got, ok := props["long"].(string)
	if !ok {
		t.Fatalf("long property not a string: %v", props["long"])
	}
	if len([]rune(got)) != 1024 {
		t.Errorf("truncated length = %d runes, want 1024", len([]rune(got)))
	}
}

// TestProxyRouting verifies that requests are routed to the proxy URL rather than
// directly to the Amplitude endpoint when ProxyReverseProxy mode is configured.
func TestProxyRouting(t *testing.T) {
	var mu sync.Mutex
	var requestedPaths []string

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestedPaths = append(requestedPaths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200}`))
	}))
	defer proxy.Close()

	p, err := amplitude.New(amplitude.Config{
		ProviderConfig: provider.ProviderConfig{
			APIKey:        "proxy-key",
			SecretType:    provider.SecretInline,
			ProxyURL:      proxy.URL,
			ProxyMode:     provider.ProxyReverseProxy,
			BatchSize:     1000,
			FlushInterval: time.Hour,
		},
		// Endpoint is left empty â€” defaults to https://api2.amplitude.com/batch.
		// ProxyReverseProxy rewrites the host to the proxy server.
	})
	if err != nil {
		t.Fatalf("amplitude.New: %v", err)
	}
	defer func() { _ = p.Shutdown(context.Background()) }()

	if err := p.Track(context.Background(), provider.TrackMessage{
		EventName: "proxy_test_event",
		UserID:    "user-1",
	}); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if err := p.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requestedPaths) == 0 {
		t.Fatal("no requests reached the proxy server; expected at least one")
	}
	// The path should be the Amplitude batch endpoint path, not empty.
	if requestedPaths[0] == "" {
		t.Errorf("proxy received request with empty path, want /batch")
	}
}

// TestNew_missingEnvVar verifies that New returns an error when the env var is not set.
func TestNew_missingEnvVar(t *testing.T) {
	_, err := amplitude.New(amplitude.Config{
		ProviderConfig: provider.ProviderConfig{
			APIKey:     "${AMPLITUDE_KEY_DEFINITELY_NOT_SET_XYZ}",
			SecretType: provider.SecretEnvVar,
		},
	})
	if err == nil {
		t.Error("New() error = nil, want non-nil for missing env var")
	}
}

// TestBatchContainsMultipleEvents verifies that multiple Track calls produce
// a single batch request with all events when Flush is called.
func TestBatchContainsMultipleEvents(t *testing.T) {
	srv, reqs := captureServer(t)
	p := newProvider(t, srv)

	for i := 0; i < 5; i++ {
		if err := p.Track(context.Background(), provider.TrackMessage{
			EventName: "multi_event",
			UserID:    "user-1",
		}); err != nil {
			t.Fatalf("Track %d: %v", i, err)
		}
	}
	if err := p.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if len(*reqs) == 0 {
		t.Fatal("no requests captured")
	}
	events, ok := (*reqs)[0]["events"].([]any)
	if !ok {
		t.Fatalf("events field missing: %v", (*reqs)[0]["events"])
	}
	if len(events) != 5 {
		t.Errorf("events count = %d, want 5", len(events))
	}
}
