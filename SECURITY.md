# Security Policy

## Supported versions

Only the latest release receives security fixes.

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report vulnerabilities by email to **dejan.radmanovic@vicert.com**. Include:

- A description of the vulnerability and its potential impact
- Steps to reproduce or a proof-of-concept
- Any suggested mitigations you have identified

You will receive an acknowledgement within 48 hours and a more detailed response within 7 days. We will coordinate a fix and disclosure timeline with you.

## Scope

Security concerns most relevant to this project:

- **Secret leakage** — API keys or credentials appearing in generated code, logs, or error messages
- **Code injection** — malicious event spec YAML leading to code injection in generated wrappers
- **Dependency vulnerabilities** — vulnerabilities in Go module dependencies

## Out of scope

- Issues in provider APIs themselves (Amplitude, PostHog, etc.) — report those to the respective vendors
- Issues that require physical access to the developer's machine