---
sidebar_position: 3
---

# Kotlin SDK

The Kotlin SDK provides the analytics runtime for JVM and Android applications.

## Modules

| Module | Maven coordinates | Purpose |
|--------|-------------------|---------|
| `api` | `io.event-spec:api-kotlin` | Core runtime — Client, Provider interface, Hook system, context |
| `provider-amplitude` | `io.event-spec:kotlin-provider-amplitude` | Amplitude HTTP API adapter |

## Installation

Add the GitHub Packages repository and declare dependencies in your `libs.versions.toml`:

```toml title="gradle/libs.versions.toml"
[versions]
event-spec = "0.1.0"

[libraries]
event-spec-api = { module = "io.event-spec:api-kotlin", version.ref = "event-spec" }
event-spec-provider-amplitude = { module = "io.event-spec:kotlin-provider-amplitude", version.ref = "event-spec" }
```

```kotlin title="build.gradle.kts"
repositories {
    maven {
        url = uri("https://maven.pkg.github.com/dejanradmanovic/event-spec")
        credentials {
            username = System.getenv("GITHUB_ACTOR")
            password = System.getenv("GITHUB_TOKEN")
        }
    }
}

dependencies {
    implementation(libs.event.spec.api)
    implementation(libs.event.spec.provider.amplitude)
}
```

## Setup

```kotlin
import io.eventspec.analytics.Client
import io.eventspec.analytics.ClientOptions
import io.eventspec.analytics.amplitude.AmplitudeConfig
import io.eventspec.analytics.amplitude.AmplitudeProvider

val amp = AmplitudeProvider(
    AmplitudeConfig(apiKey = System.getenv("AMPLITUDE_API_KEY")!!)
)

val client = Client(ClientOptions(providers = listOf(amp)))
```

## Tracking events

```kotlin
client.track(Event(
    name = "Product Viewed",
    properties = mapOf(
        "product_id" to "SKU-123",
        "category" to "electronics",
    ),
))
```

### Per-provider outcomes

`track` throws on pre-dispatch failures. Use `trackDetailed` for per-provider results:

```kotlin
val result = client.trackDetailed(event)
// result.success  — providers that succeeded
// result.failed   — providers that failed permanently
// result.partialSuccess — at least one succeeded
```

## Identify

```kotlin
client.identify("user-123", mapOf(
    "email" to "alice@example.com",
    "plan" to "pro",
))
```

## Group

```kotlin
client.group("user-123", "org-456", mapOf(
    "name" to "Acme Corp",
    "plan" to "enterprise",
))
```

## Page

```kotlin
client.page("user-123", "Checkout", mapOf("url" to "/checkout"))
```

## Alias

```kotlin
client.alias("user-123", "anon-abc")
```

## Context propagation

Set startup metadata once globally:

```kotlin
import io.eventspec.analytics.setGlobalContext
import io.eventspec.analytics.AnalyticsContext

setGlobalContext(AnalyticsContext(
    attributes = mapOf(
        "locale" to "en-US",
        "app" to mapOf("name" to "my-app", "version" to "2.1.0"),
    ),
))
```

Per-request identity via `withTransaction`:

```kotlin
val reqClient = client.withTransaction(AnalyticsContext(
    userId = "user-123",
    anonymousId = sessionId,
))
reqClient.track(Event(name = "Checkout Started"))
```

## Using generated wrappers

```kotlin
import analytics.EventSpec
import analytics.ProductViewedProperties
import analytics.ProductViewedCategory

val es = EventSpec(client)

es.productViewed(ProductViewedProperties(
    category = ProductViewedCategory.ELECTRONICS,
    productId = "SKU-123",
))
```

## Built-in hooks

```kotlin
import io.eventspec.analytics.SamplingHook
import io.eventspec.analytics.SamplingStrategy
import io.eventspec.analytics.SamplingPolicy
import io.eventspec.analytics.ValidationHook

val samplingHook = SamplingHook { eventName ->
    SamplingPolicy(strategy = SamplingStrategy.USER_ID_HASH, rate = 0.1)
}

val validationHook = ValidationHook { eventName, properties ->
    // return an error string, or null if valid
    null
}

val client = Client(ClientOptions(
    providers = listOf(amp),
    hooks = listOf(validationHook, samplingHook),
))
```

## Coroutines

All dispatch methods (`track`, `identify`, `group`, `page`, `alias`) are `suspend` functions. Call them from a coroutine scope:

```kotlin
import kotlinx.coroutines.runBlocking

runBlocking {
    client.track(Event(name = "App Started"))
}
```

## Shutdown

```kotlin
client.shutdown()
```
