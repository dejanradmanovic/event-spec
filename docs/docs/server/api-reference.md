---
sidebar_position: 4
---

# API Reference

Base URL: `http(s)://<server-host>:<port>`

All endpoints except `GET /v1/health` and `POST /v1/admin/keys` (bootstrap only) require:

```http
Authorization: Bearer <api-key>
```

Responses are JSON. Errors follow the shape:

```json
{ "error": "description of what went wrong" }
```

---

## Health

### GET /v1/health

Returns server status and uptime. No authentication required.

**Response `200`**

```json
{ "status": "ok", "uptime": "3h42m10s" }
```

---

## Event specs

### GET /v1/events

List all published event specs. Requires `viewer`.

**Query parameters**

| Parameter | Description |
|-----------|-------------|
| `namespace` | Filter by namespace |
| `status` | Filter by status: `active`, `deprecated`, `deleted` |

**Response `200`** — array of event spec objects.

---

### GET /v1/events/\{namespace\}/\{name\}

Get the latest version of an event spec. Requires `viewer`.

**Response `200`** — event spec object. `404` if not found.

---

### GET /v1/events/\{namespace\}/\{name\}/\{version\}

Get a specific version of an event spec. Requires `viewer`.

Version format: `MAJOR-MINOR-PATCH` (e.g. `1-0-0`).

**Response `200`** — event spec object. `404` if not found.

---

### POST /v1/events

Publish a new event spec version. Requires `publisher`.

**Request body** — event spec YAML/JSON object.

**Response `201`** on success.

---

### GET /v1/diff/\{namespace\}/\{name\}/\{from\}/\{to\}

Diff two event spec versions and return a list of changes. Requires `viewer`.

**Response `200`**

```json
[
  { "kind": "MAJOR", "field": "properties", "description": "required property 'price' removed" }
]
```

---

## Analytics relay

All relay endpoints require `viewer` role and return `202 Accepted` on success. The `source` field in every request body identifies which app is sending the event; the server resolves its configured destinations and routes accordingly.

See [Analytics Relay](./relay.md) for setup, routing, and server-side hook details.

### POST /v1/track

Track a named event for a user or anonymous session.

**Request body**

