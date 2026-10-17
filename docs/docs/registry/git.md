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
  remote: https://github.com/my-org/analytics-specs.git
  branch: main
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

## `branch` vs `ref`

| Key | Default | Use it to |
|-----|---------|-----------|
| `branch` | `main` | Follow a branch. Every `event-spec pull` moves the cache to that branch's current tip on the remote. |
| `ref` | — | Pin the cache to an exact commit SHA or tag, so every machine and every CI run indexes the same specs. Overrides `branch`. |

`ref` is resolved against the remote, in this order:

| Looked for | Matches a `ref` that is |
|------------|-------------------------|
| `refs/tags/<ref>` | a tag name |
| `refs/remotes/origin/<ref>` | a branch name on the remote |
| `<ref>` | a commit SHA, full or abbreviated to at least 7 hex digits |

The tag comes first, which is git's own precedence for a bare name. A `ref` that names a branch therefore follows that branch's remote tip rather than freezing on the local copy the first clone left behind ([#211](https://github.com/dejanradmanovic/event-spec/issues/211)) — but `branch:` says that more plainly, so use `branch:` to follow and `ref:` to pin.

Anything that is none of the three is rejected, including `HEAD`, `main~2` and fully qualified spellings such as `refs/heads/main`. A pin has to name something the remote has.

The cache is always left on a detached HEAD, never on a local branch, so nothing a pull checks out can go stale.

:::warning A tag and a branch of the same name
If the remote carries both a tag and a branch called `v1.4.0` and they point at different commits, `event-spec pull` fails and names both SHAs instead of picking one. Resolving it silently would mean that anyone who can push a branch — even where tags and `main` are protected — could redirect every consumer pinned to that tag. To pin past an ambiguity, set `ref` to the commit SHA: a SHA is the one spelling nothing pushed to the remote can shadow.
:::

:::note A pin means a full clone
Without `ref`, the first pull clones with `--depth 1`, which also means a single branch and no tags. A shallow clone holds only the tip of the tracked branch, which is exactly what a pinned commit or a tag is not, so setting `ref` clones the full history instead — the only way the pin can resolve. The costs are a slower first pull on a large tracking-plan repo, and every commit and branch of that repo on disk in the cache, where a shallow clone kept only the tip.

A cache that was cloned before you added the pin is still shallow, and a fetch never deepens one. `event-spec pull` then fails naming `registry.ref` and says so; re-clone with `event-spec pull --force`. The same applies to a `branch` you change after the first pull: a shallow cache tracks only the branch it was cloned with.
:::

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

This is deliberate. `event-spec` runs inside git hooks, `make` targets and CI steps, where an inherited `GIT_DIR` would retarget your own checkout. The variables that name a program for git to execute are dropped for the same reason — in those contexts the environment is often set by someone other than you. What each one controls:

- `GIT_SSH` / `GIT_SSH_COMMAND` — the transport program for `ssh://` and `git@` remotes.
- `GIT_ASKPASS` — the helper git asks for a password.
- `GIT_PROXY_COMMAND` — the proxy command git runs.
- `GIT_EXTERNAL_DIFF` — the external diff driver.
- `GIT_EXEC_PATH` — the directory git loads its own subcommands from.
- `GIT_CONFIG_COUNT` with `GIT_CONFIG_KEY_n` / `GIT_CONFIG_VALUE_n` — ad-hoc config entries, which can set `core.hooksPath`.

If you authenticate through one of those variables today, the table below is the one to act on: move the setting into your git config and it is a one-line change.

| Instead of | Use |
|------------|-----|
| `GIT_SSH_COMMAND="ssh -i ~/.ssh/deploy_key"`, or the older `GIT_SSH` | `git config --global core.sshCommand "ssh -i ~/.ssh/deploy_key"` |
| `GIT_ASKPASS=/path/to/askpass` | `git config --global core.askPass /path/to/askpass` |
| `GIT_PROXY_COMMAND=/path/to/proxy` | `git config --global core.gitProxy /path/to/proxy` — applies to `git://` remotes only; an HTTPS remote needs `git config --global http.proxy http://host:port` instead |
| `GIT_EXTERNAL_DIFF=/path/to/diff` | `git config --global diff.external /path/to/diff` — never affects a pull, which only runs `clone`, `fetch` and `checkout` |
| `GIT_CONFIG_COUNT=1` with `GIT_CONFIG_KEY_0=<key>` / `GIT_CONFIG_VALUE_0=<value>` | `git config --global <key> <value>` |

`GIT_EXEC_PATH` has no git config equivalent: git accepts it only as the `--exec-path` option, which `event-spec` never passes. To run a non-default git, put it first on `PATH`.

:::warning Do not move a credential into your git config
The last row trades a per-process variable for a file, which is the right trade for a path or a program name. But the two common CI uses of ad-hoc config are `http.extraHeader=Authorization: Bearer <token>` and `url.https://<token>@host/.insteadOf`, and `git config --global` writes that token in cleartext into `~/.gitconfig`, which git creates world-readable and `git config --list` echoes — plus your shell history. That is a longer life than the environment variable it replaced, not a shorter one. If the value is a credential, use `credential.helper` and let your platform's keychain hold it; the same applies to a program named by `core.askPass` or `core.gitProxy` that carries a token inline.

Every setting on this page belongs in `~/.gitconfig`, or in the repo's own untracked `.git/config` — never in a file you commit. A repo-checked config pulled in with `include.path` puts these values under version control, which is the one place they must not be.
:::

When a pull fails and any of those variables was set, `event-spec` names it in the error, together with that variable's replacement from the table above and a link back to this page, so a dropped `GIT_SSH_COMMAND` does not look like a broken key. Two values are set for you: `GIT_TERMINAL_PROMPT=0`, so a pull fails fast instead of blocking on a credential prompt, and `GIT_ALLOW_PROTOCOL=file:git:http:https:ssh`, so a registry `remote:` cannot select a command-executing transport such as `ext::`.

## Cache location

The local cache defaults to `~/.event-spec/cache/<repo-hash>/`. Override it with `cache_dir` in your workspace config:

```yaml title="event-spec.yaml"
registry:
  mode: git
  remote: https://github.com/my-org/analytics-specs.git
  branch: main
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
