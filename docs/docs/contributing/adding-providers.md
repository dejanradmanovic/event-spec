---
sidebar_position: 2
---

# Adding Providers

This guide walks through adding a new analytics provider to event-spec. The Go implementation is the reference; TypeScript and Kotlin follow the same structure.

## File structure

Create a new package under `provider/`:

```
provider/
└── posthog/
    ├── provider.go     # Provider implementation
    ├── config.go       # Config struct and validation
    ├── mapper.go       # Property mapping to PostHog API format
    └── provider_test.go
```

## Step 1 — Define the config

```go title="provider/posthog/config.go"
package posthog

import "github.com/dejanradmanovic/event-spec/provider"

type Config struct {
    provider.ProviderConfig
    // PostHog-specific fields
    Host string // default: https://app.posthog.com
}
```

## Step 2 — Implement the provider

```go title="provider/posthog/provider.go"
package posthog

import (
    "context"
    "github.com/dejanradmanovic/event-spec/hooks"
    "github.com/dejanradmanovic/event-spec/provider"
)

type PostHogProvider struct {
    cfg       Config
    transport *provider.Transport
    queue     *provider.Queue
}

func New(cfg Config) (*PostHogProvider, error) {
    transport, err := provider.NewTransport(cfg.ProviderConfig)
    if err != nil {
        return nil, err
    }
    p := &PostHogProvider{cfg: cfg, transport: transport}
    p.queue = provider.NewQueue(cfg.ProviderConfig, p.flush)
    return p, nil
}

func (p *PostHogProvider) Metadata() provider.ProviderMetadata {
    return provider.ProviderMetadata{
        Name:    "posthog",
        Version: "1.0.0",
        Capabilities: provider.ProviderCapabilities{
            Track:    true,
            Identify: true,
            Group:    true,
            Page:     false,
            Alias:    false,
        },
    }
}

func (p *PostHogProvider) Hooks() []hooks.Hook { return nil }

func (p *PostHogProvider) Track(ctx context.Context, msg provider.TrackMessage) error {
    return p.queue.Enqueue(ctx, provider.QueuedMessage{Op: "track", Track: &msg})
}

func (p *PostHogProvider) Identify(ctx context.Context, msg provider.IdentifyMessage) error {
    // Map identify to PostHog's $identify event
    return nil
}

func (p *PostHogProvider) Group(ctx context.Context, msg provider.GroupMessage) error {
    return nil
}

func (p *PostHogProvider) Page(_ context.Context, _ provider.PageMessage) error {
    return provider.ErrUnsupportedOperation
}

func (p *PostHogProvider) Alias(_ context.Context, _ provider.AliasMessage) error {
    return provider.ErrUnsupportedOperation
}

func (p *PostHogProvider) Flush(ctx context.Context) error {
    return p.queue.Flush(ctx)
}

func (p *PostHogProvider) Shutdown(ctx context.Context) error {
    return p.queue.Shutdown(ctx)
}

// flush is the FlushFunc called by the queue to batch-send events to PostHog.
func (p *PostHogProvider) flush(ctx context.Context, batch []provider.QueuedMessage) error {
    // Send batch to PostHog API
    return nil
}
```

## Step 3 — Write tests

Use an HTTP test server to capture actual API calls — no mocking:

```go title="provider/posthog/provider_test.go"
func TestTrack(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request format
        w.WriteHeader(http.StatusOK)
    }))
    defer srv.Close()

    p, err := New(Config{
        ProviderConfig: provider.ProviderConfig{
            APIKey:     "test-key",
            SecretType: provider.SecretInline,
        },
        Host: srv.URL,
    })
    require.NoError(t, err)

    err = p.Track(context.Background(), provider.TrackMessage{EventName: "Test Event"})
    require.NoError(t, err)
    require.NoError(t, p.Flush(context.Background()))
}
```

For integration tests that verify the full dispatch pipeline (hooks, context merging, multi-provider), use `testutil.CaptureProvider` from the `testutil` package.

## Step 4 — Add destination YAML support

Update `spec/schema.go` to recognize the new provider type in destination validation if needed.

## Step 5 — Document it

Add a page at `docs/docs/providers/<name>.md` following the [Amplitude provider page](../providers/amplitude.md) structure.

## Checklist

- [ ] `New()` constructor validates config and resolves the secret via `provider.ResolveSecret`
- [ ] All 5 operations implemented (unsupported ones return `ErrUnsupportedOperation`)
- [ ] `flush` callback signature matches `FlushFunc`: `func(ctx context.Context, batch []provider.QueuedMessage) error`
- [ ] `Flush()` and `Shutdown()` delegate to the queue
- [ ] `Metadata().Capabilities` accurately reflects supported operations
- [ ] HTTP test with a real test server (no HTTP mocking)
- [ ] TypeScript provider implemented at `sdk/typescript/packages/provider-<name>/` following the `provider-amplitude` package structure
- [ ] TypeScript provider exports the same capability set as the Go implementation
- [ ] Kotlin provider implemented as a separate Gradle submodule at `sdk/kotlin/provider-<name>/` following `sdk/kotlin/provider-amplitude/`
- [ ] Kotlin provider added to `sdk/kotlin/settings.gradle.kts` (`include(":provider-<name>")`)
- [ ] Provider added to every SDK in the repo (currently: Go at `provider/<name>/`, TypeScript at `sdk/typescript/packages/provider-<name>/`, Kotlin at `sdk/kotlin/provider-<name>/`)
- [ ] `make test && make lint` passes
