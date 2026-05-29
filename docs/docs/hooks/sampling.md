---
sidebar_position: 2
---

# Sampling Hook

The sampling hook drops events before they reach any provider, reducing analytics costs while preserving statistical validity.

## Strategies

### `user_id_hash` (recommended)

Deterministically assigns users to the sampled fraction using a consistent hash of the user ID. A user is **always** sampled or **never** sampled — ensuring full user journeys survive in the data set:

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs>
<TabItem value="go" label="Go">

```go
import "github.com/dejanradmanovic/event-spec/hooks/sampling"

hook := sampling.New(sampling.Config{
    Strategy: sampling.UserIDHash,
    Rate:     0.1,  // 10% of users
})
```

</TabItem>
<TabItem value="ts" label="TypeScript">

```typescript
import { SamplingHook, SamplingStrategy } from '@dejanradmanovic/event-spec-api';

const hook = new SamplingHook({
    strategy: SamplingStrategy.UserIdHash,
    rate: 0.1,  // 10% of users
});
```

</TabItem>
<TabItem value="kotlin" label="Kotlin">

```kotlin
import io.eventspec.analytics.SamplingHook
import io.eventspec.analytics.SamplingPolicy
import io.eventspec.analytics.SamplingStrategy

val hook = SamplingHook { eventName ->
    SamplingPolicy(strategy = SamplingStrategy.USER_ID_HASH, rate = 0.1)
}
```

</TabItem>
</Tabs>

### `random`

Randomly samples each event independently. Good for high-cardinality events where per-user consistency is not required:

<Tabs>
<TabItem value="go" label="Go">

```go
hook := sampling.New(sampling.Config{
    Strategy: sampling.Random,
    Rate:     0.05,  // 5% of events
})
```

</TabItem>
<TabItem value="ts" label="TypeScript">

```typescript
const hook = new SamplingHook({
    strategy: SamplingStrategy.Random,
    rate: 0.05,  // 5% of events
});
```

</TabItem>
<TabItem value="kotlin" label="Kotlin">

```kotlin
val hook = SamplingHook { eventName ->
    SamplingPolicy(strategy = SamplingStrategy.RANDOM, rate = 0.05)
}
```

</TabItem>
</Tabs>

## Per-event config in the spec

You can declare sampling config inside the event YAML so it is enforced automatically by the hook:

```yaml title="specs/ecommerce/product_viewed/1-0-0.yaml"
sampling:
  strategy: user_id_hash
  rate: 0.5
```

The hook reads this config from the event envelope and overrides the global default when present.

## Rate values

| Rate | Meaning |
|------|---------|
| `1.0` | All events pass (no sampling) |
| `0.5` | 50% of users / events |
| `0.1` | 10% |
| `0.0` | All events dropped |

## Outcome

A dropped event records a `Dropped` delivery state for all providers. It does not reach any `After` or `Error` hook stage — only `Finally`.