```json
{
  "source": "web-app",
  "event_name": "product_viewed",
  "properties": { "product_id": "SKU-123" },
  "context": {
    "user_id": "user-456",
    "anonymous_id": "anon-789",
    "attributes": { "app_version": "2.1.0" }
  },
  "timestamp": "2024-01-15T12:00:00Z"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `source` | Yes | Registered source name |
| `event_name` | Yes | Event name matching a spec |
| `properties` | No | Event properties |
| `context` | No | User identity and attributes |
| `timestamp` | No | Event time (defaults to server receipt time) |

**Response `202 Accepted`**

---

### POST /v1/identify

Associate a user ID with a set of traits.

**Request body**

```json
{
  "source": "web-app",
  "user_id": "user-456",
  "traits": { "email": "user@example.com", "plan": "pro" },
  "context": { "anonymous_id": "anon-789" }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `source` | Yes | Registered source name |
| `user_id` | No | Authenticated user identifier |
| `traits` | No | User attributes to associate |
| `context` | No | Additional identity context |

**Response `202 Accepted`**

---

### POST /v1/group

Associate a user with a group (account, organisation, etc.).

**Request body**

```json
{
  "source": "web-app",
  "group_id": "acme-corp",
  "traits": { "name": "Acme Corp", "industry": "retail" },
  "context": { "user_id": "user-456" }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `source` | Yes | Registered source name |
| `group_id` | Yes | Group identifier |
| `traits` | No | Group attributes |
| `context` | No | User identity context |

**Response `202 Accepted`**

---

### POST /v1/page

Record a page or screen view.

**Request body**

```json
{
  "source": "web-app",
  "name": "Product Detail",
  "properties": { "url": "/products/SKU-123", "referrer": "/search" },
  "context": { "user_id": "user-456" }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `source` | Yes | Registered source name |
| `name` | Yes | Page or screen name |
| `properties` | No | Page properties |
| `context` | No | User identity context |

**Response `202 Accepted`**

---

### POST /v1/alias

Merge two user identities (e.g. anonymous → authenticated).

**Request body**

```json
{
  "source": "web-app",
  "user_id": "user-456",
  "previous_id": "anon-789"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `source` | Yes | Registered source name |
| `user_id` | Yes | The new (canonical) user ID |
| `previous_id` | Yes | The ID to merge into `user_id` |

**Response `202 Accepted`**

---

### POST /v1/batch

Send a mix of event types in a single request. The top-level `context` provides default identity for all items; per-item `context` overrides it field by field.

**Request body**

```json
{
  "source": "web-app",
  "context": { "user_id": "user-456" },
  "events": [
    {
      "type": "track",
      "event_name": "checkout_started",
      "properties": { "cart_total": 99.00 }
    },
    {
      "type": "identify",
      "user_id": "user-456",
      "traits": { "ltv": 99.00 }
    }
  ]
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `source` | Yes | Registered source name |
| `context` | No | Default identity applied to all items |
| `events` | Yes | Array of batch items |

**Batch item fields**

| Field | Description |
|-------|-------------|
| `type` | Required. One of: `track`, `identify`, `group`, `page`, `alias` |
| `event_name` | Event name (track only) |
| `properties` | Event or page properties (track, page) |
| `traits` | User or group traits (identify, group) |
| `user_id` | User identifier (identify, alias) |
| `previous_id` | Previous user ID (alias) |
| `group_id` | Group identifier (group) |
| `name` | Page name (page) |
| `context` | Per-item identity override |
| `timestamp` | Per-item event time |

**Response `202 Accepted`**

---

### POST /v1/flush

Force delivery of any buffered events for a source. Omit `source` to flush all sources.

**Request body**

```json
{ "source": "web-app" }
```

| Field | Required | Description |
|-------|----------|-------------|
| `source` | No | Source to flush. Omit to flush all. |

**Response `202 Accepted`**

---

## API keys

### POST /v1/admin/keys

Create an API key. No auth required when the server has zero keys (bootstrap); requires `admin` otherwise.

**Request body**

```json
{
  "role": "publisher",
  "name": "web-app-prod",
  "expires_in": "90d"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `role` | Yes | `viewer`, `publisher`, or `admin` |
| `name` | No | Human-readable label |
| `expires_in` | No | Duration: `24h`, `7d`, `90d`, `1y` |

**Response `201`**

```json
{ "id": 1, "key": "a3f8c2...", "role": "publisher" }
```

The raw key is returned once and never stored.

---

### GET /v1/admin/keys

List all API keys. Requires `admin`. Raw key values are never returned.

---

### DELETE /v1/admin/keys/\{id\}

Revoke an API key immediately. Requires `admin`. Returns `204 No Content`.

---

## Sources (apps)

### GET /v1/admin/sources

List all registered sources. Requires `admin`.

---

### POST /v1/admin/sources

Create a source. Requires `admin`.

**Request body** — source definition object:

```json
{
  "name": "web-app",
  "platform": "web",
  "language": "typescript",
  "events": ["ecommerce/**"],
  "destinations": ["amplitude-prod"]
}
```

**Response `201`** on success.

---

### GET /v1/admin/sources/\{name\}

Get a source by name. Requires `admin`. Returns `404` if not found.

---

### PUT /v1/admin/sources/\{name\}

Update a source. Requires `admin`. Full replacement — supply all fields.

---

### DELETE /v1/admin/sources/\{name\}

Delete a source. Requires `admin`. Returns `204 No Content`.

---

## Destinations

### GET /v1/admin/destinations

List all registered destinations. Requires `admin`.

---

### POST /v1/admin/destinations

Create a destination. Requires `admin`.

**Request body** — destination definition object:

```json
{
  "name": "amplitude-prod",
  "provider": "amplitude",
  "config": { "api_key": "amp-key-here" }
}
```

**Response `201`** on success.

---

### GET /v1/admin/destinations/\{name\}

Get a destination by name. Requires `admin`. Returns `404` if not found.

---

### PUT /v1/admin/destinations/\{name\}

Update a destination. Requires `admin`. Full replacement — supply all fields.

---

### DELETE /v1/admin/destinations/\{name\}

Delete a destination. Requires `admin`. Returns `204 No Content`.

---

## Audit log

### GET /v1/audit

Query the server audit log. Requires `admin`.

**Query parameters**

| Parameter | Description |
|-----------|-------------|
| `since` | RFC3339 start time |
| `until` | RFC3339 end time |
| `entity` | Filter by entity type: `event`, `source`, `destination` |
| `user` | Filter by user/key ID |
| `limit` | Max entries to return (default `50`) |

---

## Webhooks

### POST /v1/webhooks

Register a webhook URL. Requires `admin`.

**Request body**

```json
{ "url": "https://hooks.example.com/event-spec" }
```

**Response `201`** on success. The server will `POST` a JSON payload to this URL whenever an event spec is published.

---

### GET /v1/webhooks

List registered webhooks. Requires `admin`.

---

### DELETE /v1/webhooks/\{id\}

Remove a webhook. Requires `admin`. Returns `204 No Content`.

---

## Configuration

### GET /v1/admin/config

Get all server configuration settings. Requires `admin`.

---

### PUT /v1/admin/config/\{key\}

Set a configuration value. Requires `admin`. Changes take effect immediately without a restart.

**Request body**

```json
{ "value": "false" }
```

See [Configuration](./configuration.md) for supported keys.

---

## Source pull

### GET /v1/sources/\{name\}/pull

Fetch a source's full event spec bundle for local caching (used by `event-spec pull` in server mode). Requires `viewer`.
