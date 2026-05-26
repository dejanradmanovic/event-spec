---
sidebar_position: 3
---

# Docker

Two images are published to GitHub Container Registry (GHCR) on every release.

## CLI image

**`ghcr.io/dejanradmanovic/event-spec-cli`**

The CLI image exposes every `event-spec` subcommand. Use it in any CI system that supports Docker, or run it locally with `docker run`.

| Tag | Description |
|-----|-------------|
| `latest` | Most recent release |
| `v0.3.1` | Specific semver release |

Multi-arch: `linux/amd64` + `linux/arm64`.

### GitLab CI

```yaml title=".gitlab-ci.yml"
validate:
  image: ghcr.io/dejanradmanovic/event-spec-cli:latest
  script:
    - event-spec validate --strict
    - event-spec diff --breaking

audit:
  image: ghcr.io/dejanradmanovic/event-spec-cli:latest
  script:
    - event-spec audit --coverage-min 80
```

### CircleCI

```yaml title=".circleci/config.yml"
jobs:
  validate:
    docker:
      - image: ghcr.io/dejanradmanovic/event-spec-cli:latest
    steps:
      - checkout
      - run: event-spec validate --strict
      - run: event-spec audit --coverage-min 80

workflows:
  ci:
    jobs:
      - validate
```

### Bitbucket Pipelines

```yaml title="bitbucket-pipelines.yml"
pipelines:
  default:
    - step:
        image: ghcr.io/dejanradmanovic/event-spec-cli:latest
        script:
          - event-spec validate --strict
          - event-spec audit --coverage-min 80
```

### Any CI with Docker available

```bash
# Validate specs in the current directory
docker run --rm \
  -v "$PWD:/workspace" \
  -w /workspace \
  ghcr.io/dejanradmanovic/event-spec-cli:latest \
  validate --strict

# Generate typed wrappers for a source
docker run --rm \
  -v "$PWD:/workspace" \
  -w /workspace \
  ghcr.io/dejanradmanovic/event-spec-cli:latest \
  generate web-app

# Audit event coverage
docker run --rm \
  -v "$PWD:/workspace" \
  -w /workspace \
  ghcr.io/dejanradmanovic/event-spec-cli:latest \
  audit web-app --coverage-min 80

# Print the CLI version
docker run --rm ghcr.io/dejanradmanovic/event-spec-cli:latest --version
```

### Digest pinning

For reproducible CI builds, pin to an image digest instead of a mutable tag:

```yaml
# GitLab CI
image: ghcr.io/dejanradmanovic/event-spec-cli@sha256:<digest>
```

```bash
# Pull and find the digest
docker pull ghcr.io/dejanradmanovic/event-spec-cli:v0.3.1
docker inspect --format='{{index .RepoDigests 0}}' ghcr.io/dejanradmanovic/event-spec-cli:v0.3.1
```

---

## Server image

**`ghcr.io/dejanradmanovic/event-spec-server`**

The server image runs the analytics relay and event spec registry. See [Server → Docker deployment](../server/docker.md) for the full reference including Docker Compose examples, bootstrap key creation, and PostgreSQL setup.

### Quick start

```bash
# SQLite — development
docker run -p 8080:8080 \
  -v "$PWD/data:/data" \
  ghcr.io/dejanradmanovic/event-spec-server:latest \
  --db file:/data/registry.db

# Print the server version
docker run --rm ghcr.io/dejanradmanovic/event-spec-server:latest --version
```

After starting, create the initial admin key:

```bash
curl -X POST http://localhost:8080/v1/admin/keys \
  -H "Content-Type: application/json" \
  -d '{"role": "admin", "name": "bootstrap"}'
```

The raw key is returned once — save it. All subsequent key creation requires an admin token.
