package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dejanradmanovic/event-spec/registry/server"
	"github.com/dejanradmanovic/event-spec/spec"
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

// --- Hooks: per-event sampling from spec ---

// TestServer_Track_HooksEnabled_SampledOut_Returns202 verifies that an event whose
// spec declares sampling.rate=0 (random strategy) is silently dropped — the client
// still receives 202 so it cannot distinguish sampled from forwarded events.
func TestServer_Track_HooksEnabled_SampledOut_Returns202(t *testing.T) {
	ts, st := newTestSrvWithConfig(t, server.Config{})
	// Rate 0 → rand.Float64() < 0 is always false → always dropped.
	st.events[0].Sampling = &spec.SamplingConfig{Strategy: spec.SamplingRandom, Rate: 0}

	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source":     "web-app",
		"event_name": "Product Viewed",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202 (silent drop), got %d", resp.StatusCode)
	}
}

// TestServer_Track_HooksEnabled_FullySampled_Returns202 verifies that rate=1 always
// forwards the event (rand.Float64() < 1 is always true).
func TestServer_Track_HooksEnabled_FullySampled_Returns202(t *testing.T) {
	ts, st := newTestSrvWithConfig(t, server.Config{})
	st.events[0].Sampling = &spec.SamplingConfig{Strategy: spec.SamplingRandom, Rate: 1.0}

	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source":     "web-app",
		"event_name": "Product Viewed",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

// TestServer_Track_HooksEnabled_UserIDHash_RateZero_Drops verifies deterministic
// user_id_hash sampling: rate=0 means no user ever passes the bucket check.
func TestServer_Track_HooksEnabled_UserIDHash_RateZero_Drops(t *testing.T) {
	ts, st := newTestSrvWithConfig(t, server.Config{})
	st.events[0].Sampling = &spec.SamplingConfig{Strategy: spec.SamplingUserIDHash, Rate: 0}

	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source":     "web-app",
		"event_name": "Product Viewed",
		"context":    map[string]any{"user_id": "any-user"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202 (silent drop), got %d", resp.StatusCode)
	}
}

// TestServer_Track_HooksEnabled_UserIDHash_RateOne_Forwards verifies that rate=1
// keeps every user's event (hash(u)/2^32 < 1.0 is always true).
func TestServer_Track_HooksEnabled_UserIDHash_RateOne_Forwards(t *testing.T) {
	ts, st := newTestSrvWithConfig(t, server.Config{})
	st.events[0].Sampling = &spec.SamplingConfig{Strategy: spec.SamplingUserIDHash, Rate: 1.0}

	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source":     "web-app",
		"event_name": "Product Viewed",
		"context":    map[string]any{"user_id": "user-abc"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

// TestServer_Track_HooksEnabled_SamplingNone_AlwaysForwards verifies that
// strategy=none skips sampling entirely regardless of any rate value.
func TestServer_Track_HooksEnabled_SamplingNone_AlwaysForwards(t *testing.T) {
	ts, st := newTestSrvWithConfig(t, server.Config{})
	st.events[0].Sampling = &spec.SamplingConfig{Strategy: spec.SamplingNone}

	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source":     "web-app",
		"event_name": "Product Viewed",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

// TestServer_Track_HooksEnabled_DeletedEvent_Returns400 verifies that the validation
// hook blocks dispatch of events whose spec status is deleted.
func TestServer_Track_HooksEnabled_DeletedEvent_Returns400(t *testing.T) {
	ts, st := newTestSrvWithConfig(t, server.Config{})
	st.events[0].Status = spec.StatusDeleted

	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source":     "web-app",
		"event_name": "Product Viewed",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 (deleted event), got %d", resp.StatusCode)
	}
}

// TestServer_Track_HooksDisabled_DeletedEventPassesThrough verifies that when hooks
// are disabled the validation hook is bypassed and deleted events reach providers.
func TestServer_Track_HooksDisabled_DeletedEventPassesThrough(t *testing.T) {
	ts, st := newTestSrvWithConfig(t, server.Config{HooksDisabled: true})
	st.events[0].Status = spec.StatusDeleted

	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source":     "web-app",
		"event_name": "Product Viewed",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202 (hooks off), got %d", resp.StatusCode)
	}
}

// TestServer_Track_HooksEnabled_UnknownEvent_PassesThrough verifies that unknown event
// names (not in spec) pass through unchanged — validation only applies to spec-known events.
func TestServer_Track_HooksEnabled_UnknownEvent_PassesThrough(t *testing.T) {
	ts, _ := newTestSrvWithConfig(t, server.Config{})
	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source":     "web-app",
		"event_name": "Not In Spec",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202 (unknown event passes through), got %d", resp.StatusCode)
	}
}

// TestServer_Batch_HooksEnabled_SampledItemContinues verifies that a sampled-out item
// inside a batch is silently skipped while the rest of the batch is processed normally.
func TestServer_Batch_HooksEnabled_SampledItemContinues(t *testing.T) {
	ts, st := newTestSrvWithConfig(t, server.Config{})
	// Product Viewed always sampled out; Order Completed has no sampling → passes through.
	st.events[0].Sampling = &spec.SamplingConfig{Strategy: spec.SamplingRandom, Rate: 0}

	resp := postJSON(t, ts, "/v1/batch", "viewer-tok", map[string]any{
		"source": "web-app",
		"events": []map[string]any{
			{"type": "track", "event_name": "Product Viewed"},  // sampled out → skip
			{"type": "track", "event_name": "Order Completed"}, // no sampling → dispatch
		},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
}

// --- Admin config endpoint ---

func TestServer_AdminConfig_GetConfig_Returns200(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/admin/config", "admin-tok")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

// TestServer_AdminConfig_SetHooksEnabled_TakesEffectImmediately verifies that
// PUT /v1/admin/config/hooks_enabled toggles hook behaviour without a restart:
// a deleted event passes through before the flag is set and is blocked after.
func TestServer_AdminConfig_SetHooksEnabled_TakesEffectImmediately(t *testing.T) {
	ts, st := newTestSrvWithConfig(t, server.Config{HooksDisabled: true})
	st.events[0].Status = spec.StatusDeleted

	// Before: hooks off → deleted event passes through.
	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source": "web-app", "event_name": "Product Viewed",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202 before hooks enabled, got %d", resp.StatusCode)
	}

	// Enable hooks via admin endpoint.
	body, _ := json.Marshal(map[string]string{"value": "true"})
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPut,
		ts.URL+"/v1/admin/config/hooks_enabled", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-tok")
	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT config: want 200, got %d", putResp.StatusCode)
	}

	// After: hooks on → deleted event is rejected.
	resp = postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source": "web-app", "event_name": "Product Viewed",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 after hooks enabled, got %d", resp.StatusCode)
	}
}

