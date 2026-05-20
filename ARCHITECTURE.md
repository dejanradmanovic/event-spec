# event-spec: Provider-Agnostic Analytics Abstraction Platform

## Context

Teams ship analytics instrumentation tightly coupled to a single vendor (Amplitude, Mixpanel, GA4). Swapping providers,
running A/B tests across platforms, or enforcing event schema consistency across a polyglot codebase requires large
refactors. This system addresses that with four architectural layers:

1. **Event Contract Layer** — YAML event specs with versioning, schema validation, and breaking-change detection
2. **SDK Runtime Layer** — pluggable analytics destinations behind a stable `Provider` interface, hook lifecycle,
   context propagation, queueing, and dispatch. Inspired by OpenFeature's provider and hook model, but analytics calls
   are side-effecting dispatch operations rather than value evaluations: the runtime also owns queueing, delivery
   semantics, batching, and partial-failure behavior.
3. **Codegen Layer** — reads the event registry and generates language-native typed wrappers (analogous to Ampli/Avo)
4. **Governance / Operations Layer** — registry server, audit tooling, docs generation, and an optional runtime
   ingestion gateway for server-proxied deployments (analogous to Segment/RudderStack rather than OpenFeature)

Scope: design consultation — this document is a blueprint for implementation.

---

## Component Map

```
┌──────────────────────────────────────────────────────────────┐
│                     Event Registry                            │
│   ┌─────────────────────┐   ┌──────────────────────────────┐│
│   │ Git-backed YAML      │OR │ Registry Server (opt.)       ││
│   │ (specs/ dir in repo) │   │ REST API + PostgreSQL/SQLite ││
│   └──────────┬──────────┘   └──────────────┬───────────────┘│
└──────────────┼──────────────────────────────┼────────────────┘
               └──────────────┬───────────────┘
                       event-spec CLI
               pull | generate | validate | diff | serve
                               │
              ┌────────────────┴──────────────────┐
              │           Codegen Engine            │
              │  text/template + per-lang templates │
              └────────────────┬──────────────────┘
                               │ generates
          ┌────────────────────┼──────────────────────────┐
          ▼                    ▼           ▼               ▼
    Go wrapper          TS wrapper   Swift wrapper  Kotlin wrapper
          │                    │           │               │
          └────────────────────┴─────┬─────┴───────────────┘
                              Core Runtime (per language)
                      track / identify / group / page / alias
                      hook chain / context merge / validation
                                      │
                         Provider Registry (runtime)
               ┌──────────┬──────────┬─────────┬──────────┐
               │Amplitude │  PostHog │   GA4   │ Mixpanel │ …
               └──────────┴──────────┴─────────┴──────────┘
```

---

## Go Module / Package Structure

```
event-spec/
├── go.mod                         module: event-spec, go 1.26
│
├── spec/
│   ├── schema.go                  EventDef, PropertyDef, SourceDef, DestinationDef
│   ├── loader.go                  YAML → EventSpec structs
│   ├── validator.go               JSON Schema Draft-07 property validation
│   └── diff.go                    Breaking change detection (SchemaVer rules)
│
├── registry/
│   ├── registry.go                Registry interface
│   ├── git/
│   │   ├── resolver.go            File-system walker, in-memory index
│   │   └── watcher.go            fsnotify hot-reload for dev mode
│   └── server/
│       ├── server.go
│       ├── handlers.go
│       ├── db/schema.sql
│       └── client/client.go      HTTP client for the server
│
├── provider/
│   ├── provider.go                Provider interface + all message types
│   ├── config.go                  ProviderConfig, RetryConfig, RateLimitConfig, ProxyConfig
│   ├── transport.go               HTTP client with proxy support, retries, circuit breaker
│   ├── queue.go                   Event buffering, batching, flush strategies
│   ├── ratelimit.go               Token bucket per-provider rate limiter
│   ├── noop/provider.go           No-op (testing / default)
│   ├── amplitude/
│   │   ├── provider.go
│   │   ├── config.go             API key, endpoint, batch size, proxy settings
│   │   └── mapper.go             Property type coercion
│   ├── posthog/provider.go
│   ├── mixpanel/provider.go
│   ├── segment/provider.go
│   ├── ga4/provider.go
│   └── rudderstack/provider.go
│
├── hooks/
│   ├── hook.go                    Hook interface, HookContext, HookHints
│   ├── unimplemented.go          UnimplementedHook embed helper
│   ├── logging/hook.go            Structured logging with trace IDs
│   ├── validation/hook.go         Runtime JSON Schema validation
│   ├── sampling/hook.go           Deterministic hash-based + random sampling
│   └── otel/hook.go              OpenTelemetry spans + trace context propagation
│
├── analytics/
│   ├── client.go                  Client struct — primary API surface
│   ├── api.go                     Global singleton: SetProvider, Track, etc.
│   ├── context.go                 AnalyticsContext, Merge, 4-level precedence chain
│   ├── transaction.go             TransactionContext (per-request scope)
│   ├── event.go                   Event, Properties types
│   ├── options.go                 ClientOption, TrackOption
│   ├── dispatch.go                DispatchResult, multi-provider error aggregation
│   └── middleware.go              HTTP middleware for transaction context injection
│
├── codegen/
│   ├── engine.go                  Template orchestration
│   ├── model.go                   TemplateData, EventTemplateData, PropTemplateData
│   ├── namer.go                   Per-language naming strategies
│   ├── languages.go               Language registry and per-lang config
│   ├── validator.go               Type safety: JSON Schema → language type constraints
│   ├── audit/
│   │   ├── scanner.go            AST-based code scanner for event usage detection
│   │   ├── coverage.go           Event coverage report generation
│   │   └── matchers.go           Per-language pattern matchers
│   ├── templates/
│   │   ├── go/
│   │   │   ├── event.go.tmpl         Per-event file (properties type, enum consts, method on *EventSpec)
│   │   │   └── eventspec.go.tmpl     EventSpec struct declaration + New() constructor
│   │   ├── typescript/
│   │   │   ├── event.ts.tmpl         Per-event file (interface, enum union type, method)
│   │   │   └── index.ts.tmpl         Re-exports + EventSpec class stub
│   │   ├── swift/event.swift.tmpl
│   │   ├── kotlin/event.kt.tmpl
│   │   ├── python/event.py.tmpl
│   │   ├── java/event.java.tmpl
│   │   ├── rust/event.rs.tmpl
│   │   ├── dart/event.dart.tmpl
│   │   └── dotnet/event.cs.tmpl
│   └── testdata/
│       └── golden/                Golden files for snapshot testing
│
├── cmd/event-spec/
│   ├── main.go
│   ├── pull.go                    event-spec pull
│   ├── generate.go                event-spec generate
│   ├── validate.go                event-spec validate
│   ├── diff.go                    event-spec diff
│   ├── docs.go                    event-spec docs (generate HTML/Markdown catalog)
│   ├── audit.go                   event-spec audit (verify event usage in codebase)
│   └── serve.go                   event-spec serve (registry server)
│
├── sdk/                           Per-language core runtimes (hand-written)
│   ├── typescript/                @event-spec/analytics (npm)
│   ├── swift/                     EventSpecAnalytics (Swift Package)
│   ├── kotlin/                    io.event-spec:analytics-kotlin (Maven)
│   ├── python/                    event_spec (PyPI)
│   ├── rust/                      event-spec-analytics (crates.io)
│   ├── dart/                      event_spec_analytics (pub.dev)
│   └── dotnet/                    EventSpec.Analytics (NuGet)
│
└── testutil/
    ├── mock_provider.go
    └── capture_provider.go        Records events for assertion in tests
```

---

## Event Spec YAML Format

### Versioning: SchemaVer (`MAJOR-MINOR-PATCH` with hyphens)

Borrowed from Snowplow Iglu. Hyphens distinguish event versions from SemVer (which applies to CLI/SDK releases).

| Change                   | Version bump |
|--------------------------|--------------|
| Add required property    | **MAJOR**    |
| Remove any property      | **MAJOR**    |
| Rename property          | **MAJOR**    |
| Change property type     | **MAJOR**    |
| Make optional → required | **MAJOR**    |
| Remove enum value        | **MAJOR**    |
| Rename event             | **MAJOR**    |
| Make required → optional | MINOR        |
| Add optional property    | MINOR/PATCH  |
| Add enum value           | MINOR        |
| Description-only change  | PATCH        |

`event-spec diff` enforces that the declared version is consistent with detected changes, failing CI on violations.

### Example Event Spec (`specs/ecommerce/product_viewed/1-2-0.yaml`)

