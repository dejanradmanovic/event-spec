## What changed

<!-- One sentence summary. Link the issue: Closes #N -->

## Why

<!-- Context or motivation not obvious from the code. Skip if the issue already explains it. -->

## How to test

<!-- Steps to verify this works. Include specific commands, inputs, or edge cases to check. -->

## Checklist

- [ ] Tests added or updated
- [ ] CHANGELOG.md updated under `## [Unreleased]` if this changes user-visible behaviour,
      and `docs/docs/changelog.md` regenerated with `go test ./internal/changelog -update`
- [ ] `go test ./...` passes
- [ ] `golangci-lint run` passes
- [ ] No new `TODO`/`FIXME` left without a tracking issue
- [ ] Documentation added or updated (if needed). If it's not created and is needed TechDebt issue is created