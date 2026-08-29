# Security Policy

## Reporting a vulnerability

**Do not open a public issue.** Use the private
[GitHub Security Advisory form](https://github.com/arandu-io/vscode-arandu/security/advisories/new).

The advisory form notifies the maintainers and keeps the report private while
the fix and coordinated disclosure are prepared.

## What to expect, in time

| Step | Deadline |
|---|---|
| Acknowledgement | 72 hours |
| Triage and severity (CVSS) | 7 days |
| Fix, or a plan with a date | 30 days |
| Coordinated disclosure | 90 days, or sooner once a fix exists |

If the flaw is being exploited, the embargo ends: fix and notice go out
immediately.

## Supported versions

While the extension is on `v0.x`, only the latest minor is supported.

## Supply chain

- The installed extension has no extension-host runtime or dependency graph
- CI creates the VSIX twice and requires byte-identical output
- A Go auditor rejects every file outside the release allowlist
- The packaging tool version is pinned and runs without a repository lockfile
