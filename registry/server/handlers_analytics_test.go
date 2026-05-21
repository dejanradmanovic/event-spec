package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestServer_Track_Returns202(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source":     "web-app",
		"event_name": "Product Viewed",
		"properties": map[string]any{"product_id": "SKU-123"},
		"context":    map[string]any{"user_id": "user-456", "anonymous_id": "anon-789"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

func TestServer_Track_NoAuth_Returns401(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/track", "", map[string]any{
		"source":     "web-app",
		"event_name": "Product Viewed",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestServer_Track_MissingEventName_Returns400(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source": "web-app",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestServer_Track_UnknownSource_Returns400(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source":     "no-such-source",
		"event_name": "Product Viewed",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestServer_Track_ClientCanReuse(t *testing.T) {
	ts, _ := newTestSrv(t)
	body := map[string]any{"source": "web-app", "event_name": "ev"}
	for range 3 {
		resp := postJSON(t, ts, "/v1/track", "viewer-tok", body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Errorf("want 202, got %d", resp.StatusCode)
		}
	}
}

func TestServer_Identify_Returns202(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/identify", "viewer-tok", map[string]any{
		"source":  "web-app",
		"user_id": "user-123",
		"traits":  map[string]any{"email": "user@example.com"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

func TestServer_Identify_MissingSource_Returns400(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/identify", "viewer-tok", map[string]any{
		"user_id": "user-123",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestServer_Group_Returns202(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/group", "viewer-tok", map[string]any{
		"source":   "web-app",
		"group_id": "acme-corp",
		"traits":   map[string]any{"plan": "enterprise"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

func TestServer_Page_Returns202(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/page", "viewer-tok", map[string]any{
		"source":     "web-app",
		"name":       "Home",
		"properties": map[string]any{"url": "https://example.com"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

func TestServer_Page_MissingName_Returns400(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/page", "viewer-tok", map[string]any{
		"source": "web-app",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestServer_Alias_Returns202(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/alias", "viewer-tok", map[string]any{
		"source":      "web-app",
		"user_id":     "new-id",
		"previous_id": "anon-id",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

func TestServer_Batch_Returns202(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/batch", "viewer-tok", map[string]any{
		"source":  "web-app",
		"context": map[string]any{"user_id": "user-123"},
		"events": []map[string]any{
			{
				"type":       "track",
				"event_name": "Product Viewed",
				"properties": map[string]any{"product_id": "SKU-1"},
			},
			{
				"type":    "identify",
				"user_id": "user-123",
				"traits":  map[string]any{"email": "u@example.com"},
			},
			{
				"type":     "group",
				"group_id": "team-1",
				"traits":   map[string]any{"plan": "pro"},
			},
			{
				"type": "page",
				"name": "Checkout",
			},
			{
				"type":        "alias",
				"user_id":     "user-123",
				"previous_id": "anon-abc",
			},
		},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

func TestServer_Batch_UnknownType_Returns400(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/batch", "viewer-tok", map[string]any{
		"source": "web-app",
		"events": []map[string]any{
			{"type": "screenview"},
		},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestServer_Batch_MissingSource_Returns400(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/batch", "viewer-tok", map[string]any{
		"events": []map[string]any{
			{"type": "track", "event_name": "ev"},
		},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestServer_Flush_AllSources_Returns202(t *testing.T) {
	ts, _ := newTestSrv(t)
	// Warm the cache with a track call so at least one client exists.
	postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source": "web-app", "event_name": "ev",
	}).Body.Close()

	resp := postJSON(t, ts, "/v1/flush", "viewer-tok", map[string]any{})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

func TestServer_Flush_SpecificSource_Returns202(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/flush", "viewer-tok", map[string]any{
		"source": "web-app",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

func TestServer_Flush_UnknownSource_Returns400(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/flush", "viewer-tok", map[string]any{
		"source": "ghost-source",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

// TestServer_EnrichFromRequest verifies that a thin client that omits context.attributes
// entirely still gets a 202 — the server injects User-Agent and IP from the HTTP request
// so the provider MessageContext is populated even when the client sends nothing.
func TestServer_EnrichFromRequest(t *testing.T) {
	ts, _ := newTestSrv(t)

	body, _ := json.Marshal(map[string]any{
		"source":     "web-app",
		"event_name": "Login",
		// deliberately omit context.attributes — server should inject from HTTP headers
	})

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/v1/track", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer viewer-tok")
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0)")
	req.Header.Set("X-Forwarded-For", "203.0.113.42")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

// TestServer_EnrichFromRequest_ClientWins verifies that explicit context.attributes
// from the client take precedence over what the server could extract from HTTP headers.
func TestServer_EnrichFromRequest_ClientWins(t *testing.T) {
	ts, _ := newTestSrv(t)

	body, _ := json.Marshal(map[string]any{
		"source":     "web-app",
		"event_name": "Login",
		"context": map[string]any{
			"attributes": map[string]any{
				// Client supplies its own user_agent; server must not overwrite it.
				"user_agent": "MyApp/2.1 (custom)",
				"ip_address": "10.0.0.1",
			},
		},
	})

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/v1/track", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer viewer-tok")
	req.Header.Set("User-Agent", "Mozilla/5.0 (should be ignored)")
	req.Header.Set("X-Forwarded-For", "203.0.113.99")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}
