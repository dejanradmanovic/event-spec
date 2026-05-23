---
sidebar_position: 10
---

# publish

Publish one or more local event specs to a running registry server.

## Synopsis

```
event-spec publish <spec-file> [<spec-file>...] [flags]
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Diff the spec against the server's current version without writing |

## Usage

Configure the server registry in `event-spec.yaml`:

```yaml title="event-spec.yaml"
version: 1
workspace: "my-company"
registry:
  mode: server
  url: https://registry.example.com
  api_key: ${REGISTRY_API_KEY}
```

Then publish a spec:

```bash
event-spec publish specs/ecommerce/product_viewed/1-0-0.yaml
```

Publish multiple specs at once:

```bash
event-spec publish specs/ecommerce/product_viewed/2-0-0.yaml \
                   specs/auth/user_signed_up/1-1-0.yaml
```

Preview changes without writing:

```bash
event-spec publish --dry-run specs/ecommerce/product_viewed/2-0-0.yaml
```

The CLI sends each spec to the server's `POST /events` endpoint with the API key from the workspace config (or `REGISTRY_API_KEY` environment variable).

## Publish in CI

```yaml title=".github/workflows/publish-specs.yml"
on:
  push:
    branches: [main]
    paths: ['specs/**']

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: go install github.com/dejanradmanovic/event-spec/cmd/event-spec@latest
      - run: |
          for file in $(git diff --name-only HEAD~1 HEAD -- specs/); do
            event-spec publish "$file"
          done
        env:
          REGISTRY_API_KEY: ${{ secrets.REGISTRY_API_KEY }}
```

## See also

- [Registry — Server](../registry/server.md) for running the server and managing API keys.
- [`serve`](./serve.md) for starting the server locally.
