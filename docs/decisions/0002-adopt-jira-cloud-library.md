# 0002: Adopt A Jira Cloud Library For Broad Jira Support

Date: 2026-06-06

## Status

Accepted

## Context

The product goal is broader than a small issue search tool. Jira TUI should eventually support
day-to-day Jira work from the terminal, including:

- Viewing and editing issues.
- Creating issues and subtasks.
- Working with comments and descriptions.
- Transitioning issue status.
- Working with epics, sprints, boards, and related Jira Software concepts.

Administrative actions such as creating projects, users, permission schemes, and global Jira
configuration are not goals.

The initial custom REST client is fine for proving out the TUI, but a hand-rolled client will become
maintenance drag as the app grows into broad Jira support.

## Decision

Prefer a maintained Jira Cloud client library over expanding a custom Jira REST client.

The current preferred candidate is `go-atlassian`, especially its Jira v3 package, because it is
focused on Atlassian Cloud and documents support for Jira v3, ADF, issue creation, issue updates,
comments, transitions, and Jira Software workflows.

Keep `internal/jira` as our application boundary. The TUI should depend on our own small domain
types and methods, not directly on SDK structs. This keeps the UI stable if the library choice
changes later.

## Consequences

- Future Jira API expansion should first check whether `go-atlassian` supports the needed endpoint.
- The existing custom client should be migrated behind `internal/jira` rather than expanded endpoint
by endpoint.
- For unsupported endpoints, use the library's raw/custom request escape hatch if available before
adding unrelated HTTP client code.
- Tests should cover our adapter behavior, not the upstream library internals.