```yaml
$schema: "https://event-spec.io/schemas/event/v1"

name: product_viewed
display_name: "Product Viewed"
description: |
  Fired when a user views the detail page of a product.
version: "1-2-0"
changelog: "Added optional coupon_code property"
status: active          # draft | active | deprecated | deleted
namespace: ecommerce
tags: [ product, ecommerce, funnel ]
owner: "growth-team@example.com"
type: track             # track | page | identify | group | alias

event_name: "Product Viewed"   # canonical name sent to providers

properties:
  product_id:
    type: string
    required: true
    description: "The SKU or database ID of the product"
  product_name:
    type: string
    required: true
  category:
    type: string
    required: true
    enum: [ clothing, electronics, books, home, sports, other ]
  price:
    type: number
    required: true
    minimum: 0
  currency:
    type: string
    required: false
    pattern: "^[A-Z]{3}$"
    default: "USD"
  coupon_code:
    type: string
    required: false

# Context properties injected from AnalyticsContext at track time
context_properties:
  - user_id
  - session_id
  - platform

# Per-provider event/property name overrides (e.g. GA4 snake_case)
provider_overrides:
  ga4:
    event_name: "view_item"
    property_map:
      product_id: item_id
      product_name: item_name
      category: item_category

# Leave empty to send to all active providers
destinations: [ ]

# Sampling: declares a default policy for this event.
# Sources can override via sampling_overrides in source config (e.g. backend at 100%, mobile at 25%).
# user_id_hash: deterministic per user; random: probabilistic A/B-style
sampling:
  strategy: user_id_hash  # user_id_hash | random | none
  rate: 0.1               # 10% sample rate

# Property collision resolution: context properties vs event properties.
# This is a default; sources can override via property_priority_override in source config.
# Options: event_wins | context_wins | merge
property_priority: event_wins
```

### Git Registry Directory Layout

```
my-tracking-plan/
├── event-spec.yaml          Workspace config
├── specs/
│   ├── _shared/             Reusable $ref fragments
│   │   └── product.yaml
│   ├── ecommerce/
│   │   ├── product_viewed/
│   │   │   ├── 1-0-0.yaml
│   │   │   └── 1-2-0.yaml   (latest active)
│   │   └── order_completed/
│   │       └── 2-0-0.yaml
│   └── auth/
│       └── user_signed_up/
│           └── 1-0-0.yaml
├── sources/
│   ├── web-app.yaml
│   └── ios-app.yaml
└── destinations/
    ├── amplitude.yaml
    └── posthog.yaml
```

### Workspace Config (`event-spec.yaml`)

```yaml
version: 1
workspace: "my-company"
registry:
  mode: git          # git | server
  # url: https://registry.example.com   (server mode)
specs_dir: ./specs
sources_dir: ./sources
destinations_dir: ./destinations
```

### Source Definition (`sources/web-app.yaml`)

```yaml
name: web-app
platform: web
language: typescript
events:
  - ecommerce/**
  - auth/user_signed_up
destinations:
  - amplitude
  - posthog
output:
  path: ./src/analytics/generated
  package: "@my-company/analytics"

# Optional: specify active event versions for migration scenarios
version_pinning:
  ecommerce/product_viewed: "1-2-0"  # use specific version
  # others use latest active
```

---

## Provider Interface (Go Reference)

```go
// package provider

type Provider interface {
Metadata() ProviderMetadata
Hooks() []hooks.Hook // provider-level hooks prepended to chain

Track(ctx context.Context, event TrackMessage) error
Identify(ctx context.Context, msg IdentifyMessage) error
Group(ctx context.Context, msg GroupMessage) error
Page(ctx context.Context, msg PageMessage) error
Alias(ctx context.Context, msg AliasMessage) error

Flush(ctx context.Context) error
Shutdown(ctx context.Context) error
}

type ProviderMetadata struct {
Name         string               // "amplitude", "posthog"
Version      string               // semver of this provider implementation
Capabilities ProviderCapabilities // advertises which analytics operations this provider natively supports
}

// ProviderCapabilities prevents silent data loss. Unsupported methods return ErrUnsupportedOperation
// instead of silently no-op-ing, so callers know the event was not delivered.
type ProviderCapabilities struct {
Track    bool
Identify bool
Group    bool // not all vendors have first-class group support
Page     bool // web-centric; mobile and backend providers often omit this
Alias    bool // identity-merge semantics differ heavily between vendors
}

// ErrUnsupportedOperation is returned by provider methods that have no equivalent in the underlying vendor API.
var ErrUnsupportedOperation = errors.New("unsupported provider operation")

// TrackMessage is the fully-resolved payload — context merged, hooks applied.
type TrackMessage struct {
MessageID      string
Timestamp      time.Time
EventName      string           // post provider_overrides mapping
Properties     map[string]any
UserID         string
AnonymousID    string
MessageContext MessageContext
}

// ProviderConfig: initialization settings for providers
type ProviderConfig struct {
// Secret management: API keys, tokens
APIKey     string
SecretType SecretType // env_var | file | vault | inline

// Proxy settings (bypass ad-blockers)
ProxyURL    string // e.g., https://analytics.yourcompany.com/amp
ProxyMode   ProxyMode // direct | reverse_proxy | custom

// Batching & queue settings
BatchSize       int           // max events per batch (default: provider-specific)
FlushInterval   time.Duration // auto-flush interval (default: 10s)
MaxQueueSize    int           // buffered events limit (default: 10000)
OverflowPolicy  OverflowPolicy // drop_oldest | drop_newest | block

// Retry & backoff
RetryConfig RetryConfig

// Rate limiting (per-provider)
RateLimitConfig RateLimitConfig

// HTTP transport overrides
Timeout         time.Duration
MaxIdleConns    int
TLSConfig       *tls.Config
}

type SecretType string
const (
SecretEnvVar SecretType = "env_var"  // read from environment
SecretFile   SecretType = "file"     // read from file path
SecretVault  SecretType = "vault"    // fetch from HashiCorp Vault, AWS Secrets Manager, etc.
SecretInline SecretType = "inline"   // plaintext in config (dev only)
)

type ProxyMode string
const (
ProxyDirect       ProxyMode = "direct"         // no proxy
ProxyReverseProxy ProxyMode = "reverse_proxy"  // send to your domain, proxy to provider
ProxyCustom       ProxyMode = "custom"         // custom URL rewrite
)

type OverflowPolicy string
const (
OverflowDropOldest OverflowPolicy = "drop_oldest"
OverflowDropNewest OverflowPolicy = "drop_newest"
OverflowBlock      OverflowPolicy = "block"  // backpressure
)

type RetryConfig struct {
MaxRetries      int           // max retry attempts (default: 3)
InitialBackoff  time.Duration // first retry delay (default: 100ms)
MaxBackoff      time.Duration // cap on exponential backoff (default: 30s)
Multiplier      float64       // backoff multiplier (default: 2.0)
Jitter          bool          // add random jitter to backoff (default: true)
RetryableErrors []int         // HTTP status codes to retry (default: 429, 500, 502, 503, 504)
}

type RateLimitConfig struct {
RequestsPerSecond int    // token bucket rate (default: provider-specific)
BurstSize         int    // max burst tokens (default: RequestsPerSecond * 2)
}
```

All five call types (`Track`, `Identify`, `Group`, `Page`, `Alias`) follow the same pattern. The single interface is
intentional — providers that don't support `Page` implement a no-op. This avoids runtime interface-checks in the
dispatch engine.

### Provider Initialization Example

```go
// Amplitude with proxy + secret from env
amplitude, err := amplitude.New(provider.ProviderConfig{
APIKey:     "${AMPLITUDE_API_KEY}",  // resolved from env
SecretType: provider.SecretEnvVar,
ProxyURL:   "https://analytics.mycompany.com/amp",
ProxyMode:  provider.ProxyReverseProxy,
BatchSize:  100,
FlushInterval: 5 * time.Second,
RetryConfig: provider.RetryConfig{
MaxRetries:     5,
InitialBackoff: 200 * time.Millisecond,
},
RateLimitConfig: provider.RateLimitConfig{
RequestsPerSecond: 30,  // Amplitude limit
},
})
```

---

## Client API (Go)

```go
// package analytics

// Global API (package-level functions)
func SetGlobalProvider(p ...provider.Provider) error
func AddGlobalProvider(p provider.Provider) error
func SetGlobalContext(ctx AnalyticsContext)
func AddGlobalHooks(h ...hooks.Hook)
func NewClient(opts ...ClientOption) *Client
func Shutdown(ctx context.Context) error

// Client methods
func (c *Client) Track(ctx context.Context, event Event, opts ...TrackOption) error                              // enqueues/dispatches; error only on pre-dispatch rejection
func (c *Client) TrackDetailed(ctx context.Context, event Event, opts ...TrackOption) (DispatchResult, error)    // full per-provider outcomes for partial-failure handling
func (c *Client) Identify(ctx context.Context, userID string, traits map[string]any, opts ...TrackOption) error
func (c *Client) Group(ctx context.Context, groupID string, traits map[string]any, opts ...TrackOption) error
func (c *Client) Page(ctx context.Context, name string, props map[string]any, opts ...TrackOption) error
func (c *Client) Alias(ctx context.Context, userID, previousID string, opts ...TrackOption) error
func (c *Client) SetContext(ctx AnalyticsContext)
func (c *Client) WithTransaction(txCtx TransactionContext) *Client
func (c *Client) Flush(ctx context.Context) error
```

---

## Hook Interface

