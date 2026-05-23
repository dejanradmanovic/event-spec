---
sidebar_position: 8
---

# docs

Generate an HTML or Markdown event catalog from the spec registry.

## Synopsis

```
event-spec docs [flags]
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `html` | Output format: `html` \| `markdown` |
| `--out` | `./docs` | Output directory |

## Usage

```bash
# Generate HTML catalog
event-spec docs

# Generate Markdown (for use in wikis or Docusaurus)
event-spec docs --format markdown --out ./wiki

# Custom output directory
event-spec docs --out ./public/event-catalog
```

## Output

The generated catalog includes:

- A listing of all events grouped by namespace
- Per-event pages with: display name, version, status, description, property table (name, type, required, constraints, description)
- Status badges (active / deprecated / draft / deleted)
- Breaking-change history (when multiple versions exist)

## Keeping the catalog in sync

Add codegen to CI to keep the catalog up to date:

```yaml
- run: event-spec docs --format html --out ./public/event-catalog
- uses: actions/upload-artifact@v4
  with:
    name: event-catalog
    path: public/event-catalog/
```

Or serve it alongside the registry server — the web admin UI includes a built-in event catalog browser.
