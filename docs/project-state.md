# Project State

Last updated: 2026-06-06

## Goal

Build a terminal-first Jira client for people who hate Jira's web interface.

The app should make common Jira workflows fast from the command line, starting with issue browsing
and eventually expanding into issue details, comments, transitions, assignment, and creation.
The intended scope includes normal Jira user workflows such as editing tickets, creating tickets,
subtasks, epics, sprints, boards, comments, and transitions. Jira administration is out of scope.
The feature roadmap and dependency-aware milestones live in [roadmap.md](roadmap.md).

## Current Stack

- Language: Go
- TUI framework: Bubble Tea v2 (`charm.land/bubbletea/v2`)
- Jira API: Jira Cloud REST API v3
- Jira library: `go-atlassian` behind `internal/jira`
- Concurrency: typed dispatcher and bounded worker pool powered by `ants`
- Authentication: `JIRA_EMAIL` + `JIRA_API_TOKEN` using Basic Auth
- Configuration: environment variables

## Current Commands

Run from the project root:

```bash
make run
```

Build a local binary:

```bash
make build-local
```

Run tests:

```bash
make test
```

Run the standard verification loop:

```bash
make check
```

## Current Environment Variables

Required:

- `JIRA_BASE_URL`: Jira Cloud site URL, for example `https://example.atlassian.net`
- `JIRA_EMAIL`: Jira account email
- `JIRA_API_TOKEN`: Jira API token

Optional:

- `JIRA_JQL`: default issue query
- `JIRA_REFRESH_INTERVAL`: background refresh cadence as a Go duration, for example `30s` or `2m`; set to `0` to disable
- `JIRA_REQUEST_TIMEOUT`: Jira request timeout as a Go duration, for example `10s`
- `JIRA_WORKERS`: number of background Jira workers
- `JIRA_QUEUE_SIZE`: background work queue size

Default JQL:

```text
assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC
```

## Current Behavior

- Loads up to 50 Jira issues matching the configured JQL.
- Refreshes issues in the background on a configurable interval.
- Routes Jira refresh work through a typed dispatcher and bounded worker pool powered by `ants`.
- Uses request IDs to ignore stale Jira responses.
- Preserves the selected issue across refreshes when it still exists in the refreshed list.
- Bounds Jira calls with a configurable request timeout.
- Displays issues in a Bubble Tea alternate-screen TUI.
- Supports moving through the issue list with `j`/`k` or arrow keys.
- Supports refresh with `r`.
- Supports quit with `q` or `ctrl+c`.

## Known Constraints

- The TUI currently shows only a list view.
- There is no issue detail panel yet.
- There is no config file yet.
- There is no OAuth flow; API token auth is the only supported auth mode.
- There is no git repository initialized in this folder yet.
- The Jira client uses `go-atlassian` for search, but only issue search is exposed through
  `internal/jira` so far.
- The worker pool currently supports issue search only; issue detail is the next workflow to add.
