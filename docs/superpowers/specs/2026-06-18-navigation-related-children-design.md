# Navigation Related Children Design

## Goal

Complete the next Navigation and Query slice by making automatic child issue loading happen only
for saved views that are intended to show hierarchy, starting with the default Epics view.

## Current Behavior

The worker already normalizes issue searches by fetching missing parent issues, adding subtasks
that Jira includes on parent issue payloads, and allowing explicit `x`/`X` expansion from the
selected row. It also currently performs an automatic `parent in (...)` child lookup for every
non-subtask issue in every search result.

That behavior makes hierarchy-rich views useful, but it adds extra Jira reads to ordinary assigned,
project, sprint, and watching views even when those views are not asking for children.

## Design

Add `IncludeChildren bool` to `config.IssueView`. The config TOML field is
`include_children`, defaulting to false when omitted. `DefaultViews` marks only the Epics view with
`IncludeChildren: true`.

The TUI passes the active view's `IncludeChildren` flag into `worker.SearchIssuesRequest`. Direct
JQL searches and saved views created from query history default to false unless the user later edits
the config file and opts the view into child loading.

The worker keeps existing missing-parent and known-subtask normalization for all searches. It gates
the extra automatic `parent in (...)` child lookup behind `SearchIssuesRequest.IncludeChildren`.
Explicit `x` and `X` expansion remains available for every visible issue and continues to use the
existing expanded-children cache.

## Error Handling

If the include-children lookup fails, the worker preserves the current best-effort behavior: return
the base search results rather than failing the whole view. Explicit expansion still reports errors
through the existing expand result path.

## Testing

Add tests that prove:

- The default Epics view opts into child loading.
- Config load/save round-trips `include_children`.
- Worker search skips automatic child lookup unless `IncludeChildren` is true.
- TUI search submissions pass the active view flag into the worker request.

## Out Of Scope

- Sprint and board APIs.
- Recursive descendant loading beyond Jira's direct `parent in (...)` behavior.
- New UI controls for toggling `include_children` at runtime.
