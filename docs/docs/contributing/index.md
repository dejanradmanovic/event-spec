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
4. Open a PR with a clear description of what changed and why

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
