---
sidebar_position: 3
---

# Destination Reference

Destination definitions declare an analytics provider configuration — the credentials, batching settings, and other parameters for a specific backend.

## File location

```
destinations/<destination_name>.yaml
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Machine-readable destination name. Referenced in source `destinations` lists. |
| `type` | string | Yes | Provider type: `amplitude` \| `noop` \| (future providers) |
| `config` | object | No | Provider-specific configuration. Keys depend on `type`. |

## Amplitude config

```yaml title="destinations/amplitude.yaml"
name: amplitude
type: amplitude
config:
  api_key: "${AMPLITUDE_API_KEY}"
  batch_size: 100
  flush_interval: "5s"
  max_queue_size: 10000
  proxy_url: "https://analytics.yourcompany.com/amp"
```

| Config key | Type | Description |
|-----------|------|-------------|
| `api_key` | string | Amplitude API key or `${ENV_VAR}` reference |
| `batch_size` | integer | Max events per batch HTTP request |
| `flush_interval` | duration | How often to flush the event queue |
| `max_queue_size` | integer | Max buffered events before overflow policy kicks in |
| `proxy_url` | string | Optional reverse proxy URL |

## Environment variable expansion

Any config value of the form `${VAR_NAME}` is expanded from the environment at runtime. Never store raw API keys in destination YAML files.
