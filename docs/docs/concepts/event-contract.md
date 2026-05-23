---
sidebar_position: 1
---

# Event Contract

The **event contract** is the central primitive in event-spec. It is a YAML file that describes exactly one analytics event: its name, version, status, type, and every property it carries.

## Anatomy of a spec file

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
  price:
    type: number
    required: false
    minimum: 0
  currency:
    type: string
    required: false
    default: "USD"
  tags:
    type: array
    required: false

hooks:
  sampling:
    strategy: user_id_hash
    rate: 0.5
  validation:
    mode: strict
```

## Versioning: SchemaVer

event-spec uses **SchemaVer** (`MAJOR-MINOR-PATCH`) for event versions — borrowed from Snowplow Iglu. Hyphens visually distinguish event versions from SemVer (which governs CLI and SDK releases).

| Change | Bump |
|--------|------|
| Add required property | **MAJOR** |
| Remove any property | **MAJOR** |
| Rename property | **MAJOR** |
| Rename event (`name` or `event_name` changed) | **MAJOR** |
| Change property type | **MAJOR** |
| Change event type (`track` → `page` etc.) | **MAJOR** |
| Make optional → required | **MAJOR** |
| Remove enum value | **MAJOR** |
| Status changed to `deleted` | **MAJOR** |
| Make required → optional | MINOR |
| Add optional property | MINOR |
| Add enum value | MINOR |
| Status changed (non-`deleted`) | MINOR |
| Sampling config added / changed / removed | MINOR |
| Context properties changed | MINOR |
| Provider overrides changed | MINOR |
| Property description-only change | PATCH |
| Metadata changed (`display_name`, `description`, `owner`, `tags`) | PATCH |
| Destinations changed | PATCH |
| Property priority changed | PATCH |

The CLI enforces these rules with `event-spec diff`:

```bash
event-spec diff specs/ecommerce/product_viewed/1-0-0.yaml \
               specs/ecommerce/product_viewed/2-0-0.yaml
# BREAKING: removed property "currency"
# BREAKING: "category" changed type string → integer
```

See [CLI — diff](../cli/diff.md) for the full command reference.

## Status lifecycle

| Status | Meaning |
|--------|---------|
| `draft` | Work in progress. Not validated in CI by default. |
| `active` | In production use. Validated strictly. |
| `deprecated` | Being phased out. `validate --strict` emits a warning. |
| `deleted` | No longer used. Retained for historical audit purposes. |

The validation hook uses status to decide behavior: `active` events are fully validated; `deprecated` events emit warnings; `deleted` events are skipped.

## Event type

| Type | Analytics call |
|------|---------------|
| `track` | `client.Track()` |
| `identify` | `client.Identify()` |
| `group` | `client.Group()` |
| `page` | `client.Page()` |
| `alias` | `client.Alias()` |

## Property types

| Type | Generated Go type | Generated TypeScript type |
|------|------------------|--------------------------|
| `string` | `string` | `string` |
| `number` | `float64` | `number` |
| `integer` | `int64` | `number` |
| `boolean` | `bool` | `boolean` |
| `object` | `map[string]any` | `Record<string, unknown>` |
| `array` | `[]any` | `unknown[]` |

### Constraints

| Constraint | Applies to | Example |
|------------|-----------|---------|
| `enum` | string, integer | `enum: [clothing, electronics]` |
| `pattern` | string | `pattern: "^[A-Z]{2}$"` |
| `minimum` | number, integer | `minimum: 0` |
| `maximum` | number, integer | `maximum: 100` |
| `default` | any | `default: "USD"` |
| `aliases` | any | `aliases: [productId, product-id]` |
| `required` | any | `required: true` |

## Directory layout

Specs are organized as:
```
specs/
└── <namespace>/
    └── <event_name>/
        ├── 1-0-0.yaml
        └── 2-0-0.yaml   # newer version
```

Multiple versions of the same event can coexist. The registry keeps all versions; codegen defaults to the latest active version.

## Full field reference

See [Spec Reference — Event](../spec-reference/event.md) for the complete field documentation.
