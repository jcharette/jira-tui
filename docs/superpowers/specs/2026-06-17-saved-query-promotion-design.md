# Saved Query Promotion Design

## Goal

Users can promote a useful recent direct or AI-generated JQL query into a durable named saved view.
The saved view appears in normal saved-view rotation and is written to the existing config file.

## UX

- In the query modal's `Recent` mode, pressing `s` starts a compact save prompt for the selected
  recent query.
- The prompt asks for a view name. `ctrl+s` or `enter` saves the selected recent query under that
  name. `esc` cancels the prompt and returns to `Recent` mode.
- Blank names are rejected.
- Duplicate saved-view names are rejected case-insensitively.
- Saving a view does not run Jira, change the active query, or switch the current view. It only adds
  the view to rotation and writes config.

## Architecture

- Add a small config helper that appends a saved view after trimming input and checking duplicate
  names.
- Add a TUI writer callback option so `main` can capture the loaded config and config path and call
  `config.Save`.
- Keep the TUI state local to the query modal: a save prompt flag, name draft, and text input.
- After a successful save, append the saved view to `m.views` so `tab` rotation can find it
  immediately.

## Data Flow

1. App loads config as today.
2. `main` resolves the config path and passes `WithSavedViewWriter`.
3. User opens `/`, tabs to `Recent`, selects a recent query, presses `s`, enters a name, and saves.
4. TUI validates and calls the writer with `config.IssueView{Name, JQL}`.
5. Writer appends the view to captured config and calls `config.Save`.
6. TUI appends the view in memory and shows a notice.

## Error Handling

- No selected recent query: show `No recent queries yet.`
- Blank name: show `saved view name is required`.
- Duplicate name: show `saved view "<name>" already exists`.
- Missing writer: show `Saved-view persistence is not available.`
- Write failure: show `Saved view failed: <error>`.

## Testing

- Config helper tests for append, trim, blank rejection, duplicate rejection, and preserving active
  view.
- TUI tests for opening the save prompt from `Recent`, rejecting duplicates, successful save calling
  the writer, in-memory view append, and no Jira search command.
- Main wiring test can exercise the writer closure by saving to a temp config path if the command
  structure already supports it; otherwise rely on config helper and TUI option tests.
