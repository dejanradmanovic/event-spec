---
sidebar_position: 4
---

# Custom Hooks

Custom hooks let you inject logic at any point in the event lifecycle without modifying providers or generated wrappers.

## Common use cases

- **Consent / privacy** — drop events for users who have not given consent
- **PII stripping** — remove or hash sensitive property values before dispatch
- **Enrichment** — add server-side properties (e.g. experiment assignments, feature flags)
- **Logging / observability** — emit structured logs or spans
- **A/B test tagging** — annotate events with active experiment variants

## Writing a custom hook

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs>
<TabItem value="go" label="Go">

```go
import (
    "context"
    "github.com/dejanradmanovic/event-spec/hooks"
)

type ConsentHook struct {
    hooks.UnimplementedHook
    consentStore ConsentStore
}

// Before runs once before any provider receives the event.
// Return an error to cancel dispatch (event is Dropped).
func (h *ConsentHook) Before(
    ctx context.Context,
    hc hooks.HookContext,
    hints hooks.HookHints,
) (*hooks.EventEnvelope, error) {
    if hc.Context.UserID == "" {
        return hc.Envelope, nil // anonymous users always pass
    }
    if !h.consentStore.HasConsent(hc.Context.UserID, "analytics") {
        return nil, hooks.ErrDrop // drop silently
    }
    return hc.Envelope, nil
}

// Finally runs after every provider result (success or failure).
func (h *ConsentHook) Finally(
    ctx context.Context,
    hc hooks.HookContext,
    result hooks.HookResult,
    hints hooks.HookHints,
) {
    log.Printf("event=%s provider=%s state=%s", hc.EventName, hc.Provider.Name, result.State)
}
```

Register it:

```go
client := analytics.NewClient(
    analytics.WithProviders(amp),
    analytics.WithHooks(&ConsentHook{consentStore: store}),
)
```

</TabItem>
<TabItem value="ts" label="TypeScript">

```typescript
import {
    UnimplementedHook,
    type HookContext,
    type HookHints,
    type HookResult,
    type EventEnvelope,
} from '@dejanradmanovic/event-spec-api';

class ConsentHook extends UnimplementedHook {
    constructor(private consentStore: ConsentStore) {
        super();
    }

    async before(hc: HookContext, hints?: HookHints): Promise<EventEnvelope | null> {
        if (!hc.context?.userId) return null; // anonymous users pass through
        const hasConsent = await this.consentStore.hasConsent(hc.context.userId, 'analytics');
        if (!hasConsent) throw new Error('no analytics consent'); // throw = drop
        return null; // pass through unchanged
    }

    finally(hc: HookContext, result: HookResult, hints?: HookHints): void {
        console.log(`event=${hc.eventName} delivered=${result.delivered}`);
    }
}

const client = new Client({
    providers: [amp],
    hooks: [new ConsentHook(store)],
});
```

</TabItem>
<TabItem value="kotlin" label="Kotlin">

```kotlin
import io.eventspec.analytics.*

class ConsentHook(private val consentStore: ConsentStore) : UnimplementedHook() {

    // before runs once before any provider receives the event.
    // Throw to cancel dispatch (event is Dropped).
    override suspend fun before(hc: HookContext, hints: HookHints): EventEnvelope? {
        if (hc.context.userId.isEmpty()) return null // anonymous users always pass
        if (!consentStore.hasConsent(hc.context.userId, "analytics")) {
            throw Exception("no analytics consent") // throw = drop
        }
        return null // pass through unchanged
    }

    // finally runs after every provider result (success or failure).
    override fun finally(hc: HookContext, result: HookResult, hints: HookHints) {
        println("event=${hc.eventName} delivered=${result.delivered}")
    }
}

val client = Client(ClientOptions(
    providers = listOf(amp),
    hooks = listOf(ConsentHook(store)),
))
```

</TabItem>
</Tabs>

## Mutating the envelope

Hooks can add, remove, or modify properties in the `EventEnvelope.Properties` map. Changes affect what all providers receive:

<Tabs>
<TabItem value="go" label="Go">

```go
func (h *EnrichmentHook) Before(ctx context.Context, hc hooks.HookContext, hints hooks.HookHints) (*hooks.EventEnvelope, error) {
    hc.Envelope.Properties["server_timestamp"] = time.Now().UTC().Unix()
    hc.Envelope.Properties["region"] = h.region
    // Remove PII
    delete(hc.Envelope.Properties, "email")
    return hc.Envelope, nil
}
```

</TabItem>
<TabItem value="ts" label="TypeScript">

```typescript
async before(hc: HookContext, hints?: HookHints): Promise<EventEnvelope | null> {
    const msg = hc.message as EventEnvelope;
    const { email, ...rest } = msg.properties; // strip PII
    return {
        ...msg,
        properties: {
            ...rest,
            server_timestamp: Date.now(),
            region: this.region,
        },
    };
}
```

</TabItem>
<TabItem value="kotlin" label="Kotlin">

```kotlin
override suspend fun before(hc: HookContext, hints: HookHints): EventEnvelope? {
    val env = hc.message as? EventEnvelope ?: return null
    val stripped = env.properties.filterKeys { it != "email" } // strip PII
    return env.copy(properties = stripped + mapOf(
        "server_timestamp" to System.currentTimeMillis(),
        "region" to region,
    ))
}
```

</TabItem>
</Tabs>

## Hook registration scopes

| Scope | How | Runs for |
|-------|-----|---------|
| Global | `analytics.AddGlobalHook(h)` | All clients |
| Client | `analytics.WithHooks(h)` | This client only |
| Provider | `Provider.Hooks()` return value | This provider's events only |

Execution order within a scope follows registration order. Between scopes: API → Client → Provider in `Before`; reversed in `After`/`Error`/`Finally`.
