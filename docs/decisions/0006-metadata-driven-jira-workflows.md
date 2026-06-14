# 0006: Use Jira Metadata For Write Workflows

Date: 2026-06-13

## Status

Accepted

## Context

Jira sites vary heavily by organization and project. Issue types, priorities, statuses, required
fields, workflows, users, boards, sprints, and custom fields can all be configured differently.

Previous AI-assisted tooling made generic Jira assumptions and required expensive rewrites when
those assumptions did not match the real Jira instance. This app should avoid that failure mode as
it grows from read-only browsing into creation, editing, assignment, sprint, workflow, git, and AI
assisted features.

## Decision

All Jira write workflows must be driven by Jira metadata for the active site, project, and issue.

Create issue flows must use Jira create metadata to determine available issue types, required
fields, field schemas, and allowed values. Edit flows must use edit metadata for the selected issue.
Transition flows must use transition metadata, including required transition fields. Assignment
flows must use assignable-user search and account IDs. Sprint, board, priority, status, and custom
field behavior must come from Jira responses instead of hard-coded product assumptions.

The TUI may provide sane display defaults, but it must not submit Jira writes based on fixed lists
of generic Jira values. If the needed metadata endpoint is not exposed through `internal/jira`, add
that adapter method before adding the write UI.

## Consequences

- Write workflows require more API plumbing before UI forms are useful.
- Tests should use fake Jira metadata so custom fields, required fields, and alternate workflows are
  covered from the start.
- Metadata caching is allowed, but invalidation must be explicit because Jira admins can change
  project and workflow configuration outside this app.
- Git and AI-assisted workflows that update Jira must still route through the same metadata-driven
  validation and explicit confirmation paths.
