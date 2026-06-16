# Package And File Boundary Audit

Date: 2026-06-16

## Scope

Evaluate whether the codebase is drifting toward monolithic files or unclear internal libraries
after the TUI consistency work. This audit uses file size, package cohesion, dependency direction,
test locality, repeated helper patterns, and current change concentration as evidence. The goal is
not to split files for its own sake; the goal is to reduce future bug risk while keeping one
consistent way to build Jira workflows.

## Current Shape

| Area | Evidence | Assessment |
| --- | --- | --- |
| `internal/tui/model.go` | Started at 10,540 lines; now 1,105 lines after same-package workflow splits. It owns core app state, constructor/options, `Init`, `Update`, and top-level `View`/`render` dispatch | Reduced to core model boundary |
| `internal/tui/model_test.go` | Started at 8,057 lines; now 658 lines after workflow test splits. It keeps shared fakes/helpers and core model/update tests | Reduced to shared test harness |
| `internal/jira/client.go` | 1,400 lines; Jira API adapter, model conversion, metadata parsing, ADF access, and fallback services | Large but cohesive |
| `internal/worker/pool.go` | 1,075 lines; request/result DTOs, pool mechanics, and all Jira handlers | Cohesive today; likely future scheduler boundary |
| `internal/configui/model.go` | 892 lines; config editor state, update, rendering, validation, and connection test | Medium size and cohesive |
| `internal/adf/render.go` | 442 lines; ADF renderer boundary plus table/code compatibility logic | Acceptable |

Package dependency direction is currently healthy: `internal/tui` composes app-facing packages,
`internal/worker` depends on `internal/jira`, and `internal/jira` hides `go-atlassian` plus ADF
conversion details. There are no obvious cycles. The risk is concentrated in file-level
responsibility inside `internal/tui`, not package-level coupling.

## Recommendations

### Keep `internal/tui/model.go` focused on the core model

Use same-package workflow files before creating new packages. This preserves the current Bubble Tea
`Model` boundary, avoids import cycles, and keeps future boundary changes mechanical enough to
review safely.

Completed same-package splits:

- `commands.go`: worker-backed command constructors such as search, detail, comments, transition,
  metadata, create, and update submissions.
- `diagnostics.go`: diagnostic event recording, overlay rendering, and activity helpers.
- `issue_list.go`: issue list layout, hierarchy tree rows, responsive columns, sorting, and table
  rendering.
- `comment.go`: comment composer, mention picker, detected links, submit/cancel behavior, and
  comment editor helpers.
- `create_issue.go`: create issue state, metadata-backed field rendering, typeahead/filtering,
  supported field helpers, create submission, and create-ticket AI support.
- `claude_assist.go`: ticket-detail Claude plan/assist/inline AI flows, progress handling, apply,
  comment, copy, and refinement helpers.
- `detail.go`: detail target/section helpers, detail focus, hierarchy, links, scrolling, dialogs,
  and detail request state transitions.
- `rich_text.go`: rich description/comment body wrapping, fitted Markdown tables, and compact
  code-block rendering.
- `chrome.go`: browser layout, header, filter summary, footer/help rendering, and issue-row counts.
- `results.go`: worker result handling, cache freshness updates, and user-search cache updates.
- `navigation.go`: refresh, view/sort switching, selection movement, issue replacement, and detail
  scrolling.
- `format.go`: shared formatting, badges/styles, truncation/padding, project-key extraction, and
  selection-window helpers.
- `summary.go`: summary editor textarea helpers.
- `external.go`: external browser and clipboard helpers.

Keep `choice_list.go` and `keymap.go` as they are. They are already small, cohesive examples of the
direction we want.

The split is complete for the original monolith concern: `internal/tui/model.go` dropped from
10,540 to 1,105 lines without changing the `package tui` boundary or the Bubble Tea `Model`
ownership model.

### Keep TUI tests near their workflow files

The tests now follow the same ownership so regressions stay close to the behavior they protect:

- `detail_test.go`
- `comment_test.go`
- `create_issue_test.go`
- `claude_assist_test.go`
- `issue_list_test.go`
- `rich_text_test.go`
- `diagnostics_test.go`

Shared fakes and cross-workflow model tests remain in `model_test.go`. That keeps the workflow
test files focused without creating a second helper-only file before it is needed.

### Defer new internal packages for TUI components

Do not create `internal/tui/components` yet. The current reusable pieces are still tightly coupled
to `Model`, theme, footer contexts, and Jira workflow state. Continue extracting small same-package
adapters first. Promote an adapter to its own package only when it has at least two independent
callers and no dependency on app-specific model state.

### Keep `internal/jira` as the Jira adapter boundary

`internal/jira/client.go` is large, but it is cohesive: it shields the rest of the app from
`go-atlassian`, raw ADF payload shapes, metadata JSON, and Jira API quirks. Split by endpoint family
only when churn continues:

- `issue.go`
- `comments.go`
- `metadata.go`
- `users.go`
- `adf_fixture.go`
- `parse.go`

Do not move Jira parsing into the TUI or worker packages.

### Keep `internal/worker` cohesive until cache/scheduler work starts

`internal/worker/pool.go` is a useful boundary today: the TUI submits typed requests and receives
typed results. The next real pressure will come from the planned large-view cache and background
sync work. At that point, split first by responsibility:

- `types.go`: request/result DTOs and `Kind` constants.
- `pool.go`: `ants` pool, admission, lifecycle, submit, and result plumbing.
- `handlers.go`: current Jira request handlers.

If priority queues, stale-while-refresh, persistent storage, or background sync policy become
substantial, design a separate scheduler/cache package before implementation.

### Do not split smaller cohesive packages now

`internal/configui`, `internal/config`, `internal/adf`, `internal/claude`, `internal/linkdetect`,
`internal/mentiondetect`, and `internal/ui` are not current boundary problems. They can split later
when feature growth gives them a clearer second responsibility.

## Next Work

1. Keep future TUI workflow additions in the existing workflow files instead of adding unrelated
   behavior back to `model.go`.
2. If any split file approaches the same mixed-responsibility shape again, split mechanically inside
   `package tui` before introducing new packages.
3. Revisit package boundaries only when a helper has at least two independent callers and no direct
   dependency on app-specific `Model` state.
4. Reassess `internal/worker` during the planned cache/scheduler work, where priority queues and
   freshness policy may justify a new boundary.
