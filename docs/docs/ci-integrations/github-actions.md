---
sidebar_position: 2
---

# GitHub Actions

Three composite actions are published directly from this repository. Reference them by path — no separate repo needed:

```yaml
uses: dejanradmanovic/event-spec/validate@main
uses: dejanradmanovic/event-spec/generate@main
uses: dejanradmanovic/event-spec/audit@main
```

## Workspace file discovery

Every action auto-discovers `event-spec.yaml` from the repository root and reads source definitions from it. Override behaviour via inputs:

| Priority | Mechanism |
|----------|-----------|
| 1 (highest) | `specs-dir` / `sources-dir` input |
| 2 | `event-spec-yaml` input — explicit path, skips root discovery |
| 3 (default) | `event-spec.yaml` in repository root |

## Common inputs (all actions)

| Input | Default | Description |
|-------|---------|-------------|
| `version` | `latest` | event-spec CLI version to install (semver without leading `v`, or `"latest"`) |
| `event-spec-yaml` | _(none)_ | Explicit path to workspace file; skips auto-discovery |

## `validate` inputs

| Input | Default | Description |
|-------|---------|-------------|
| `specs-dir` | _(from workspace)_ | Override specs location |
| `strict` | `false` | Treat warnings as errors |

The validate action also runs `event-spec diff --breaking` on pull requests and posts a PR comment when breaking spec changes are detected.

## `generate` inputs

| Input | Default | Description |
|-------|---------|-------------|
| `sources-dir` | _(from workspace)_ | Override sources location |
| `sources` | _(all sources)_ | Comma-separated source names to generate |
| `open-pr` | `true` | Open a PR for generated files instead of committing directly |

## `audit` inputs

| Input | Default | Description |
|-------|---------|-------------|
| `sources-dir` | _(from workspace)_ | Override sources location |
| `sources` | _(all sources)_ | Comma-separated source names to audit |
| `coverage-min` | `80` | Minimum coverage percentage |
| `strict` | `false` | Fail on any unused required event |

The audit action emits GitHub Actions annotations for uncovered events directly in the pull request diff.

---

## Workflow recipes

### Single-source repo (specs and source in same repo)

```yaml title=".github/workflows/ci.yml"
on:
  push:
    branches: [main]
    paths: ['specs/**', 'sources/**']
  pull_request:
    branches: [main]
    paths: ['specs/**', 'sources/**', 'src/**']

jobs:
  validate:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write   # needed to post breaking-change PR comment
    steps:
      - uses: actions/checkout@v4
      - uses: dejanradmanovic/event-spec/validate@main
        # event-spec.yaml at repo root is discovered automatically
        # with:
        #   strict: true

  generate:
    runs-on: ubuntu-latest
    needs: validate
    permissions:
      contents: write
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
      - uses: dejanradmanovic/event-spec/generate@main
        # with:
        #   open-pr: true   # default — opens a PR for generated files

  audit:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write   # needed to post coverage annotations
    steps:
      - uses: actions/checkout@v4
      - uses: dejanradmanovic/event-spec/audit@main
        # with:
        #   coverage-min: 80   # default
        #   strict: true
```

### Multi-source repo (generate and audit a subset of sources)

```yaml title=".github/workflows/ci.yml"
jobs:
  audit-web:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
      - uses: dejanradmanovic/event-spec/audit@main
        with:
          sources: web-app,marketing-site
          coverage-min: 90
          strict: true

  generate-mobile:
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
      - uses: dejanradmanovic/event-spec/generate@main
        with:
          sources: ios-app,android-app
          open-pr: true
```

### Specs in a separate repo (git registry mode)

When `event-spec.yaml` in your app repo points to a remote git registry, run `event-spec pull` once before the other actions:

```yaml title=".github/workflows/ci.yml"
jobs:
  validate-and-audit:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
    steps:
      - uses: actions/checkout@v4

      # Pull specs from the shared tracking-plan repo.
      - run: |
          VERSION=$(curl -fsSL \
            -H "Authorization: Bearer ${{ github.token }}" \
            "https://api.github.com/repos/dejanradmanovic/event-spec/releases/latest" \
            | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')
          curl -fsSL \
            "https://github.com/dejanradmanovic/event-spec/releases/download/v${VERSION}/event-spec_v${VERSION}_linux_amd64.tar.gz" \
            | tar -xz event-spec
          sudo mv event-spec /usr/local/bin/event-spec
          event-spec pull

      - uses: dejanradmanovic/event-spec/validate@main
      - uses: dejanradmanovic/event-spec/audit@main
        with:
          coverage-min: 80
```

### Explicit workspace file (monorepo)

If `event-spec.yaml` is not at the repo root, point the actions to it:

```yaml title=".github/workflows/ci.yml"
jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dejanradmanovic/event-spec/validate@main
        with:
          event-spec-yaml: apps/backend/event-spec.yaml
          strict: true

  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dejanradmanovic/event-spec/audit@main
        with:
          event-spec-yaml: apps/backend/event-spec.yaml
          coverage-min: 75
```

### Pinning to a specific version

Pin to a released version (recommended for production workflows) to avoid unexpected changes:

```yaml
# Pin by version tag
- uses: dejanradmanovic/event-spec/validate@v0.3.1

# Pin by commit SHA (most reproducible)
- uses: dejanradmanovic/event-spec/validate@abc1234
```

Or pin the CLI version independently of the action ref:

```yaml
- uses: dejanradmanovic/event-spec/validate@main
  with:
    version: '0.3.1'
```

### Breaking-change PR comment

When `validate` runs on a pull request and `event-spec diff --breaking` detects regressions, it posts a comment automatically:

```
## ⚠️ Breaking spec changes detected

The following breaking changes were found in this pull request...

BREAKING    remove_prop             product_id
BREAKING    type_changed            price (number → string)

Version: declared 1-1-0, required 2-0-0 — ERROR
```

No extra configuration is needed — just ensure the job has `pull-requests: write` permission.
