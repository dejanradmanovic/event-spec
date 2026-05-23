---
sidebar_position: 1
---

# Providers

Providers are adapters that deliver analytics events to specific backends. They implement a stable interface so application code never depends on a vendor SDK directly.

## Capability matrix

| Provider | Language | Track | Identify | Group | Page | Alias | Status |
|----------|----------|:-----:|:--------:|:-----:|:----:|:-----:|--------|
| [Amplitude](./amplitude.md) | Go | ✅ | ✅ | ✅ | ✅ | ✅ | Available |
| [Amplitude](./amplitude.md) | TypeScript | ✅ | ✅ | ✅ | ✅ | ✅ | Available |
| [Noop](./noop.md) | Go | ✅ | ✅ | ✅ | ✅ | ✅ | Available |
| PostHog | Go | ❌ | ❌ | ❌ | ❌ | ❌ | Planned |
| Mixpanel | Go | ❌ | ❌ | ❌ | ❌ | ❌ | Planned |
| Segment | Go | ❌ | ❌ | ❌ | ❌ | ❌ | Planned |
| GA4 | Go | ❌ | ❌ | ❌ | ❌ | ❌ | Planned |
| RudderStack | Go | ❌ | ❌ | ❌ | ❌ | ❌ | Planned |

## Provider error semantics

Providers that don't support an operation return `ErrUnsupportedOperation` — they never silently no-op. This prevents data from appearing delivered when it was actually discarded.

## See also

- [Concepts — Providers](../concepts/providers.md) for the full interface documentation
- [Custom Provider](./custom.md) for implementing your own adapter
