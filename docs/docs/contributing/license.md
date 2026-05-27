---
sidebar_position: 4
---

# License

event-spec uses a **split licensing model** designed to protect the platform from proprietary
forks while allowing commercial teams to freely adopt the client SDKs in their applications.

## License by component

| Component | License | SPDX |
|---|---|---|
| CLI (`cmd/`) | AGPL-3.0 | `AGPL-3.0-only` |
| Registry server (`registry/`) | AGPL-3.0 | `AGPL-3.0-only` |
| Schema engine (`spec/`) | AGPL-3.0 | `AGPL-3.0-only` |
| Codegen engine (`codegen/`) | AGPL-3.0 | `AGPL-3.0-only` |
| Analytics client runtime (`analytics/`) | Apache-2.0 | `Apache-2.0` |
| Provider adapters (`provider/`) | Apache-2.0 | `Apache-2.0` |
| Hook implementations (`hooks/`) | Apache-2.0 | `Apache-2.0` |
| Language SDKs (`sdk/`) | Apache-2.0 | `Apache-2.0` |

The full AGPL-3.0 text is in the [`LICENSE`](https://github.com/dejanradmanovic/event-spec/blob/main/LICENSE)
file at the repository root. The full Apache-2.0 text is in
[`LICENSE-APACHE`](https://github.com/dejanradmanovic/event-spec/blob/main/LICENSE-APACHE).
Each Apache-2.0 directory also contains a short `LICENSE` notice file with an SPDX identifier.

## Why this split?

The original motivation for a copyleft license was:

> *prevent bad actors from building proprietary products on top of this work without contributing back*

That goal is real, but it applies to two very different scenarios:

**Scenario A — someone forks the registry server** and sells it as a closed-source hosted service
without sharing their modifications. This is the threat the license is designed to block.

**Scenario B — a company imports `@event-spec/analytics`** into their proprietary web app to track
events. This is the *intended use case*, not a threat.

GPL-3.0 blocked both. The split model blocks only Scenario A:

- **AGPL-3.0 on the platform** (CLI, server, codegen) prevents proprietary forks. The Affero
  variant closes the SaaS loophole: even running a modified registry server over a network requires
  sharing the source, with no need to distribute a binary.
- **Apache-2.0 on the client libraries** lets commercial teams import the SDK into proprietary
  applications without any copyleft obligations on their own code — exactly as Amplitude, PostHog,
  and Segment SDKs work.

## What this means in practice

| You want to… | Allowed? |
|---|---|
| Use the CLI and codegen in your build pipeline | ✅ Yes |
| Run the registry server internally | ✅ Yes |
| Import `analytics`, `provider`, `hooks`, or `sdk` packages into a proprietary app | ✅ Yes — Apache-2.0 |
| Modify the registry server and run it as a hosted service | ✅ Yes, but you must publish your server-side modifications (AGPL-3.0 §13) |
| Fork the CLI or codegen engine into a closed-source product | ❌ No — derivative works must be AGPL-3.0 |
| Fork the client SDKs into a closed-source product | ✅ Yes — Apache-2.0 permits this |

## Contributing and copyright

By submitting a pull request you agree that your contribution will be licensed under whichever
license applies to the directory you are contributing to (AGPL-3.0 or Apache-2.0, as shown in
the table above). You retain copyright on your own work; the license grants the project and its
users the right to use, modify, and distribute it under those terms.

See the [Contributing guide](./index.md) for the development workflow.