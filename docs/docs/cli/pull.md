---
sidebar_position: 6
---

# pull

Pull event specs from a remote git registry into a local cache.

## Synopsis

```
event-spec pull [flags]
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--force` | `false` | Re-clone the remote repository even if a local cache already exists |
| `--ref` | `""` | Override the branch, tag, or commit SHA to check out (overrides `registry.ref` in workspace config) |

## Usage

Configure the git registry in `event-spec.yaml`:

```yaml title="event-spec.yaml"
version: 1
workspace: "my-company"
registry:
  mode: git
  url: https://github.com/my-org/analytics-specs.git
  ref: main
specs_dir: ./specs
```

Then pull the remote specs:

```bash
event-spec pull
```

The CLI clones or fetches the remote repository into a local cache directory and indexes the spec files.

## After pulling

Once specs are cached locally, all other CLI commands work against the local cache:

```bash
event-spec pull                    # sync with remote
event-spec validate                # validate against cached specs
event-spec generate web-app        # generate using cached specs
event-spec diff ecommerce/product_viewed  # diff cached versions
```

## Version pinning

Source configs can pin a specific git ref:

```yaml title="sources/web-app.yaml"
version_pinning:
  ecommerce/product_viewed: "1-0-0"
```

Events pinned to a specific version are generated with that version regardless of what's latest in the registry.

## See also

- [Registry — Git](../registry/git.md) for full setup details.
