# TUI Component Audit

This audit tracks custom TUI rendering and input code that may be replaced or wrapped with
maintained Bubble Tea, Bubbles, Lip Gloss, or compatible ecosystem libraries.

| Surface | Current code | Pain / risk | Candidate maintained primitive | Recommendation | Priority | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| Footer and keyboard help | `internal/tui/keymap.go`; `internal/tui/model.go`: `renderFooterHelpWithBindings`, `renderHelp`, `helpLines` | Custom binding type, footer truncation, help grouping, and manual help scrolling | Bubbles `key` and `help`, with a local adapter for context grouping | Migrated | Done | Footer rendering now uses Bubbles `help`; key metadata adapts to Bubbles `key.Binding`. |
| Config scalar text input | `internal/configui/model.go`: `updateEditor`, `renderFields` | Manual single-line edit buffer with append/backspace only and no cursor model | Bubbles `textinput` | First implementation candidate | P0 | Separate config model makes this easy to test. Keep custom bool/color picker rendering for now. |
| Assignee and mention pickers | `internal/tui/model.go`: `renderAssigneeDialog`, `updateAssigneePicker`, `renderMentionPicker`, `updateMentionPicker`; `internal/tui/choice_list.go` | Manual query strings, cursors, selection, filtering, result rendering, and async state glue | Bubbles `textinput` for query plus `list` or `table` for selectable results | In progress | P1 | Mention search and Assignee search now render through the shared Bubbles list-backed choice adapter. Query input is still manual and can move to `textinput` later. |
| Repeated action and option lists | `internal/tui/model.go`: `renderPriorityDialog`, `renderStatusSection`, `renderActionsSection`, `renderClaudeSection`; `internal/tui/choice_list.go` | Repeated selected-index, `>` marker, and table/list rendering patterns | Local wrapper over Bubbles `list` or `table` | Wrap later | P1 | Priority now renders through the shared choice-list adapter. Status, Actions, and Claude still have multi-column or action-state semantics that need separate review. |
| Create metadata option picker | `internal/tui/model.go`: `renderCreateIssueTypePickerLines`, `renderCreateDynamicField`, `updateCreateIssue`; `internal/tui/choice_list.go` | Long Jira option lists, custom filtering/windowing, repeated selection logic | Bubbles `list` and `textinput` via local adapters | Migrated | Done | Issue type selection and dynamic create option fields render through the shared choice-list adapter. Dynamic option filtering uses Bubbles `textinput` while preserving existing string filter state for matching. |
| Rich text and fitted tables | `internal/tui/model.go`: `wrapRichText`, `renderFittedTable`, `renderWrappedTableRow` | Custom table fitting and Markdown-ish parsing | Lip Gloss table helpers where fixture-compatible | Investigate later | P1 | Keep ADF fixture tests and the `internal/adf.Render` boundary. Existing table behavior is covered by real-shaped fixtures. |
| Dialog layout and editor configuration | `internal/tui/model.go`: `renderDetailDialog*`, `configured*Editor`, `new*Editor` | Repeated sizing and styling logic around existing Bubbles textareas | Internal wrapper around Bubbles `textarea` and Lip Gloss layout | Wrap later | P1 | The primitive choice is already good; cleanup should reduce duplication without changing behavior. |
| Detail scrolling | `internal/tui/model.go`: `newDetailViewport`, `renderScrollableDetailBody` | Custom line indicator around Bubbles viewport | Bubbles `viewport` | Keep with light wrapper | P2 | Existing primitive is appropriate. Avoid churn unless scroll behavior changes. |
| Main issue table | `internal/tui/model.go`: `renderIssueList`, `issueRows`, `renderIssueDisplayRow`, `tableRows` | Custom responsive columns, hierarchy tree, cursor visibility, viewport slicing | Possibly Bubbles `table`, or keep current renderer behind local adapter | Defer | P2 | High-risk core UX with many exact-width behaviors. Do not start here. |
| Loading indicators and activity bars | `internal/tui/model.go`: `detailStatusBlock`, `claudeActivityFrame`, `diagnosticActivityBar` | Static/manual spinner frames and ASCII activity bars | Bubbles `spinner` and possibly `progress` | Defer | P2 | Low code complexity, but tick integration must fit existing worker/Claude update flow. |

## First Slice Recommendation

Start with footer and keyboard help, or config scalar text input.

Footer/help is the best app-wide maintenance win because the key metadata already lives in
`internal/tui/keymap.go`, and Bubbles `key`/`help` can replace display/truncation mechanics without
changing Jira state. Config scalar input is the lowest-risk proof that simple manual input buffers
should move to Bubbles `textinput`.

Create metadata pickers should remain a near-term target, but they are no longer the first
recommended replacement. They have more user-facing edge cases and should be approached only after
the audit proves a smaller component migration pattern.

## First Slice Outcome

The first implementation slice moved config scalar text editing from a manual string buffer to
Bubbles `textinput`. This preserved the config field shape and custom boolean/color controls while
adding cursor-aware text editing for scalar fields.

## Second Slice Outcome

The second implementation slice adapted local key metadata to Bubbles `key.Binding` and routed
footer command rendering through Bubbles `help.ShortHelpView`. The full keyboard help overlay keeps
the existing grouped vertical layout and now consumes the same adapted key/help metadata.

Next target: migrate a bounded picker/list surface, starting with assignee or mention pickers before
the more complex create metadata picker.

## Third Slice Outcome

The third implementation slice introduced a shared Bubbles `list`-backed choice-list adapter and
migrated comment mention result rendering onto it. The adapter owns selected-row rendering,
pagination, and range indicators so future picker migrations have one reusable path instead of
one-off list renderers.

## Fourth Slice Outcome

The fourth implementation slice migrated Assignee search results onto the shared Bubbles
`list`-backed choice-list adapter. Assignee now shares the same selected-row rendering,
pagination, and range indicators as mention results while keeping its existing worker-backed Jira
search and submit flow.

## Fifth Slice Outcome

The fifth implementation slice migrated focused dynamic create option fields, such as Components,
onto the shared Bubbles `list`-backed choice-list adapter. Create field filtering, optional
unselected defaults, Jira-provided option matching, and submit behavior stayed unchanged while
selected-row rendering, pagination, and range indicators moved onto the shared adapter.

## Sixth Slice Outcome

The sixth implementation slice migrated create issue type selection onto the shared Bubbles
`list`-backed choice-list adapter. Issue type selection keeps the same keyboard movement and
metadata loading behavior while sharing selected-row rendering, pagination, and range indicators
with mention, assignee, and dynamic create option pickers.

## Seventh Slice Outcome

The seventh implementation slice moved dynamic create option filter input from manual string
append/backspace handling to Bubbles `textinput`. The existing `createDynamicFilters` map remains
the matching/submission source of truth, but focused picker input now has cursor-aware editing and
stays synchronized with the textinput model.

Next target: either defer lower-risk repeated action lists, or migrate a single repeated detail
option list such as Priority onto the shared choice-list adapter.

## Eighth Slice Outcome

The eighth implementation slice migrated the Priority picker onto the shared Bubbles
`list`-backed choice-list adapter. Priority keeps the same metadata-backed selection and submit
behavior while sharing selected-row rendering, pagination, and range indicators with the other
single-column picker surfaces.

Remaining repeated lists, such as Status transitions, Actions, and Claude actions, have richer
state or multi-column semantics and should be reviewed during the package/file boundary audit rather
than forced into the simple choice-list adapter now.
