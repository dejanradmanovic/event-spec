package eventspec_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/dejanradmanovic/event-spec/provider"
	eventspec "github.com/dejanradmanovic/event-spec/provider/event-spec"
)

// compile-time interface check.
var _ provider.Provider = (*eventspec.ServerProvider)(nil)

// testServer captures requests from the provider under test.
type testServer struct {
	mu   sync.Mutex
	reqs []capturedRequest
	srv  *httptest.Server
}

type capturedRequest struct {
	method string
	path   string
	auth   string
	body   map[string]any
}

func newTestServer(t *testing.T, status int) *testServer {
	t.Helper()
	ts := &testServer{}
	ts.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		ts.mu.Lock()
		ts.reqs = append(ts.reqs, capturedRequest{
			method: r.Method,
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
			body:   parsed,
		})
		ts.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(ts.srv.Close)
	return ts
}

func (ts *testServer) captured() []capturedRequest {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	out := make([]capturedRequest, len(ts.reqs))
	copy(out, ts.reqs)
	return out
}

func newProvider(t *testing.T, srv *httptest.Server) *eventspec.ServerProvider {
	t.Helper()
	p, err := eventspec.New(eventspec.Config{
		APIKey:  "test-api-key",
		BaseURL: srv.URL,
		Source:  "test-source",
	})
	if err != nil {
		t.Fatalf("eventspec.New: %v", err)
	}
	return p
}

// TestMetadata verifies provider name, version, and capability declarations.
func TestMetadata(t *testing.T) {
	ts := newTestServer(t, http.StatusOK)
	p := newProvider(t, ts.srv)

	meta := p.Metadata()
	if meta.Name != "event-spec" {
		t.Errorf("Name = %q, want event-spec", meta.Name)
	}
	if meta.Version == "" {
		t.Error("Version is empty")
	}
	caps := meta.Capabilities
	if !caps.Track {
		t.Error("Capabilities.Track = false")
	}
	if !caps.Identify {
		t.Error("Capabilities.Identify = false")
	}
	if !caps.Group {
		t.Error("Capabilities.Group = false")
	}
	if !caps.Page {
		t.Error("Capabilities.Page = false")
	}
	if !caps.Alias {
		t.Error("Capabilities.Alias = false")
	}
}

// TestHooks verifies the provider returns no provider-level hooks.
func TestHooks(t *testing.T) {
	ts := newTestServer(t, http.StatusOK)
	p := newProvider(t, ts.srv)
	if h := p.Hooks(); h != nil {
		t.Errorf("Hooks() = %v, want nil", h)
	}
}

// TestTrack verifies the track endpoint, auth header, and JSON body.
func TestTrack(t *testing.T) {
	ts := newTestServer(t, http.StatusAccepted)
	p := newProvider(t, ts.srv)

	ts2 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	msg := provider.TrackMessage{
		MessageID:   "msg-001",
		Timestamp:   ts2,
		EventName:   "Button Clicked",
		UserID:      "user-42",
		AnonymousID: "anon-99",
		Properties:  map[string]any{"button": "signup"},
	}
	if err := p.Track(context.Background(), msg); err != nil {
		t.Fatalf("Track: %v", err)
	}

	reqs := ts.captured()
	if len(reqs) != 1 {
		t.Fatalf("captured %d requests, want 1", len(reqs))
	}
	req := reqs[0]

	if req.path != "/v1/track" {
		t.Errorf("path = %q, want /v1/track", req.path)
	}
	if req.auth != "Bearer test-api-key" {
		t.Errorf("Authorization = %q, want Bearer test-api-key", req.auth)
	}
	if req.body["source"] != "test-source" {
		t.Errorf("source = %v, want test-source", req.body["source"])
	}
	if req.body["event_name"] != "Button Clicked" {
		t.Errorf("event_name = %v, want Button Clicked", req.body["event_name"])
	}
	ctx, ok := req.body["context"].(map[string]any)
	if !ok {
		t.Fatalf("context missing or wrong type: %v", req.body["context"])
	}
	if ctx["user_id"] != "user-42" {
		t.Errorf("context.user_id = %v, want user-42", ctx["user_id"])
	}
	if ctx["anonymous_id"] != "anon-99" {
		t.Errorf("context.anonymous_id = %v, want anon-99", ctx["anonymous_id"])
	}
	props, ok := req.body["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties missing or wrong type: %v", req.body["properties"])
	}
	if props["button"] != "signup" {
		t.Errorf("properties.button = %v, want signup", props["button"])
	}
}

