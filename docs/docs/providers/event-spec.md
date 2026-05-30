---
sidebar_position: 3
---

# event-spec Server Provider

The event-spec server provider is a **thin client** that forwards analytics events to an [event-spec runtime ingestion server](../server/index.md) over HTTP. The server runs the full hook chain, validation, sampling, batching, and multi-provider dispatch — the client only needs the server URL, an API key, and a source name.

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## When to use this provider

| Scenario | Recommendation |
|----------|----------------|
| Web apps where bundle size matters | ✅ Thin client — no vendor SDK in the browser |
| Centralised provider credentials | ✅ API keys live server-side only |
| Dynamic provider routing without client updates | ✅ Change server config, not client code |
| PII filtering or server-side governance hooks | ✅ Applied server-side before reaching any vendor |
| Native mobile apps requiring offline buffering | ❌ Use the embedded Amplitude or PostHog provider instead |

## Installation

<Tabs>
<TabItem value="go" label="Go">

```bash
go get github.com/dejanradmanovic/event-spec@latest
```

The event-spec server provider is included in the module at `provider/event-spec`.

</TabItem>
<TabItem value="ts" label="TypeScript">

```bash
npm install @dejanradmanovic/event-spec-provider
```

</TabItem>
<TabItem value="kotlin" label="Kotlin">

Add to `gradle/libs.versions.toml`:
```toml
[libraries]
event-spec-provider-event-spec = { module = "io.event-spec:kotlin-provider-event-spec", version = "1.0.0" }
```

```kotlin title="build.gradle.kts"
dependencies {
    implementation(libs.event.spec.provider.event.spec)
}
```

</TabItem>
</Tabs>

## Basic setup

<Tabs>
<TabItem value="go" label="Go">

```go
import (
    "github.com/dejanradmanovic/event-spec/provider"
    eventspec "github.com/dejanradmanovic/event-spec/provider/event-spec"
)

p, err := eventspec.New(eventspec.Config{
    BaseURL: "https://events.internal",
    APIKey:  "bearer-token",
    Source:  "web-app",
})
```

</TabItem>
<TabItem value="ts" label="TypeScript">

```typescript
import { EventSpecProvider } from '@dejanradmanovic/event-spec-provider';

const p = new EventSpecProvider({
    baseURL: 'https://events.internal',
    apiKey: 'bearer-token',
    source: 'web-app',
});
```

</TabItem>
<TabItem value="kotlin" label="Kotlin">

```kotlin
import io.eventspec.analytics.eventspec.EventSpecConfig
import io.eventspec.analytics.eventspec.EventSpecProvider

val p = EventSpecProvider(EventSpecConfig(
    baseUrl = "https://events.internal",
    apiKey = "bearer-token",
    source = "web-app",
))
```

</TabItem>
</Tabs>

## Full configuration

<Tabs>
<TabItem value="go" label="Go">

```go
p, err := eventspec.New(eventspec.Config{
    ProviderConfig: provider.ProviderConfig{
        // Proxy through your domain to route through a firewall or bypass ad-blockers
        ProxyURL:  "https://analytics.yourcompany.com",
        ProxyMode: provider.ProxyReverseProxy,

        // Retry on transient HTTP errors (429, 500, 502, 503, 504)
        RetryConfig: provider.RetryConfig{
            MaxRetries:     5,
            InitialBackoff: 100 * time.Millisecond,
            MaxBackoff:     30 * time.Second,
            Jitter:         true,
        },

        // Token-bucket rate limiting
        RateLimitConfig: provider.RateLimitConfig{
            RequestsPerSecond: 100,
        },
    },
    BaseURL: "https://events.internal",
    APIKey:  "bearer-token",
    Source:  "web-app",
})
```

</TabItem>
<TabItem value="ts" label="TypeScript">

```typescript
const p = new EventSpecProvider({
    baseURL: 'https://events.internal',
    apiKey: 'bearer-token',
    source: 'web-app',
    proxyUrl: 'https://analytics.yourcompany.com',
    proxyMode: 'reverse_proxy',
    retryConfig: { maxRetries: 5, jitter: true },
    rateLimitConfig: { requestsPerSecond: 100 },
});
```

</TabItem>
<TabItem value="kotlin" label="Kotlin">

```kotlin
val p = EventSpecProvider(EventSpecConfig(
    baseUrl = "https://events.internal",
    apiKey = "bearer-token",
    source = "web-app",
    providerConfig = ProviderConfig(
        proxyUrl = "https://analytics.yourcompany.com",
        proxyMode = ProxyMode.REVERSE_PROXY,
        retryConfig = RetryConfig(maxRetries = 5, jitter = true),
        rateLimitConfig = RateLimitConfig(requestsPerSecond = 100),
    ),
))
```

</TabItem>
</Tabs>

## Wire format

All five analytics operations POST to the corresponding `/v1/*` endpoint on the ingestion server.
Every request includes `Authorization: Bearer <api-key>` and `Content-Type: application/json`.

| Method | Endpoint | Key body fields |
|--------|----------|-----------------|
| `track` | `POST /v1/track` | `source`, `event_name`, `properties`, `context`, `timestamp` |
| `identify` | `POST /v1/identify` | `source`, `user_id`, `anonymous_id`, `traits`, `timestamp` |
| `group` | `POST /v1/group` | `source`, `user_id`, `group_id`, `traits`, `timestamp` |
| `page` | `POST /v1/page` | `source`, `user_id`, `name`, `properties`, `timestamp` |
| `alias` | `POST /v1/alias` | `source`, `user_id`, `previous_id`, `timestamp` |
| `flush` | `POST /v1/flush` | `source` |

**Example track request body:**

```json
{
  "source": "web-app",
  "event_name": "Product Viewed",
  "properties": { "product_id": "SKU-123" },
  "context": {
    "user_id": "user-456",
    "anonymous_id": "anon-789",
    "attributes": { "user_agent": "Mozilla/5.0 ...", "ip_address": "1.2.3.4" }
  },
  "timestamp": "2026-05-21T10:30:00Z"
}
```

`context.attributes` is populated from `MessageContext.UserAgent`, `MessageContext.IPAddress`, and
`MessageContext.Extra`. If omitted, the server fills them in from the HTTP request automatically.

## Flush and shutdown

`Flush` sends `POST /v1/flush` to trigger a server-side drain of queued events and waits for the
response. `Shutdown` calls `Flush` then closes the provider; all subsequent method calls return an
error.

## Supported operations

All five analytics operations are supported: `track`, `identify`, `group`, `page`, `alias`.
