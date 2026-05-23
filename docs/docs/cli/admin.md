---
sidebar_position: 10
---

# admin

Manage a running registry server from the command line. All subcommands connect to the server configured in `event-spec.yaml` (`registry.mode: server`).

## Prerequisites

Your workspace must be configured for server mode:

```yaml title="event-spec.yaml"
registry:
  mode: server
  url: https://registry.example.com
  api_key: "${REGISTRY_API_KEY}"
```

## Synopsis

```
event-spec admin <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|-----------|-------------|
| [`status`](#status) | Print server status and uptime |
| [`keys`](#keys) | Create, list, and revoke API keys |
| [`audit`](#audit) | Query the server audit log |
| [`webhooks`](#webhooks) | Add, list, and remove webhook endpoints |
| [`config`](#config) | Get and set server configuration |
| [`apps`](#apps) | Manage app (source) registrations |
| [`destinations`](#destinations) | Manage destination (provider) configurations |

---

## status

Print the server's current status and uptime.

```bash
event-spec admin status
```

---

## keys

Manage API keys for server access.

### keys create

```
event-spec admin keys create --role <role> [--name <name>] [--expires <duration>]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--role` | Yes | Key role: `viewer`, `publisher`, or `admin` |
| `--name` | No | Human-readable label for the key |
| `--expires` | No | Expiry duration, e.g. `24h`, `7d`, `90d` |

```bash
# Read-only key for a dashboard
event-spec admin keys create --role viewer --name "grafana-dashboard"

# Publisher key that expires in 90 days
event-spec admin keys create --role publisher --name "web-app" --expires 90d

# Long-lived admin key
event-spec admin keys create --role admin --name "ci-pipeline"
```

The created key value is printed once — store it immediately.

### keys list

```
event-spec admin keys list [--format <format>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |

### keys revoke

```
event-spec admin keys revoke <key-id>
```

Immediately revokes the key. Any request using the revoked key will receive `401 Unauthorized`.

---

## audit

Query the server audit log.

```
event-spec admin audit [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | — | Start time (RFC3339), e.g. `2024-01-01T00:00:00Z` |
| `--until` | — | End time (RFC3339) |
| `--entity` | — | Filter by entity type: `event`, `source`, or `destination` |
| `--user` | — | Filter by user/key that performed the action |
| `--format` | `text` | Output format: `text` or `json` |
| `--limit` | — | Maximum number of entries to return |

```bash
# All entries from the past day
event-spec admin audit --since 2024-01-01T00:00:00Z

# Recent event publishes in JSON
event-spec admin audit --entity event --format json --limit 50
```

---

## webhooks

Manage webhook endpoints that receive server-side event notifications.

### webhooks add

```
event-spec admin webhooks add <url>
```

```bash
event-spec admin webhooks add https://hooks.example.com/event-spec
```

### webhooks list

```
event-spec admin webhooks list [--format <format>]
```

### webhooks remove

```
event-spec admin webhooks remove <id>
```

---

## config

Get and set server runtime configuration.

### config get

```
event-spec admin config get [--format <format>]
```

Prints the current server configuration. Use `--format json` for machine-readable output.

### config set

```
event-spec admin config set <key> <value>
```

| Key | Values | Description |
|-----|--------|-------------|
| `hooks_enabled` | `true` \| `false` | Enable or disable server-side hook execution |

```bash
event-spec admin config set hooks_enabled false
```

---

## apps

Manage app registrations. Apps are named clients that group API keys and event subscriptions.

App definitions are YAML files:

```yaml title="apps/web-app.yaml"
name: web-app
platform: web
language: typescript
events:
  - ecommerce/**
```

### apps list

```
event-spec admin apps list
```

### apps get

```
event-spec admin apps get <name>
```

### apps create

```
event-spec admin apps create <file.yaml>
```

```bash
event-spec admin apps create apps/web-app.yaml
```

### apps update

```
event-spec admin apps update <file.yaml>
```

### apps delete

```
event-spec admin apps delete <name> [--yes]
```

Pass `--yes` to skip the confirmation prompt.

---

## destinations

Manage destination configurations. Destinations define analytics provider credentials and settings.

Destination definitions are YAML files:

```yaml title="destinations/amplitude.yaml"
name: amplitude
provider: amplitude
config:
  api_key: "${AMPLITUDE_API_KEY}"
```

### destinations list

```
event-spec admin destinations list
```

### destinations get

```
event-spec admin destinations get <name>
```

### destinations create

```
event-spec admin destinations create <file.yaml>
```

```bash
event-spec admin destinations create destinations/amplitude.yaml
```

### destinations update

```
event-spec admin destinations update <file.yaml>
```

### destinations delete

```
event-spec admin destinations delete <name> [--yes]
```

Pass `--yes` to skip the confirmation prompt.

---

## See also

- [Registry — Server](../registry/server.md) for deployment, relay, and configuration details.
- [`event-spec serve`](./serve.md) for starting the server.
