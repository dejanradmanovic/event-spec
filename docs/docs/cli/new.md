---
sidebar_position: 2
---

# new

Scaffold a new event spec YAML file with the required boilerplate.

## Synopsis

```
event-spec new [<namespace/event_name>] [flags]
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | `track` | Event type: `track` \| `page` \| `identify` \| `group` \| `alias` |
| `--status` | `draft` | Initial status: `draft` \| `active` |
| `--owner` | `""` | Team or person responsible for this event |
| `--description` | `""` | Short description written into the generated file |
| `--display-name` | `""` | Human-readable display name (defaults to title-cased event name) |
| `--workspace` | `""` | Path to `event-spec.yaml` (defaults to auto-discovery) |

## Usage

### Interactive mode

Running `new` without arguments launches an interactive prompt:

```bash
event-spec new
```

The prompt walks through namespace, event name, type, and owner.

### Non-interactive

```bash
event-spec new ecommerce/product_viewed
# Creates: specs/ecommerce/product_viewed/1-0-0.yaml
```

```bash
event-spec new ecommerce/product_viewed \
  --type track \
  --status draft \
  --owner "growth-team" \
  --description "Fired when a user views a product detail page"
```

## Generated output

```yaml title="specs/ecommerce/product_viewed/1-0-0.yaml"
$schema: "https://event-spec.io/schemas/event/v1"
name: product_viewed
display_name: "Product Viewed"
version: "1-0-0"
status: draft
namespace: ecommerce
type: track
event_name: "Product Viewed"
description: ""

properties:
  example_property:
    type: string
    required: true
    description: ""
```

Status is `draft` by default. Change to `active` when the spec is finalized and you're ready to enforce it in CI.

## Server mode

When the workspace is configured in `server` registry mode, `new` auto-publishes the scaffold to the registry after creating the local file:

```bash
event-spec new ecommerce/product_viewed
# Created: specs/ecommerce/product_viewed/1-0-0.yaml
# Published to registry: https://registry.example.com
```

## Git mode

In git mode, the file is written locally. A reminder is printed to commit and push the new file to the registry repository:

```
Created: specs/ecommerce/product_viewed/1-0-0.yaml
Don't forget to commit and push the new spec to the registry repository.
```

## Next steps after scaffolding

1. Edit the YAML to add your properties and constraints
2. Run `event-spec validate` to check for errors
3. Run `event-spec generate` to produce typed wrappers
4. Change `status: draft` → `status: active` when ready
