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
- All CI checks must pass before merge

## Issue triage

Issues are tagged by phase (`phase-1` through `phase-4`) and layer (`spec-layer`, `sdk-runtime`, `codegen`, `governance`, `provider`, `hooks`, `registry`, `cli`). Pick something from `phase-1` if you want to start contributing.

## License

By contributing to event-spec you agree that your contributions will be licensed under the [GNU General Public License v3.0](LICENSE).

This means:
- You may use, copy, modify, and distribute the software under the same GPL v3 terms.
- Any derivative work or project that incorporates event-spec source code must also be distributed under GPL v3.
- You retain copyright on your own contributions; the CLA simply grants the project the right to include them.

See the full [LICENSE](LICENSE) file for the exact terms.
