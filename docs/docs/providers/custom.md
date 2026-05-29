---
sidebar_position: 4
---

# Custom Provider

Implement the `Provider` interface to send events to any backend — internal databases, custom HTTP APIs, or analytics platforms not yet supported natively.

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## Implementing the interface (Go)

```go
package myprovider

import (
    "context"
    "github.com/dejanradmanovic/event-spec/analytics"
    "github.com/dejanradmanovic/event-spec/provider"
    "github.com/dejanradmanovic/event-spec/hooks"
)

type MyProvider struct{}

func New() *MyProvider { return &MyProvider{} }

func (p *MyProvider) Metadata() provider.ProviderMetadata {
    return provider.ProviderMetadata{Name: "my-provider", Version: "1.0.0"}
}

func (p *MyProvider) Hooks() []hooks.Hook { return nil }

func (p *MyProvider) Track(ctx context.Context, msg provider.TrackMessage) error {
    // Send msg to your backend
    return nil
}

func (p *MyProvider) Identify(ctx context.Context, msg provider.IdentifyMessage) error {
    return provider.ErrUnsupportedOperation // if not supported
}

func (p *MyProvider) Group(ctx context.Context, msg provider.GroupMessage) error {
    return provider.ErrUnsupportedOperation
}

func (p *MyProvider) Page(ctx context.Context, msg provider.PageMessage) error {
    return provider.ErrUnsupportedOperation
}

func (p *MyProvider) Alias(ctx context.Context, msg provider.AliasMessage) error {
    return provider.ErrUnsupportedOperation
}

func (p *MyProvider) Flush(ctx context.Context) error    { return nil }
func (p *MyProvider) Shutdown(ctx context.Context) error { return nil }
```

## Implementing the interface (Kotlin)

```kotlin
import io.eventspec.analytics.*

class MyProvider : Provider {
    override fun metadata() = ProviderMetadata(
        name = "my-provider",
        version = "1.0.0",
        capabilities = ProviderCapabilities(
            track = true, identify = false, group = false, page = false, alias = false,
        ),
    )

    override fun hooks(): List<Hook> = emptyList()

    override suspend fun track(msg: TrackMessage) {
        // Send msg to your backend
    }

    override suspend fun identify(msg: IdentifyMessage) {
        throw UnsupportedOperationException("identify")
    }

    override suspend fun group(msg: GroupMessage) {
        throw UnsupportedOperationException("group")
    }

    override suspend fun page(msg: PageMessage) {
        throw UnsupportedOperationException("page")
    }

    override suspend fun alias(msg: AliasMessage) {
        throw UnsupportedOperationException("alias")
    }

    override suspend fun flush() {}
    override suspend fun shutdown() {}
}
```

## Implementing the interface (TypeScript)

```typescript
import type {
    Provider,
    ProviderMetadata,
    TrackMessage,
    IdentifyMessage,
    GroupMessage,
    PageMessage,
    AliasMessage,
} from '@dejanradmanovic/event-spec-api';

export class MyProvider implements Provider {
    readonly metadata: ProviderMetadata = {
        name: 'my-provider',
        version: '1.0.0',
    };

    async track(ctx: unknown, msg: TrackMessage): Promise<void> {
        // Send to your backend
    }

    async identify(ctx: unknown, msg: IdentifyMessage): Promise<void> {
        throw new Error('unsupported');
    }

    async group(ctx: unknown, msg: GroupMessage): Promise<void> {
        throw new Error('unsupported');
    }

    async page(ctx: unknown, msg: PageMessage): Promise<void> {
        throw new Error('unsupported');
    }

    async alias(ctx: unknown, msg: AliasMessage): Promise<void> {
        throw new Error('unsupported');
    }

    async flush(ctx: unknown): Promise<void> {}
    async shutdown(ctx: unknown): Promise<void> {}
}
```

## Important: unsupported operations

Return `provider.ErrUnsupportedOperation` (Go) or throw `new Error('unsupported')` (TypeScript) for operations your provider doesn't support. Never silently return `nil` / `void` — that would look like a successful delivery.

## Using built-in infrastructure

You can use the shared transport, queue, and rate-limiter instead of writing your own:

```go
type MyProvider struct {
    queue *provider.EventQueue
    transport *provider.Transport
}

func New(cfg provider.ProviderConfig) (*MyProvider, error) {
    transport, err := provider.NewTransport(cfg)
    if err != nil {
        return nil, err
    }
    queue := provider.NewEventQueue(provider.QueueConfig{
        MaxSize:       cfg.MaxQueueSize,
        FlushInterval: cfg.FlushInterval,
        BatchSize:     cfg.BatchSize,
        OnFlush:       func(batch []provider.QueuedEvent) { /* send batch */ },
    })
    return &MyProvider{queue: queue, transport: transport}, nil
}
```

## Provider capabilities

Declare what your provider supports so the runtime can produce accurate `Dropped` outcomes:

```go
func (p *MyProvider) Metadata() provider.ProviderMetadata {
    return provider.ProviderMetadata{
        Name:    "my-provider",
        Version: "1.0.0",
        Capabilities: provider.ProviderCapabilities{
            Track:    true,
            Identify: false,
            Group:    false,
            Page:     false,
            Alias:    false,
        },
    }
}
```

## Testing your provider

Use `testutil.CaptureProvider` to verify your provider receives the right messages in integration tests, and write unit tests that mock the HTTP layer.

## Contributing

If your provider could be useful to others, consider contributing it to the main repository. See [Contributing — Adding Providers](../contributing/adding-providers.md).
