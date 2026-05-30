---
sidebar_position: 3
---

# Adding SDKs

This guide covers adding a new language SDK to event-spec.

## What an SDK consists of

An event-spec SDK for a language has three parts:

1. **Core runtime** — Client, Provider interface, Hook interface, context propagation, queue, dispatch
2. **Codegen template** — `text/template` template files that generate typed wrappers for that language
3. **Provider implementations** — adapters for specific analytics backends

## Step 1 — Core runtime

The TypeScript SDK (`sdk/typescript/packages/api`) and the Kotlin SDK (`sdk/kotlin/api/`) are the reference implementations to follow alongside the Go `analytics/` package.

For JVM/Gradle-based languages, use a multi-module Gradle build with a `gradle/libs.versions.toml` version catalog. See `sdk/kotlin/` for the reference structure:

```
sdk/<language>/
├── settings.gradle.kts           # include(":api", ":provider-<name>")
├── gradle/
│   └── libs.versions.toml        # centralised dependency versions
└── api/
    └── src/main/kotlin/...
```

For other languages, create a new directory:

```
sdk/<language>/
└── src/
    ├── client.ts / client.swift / ...
    ├── provider.ts
    ├── hooks.ts
    ├── context.ts
    └── queue.ts
```

The runtime must implement:
- `Client` — manages providers, hooks, and context; dispatches `track`, `identify`, `group`, `page`, `alias`
- `Provider` interface — the stable contract between the runtime and vendor adapters
- `Hook` interface — `before`, `after`, `error`, `finally` lifecycle stages
- Context merging — 4-level chain: **global → transaction → client → invocation** (each level overrides the previous)
- Event queue — batching and flush semantics

## Step 2 — Codegen template

Templates live in `codegen/<language>/`:

```
codegen/swift/
├── engine.go               # registers the "swift" language ID
├── templates/
│   ├── eventspec.swift.tmpl  # entry point struct/class
│   └── event.swift.tmpl      # per-event method + types
└── engine_test.go            # golden test
```

`engine.go` must implement the `codegen.Engine` interface and register itself:

```go title="codegen/swift/engine.go"
package swift

import "github.com/dejanradmanovic/event-spec/codegen"

func init() {
    codegen.Register(
        codegen.LangConfig{
            ID:         "swift",
            Namer:      SwiftNamer{},
            TypeMapper: SwiftTypeMapper{},
            FileExt:    ".swift",
        },
        &Engine{},
    )
}

type Engine struct{}

func (e *Engine) Generate(td codegen.TemplateData, outDir string) error {
    // render templates into outDir
    return nil
}
```

Import it with a blank import in `cmd/event-spec/generate.go`:

```go
import _ "github.com/dejanradmanovic/event-spec/codegen/swift"
```

### Golden tests

Golden tests verify codegen output is stable. The pattern used throughout the codebase is:

```go title="codegen/engine_test.go (pattern)"
func TestGenerate_Swift(t *testing.T) {
    events := testEvents()
    outDir := t.TempDir()
    if err := codegen.Run(events, "swift", outDir, "test-workspace", "test-source", ""); err != nil {
        t.Fatalf("Run: %v", err)
    }
    compareOrUpdate(t, outDir, filepath.Join("testdata", "golden", "swift"))
}
```

Add expected output files to `codegen/testdata/golden/swift/` and run with `-update` to generate them initially:

```bash
go test ./codegen/... -update
```

## Step 3 — Provider implementations

Each provider for the new language follows the same pattern as the Go Amplitude provider. See [Adding Providers](./adding-providers.md) for the structure.

## Step 4 — Document it

1. Add an SDK page at `docs/docs/sdks/<language>.md`
2. Update the provider capability matrix in `docs/docs/providers/index.md`
3. Update the codegen table in `docs/docs/concepts/codegen.md`
4. Update the landing page SDK strip in `docs/src/pages/index.tsx`
5. Add a language tab to every page that shows Go/TypeScript examples — see the Kotlin PR for the full list of pages to update

## Checklist

- [ ] Core runtime implements the full Provider and Hook interfaces
- [ ] Codegen engine registered with `codegen.Register(config, engine)` (two arguments)
- [ ] Golden test data added and passing (`go test ./codegen/... -update` to generate)
- [ ] `event-spec generate --lang <lang>` works end-to-end
- [ ] At least one provider implementation
- [ ] Built-in zero-dependency hooks (`sampling`, `validation`) implemented inside the core runtime package
- [ ] Hooks with external dependencies (logging, otel, etc.) live in separate packages (`sdk/<language>/packages/hooks-<name>/`) so they don't bloat the core API
- [ ] Documentation pages added
- [ ] `make test && make lint` passes
