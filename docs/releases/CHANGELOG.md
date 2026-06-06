# Changelog

All notable changes to this project should be recorded here.

## Unreleased

- Added a dependency-aware product roadmap with milestones for issue details, comments, workflow actions, config, planning views, creation/editing, and power-user workflows.
- Adopted `ants/v2` as the execution engine under the internal worker dispatcher.
- Added a library-first planning rule for non-core infrastructure.
- Added an internal typed dispatcher and bounded worker pool for Jira background work.
- Routed issue search refreshes through the worker pool.
- Added configurable Jira worker count and queue size.
- Planned a small typed dispatcher and bounded worker pool as the next concurrency step.
- Added a Makefile for repeatable format, test, build, run, tidy, clean, and check workflows.
- Made issue refresh use an explicit goroutine and result channel at the TUI boundary.
- Recorded channel-backed background work as the concurrency pattern for future Jira workflows.
- Added configurable background issue refresh with stale-response protection.
- Added configurable Jira request timeout.
- Preserved selected issue across refreshed issue lists.
- Removed generated build artifact from the project tree.
- Migrated Jira issue search from custom HTTP code to `go-atlassian` Jira v3.
- Accepted `go-atlassian` as the preferred Jira Cloud library direction for broad Jira support.
- Added a working agreement that makes docs, planning, backlog, and changelog updates part of done work.
- Started Go/Bubble Tea Jira TUI project.
- Added Jira Cloud issue search client.
- Added env-based configuration.
- Added first issue list TUI with navigation and refresh.
- Added initial tests for config and Jira client.
- Added project docs for planning, backlog, release notes, and decisions.
