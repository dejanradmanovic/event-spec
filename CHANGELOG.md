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
  starts, so an inherited `GIT_DIR` can no longer retarget your checkout and
  `GIT_SSH_COMMAND`, `GIT_ASKPASS`, `GIT_PROXY_COMMAND`, `GIT_EXTERNAL_DIFF`,
  `GIT_EXEC_PATH` or `GIT_CONFIG_KEY_n` can no longer decide which program git executes.
  If you authenticate through one of those variables today, move the setting into your git
  config — it is a one-line change:

  | Instead of | Use |
  |---|---|
  | `GIT_SSH_COMMAND="ssh -i ~/.ssh/deploy_key"` | `git config --global core.sshCommand "ssh -i ~/.ssh/deploy_key"` |
  | `GIT_ASKPASS=/path/to/askpass` | `git config --global core.askPass /path/to/askpass` |
  | `GIT_CONFIG_KEY_0=<key>` / `GIT_CONFIG_VALUE_0=<value>` | `git config --global <key> <value>` |

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

## Earlier releases

Entries in this file start after `v0.4.2`. The releases up to and including `v0.4.2`
(`v0.1.1`, `v0.2.0`, `v0.4.0`, `v0.4.1`, `v0.4.2`) were published with auto-generated
notes — a flat list of merged pull request titles — and are deliberately **not**
backfilled: splitting those titles into Added/Changed/Breaking sections after the fact
would read as recorded history while being reconstruction. For anything released before
this file existed, read the published notes on
[the releases page](https://github.com/dejanradmanovic/event-spec/releases).
