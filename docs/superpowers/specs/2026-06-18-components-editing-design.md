# Components Editing Design

## Goal

Allow users to edit Jira Components from focused ticket detail through the existing action surfaces.

## UX

- The Actions section and Ticket Actions palette expose `Edit Components`.
- Running the action loads Jira edit metadata when needed.
- If Jira reports `components` as editable and returns allowed values, a bounded multi-select
  `Edit Components` dialog opens.
- Users type to filter options, move with `j`/`k`, toggle the highlighted component with `space`,
  save with `enter`, and cancel with `esc`.

## Architecture

- Extend `jira.EditMetadata` with a `Components` field parsed from Jira edit metadata.
- Add `UpdateComponents(ctx, key, components)` to the Jira client and worker pool.
- Add Components picker state to the TUI model and keep picker helper code isolated in a small
  same-package file.
- Reuse existing detail action dispatch so Actions and Ticket Actions stay in sync.

## Non-Goals

- No component creation.
- No project component discovery outside Jira edit metadata.
- No generic edit-all-fields framework.

## Verification

- Jira client tests cover metadata parsing and update payloads.
- Worker tests cover request/result routing.
- TUI tests cover action launch, modal rendering/filtering/toggling, submit state, and success
  patching.
