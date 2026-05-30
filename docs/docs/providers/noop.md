---
sidebar_position: 4
---

# Noop Provider

The noop provider silently discards all events. It is useful as a default provider during development or testing when you don't want real data sent to an analytics service.

The noop provider is currently available for Go only. For Kotlin, use an inline object:

## Usage

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs>
<TabItem value="go" label="Go">

```go
import "github.com/dejanradmanovic/event-spec/provider/noop"

client := analytics.NewClient(
    analytics.WithProviders(noop.New()),
)
```

</TabItem>
<TabItem value="kotlin" label="Kotlin (inline)">

```kotlin
import io.eventspec.analytics.*

val noopProvider = object : Provider {
    override fun metadata() = ProviderMetadata(name = "noop", version = "1.0.0")
    override fun hooks(): List<Hook> = emptyList()
    override suspend fun track(msg: TrackMessage) {}
    override suspend fun identify(msg: IdentifyMessage) {}
    override suspend fun group(msg: GroupMessage) {}
    override suspend fun page(msg: PageMessage) {}
    override suspend fun alias(msg: AliasMessage) {}
    override suspend fun flush() {}
    override suspend fun shutdown() {}
}

val client = Client(ClientOptions(providers = listOf(noopProvider)))
```

</TabItem>
</Tabs>

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
