package eventspec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/dejanradmanovic/event-spec/hooks"
	"github.com/dejanradmanovic/event-spec/provider"
)

// ErrShutdown is returned by all methods after Shutdown has been called.
var ErrShutdown = errors.New("event-spec provider: already shut down")

// ServerProvider implements provider.Provider by forwarding every analytics call
// to the event-spec runtime ingestion server. The server runs the full hook chain,
// validation, sampling, and multi-provider dispatch; the client only needs the
// server URL, an API key, and a source name.
type ServerProvider struct {
	source      string
	apiKey      string
	baseURL     string
	transport   *provider.Transport
	rateLimiter *provider.RateLimiter
	closed      atomic.Bool
}

// New constructs a ServerProvider from cfg.
// The API key is resolved according to cfg.SecretType before the provider starts.
func New(cfg Config) (*ServerProvider, error) {
	apiKey, err := provider.ResolveSecret(cfg.APIKey, cfg.SecretType)
	if err != nil {
		return nil, fmt.Errorf("event-spec provider: %w", err)
	}

	transport, err := provider.NewTransport(cfg.ProviderConfig)
	if err != nil {
		return nil, fmt.Errorf("event-spec provider: %w", err)
	}

	return &ServerProvider{
		source:      cfg.Source,
		apiKey:      apiKey,
		baseURL:     cfg.BaseURL,
		transport:   transport,
		rateLimiter: provider.NewRateLimiter(cfg.RateLimitConfig),
	}, nil
}

// Metadata returns the event-spec provider identity and full capability set.
func (p *ServerProvider) Metadata() provider.ProviderMetadata {
	return provider.ProviderMetadata{
		Name:    "event-spec",
		Version: version,
		Capabilities: provider.ProviderCapabilities{
			Track:    true,
			Identify: true,
			Group:    true,
			Page:     true,
			Alias:    true,
		},
	}
}

// Hooks returns nil; validation and sampling run server-side.
func (p *ServerProvider) Hooks() []hooks.Hook { return nil }

// Track POSTs to /v1/track.
func (p *ServerProvider) Track(ctx context.Context, msg provider.TrackMessage) error {
	if p.closed.Load() {
		return ErrShutdown
	}
	return p.post(ctx, "/v1/track", trackPayload{
		Source:     p.source,
		EventName:  msg.EventName,
		Properties: msg.Properties,
		Context: contextPayload{
			UserID:      msg.UserID,
			AnonymousID: msg.AnonymousID,
			Attributes:  buildAttributes(msg.MessageContext),
		},
		Timestamp: msg.Timestamp,
	})
}

// Identify POSTs to /v1/identify.
func (p *ServerProvider) Identify(ctx context.Context, msg provider.IdentifyMessage) error {
	if p.closed.Load() {
		return ErrShutdown
	}
	return p.post(ctx, "/v1/identify", identifyPayload{
		Source: p.source,
		UserID: msg.UserID,
		Traits: msg.Traits,
		Context: contextPayload{
			AnonymousID: msg.AnonymousID,
			Attributes:  buildAttributes(msg.MessageContext),
		},
		Timestamp: msg.Timestamp,
	})
}

// Group POSTs to /v1/group.
func (p *ServerProvider) Group(ctx context.Context, msg provider.GroupMessage) error {
	if p.closed.Load() {
		return ErrShutdown
	}
	return p.post(ctx, "/v1/group", groupPayload{
		Source:  p.source,
		GroupID: msg.GroupID,
		Traits:  msg.Traits,
		Context: contextPayload{
			UserID:      msg.UserID,
			AnonymousID: msg.AnonymousID,
			Attributes:  buildAttributes(msg.MessageContext),
		},
		Timestamp: msg.Timestamp,
	})
}

// Page POSTs to /v1/page.
func (p *ServerProvider) Page(ctx context.Context, msg provider.PageMessage) error {
	if p.closed.Load() {
		return ErrShutdown
	}
	return p.post(ctx, "/v1/page", pagePayload{
		Source:     p.source,
		Name:       msg.Name,
		Properties: msg.Properties,
		Context: contextPayload{
			UserID:      msg.UserID,
			AnonymousID: msg.AnonymousID,
			Attributes:  buildAttributes(msg.MessageContext),
		},
		Timestamp: msg.Timestamp,
	})
}

