---
sidebar_position: 2
---

# Registry

The **registry** is the source of truth for event specs. It exposes a uniform `Registry` interface regardless of the backend — local filesystem, remote git repository, or a running server.

## The Registry interface

```go
type Registry interface {
    // ListEvents returns one EventDef per (namespace, name) — the highest SchemaVer
    // that matches the filter. Use ListAllEvents when all versions are needed.
    ListEvents(ctx context.Context, filter ListFilter) ([]spec.EventDef, error)
    // ListAllEvents returns every matching EventDef without deduplication.
    // Use this for diff views, codegen with version pinning, and history pages.
    ListAllEvents(ctx context.Context, filter ListFilter) ([]spec.EventDef, error)
    GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error)
    GetSource(ctx context.Context, name string) (*spec.SourceDef, error)
    GetDestination(ctx context.Context, name string) (*spec.DestinationDef, error)
    // PublishEvent writes a new event version. Returns ErrReadOnly in git mode.
    PublishEvent(ctx context.Context, event spec.EventDef) error
    // Diff returns the detected changes between two versions of an event.
    Diff(ctx context.Context, namespace, name, from, to string) ([]spec.Change, error)
}
```

Two sentinel errors are defined at the package level:

- `ErrReadOnly` — returned by `PublishEvent` on git-backed registries; use git commits to publish.
- `ErrNotFound` — returned when a requested resource does not exist.

The CLI and runtime use this interface — swapping the backend requires only a config change.

## Three modes

| Mode | Config | Use case |
|------|--------|----------|
| **Local** | `registry.mode: local` | Specs live in this repo alongside application code |
| **Git** | `registry.mode: git` | Specs are managed in a separate analytics repository |
| **Server** | `registry.mode: server` | Centrally hosted registry with access control and a web UI |

## Listing and filtering

`ListEvents` and `ListAllEvents` accept a `ListFilter`:

```go
defs, err := reg.ListEvents(ctx, registry.ListFilter{
    Namespace: "ecommerce",        // restrict to this namespace; empty = all
    Status:    spec.StatusActive,  // restrict to this status; empty = all
    Tags:      []string{"gdpr-safe"},
})
```

## Mode details

- [Local registry](../registry/local.md) — filesystem walker with fsnotify hot-reload
- [Git registry](../registry/git.md) — remote-repo pull with local cache and version pinning
- [Server registry](../registry/server.md) — REST API with SQLite, access control, admin CLI, and web UI
