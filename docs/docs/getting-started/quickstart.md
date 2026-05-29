---
sidebar_position: 3
---

# Quickstart

This guide walks through the complete flow: write an event spec → validate → generate typed wrappers → instrument your app.

**Time to complete:** ~5 minutes

## Prerequisites

- [event-spec CLI installed](./installation.md)
- Go 1.21+, Node.js 18+, or JDK 17+ (depending on your target language)

## Step 1 — Workspace setup

```bash
mkdir my-project && cd my-project
```

Create `event-spec.yaml`:

```yaml title="event-spec.yaml"
version: 1
workspace: "my-company"
registry:
  mode: local
specs_dir: ./specs
sources_dir: ./sources
destinations_dir: ./destinations
```

## Step 2 — Write an event spec

```bash
mkdir -p specs/ecommerce/product_viewed
```

```yaml title="specs/ecommerce/product_viewed/1-0-0.yaml"
$schema: "https://event-spec.io/schemas/event/v1"
name: product_viewed
display_name: "Product Viewed"
version: "1-0-0"
status: active
namespace: ecommerce
type: track
event_name: "Product Viewed"
description: "Fired when a user views a product detail page."

properties:
  product_id:
    type: string
    required: true
    description: "SKU or internal product identifier"
  category:
    type: string
    required: true
    enum: [clothing, electronics, other]
  currency:
    type: string
    required: false
    default: "USD"
```

## Step 3 — Validate

```bash
event-spec validate
```

Expected output:
```
validated 1 event spec(s): ok
```

Run with `--strict` to fail on deprecated or deleted events (useful in CI):

```bash
event-spec validate --strict
```

## Step 4 — Generate typed wrappers

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs>
<TabItem value="go" label="Go">

```bash
event-spec generate --lang go --out ./generated
```

This creates:
```
generated/
├── eventspec.go              # EventSpec struct + New()
└── ecommerce/
    └── product_viewed.go     # ProductViewed(), ProductViewedProperties, enum consts
```

</TabItem>
<TabItem value="typescript" label="TypeScript">

```bash
event-spec generate --lang typescript --out ./src/analytics/generated
```

This creates:
```
src/analytics/generated/
├── index.ts
└── ecommerce/
    └── product_viewed.ts
```

</TabItem>
<TabItem value="kotlin" label="Kotlin">

```bash
event-spec generate --lang kotlin --out ./generated
```

This creates:
```
generated/
├── EventSpec.kt              # EventSpec class
└── ProductViewed.kt          # enum, data class, suspend extension fun
```

</TabItem>
</Tabs>

## Step 5 — Instrument your app

<Tabs>
<TabItem value="go" label="Go">

```go title="main.go"
package main

import (
    "context"

    core "github.com/dejanradmanovic/event-spec/analytics"
    "github.com/dejanradmanovic/event-spec/provider/amplitude"
    "github.com/dejanradmanovic/event-spec/provider"
    generated "my-module/generated"
)

func main() {
    amp, err := amplitude.New(amplitude.Config{
        ProviderConfig: provider.ProviderConfig{
            APIKey:     "${AMPLITUDE_API_KEY}",
            SecretType: provider.SecretEnvVar,
        },
    })
    if err != nil {
        panic(err)
    }

    client := core.NewClient(core.WithProviders(amp))
    es := generated.New(client)

    es.ProductViewed(context.Background(), generated.ProductViewedProperties{
        Category:  generated.ProductViewedCategoryElectronics,
        ProductId: "SKU-123",
        // Currency omitted — uses default "USD"
    })
}
```

</TabItem>
<TabItem value="typescript" label="TypeScript">

```typescript title="index.ts"
import { Client } from '@dejanradmanovic/event-spec-api';
import { AmplitudeProvider } from '@dejanradmanovic/event-spec-provider-amplitude';
import { productViewed, ProductViewedCategory } from './analytics/generated';

const amp = new AmplitudeProvider({ apiKey: process.env.AMPLITUDE_API_KEY! });
const client = new Client({ providers: [amp] });

await client.productViewed({
    category: ProductViewedCategory.Electronics,
    productId: 'SKU-123',
});
```

</TabItem>
<TabItem value="kotlin" label="Kotlin">

```kotlin title="Main.kt"
import io.eventspec.analytics.Client
import io.eventspec.analytics.ClientOptions
import io.eventspec.analytics.amplitude.AmplitudeConfig
import io.eventspec.analytics.amplitude.AmplitudeProvider
import analytics.EventSpec
import analytics.ProductViewedProperties
import analytics.ProductViewedCategory
import kotlinx.coroutines.runBlocking

fun main() = runBlocking {
    val amp = AmplitudeProvider(AmplitudeConfig(apiKey = System.getenv("AMPLITUDE_API_KEY")!!))
    val client = Client(ClientOptions(providers = listOf(amp)))
    val es = EventSpec(client)

    es.productViewed(ProductViewedProperties(
        category = ProductViewedCategory.ELECTRONICS,
        productId = "SKU-123",
        // currency omitted — uses default null
    ))
}
```

</TabItem>
</Tabs>

## Step 6 — Use a source config (optional)

Instead of passing `--lang` and `--out` every time, define a source file that captures your app's settings:

```yaml title="sources/web-app.yaml"
name: web-app
platform: web
language: typescript
events:
  - ecommerce/**
output:
  path: ./src/analytics/generated
  package: "@my-company/analytics"
```

Then generate with:

```bash
event-spec generate web-app
```

## What's next?

- [Event Contract concepts](../concepts/event-contract.md) — versioning, SchemaVer, property types
- [Context propagation](../concepts/context.md) — user identity, 4-level context chain
- [Hooks](../concepts/hooks.md) — validation, sampling, custom hooks
- [CLI reference](../cli/index.md) — full flag documentation for every subcommand
