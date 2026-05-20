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
	"net/http"
	"net/url"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/spec"
)

// Config configures the HTTP client.
type Config struct {
	// BaseURL is the registry server base URL, e.g. "https://registry.example.com".
	BaseURL string
	// APIKey is the Bearer token used for authentication.
	APIKey string
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
	if err := c.get(ctx, u, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// GetEvent fetches an event by namespace and name.
// When version is empty the server returns the latest active version.
func (c *Client) GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error) {
	var u string
	if version != "" {
		u = fmt.Sprintf("%s/v1/events/%s/%s/%s", c.cfg.BaseURL, namespace, name, version)
	} else {
		u = fmt.Sprintf("%s/v1/events/%s/%s", c.cfg.BaseURL, namespace, name)
	}
	var def spec.EventDef
	if err := c.get(ctx, u, &def); err != nil {
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
