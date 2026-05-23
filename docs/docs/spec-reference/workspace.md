---
sidebar_position: 4
---

# Workspace Config Reference

`event-spec.yaml` is the root configuration file for a project. See [Getting Started — Workspace](../getting-started/workspace.md) for an introduction.

## Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `version` | integer | Yes | — | Config schema version. Always `1`. |
| `workspace` | string | Yes | — | Logical workspace name. Used in generated package/module names and registry grouping. |
| `registry.mode` | string | No | `local` | Registry backend: `local` \| `git` \| `server`. |
| `registry.url` | string | Conditional | — | Git repo URL (git mode) or server base URL (server mode). |
| `registry.ref` | string | No | `main` | Git branch/tag/commit (git mode only). |
| `registry.cache_dir` | string | No | `~/.event-spec/cache/<hash>` | Local clone path for git mode. Override for CI or monorepo layouts. |
| `registry.api_key` | string | Conditional | — | API key for server mode. Supports `${ENV_VAR}` expansion. |
| `specs_dir` | string | No | `./specs` | Path to event spec YAML directory tree. |
| `sources_dir` | string | No | `./sources` | Path to source definition directory. |
| `destinations_dir` | string | No | `./destinations` | Path to destination definition directory. |
| `audit.path` | string | No | `.` | Default scan path for `event-spec audit`. |
| `audit.coverage_min` | number | No | `0` | Default minimum coverage percentage. |
| `audit.report` | string | No | `text` | Default audit report format. |

## Examples

### Local mode (simplest)

```yaml
version: 1
workspace: "my-company"
registry:
  mode: local
specs_dir: ./specs
sources_dir: ./sources
destinations_dir: ./destinations
```

### Git mode

```yaml
version: 1
workspace: "my-company"
registry:
  mode: git
  url: https://github.com/my-org/analytics-specs.git
  ref: main
```

### Server mode

```yaml
version: 1
workspace: "my-company"
registry:
  mode: server
  url: https://registry.example.com
  api_key: "${REGISTRY_API_KEY}"
```

### With audit defaults

```yaml
version: 1
workspace: "my-company"
registry:
  mode: local
specs_dir: ./specs
sources_dir: ./sources
destinations_dir: ./destinations
audit:
  path: ./src
  coverage_min: 80
  report: json
```
