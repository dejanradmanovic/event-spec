---
sidebar_position: 2
---

# Amplitude Provider

The Amplitude provider delivers events to the [Amplitude HTTP API v2](https://www.docs.developers.amplitude.com/analytics/apis/http-v2-api/) using batching, retry, and optional reverse-proxy support.

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## Installation

<Tabs>
<TabItem value="go" label="Go">

```bash
go get github.com/dejanradmanovic/event-spec@latest
```

The Amplitude provider is included in the module at `provider/amplitude`.

</TabItem>
<TabItem value="ts" label="TypeScript">

```bash
npm install @dejanradmanovic/event-spec-provider-amplitude
```

</TabItem>
</Tabs>

## Basic setup

<Tabs>
<TabItem value="go" label="Go">

```go
import (
    "github.com/dejanradmanovic/event-spec/provider"
    "github.com/dejanradmanovic/event-spec/provider/amplitude"
)

amp, err := amplitude.New(amplitude.Config{
    ProviderConfig: provider.ProviderConfig{
        APIKey:     "${AMPLITUDE_API_KEY}",
        SecretType: provider.SecretEnvVar,
    },
})
```

</TabItem>
<TabItem value="ts" label="TypeScript">

```typescript
import { AmplitudeProvider } from '@dejanradmanovic/event-spec-provider-amplitude';

const amp = new AmplitudeProvider({
    apiKey: process.env.AMPLITUDE_API_KEY!,
});
```

</TabItem>
</Tabs>

## Full configuration (Go)

```go
amp, err := amplitude.New(amplitude.Config{
    ProviderConfig: provider.ProviderConfig{
        APIKey:     "${AMPLITUDE_API_KEY}",
        SecretType: provider.SecretEnvVar,

        // Proxy through your domain to bypass ad-blockers
        ProxyURL:  "https://analytics.yourcompany.com/amp",
        ProxyMode: provider.ProxyReverseProxy,

        // Batching
        BatchSize:     100,
        FlushInterval: 5 * time.Second,
        MaxQueueSize:  10_000,
        OverflowPolicy: provider.OverflowDropOldest,

        // Retry
        RetryConfig: provider.RetryConfig{
            MaxRetries:     3,
            InitialBackoff: 100 * time.Millisecond,
            MaxBackoff:     30 * time.Second,
            Multiplier:     2.0,
            Jitter:         true,
        },

        // Rate limiting
        RateLimitConfig: provider.RateLimitConfig{
            RequestsPerSecond: 30,
        },
    },
})
```

## Configuration reference

### Secret management

| `SecretType` | Behavior |
|-------------|---------|
| `SecretEnvVar` | Reads the API key from the environment variable named by `APIKey` (e.g. `APIKey: "${AMPLITUDE_API_KEY}"`) |
| `SecretPlain` | Uses `APIKey` as the literal key value (not recommended in production) |

### Proxy modes

| `ProxyMode` | Behavior |
|------------|---------|
| `ProxyNone` | Sends directly to `api2.amplitude.com` |
| `ProxyReverseProxy` | Routes through `ProxyURL` (your reverse proxy forwards to Amplitude) |

The proxy mode is used to bypass browser ad-blockers in client-side applications by routing analytics through your own domain.

### Overflow policies

| `OverflowPolicy` | Behavior |
|-----------------|---------|
| `OverflowDropOldest` | Drops oldest buffered events when queue is full |
| `OverflowDropNewest` | Drops new events when queue is full (back pressure) |
| `OverflowBlock` | Blocks the caller until queue space is available |

## Destination config (YAML)

When using the server registry, declare destination config in YAML:

```yaml title="destinations/amplitude.yaml"
name: amplitude
type: amplitude
config:
  api_key: "${AMPLITUDE_API_KEY}"
  batch_size: 100
  flush_interval: "5s"
```

## Supported operations

All five analytics operations are supported: `track`, `identify`, `group`, `page`, `alias`.
