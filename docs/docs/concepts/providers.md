---
sidebar_position: 3
---

# Providers

A **provider** is an adapter that delivers analytics events to a specific backend (Amplitude, Mixpanel, PostHog, etc.). Providers implement a stable interface so your application code never depends on a vendor SDK directly.

## The Provider interface

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs>
<TabItem value="go" label="Go">

```go
type Provider interface {
    Metadata() ProviderMetadata
    Hooks() []hooks.Hook

    Track(ctx context.Context, event TrackMessage) error
    Identify(ctx context.Context, msg IdentifyMessage) error
    Group(ctx context.Context, msg GroupMessage) error
    Page(ctx context.Context, msg PageMessage) error
    Alias(ctx context.Context, msg AliasMessage) error

    Flush(ctx context.Context) error
    Shutdown(ctx context.Context) error
}
```

</TabItem>
<TabItem value="ts" label="TypeScript">

```typescript
interface Provider {
  metadata(): ProviderMetadata;
  hooks(): Hook[];

  track(msg: TrackMessage): Promise<void>;
  identify(msg: IdentifyMessage): Promise<void>;
  group(msg: GroupMessage): Promise<void>;
  page(msg: PageMessage): Promise<void>;
  alias(msg: AliasMessage): Promise<void>;

  flush(): Promise<void>;
  shutdown(): Promise<void>;
}
```

</TabItem>
<TabItem value="kotlin" label="Kotlin">

```kotlin
interface Provider {
  fun metadata(): ProviderMetadata
  fun hooks(): List<Hook>

  suspend fun track(msg: TrackMessage)
  suspend fun identify(msg: IdentifyMessage)
  suspend fun group(msg: GroupMessage)
  suspend fun page(msg: PageMessage)
  suspend fun alias(msg: AliasMessage)

  suspend fun flush()
  suspend fun shutdown()
}
```

</TabItem>
</Tabs>

Providers that don't support a given operation return `ErrUnsupportedOperation` rather than silently no-op — preventing silent data loss.

## Message types

Every call to the runtime dispatches a typed message struct to the provider:

| Method | Message type | Key fields |
|--------|-------------|-----------|
| `Track` | `TrackMessage` | `EventName`, `Properties`, `UserId`, `AnonymousId`, `Context` |
| `Identify` | `IdentifyMessage` | `UserId`, `AnonymousId`, `Traits`, `Context` |
| `Group` | `GroupMessage` | `UserId`, `GroupId`, `Traits`, `Context` |
| `Page` | `PageMessage` | `UserId`, `Name`, `Properties`, `Context` |
| `Alias` | `AliasMessage` | `UserId`, `PreviousId` |

`MessageContext` carries structured environment metadata: `UserAgent`, `Locale`, `IP`, `App`, `Device`, `OS`, `Screen`, `Campaign`, and an `Extra` map for custom keys.

## Provider capabilities

<Tabs>
<TabItem value="go" label="Go">

```go
type ProviderCapabilities struct {
    Track    bool
    Identify bool
    Group    bool
    Page     bool
    Alias    bool
}
```

</TabItem>
<TabItem value="ts" label="TypeScript">

```typescript
interface ProviderCapabilities {
  track: boolean;
  identify: boolean;
  group: boolean;
  page: boolean;
  alias: boolean;
}
```

</TabItem>
<TabItem value="kotlin" label="Kotlin">

```kotlin
data class ProviderCapabilities(
    val track: Boolean = true,
    val identify: Boolean = true,
    val group: Boolean = true,
    val page: Boolean = true,
    val alias: Boolean = true,
)
```

</TabItem>
</Tabs>

The runtime checks capabilities before dispatch and records `Dropped` outcomes for unsupported operations.

## Multi-provider dispatch

The client sends events to all registered providers simultaneously:

<Tabs>
<TabItem value="go" label="Go">

```go
client := analytics.NewClient(
    analytics.WithProviders(amplitudeProvider, posthogProvider),
)

// Both providers receive the event concurrently.
// Per-provider outcomes accessible via TrackDetailed:
result, err := client.TrackDetailed(ctx, event)
// result.Success  — providers that succeeded
// result.Failed   — providers that failed (permanent)
```

</TabItem>
<TabItem value="ts" label="TypeScript">

```typescript
const client = new Client({
  providers: [amplitudeProvider, posthogProvider],
});

// Both providers receive the event concurrently via Promise.allSettled.
// Per-provider outcomes accessible via trackDetailed:
const result = await client.trackDetailed(event);
// result.success  — providers that succeeded
// result.failed   — providers that failed (permanent)
```

</TabItem>
<TabItem value="kotlin" label="Kotlin">

```kotlin
val client = Client(ClientOptions(
    providers = listOf(amplitudeProvider, posthogProvider),
))

// Both providers receive the event concurrently via coroutines.
// Per-provider outcomes accessible via trackDetailed:
val result = client.trackDetailed(event)
// result.success  — providers that succeeded
// result.failed   — providers that failed (permanent)
```

</TabItem>
</Tabs>

## Delivery states

| State | Meaning |
|-------|---------|
| `Delivered` | Provider confirmed receipt |
| `Failed` | Permanently rejected after max retries |
| `Dropped` | Discarded by sampling, queue overflow, schema violation, or unsupported operation |

## Built-in providers

| Provider | Language | Status |
|----------|----------|--------|
| [Amplitude](../providers/amplitude.md) | Go, TypeScript, Kotlin | ✅ Available |
| [Noop](../providers/noop.md) | Go | ✅ Available |
| PostHog | Go | ❌ Planned |
| Mixpanel | Go | ❌ Planned |
| Segment | Go | ❌ Planned |
| GA4 | Go | ❌ Planned |

## Writing a custom provider

See [Providers — Custom](../providers/custom.md) for a full walkthrough.
