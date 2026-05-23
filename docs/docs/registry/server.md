---
sidebar_position: 3
---

# Server Registry

The server registry mode connects your workspace to a running event-spec server. The server stores event specs in a database, enforces access control, and acts as an analytics relay so that applications never need to know about providers directly.

## Workspace config

```yaml title="event-spec.yaml"
version: 1
workspace: "my-company"
registry:
  mode: server
  url: https://registry.example.com
  api_key: "${REGISTRY_API_KEY}"
```

The `api_key` field supports `${ENV_VAR}` expansion. A `viewer` role key is sufficient for read-only operations like `event-spec generate` and `event-spec pull`.

## Starting the server

```bash
event-spec serve --port 8080 --db file:./registry.db
```

See the [Server](../server/index.md) section for full deployment documentation, the analytics relay, API reference, and admin UI.