```go
// package hooks

// HookContext carries the event being processed through the hook chain.
type HookContext struct {
Operation string           // "track" | "identify" | "group" | "page" | "alias"
EventName string           // canonical event name from spec
Context   AnalyticsContext      // merged analytics context at this hook stage
Message   any              // outbound message type (TrackMessage, IdentifyMessage, etc.)
Provider  string           // set only in After/Error/Finally; empty in Before
}

// EventEnvelope is the mutable event representation passed through Before hooks.
// Return a non-nil *EventEnvelope from Before to replace the event for subsequent hooks and providers.
type EventEnvelope struct {
EventName  string
Properties map[string]any
Context    AnalyticsContext
Metadata   map[string]any // hook-private: routing hints, consent flags, enrichment data
}

type Hook interface {
// Before: runs once before dispatch. Return a modified *EventEnvelope to replace the event
// for subsequent hooks and providers. Return error to cancel the event entirely.
// Use this stage for: PII removal, campaign enrichment, consent filtering, normalization.
Before(ctx context.Context, hc HookContext, hints HookHints) (*EventEnvelope, error)

// After: called per-provider on success.
After(ctx context.Context, hc HookContext, result HookResult, hints HookHints) error

// Error: called per-provider on failure. Must not itself return error.
Error(ctx context.Context, hc HookContext, err error, hints HookHints)

// Finally: always called after After or Error (defer semantics).
Finally(ctx context.Context, hc HookContext, result HookResult, hints HookHints)
}

// UnimplementedHook: embed in your hook to only override the stages you need.
type UnimplementedHook struct{}
```

Hook ordering is governance-first. Unlike OpenFeature (where provider hooks run first), analytics requires global
governance hooks — consent, PII stripping, schema validation, sampling — to run before provider-specific adapters
so they apply universally regardless of which providers are configured.

```
Before:           api-hooks → client-hooks → invocation-hooks → provider-hooks
After/Error/Finally:  reverse order (provider → invocation → client → api)
```

`Before` runs once, gating all providers. `After`/`Error`/`Finally` fire once per provider result, enabling per-provider
observability.

---

## Event Processing Pipeline

The exact sequence from a generated wrapper call to provider delivery:

```
1. Generated wrapper call    →  es.ProductViewed(ctx, props)
2. Build canonical Event     →  {name: "Product Viewed", properties: {...}}
3. Merge AnalyticsContext    →  global → transaction → client → invocation (highest wins)
4. Run Before hooks          →  api-hooks → client-hooks → invocation-hooks → provider-hooks
                                 (any hook may mutate EventEnvelope or cancel by returning error)
5. Validate canonical schema →  JSON Schema check against event spec (if validation hook active)
6. Apply sampling/consent    →  hash-based or random; event dropped here if sampled out
7. Select destinations       →  event spec destinations[] filtered by source config
8. For each provider (concurrent):
   a. Apply provider_overrides mapping   →  rename properties, remap event name (e.g. GA4)
   b. Apply provider-specific coercion   →  type coercion, size limits (e.g. Amplitude 1024-char)
   c. Run provider Before hook           →  adapter-level finalization
   d. Enqueue or send                    →  per-provider queue or direct HTTP
   e. Record result                      →  Accepted | Delivered | Failed | Dropped
   f. Run provider After/Error/Finally   →  per-provider observability hooks
9. Aggregate results         →  DispatchResult{Success, Failed, PartialSuccess}
10. Return to caller         →  error (Track) or DispatchResult (TrackDetailed)
```

**Delivery states**:

| State       | Meaning                                                                      |
|-------------|------------------------------------------------------------------------------|
| `Accepted`  | Event passed validation and entered the provider queue (async/batched)       |
| `Delivered` | Provider confirmed receipt over the network (sync providers or after flush)  |
| `Failed`    | Provider permanently rejected the event after max retries                    |
| `Dropped`   | Intentionally discarded: sampling, consent filter, queue overflow, or schema violation |

`Track` returns a non-nil error only for pre-dispatch failures (schema invalid, hook cancelled). Post-dispatch failures
per provider are accessible via `TrackDetailed`, which returns the full `DispatchResult`.

---

## Context Propagation (4-Level Chain)

```
Priority (highest overrides lowest):

  4. Invocation   →  TrackOption WithContextOverride(...)
  3. Client       →  NewClient(WithAnalyticsContext(...)) or c.SetContext(...)
  2. Transaction  →  client.WithTransaction(txCtx)  [stored in context.Context]
  1. Global       →  analytics.SetGlobalContext(...)
```

```go
// AnalyticsContext is a value type (not stored in context.Context except for transaction).
type AnalyticsContext struct {
UserID      string
AnonymousID string
Attributes  map[string]any
}

// Merge: non-empty override fields win; Attributes merged key-by-key.
func Merge(base, override AnalyticsContext) AnalyticsContext

// Transaction context is the only level stored in context.Context.
func WithAnalyticsContext(ctx context.Context, tx TransactionContext) context.Context
func TransactionContextFrom(ctx context.Context) (TransactionContext, bool)
```

Dispatch engine merges all four levels before building the provider message, keeping context logic in one place.

### HTTP Middleware Example (Transaction Context)

```go
// Inject transaction-scoped context into incoming HTTP requests
func AnalyticsMiddleware(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// Extract user_id from session, JWT, etc.
userID := extractUserID(r)
sessionID := extractSessionID(r)

txCtx := analytics.TransactionContext{
UserID:      userID,
AnonymousID: sessionID,
Attributes: map[string]any{
"request_id": r.Header.Get("X-Request-ID"),
"user_agent": r.UserAgent(),
"ip_address": r.RemoteAddr,
},
}

ctx := analytics.WithAnalyticsContext(r.Context(), txCtx)
next.ServeHTTP(w, r.WithContext(ctx))
})
}

// Usage in HTTP handler
func handleCheckout(w http.ResponseWriter, r *http.Request) {
// Transaction context automatically available via r.Context()
client.Track(r.Context(), analytics.Event{
Name: "Checkout Started",
Properties: map[string]any{"cart_value": 99.99},
})
}
```

---

## Codegen Architecture

### Design Decision: `text/template` (not Jennifer)

Jennifer is Go-specific. `text/template` works for all 9+ target languages. Template files are editable by language
experts without Go codegen knowledge. Per-language concerns live entirely inside `.tmpl` files; the template data model
is language-agnostic.

### Template Data Model (language-independent intermediate representation)

```go
// package codegen

type TemplateData struct {
Workspace   string
Source      string
GeneratedAt time.Time
Events      []EventTemplateData
Lang        LangConfig
}

type EventTemplateData struct {
NameRaw        string // "product_viewed"
NameDisplay    string // "Product Viewed"
EventName      string // post-override name for this language's providers
Version        string // "1-2-0"
Description    string
MethodName     string // "productViewed" (camelCase) or "ProductViewed" (Go)
ClassName      string    // "ProductViewedEvent"
ParamsTypeName string    // "ProductViewedProperties"
RequiredProps  []PropTemplateData
OptionalProps  []PropTemplateData
}

type PropTemplateData struct {
NameRaw      string // "product_id"
NameField    string // language-adapted: "productId", "product_id", "ProductId"
TypeNative   string // "string", "String", "str"
TypeOptional string // "*string", "string | undefined", "Optional<String>"
Required     bool
Enum         []string
IsEnum       bool
Description  string
}
```

### Namer Strategies

| Language   | Method style | Type style | Field style |
|------------|--------------|------------|-------------|
| Go         | PascalCase   | PascalCase | PascalCase  |
| TypeScript | camelCase    | PascalCase | camelCase   |
| Swift      | camelCase    | PascalCase | camelCase   |
| Kotlin     | camelCase    | PascalCase | camelCase   |
| Python     | snake_case   | PascalCase | snake_case  |
| Java       | camelCase    | PascalCase | camelCase   |
| Rust       | snake_case   | PascalCase | snake_case  |
| Dart       | camelCase    | PascalCase | camelCase   |
| .NET       | PascalCase   | PascalCase | PascalCase  |

### Generated Go Wrapper (example output)

Each event gets its own file. The engine also emits `eventspec.go` with the struct declaration.

`generated/eventspec.go`:
```go
// Code generated by event-spec v0.1.0. DO NOT EDIT.

package analytics

import core "event-spec/analytics"

type EventSpec struct {
	client *core.Client
}

func New(client *core.Client) *EventSpec {
	return &EventSpec{client: client}
}
```

`generated/product_viewed.go`:
```go
// Code generated by event-spec v0.1.0. DO NOT EDIT.
// Source: ecommerce/product_viewed (v1-2-0)

package analytics

import (
	"context"
	core "event-spec/analytics"
)

type ProductViewedCategory string

const (
	ProductViewedCategoryClothing    ProductViewedCategory = "clothing"
	ProductViewedCategoryElectronics ProductViewedCategory = "electronics"
	// ...
)

type ProductViewedProperties struct {
	ProductID   string
	ProductName string
	Category    ProductViewedCategory
	Price       float64
	Currency    *string // optional
	CouponCode  *string // optional, added in v1-2-0
}

func (es *EventSpec) ProductViewed(ctx context.Context, props ProductViewedProperties, opts ...core.TrackOption) error {
	return es.client.Track(ctx, core.Event{
		Name: "Product Viewed",
		Properties: map[string]any{
			"product_id":   props.ProductID,
			"product_name": props.ProductName,
			"category":     string(props.Category),
			"price":        props.Price,
			"currency":     props.Currency,
			"coupon_code":  props.CouponCode,
		},
	}, opts...)
}
```

### Generated TypeScript Wrapper (example output)

