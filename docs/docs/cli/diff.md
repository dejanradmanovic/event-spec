---
sidebar_position: 4
---

# diff

Compare two event spec versions and validate that the declared SchemaVer bump matches the detected changes.

## Synopsis

```
event-spec diff [from.yaml to.yaml]
event-spec diff <ns/name> [from-ver to-ver]
event-spec diff [--source <name>]
```

## Modes

### Mode 1 — Explicit file paths (no registry required)

```bash
event-spec diff specs/ecommerce/product_viewed/1-0-0.yaml \
               specs/ecommerce/product_viewed/2-0-0.yaml
```

### Mode 2 — Registry-aware with explicit versions

```bash
event-spec diff ecommerce/product_viewed 1-0-0 2-0-0
```

### Mode 3 — Diff the two latest active versions

```bash
event-spec diff ecommerce/product_viewed
```

### Mode 4 — All events from workspace source configs

```bash
# All sources
event-spec diff

# Single source
event-spec diff --source web-app
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--breaking` | `false` | Show only breaking changes |
| `--format` | `text` | Output format: `text` \| `json` |
| `--source` | `""` | Source name (mode 4) |

## Output

### Text format

```
BREAKING   property_removed      currency
BREAKING   type_changed          category: string → integer
OK         property_added        description

Version: declared 2-0-0, required 2-0-0 — ok
```

### JSON format

```json
{
  "namespace": "ecommerce",
  "name": "product_viewed",
  "from_version": "1-0-0",
  "to_version": "2-0-0",
  "changes": [
    { "kind": "property_removed", "property": "currency", "breaking": true },
    { "kind": "type_changed", "property": "category", "breaking": true, "from": "string", "to": "integer" }
  ],
  "version_valid": true,
  "required_version": "2-0-0"
}
```

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | No inconsistencies |
| `1` | Version bump is inconsistent with detected changes |
| `2` | Parse or configuration error |

## Breaking change rules

| Change | Breaking |
|--------|---------|
| Remove a property | Yes |
| Add a required property | Yes |
| Rename a property | Yes |
| Change property type | Yes |
| Make optional → required | Yes |
| Remove enum value | Yes |
| Add optional property | No |
| Make required → optional | No |
| Add enum value | No |
| Description-only change | No |

## CI usage

```bash
# Fail CI if any breaking changes are declared with an incorrect version bump
event-spec diff --format json --breaking
```
