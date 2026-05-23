---
sidebar_position: 3
---

# validate

Validate event specs, sources, destinations, and workspace config against their JSON Schemas.

## Synopsis

```
event-spec validate [spec-dir] [flags]
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--strict` | `false` | Exit 1 if any deprecated or deleted events are found |

## Behavior

**Without arguments** — validates the entire workspace using `event-spec.yaml` for directory config:

```bash
event-spec validate
```

Validates in order:
1. `event-spec.yaml` workspace config (if present)
2. All event spec YAML files under `specs_dir`
3. All source definitions under `sources_dir`
4. All destination definitions under `destinations_dir`

**With a `spec-dir` argument** — validates only event specs in that directory (backward-compatible mode):

```bash
event-spec validate ./specs
event-spec validate ./specs/ecommerce
```

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | All valid |
| `1` | One or more validation errors, or strict mode warnings |

## Output

Errors and warnings are written to stderr, one per line:

```
error: specs/ecommerce/product_viewed/1-0-0.yaml: property "category" enum is empty
warning: specs/auth/user_logged_out/2-0-0.yaml: status "deprecated"
```

On success:
```
validated 12 event spec(s), 3 source(s), 2 destination(s): ok
```

## CI usage

```bash
# Fail on validation errors or deprecated/deleted events
event-spec validate --strict
```

The GitHub Actions workflow at `.github/workflows/validate-specs.yml` runs this automatically on pushes that modify spec files.