Each event gets its own file. `index.ts` re-exports everything and holds the `EventSpec` class.

`generated/product_viewed.ts`:
```typescript
// Code generated by event-spec v0.1.0. DO NOT EDIT.
// Source: ecommerce/product_viewed (v1-2-0)

export type ProductViewedCategory = "clothing" | "electronics" | "books" | "home" | "sports" | "other";

export interface ProductViewedProperties {
    productId: string;
    productName: string;
    category: ProductViewedCategory;
    price: number;
    currency?: string;
    couponCode?: string;
}
```

`generated/index.ts`:
```typescript
// Code generated by event-spec v0.1.0. DO NOT EDIT.

import {Client, TrackOptions} from '@event-spec/analytics';
export * from './product_viewed';
export * from './order_completed';
// ... one export per event file

import {ProductViewedProperties} from './product_viewed';
import {OrderCompletedProperties} from './order_completed';

export class EventSpec {
    constructor(private readonly client: Client) {}

    productViewed(props: ProductViewedProperties, opts?: TrackOptions): Promise<void> {
        return this.client.track({
            name: "Product Viewed",
            properties: {
                product_id: props.productId,
                product_name: props.productName,
                category: props.category,
                price: props.price,
                currency: props.currency,
                coupon_code: props.couponCode,
            }
        }, opts);
    }

    orderCompleted(props: OrderCompletedProperties, opts?: TrackOptions): Promise<void> {
        // ...
    }
}
```

### CLI Commands

```
event-spec init                 scaffold event-spec.yaml + specs/ structure
event-spec pull [source-name]   fetch specs from registry (git or server)
event-spec generate [source]    generate typed SDK wrappers for the source
  --lang    override language
  --out     override output path
event-spec validate [spec-dir]  validate all specs against JSON Schema
  --strict  fail on warnings
event-spec diff <from> <to>     show changes between two spec versions
  --breaking  only show breaking changes
event-spec docs [spec-dir]      generate human-readable event catalog
  --format  html | markdown (default: html)
  --out     output directory
event-spec audit [source]       verify event usage in codebase
  --path    path to scan (default: current directory)
  --strict  fail if any required events unused
  --coverage-min  minimum coverage % required (default: 0)
  --report  json | text | html (default: text)
event-spec serve                start the registry server
  --port    (default 8080)
  --db      database DSN
```

---

## Event Usage Auditing

### Problem Statement

Teams define events in specs but have no automated way to verify:
1. **Required events are actually being sent** in production code
2. **Dead events** defined but never called (spec drift)
3. **Rogue events** sent but not in the spec (bypass type safety)
4. **Coverage metrics** — what % of critical user flows have instrumentation?

### Solution: `event-spec audit`

Scans your codebase and compares actual event usage against the spec registry.

### How It Works

1. **Parse source config** (`sources/web-app.yaml`) to determine:
   - Language (TypeScript, Go, etc.)
   - Events included in this source
   - Codebase path

2. **Scan codebase** using AST parsers:
   - **Go**: `go/parser`, `go/ast` — find `es.ProductViewed(...)` calls
   - **TypeScript**: `@typescript-eslint/parser` — find `es.productViewed(...)` calls
   - **Swift**: SwiftSyntax — find `es.productViewed(...)` calls
   - Match against generated method names from codegen

3. **Generate coverage report**:
   - ✅ Events defined in spec + found in code
   - ⚠️ Events defined in spec but NOT found in code (unused specs)
   - ❌ Events sent but NOT in spec (rogue/untyped events via raw `Track` calls)
   - 📊 Coverage %: (events_used / events_defined) × 100

### Example Output (Text Report)

```
$ event-spec audit web-app --path ./src

Event Coverage Report: web-app
================================================================================
Source:     web-app (TypeScript)
Scanned:    247 files
Events:     45 defined, 38 used (84% coverage)

✅ USED (38 events)
  ecommerce/product_viewed          src/pages/ProductDetail.tsx:42
  ecommerce/product_added_to_cart   src/components/AddToCart.tsx:18
  ecommerce/checkout_started        src/pages/Checkout.tsx:67
  auth/user_signed_up              src/pages/Signup.tsx:93
  ...

⚠️  UNUSED (7 events - defined in spec but not found in code)
  ecommerce/cart_abandoned          Declared in specs/ecommerce/cart_abandoned/1-0-0.yaml
  ecommerce/coupon_applied          Declared in specs/ecommerce/coupon_applied/1-1-0.yaml
  ecommerce/product_shared          Declared in specs/ecommerce/product_shared/1-0-0.yaml
  ...

❌ ROGUE EVENTS (3 - sent but not in spec)
  checkout_error                   src/pages/Checkout.tsx:102 (raw Track call)
  legacy_page_view                 src/utils/analytics.ts:45 (raw Track call)
  ...

📊 COVERAGE BY NAMESPACE
  ecommerce:  32/38 events (84%)
  auth:       5/5 events (100%)
  engagement: 1/2 events (50%)
```

### Example Output (JSON Report)

```json
{
  "source": "web-app",
  "language": "typescript",
  "scanned_files": 247,
  "coverage": {
    "total_events": 45,
    "used_events": 38,
    "unused_events": 7,
    "rogue_events": 3,
    "coverage_pct": 84.4
  },
  "used": [
    {
      "event": "ecommerce/product_viewed",
      "version": "1-2-0",
      "locations": [
        {"file": "src/pages/ProductDetail.tsx", "line": 42}
      ]
    }
  ],
  "unused": [
    {
      "event": "ecommerce/cart_abandoned",
      "version": "1-0-0",
      "spec_file": "specs/ecommerce/cart_abandoned/1-0-0.yaml"
    }
  ],
  "rogue": [
    {
      "event_name": "checkout_error",
      "locations": [
        {"file": "src/pages/Checkout.tsx", "line": 102}
      ]
    }
  ]
}
```

### CI/CD Integration

**Enforce minimum coverage in CI**:
```yaml
# .github/workflows/ci.yml
- name: Verify analytics coverage
  run: event-spec audit web-app --coverage-min 80 --strict
```

Fails if:
- Coverage < 80%
- Any rogue events detected (bypassing type safety)
- `--strict` flag: any required events marked in spec but unused

### Event Spec YAML Extension (Required Events)

```yaml
name: checkout_started
# ... other fields ...
required: true  # Mark critical events that MUST be instrumented
tags: [critical, revenue]
```

Audit fails in `--strict` mode if `required: true` events are unused.

### Detection Strategy Per Language

#### Go
```go
// Detects calls to generated methods
es.ProductViewed(ctx, analytics.ProductViewedProperties{...})
```
**AST matcher**: Look for `SelectorExpr` with `*EventSpec` receiver + method name matching codegen output.

#### TypeScript
```typescript
es.productViewed({productId: "123", ...})
```
**AST matcher**: `CallExpression` with `MemberExpression` where object is an `EventSpec` instance and property matches event method.

#### Swift
```swift
es.productViewed(ProductViewedProperties(...))
```
**AST matcher**: SwiftSyntax `FunctionCallExpr` with receiver typed as `EventSpec`.

### Limitations & Future Work

**Current scope** (Phase 2):
- Detects direct calls to generated event methods
- Does not trace dynamic/runtime event construction
- Requires generated code to follow naming conventions

**Future enhancements** (Phase 4):
- **Control flow analysis**: detect events in conditional branches
- **Dead code elimination**: mark events in unreachable code
- **Runtime telemetry**: compare audit results with actual production event volume
- **Deprecation warnings**: flag usage of deprecated event versions

---

## Registry Design

### Registry Interface

```go
type Registry interface {
ListEvents(ctx context.Context, filter ListFilter) ([]spec.EventDef, error)
GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error)
GetSource(ctx context.Context, name string) (*spec.SourceDef, error)
GetDestination(ctx context.Context, name string) (*spec.DestinationDef, error)
PublishEvent(ctx context.Context, event spec.EventDef) error // ErrReadOnly on git mode
Diff(ctx context.Context, namespace, name, from, to string) ([]spec.Change, error)
}
```

Both git and server implementations satisfy this interface. The CLI doesn't know which mode it's in.

### Git-Backed Registry

- Walk `specs_dir` recursively for `*.yaml`
- Validate `$schema` header, build in-memory index: `namespace/name/version → EventDef`
- `GetEvent` with no version → highest SchemaVer with `status: active`
  - **Version coexistence**: Multiple `active` versions allowed for gradual migration
  - Sources can pin specific versions via `version_pinning` in source config
  - Default behavior: use highest active version
- `fsnotify` watcher for hot-reload in dev mode
- `PublishEvent` returns `ErrReadOnly` — use git commits to publish

### Registry Server (Optional Upgrade)

> **Registry server vs. Runtime ingestion server**: These are distinct modules that may share a binary.
> `registry/server` stores and serves event specs, sources, destinations, and the audit log.
> `runtime/server` (Phase 2) accepts live events from thin clients and dispatches to providers.
> Run them separately: `event-spec serve --mode registry` | `--mode runtime` | `--mode all`.

