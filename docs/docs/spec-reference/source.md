---
sidebar_position: 2
---

# Source Reference

Source definitions describe a consuming application — which events it tracks, which language it uses, and where generated code should be written.

## File location

```
sources/<source_name>.yaml
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Machine-readable source name. Used as the CLI argument to `generate`, `audit`, `diff`. |
| `platform` | string | No | Platform hint: `web`, `mobile`, `server`, `cli`. Informational. |
| `language` | string | Yes | Target language for codegen: `go` \| `typescript` \| `swift` \| `kotlin` |
| `events` | list[string] | No | Event patterns to include. Supports glob syntax. Empty = all events. |
| `destinations` | list[string] | No | Destination names this source sends to. Informational. |
| `output.path` | string | No | Output directory for generated files. Default: `./generated`. |
| `output.package` | string | No | Package / module name for generated files (TypeScript: npm scope, Go: module path). |
| `version_pinning` | map | No | Pin specific events to a version: `namespace/name: "1-0-0"`. |
| `audit.path` | string | No | Directory to scan with `event-spec audit`. Default: `.`. |
| `audit.coverage_min` | number | No | Minimum coverage percentage (0 = disabled). |
| `audit.report` | string | No | Default audit report format: `text` \| `json` \| `html`. |

## Event patterns

Patterns use glob syntax matching `namespace/event_name`:

```yaml
events:
  - ecommerce/**          # all events in the ecommerce namespace
  - auth/user_signed_up   # exact match
  - auth/*                # all events directly in auth (not nested)
```

## Complete example

```yaml title="sources/web-app.yaml"
name: web-app
platform: web
language: typescript
events:
  - ecommerce/**
  - auth/user_signed_up
  - auth/user_logged_out
destinations:
  - amplitude
output:
  path: ./src/analytics/generated
  package: "@my-company/analytics"
version_pinning:
  ecommerce/product_viewed: "1-0-0"
audit:
  path: ./src
  coverage_min: 80
  report: text
```
