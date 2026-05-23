---
sidebar_position: 1
---

# Server Overview

The event-spec server is a self-hosted REST API that acts as the central hub for your analytics infrastructure. It stores event specs, manages access control, and — most importantly — acts as an **analytics relay** so that no application ever needs to know which providers you use.

## When to use the server

| Scenario | Recommended mode |
|----------|-----------------|
| Single repo, small team | [Local registry](../registry/local.md) |
| Specs in a separate git repo | [Git registry](../registry/git.md) |
| Multiple apps, shared specs, centralized delivery | **Server** |

The server mode is the right choice once you have more than one application sending analytics, or once you want to centralize provider credentials and validation.

## Starting the server

```bash
# SQLite — development and single-node deployments
event-spec serve --port 8080 --db file:./registry.db

# PostgreSQL — production and multi-instance deployments
event-spec serve --port 8080 --db "postgres://user:pass@localhost:5432/event_spec"
```

The database schema is applied automatically on first startup. See [`event-spec serve`](../cli/serve.md) for all flags.

## Bootstrap

On a fresh server with no API keys, the first key can be created without authentication — either via the API or by opening the admin UI in a browser:

```bash
# Via API
curl -X POST http://localhost:8080/v1/admin/keys \
  -H "Content-Type: application/json" \
  -d '{"role": "admin", "name": "bootstrap"}'
```

```
# Via browser
http://localhost:8080/ui/
```

On the first UI launch the interface presents a bootstrap screen to create the initial admin key. Store the returned key — all subsequent key creation requires an admin token.

## Architecture

```mermaid
flowchart TB
    subgraph apps["Your Applications"]
        web["Web App"]
        mobile["Mobile App"]
        backend["Backend Service"]
    end

    subgraph server["Registry Server"]
        relay["Analytics Relay\n(validate → sample → route)"]
        registry["Event Registry\n(specs, sources, destinations)"]
        admin["Admin API\n(keys, config, audit)"]
    end

    subgraph providers["Analytics Providers"]
        amp["Amplitude"]
        seg["Segment"]
        other["..."]
    end

    web -->|"POST /v1/track"| relay
    mobile -->|"POST /v1/batch"| relay
    backend -->|"POST /v1/identify"| relay
    relay --> amp
    relay --> seg
    relay --> other
    registry -.->|"spec lookup"| relay
```