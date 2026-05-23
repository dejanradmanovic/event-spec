---
sidebar_position: 1
---

# Local Registry

The local registry reads event specs from the filesystem. It is the simplest mode — no network, no server, no credentials.

## Setup

```yaml title="event-spec.yaml"
version: 1
workspace: "my-company"
registry:
  mode: local
specs_dir: ./specs
sources_dir: ./sources
destinations_dir: ./destinations
```

The CLI will use this config automatically when `event-spec.yaml` is present. If the file is absent the CLI falls back to `./specs`.

## Directory layout

```
specs/
└── <namespace>/
    └── <event_name>/
        ├── 1-0-0.yaml      # version 1
        └── 2-0-0.yaml      # version 2 (latest)
sources/
└── web-app.yaml
destinations/
└── amplitude.yaml
```

Every `.yaml` file directly under a namespace/event directory is treated as one version of that event spec. Files that fail schema validation are reported as errors by `validate`.

## Hot reload

The local registry uses [fsnotify](https://github.com/fsnotify/fsnotify) to watch the `specs_dir` tree. When a spec file changes on disk the in-memory index updates within milliseconds — no restart required. Useful during active spec development.

## In-memory index

On startup the registry walks the entire `specs_dir` tree, validates every file, and builds an in-memory index keyed by `(namespace, name, version)`. `GetEvent` and `ListEvents` serve from this index without hitting disk at query time.

## Using in Go

```go
import "github.com/dejanradmanovic/event-spec/registry/local"

reg, err := local.New(local.Config{
    SpecsDir: "./specs",
})
if err != nil {
    return err
}
defer reg.Shutdown(ctx)

def, err := reg.GetEvent(ctx, "ecommerce", "product_viewed", "")
// version "" → latest active version
```

## Version resolution

When `version` is empty string, the registry returns the **latest active version** (highest SchemaVer with `status: active`). Pass an explicit version string to pin: `reg.GetEvent(ctx, "ecommerce", "product_viewed", "1-0-0")`.
