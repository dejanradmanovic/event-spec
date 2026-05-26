---
sidebar_position: 1
---

# CI Integrations Overview

event-spec ships two ways to integrate into any CI pipeline: **composite GitHub Actions** that live in this repo and **pre-built Docker images** that work with any CI system that can run a container.

## Comparison

| | Composite Actions | Docker image |
|---|---|---|
| **Works with** | GitHub Actions | Any CI (GitLab, CircleCI, Bitbucket, plain `docker run`) |
| **Installation** | `uses: dejanradmanovic/event-spec/validate@main` | `image: ghcr.io/dejanradmanovic/event-spec-cli:latest` |
| **Auto-discovers `event-spec.yaml`** | ✅ | ✅ (mount repo as volume) |
| **PR comments / annotations** | ✅ (built in) | ❌ (set up separately) |
| **Pinnable to a version** | ✅ via `version` input | ✅ via image tag |
| **Digest pinning** | ✅ `uses: …@<sha>` | ✅ `image: …@sha256:<digest>` |

## Composite GitHub Actions

Three ready-to-use actions are published directly from this repository:

```yaml
uses: dejanradmanovic/event-spec/validate@main   # validate specs + detect breaking changes
uses: dejanradmanovic/event-spec/generate@main   # generate typed SDK wrappers
uses: dejanradmanovic/event-spec/audit@main      # coverage audit across all sources
```

All three auto-discover `event-spec.yaml` from the repository root, read source definitions from it, and operate across all defined sources by default. No separate setup action is needed — the CLI is installed inside each action.

→ See [GitHub Actions](./github-actions.md) for full input tables and copy-paste workflow recipes.

## Docker images

Two images are published to GitHub Container Registry on every release:

| Image | Purpose |
|-------|---------|
| `ghcr.io/dejanradmanovic/event-spec-cli` | All CLI subcommands — use in GitLab CI, CircleCI, Bitbucket Pipelines, or plain `docker run` |
| `ghcr.io/dejanradmanovic/event-spec-server` | Self-hosted registry server — run the analytics relay and event spec store as a container |

→ See [Docker](./docker.md) for CLI image examples across multiple CI platforms and server image deployment recipes.

→ See [Server → Docker deployment](../server/docker.md) for the full server deployment reference (Compose files, bootstrap key creation, PostgreSQL setup).
