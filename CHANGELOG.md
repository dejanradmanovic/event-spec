# Changelog

All notable changes to event-spec are recorded here.

The format follows [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Each released version gets a `## [x.y.z] - YYYY-MM-DD` section. Unreleased work lands
under `## [Unreleased]` in the pull request that makes the change, and is promoted to a
version heading in a separate pull request just before the release is dispatched.

## [Unreleased]

### Breaking

- **`event-spec pull` no longer passes `GIT_*` environment variables through to git.**
  Every variable whose name starts with `GIT_` is removed from the environment before git
  starts, so an inherited `GIT_DIR` can no longer retarget your checkout, and
  `GIT_SSH_COMMAND` (or the older `GIT_SSH`), `GIT_ASKPASS`, `GIT_PROXY_COMMAND`,
  `GIT_EXTERNAL_DIFF`, `GIT_EXEC_PATH` and `GIT_CONFIG_COUNT` with `GIT_CONFIG_KEY_n` /
  `GIT_CONFIG_VALUE_n` can no longer decide which program git executes. If you
  authenticate or configure git through one of those variables today, move the setting
  into your git config:

  | Instead of | Use |
  |---|---|
  | `GIT_SSH_COMMAND="ssh -i ~/.ssh/deploy_key"`, or the older `GIT_SSH` | `git config --global core.sshCommand "ssh -i ~/.ssh/deploy_key"` |
  | `GIT_ASKPASS=/path/to/askpass` | `git config --global core.askPass /path/to/askpass` |
  | `GIT_PROXY_COMMAND=/path/to/proxy` | `git config --global core.gitProxy /path/to/proxy` — applies to `git://` remotes only; an HTTPS remote needs `git config --global http.proxy http://host:port` instead |
  | `GIT_EXTERNAL_DIFF=/path/to/diff` | `git config --global diff.external /path/to/diff` — never affects a pull, which only runs `clone`, `fetch` and `checkout` |
  | `GIT_CONFIG_COUNT=1` with `GIT_CONFIG_KEY_0=<key>` / `GIT_CONFIG_VALUE_0=<value>` | `git config --global <key> <value>` |

  `GIT_EXEC_PATH` has no git config equivalent: git accepts it only as the `--exec-path`
  option, which `event-spec` never passes. To run a non-default git, put it first on
  `PATH`.

  **If the value is a credential, do not persist it.** The usual reason to reach for
  `GIT_CONFIG_KEY_n` in CI is `http.extraHeader=Authorization: Bearer <token>` or
  `url.https://<token>@host/.insteadOf`, and `git config --global` writes that token in
  plaintext to `~/.gitconfig` — created mode 0644, echoed by `git config --list`, and
  routinely swept up by dotfile backups, image layers and CI caches — plus a copy in your
  shell history. That is worse than the environment variable it replaces. Put secrets in a
  `credential.helper`, or in a `.git/config` you do not commit, and keep `--global` for
  settings that are not themselves credentials.

  Two values are now set for you instead of inherited: `GIT_TERMINAL_PROMPT=0`, so a pull
  fails fast rather than blocking on a credential prompt, and
  `GIT_ALLOW_PROTOCOL=file:git:http:https:ssh`, so a registry `remote:` cannot select a
  command-executing transport such as `ext::`. When a pull fails and one of the dropped
  variables was set, the error names it, so a dropped `GIT_SSH_COMMAND` does not look like
  a broken key. Your git **config** is untouched: `url.*.insteadOf`, `core.sshCommand` and
  `credential.helper` in `~/.gitconfig` or the repository's `.git/config` all still apply.

  Details: [git registry docs](https://event-spec.io/docs/registry/git),
  [issue #184](https://github.com/dejanradmanovic/event-spec/issues/184),
  [PR #197](https://github.com/dejanradmanovic/event-spec/pull/197).

### Added

- `CHANGELOG.md` itself, plus a release workflow that puts the released version's section
  at the top of the GitHub Release body.
  ([issue #205](https://github.com/dejanradmanovic/event-spec/issues/205))

### Fixed

- **`event-spec pull` kept the cache frozen when `registry.ref` named a branch.** A
  `registry.ref` holding a branch name was checked out as a local branch, and the local
  branch a shallow clone creates is never advanced by a fetch: every later pull printed
  `✓ Indexed N event(s).` and indexed the same commit as the first one, forever. On a
  fresh cache the opposite half of the bug applied — the clone followed
  `registry.branch` and ignored `registry.ref` entirely, so a pin to a commit or tag was
  silently not honoured.

  `registry.ref` is now resolved against the remote — `origin/<ref>`, then
  `refs/tags/<ref>`, then `<ref>` as a commit — and checked out detached, so a
  branch-named ref follows the remote branch while a commit or tag still pins. The cache
  is never left on a local branch. Nothing to change in your config: a `ref` naming a
  branch now does what it looked like it did. `registry.branch` remains the plainer way
  to say "follow a branch".

  Two consequences worth knowing. Setting `registry.ref` makes the first clone a full
  one, because a `--depth 1` clone cannot contain an older commit or a tag outside the
  tracked branch's tip — a slower first pull on a large tracking-plan repo. And a cache
  cloned before you added a pin may be shallow and unable to resolve it; the pull now
  fails naming `registry.ref` instead of succeeding against the wrong commit, and
  `event-spec pull --force` re-clones it.

  Details: [git registry docs](https://event-spec.io/docs/registry/git),
  [issue #211](https://github.com/dejanradmanovic/event-spec/issues/211).

## Earlier releases

Entries in this file start after `v0.4.2`. The releases up to and including `v0.4.2`
(`v0.1.1`, `v0.2.0`, `v0.4.0`, `v0.4.1`, `v0.4.2`) were published with auto-generated
notes — a flat list of merged pull request titles — and are deliberately **not**
backfilled: splitting those titles into Added/Changed/Breaking sections after the fact
would read as recorded history while being reconstruction. For anything released before
this file existed, read the published notes on
[the releases page](https://github.com/dejanradmanovic/event-spec/releases).
