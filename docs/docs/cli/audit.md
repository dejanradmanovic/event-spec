---
sidebar_position: 7
---

# audit

Scan a codebase and report which events from the spec registry are actually used in the code.

## Synopsis

```
event-spec audit [source] [flags]
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--path` | `.` | Directory to scan. Falls back to `audit.path` in `event-spec.yaml`. |
| `--strict` | `false` | Exit 1 if any required events are unused |
| `--coverage-min` | `0` | Minimum coverage percentage required (0 = disabled) |
| `--report` | `text` | Output format: `text` \| `json` \| `html` |

## Supported languages

| Language | Scanner |
|----------|---------|
| Go | AST-based (parses generated wrapper call sites) |
| TypeScript | AST-based (parses import and call expressions) |
| Swift | AST-based (planned) |

The scanner is language-aware and understands generated wrapper naming conventions so it doesn't require manual annotation.

## Usage

```bash
# Scan current directory, infer language from source config
event-spec audit

# Scan using a specific source's event list and language
event-spec audit web-app

# Custom path
event-spec audit --path ./src

# HTML report
event-spec audit --report html > audit-report.html

# CI enforcement: fail if coverage < 80%
event-spec audit --coverage-min 80 --strict
```

## Output (text)

```
Source: web-app  Language: typescript  Scanned: ./src

EVENT                          STATUS     CALLS   FILES
ecommerce/product_viewed       used       12      3
ecommerce/checkout_started     used       4       2
auth/user_signed_up            unused     0       0
auth/password_reset            used       2       1

Coverage: 75.0% (3/4 events used)
```

## Output (JSON)

```json
{
  "source": "web-app",
  "language": "typescript",
  "coverage_pct": 75.0,
  "events": [
    { "event_key": "ecommerce/product_viewed", "used": true, "call_count": 12, "files": ["src/pages/pdp.ts"] },
    { "event_key": "auth/user_signed_up", "used": false, "call_count": 0, "files": [] }
  ],
  "unused": [
    { "event_key": "auth/user_signed_up", "spec_file": "specs/auth/user_signed_up/1-0-0.yaml", "required": false }
  ]
}
```

## Workspace config

Audit defaults can be set in `event-spec.yaml`:

```yaml title="event-spec.yaml"
# ... other config ...
audit:
  path: ./src
  coverage_min: 80
  report: json
```

CLI flags override workspace config values.

## CI usage

```bash
# Fail if coverage drops below 80% or required events are unused
event-spec audit web-app --coverage-min 80 --strict
```
