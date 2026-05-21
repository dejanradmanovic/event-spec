// Package client provides an HTTP client for the event-spec registry server.
// It implements the registry.Registry interface so the CLI can remain agnostic to
// the registry backend in server mode.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

// StatusResponse is the body returned by GET /v1/health.
type StatusResponse struct {
	Status string `json:"status"`
	Uptime string `json:"uptime,omitempty"`
}

// APIKeyRecord is the public metadata for a stored API key.
type APIKeyRecord struct {
	ID        int64      `json:"id"`
	Role      string     `json:"role"`
	Name      string     `json:"name,omitempty"`
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// CreatedAPIKey is the response body for POST /v1/admin/keys.
// The raw key is returned once and is not recoverable thereafter.
type CreatedAPIKey struct {
	ID   int64  `json:"id"`
	Key  string `json:"key"`
	Role string `json:"role"`
}

// WebhookRecord is a registered webhook entry with its database ID.
type WebhookRecord struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditEntry is a single record from the server's audit log.
type AuditEntry struct {
	ID         int64     `json:"id"`
	Action     string    `json:"action"`
	EntityType string    `json:"entity_type"`
	EntityID   int64     `json:"entity_id"`
	UserID     string    `json:"user_id"`
	Timestamp  time.Time `json:"timestamp"`
	Details    string    `json:"details,omitempty"`
}

// Config configures the HTTP client.
type Config struct {
	// BaseURL is the registry server base URL, e.g. "https://registry.example.com".
	BaseURL string
	// APIKey is the Bearer token used for authentication.
	APIKey string
	// CacheDir is an optional directory for offline caching. When set, successful
	// GET responses are persisted and served as a fallback when the server is unreachable.
	CacheDir string
}

// Client is an HTTP client for the event-spec registry server.
// It implements the registry.Registry interface.
type Client struct {
	cfg  Config
	http *http.Client
}

// New creates a Client with the given configuration.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{}}
}

