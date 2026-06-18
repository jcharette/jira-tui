# Comments And Workflow Actions Design

## Goal

Make the app reliable for daily ticket response and status movement by improving comment formatting
and supporting Jira transition-screen fields that block status changes.

## Approach

Use the existing focused ticket detail workflows rather than adding a new command mode. Comments
stay in the bounded textarea composer. Formatting uses visible markdown-style tokens inserted by
keyboard controls and converted into Jira ADF marks before posting. This keeps pasted terminal text
predictable while giving Jira correct `strong`, `em`, and `code` marks.

Transitions keep the current Status section and Actions routing. Loading transitions must request
Jira transition metadata with `expand=transitions.fields`, parse supported transition-screen fields,
and cache those fields with the transition list. Applying a transition with required supported
fields opens a small field form before submitting. Unsupported required fields block submission
with a clear notice rather than sending a doomed Jira request.

## Supported Scope

- Comment formatting supports bold (`**text**`), italic (`_text_`), and inline code
  (`` `text` ``) conversion to Jira ADF marks.
- Comment composer keyboard controls insert bold, italic, inline-code, and bullet-list tokens.
- Existing link detection and selected Jira mentions must continue to produce ADF link marks and
  mention nodes.
- Transition metadata supports required Resolution fields as a Jira field value.
- Transition metadata supports transition Comment fields as a Jira `update.comment.add` operation
  with the same ADF conversion used by normal comments.
- Unsupported required transition fields block submission and identify the blocking field names.

## Out Of Scope

- Full rich-text editing, mouse selection, or hidden WYSIWYG controls.
- Arbitrary custom transition fields.
- Comment editing or deleting.
- Transition field persistence separate from the existing transition cache shape.
- New Jira read families outside transition metadata expansion.

## Files

- `internal/jira/client.go` parses transition fields, builds transition payloads, and converts
  formatted comment text to ADF.
- `internal/worker/pool.go` carries transition field values through the worker-backed write path.
- `internal/tui/detail.go`, `internal/tui/commands.go`, `internal/tui/results.go`, and
  `internal/tui/model.go` own the transition field form state, rendering, submission, and results.
- `internal/tui/comment.go` owns comment formatting controls.
- `internal/tui/keymap.go` documents the new composer controls.
- Focused tests live next to the existing Jira, worker, and TUI tests.

## Testing

Add failing tests before implementation for:

- Jira transition metadata parsing from `transitions.fields`.
- Jira transition submission including resolution and transition comment payloads.
- Worker request/result propagation of transition field values.
- TUI blocking unsupported required transition fields.
- TUI opening and submitting a required resolution/comment transition form.
- Comment ADF marks for bold, italic, and inline code while preserving links and mentions.
- Comment composer formatting keyboard controls.
