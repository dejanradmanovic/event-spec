package analytics_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"event-spec/analytics"
	"event-spec/testutil"
)

func TestAnalyticsMiddleware_injectsTransactionContext(t *testing.T) {
	resetGlobal(t)

	cap := testutil.NewCaptureProvider("cap")
	client := analytics.NewClient(analytics.WithProviders(cap))

	// The middleware extracts userID from a request header for this test.
	extract := func(r *http.Request) analytics.TransactionContext {
		return analytics.TransactionContext{
			UserID:      r.Header.Get("X-User-ID"),
			AnonymousID: r.Header.Get("X-Session-ID"),
			Attributes:  map[string]any{"request_id": r.Header.Get("X-Request-ID")},
		}
	}

	var capturedCtx context.Context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		_ = client.Track(r.Context(), analytics.Event{Name: "Page Viewed"})
		w.WriteHeader(http.StatusOK)
	})

	wrapped := analytics.AnalyticsMiddleware(extract)(handler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-42")
	req.Header.Set("X-Session-ID", "sess-99")
	req.Header.Set("X-Request-ID", "req-abc")

	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	// Verify context was injected.
	tx, ok := analytics.TransactionContextFrom(capturedCtx)
	if !ok {
		t.Fatal("TransactionContextFrom: expected ok=true in handler context")
	}
	if tx.UserID != "user-42" {
		t.Errorf("UserID: got %q, want %q", tx.UserID, "user-42")
	}
	if tx.AnonymousID != "sess-99" {
		t.Errorf("AnonymousID: got %q, want %q", tx.AnonymousID, "sess-99")
	}
	if tx.Attributes["request_id"] != "req-abc" {
		t.Errorf("Attributes[request_id]: got %v", tx.Attributes["request_id"])
	}

	// Verify the Track call received the context values.
	if len(cap.Tracks) != 1 {
		t.Fatalf("expected 1 track call, got %d", len(cap.Tracks))
	}
	if cap.Tracks[0].UserID != "user-42" {
		t.Errorf("Track UserID: got %q, want %q", cap.Tracks[0].UserID, "user-42")
	}
	if cap.Tracks[0].AnonymousID != "sess-99" {
		t.Errorf("Track AnonymousID: got %q, want %q", cap.Tracks[0].AnonymousID, "sess-99")
	}
}

func TestAnalyticsMiddleware_noContextWithoutMiddleware(t *testing.T) {
	// Without middleware, TransactionContextFrom should return false.
	_, ok := analytics.TransactionContextFrom(context.Background())
	if ok {
		t.Error("expected ok=false without middleware")
	}
}
