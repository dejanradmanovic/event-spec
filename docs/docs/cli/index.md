---
sidebar_position: 1
---

# CLI Overview

The `event-spec` CLI is the primary tool for managing the analytics event lifecycle — from scaffolding and validation to codegen, auditing, and running the registry server.

## Installation

See [Installation](../getting-started/installation.md#cli).

## Global behavior

The CLI looks for `event-spec.yaml` in the current directory and walks up to the repository root. If found, it uses the configured registry mode, directories, and workspace name. If not found, it falls back to sensible defaults (`./specs`, `./sources`, `./destinations`, local mode).

## Commands

| Command | Description |
|---------|-------------|
| [`new`](./new.md) | Scaffold a new event spec YAML file |
| [`validate`](./validate.md) | Validate event specs, sources, destinations, and workspace config |
| [`diff`](./diff.md) | Compare two event spec versions and detect breaking changes |
| [`generate`](./generate.md) | Generate typed wrappers for Go and TypeScript |
| [`pull`](./pull.md) | Pull event specs from a remote git registry |
| [`audit`](./audit.md) | Scan a codebase for event usage against the spec registry |
| [`docs`](./docs.md) | Generate an HTML or Markdown event catalog |
| [`serve`](./serve.md) | Start the registry HTTP server |
| [`publish`](./publish.md) | Publish an event spec to the registry server |
| [`admin`](./admin.md) | Manage a running registry server (keys, apps, destinations, webhooks, config) |

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Business logic failure (validation errors, breaking changes, coverage below threshold) |
| `2` | Parse or configuration error |

## CI integration

For GitHub Actions, use the composite actions published from this repository — they install the CLI automatically and provide built-in PR comments and annotations:

```yaml title=".github/workflows/ci.yml"
jobs:
  validate:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
      - uses: dejanradmanovic/event-spec/validate@main
        with:
          strict: true
```

For GitLab CI, CircleCI, Bitbucket Pipelines, or plain `docker run`, use the pre-built CLI image:

```bash
docker run --rm -v "$PWD:/workspace" -w /workspace \
  ghcr.io/dejanradmanovic/event-spec-cli:latest validate --strict
```

→ See [CI Integrations](../ci-integrations/index.md) for full documentation including all action inputs, copy-paste workflow recipes, and multi-CI Docker examples.
