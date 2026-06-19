# Comment Editing Design

## Goal

Allow users to edit an existing Jira comment from focused ticket detail with an explicit review step
and clear failure feedback.

## UX

- Selecting the Comments detail section and pressing `enter` focuses the comment list when comments
  are loaded; if there are no comments, it keeps opening the add-comment composer.
- While the comment list is focused, `j/k` selects a comment, `enter` adds a new comment, `e` edits
  the selected comment, and `esc` leaves comment focus.
- Editing reuses the existing comment composer surface with the selected comment body prefilled.
- `ctrl+s` or `tab` opens review, `y` updates the Jira comment, `n` returns to editing, and `esc`
  cancels without writing.
- Successful updates refresh comments and invalidate retained comment cache entries, matching
  add-comment behavior.
- This keeps the current keyboard flow intact for this slice, but the follow-up deep UX review
  should reduce text-heavy view chrome, make selections more visual, and consider color/icon cues or
  table/dropdown-style controls where they reduce memorized key usage.

## Data Flow

- The TUI tracks comment focus, selected comment index, edit issue key, edit comment ID, and original
  body.
- The worker pool exposes `KindUpdateComment`, carrying issue key, comment ID, body, and selected
  mentions.
- The Jira client uses go-atlassian's ADF comment update endpoint and the existing plain-text to ADF
  conversion used by comment creation.

## Error Handling

- Empty update bodies are blocked locally.
- Missing selected issue/comment IDs are blocked locally.
- Jira permission, stale ID, or validation errors surface as `Comment update failed: ...` while
  leaving the draft open.

## Out Of Scope

- Comment deletion.
- Ownership/permission prediction before submit.
- Rich preservation of the original ADF document beyond the existing text rendering/editing model.