// TestIdentify verifies the identify endpoint and body.
func TestIdentify(t *testing.T) {
	ts := newTestServer(t, http.StatusOK)
	p := newProvider(t, ts.srv)

	msg := provider.IdentifyMessage{
		MessageID: "id-001",
		Timestamp: time.Now(),
		UserID:    "user-1",
		Traits:    map[string]any{"email": "user@example.com"},
	}
	if err := p.Identify(context.Background(), msg); err != nil {
		t.Fatalf("Identify: %v", err)
	}

	reqs := ts.captured()
	if len(reqs) != 1 {
		t.Fatalf("captured %d requests, want 1", len(reqs))
	}
	req := reqs[0]
	if req.path != "/v1/identify" {
		t.Errorf("path = %q, want /v1/identify", req.path)
	}
	if req.body["source"] != "test-source" {
		t.Errorf("source = %v, want test-source", req.body["source"])
	}
	if req.body["user_id"] != "user-1" {
		t.Errorf("user_id = %v, want user-1", req.body["user_id"])
	}
	traits, ok := req.body["traits"].(map[string]any)
	if !ok {
		t.Fatalf("traits missing or wrong type: %v", req.body["traits"])
	}
	if traits["email"] != "user@example.com" {
		t.Errorf("traits.email = %v, want user@example.com", traits["email"])
	}
}

// TestGroup verifies the group endpoint and body.
func TestGroup(t *testing.T) {
	ts := newTestServer(t, http.StatusOK)
	p := newProvider(t, ts.srv)

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

	reqs := ts.captured()
	req := reqs[0]
	if req.path != "/v1/group" {
		t.Errorf("path = %q, want /v1/group", req.path)
	}
	if req.body["group_id"] != "acme-corp" {
		t.Errorf("group_id = %v, want acme-corp", req.body["group_id"])
	}
}

// TestPage verifies the page endpoint and body.
func TestPage(t *testing.T) {
	ts := newTestServer(t, http.StatusOK)
	p := newProvider(t, ts.srv)

	msg := provider.PageMessage{
		MessageID:  "pg-001",
		Timestamp:  time.Now(),
		UserID:     "user-1",
		Name:       "/home",
		Properties: map[string]any{"title": "Home"},
	}
	if err := p.Page(context.Background(), msg); err != nil {
		t.Fatalf("Page: %v", err)
	}

	reqs := ts.captured()
	req := reqs[0]
	if req.path != "/v1/page" {
		t.Errorf("path = %q, want /v1/page", req.path)
	}
	if req.body["name"] != "/home" {
		t.Errorf("name = %v, want /home", req.body["name"])
	}
}

// TestAlias verifies the alias endpoint and body.
func TestAlias(t *testing.T) {
	ts := newTestServer(t, http.StatusOK)
	p := newProvider(t, ts.srv)

	msg := provider.AliasMessage{
		MessageID:  "alias-001",
		Timestamp:  time.Now(),
		UserID:     "user-new",
		PreviousID: "anon-old",
	}
	if err := p.Alias(context.Background(), msg); err != nil {
		t.Fatalf("Alias: %v", err)
	}

	reqs := ts.captured()
	req := reqs[0]
	if req.path != "/v1/alias" {
		t.Errorf("path = %q, want /v1/alias", req.path)
	}
	if req.body["user_id"] != "user-new" {
		t.Errorf("user_id = %v, want user-new", req.body["user_id"])
	}
	if req.body["previous_id"] != "anon-old" {
		t.Errorf("previous_id = %v, want anon-old", req.body["previous_id"])
	}
}

// TestFlush verifies the flush endpoint sends the source in the body.
func TestFlush(t *testing.T) {
	ts := newTestServer(t, http.StatusOK)
	p := newProvider(t, ts.srv)

	if err := p.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	reqs := ts.captured()
	if len(reqs) != 1 {
		t.Fatalf("captured %d requests, want 1", len(reqs))
	}
	req := reqs[0]
	if req.path != "/v1/flush" {
		t.Errorf("path = %q, want /v1/flush", req.path)
	}
	if req.body["source"] != "test-source" {
		t.Errorf("source = %v, want test-source", req.body["source"])
	}
}

// TestShutdown verifies that Shutdown flushes (posts to /v1/flush) and that
// subsequent calls to any provider method return ErrShutdown.
func TestShutdown(t *testing.T) {
	ts := newTestServer(t, http.StatusOK)
	p := newProvider(t, ts.srv)

	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// Shutdown should have sent a flush request.
	reqs := ts.captured()
	if len(reqs) != 1 || reqs[0].path != "/v1/flush" {
		t.Errorf("expected flush request on shutdown, got: %v", reqs)
	}

	// All subsequent calls must return ErrShutdown.
	if err := p.Track(context.Background(), provider.TrackMessage{}); !errors.Is(err, eventspec.ErrShutdown) {
		t.Errorf("Track after Shutdown = %v, want ErrShutdown", err)
	}
	if err := p.Flush(context.Background()); !errors.Is(err, eventspec.ErrShutdown) {
		t.Errorf("Flush after Shutdown = %v, want ErrShutdown", err)
	}
	if err := p.Shutdown(context.Background()); !errors.Is(err, eventspec.ErrShutdown) {
		t.Errorf("Shutdown after Shutdown = %v, want ErrShutdown", err)
	}
}

