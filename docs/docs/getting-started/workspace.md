---
sidebar_position: 4
---

# Workspace Config

`event-spec.yaml` is the root configuration file for your project. The CLI looks for it in the current directory by default.

## Full reference

```yaml title="event-spec.yaml"
version: 1                      # config schema version, always 1
workspace: "my-company"         # logical workspace name, used in generated package names

registry:
  mode: local                   # local | git | server
  # git mode only:
  # url: https://github.com/my-org/analytics-specs.git
  # ref: main
  # server mode only:
  # url: https://registry.example.com
  # api_key: ${REGISTRY_API_KEY}

specs_dir: ./specs              # directory tree of event spec YAML files
sources_dir: ./sources          # directory of source definition YAML files
destinations_dir: ./destinations # directory of destination definition YAML files

# audit:
#   path: ./src                 # directory to scan with event-spec audit (default: .)
#   coverage_min: 80            # minimum coverage % — 0 disables the check
#   report: text                # report format: text | json | html
```

## Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `version` | integer | — | Config schema version. Must be `1`. |
| `workspace` | string | — | Logical workspace name, used in generated package/module names. |
| `registry.mode` | string | `local` | Registry backend: `local`, `git`, or `server`. |
| `registry.url` | string | — | Git repo URL (git mode) or server base URL (server mode). |
| `registry.ref` | string | `main` | Git branch, tag, or commit (git mode only). |
| `registry.cache_dir` | string | `~/.event-spec/cache/<hash>` | Local clone path for git mode. |
| `registry.api_key` | string | — | API key for server mode. Supports `${ENV_VAR}` expansion. |
| `specs_dir` | string | `./specs` | Path to the event spec YAML directory tree. |
| `sources_dir` | string | `./sources` | Path to the source definition directory. |
| `destinations_dir` | string | `./destinations` | Path to the destination definition directory. |
| `audit.path` | string | `.` | Default scan path for `event-spec audit`. |
| `audit.coverage_min` | number | `0` | Minimum coverage percentage (0 = disabled). |
| `audit.report` | string | `text` | Default audit report format: `text`, `json`, or `html`. |

## Registry modes

| Mode | When to use |
|------|-------------|
| `local` | Specs live in this repo (monorepo or small team). No network required. Supports fsnotify hot-reload. |
| `git` | Specs live in a separate git repository. Use `event-spec pull` to sync the cache locally. |
| `server` | Specs are managed by a running registry server. Use `event-spec serve` to start one. |

See [Registry — Local](../registry/local.md), [Registry — Git](../registry/git.md), and [Registry — Server](../registry/server.md) for setup details.

## CLI fallback

If `event-spec.yaml` is absent the CLI falls back to:
- `specs_dir`: `./specs`
- `sources_dir`: `./sources`
- `destinations_dir`: `./destinations`
- Registry mode: `local`

This means simple projects with the conventional layout can skip the config file entirely.
