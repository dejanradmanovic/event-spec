---
sidebar_position: 2
---

# Installation

## CLI

The `event-spec` CLI is used to validate specs, generate typed wrappers, diff versions, audit codebases, and run the registry server.

### Via Go install

```bash
go install github.com/dejanradmanovic/event-spec/cmd/event-spec@latest
```

Requires Go 1.21+. The binary lands in `$GOPATH/bin` (ensure it's on your `$PATH`).

### From GitHub releases

Download a pre-built binary from the [releases page](https://github.com/dejanradmanovic/event-spec/releases):

```bash
# macOS (Apple Silicon)
curl -L https://github.com/dejanradmanovic/event-spec/releases/latest/download/event-spec_darwin_arm64.tar.gz | tar xz
sudo mv event-spec /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/dejanradmanovic/event-spec/releases/latest/download/event-spec_darwin_amd64.tar.gz | tar xz
sudo mv event-spec /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/dejanradmanovic/event-spec/releases/latest/download/event-spec_linux_amd64.tar.gz | tar xz
sudo mv event-spec /usr/local/bin/
```

Verify the installation:

```bash
event-spec --version
```

## Go SDK

Add the analytics runtime to your Go module:

```bash
go get github.com/dejanradmanovic/event-spec@latest
```

To use the Amplitude provider:

```bash
go get github.com/dejanradmanovic/event-spec/provider/amplitude@latest
```

## TypeScript SDK

Install from GitHub Packages. Add a `.npmrc` (or `.yarnrc.yml` / `.pnpmrc`) pointing the `@dejanradmanovic` scope to GitHub Packages:

```ini title=".npmrc"
@dejanradmanovic:registry=https://npm.pkg.github.com
```

Then install:

```bash
# npm
npm install @dejanradmanovic/event-spec-api

# pnpm
pnpm add @dejanradmanovic/event-spec-api

# yarn
yarn add @dejanradmanovic/event-spec-api
```

For the Amplitude provider:

```bash
npm install @dejanradmanovic/event-spec-provider-amplitude
```

## Workspace setup

Create an `event-spec.yaml` in your project root to configure the registry mode and directory layout:

```yaml title="event-spec.yaml"
version: 1
workspace: "my-company"
registry:
  mode: local
specs_dir: ./specs
sources_dir: ./sources
destinations_dir: ./destinations
```

See the [Workspace config reference](./workspace.md) for all options.

## Next steps

- [Quickstart](./quickstart.md) — write your first spec and generate typed wrappers
