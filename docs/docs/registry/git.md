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

The CLI reads your git **config** — `url.*.insteadOf`, `core.sshCommand`, `credential.helper` in `~/.gitconfig` or the repo's `.git/config` all apply as usual.

:::warning `GIT_*` environment variables are not passed through
`event-spec pull` runs git with a scrubbed environment: every `GIT_*` variable is removed before git starts. Configure SSH and credentials in your git config, not via environment variables.
:::

This is deliberate. `event-spec` runs inside git hooks, `make` targets and CI steps, where an inherited `GIT_DIR` would retarget your own checkout, and where `GIT_SSH_COMMAND`, `GIT_ASKPASS`, `GIT_PROXY_COMMAND`, `GIT_EXTERNAL_DIFF`, `GIT_EXEC_PATH` or `GIT_CONFIG_KEY_n` would let the environment decide which program git executes. If you authenticate through one of those variables today, move the setting into your git config — it is a one-line change:

| Instead of | Use |
|---|---|
| `GIT_SSH_COMMAND="ssh -i ~/.ssh/deploy_key"` | `git config --global core.sshCommand "ssh -i ~/.ssh/deploy_key"` |
| `GIT_ASKPASS=/path/to/askpass` | `git config --global core.askPass /path/to/askpass` |
| `GIT_CONFIG_KEY_0=<key>` / `GIT_CONFIG_VALUE_0=<value>` | `git config --global <key> <value>` |

When a pull fails and any of those variables was set, `event-spec` names it in the error, so a dropped `GIT_SSH_COMMAND` does not look like a broken key. Two values are set for you: `GIT_TERMINAL_PROMPT=0`, so a pull fails fast instead of blocking on a credential prompt, and `GIT_ALLOW_PROTOCOL=file:git:http:https:ssh`, so a registry `remote:` cannot select a command-executing transport such as `ext::`.

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