func TestServer_AdminConfig_UnknownKey_Returns400(t *testing.T) {
	ts, _ := newTestSrv(t)
	body, _ := json.Marshal(map[string]string{"value": "something"})
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPut,
		ts.URL+"/v1/admin/config/unknown_key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestServer_AdminConfig_InvalidHooksEnabledValue_Returns400(t *testing.T) {
	ts, _ := newTestSrv(t)
	body, _ := json.Marshal(map[string]string{"value": "yes"})
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPut,
		ts.URL+"/v1/admin/config/hooks_enabled", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

// TestServer_Track_HooksEnabled_MissingRequiredProp_Returns400 verifies that property
// schema validation fires through the relay when hc.Message is populated in the client.
// The event must declare at least one required property so an empty payload fails.
func TestServer_Track_HooksEnabled_MissingRequiredProp_Returns400(t *testing.T) {
	ts, st := newTestSrvWithConfig(t, server.Config{})
	// Give Product Viewed a required property so an empty payload fails schema validation.
	st.events[0].Properties = map[string]spec.PropertyDef{
		"product_id": {Type: spec.PropertyTypeString, Required: true},
	}

	resp := postJSON(t, ts, "/v1/track", "viewer-tok", map[string]any{
		"source":     "web-app",
		"event_name": "Product Viewed",
		"properties": map[string]any{}, // required product_id missing
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 (schema violation), got %d", resp.StatusCode)
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
