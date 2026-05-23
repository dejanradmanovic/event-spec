---
sidebar_position: 3
---

# Noop Provider

The noop provider silently discards all events. It is useful as a default provider during development or testing when you don't want real data sent to an analytics service.

## Usage

```go
import "github.com/dejanradmanovic/event-spec/provider/noop"

client := analytics.NewClient(
    analytics.WithProviders(noop.New()),
)
```

## Use cases

- **Development** — prevent local development events from polluting production analytics
- **Tests** — when you only care about hook behavior, not provider delivery
- **Feature flags** — use the noop provider as a fallback when analytics is disabled at runtime

## Comparison with testutil providers

| Provider | Records events | Purpose |
|----------|---------------|---------|
| `noop.New()` | No | Silent discard; default / dev placeholder |
| `testutil.NewCaptureProvider()` | Yes | Test assertions — verify what was sent |
| `testutil.NewMockProvider()` | No | Simulate latency / errors in tests |

Use `noop` when you want nothing to happen. Use `CaptureProvider` when you want to assert what _would have_ happened.
