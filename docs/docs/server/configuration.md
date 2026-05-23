---
sidebar_position: 5
---

# Configuration

## Database

The server requires a database DSN passed via the `--db` flag:

```bash
event-spec serve --db <dsn>
```

| Database | DSN format | Use case |
|----------|-----------|---------|
| SQLite | `file:./registry.db` | Development, single-node deployments |
| PostgreSQL | `postgres://user:pass@host:5432/dbname` | Production, multi-instance deployments |

The schema is applied automatically on first startup. You can safely stop the server, swap the binary, and restart — migrations are additive and applied automatically.

### PostgreSQL example

```bash
event-spec serve \
  --port 8080 \
  --db "postgres://event_spec:secret@db.internal:5432/event_spec"
```

### SQLite in CI

```bash
event-spec serve --db file:/tmp/registry.db &
```

## Server flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8080` | HTTP listen port |
| `--db` | *(required)* | Database DSN |

## Runtime configuration

Runtime settings are stored in the database and can be changed without restarting the server. Changes take effect immediately.

### hooks_enabled

Controls whether the validation and sampling hooks run on relay events.

| Value | Behavior |
|-------|---------|
| `true` (default) | Validation and sampling hooks run on every relay event |
| `false` | All relay events pass through to providers without hook processing |

```bash
# Disable hooks (useful during migrations or debugging)
event-spec admin config set hooks_enabled false

# Re-enable
event-spec admin config set hooks_enabled true
```

Via the API:

```bash
curl -X PUT http://localhost:8080/v1/admin/config/hooks_enabled \
  -H "Authorization: Bearer <admin-key>" \
  -H "Content-Type: application/json" \
  -d '{"value": "false"}'
```

The startup value is `true` unless `HooksDisabled: true` is set in the server Config struct. The database setting overrides the startup value, so the admin endpoint persists across restarts.

## Environment variables

The server does not read environment variables directly, but destination configs support `${VAR}` expansion for provider credentials:

```yaml title="destinations/amplitude.yaml"
config:
  api_key: "${AMPLITUDE_API_KEY}"
```

The variable is expanded when the destination is loaded from the database, using the server process's environment.