```
POST /v1/events                              publish a new event version (requires auth)
GET  /v1/events                              list (filterable)
GET  /v1/events/{namespace}/{name}           get latest active
GET  /v1/events/{namespace}/{name}/{version} get specific version
GET  /v1/diff/{namespace}/{name}/{from}/{to} diff two versions
GET  /v1/sources/{name}/pull                 download generated SDK (zip)
GET  /v1/audit                               audit log of spec changes
POST /v1/webhooks                            register webhook for event publish notifications
```

**Authentication & Authorization**

- API key auth via `Authorization: Bearer <token>` header
- Role-based access control (RBAC):
  - `viewer`: read-only access to specs
  - `publisher`: can publish new event versions (POST /v1/events)
  - `admin`: full access including webhook management
- Audit log tracks all spec changes with user attribution

**DB schema (PostgreSQL / SQLite)**

```sql
CREATE TABLE events
(
    id          BIGSERIAL PRIMARY KEY,
    namespace   TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    version     TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'draft',
    spec_yaml   TEXT        NOT NULL,
    json_schema JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    changelog   TEXT,
    UNIQUE (namespace, name, version)
);
CREATE TABLE sources
(
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT UNIQUE,
    spec_yaml  TEXT,
    updated_at TIMESTAMPTZ
);
CREATE TABLE destinations
(
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT UNIQUE,
    spec_yaml  TEXT,
    updated_at TIMESTAMPTZ
);
CREATE TABLE audit_log
(
    id         BIGSERIAL PRIMARY KEY,
    action     TEXT        NOT NULL,  -- 'create' | 'update' | 'delete'
    entity_type TEXT       NOT NULL,  -- 'event' | 'source' | 'destination'
    entity_id  BIGINT      NOT NULL,
    user_id    TEXT        NOT NULL,
    timestamp  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    details    JSONB
);
CREATE TABLE api_keys
(
    id         BIGSERIAL PRIMARY KEY,
    key_hash   TEXT        NOT NULL UNIQUE,
    role       TEXT        NOT NULL,  -- 'viewer' | 'publisher' | 'admin'
    created_by TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);
```

---

## Breaking Change Detection (`spec/diff.go`)

```go
type ChangeKind string

const (
ChangeAddRequiredProp  ChangeKind = "add_required_prop" // MAJOR
ChangeRemoveProp       ChangeKind = "remove_prop" // MAJOR
ChangeRenameProp       ChangeKind = "rename_prop" // MAJOR
ChangeTypeChanged      ChangeKind = "type_changed"  // MAJOR
ChangeMakeRequired     ChangeKind = "make_required" // MAJOR
ChangeRemoveEnumValue  ChangeKind = "remove_enum_value" // MAJOR
ChangeMakeOptional     ChangeKind = "make_optional"     // MINOR
ChangeAddOptionalProp  ChangeKind = "add_optional_prop" // MINOR
ChangeAddEnumValue     ChangeKind = "add_enum_value"    // MINOR
)

type Change struct {
Kind       ChangeKind
Property   string
Breaking   bool
From, To   string
Suggestion string // suggested new SchemaVer
}
```

`event-spec diff` fails if the declared version in the YAML is inconsistent with detected changes — this is the CI
enforcement mechanism.

---

## Multi-Provider Concurrent Dispatch

```go
// DispatchResult: detailed per-provider outcomes
type DispatchResult struct {
Success       []ProviderResult // providers that succeeded
Failed        []ProviderResult // providers that failed
PartialSuccess bool             // true if at least one provider succeeded
}

type ProviderResult struct {
ProviderName string
Error        error
Latency      time.Duration
}

func (a *API) dispatch(ctx context.Context, msg TrackMessage, destFilter []string) DispatchResult {
providers := a.selectProviders(destFilter)
results := make([]ProviderResult, len(providers))
var wg sync.WaitGroup

for i, p := range providers {
wg.Add(1)
go func(i int, p provider.Provider) {
defer wg.Done()
start := time.Now()
err := p.Track(ctx, msg)
results[i] = ProviderResult{
ProviderName: p.Metadata().Name,
Error:        err,
Latency:      time.Since(start),
}
}(i, p)
}
wg.Wait()

// Categorize results
var success, failed []ProviderResult
for _, r := range results {
if r.Error == nil {
success = append(success, r)
} else {
failed = append(failed, r)
}
}

return DispatchResult{
Success:        success,
Failed:         failed,
PartialSuccess: len(success) > 0,
}
}
```

The `Before` hook chain runs once before dispatch. `After`/`Error`/`Finally` fire once per provider result. Providers
must respect `ctx.Done()` — context deadline is the backpressure mechanism.

**Error Handling Strategy**:
- Partial failures return `DispatchResult` with both success and failed lists
- Hooks can inspect per-provider outcomes via `After`/`Error` callbacks
- Client code can decide policy: fail-fast, warn-on-partial, or silent-on-any-success

---

## SDK Package Names Per Language

| Language   | Runtime package                  | Distribution          |
|------------|----------------------------------|-----------------------|
| Go         | `event-spec/analytics`           | Go module proxy       |
| TypeScript | `@event-spec/analytics`          | npm                   |
| Swift      | `EventSpecAnalytics`             | Swift Package Manager |
| Kotlin     | `io.event-spec:analytics-kotlin` | Maven Central         |
| Python     | `event_spec`                     | PyPI                  |
| Java       | `io.event-spec:analytics-java`   | Maven Central         |
| Rust       | `event-spec-analytics`           | crates.io             |
| Dart       | `event_spec_analytics`           | pub.dev               |
| .NET       | `EventSpec.Analytics`            | NuGet                 |

Generated SDKs import the runtime as a peer dependency. The source config's `output.package` field controls the
generated package name (user-namespaced).

---

## Phased Implementation Roadmap

### Phase 1 — Core Infrastructure (MVP)

**Goal**: End-to-end loop: spec file → codegen → typed wrapper → provider call.

1. `spec/` — YAML loader, `EventDef`/`PropertyDef` structs, JSON Schema Draft-07 property validation
2. `analytics/` — `AnalyticsContext`, `Merge`, `Client`, `API`, 4-level context propagation, `WithAnalyticsContext`, `DispatchResult`
3. `provider/` — `Provider` interface, all message types, `noop` provider, `ProviderConfig`, `RetryConfig`, `RateLimitConfig`
4. `provider/transport.go` — HTTP client with retry logic, exponential backoff with jitter
5. `provider/queue.go` — Event buffering, batching, auto-flush
6. `provider/ratelimit.go` — Token bucket rate limiter
7. `hooks/` — `Hook` interface, `UnimplementedHook`, hook chain executor
8. `hooks/validation/` — Runtime JSON Schema validation hook
9. `hooks/sampling/` — Hash-based + random sampling hook
10. `registry/git/` — file-system walker, in-memory index, version pinning support
11. `codegen/` — `TemplateData` model, `CamelCaseNamer`/`GoNamer`, Go template, TypeScript template
12. `codegen/testdata/golden/` — Golden file tests for template output
13. `cmd/event-spec/` — `generate` and `validate` subcommands
14. `provider/amplitude/` — First real provider with proxy support, secret management
15. `sdk/typescript/` — TypeScript core runtime (client.ts, provider.ts, hooks.ts)
16. `testutil/` — Capture provider for test assertions
17. `analytics/middleware.go` — HTTP middleware for transaction context

Deliverables: `event-spec generate --lang go`, `event-spec generate --lang typescript`, Amplitude provider with batching/retry/proxy, full test
coverage on context merge, hook chain, dispatch results, and golden file tests for codegen.

### Phase 2 — Full Go Feature Set + More Languages

1. `spec/diff.go` + `event-spec diff` subcommand (breaking change detection)
2. Remaining providers: PostHog, Mixpanel, Segment, GA4, RudderStack (all with proxy support)
3. `hooks/logging/` — Structured logging with trace IDs
4. `hooks/otel/` — OpenTelemetry spans + trace context propagation
5. `event-spec pull` + git registry integration
6. `event-spec docs` — HTML/Markdown event catalog generation
7. `event-spec audit` — Event usage verification (AST-based scanning for Go, TypeScript, Swift)
8. `codegen/audit/` — Scanner, coverage report, per-language matchers
9. `registry/server/` — REST API, DB schema, HTTP client
10. Swift runtime (`sdk/swift/`) + Swift codegen template
11. Kotlin runtime (`sdk/kotlin/`) + Kotlin codegen template
12. Python runtime + codegen template
13. Provider-specific property type validation/coercion (e.g., Amplitude type limits)

### Phase 3 — Language Breadth + Registry Server

1. Java, Rust, Dart, .NET runtimes + templates (all with batching/retry/proxy)
2. Registry server: API key auth, RBAC, audit log, webhooks on event publish
3. `event-spec serve` as a standalone binary
4. `provider_overrides` enforcement in codegen (GA4 property mapping)
5. CI/CD: GitHub Actions workflow to auto-generate SDKs on spec change
6. Offline mode: local cache fallback if registry server unreachable

### Phase 4 — Ecosystem Maturity

1. Consent management hook (GDPR/CCPA — block events pre-consent)
2. Replay/audit log provider (event capture sink for debugging)
3. VS Code extension: YAML schema hover, autocomplete
4. GitHub App: PR review comments on breaking spec changes
5. Migration tooling: Ampli/Avo → event-spec converter
6. Circuit breaker pattern for provider failures

