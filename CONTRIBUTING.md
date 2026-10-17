# Contributing to event-spec

Thank you for your interest in contributing. This document covers the development setup, workflow, and conventions.

## Prerequisites

- Go 1.26+
- [golangci-lint](https://golangci-lint.run/usage/install/)
- [lefthook](https://github.com/evilmartians/lefthook) (for git hooks)

Install all dev tools at once:

```bash
make install-tools
```

## Setup

```bash
git clone https://github.com/dejanradmanovic/event-spec.git
cd event-spec
go mod download
make hooks   # install pre-commit and pre-push hooks
```

## Development workflow

| Command | What it does |
|---|---|
| `make build` | Compile the CLI binary |
| `make test` | Run tests with race detector |
| `make test-cover` | Run tests and open HTML coverage report |
| `make lint` | Run golangci-lint |
| `make fmt` | Format all Go source files |
| `make tidy` | Tidy and verify go modules |

- Never exec `git` directly — use `testutil.GitCommand` / `testutil.RunGit`
- Why: git exports `GIT_DIR` into hook environments, and it overrides `cmd.Dir` and `-C`, so an unguarded fixture operates on your own checkout under `pre-push`
- `TestNoUnguardedGitExec` fails the build when a new direct call site appears
- Running the Go tests needs git >= 2.32 (`GIT_CONFIG_GLOBAL`)

## Pre-commit hooks

After running `make hooks`, the following checks run automatically:

- **pre-commit** — `gofmt` (auto-formats staged files), `go vet`
- **pre-push** — `go test -race ./...`, `golangci-lint run`

## Adding a new analytics provider

1. Create `provider/{name}/provider.go` implementing the `provider.Provider` interface
2. Create `provider/{name}/config.go` for any provider-specific config
3. Register the provider in `provider/{name}/mapper.go` if property coercion is needed
4. Add the provider to the acceptance criteria checklist in the relevant GitHub issue
5. Add integration tests using `testutil.CaptureProvider`

## Adding a new codegen language

1. Create `codegen/templates/{lang}/wrapper.{ext}.tmpl`
2. Add a `LangConfig` entry in `codegen/languages.go`
3. Implement a `Namer` in `codegen/namer.go` if a new naming style is needed
4. Add golden file tests under `codegen/testdata/golden/{lang}/`

## Commit style

```
[scope] short imperative description

Optional body explaining WHY, not what. Reference issues with #N.
```

Examples:
```
[spec] add PropertyDef aliases for backwards-compatible property names
[provider/amplitude] fix batch payload encoding for integer properties
[codegen] generate enum const block for Go wrappers
```

## Pull requests

- One PR per issue
- Link the issue in the description: `Closes #N`
- Keep PRs focused; refactors unrelated to the issue go in separate PRs
- A user-visible behaviour change adds a `CHANGELOG.md` entry under `## [Unreleased]` in
  the same PR (see **Changelog** below)
- All CI checks must pass before merge

## Changelog

`CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

- If a PR changes something a user can observe — CLI behaviour, flags, config keys,
  generated code, SDK or provider APIs, an error message someone scripts against — it adds
  a bullet under `## [Unreleased]`, in the same PR, under `### Added`, `### Changed`,
  `### Deprecated`, `### Removed`, `### Fixed`, `### Security` or `### Breaking`.
- Internal refactors, test-only changes and typo fixes need no entry.
- A `### Breaking` entry says what to do, not only what changed: name the old setting and
  the new one, and link the issue or PR.
- Links in `CHANGELOG.md` must be absolute. The file is mirrored to
  `docs/docs/changelog.md` for [event-spec.io](https://event-spec.io/docs/changelog),
  which builds with `onBrokenLinks: 'throw'`. After editing `CHANGELOG.md`, regenerate the
  mirror and commit it:

  ```bash
  go test ./internal/changelog -update
  ```

Releasing: `release.yml` extracts the released version's section into the GitHub Release
body **before** it pushes the tag, so a release needs a promoted section. Open a PR that
renames `## [Unreleased]` to `## [x.y.z] - YYYY-MM-DD` (adding a fresh empty
`## [Unreleased]` above it) and merge it before dispatching the release workflow —
dispatching a version with no section fails the run at the pre-tag step.

## Issue triage

Issues are tagged by phase (`phase-1` through `phase-4`) and layer (`spec-layer`, `sdk-runtime`, `codegen`, `governance`, `provider`, `hooks`, `registry`, `cli`). Pick something from `phase-1` if you want to start contributing.

## License

By contributing to event-spec you agree that your contributions will be licensed under the [GNU General Public License v3.0](LICENSE).

This means:
- You may use, copy, modify, and distribute the software under the same GPL v3 terms.
- Any derivative work or project that incorporates event-spec source code must also be distributed under GPL v3.
- You retain copyright on your own contributions; the CLA simply grants the project the right to include them.

See the full [LICENSE](LICENSE) file for the exact terms.
