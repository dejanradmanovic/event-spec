---
sidebar_position: 3
---

# Authentication

All API endpoints (except `GET /v1/health` and the bootstrap key creation) require a Bearer token:

```http
Authorization: Bearer <api-key>
```

## Roles

Every API key carries one of three roles. Role access is hierarchical — a higher role satisfies any requirement for a lower role.

| Role | Level | Can do |
|------|-------|--------|
| `viewer` | 1 | Read events, sources, destinations. Send relay events. |
| `publisher` | 2 | Everything `viewer` can do, plus publish event specs. |
| `admin` | 3 | Everything `publisher` can do, plus manage keys, apps, destinations, webhooks, config, and audit log. |

## Creating keys

### Bootstrap (first key)

On a fresh server with no keys, the first key can be created without authentication — either via the API or through the admin UI. On the first UI launch (`http://localhost:8080/ui/`) the interface presents a bootstrap screen to create the initial admin key.

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

The raw key value is returned **once** — store it immediately. Only its SHA-256 hash is persisted.

### Subsequent keys

Once any key exists, creating further keys requires an `admin` token:

```bash
# Long-lived publisher key for a web app
event-spec admin keys create --role publisher --name "web-app-prod"

# Viewer key that expires in 90 days
event-spec admin keys create --role viewer --name "grafana" --expires 90d

# Admin key for CI
event-spec admin keys create --role admin --name "ci-pipeline" --expires 1y
```

The `--expires` flag accepts Go duration strings (`24h`, `7d`) as well as `Nd` (days) and `Ny` (years).

### Via the API directly

```bash
curl -X POST http://localhost:8080/v1/admin/keys \
  -H "Authorization: Bearer <admin-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "role": "publisher",
    "name": "web-app-prod",
    "expires_in": "90d"
  }'
```

## Listing keys

```bash
event-spec admin keys list
```

Or via the API:

```bash
curl http://localhost:8080/v1/admin/keys \
  -H "Authorization: Bearer <admin-key>"
```

## Revoking keys

```bash
event-spec admin keys revoke <key-id>
```

Or via the API:

```bash
curl -X DELETE http://localhost:8080/v1/admin/keys/<id> \
  -H "Authorization: Bearer <admin-key>"
```

Revocation is immediate — any request using the revoked key receives `401 Unauthorized`.

## Key storage

Keys are stored as SHA-256 hashes. The server never stores the raw key value. If a key is lost it cannot be recovered — revoke it and create a new one.

## Assigning keys to apps

Each application should have its own key with the minimum required role. Relay-only apps (sending events) need `viewer` or `publisher`. Apps that publish event specs need `publisher`. Only CI/admin tooling should hold `admin` keys.

See [`event-spec admin`](../cli/admin.md) for the full key management CLI reference.
