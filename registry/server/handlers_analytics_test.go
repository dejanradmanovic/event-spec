package server_test

import (
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
