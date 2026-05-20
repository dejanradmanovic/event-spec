package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/registry/server"
	"github.com/dejanradmanovic/event-spec/registry/server/client"
	"github.com/dejanradmanovic/event-spec/spec"
)

// --- mock store ---

type mockStore struct {
	events   []spec.EventDef
	sources  map[string]*spec.SourceDef
	apiKeys  map[string]keyEntry // sha256hex(rawToken) → entry
	audit    []server.AuditEntry
	webhooks []string

	publishCalled bool
	published     spec.EventDef
}

type keyEntry struct{ userID, role string }

func (m *mockStore) ListEvents(_ context.Context, filter registry.ListFilter) ([]spec.EventDef, error) {
	var out []spec.EventDef
	for _, ev := range m.events {
		if filter.Namespace != "" && ev.Namespace != filter.Namespace {
			continue
		}
		if filter.Status != "" && ev.Status != filter.Status {
			continue
		}
		out = append(out, ev)
	}
	return out, nil
}

func (m *mockStore) GetEvent(_ context.Context, namespace, name, version string) (*spec.EventDef, error) {
	for i, ev := range m.events {
		if ev.Namespace == namespace && ev.Name == name && (version == "" || ev.Version == version) {
			return &m.events[i], nil
		}
	}
	return nil, fmt.Errorf("event %s/%s: %w", namespace, name, registry.ErrNotFound)
}

func (m *mockStore) GetSource(_ context.Context, name string) (*spec.SourceDef, error) {
	if src, ok := m.sources[name]; ok {
		return src, nil
	}
	return nil, fmt.Errorf("source %q: %w", name, registry.ErrNotFound)
}

func (m *mockStore) GetDestination(_ context.Context, _ string) (*spec.DestinationDef, error) {
	return nil, fmt.Errorf("destination: %w", registry.ErrNotFound)
}

func (m *mockStore) PublishEvent(_ context.Context, event spec.EventDef, _ string) error {
	m.publishCalled = true
	m.published = event
	m.events = append(m.events, event)
	return nil
}

func (m *mockStore) LookupAPIKey(_ context.Context, keyHash string) (userID, role string, err error) {
	if e, ok := m.apiKeys[keyHash]; ok {
		return e.userID, e.role, nil
	}
	return "", "", registry.ErrNotFound
}

func (m *mockStore) ListAuditLog(_ context.Context) ([]server.AuditEntry, error) {
	return m.audit, nil
}

func (m *mockStore) RegisterWebhook(_ context.Context, webhookURL, _ string) error {
	m.webhooks = append(m.webhooks, webhookURL)
	return nil
}

// --- helpers ---

func keyHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func newTestSrv(t *testing.T) (*httptest.Server, *mockStore) {
	t.Helper()
	st := &mockStore{
		apiKeys: map[string]keyEntry{
			keyHash("viewer-tok"):  {"alice", server.RoleViewer},
			keyHash("publish-tok"): {"bob", server.RolePublisher},
			keyHash("admin-tok"):   {"carol", server.RoleAdmin},
		},
		events: []spec.EventDef{
			{
				Namespace: "ecommerce",
				Name:      "product_viewed",
				Version:   "1-0-0",
				Status:    spec.StatusActive,
				EventName: "Product Viewed",
				Type:      spec.TypeTrack,
			},
			{
				Namespace: "ecommerce",
				Name:      "order_completed",
				Version:   "2-0-0",
				Status:    spec.StatusActive,
				EventName: "Order Completed",
				Type:      spec.TypeTrack,
			},
		},
		sources: map[string]*spec.SourceDef{
			"web-app": {
				Name:     "web-app",
				Language: "typescript",
				Events:   []string{"ecommerce/**"},
			},
		},
		audit: []server.AuditEntry{
			{
				ID: 1, Action: "create", EntityType: "event",
				EntityID: 1, UserID: "alice", Timestamp: time.Now(),
			},
		},
	}
	ts := httptest.NewServer(server.New(st, server.Config{}))
	t.Cleanup(ts.Close)
	return ts, st
}