// TestAuthHeader verifies that every request includes the Bearer token.
func TestAuthHeader(t *testing.T) {
	ts := newTestServer(t, http.StatusOK)
	p := newProvider(t, ts.srv)

	_ = p.Track(context.Background(), provider.TrackMessage{EventName: "ev"})
	_ = p.Identify(context.Background(), provider.IdentifyMessage{})
	_ = p.Group(context.Background(), provider.GroupMessage{})
	_ = p.Page(context.Background(), provider.PageMessage{})
	_ = p.Alias(context.Background(), provider.AliasMessage{})
	_ = p.Flush(context.Background())

	for _, req := range ts.captured() {
		if req.auth != "Bearer test-api-key" {
			t.Errorf("path %s: Authorization = %q, want Bearer test-api-key", req.path, req.auth)
		}
	}
}

// TestMessageContextAttributes verifies that user_agent and ip_address from
// MessageContext are forwarded in context.attributes.
func TestMessageContextAttributes(t *testing.T) {
	ts := newTestServer(t, http.StatusOK)
	p := newProvider(t, ts.srv)

	msg := provider.TrackMessage{
		EventName: "ctx_test",
		MessageContext: provider.MessageContext{
			UserAgent: "TestBot/1.0",
			IPAddress: "1.2.3.4",
			Extra:     map[string]any{"session_id": "sess-abc"},
		},
	}
	if err := p.Track(context.Background(), msg); err != nil {
		t.Fatalf("Track: %v", err)
	}

	req := ts.captured()[0]
	ctx, ok := req.body["context"].(map[string]any)
	if !ok {
		t.Fatalf("context missing: %v", req.body["context"])
	}
	attrs, ok := ctx["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("context.attributes missing: %v", ctx["attributes"])
	}
	if attrs["user_agent"] != "TestBot/1.0" {
		t.Errorf("user_agent = %v, want TestBot/1.0", attrs["user_agent"])
	}
	if attrs["ip_address"] != "1.2.3.4" {
		t.Errorf("ip_address = %v, want 1.2.3.4", attrs["ip_address"])
	}
	if attrs["session_id"] != "sess-abc" {
		t.Errorf("session_id = %v, want sess-abc", attrs["session_id"])
	}
}

// TestProxyRouting verifies that requests are routed to the proxy host when
// ProxyReverseProxy mode is configured.
func TestProxyRouting(t *testing.T) {
	var mu sync.Mutex
	var paths []string

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer proxy.Close()

	// Real target URL that should be rewritten.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request reached target directly; expected it to be proxied")
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	p, err := eventspec.New(eventspec.Config{
		ProviderConfig: provider.ProviderConfig{
			ProxyURL:  proxy.URL,
			ProxyMode: provider.ProxyReverseProxy,
		},
		APIKey:  "proxy-key",
		BaseURL: target.URL,
		Source:  "proxy-test",
	})
	if err != nil {
		t.Fatalf("eventspec.New: %v", err)
	}

	if err := p.Track(context.Background(), provider.TrackMessage{EventName: "test"}); err != nil {
		t.Fatalf("Track: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(paths) == 0 {
		t.Fatal("no requests reached proxy")
	}
	if paths[0] != "/v1/track" {
		t.Errorf("proxy path = %q, want /v1/track", paths[0])
	}
}

// TestRetryOnTransientError verifies that the transport retries on 500 responses.
func TestRetryOnTransientError(t *testing.T) {
	var mu sync.Mutex
	attempts := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p, err := eventspec.New(eventspec.Config{
		ProviderConfig: provider.ProviderConfig{
			RetryConfig: provider.RetryConfig{
				MaxRetries:     2,
				InitialBackoff: time.Millisecond,
				Jitter:         false,
			},
		},
		APIKey:  "retry-key",
		BaseURL: srv.URL,
		Source:  "retry-test",
	})
	if err != nil {
		t.Fatalf("eventspec.New: %v", err)
	}

	if err := p.Track(context.Background(), provider.TrackMessage{EventName: "retry_test"}); err != nil {
		t.Fatalf("Track: %v", err)
	}

	mu.Lock()
	got := attempts
	mu.Unlock()
	if got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
}

// TestErrorResponse verifies a non-2xx response is surfaced as an error.
func TestErrorResponse(t *testing.T) {
	ts := newTestServer(t, http.StatusBadRequest)
	p := newProvider(t, ts.srv)

	err := p.Track(context.Background(), provider.TrackMessage{EventName: "bad"})
	if err == nil {
		t.Fatal("Track() error = nil, want non-nil for 400 response")
	}
}

// TestNew_missingEnvVar verifies that New returns an error when the env var is not set.
func TestNew_missingEnvVar(t *testing.T) {
	_, err := eventspec.New(eventspec.Config{
		ProviderConfig: provider.ProviderConfig{
			SecretType: provider.SecretEnvVar,
		},
		APIKey: "${EVENTSPEC_KEY_DEFINITELY_NOT_SET_XYZ}",
	})
	if err == nil {
		t.Error("New() error = nil, want non-nil for missing env var")
	}
}
