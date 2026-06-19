# Backlog

GitHub Issues is the public source of truth for user-visible backlog, bugs, feature requests, and
release-oriented work:

- [jcharette/jira-tui issues](https://github.com/jcharette/jira-tui/issues)

This file is the local curated index. Keep it short and use it to preserve sequencing, grouping, and
engineering context that should stay close to the codebase. Do not duplicate full issue bodies here.

Docs are part of done. When user-visible backlog changes, update the matching GitHub issue and this
index in the same change. When an item lands, close or update the issue and add a matching entry to
[releases/CHANGELOG.md](releases/CHANGELOG.md).

## Public Backlog Index

### Security And Auth

- [#2 Add Jira OAuth or device authorization](https://github.com/jcharette/jira-tui/issues/2)
- [#3 Store Jira credentials in the OS keychain](https://github.com/jcharette/jira-tui/issues/3)

### Git And AI Workflows

- [#6 Expand AI workflows behind provider-neutral ai.task events](https://github.com/jcharette/jira-tui/issues/6)

## Local-Only Backlog Guidance

- Add more metadata-backed Ticket Actions only when Jira workflows expose a concrete supported field
  or operation that is not already covered by generic edit, comments, status, links, worklogs,
  create, assignee, labels, components, summary, or priority.
- Multi-site Jira profiles stay Maybe Later unless real usage shows people regularly need to switch
  between unrelated Jira tenants from the same install.
- Full Jira development-panel integration stays Maybe Later. V1 git workflows should link work with
  Jira keys in branches/commits/PRs and short confirmed Jira comments containing branch or PR URLs.

## Open Product Questions

- Should the app optimize first for personal assigned work, team triage, or project/release
  management?