func get(t *testing.T, ts *httptest.Server, path, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func postJSON(t *testing.T, ts *httptest.Server, path, token string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	return v
}

// --- auth tests ---

func TestServer_NoAuth_Returns401(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/events", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestServer_InvalidKey_Returns401(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/events", "bad-key")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestServer_InsufficientRole_Returns403(t *testing.T) {
	ts, _ := newTestSrv(t)
	// viewer token cannot publish
	resp := postJSON(t, ts, "/v1/events", "viewer-tok", spec.EventDef{
		Namespace: "ns", Name: "ev", Version: "1-0-0",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("want 403, got %d", resp.StatusCode)
	}
}

// --- ListEvents ---

func TestServer_ListEvents_Returns200(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/events", "viewer-tok")
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var events []spec.EventDef
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(events) != 2 {
		t.Errorf("want 2 events, got %d", len(events))
	}
}

func TestServer_ListEvents_FilterByNamespace(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/events?namespace=ecommerce", "viewer-tok")
	var events []spec.EventDef
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	for _, ev := range events {
		if ev.Namespace != "ecommerce" {
			t.Errorf("unexpected namespace %q in filtered results", ev.Namespace)
		}
	}
}

// --- GetEvent ---

func TestServer_GetEvent_Found(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/events/ecommerce/product_viewed", "viewer-tok")
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var def spec.EventDef
	json.NewDecoder(resp.Body).Decode(&def)
	resp.Body.Close()
	if def.Name != "product_viewed" {
		t.Errorf("want product_viewed, got %q", def.Name)
	}
}

func TestServer_GetEventVersion_Found(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/events/ecommerce/product_viewed/1-0-0", "viewer-tok")
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var def spec.EventDef
	json.NewDecoder(resp.Body).Decode(&def)
	resp.Body.Close()
	if def.Version != "1-0-0" {
		t.Errorf("want version 1-0-0, got %q", def.Version)
	}
}

func TestServer_GetEvent_NotFound_Returns404(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/events/ecommerce/missing", "viewer-tok")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

// --- PublishEvent ---

func TestServer_PublishEvent_Success(t *testing.T) {
	ts, st := newTestSrv(t)
	ev := spec.EventDef{
		Namespace: "payments",
		Name:      "charge_created",
		Version:   "1-0-0",
		Status:    spec.StatusActive,
		EventName: "Charge Created",
		Type:      spec.TypeTrack,
	}
	resp := postJSON(t, ts, "/v1/events", "publish-tok", ev)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	if !st.publishCalled {
		t.Error("PublishEvent was not called on store")
	}
	if st.published.Name != "charge_created" {
		t.Errorf("published name = %q, want charge_created", st.published.Name)
	}
}

func TestServer_PublishEvent_MissingFields_Returns400(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/events", "publish-tok", spec.EventDef{Name: "oops"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

// --- Diff ---

func TestServer_Diff_Returns200(t *testing.T) {
	ts, st := newTestSrv(t)
	// Add a second version of product_viewed with an extra property.
	st.events = append(st.events, spec.EventDef{
		Namespace:  "ecommerce",
		Name:       "product_viewed",
		Version:    "1-1-0",
		Status:     spec.StatusActive,
		EventName:  "Product Viewed",
		Type:       spec.TypeTrack,
		Properties: map[string]spec.PropertyDef{"coupon": {Type: spec.PropertyTypeString}},
	})

	resp := get(t, ts, "/v1/diff/ecommerce/product_viewed/1-0-0/1-1-0", "viewer-tok")
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var changes []spec.Change
	json.NewDecoder(resp.Body).Decode(&changes)
	resp.Body.Close()
	// product_viewed 1-0-0 has no properties; 1-1-0 adds optional coupon → ChangeAddOptionalProp
	if len(changes) == 0 {
		t.Error("expected at least one change")
	}
}

func TestServer_Diff_MissingVersion_Returns404(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/diff/ecommerce/product_viewed/1-0-0/9-9-9", "viewer-tok")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

// --- SourcePull ---

func TestServer_SourcePull_Returns200Zip(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/sources/web-app/pull", "viewer-tok")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/zip" {
		t.Errorf("want Content-Type application/zip, got %q", ct)
	}
	// Ensure body is non-empty.
	data, _ := io.ReadAll(resp.Body)
	if len(data) == 0 {
		t.Error("zip body is empty")
	}
}

func TestServer_SourcePull_UnknownSource_Returns404(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/sources/missing/pull", "viewer-tok")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

// --- AuditLog ---

func TestServer_AuditLog_AdminReturns200(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/audit", "admin-tok")
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var entries []server.AuditEntry
	json.NewDecoder(resp.Body).Decode(&entries)
	resp.Body.Close()
	if len(entries) != 1 {
		t.Errorf("want 1 audit entry, got %d", len(entries))
	}
}

func TestServer_AuditLog_PublisherForbidden(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := get(t, ts, "/v1/audit", "publish-tok")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("want 403, got %d", resp.StatusCode)
	}
}

// --- RegisterWebhook ---

func TestServer_RegisterWebhook_AdminReturns201(t *testing.T) {
	ts, st := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/webhooks", "admin-tok", map[string]string{
		"url": "https://example.com/hook",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	if len(st.webhooks) != 1 || st.webhooks[0] != "https://example.com/hook" {
		t.Errorf("webhook not recorded: %v", st.webhooks)
	}
}

func TestServer_RegisterWebhook_MissingURL_Returns400(t *testing.T) {
	ts, _ := newTestSrv(t)
	resp := postJSON(t, ts, "/v1/webhooks", "admin-tok", map[string]string{})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

// --- Registry interface via Server ---

func TestServer_RegistryInterface_ListEvents(t *testing.T) {
	st := &mockStore{
		events:  []spec.EventDef{{Namespace: "ns", Name: "ev", Version: "1-0-0", Status: spec.StatusActive}},
		apiKeys: map[string]keyEntry{},
		sources: map[string]*spec.SourceDef{},
	}
	srv := server.New(st, server.Config{})

	var r registry.Registry = srv // compile-time interface check
	events, err := r.ListEvents(context.Background(), registry.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Errorf("want 1, got %d", len(events))
	}
}

// --- HTTP client tests ---

func TestClient_ListEvents(t *testing.T) {
	ts, _ := newTestSrv(t)
	c := client.New(client.Config{BaseURL: ts.URL, APIKey: "viewer-tok"})
	events, err := c.ListEvents(context.Background(), registry.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Errorf("want 2 events, got %d", len(events))
	}
}

func TestClient_GetEvent_LatestActive(t *testing.T) {
	ts, _ := newTestSrv(t)
	c := client.New(client.Config{BaseURL: ts.URL, APIKey: "viewer-tok"})
	def, err := c.GetEvent(context.Background(), "ecommerce", "product_viewed", "")
	if err != nil {
		t.Fatal(err)
	}
	if def.Name != "product_viewed" {
		t.Errorf("want product_viewed, got %q", def.Name)
	}
}

func TestClient_GetEvent_SpecificVersion(t *testing.T) {
	ts, _ := newTestSrv(t)
	c := client.New(client.Config{BaseURL: ts.URL, APIKey: "viewer-tok"})
	def, err := c.GetEvent(context.Background(), "ecommerce", "product_viewed", "1-0-0")
	if err != nil {
		t.Fatal(err)
	}
	if def.Version != "1-0-0" {
		t.Errorf("want 1-0-0, got %q", def.Version)
	}
}

func TestClient_GetEvent_NotFound(t *testing.T) {
	ts, _ := newTestSrv(t)
	c := client.New(client.Config{BaseURL: ts.URL, APIKey: "viewer-tok"})
	_, err := c.GetEvent(context.Background(), "ns", "missing", "")
	if !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestClient_PublishEvent(t *testing.T) {
	ts, st := newTestSrv(t)
	c := client.New(client.Config{BaseURL: ts.URL, APIKey: "publish-tok"})
	ev := spec.EventDef{
		Namespace: "payments",
		Name:      "refund_issued",
		Version:   "1-0-0",
		Status:    spec.StatusActive,
		EventName: "Refund Issued",
		Type:      spec.TypeTrack,
	}
	if err := c.PublishEvent(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if !st.publishCalled {
		t.Error("PublishEvent was not called")
	}
}

func TestClient_Diff(t *testing.T) {
	ts, st := newTestSrv(t)
	st.events = append(st.events, spec.EventDef{
		Namespace:  "ecommerce",
		Name:       "product_viewed",
		Version:    "1-1-0",
		Status:     spec.StatusActive,
		EventName:  "Product Viewed",
		Properties: map[string]spec.PropertyDef{"sku": {Type: spec.PropertyTypeString}},
	})
	c := client.New(client.Config{BaseURL: ts.URL, APIKey: "viewer-tok"})
	changes, err := c.Diff(context.Background(), "ecommerce", "product_viewed", "1-0-0", "1-1-0")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) == 0 {
		t.Error("expected at least one change")
	}
}

func TestClient_GetSource_ReturnsErrNotFound(t *testing.T) {
	c := client.New(client.Config{BaseURL: "http://localhost:9999", APIKey: "tok"})
	_, err := c.GetSource(context.Background(), "web-app")
	if !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestClient_InvalidKey_Returns401Error(t *testing.T) {
	ts, _ := newTestSrv(t)
	c := client.New(client.Config{BaseURL: ts.URL, APIKey: "wrong"})
	_, err := c.ListEvents(context.Background(), registry.ListFilter{})
	if err == nil {
		t.Error("want error for invalid API key, got nil")
	}
	if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "invalid") {
		// expected
	} else {
		t.Errorf("unexpected error: %v", err)
	}
}