// Alias POSTs to /v1/alias.
func (p *ServerProvider) Alias(ctx context.Context, msg provider.AliasMessage) error {
	if p.closed.Load() {
		return ErrShutdown
	}
	return p.post(ctx, "/v1/alias", aliasPayload{
		Source:     p.source,
		UserID:     msg.UserID,
		PreviousID: msg.PreviousID,
		Timestamp:  msg.Timestamp,
	})
}

// Flush calls POST /v1/flush, signalling the server to drain its internal event queue.
func (p *ServerProvider) Flush(ctx context.Context) error {
	if p.closed.Load() {
		return ErrShutdown
	}
	return p.post(ctx, "/v1/flush", flushPayload{Source: p.source})
}

// Shutdown flushes the server queue then closes the provider.
// All subsequent calls to any method return ErrShutdown.
func (p *ServerProvider) Shutdown(ctx context.Context) error {
	if !p.closed.CompareAndSwap(false, true) {
		return ErrShutdown
	}
	return p.post(ctx, "/v1/flush", flushPayload{Source: p.source})
}

// post serializes payload and POSTs it to baseURL+path with the bearer-token auth header.
// The rate limiter and transport handle token-bucket throttling and retry/backoff respectively.
func (p *ServerProvider) post(ctx context.Context, path string, payload any) error {
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("event-spec provider: marshal payload: %w", err)
	}

	effectiveURL, err := p.transport.RewriteURL(p.baseURL + path)
	if err != nil {
		return fmt.Errorf("event-spec provider: rewrite URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, effectiveURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("event-spec provider: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.transport.Do(ctx, req, body)
	if err != nil {
		return fmt.Errorf("event-spec provider: request to %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("event-spec provider: unexpected status %d from %s", resp.StatusCode, path)
	}
	return nil
}

// buildAttributes merges standard MessageContext fields into a flat attributes map
// so the server can use them for enrichment (e.g. user-agent, IP detection).
func buildAttributes(mc provider.MessageContext) map[string]any {
	attrs := make(map[string]any, len(mc.Extra)+2)
	if mc.UserAgent != "" {
		attrs["user_agent"] = mc.UserAgent
	}
	if mc.IPAddress != "" {
		attrs["ip_address"] = mc.IPAddress
	}
	for k, v := range mc.Extra {
		attrs[k] = v
	}
	if len(attrs) == 0 {
		return nil
	}
	return attrs
}

// Wire-format request types — mirror the event-spec server REST API.

type contextPayload struct {
	UserID      string         `json:"user_id,omitempty"`
	AnonymousID string         `json:"anonymous_id,omitempty"`
	Attributes  map[string]any `json:"attributes,omitempty"`
}

type trackPayload struct {
	Source     string         `json:"source"`
	EventName  string         `json:"event_name"`
	Properties map[string]any `json:"properties,omitempty"`
	Context    contextPayload `json:"context"`
	Timestamp  time.Time      `json:"timestamp"`
}

type identifyPayload struct {
	Source    string         `json:"source"`
	UserID    string         `json:"user_id,omitempty"`
	Traits    map[string]any `json:"traits,omitempty"`
	Context   contextPayload `json:"context,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

type groupPayload struct {
	Source    string         `json:"source"`
	GroupID   string         `json:"group_id"`
	Traits    map[string]any `json:"traits,omitempty"`
	Context   contextPayload `json:"context,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

type pagePayload struct {
	Source     string         `json:"source"`
	Name       string         `json:"name"`
	Properties map[string]any `json:"properties,omitempty"`
	Context    contextPayload `json:"context,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

type aliasPayload struct {
	Source     string    `json:"source"`
	UserID     string    `json:"user_id,omitempty"`
	PreviousID string    `json:"previous_id,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

type flushPayload struct {
	Source string `json:"source"`
}