// ListEvents fetches all events matching filter from the server.
// When the server is unreachable and CacheDir is configured, the last cached response is returned.
func (c *Client) ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error) {
	q := url.Values{}
	if filter.Namespace != "" {
		q.Set("namespace", filter.Namespace)
	}
	if filter.Status != "" {
		q.Set("status", string(filter.Status))
	}
	for _, tag := range filter.Tags {
		q.Add("tag", tag)
	}
	u := c.cfg.BaseURL + "/v1/events"
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var events []spec.EventDef
	if err := c.getWithCache(ctx, u, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// GetEvent fetches an event by namespace and name.
// When version is empty the server returns the latest active version.
// When the server is unreachable and CacheDir is configured, the last cached response is returned.
func (c *Client) GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error) {
	var u string
	if version != "" {
		u = fmt.Sprintf("%s/v1/events/%s/%s/%s", c.cfg.BaseURL, namespace, name, version)
	} else {
		u = fmt.Sprintf("%s/v1/events/%s/%s", c.cfg.BaseURL, namespace, name)
	}
	var def spec.EventDef
	if err := c.getWithCache(ctx, u, &def); err != nil {
		return nil, err
	}
	return &def, nil
}

// GetSource returns ErrNotFound in server mode — sources are always local to the
// consuming repo and not fetched from the server. Use PullSource to download a
// spec snapshot from GET /v1/sources/{name}/pull.
func (c *Client) GetSource(_ context.Context, name string) (*spec.SourceDef, error) {
	return nil, fmt.Errorf("GetSource %q: %w", name, registry.ErrNotFound)
}

// GetDestination returns ErrNotFound in server mode — destinations are always local.
func (c *Client) GetDestination(_ context.Context, name string) (*spec.DestinationDef, error) {
	return nil, fmt.Errorf("GetDestination %q: %w", name, registry.ErrNotFound)
}

// PublishEvent publishes a new event version to the server.
func (c *Client) PublishEvent(ctx context.Context, event spec.EventDef) error {
	return c.post(ctx, c.cfg.BaseURL+"/v1/events", event, nil)
}

// Diff fetches the detected changes between two event versions from the server.
func (c *Client) Diff(ctx context.Context, namespace, name, from, to string) ([]spec.Change, error) {
	u := fmt.Sprintf("%s/v1/diff/%s/%s/%s/%s", c.cfg.BaseURL, namespace, name, from, to)
	var changes []spec.Change
	if err := c.get(ctx, u, &changes); err != nil {
		return nil, err
	}
	return changes, nil
}

// Status calls GET /v1/health and returns the server status. An error is returned
// if the server is unreachable or returns a non-2xx status.
func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	var s StatusResponse
	if err := c.get(ctx, c.cfg.BaseURL+"/v1/health", &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// CreateAPIKey calls POST /v1/admin/keys. On a fresh server with no keys, no API key
// credential is required. The returned CreatedAPIKey contains the raw key (shown once).
// expiresIn uses extended duration syntax: "90d", "1y", or standard Go durations.
func (c *Client) CreateAPIKey(ctx context.Context, role, name, expiresIn string) (*CreatedAPIKey, error) {
	body := map[string]string{"role": role}
	if name != "" {
		body["name"] = name
	}
	if expiresIn != "" {
		body["expires_in"] = expiresIn
	}
	var result CreatedAPIKey
	if err := c.post(ctx, c.cfg.BaseURL+"/v1/admin/keys", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListAPIKeys calls GET /v1/admin/keys (admin role required).
func (c *Client) ListAPIKeys(ctx context.Context) ([]APIKeyRecord, error) {
	var records []APIKeyRecord
	if err := c.get(ctx, c.cfg.BaseURL+"/v1/admin/keys", &records); err != nil {
		return nil, err
	}
	return records, nil
}

// RevokeAPIKey calls DELETE /v1/admin/keys/{id} (admin role required).
func (c *Client) RevokeAPIKey(ctx context.Context, id int64) error {
	return c.delete(ctx, fmt.Sprintf("%s/v1/admin/keys/%d", c.cfg.BaseURL, id))
}

// RegisterWebhook calls POST /v1/webhooks to register a webhook URL (admin role required).
func (c *Client) RegisterWebhook(ctx context.Context, webhookURL string) error {
	return c.post(ctx, c.cfg.BaseURL+"/v1/webhooks", map[string]string{"url": webhookURL}, nil)
}

// ListWebhooksAdmin calls GET /v1/webhooks (admin role required).
func (c *Client) ListWebhooksAdmin(ctx context.Context) ([]WebhookRecord, error) {
	var records []WebhookRecord
	if err := c.get(ctx, c.cfg.BaseURL+"/v1/webhooks", &records); err != nil {
		return nil, err
	}
	return records, nil
}

// RemoveWebhook calls DELETE /v1/webhooks/{id} (admin role required).
func (c *Client) RemoveWebhook(ctx context.Context, id int64) error {
	return c.delete(ctx, fmt.Sprintf("%s/v1/webhooks/%d", c.cfg.BaseURL, id))
}

// ListAuditLog calls GET /v1/audit with optional query parameters (admin role required).
// Supported params: limit, since, until, entity, user.
func (c *Client) ListAuditLog(ctx context.Context, params url.Values) ([]AuditEntry, error) {
	u := c.cfg.BaseURL + "/v1/audit"
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	var entries []AuditEntry
	if err := c.get(ctx, u, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (c *Client) auth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
}

func (c *Client) get(ctx context.Context, u string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return err
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return c.decode(resp, dst)
}

func (c *Client) delete(ctx context.Context, u string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, http.NoBody)
	if err != nil {
		return err
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return c.decode(resp, nil)
}

func (c *Client) post(ctx context.Context, u string, body, dst any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return c.decode(resp, dst)
}

// decode inspects the HTTP response and decodes a successful body into dst.
func (c *Client) decode(resp *http.Response, dst any) error {
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		if dst != nil {
			return json.NewDecoder(resp.Body).Decode(dst)
		}
		return nil
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return registry.ErrNotFound
	default:
		var e struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&e); err == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
}

// getWithCache performs a GET request and writes the response body to the local cache on
// success. If the transport fails (server unreachable) and a cached response exists for this
// URL, the cached bytes are decoded into dst instead of returning an error.
func (c *Client) getWithCache(ctx context.Context, u string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return err
	}
	c.auth(req)
	resp, transportErr := c.http.Do(req)
	if transportErr != nil {
		if c.cfg.CacheDir != "" {
			if data, cacheErr := c.readCache(urlCacheKey(u)); cacheErr == nil {
				if dst != nil {
					return json.Unmarshal(data, dst)
				}
				return nil
			}
		}
		return transportErr
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		if c.cfg.CacheDir != "" {
			_ = c.writeCache(urlCacheKey(u), body)
		}
		if dst != nil {
			return json.Unmarshal(body, dst)
		}
		return nil
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return registry.ErrNotFound
	default:
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
}

// urlCacheKey returns a filesystem-safe cache key derived from the URL path and query,
// stripping the host so the key is stable across server restarts or URL changes.
func urlCacheKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return sanitizeForFS(rawURL)
	}
	key := u.Path
	if u.RawQuery != "" {
		key += "?" + u.RawQuery
	}
	return sanitizeForFS(strings.TrimLeft(key, "/"))
}

func sanitizeForFS(s string) string {
	return strings.NewReplacer("/", "_", "?", "_", "&", "_", "=", "_").Replace(s)
}

func (c *Client) readCache(key string) ([]byte, error) {
	return os.ReadFile(filepath.Join(c.cfg.CacheDir, key+".json"))
}

func (c *Client) writeCache(key string, data []byte) error {
	if err := os.MkdirAll(c.cfg.CacheDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.cfg.CacheDir, key+".json"), data, 0o644)
}
