# Issue List Subtree Collapse Design

## Purpose

The issue table already renders loaded Jira issues as a tree and supports explicit child loading with
`x` and `X`. Dense branches can dominate the viewport, making it hard to focus on a smaller area of
work. Add a purely local UX collapse/expand control so users can hide or reveal a selected node's
descendant rows without changing Jira reads, saved views, cache records, issue ordering, or the
underlying loaded issue set.

## Scope

- Add subtree collapse/expand behavior to the main issue table only.
- Keep all loaded issues in memory; collapse only changes visible rendered rows.
- Preserve today's default: all rows render expanded until the user collapses a node.
- Preserve explicit expansion behavior: `x` and `X` still load open/all children for the selected
  issue through the existing worker path.
- Preserve collapse state by issue key across refreshes where the key still exists.
- Keep this in the existing TUI issue-list/navigation files unless implementation proves a small
  same-package helper file is clearer.

## Interaction

When the selected issue has descendants in the current loaded tree, a collapse toggle collapses that
node's entire visible subtree. Toggling the same node again expands that node only. Deeper collapsed
branches keep their own state, so reopening a parent does not unexpectedly expand every descendant.

If the selected row becomes hidden because an ancestor was collapsed, selection moves to the
collapsed ancestor. Normal list movement operates over visible rows only, so hidden descendants are
skipped until their ancestor is expanded again.

Collapsed rows show a compact descendant count such as `12 hidden`, fitted into the existing table
row detail space. Missing-parent context groups may also be collapsed as a presentation group, using
the same principle: the placeholder row remains visible while its child rows are hidden.

Key binding should be chosen after auditing the current issue-table bindings. Prefer one clear
toggle command before adding bulk commands. Collapse-all and expand-all are out of scope for the
first implementation unless the keymap audit finds an obvious, low-conflict command.

## Data Model

Add model-local view state keyed by issue key, for example `collapsedIssueKeys map[string]bool`.
This state is not persisted, not sent to Jira, and not written into cache records. It is discarded
when the app exits.

Rendering should derive a visible row list from the existing issue tree plus collapse state. The
derived row list should carry the original `m.issues` index so existing open/detail behavior can keep
using loaded issue data without copying or mutating it.

## Rendering And Navigation

The issue-list renderer should continue building the tree from `m.issues`, then skip descendants of
collapsed nodes. Hidden descendant counts should be computed from loaded tree rows only. The row
title count can continue to show loaded issue count; the viewport row range should continue to use
visible rendered rows.

Navigation, paging, and selection visibility need to clamp against visible rows rather than raw issue
indexes when collapse state hides rows. Opening a selected visible row should still open the matching
loaded issue.

## Error Handling

Collapse is a local presentation feature, so there should be no new Jira error states. If the user
toggles collapse on a leaf row, show a short existing-style notice that no loaded child issues are
available for that ticket. If refresh removes a collapsed key, stale collapse state may remain
harmlessly in the map and should not affect rendering.

## Testing

Add focused TUI tests for:

- default expanded rendering stays unchanged;
- collapsing a parent hides all loaded descendants and shows a hidden count;
- expanding a collapsed parent restores direct children while preserving deeper collapsed branches;
- collapsing an ancestor moves selection to that ancestor when the current selection was hidden;
- navigation skips hidden rows;
- explicit `x`/`X` child loading continues to merge issue rows without clearing collapse state.

Run focused issue-list/navigation tests first, then the standard repo verification loop before
marking implementation complete.