---

## Critical Files to Build First

| File                   | Why it's the foundation                                                                |
|------------------------|----------------------------------------------------------------------------------------|
| `spec/schema.go`       | `EventDef`, `PropertyDef`, `SourceDef` — the canonical data model everything builds on |
| `analytics/context.go` | `AnalyticsContext`, `Merge` — the context chain is the core runtime contract                |
| `provider/provider.go` | `Provider` interface + all message types — the contract every adapter satisfies        |
| `hooks/hook.go`        | `Hook` interface, `UnimplementedHook` — middleware contract                            |
| `codegen/model.go`     | `TemplateData`, `Namer` — the language-agnostic intermediate representation            |

---

## Verification Approach (End-to-End)

1. **Spec validation**: `event-spec validate ./specs` → zero errors on a valid spec, correct errors on schema violations
2. **Codegen Go**: `event-spec generate --lang go --out ./testdata/generated` → generated file compiles with `go build`
3. **Codegen TypeScript**: `event-spec generate --lang typescript` → generated file passes `tsc --noEmit`
4. **Golden file tests**: generated output matches committed golden files; diffs caught in CI
5. **Context merge**: unit tests covering all 16 combinations of 4-level precedence
6. **Hook chain**: unit tests verifying Before/After/Error/Finally fire in correct order; Before cancellation stops dispatch
7. **Provider dispatch**: capture provider records all messages; assert `ProductViewed` call produces correct `TrackMessage`
8. **Dispatch results**: verify `DispatchResult` correctly categorizes success/failed providers; partial failures handled
9. **Breaking change detection**: `event-spec diff 1-0-0 1-2-0` correctly labels add-required as MAJOR, add-optional as MINOR
10. **Multi-provider**: two capture providers registered; single `Track` call results in messages in both
11. **Batching & flush**: queue fills to batch size, auto-flush on interval, overflow policy enforced
12. **Retry logic**: simulate 429/500 errors, verify exponential backoff with jitter, max retries respected
13. **Rate limiting**: burst traffic exceeds limit, verify token bucket throttling
14. **Proxy support**: provider configured with proxy URL, verify HTTP requests route correctly
15. **Sampling**: hash-based sampling deterministic per user_id; random sampling distribution matches rate
16. **Runtime validation**: invalid property types caught by validation hook, event rejected pre-dispatch
17. **HTTP middleware**: transaction context injected via middleware, available in handler Track calls
18. **Event audit**: `event-spec audit` detects used/unused/rogue events, coverage % calculated correctly
19. **CI enforcement**: audit with `--strict --coverage-min 80` fails on violations

---

## Key Design Decisions

| Decision                              | Choice                        | Rationale                                                                                              |
|---------------------------------------|-------------------------------|--------------------------------------------------------------------------------------------------------|
| Single `Provider` interface vs. split | Single                        | Avoids runtime interface checks in dispatch; `noop` shows the pattern                                  |
| Codegen engine                        | `text/template`               | Works for all 9 languages; Jennifer is Go-only; template files editable by language experts            |
| Event versioning                      | SchemaVer `1-2-0` (hyphens)   | Visually distinct from SemVer; no ambiguity with file extensions; sorts cleanly                        |
| `AnalyticsContext` storage                 | Value type + explicit merge   | No `context.Value` type assertions; merge logic is explicit and testable                               |
| Multi-provider dispatch               | Concurrent goroutines         | Providers have independent I/O; serializing multiplies latency; `ctx.Done()` is the backstop           |
| Registry primary mode                 | Git-backed                    | Zero infrastructure; diffs are first-class in PR review; `Registry` interface abstracts both modes     |
| Provider overrides location           | Event spec YAML               | GA4/Mixpanel naming quirks encoded once, applied everywhere; no per-provider lookup table at runtime   |
| Batching & queue                      | Per-provider queues           | Isolates slow providers; enables provider-specific batch sizes and flush intervals                     |
| Retry strategy                        | Exponential backoff + jitter  | Industry standard; jitter prevents thundering herd; configurable per-provider                          |
| Proxy support                         | Built-in ProviderConfig field | Ad-blockers common; proxy bypasses blockers; reverse proxy keeps analytics on user's domain           |
| Secret management                     | Pluggable (env/file/vault)    | Dev uses env vars, prod uses vault; no hardcoded credentials                                          |
| Error handling                        | `DispatchResult` with details | Partial failures common; caller decides policy (fail-fast vs. best-effort)                             |
| Sampling strategy                     | Hash-based + random           | Hash ensures consistency per user; random for A/B testing; declared in event spec YAML                |
| Property collision                    | Configurable priority         | Event properties vs. context properties; `property_priority` field in spec YAML                       |
| Rate limiting                         | Token bucket per-provider     | Respects provider limits (Amplitude 30/s, Mixpanel varies); prevents 429 errors                       |
| Runtime validation                    | Hook-based, opt-in            | Codegen provides compile-time safety; runtime validation for dynamic properties or migration scenarios |
| Event usage auditing                  | AST-based static analysis     | Detects spec drift, unused events, rogue calls; CI-enforced coverage minimums prevent instrumentation gaps |
| Codegen output layout                 | One file per event            | A 1000-event project would produce a 50k-line monolithic file; per-event files keep diffs readable, eliminate merge conflicts on shared branches, and make golden-file regressions trivially pinpointed |
| Generated wrapper name                | `EventSpec`                   | Named after the tool; avoids confusion with third-party wrappers (`Ampli`, `Avo`) and signals provenance clearly |

---

## Operational Considerations

### Ad-Blocker Bypass via Proxy

**Problem**: Browser extensions (uBlock, Ghostery) block requests to analytics domains (`*.amplitude.com`, `*.mixpanel.com`).

**Solution**: Reverse proxy analytics traffic through your own domain.

```yaml
# destinations/amplitude.yaml
name: amplitude
provider: amplitude
config:
  api_key: "${AMPLITUDE_API_KEY}"
  secret_type: env_var
  proxy_url: "https://analytics.yourcompany.com/amp"
  proxy_mode: reverse_proxy
```

**Infrastructure setup** (example: nginx):
```nginx
location /amp/ {
  proxy_pass https://api2.amplitude.com/;
  proxy_set_header Host api2.amplitude.com;
  proxy_ssl_server_name on;
}
```

Client sends to `https://analytics.yourcompany.com/amp/2/httpapi`, nginx forwards to Amplitude.

**Benefit**: Bypasses ad-blockers, analytics on your domain looks like first-party traffic.

### Secret Management Patterns

#### Development (local)
```go
amplitude.New(provider.ProviderConfig{
APIKey:     "${AMPLITUDE_API_KEY}",
SecretType: provider.SecretEnvVar,
})
```
Set `export AMPLITUDE_API_KEY=abc123` in shell.

#### Production (Kubernetes + Vault)
```go
amplitude.New(provider.ProviderConfig{
APIKey:     "vault:secret/data/analytics#amplitude_key",
SecretType: provider.SecretVault,
})
```
Runtime fetches from HashiCorp Vault, AWS Secrets Manager, etc. (via integration library).

#### CI/CD (file-based)
```go
amplitude.New(provider.ProviderConfig{
APIKey:     "/run/secrets/amplitude_api_key",  // Docker secret mount
SecretType: provider.SecretFile,
})
```

### Batching & Queue Tuning

**Default settings** (optimized for web apps):
- `BatchSize`: 100 events
- `FlushInterval`: 10 seconds
- `MaxQueueSize`: 10,000 events
- `OverflowPolicy`: `drop_oldest`

**High-throughput backend** (e.g., streaming service):
```go
amplitude.New(provider.ProviderConfig{
BatchSize:      500,
FlushInterval:  1 * time.Second,
MaxQueueSize:   100000,
OverflowPolicy: provider.OverflowBlock,  // backpressure
})
```

**Mobile app** (minimize battery/network):
```go
amplitude.New(provider.ProviderConfig{
BatchSize:      50,
FlushInterval:  30 * time.Second,
OverflowPolicy: provider.OverflowDropNewest,
})
```

### Observability & Debugging

**Structured logging hook**:
```go
import "event-spec/hooks/logging"

client := analytics.NewClient(
analytics.WithHooks(logging.NewHook(logger)),
)
```
Logs every event with trace ID, timestamp, provider outcomes.

**OpenTelemetry spans** (Phase 2):
```go
import "event-spec/hooks/otel"

client := analytics.NewClient(
analytics.WithHooks(otel.NewHook()),
)
```
Creates spans for `Track` calls, annotates with event name, user_id, provider latency.

**Provider dispatch metrics**:
- Track `DispatchResult.Success` vs. `DispatchResult.Failed` per provider
- Alert on partial failure rate > 5%
- Dashboard: p95 provider latency, batch size distribution

### Migration from Existing SDKs

**From Amplitude Ampli**:
1. Export existing tracking plan from Amplitude UI (JSON)
2. Run `event-spec import --source ampli --file tracking-plan.json`
3. Generates event spec YAMLs in `specs/` directory
4. Update code: replace `ampli.productViewed(...)` with generated `analytics.ProductViewed(...)`

**From Segment**:
1. Codegen generates compatible `Track(name, properties)` API
2. Incrementally migrate event-by-event
3. Run both SDKs in parallel (Segment as provider + legacy SDK) during transition

