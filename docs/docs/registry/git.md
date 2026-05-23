---
sidebar_position: 2
---

# Git Registry

The git registry fetches event specs from a remote git repository. Useful when analytics specs are maintained in a separate repository shared across multiple apps.

## Setup

```yaml title="event-spec.yaml"
version: 1
workspace: "my-company"
registry:
  mode: git
  url: https://github.com/my-org/analytics-specs.git
  ref: main
```

## Pulling specs

```bash
event-spec pull
```

This clones or fetches the remote repository into a local cache (`~/.event-spec/cache/` by default) and rebuilds the in-memory index.

See [CLI — pull](../cli/pull.md) for the full command reference.

After pulling, all CLI commands work against the cached specs — no network required:

```bash
event-spec pull && event-spec generate web-app
```

## Version pinning

Source configs can pin individual events to a specific version:

```yaml title="sources/web-app.yaml"
name: web-app
platform: web
language: typescript
events:
  - ecommerce/**
version_pinning:
  ecommerce/product_viewed: "1-0-0"   # pin to v1 despite v2 existing
output:
  path: ./src/analytics/generated
```

This is useful during migrations — you can test against a new version in a branch source while keeping `main` pinned.

## Authentication

For private repositories, configure git credentials using standard git mechanisms:

```bash
# SSH (recommended)
git config --global url."git@github.com:".insteadOf "https://github.com/"

# HTTPS token
git config --global url."https://<token>@github.com/".insteadOf "https://github.com/"
```

The CLI inherits your git credential configuration — no special event-spec config is needed.

## Cache location

The local cache defaults to `~/.event-spec/cache/<repo-hash>/`. Override it with `cache_dir` in your workspace config:

```yaml title="event-spec.yaml"
registry:
  mode: git
  url: https://github.com/my-org/analytics-specs.git
  ref: main
  cache_dir: ./.event-spec-cache   # repo-local cache, useful for CI
```

You can safely delete the cache directory and re-run `event-spec pull` to rebuild.

## CI usage

In CI, pull before codegen and validate:

```yaml
- run: event-spec pull
- run: event-spec validate --strict
- run: event-spec generate web-app
```
