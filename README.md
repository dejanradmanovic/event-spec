<div align="center">

# event-spec

**Define events once. Generate type-safe wrappers for every language. Swap analytics vendors without touching your code.**

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26+-00ADD8?logo=go&logoColor=white)](go.mod)
[![CI](https://github.com/dejanradmanovic/event-spec/actions/workflows/ci.yml/badge.svg)](https://github.com/dejanradmanovic/event-spec/actions/workflows/ci.yml)
[![Docs](https://img.shields.io/badge/docs-dejanradmanovic.github.io%2Fevent--spec-orange)](https://dejanradmanovic.github.io/event-spec/)

</div>

---

Analytics instrumentation is tightly coupled to a single vendor by default. Swapping providers, running A/B tests across platforms, or enforcing event schema consistency across a polyglot codebase requires large refactors. **event-spec** breaks that coupling.

Write an event spec in YAML once. The CLI generates a type-safe wrapper in Go, TypeScript, or any other target language. The runtime dispatches to one or many analytics providers behind a stable interface — swap Amplitude for PostHog, or run both in parallel, with no changes to your instrumentation code.

```yaml
# specs/ecommerce/product_viewed/1-0-0.yaml
name: product_viewed
version: "1-0-0"
status: active
namespace: ecommerce
type: track
properties:
  product_id:   { type: string,  required: true }
  category:     { type: string,  required: true, enum: [clothing, electronics, other] }
  currency:     { type: string,  required: false, default: "USD" }
```

```bash
event-spec generate --lang go --out ./generated
```

```go
es.ProductViewed(ctx, generated.ProductViewedProperties{
    Category:  generated.ProductViewedCategoryElectronics,
    ProductId: "SKU-123",
})
```

That's it. Swap `amplitude.New(...)` for `posthog.New(...)` and nothing else changes.

---

## Features

- **Schema-first** — YAML event specs with JSON Schema validation, SchemaVer versioning, and breaking-change detection
- **Codegen** — type-safe event wrappers generated for Go and TypeScript today; Swift, Kotlin, Python, Rust, Dart, and .NET on the roadmap
- **Provider abstraction** — stable `Provider` interface with batching, retry, rate-limiting, and reverse-proxy support built in
- **Hook pipeline** — middleware chain for validation, sampling, PII stripping, and consent filtering before any provider receives an event
- **Audit** — AST-based scanner reports which events are used, unused, or sent without a spec
- **Registry server** — self-hosted REST API with RBAC, an analytics relay, and an admin UI; ships as a Docker image

---

## Getting started

→ **[Full documentation](https://dejanradmanovic.github.io/event-spec/)**

| | |
|---|---|
| [Installation](https://dejanradmanovic.github.io/event-spec/docs/getting-started/installation) | Install the CLI and runtime packages |
| [Quickstart](https://dejanradmanovic.github.io/event-spec/docs/getting-started/quickstart) | Write a spec, generate wrappers, track an event |
| [CLI reference](https://dejanradmanovic.github.io/event-spec/docs/cli/) | Every subcommand and flag |
| [CI integrations](https://dejanradmanovic.github.io/event-spec/docs/ci-integrations/) | GitHub Actions, GitLab CI, Docker |
| [Server](https://dejanradmanovic.github.io/event-spec/docs/server/) | Self-host the registry + analytics relay |

---

## What's implemented

| Layer | Status |
|---|---|
| YAML spec loader, JSON Schema validation, diff | ✅ |
| Go runtime — client, hooks, dispatch, queue, retry | ✅ |
| TypeScript runtime — `@event-spec/analytics` | ✅ |
| Go codegen + TypeScript codegen | ✅ |
| Amplitude provider (Go + TypeScript) | ✅ |
| Registry server — REST API, SQLite/Postgres, RBAC, relay | ✅ |
| Audit — AST-based event coverage scanning | ✅ |
| CI/CD — composite GitHub Actions + Docker images | ✅ |
| PostHog, Mixpanel, Segment, GA4, RudderStack providers | 🔜 |
| Swift, Kotlin, Python, Rust, Dart, .NET SDKs | 🔜 |
| Logging and OpenTelemetry hooks | 🔜 |

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development setup, conventions, and PR workflow.

This project is licensed under the **[GNU General Public License v3.0](LICENSE)**.
