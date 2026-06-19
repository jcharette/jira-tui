# View Creation UX Design

Users should be able to create and maintain saved issue views without knowing where query history
lives or editing TOML by hand.

## UX

- `v` from the issue table opens a save-view prompt for the currently active JQL.
- In the query modal, `ctrl+v` saves the direct JQL draft or generated AI preview as a view.
- In list modes, `s` saves the selected recent query or selected template as a view.
- The save prompt shows the proposed view name, the JQL that will be saved, and an
  `include_children` toggle. Saving must not run Jira, change the active JQL, or switch views.
- The query modal adds `Templates` and `Views` modes. Templates provide common starter JQL views
  scoped to the project found in the current JQL. Views lets users rename, reorder, delete, and
  toggle `include_children` for existing saved views.

## Architecture

The query modal remains the single surface for query/view composition. Saved-view creation reuses
`config.AddSavedView` and the existing config writer. View management adds a full saved-views writer
that validates and persists the complete `[]config.IssueView` list through the same config file.

## Constraints

- Do not add Jira requests for create/manage actions.
- Do not change active JQL, selected view, loaded issues, or caches when saving/managing views.
- Keep view persistence in config; recent query history remains cache-backed.
