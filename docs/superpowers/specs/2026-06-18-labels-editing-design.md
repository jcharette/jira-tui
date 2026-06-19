# Labels Editing Design

## Goal

Allow users to edit Jira Labels from focused ticket detail through the existing action surfaces.

## UX

- The Actions section and Ticket Actions palette expose `Edit Labels`.
- Running the action loads Jira edit metadata when needed.
- If Jira reports `labels` as editable, a bounded `Edit Labels` dialog opens with the current labels
  as comma-separated text.
- `enter` saves, `esc` cancels, and unchanged label sets do not submit.

## Architecture

- Extend `jira.EditMetadata` with a `Labels` field parsed from Jira edit metadata.
- Add `UpdateLabels(ctx, key, labels)` to the Jira client and worker pool.
- Add Labels editor state to the TUI model and keep editor helper code isolated in a small
  same-package file.
- Reuse existing detail action dispatch so the Actions section and palette stay in sync.

## Non-Goals

- No label autocomplete or suggestion API.
- No generic edit-all-fields framework.
- No create-field option source expansion in this slice.

## Verification

- Jira client tests cover metadata parsing and update payloads.
- Worker tests cover request/result routing.
- TUI tests cover action launch, modal rendering, submit state, and success patching.
