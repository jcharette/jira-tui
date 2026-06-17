# JQL Query UX Design

## Goal

Add an in-app query workflow that lets users run a new Jira issue search without restarting. The
workflow must support direct raw JQL entry and AI-assisted JQL generation with preview, revision
feedback, and explicit confirmation before any generated query runs.

## Scope

- Table-mode `/` opens a query modal.
- The modal has two modes:
  - `JQL`: direct raw JQL editing.
  - `AI`: natural-language prompt for generating JQL.
- Direct JQL applies only when the user confirms with `ctrl+s`.
- AI generation uses the existing provider-neutral AI request path and local Claude runner.
- AI output is parsed into a JQL candidate, shown as preview text, and never run automatically.
- Users can revise AI output by editing the prompt and submitting again.
- `enter` from a generated preview copies the candidate into the direct JQL editor for review.
- `ctrl+s` from the preview runs the generated candidate.
- `esc` cancels the modal or cancels an in-flight AI request.

## Architecture

- Keep all Jira reads on the existing `KindSearchIssues` worker path.
- Add query workflow state to `Model` because the existing app keeps modal state there.
- Add `internal/tui/query.go` for query modal rendering, key handling, AI prompt construction,
  result parsing, and apply helpers.
- Add `events.AIOperationGenerateJQL` so Diagnostics can distinguish query generation from ticket
  assist and create-draft AI tasks.
- Reuse Bubbles `textarea` for query and AI prompt entry.
- Reuse `detailNotice` for compact feedback instead of adding a new notification system.

## Data Flow

Direct JQL:

1. `/` opens modal with current `m.jql`.
2. User edits the JQL text.
3. `ctrl+s` trims and applies it.
4. Applying updates `m.jql`, clears saved-view selection to ad hoc mode, resets table-local
   filters/collapse/selection state, hydrates cached rows for that JQL if available, and starts a
   foreground search when needed.

AI-assisted JQL:

1. User toggles to AI mode with `tab`.
2. User enters a natural-language request.
3. `ctrl+s` submits a provider-neutral AI request with default project, current JQL, saved view
   names, and strict output instructions.
4. Result parser extracts a single JQL candidate from plain text, fenced blocks, or `JQL:` lines.
5. The modal shows the generated candidate as preview.
6. User can revise by editing the prompt and submitting again, copy the preview into direct JQL with
   `enter`, or run it with `ctrl+s`.

## Error Handling

- Empty direct JQL does not apply and shows a notice.
- AI unavailable shows a notice and leaves the modal editable.
- AI failures stay in the modal and show a compact notice.
- Empty or unparsable AI output shows a notice and does not change `m.jql`.
- Stale AI results are ignored by request ID.

## Testing

- Direct JQL opens from the table and confirms into a foreground issue search.
- Empty direct JQL is rejected without changing `m.jql`.
- AI mode submits an AI task with `generate_jql`.
- AI result is previewed but not run automatically.
- AI preview can be confirmed into a search.
- AI prompt revisions can submit again using the current preview as context.
- Help/footer exposes the query workflow.
