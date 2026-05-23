---
sidebar_position: 1
---

# Go SDK

The Go SDK is the reference runtime implementation of event-spec. It provides the analytics client, provider interface, hook lifecycle, context propagation, and testing utilities.

## Installation

```bash
go get github.com/dejanradmanovic/event-spec@latest
```

Module path: `github.com/dejanradmanovic/event-spec`

## Setup

```go
import (
    core "github.com/dejanradmanovic/event-spec/analytics"
    "github.com/dejanradmanovic/event-spec/provider"
    "github.com/dejanradmanovic/event-spec/provider/amplitude"
)

amp, err := amplitude.New(amplitude.Config{
    ProviderConfig: provider.ProviderConfig{
        APIKey:     "${AMPLITUDE_API_KEY}",
        SecretType: provider.SecretEnvVar,
    },
})
if err != nil {
    panic(err)
}

client := core.NewClient(core.WithProviders(amp))
defer client.Shutdown(context.Background())
```

## Global API

For simple applications without per-request identity, the package-level functions use a global client:

```go
core.SetGlobalProvider(amp)

core.Track(ctx, core.Event{
    Name:       "Product Viewed",
    Properties: map[string]any{"product_id": "SKU-123"},
})
```

## Tracking events

```go
err := client.Track(ctx, core.Event{
    Name: "Product Viewed",
    Properties: map[string]any{
        "product_id": "SKU-123",
        "category":   "electronics",
    },
})
```

### Per-provider outcomes

`Track` returns non-nil error only for pre-dispatch failures (hook cancelled, schema invalid). Use `TrackDetailed` for per-provider results:

```go
result, err := client.TrackDetailed(ctx, event)
// result.Success  — providers that succeeded
// result.Failed   — providers that failed permanently
// result.PartialSuccess — at least one succeeded
```

## Identify

```go
err := client.Identify(ctx, "user-123", map[string]any{
    "email": "alice@example.com",
    "plan":  "pro",
})
```

## Group

```go
err := client.Group(ctx, "user-123", "org-456", map[string]any{
    "name":  "Acme Corp",
    "plan":  "enterprise",
})
```

## Page

```go
err := client.Page(ctx, core.PageEvent{
    Name:       "Product Detail",
    Properties: map[string]any{"url": "/products/sku-123"},
})
```

## Using generated wrappers

With generated code, calls are fully typed:

```go
import generated "your-module/generated"

es := generated.New(client)

es.ProductViewed(ctx, generated.ProductViewedProperties{
    Category:  generated.ProductViewedCategoryElectronics,
    ProductId: "SKU-123",
})
```

## Context propagation

```go
// Set global context (startup)
core.SetGlobalContext(core.AnalyticsContext{
    Attributes: map[string]any{"locale": "en-US"},
})

// HTTP middleware pattern
txCtx := core.TransactionContext{
    UserID:      extractUserID(r),
    AnonymousID: extractSessionID(r),
}
ctx = core.WithAnalyticsContext(r.Context(), txCtx)

// Per-call override
err := client.Track(ctx, event,
    core.WithContextOverride(core.AnalyticsContext{UserID: "override-id"}),
)
```

## Multiple providers

```go
client := core.NewClient(
    core.WithProviders(amplitudeProvider, posthogProvider),
)
// Both receive every event concurrently.
```

## Hooks

```go
import (
    "github.com/dejanradmanovic/event-spec/hooks/validation"
    "github.com/dejanradmanovic/event-spec/hooks/sampling"
)

client := core.NewClient(
    core.WithProviders(amp),
    core.WithHooks(
        validation.New(lookup),
        sampling.New(sampling.Config{
            Strategy: sampling.UserIDHash,
            Rate:     0.1,
        }),
    ),
)
```

## Testing utilities

### CaptureProvider

Records every provider call for test assertions:

```go
import "github.com/dejanradmanovic/event-spec/testutil"

cap := testutil.NewCaptureProvider("test")
client := core.NewClient(core.WithProviders(cap))

client.Track(ctx, event)

assert.Equal(t, "Product Viewed", cap.Tracks[0].EventName)
assert.Len(t, cap.Tracks, 1)

cap.Reset() // clear captured events
```

### MockProvider

Simulates latency and per-operation errors:

```go
mock := testutil.NewMockProvider("test",
    testutil.WithLatency(50*time.Millisecond),
    testutil.WithTrackError(errors.New("simulated failure")),
)
```

## Package layout

| Package | Purpose |
|---------|---------|
| `analytics` | Client, global API, context, dispatch |
| `provider` | Provider interface, message types, config |
| `provider/amplitude` | Amplitude HTTP batch API provider |
| `provider/noop` | No-op provider |
| `hooks` | Hook interface, chain executor, UnimplementedHook |
| `hooks/sampling` | Sampling hook |
| `hooks/validation` | Schema validation hook |
| `registry` | Registry interface |
| `registry/local` | Filesystem registry |
| `registry/git` | Git-backed registry |
| `registry/server/client` | HTTP client for server mode |
| `spec` | EventDef, SourceDef, validation, diff |
| `codegen` | Code generation engine |
| `testutil` | CaptureProvider, MockProvider |
