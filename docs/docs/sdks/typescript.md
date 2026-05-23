---
sidebar_position: 2
---

# TypeScript SDK

The TypeScript SDK provides the analytics runtime for web, Node.js, and React Native applications.

## Packages

| Package | npm scope | Purpose |
|---------|-----------|---------|
| `@dejanradmanovic/event-spec-api` | Core runtime | Client, Provider interface, Hook system, context |
| `@dejanradmanovic/event-spec-provider-amplitude` | Amplitude provider | Amplitude HTTP API adapter |

## Installation

Add a `.npmrc` pointing the `@dejanradmanovic` scope to GitHub Packages:

```ini title=".npmrc"
@dejanradmanovic:registry=https://npm.pkg.github.com
```

Then install:

```bash
npm install @dejanradmanovic/event-spec-api @dejanradmanovic/event-spec-provider-amplitude
```

## Setup

```typescript
import { Client } from '@dejanradmanovic/event-spec-api';
import { AmplitudeProvider } from '@dejanradmanovic/event-spec-provider-amplitude';

const amp = new AmplitudeProvider({
    apiKey: process.env.AMPLITUDE_API_KEY!,
});

const client = new Client({
    providers: [amp],
});
```

## Tracking events

```typescript
await client.track('Product Viewed', {
    product_id: 'SKU-123',
    category: 'electronics',
});
```

## Identify

```typescript
await client.identify('user-123', {
    email: 'alice@example.com',
    plan: 'pro',
});
```

## Group

```typescript
await client.group('user-123', 'org-456', {
    name: 'Acme Corp',
    plan: 'enterprise',
});
```

## Using generated wrappers

```typescript
import { productViewed, ProductViewedCategory } from './analytics/generated';
import { client } from './analytics/client';

await productViewed(client, {
    category: ProductViewedCategory.Electronics,
    productId: 'SKU-123',
});
```

## Context propagation

```typescript
import { setGlobalContext } from '@dejanradmanovic/event-spec-api';

// Set at startup
setGlobalContext({
    attributes: {
        locale: navigator.language,
        app: { name: 'web-app', version: APP_VERSION },
    },
});

// Per-request (server-side)
const reqClient = client.withTransaction({
    userId: req.user.id,
    anonymousId: req.session.id,
});
```

## Multiple providers

```typescript
const client = new Client({
    providers: [amplitudeProvider, posthogProvider],
});
// Both receive every event.
```

## Hooks

```typescript
import { SamplingHook, SamplingStrategy, ValidationHook } from '@dejanradmanovic/event-spec-api';

const client = new Client({
    providers: [amp],
    hooks: [
        new SamplingHook({ strategy: SamplingStrategy.UserIdHash, rate: 0.1 }),
        new ValidationHook({ lookup: async (name) => registry.getEvent(name) }),
    ],
});
```

## Event queue

The TypeScript client batches events and flushes them:
- **Automatically** every `flushInterval` (default: 10 seconds)
- **Manually** via `await client.flush()`
- **On shutdown** via `await client.shutdown()`

Always call `shutdown()` in serverless environments (Vercel edge functions, Cloudflare Workers) to flush before the process exits:

```typescript
process.on('SIGTERM', async () => {
    await client.shutdown();
    process.exit(0);
});
```

## TypeScript types

All provider and hook interfaces are fully typed. Import types directly:

```typescript
import type {
    Provider,
    Hook,
    HookContext,
    EventEnvelope,
    TrackMessage,
    IdentifyMessage,
    AnalyticsContext,
} from '@dejanradmanovic/event-spec-api';
```
