# Assignable User Metadata Design

Creation and editing need permission-aware metadata so the TUI does not offer actions Jira will
reject. The first slice improves assignee editing.

## UX

The current `Change Assignee` picker stays intact: users type, see a compact selectable list, choose
with arrows, and press enter to apply. The result set changes from global Jira users to users Jira
reports as assignable to the selected issue. Empty, loading, error, stale-result, and cached-result
behavior stays quiet and local to the picker.

## Architecture

Add an issue-scoped `SearchAssignableUsers` Jira client method backed by Jira's assignable-user REST
endpoint. Extend the existing worker `SearchUsers` request with an optional issue key; when present,
the worker calls assignable search, otherwise it keeps global search for mentions. The TUI assignee
picker passes the selected issue key and caches assignee search results by `issueKey + query`.

## Constraints

- Do not change mention search behavior.
- Do not submit Jira writes until the user selects a user and confirms with enter.
- Do not share global user search cache entries with issue-scoped assignee search entries.
- Keep assignee picker rendering and key bindings unchanged in this slice.
