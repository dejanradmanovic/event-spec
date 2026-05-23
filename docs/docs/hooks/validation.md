---
sidebar_position: 3
---

# Validation Hook

The validation hook enforces event schema contracts at runtime. When an event is dispatched, the hook fetches its spec from the registry and validates all properties against the JSON Schema — before any provider receives the event.

## Setup

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs>
<TabItem value="go" label="Go">

```go
import (
    "github.com/dejanradmanovic/event-spec/hooks/validation"
    "github.com/dejanradmanovic/event-spec/registry/local"
    "github.com/dejanradmanovic/event-spec/spec"
)

reg, _ := local.New(local.Config{SpecsDir: "./specs"})

lookup := func(name string) (*spec.EventDef, bool) {
    def, err := reg.GetEvent(ctx, namespace, name, "")
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

</TabItem>
<TabItem value="ts" label="TypeScript">

```typescript
import { ValidationHook } from '@dejanradmanovic/event-spec-api';

const hook = new ValidationHook({
    lookup: async (name) => {
        const def = await registry.getEvent('ecommerce', name, '');
        return def ?? null;
    },
});

const client = new Client({
    providers: [amp],
    hooks: [hook],
});
```

</TabItem>
</Tabs>

## Behavior

| Scenario | Outcome |
|----------|---------|
| Event matches spec | Passes through to providers |
| Missing required property | Dropped + error logged |
| Wrong type (e.g. integer for string) | Dropped + error logged |
| Unknown event name (no spec found) | Configurable: pass or drop |
| `status: deprecated` event | Warning logged, event passes |
| `status: deleted` event | Dropped |

## What gets validated

- Required properties are present and non-null
- Property types match the spec (string, number, integer, boolean, object, array)
- Enum constraints — value must be one of the declared enum values
- Pattern constraints — string matches the regex pattern
- Minimum / maximum constraints — number is within range
