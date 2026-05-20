package amplitude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"event-spec/hooks"
	"event-spec/provider"
)

// Provider sends analytics events to Amplitude via the batch HTTP API.
//
// Capabilities: Track ✅, Identify ✅, Group ✅, Page ❌, Alias ✅.
// Page returns ErrUnsupportedOperation — Amplitude has no native page concept.
type Provider struct {
	apiKey      string
	endpoint    string
	transport   *provider.Transport
	queue       *provider.Queue
	rateLimiter *provider.RateLimiter
}

// New constructs and starts an Amplitude Provider from cfg.
// The API key is resolved according to cfg.SecretType before the provider starts.
// Returns an error if secret resolution or transport construction fails.
func New(cfg Config) (*Provider, error) {
	apiKey, err := provider.ResolveSecret(cfg.APIKey, cfg.SecretType)
	if err != nil {
		return nil, fmt.Errorf("amplitude: %w", err)
	}

	transport, err := provider.NewTransport(cfg.ProviderConfig)
	if err != nil {
		return nil, fmt.Errorf("amplitude: %w", err)
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	p := &Provider{
		apiKey:      apiKey,
		endpoint:    endpoint,
		transport:   transport,
		rateLimiter: provider.NewRateLimiter(cfg.RateLimitConfig),
	}
	p.queue = provider.NewQueue(cfg.ProviderConfig, p.flushBatch)
	return p, nil
}

// Metadata returns the Amplitude provider's identity and capability set.
func (p *Provider) Metadata() provider.ProviderMetadata {
	return provider.ProviderMetadata{
		Name:    "amplitude",
		Version: version,
		Capabilities: provider.ProviderCapabilities{
			Track:    true,
			Identify: true,
			Group:    true,
			Page:     false,
			Alias:    true,
		},
	}
}

// Hooks returns no provider-level hooks.
func (p *Provider) Hooks() []hooks.Hook { return nil }

// Track enqueues a track event for batched delivery to Amplitude.
func (p *Provider) Track(ctx context.Context, msg provider.TrackMessage) error {
	return p.queue.Enqueue(ctx, provider.QueuedMessage{Op: "track", Track: &msg})
}

// Identify enqueues an identify call as an Amplitude $identify event.
func (p *Provider) Identify(ctx context.Context, msg provider.IdentifyMessage) error {
	return p.queue.Enqueue(ctx, provider.QueuedMessage{Op: "identify", Identify: &msg})
}

// Group enqueues a group call as an Amplitude $groupidentify event.
func (p *Provider) Group(ctx context.Context, msg provider.GroupMessage) error {
	return p.queue.Enqueue(ctx, provider.QueuedMessage{Op: "group", Group: &msg})
}

// Page returns ErrUnsupportedOperation; Amplitude has no native page concept.
func (p *Provider) Page(_ context.Context, _ provider.PageMessage) error {
	return provider.ErrUnsupportedOperation
}

// Alias enqueues an alias call that links PreviousID to UserID in Amplitude.
func (p *Provider) Alias(ctx context.Context, msg provider.AliasMessage) error {
	return p.queue.Enqueue(ctx, provider.QueuedMessage{Op: "alias", Alias: &msg})
}

// Flush synchronously drains all buffered events to Amplitude.
func (p *Provider) Flush(ctx context.Context) error {
	return p.queue.Flush(ctx)
}

// Shutdown flushes remaining events and stops background processing.
func (p *Provider) Shutdown(ctx context.Context) error {
	return p.queue.Shutdown(ctx)
}

// flushBatch converts a batch of queued messages to amplitudeEvents and sends them.
func (p *Provider) flushBatch(ctx context.Context, batch []provider.QueuedMessage) error {
	events := make([]amplitudeEvent, 0, len(batch))
	for _, msg := range batch {
		switch msg.Op {
		case "track":
			events = append(events, mapTrackMessage(*msg.Track))
		case "identify":
			events = append(events, mapIdentifyMessage(*msg.Identify))
		case "group":
			events = append(events, mapGroupMessage(*msg.Group))
		case "alias":
			events = append(events, mapAliasMessage(*msg.Alias))
		}
	}
	if len(events) == 0 {
		return nil
	}
	return p.sendBatch(ctx, events)
}

// sendBatch encodes events and POSTs them to the Amplitude batch endpoint,
// applying the configured rate limit before each HTTP request.
func (p *Provider) sendBatch(ctx context.Context, events []amplitudeEvent) error {
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return err
	}

	payload := amplitudeBatchRequest{
		APIKey: p.apiKey,
		Events: events,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("amplitude: marshal payload: %w", err)
	}

	effectiveURL, err := p.transport.RewriteURL(p.endpoint)
	if err != nil {
		return fmt.Errorf("amplitude: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, effectiveURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("amplitude: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.transport.Do(ctx, req, body)
	if err != nil {
		return fmt.Errorf("amplitude: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("amplitude: unexpected status %d", resp.StatusCode)
	}
	return nil
}
