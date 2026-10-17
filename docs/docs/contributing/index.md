---
sidebar_position: 1
---

# Contributing

Thank you for your interest in contributing to event-spec!

## Development setup

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.26+ | Core runtime and CLI |
| golangci-lint | latest | Go linting |
| lefthook | latest | Git hooks |
| Node.js | 18+ | TypeScript SDK |
| pnpm | 8+ | TypeScript package manager |

### Setup

```bash
git clone https://github.com/dejanradmanovic/event-spec.git
cd event-spec

# Download Go dependencies and install dev tools (golangci-lint, lefthook, pnpm + TypeScript deps)
go mod download
make install-tools

# Install git hooks
make hooks
```

### Makefile targets

| Target | Description |
|--------|-------------|
| `make build` | Compile the CLI binary |
| `make test` | Run all Go tests |
| `make test-cover` | Run tests with HTML coverage report |
| `make lint` | Run golangci-lint |
| `make fmt` | Format Go source files |
| `make vet` | Run go vet |
| `make tidy` | Tidy and verify Go modules |

### Git hooks (lefthook)

- **pre-commit** — runs `go fmt` and `go vet` on staged files
- **pre-push** — runs the full test suite and linter

If a hook blocks you unexpectedly, investigate the root cause rather than bypassing with `--no-verify`.

## Code conventions

- Follow standard Go idioms (the linter enforces most of this)
- No comments explaining _what_ the code does — only _why_ when it's non-obvious
- No error handling for impossible cases — trust internal invariants
- Tests should use `testutil.CaptureProvider` for integration tests, not mocks

## Pull request process

1. Fork the repository and create a branch from `main`
2. Write tests for any new functionality
3. Ensure `make test && make lint` passes
4. Add a `CHANGELOG.md` entry under `## [Unreleased]` if the change is user-visible (see
   [Changelog entries](#changelog-entries))
5. Open a PR with a clear description of what changed and why

## Changelog entries

A user-visible behaviour change adds a `CHANGELOG.md` entry under `## [Unreleased]` in the
same pull request. The file follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and is published as the [Changelog](../changelog.md) page on this site.

- User-visible means observable from outside the repository: CLI behaviour, flags, config
  keys, generated code, SDK or provider APIs, or an error message someone scripts against.
  Internal refactors, test-only changes and typo fixes need no entry.
- Put the bullet under one of `### Added`, `### Changed`, `### Deprecated`,
  `### Removed`, `### Fixed`, `### Security` or `### Breaking`.
- A `### Breaking` entry says what to do, not only what changed: name the old setting and
  the new one, and link the issue or pull request.
- Links in `CHANGELOG.md` must be absolute — this site builds with
  `onBrokenLinks: 'throw'` and the file is mirrored verbatim.
- Regenerate the mirrored page and commit it:

  ```bash
  go test ./internal/changelog -update
  ```

The release workflow extracts the released version's section into the GitHub Release body
*before* it pushes the tag, so a release needs its `## [x.y.z] - YYYY-MM-DD` heading
promoted before the workflow is dispatched. That promotion is an ordinary pull request:
rename `## [Unreleased]`, add a fresh (possibly empty) `## [Unreleased]` above it, and
regenerate the mirror. See
[Releasing](https://github.com/dejanradmanovic/event-spec/blob/main/CONTRIBUTING.md#releasing)
for the full recipe.

## Areas to contribute

- **New providers** — See [Adding Providers](./adding-providers.md)
- **New SDK languages** — See [Adding SDKs](./adding-sdks.md)
- **Hook implementations** — `hooks/logging` and `hooks/otel` are planned but not implemented
- **Documentation** — improvements to this site are always welcome

## License

event-spec uses a split license model: **AGPL-3.0** for the platform (CLI, registry server,
codegen engine) and **Apache-2.0** for the client libraries (`analytics/`, `provider/`,
`hooks/`, `sdk/`). See the [License page](./license.md) for the full breakdown and rationale.

By submitting a pull request you agree that your contribution will be licensed under whichever
license applies to the directory you are contributing to. You retain copyright on your own work.
