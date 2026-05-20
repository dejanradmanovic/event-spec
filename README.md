# event-spec

> **Alpha** — core runtime and codegen are functional; several planned features are not yet implemented. See [Status](#status) for what is and isn't done.

Provider-agnostic analytics abstraction platform. Define your events once in YAML, generate type-safe wrappers for every language, and swap analytics vendors without touching application code.

---

## What it does

Teams couple instrumentation directly to a single vendor (Amplitude, Mixpanel, GA4). Swapping providers, running A/B tests across platforms, or enforcing schema consistency across a polyglot codebase requires large refactors. event-spec addresses this with four layers:

1. **Event Contract Layer** — YAML event specs with versioning, schema validation, and breaking-change detection
2. **SDK Runtime Layer** — pluggable analytics destinations behind a stable `Provider` interface, hook lifecycle, context propagation, queueing, and dispatch
3. **Codegen Layer** — reads the event registry and generates language-native typed wrappers
4. **Governance / Operations Layer** — registry server, audit tooling, and docs generation *(planned)*

---

## Status

| Component | State |
|---|---|
| `spec/` — YAML loader, schema structs, JSON Schema validation | ✅ Done |
| `analytics/` — Client, global API, 4-level context chain, dispatch | ✅ Done |
| `provider/` — interface, transport (retry + proxy), queue, rate limiter | ✅ Done |
| `provider/amplitude` (Go) | ✅ Done |
| `provider/noop` (Go) | ✅ Done |
| `hooks/` — Hook interface, chain executor, sampling, validation | ✅ Done |
| `registry/local` — filesystem walker, in-memory index, fsnotify hot-reload | ✅ Done |
| `codegen/` — Go and TypeScript engines, `text/template`, golden tests | ✅ Done |
| `testutil/` — CaptureProvider, MockProvider | ✅ Done |
| `cmd/event-spec generate` | ✅ Done |
| `cmd/event-spec validate` | ✅ Done |
| `sdk/typescript` — `@event-spec/analytics` runtime | ✅ Done |
| `sdk/typescript` — `@event-spec/provider-amplitude` | ✅ Done |
| `spec/diff` — breaking-change computation | 🚧 Types defined; logic not yet wired |
| `cmd/event-spec diff` | ❌ Not yet |
| `registry/git` — remote-repo pull and cache | ❌ Not yet |
| `registry/server` — REST API backend | ❌ Not yet |
| `hooks/logging`, `hooks/otel` | ❌ Not yet |
| `codegen/audit` — AST-based event usage scanning | ❌ Not yet |
| `cmd/event-spec audit`, `pull`, `docs`, `serve` | ❌ Not yet |
| Go providers: PostHog, Mixpanel, Segment, GA4, RudderStack | ❌ Not yet |
| Swift, Kotlin, Python, Rust, Dart, .NET SDKs | ❌ Not yet |

---

## Quick start

### 1. Write an event spec

```yaml
# specs/ecommerce/product_viewed/1-0-0.yaml
$schema: "https://event-spec.io/schemas/event/v1"
name: product_viewed
display_name: "Product Viewed"
version: "1-0-0"
status: active
namespace: ecommerce
type: track
event_name: "Product Viewed"

properties:
  product_id:
    type: string
    required: true
  category:
    type: string
    required: true
    enum: [ clothing, electronics, other ]
  currency:
    type: string
    required: false
    default: "USD"
```

### 2. Generate typed wrappers

```bash
# Go
event-spec generate --lang go --out ./generated

# TypeScript
event-spec generate --lang typescript --out ./src/analytics/generated
```

Or use a source config (`sources/web-app.yaml`) to attach language, output path, and event filters — then just run `event-spec generate web-app`.

### 3. Use the generated wrapper

**Go**

```go
import (
    "context"
    core "github.com/dejanradmanovic/event-spec/analytics"
    "github.com/dejanradmanovic/event-spec/provider/amplitude"
    generated "your-module/generated"
)

amp, _ := amplitude.New(amplitude.Config{
    ProviderConfig: core.ProviderConfig{
        APIKey:     "${AMPLITUDE_API_KEY}",
        SecretType: provider.SecretEnvVar,
    },
})

client := core.NewClient(core.WithProviders(amp))
es := generated.New(client)

es.ProductViewed(context.Background(), generated.ProductViewedProperties{
    Category:  generated.ProductViewedCategoryElectronics,
    ProductId: "SKU-123",
})
```

**TypeScript**

```typescript
import { Client } from '@event-spec/analytics';
import { AmplitudeProvider } from '@event-spec/provider-amplitude';
import { productViewed, ProductViewedCategory } from './generated/product_viewed';

const amp = new AmplitudeProvider({ apiKey: process.env.AMPLITUDE_API_KEY! });
const client = new Client({ providers: [amp] });

client.productViewed({
    category: ProductViewedCategory.Electronics,  // or the union literal "electronics"
    productId: 'SKU-123',
});
```

### 4. Identify a user

`identify` ships user traits to every configured provider. Call it after sign-up, login, or any time user traits change.

Every `IdentifyMessage` carries two distinct payloads — understanding the difference matters for how you set things up:

| Field | What it is | Where it comes from |
|---|---|---|
| `traits` | Who the user IS — email, name, plan, custom properties | The `traits` argument you pass directly |
| `context` | The environment the call was made FROM — device, locale, user agent, IP | `AnalyticsContext.attributes` from the context chain |

Traits and context are never mixed. `buildMessageContext` is called internally on every dispatch (track, identify, group, page, alias) to promote well-known attributes keys (`user_agent`, `locale`, `ip_address`, `app`, `device`, `os`, etc.) into typed `MessageContext` fields; unknown keys land in `extra` and still flow to the provider.

**Setting environment metadata once**

Set device/locale/app metadata at init time via the global or client-level context. It attaches automatically to every call without being part of traits:

**Go**
```go
analytics.SetGlobalContext(analytics.AnalyticsContext{
    Attributes: map[string]any{
        "locale":     "en-US",
        "user_agent": r.UserAgent(),
        "app":        map[string]any{"name": "my-app", "version": "2.1.0"},
    },
})
```

**TypeScript**
```typescript
setGlobalContext({
    attributes: {
        locale: navigator.language,
        user_agent: navigator.userAgent,
        app: { name: 'web-app', version: APP_VERSION },
    },
});
```

**Calling identify**

**Go**
```go
err := client.Identify(ctx, "user-123", map[string]any{
    "email":      "alice@example.com",
    "name":       "Alice",
    "plan":       "pro",
    "created_at": "2026-01-15T10:00:00Z",
    "company":    "Acme Corp",
})
```

**TypeScript**
```typescript
await client.identify('user-123', {
    email: 'alice@example.com',
    name: 'Alice',
    plan: 'pro',
    createdAt: '2026-01-15T10:00:00Z',
    company: 'Acme Corp',
});
```

The provider receives both buckets separately — traits describe the user, context describes the environment:

```
IdentifyMessage {
    userId:      "user-123"
    anonymousId: "anon-abc"          ← always from the context chain, never passed directly
    traits: {                        ← your traits argument (after hooks)
        email:   "alice@example.com"
        plan:    "pro"
    }
    context: {                       ← built from AnalyticsContext.attributes
        locale:    "en-US"
        userAgent: "Mozilla/5.0 ..."
        app:       { name: "web-app", version: "2.1.0" }
    }
}
```

**Identity resolution**

The `userID` positional argument is the canonical identity, but the merged context chain takes precedence: if a non-empty `UserID` is already present (e.g. injected by `AnalyticsMiddleware` in Go or `withTransaction` in TypeScript), it wins. This means you can call `identify` from inside a middleware-wrapped handler without re-extracting the user ID:

**Go**
```go
// AnalyticsMiddleware has already set UserID in ctx — pass "" to let context win
client.Identify(ctx, "", map[string]any{"plan": "enterprise"})
```

**TypeScript**
```typescript
// withTransaction has already bound userId — pass '' to let context win
const reqClient = client.withTransaction({ userId: req.user.id });
await reqClient.identify('', { plan: 'enterprise' });
```

Traits flow through the full hook chain (Before → providers → After/Error/Finally), so validation, sampling, and custom hooks apply exactly as they do for track events.

---

## Event spec YAML

### Versioning: SchemaVer (`MAJOR-MINOR-PATCH`)

Borrowed from Snowplow Iglu. Hyphens visually distinguish event versions from SemVer (which governs CLI/SDK releases).

| Change | Bump |
|---|---|
| Add required property | **MAJOR** |
| Remove any property | **MAJOR** |
| Rename property | **MAJOR** |
| Change property type | **MAJOR** |
| Make optional → required | **MAJOR** |
| Remove enum value | **MAJOR** |
| Make required → optional | MINOR |
| Add optional property | MINOR |
| Add enum value | MINOR |
| Description-only change | PATCH |

### Property types

`string`, `number`, `integer`, `boolean`, `object`, `array`

Constraints: `enum`, `pattern`, `minimum`, `maximum`, `default`, `aliases`

### Workspace config (`event-spec.yaml`)

Currently only `mode: local` is implemented. `git` and `server` modes are planned.

```yaml
version: 1
workspace: "my-company"
registry:
  mode: local
specs_dir: ./specs
sources_dir: ./sources
destinations_dir: ./destinations
```

### Source config (`sources/web-app.yaml`)

```yaml
name: web-app
platform: web
language: typescript
events:
  - ecommerce/**
  - auth/user_signed_up
destinations:
  - amplitude
output:
  path: ./src/analytics/generated
  package: "@my-company/analytics"
```

---

## Provider interface (Go)

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

Providers that don't support an operation return `ErrUnsupportedOperation` rather than silently no-op — preventing silent data loss. Amplitude's Go and TypeScript providers are the only ones currently implemented; PostHog, Mixpanel, Segment, GA4, and RudderStack are planned.

### Provider configuration

```go
amp, err := amplitude.New(amplitude.Config{
    ProviderConfig: provider.ProviderConfig{
        APIKey:     "${AMPLITUDE_API_KEY}",
        SecretType: provider.SecretEnvVar,

        // Proxy through your domain to bypass ad-blockers
        ProxyURL:  "https://analytics.yourcompany.com/amp",
        ProxyMode: provider.ProxyReverseProxy,

        BatchSize:     100,
        FlushInterval: 5 * time.Second,
        MaxQueueSize:  10_000,
        OverflowPolicy: provider.OverflowDropOldest,

        RetryConfig: provider.RetryConfig{
            MaxRetries:     3,
            InitialBackoff: 100 * time.Millisecond,
            MaxBackoff:     30 * time.Second,
            Multiplier:     2.0,
            Jitter:         true,
        },
        RateLimitConfig: provider.RateLimitConfig{
            RequestsPerSecond: 30,
        },
    },
})
```

---

## Hook interface

Hooks are the middleware layer for the event pipeline. Governance-first ordering means consent, PII stripping, sampling, and validation hooks run before any provider-specific adapters.

```
Before:              api-hooks → client-hooks → provider-hooks
After/Error/Finally: reverse order (provider → client → api)
```

`Before` runs once and gates all providers. `After`/`Error`/`Finally` fire once per provider result.

```go
type Hook interface {
    Before(ctx context.Context, hc HookContext, hints HookHints) (*EventEnvelope, error)
    After(ctx context.Context, hc HookContext, result HookResult, hints HookHints) error
    Error(ctx context.Context, hc HookContext, err error, hints HookHints)
    Finally(ctx context.Context, hc HookContext, result HookResult, hints HookHints)
}
```

Embed `hooks.UnimplementedHook` and override only the stages you need.

### Built-in hooks

| Hook | Package | State |
|---|---|---|
| Schema validation | `hooks/validation` | ✅ Done |
| Deterministic / random sampling | `hooks/sampling` | ✅ Done |
| Structured logging | `hooks/logging` | ❌ Planned |
| OpenTelemetry spans | `hooks/otel` | ❌ Planned |

### Validation hook example

```go
lookup := func(name string) (*spec.EventDef, bool) {
    def, err := registry.GetEvent(ctx, namespace, name, "")
    if err != nil {
        return nil, false
    }
    return def, true
}

client := analytics.NewClient(
    analytics.WithProviders(amp),
    analytics.WithHooks(validation.New(lookup)),
)
```

---

## Context propagation (4-level chain)

```
Priority (highest overrides lowest):

  4. Invocation  → TrackOption WithContextOverride(...)
  3. Client      → NewClient(WithContext(...)) or c.SetContext(...)
  2. Transaction → WithTransaction(txCtx) stored in context.Context
  1. Global      → analytics.SetGlobalContext(...)
```

Non-empty override fields win at each level; `Attributes` are merged key-by-key.

### HTTP middleware

```go
func AnalyticsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        txCtx := analytics.TransactionContext{
            UserID:      extractUserID(r),
            AnonymousID: extractSessionID(r),
            Attributes: map[string]any{
                "request_id": r.Header.Get("X-Request-ID"),
                "user_agent": r.UserAgent(),
            },
        }
        ctx := analytics.WithAnalyticsContext(r.Context(), txCtx)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

## Dispatch and delivery states

`Track` returns a non-nil error only for pre-dispatch failures (hook cancelled, schema invalid). Per-provider outcomes are accessible via `TrackDetailed`:

```go
result, err := client.TrackDetailed(ctx, event)
// result.Success  — providers that succeeded
// result.Failed   — providers that failed (permanent)
// result.PartialSuccess — at least one provider succeeded
```

| State | Meaning |
|---|---|
| `Delivered` | Provider confirmed receipt |
| `Failed` | Provider permanently rejected after max retries |
| `Dropped` | Discarded by sampling, consent filter, queue overflow, or schema violation |

---

## CLI

```
event-spec generate [source]    generate typed wrappers (Go, TypeScript)
  --lang   go | typescript
  --out    output directory

event-spec validate [spec-dir]  validate specs, sources, destinations, workspace config
  --strict fail on warnings
```

Planned but not yet implemented: `diff`, `pull`, `docs`, `audit`, `serve`.

---

## Codegen

The engine uses `text/template` (not a Go-specific AST generator) so it works across all target languages. Language-specific concerns live entirely inside `.tmpl` files.

Currently generated:

| Language | Runtime package | Codegen |
|---|---|---|
| Go | `github.com/dejanradmanovic/event-spec/analytics` | ✅ |
| TypeScript | `@event-spec/analytics` | ✅ |

Planned: Swift, Kotlin, Python, Java, Rust, Dart, .NET.

### Generated output (Go)

`generated/eventspec.go`:
```go
// Code generated by event-spec. DO NOT EDIT.
package analytics

import core "github.com/dejanradmanovic/event-spec/analytics"

type EventSpec struct { client *core.Client }

func New(client *core.Client) *EventSpec { return &EventSpec{client: client} }
```

`generated/product_viewed.go`:
```go
// Code generated by event-spec. DO NOT EDIT.
// Source: product_viewed (v1-0-0)

type ProductViewedCategory string

const (
    ProductViewedCategoryClothing    ProductViewedCategory = "clothing"
    ProductViewedCategoryElectronics ProductViewedCategory = "electronics"
    ProductViewedCategoryOther       ProductViewedCategory = "other"
)

type ProductViewedProperties struct {
    Category  ProductViewedCategory
    ProductId string
    Currency  *string
}

func (es *EventSpec) ProductViewed(ctx context.Context, props ProductViewedProperties, opts ...core.TrackOption) error {
    return es.client.Track(ctx, core.Event{
        Name: "Product Viewed",
        Properties: map[string]any{
            "category":   string(props.Category),
            "product_id": props.ProductId,
            "currency":   props.Currency,
        },
    }, opts...)
}
```

---

## Testing utilities

`testutil.CaptureProvider` records every provider call for assertion in tests:

```go
cap := testutil.NewCaptureProvider("test")
client := analytics.NewClient(analytics.WithProviders(cap))
// ... call client.Track(...)
// cap.Tracks[0].EventName == "Product Viewed"
```

`testutil.MockProvider` simulates latency and per-operation errors without recording events.

---

## Module

```
module: github.com/dejanradmanovic/event-spec
go:     1.26
```

### TypeScript packages

```
sdk/typescript/packages/api                  @dejanradmanovic/event-spec-api
sdk/typescript/packages/provider-amplitude   @dejanradmanovic/event-spec-provider-amplitude
```

Package manager: pnpm workspaces.

---

## Repository layout (implemented portions)

```
event-spec/
├── spec/          YAML loader, EventDef structs, JSON Schema validation
├── analytics/     Client, global API, 4-level context, dispatch pipeline
├── provider/      Provider interface, transport, queue, rate limiter
│   ├── amplitude/ Amplitude batch API provider
│   └── noop/      No-op provider (testing / default)
├── hooks/         Hook interface, chain executor
│   ├── sampling/  Hash-based + random sampling
│   └── validation/ JSON Schema runtime validation
├── registry/
│   └── local/     Filesystem walker, in-memory index, fsnotify hot-reload
├── codegen/       Template engine
│   ├── golang/    Go engine + text/template templates
│   └── typescript/ TypeScript engine + text/template templates
├── cmd/event-spec/ CLI (generate, validate)
├── sdk/typescript/ Typescript sdk monorepo
└── testutil/      CaptureProvider, MockProvider
```

---

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design including planned phases, the complete component map, event processing pipeline, breaking-change detection rules, multi-provider dispatch, and the server/proxy deployment model.