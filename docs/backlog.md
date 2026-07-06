# Backlog

GitHub Issues is the public source of truth for user-visible backlog, bugs, feature requests, and
release-oriented work:

- [jcharette/jira-tui issues](https://github.com/jcharette/jira-tui/issues)

This file is the local curated index. Keep it short and use it to preserve sequencing, grouping, and
engineering context that should stay close to the codebase.

## Public Backlog Index

### Claude AI Workflows

- [#6 Polish existing Claude AI workflows before provider-neutral expansion](https://github.com/jcharette/jira-tui/issues/6)

## Local-Only Backlog Guidance

- Metadata-backed Ticket Actions should only ship when Jira exposes a concrete supported field or
  operation that is not already covered by generic edit, comments, status, links, worklogs, create,
  assignee, labels, components, summary, priority, or Sprint Actions.
- Multi-site Jira profiles stay Maybe Later unless real usage shows people regularly need to switch
  between unrelated Jira tenants from the same install.
- Jira OAuth/device authorization is intentionally not planned. API-token auth remains the supported
  path because tokens are keychain-backed and Atlassian's OAuth path is more complex than API-token
  setup for this CLI. See [ADR 0010](decisions/0010-api-token-auth-over-oauth.md).
- Full Jira development-panel integration stays Maybe Later. V1 git workflows should link work with
  Jira keys in branches/commits/PRs and short confirmed Jira comments containing branch or PR URLs.

## Open Product Questions

- Should the app optimize first for personal assigned work, team triage, or project/release
  management?
