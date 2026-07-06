# Roadmap

Jira TUI is at its `1.0.0` baseline: daily Jira browsing, ticket detail, Jira write workflows,
Git/GitHub developer workflows, notifications, local Claude assistance, caching, Diagnostics, and the
current security posture are implemented.

## Current Product Pillars

- Fast Jira triage from saved views, direct JQL, layouts, filtering, sorting, and hierarchy controls.
- Focused ticket detail for descriptions, comments, hierarchy, links, worklogs, and metadata-backed
  actions.
- Jira writes through reviewed, worker-backed flows using Jira metadata rather than hard-coded field
  assumptions.
- Developer workflows through `jira start`, `jira commit`, and `jira finish` with explicit review
  before Git, Jira, or GitHub writes.
- Optional local Claude assistance behind feature flags and write gates.
- Local security controls: keychain-backed tokens, HTTPS-only Jira URLs, owner-only local files,
  sanitized Diagnostics, and documented residual risks.

## Active Direction

### M0: Foundation

Status: complete (2026-06-19).

### M1: Jira TUI Baseline

Status: complete (2026-06-19).

### M2: Claude AI Workflow Polish

Status: planned.

### Claude AI Workflow Cleanup

Issue #6 tracks the next product direction: make the existing Claude-backed AI workflows clearer and
more useful before adding provider-neutral execution. Provider-neutral routing, Codex support, and
future providers stay deferred until the Claude-only workflow stack is cohesive.

### Security And Auth

API-token auth remains the supported path. OAuth/device authorization is not planned unless Atlassian
offers a CLI-friendly device flow or real users require OAuth because API tokens are blocked by org
policy. See [ADR 0010](decisions/0010-api-token-auth-over-oauth.md).

### Future Workflow Depth

Future Jira workflow work should be driven by concrete missing Jira metadata-backed operations rather
than broad action-menu expansion. If Jira exposes a field or transition shape safely, add it behind
metadata discovery, explicit validation, and worker-backed writes.