**Property name mapping** (for backwards compatibility):
```yaml
# Event spec can alias old property names
properties:
  product_id:
    type: string
    required: true
    aliases: [productId, prod_id]  # legacy names
```

### Performance Benchmarks (Target)

| Metric                     | Target            | Notes                                     |
|----------------------------|-------------------|-------------------------------------------|
| `Track` call overhead      | < 1ms (p95)       | Async queue, no blocking I/O              |
| Provider batch dispatch    | < 100ms (p95)     | Concurrent dispatch, timeout enforcement  |
| Memory per buffered event  | ~500 bytes        | JSON encoding overhead                    |
| Max throughput (single Go) | 100k events/sec   | With batching, multiple providers         |
| Codegen time (1000 events) | < 5 seconds       | Template rendering, file writes           |
| Audit scan (10k files)     | < 30 seconds      | AST parsing, parallel file processing     |

---

## Client Architecture Modes

The architecture supports two deployment modes to accommodate different client scenarios, similar to how OpenFeature can function as both an embedded SDK and a client to remote feature flag servers.

### Embedded Mode (default)

**Client embeds the full runtime**: providers, hooks, batching, retry, rate limiting.

**Architecture**:
```
┌─────────────────────────────────────────────┐
│   Mobile App / Web App / Backend Service    │
│  ┌────────────────────────────────────────┐ │
│  │  Generated Type-Safe Wrapper           │ │
│  │  ampli.productViewed(...)              │ │
│  └──────────────┬─────────────────────────┘ │
│                 │                            │
│  ┌──────────────▼─────────────────────────┐ │
│  │  event-spec Runtime (embedded)         │ │
│  │  - Hook chain execution                │ │
│  │  - Context merge (4-level)             │ │
│  │  - Validation                           │ │
│  │  - Event queue & batching              │ │
│  └──────────────┬─────────────────────────┘ │
│                 │                            │
│  ┌──────────────▼─────────────────────────┐ │
│  │  Provider Implementations              │ │
│  │  - Amplitude, PostHog, GA4, etc.       │ │
│  │  - Direct HTTP to provider APIs        │ │
│  │  - Retry, rate limiting, circuit break │ │
│  └────────────────────────────────────────┘ │
└─────────────────┼───────────────────────────┘
                  │
                  ▼
        ┌─────────────────────┐
        │  Analytics Providers │
        │  (Amplitude, etc.)   │
        └─────────────────────┘
```

**Use when**:
- Backend services with full control over infrastructure
- Native mobile apps requiring offline event buffering
- Web apps with direct provider communication (via proxy)
- Maximum performance (no extra network hop)
- Offline-first scenarios

**Trade-offs**:
- ✅ Lower latency (no intermediary server)
- ✅ Offline support (events queued locally)
- ✅ Works without network to event-spec server
- ❌ Larger client bundle size (full runtime + providers)
- ❌ Provider credentials distributed to clients
- ❌ Configuration changes require client updates

**Source Configuration** (`sources/mobile-app.yaml`):
```yaml
name: mobile-app
platform: ios
language: swift
mode: embedded  # Full runtime embedded in client
events:
  - ecommerce/**
destinations:
  - amplitude
  - posthog
output:
  path: ./ios/Analytics/Generated
```

**Codegen**:
```bash
event-spec generate mobile-app --client-mode embedded
# Generates:
# - Full Swift runtime with Client, Provider interface, Hooks
# - Provider implementations (Amplitude, PostHog)
# - Event queue, batching, retry logic
# - Type-safe event wrappers
```

---

### Server-Proxied Mode

**Client is a thin wrapper** that sends events to an event-spec server via HTTP; the server runs the full runtime and dispatches to providers.

**Architecture**:
```
┌────────────────────────────────────────────┐
│   Mobile App / Web App                     │
│  ┌───────────────────────────────────────┐ │
│  │  Generated Type-Safe Wrapper          │ │
│  │  ampli.productViewed(...)             │ │
│  └──────────────┬────────────────────────┘ │
│                 │                           │
│  ┌──────────────▼────────────────────────┐ │
│  │  event-spec Thin Client (HTTP)        │ │
│  │  - Schema validation (optional)       │ │
│  │  - Context merge (client-side only)   │ │
│  │  - POST to server endpoint            │ │
│  │  - Minimal retry (network failures)   │ │
│  └───────────────────────────────────────┘ │
└─────────────────┼──────────────────────────┘
                  │ HTTPS
                  ▼
┌─────────────────────────────────────────────┐
│   event-spec Server (your infrastructure)   │
│  ┌────────────────────────────────────────┐ │
│  │  Runtime (full)                        │ │
│  │  - Hook chain execution                │ │
│  │  - Server-side validation              │ │
│  │  - Event queue & batching              │ │
│  │  - Multi-provider dispatch             │ │
│  └──────────────┬─────────────────────────┘ │
│                 │                            │
│  ┌──────────────▼─────────────────────────┐ │
│  │  Provider Implementations              │ │
│  │  - Amplitude, PostHog, GA4, etc.       │ │
│  │  - Retry, rate limiting, circuit break │ │
│  └────────────────────────────────────────┘ │
└─────────────────┼───────────────────────────┘
                  │
                  ▼
        ┌─────────────────────┐
        │  Analytics Providers │
        │  (Amplitude, etc.)   │
        └─────────────────────┘
```

**Use when**:
- Minimizing client bundle size (web apps, embedded devices)
- Centralizing provider configuration and credentials
- Enforcing server-side validation and governance
- Dynamic provider routing without client updates
- Compliance requirements (PII filtering server-side)
- Clients don't have stable internet (server buffers)

**Trade-offs**:
- ✅ Minimal client SDK size (~10-20KB vs. 100-500KB embedded)
- ✅ Provider credentials stay server-side (security)
- ✅ Configuration changes without client updates
- ✅ Centralized logging, monitoring, compliance
- ❌ Extra network hop (latency ~50-200ms)
- ❌ Requires event-spec server availability
- ❌ No offline event buffering (unless client implements queue)

**Source Configuration** (`sources/web-app.yaml`):
```yaml
name: web-app
platform: web
language: typescript
mode: server_proxied  # Thin client, delegates to server
runtime_endpoint: https://analytics.yourcompany.com/v1/track
events:
  - ecommerce/**
output:
  path: ./src/analytics/generated
  package: "@my-company/analytics"

# Optional: client-side validation before sending to server
client_validation: true  # Validate against JSON Schema before POST
```

**Codegen**:
```bash
event-spec generate web-app --client-mode server_proxied
# Generates:
# - TypeScript type-safe wrappers
# - HTTP client that POSTs to runtime_endpoint
# - Optional client-side JSON Schema validation
# NOT generated:
# - Provider implementations
# - Hook chain executor (server-side only)
# - Queue/batching logic (server-side only)
```

**Generated TypeScript Example** (server-proxied mode):
```typescript
// Code generated by event-spec v0.1.0. DO NOT EDIT.

import {ServerProxiedClient, TrackOptions} from '@event-spec/analytics-client';

export class EventSpec {
    constructor(private readonly client: ServerProxiedClient) {}

    async productViewed(props: ProductViewedProperties, opts?: TrackOptions): Promise<void> {
        // Optional client-side validation
        if (this.client.config.clientValidation) {
            validateProductViewedProperties(props); // JSON Schema check
        }

        // POST to server endpoint
        return this.client.track({
            name: "Product Viewed",
            properties: {
                product_id: props.productId,
                product_name: props.productName,
                category: props.category,
                price: props.price,
                currency: props.currency,
                coupon_code: props.couponCode,
            }
        }, opts);
    }
}
```

**Server Endpoints** (event-spec server REST API):

```
POST /v1/track             # Accept track events from thin clients
POST /v1/identify          # Accept identify calls
POST /v1/group             # Accept group calls
POST /v1/page              # Accept page calls
POST /v1/alias             # Accept alias calls
POST /v1/batch             # Accept batch of multiple events
POST /v1/flush             # Force flush queued events (debugging)
```

**Request Format** (POST /v1/track):
```json
{
  "source": "web-app",
  "event_name": "Product Viewed",
  "properties": {
    "product_id": "SKU-123",
    "product_name": "Blue Widget",
    "category": "electronics",
    "price": 49.99
  },
  "context": {
    "user_id": "user-456",
    "anonymous_id": "anon-789",
    "attributes": {
      "session_id": "session-abc",
      "platform": "web"
    }
  },
  "timestamp": "2026-05-19T10:30:00Z"
}
```

**Server Implementation** (Go):
```go
// cmd/event-spec/serve.go

func handleTrackEvent(w http.ResponseWriter, r *http.Request) {
    var req TrackRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Server-side validation against event spec
    if err := validateEvent(req.Source, req.EventName, req.Properties); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Dispatch through full runtime (hooks, providers, batching)
    client := getClientForSource(req.Source) // cached per source
    err := client.Track(r.Context(), analytics.Event{
        Name:       req.EventName,
        Properties: req.Properties,
    }, analytics.WithContextOverride(analytics.AnalyticsContext{
        UserID:      req.Context.UserID,
        AnonymousID: req.Context.AnonymousID,
        Attributes:  req.Context.Attributes,
    }))

    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusAccepted) // 202 Accepted (async processing)
}
```

