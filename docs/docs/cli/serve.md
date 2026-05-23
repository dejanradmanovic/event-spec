---
sidebar_position: 9
---

# serve

Start the event-spec registry HTTP server. The server provides a REST API for event spec management, analytics ingestion, and a web admin UI.

## Synopsis

```
event-spec serve [flags]
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8080` | Listening port |
| `--db` | *(required)* | Database DSN: SQLite (`file:./registry.db`) or PostgreSQL (`postgres://...`) |

## Usage

```bash
# SQLite (development / small deployments)
event-spec serve --port 8080 --db file:./registry.db

# PostgreSQL (production)
event-spec serve --port 8080 --db "postgres://user:pass@localhost:5432/event_spec"

# With environment variables
DB_DSN=file:./registry.db event-spec serve
```

## Server features

- **Event registry API** — CRUD for event specs, sources, and destinations
- **Analytics relay** — `/track`, `/identify`, `/group`, `/page`, `/alias` endpoints for server-proxied ingestion
- **API key authentication** — all API endpoints require a valid API key
- **Web admin UI** — event catalog browser, audit log, key management
- **Admin CLI subcommands** — manage apps, destinations, keys, and webhooks without the UI

## API endpoints

| Method | Path | Role | Description |
|--------|------|------|-------------|
| `GET` | `/v1/health` | — | Server health and uptime |
| `GET` | `/v1/events` | viewer | List event specs |
| `GET` | `/v1/events/{namespace}/{name}` | viewer | Get latest event spec |
| `GET` | `/v1/events/{namespace}/{name}/{version}` | viewer | Get specific version |
| `POST` | `/v1/events` | publisher | Publish an event spec |
| `GET` | `/v1/diff/{namespace}/{name}/{from}/{to}` | viewer | Diff two versions |
| `POST` | `/v1/track` | viewer | Relay a track event |
| `POST` | `/v1/identify` | viewer | Relay an identify call |
| `POST` | `/v1/group` | viewer | Relay a group call |
| `POST` | `/v1/page` | viewer | Relay a page view |
| `POST` | `/v1/alias` | viewer | Relay an alias call |
| `POST` | `/v1/batch` | viewer | Relay a batch of events |
| `POST` | `/v1/flush` | viewer | Flush buffered events |
| `GET` | `/v1/admin/keys` | admin | List API keys |
| `POST` | `/v1/admin/keys` | — | Create an API key |
| `DELETE` | `/v1/admin/keys/{id}` | admin | Revoke an API key |
| `GET` | `/v1/admin/sources` | admin | List sources |
| `POST` | `/v1/admin/sources` | admin | Create a source |
| `GET` | `/v1/admin/destinations` | admin | List destinations |
| `POST` | `/v1/admin/destinations` | admin | Create a destination |
| `GET` | `/v1/admin/config` | admin | Get server config |
| `PUT` | `/v1/admin/config/{key}` | admin | Set a config value |

All endpoints except `POST /v1/admin/keys` and `GET /v1/health` require `Authorization: Bearer <api-key>`.

## Graceful shutdown

The server handles `SIGINT` and `SIGTERM` with a 30-second shutdown timeout — in-flight requests complete before the process exits.

## See also

- [Registry — Server](../registry/server.md) for full deployment and configuration details.
