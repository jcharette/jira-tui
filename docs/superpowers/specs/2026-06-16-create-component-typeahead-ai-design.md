# Create Component Typeahead And AI Guessing Design

## Context

Create-ticket metadata can return hundreds of component options. The current create form renders a
bounded picker window and supports only `j/k` movement, which makes selecting a known component
slow. Claude-assisted ticket creation already parses named draft sections and can apply matching
Jira metadata-backed field values, but the prompt does not explicitly list available Components.

## Goal

Make component selection fast and let Claude suggest a Jira-supported component when generating a
ticket draft.

## Scope

- Add typeahead filtering for Jira option-backed create fields, including Components.
- Keep Jira metadata as the only source of allowed component values.
- Add an `Available Components` section to the create-ticket AI prompt when a Components field is
  available.
- Reuse the existing `Components:` draft parsing and option matching path to auto-select a matching
  component.

## Behavior

When a picker field is focused, printable characters update a local filter query. The visible picker
list is filtered by option name or ID. `j/k` and arrow keys move within filtered results. `enter`
selects the highlighted filtered option and clears the filter. `backspace` edits the filter. `esc`
clears the filter first; when no filter is active, it keeps the existing create-modal cancel
behavior.

Claude prompts include only Jira-returned component names under `Available Components`. Claude may
return `Components:` with a Jira-supported component or `Unknown`. If the recommendation matches,
the form selects that component after create metadata loads. If it does not match, no arbitrary
component is selected.

## Verification

- Focused tests for picker filtering, filtered selection, and clearing the filter.
- Focused tests that AI prompts list available Components.
- Focused tests that matching AI `Components:` output selects the correct Jira option and unknown
  output does not select a random option.
- Standard verification with `go test ./internal/tui`, `go test ./...`, `make check`, and
  `make install-user`.