**Authentication** (server-proxied mode):
```yaml
# Server config (event-spec.yaml)
server:
  port: 8080
  auth:
    mode: api_key  # api_key | jwt | oauth
    header: "X-Analytics-Key"
  rate_limit:
    requests_per_second: 1000
    burst: 2000
```

Client includes auth header:
```typescript
const client = new ServerProxiedClient({
    endpoint: "https://analytics.yourcompany.com/v1",
    apiKey: "web-app-key-abc123",  // scoped to source
});
const es = new EventSpec(client);
```

---

### Hybrid Mode (Advanced)

**Combination of embedded + server-proxied** for different scenarios:

**Use case**: Mobile app that normally sends to server, but buffers events locally when offline and syncs later.

```yaml
# sources/mobile-app.yaml
name: mobile-app
platform: ios
language: swift
mode: hybrid
runtime_endpoint: https://analytics.yourcompany.com/v1/track
fallback_mode: embedded  # Use embedded providers if server unreachable
offline_buffer: true
destinations:
  - amplitude  # Fallback only
```

**Generated SDK behavior**:
1. **Online**: POST to server endpoint (server-proxied)
2. **Offline detected**: Queue events locally (embedded queue)
3. **Back online**: Batch sync to server
4. **Server down > 5 minutes**: Fallback to embedded Amplitude provider (direct send)

**Configuration** (in generated SDK):
```swift
let client = AnalyticsClient(config: AnalyticsConfig(
    mode: .hybrid,
    serverEndpoint: "https://analytics.yourcompany.com/v1/track",
    fallbackProviders: [AmplitudeProvider(apiKey: "...")],
    offlineBufferSize: 10000,
    syncInterval: 30.seconds
))
```

---

### Mode Comparison Table

| Feature                     | Embedded            | Server-Proxied      | Hybrid              |
|-----------------------------|---------------------|---------------------|---------------------|
| Client bundle size          | Large (100-500KB)   | Small (10-20KB)     | Medium (50-200KB)   |
| Offline support             | ✅ Full             | ❌ None             | ✅ Buffered         |
| Provider credentials        | Client-side         | Server-side         | Both                |
| Configuration updates       | Requires client update | Server-side only | Server-side + fallback |
| Latency                     | Low (~10ms)         | Medium (~100ms)     | Low (online) / High (offline sync) |
| Server dependency           | None                | Required            | Optional            |
| PII filtering               | Client-side only    | Server-side         | Both                |
| Use case                    | Backend, native apps | Web apps, embedded devices | Mobile apps, PWAs |

---

### Implementation Phases

**Phase 1** (MVP):
- Embedded mode only (full runtime in all SDKs)
- Establishes core Provider interface, hooks, codegen

**Phase 2**:
- Server-proxied mode for TypeScript/JavaScript
- Add REST endpoints to registry server: `/v1/track`, `/v1/identify`, etc.
- Codegen flag: `--client-mode server_proxied`
- Thin client SDK: `@event-spec/analytics-client`

**Phase 3**:
- Server-proxied mode for Swift, Kotlin, Python
- Hybrid mode support (offline buffering + server sync)
- Server-side SDK for running event-spec as a service (`event-spec serve --mode runtime`)

**Phase 4**:
- Edge runtime support (Cloudflare Workers, Vercel Edge Functions)
- WebAssembly thin client for maximum portability
- Server-side batching optimizations (multi-tenant event buffering)

---

### Event Governance Workflow

**Scenario**: E-commerce team wants to ensure checkout funnel instrumentation is complete before launch.

1. **Define critical events**:
```yaml
# specs/ecommerce/checkout_started/1-0-0.yaml
name: checkout_started
required: true  # MUST be instrumented
tags: [critical, revenue, funnel]
```

2. **Generate typed SDK**:
```bash
event-spec generate web-app
# Creates ampli.checkoutStarted(...) method
```

3. **Implement in code**:
```typescript
// src/pages/Checkout.tsx
es.checkoutStarted({cartValue: total, itemCount: items.length})
```

4. **PR checks** (GitHub Actions):
```yaml
- name: Audit analytics coverage
  run: event-spec audit web-app --coverage-min 90 --strict
# Fails if:
#  - checkout_started not found in code (required: true)
#  - Coverage < 90%
#  - Any rogue events detected
```

5. **Monitor in production**:
- Logging hook captures all events → structured logs
- Alert if `checkout_started` event volume drops by >20% (indicates broken instrumentation)

**Benefit**: Spec → codegen → audit → monitoring creates a closed-loop governance system. Instrumentation gaps caught in CI, not production.

---

## Operational Considerations

### Ad-Blocker Bypass via Proxy

**Problem**: Browser extensions (uBlock, Ghostery) block requests to analytics domains (`*.amplitude.com`, `*.mixpanel.com`).

**Solution**: Reverse proxy analytics traffic through your own domain.

```yaml
# destinations/amplitude.yaml
name: amplitude
provider: amplitude
config:
  api_key: "${AMPLITUDE_API_KEY}"
  secret_type: env_var
  proxy_url: "https://analytics.yourcompany.com/amp"
  proxy_mode: reverse_proxy
```

**Infrastructure setup** (example: nginx):
```nginx
location /amp/ {
  proxy_pass https://api2.amplitude.com/;
  proxy_set_header Host api2.amplitude.com;
  proxy_ssl_server_name on;
}
```

Client sends to `https://analytics.yourcompany.com/amp/2/httpapi`, nginx forwards to Amplitude.

**Benefit**: Bypasses ad-blockers, analytics on your domain looks like first-party traffic.

### Secret Management Patterns

#### Development (local)
```go
amplitude.New(provider.ProviderConfig{
APIKey:     "${AMPLITUDE_API_KEY}",
SecretType: provider.SecretEnvVar,
})
```
Set `export AMPLITUDE_API_KEY=abc123` in shell.

#### Production (Kubernetes + Vault)
```go
amplitude.New(provider.ProviderConfig{
APIKey:     "vault:secret/data/analytics#amplitude_key",
SecretType: provider.SecretVault,
})
```
Runtime fetches from HashiCorp Vault, AWS Secrets Manager, etc. (via integration library).

#### CI/CD (file-based)
```go
amplitude.New(provider.ProviderConfig{
APIKey:     "/run/secrets/amplitude_api_key",  // Docker secret mount
SecretType: provider.SecretFile,
})
```

### Batching & Queue Tuning

**Default settings** (optimized for web apps):
- `BatchSize`: 100 events
- `FlushInterval`: 10 seconds
- `MaxQueueSize`: 10,000 events
- `OverflowPolicy`: `drop_oldest`

**High-throughput backend** (e.g., streaming service):
```go
amplitude.New(provider.ProviderConfig{
BatchSize:      500,
FlushInterval:  1 * time.Second,
MaxQueueSize:   100000,
OverflowPolicy: provider.OverflowBlock,  // backpressure
})
```

**Mobile app** (minimize battery/network):
```go
amplitude.New(provider.ProviderConfig{
BatchSize:      50,
FlushInterval:  30 * time.Second,
OverflowPolicy: provider.OverflowDropNewest,
})
```

### Observability & Debugging

**Structured logging hook**:
```go
import "event-spec/hooks/logging"

client := analytics.NewClient(
analytics.WithHooks(logging.NewHook(logger)),
)
```
Logs every event with trace ID, timestamp, provider outcomes.

**OpenTelemetry spans** (Phase 2):
```go
import "event-spec/hooks/otel"

client := analytics.NewClient(
analytics.WithHooks(otel.NewHook()),
)
```
Creates spans for `Track` calls, annotates with event name, user_id, provider latency.

**Provider dispatch metrics**:
- Track `DispatchResult.Success` vs. `DispatchResult.Failed` per provider
- Alert on partial failure rate > 5%
- Dashboard: p95 provider latency, batch size distribution

### Migration from Existing SDKs

**From Amplitude Ampli**:
1. Export existing tracking plan from Amplitude UI (JSON)
2. Run `event-spec import --source ampli --file tracking-plan.json`
3. Generates event spec YAMLs in `specs/` directory
4. Update code: replace `ampli.productViewed(...)` with generated `analytics.ProductViewed(...)`

**From Segment**:
1. Codegen generates compatible `Track(name, properties)` API
2. Incrementally migrate event-by-event
3. Run both SDKs in parallel (Segment as provider + legacy SDK) during transition

**Property name mapping** (for backwards compatibility):
```yaml
# Event spec can alias old property names
properties:
  product_id:
    type: string
    required: true
    aliases: [productId, prod_id]  # legacy names
```

### Performance Benchmarks (Target)

| Metric                     | Target            | Notes                                     |
|----------------------------|-------------------|-------------------------------------------|
| `Track` call overhead      | < 1ms (p95)       | Async queue, no blocking I/O              |
| Provider batch dispatch    | < 100ms (p95)     | Concurrent dispatch, timeout enforcement  |
| Memory per buffered event  | ~500 bytes        | JSON encoding overhead                    |
| Max throughput (single Go) | 100k events/sec   | With batching, multiple providers         |
| Codegen time (1000 events) | < 5 seconds       | Template rendering, file writes           |
