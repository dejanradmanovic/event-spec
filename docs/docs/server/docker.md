---
sidebar_position: 7
---

# Docker deployment

The registry server ships as a pre-built Docker image published to GitHub Container Registry on every release.

**Image:** `ghcr.io/dejanradmanovic/event-spec-server`

| Tag | Description |
|-----|-------------|
| `latest` | Most recent release |
| `v0.3.1` | Specific semver release |

Multi-arch: `linux/amd64` + `linux/arm64`. The image is based on `alpine:3` and includes `ca-certificates` and `git`.

---

## Quick start — SQLite

For development and single-node deployments:

```bash
docker run -p 8080:8080 \
  -v "$PWD/data:/data" \
  ghcr.io/dejanradmanovic/event-spec-server:latest \
  --db file:/data/registry.db
```

The database schema is applied automatically on first startup. Open `http://localhost:8080/ui/` to verify the server is running.

---

## Bootstrap — first API key

On a fresh database with no keys, the first key can be created without authentication.

**Via API:**

```bash
curl -X POST http://localhost:8080/v1/admin/keys \
  -H "Content-Type: application/json" \
  -d '{"role": "admin", "name": "bootstrap"}'
```

Response:

```json
{
  "id": 1,
  "key": "a3f8c2...",
  "role": "admin"
}
```

**Via admin UI:**

Open `http://localhost:8080/ui/` in a browser. On the first launch the interface presents a bootstrap screen to create the initial admin key.

The raw key value is returned **once** — save it immediately. Only its SHA-256 hash is persisted. If it is lost, it cannot be recovered; revoke it and create a new one.

---

## Docker Compose — SQLite (development)

```yaml title="docker-compose.yml"
services:
  registry:
    image: ghcr.io/dejanradmanovic/event-spec-server:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
    command: ["--db", "file:/data/registry.db"]
    restart: unless-stopped
```

```bash
docker compose up -d
```

---

## Docker Compose — PostgreSQL (production)

```yaml title="docker-compose.yml"
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: event_spec
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: event_spec
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U event_spec"]
      interval: 5s
      timeout: 3s
      retries: 5

  registry:
    image: ghcr.io/dejanradmanovic/event-spec-server:latest
    ports:
      - "8080:8080"
    depends_on:
      db:
        condition: service_healthy
    environment:
      POSTGRES_USER: event_spec
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: event_spec
    command:
      - "--db"
      - "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@db:5432/${POSTGRES_DB}"
    restart: unless-stopped

volumes:
  pgdata:
```

:::tip Credentials in the `--db` flag
Shell variable expansion (`${VAR}`) works in the `command` array when the variables are defined in the `environment` block of the same service.
:::

---

## `--db` flag reference

| DSN format | Description |
|-----------|-------------|
| `file:/data/registry.db` | SQLite file at an absolute path |
| `file:./registry.db` | SQLite file relative to the working directory |
| `postgres://user:pass@host:5432/dbname` | PostgreSQL connection string |
| `postgres://user:pass@host:5432/dbname?sslmode=require` | PostgreSQL with TLS |

The schema is applied automatically on startup. No manual migration steps are needed.

---

## Digest pinning

For reproducible production deployments, pin to an image digest rather than a mutable tag:

```yaml
# docker-compose.yml
services:
  registry:
    image: ghcr.io/dejanradmanovic/event-spec-server@sha256:<digest>
```

Find the digest for a tagged release:

```bash
docker pull ghcr.io/dejanradmanovic/event-spec-server:v0.3.1
docker inspect --format='{{index .RepoDigests 0}}' \
  ghcr.io/dejanradmanovic/event-spec-server:v0.3.1
```

---

## Next steps

- [Authentication](./authentication.md) — API key roles, creating keys, and RBAC
- [Analytics Relay](./relay.md) — sending events from applications through the server
- [API Reference](./api-reference.md) — full REST API documentation
- [Configuration](./configuration.md) — all server flags and environment variables
