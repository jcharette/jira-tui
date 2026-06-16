# Hand-Rolled TUI Component Audit Design

## Context

The app has enough creation, detail, AI, comment, and diagnostics surfaces that the TUI layer needs
strict consistency around rendering and input behavior. At the start of this audit,
`internal/tui/model.go` was over 10,000 lines and owned the model, update routing, modal rendering,
create-field pickers, rich text formatting, table fitting, editor configuration, footer hints, and
detail scrolling. The follow-up package-boundary split moved those workflows into same-package
files; the consistency rule still applies to future additions.

The working agreement already says standard TUI primitives should use Bubble Tea, Bubbles, Lip
Gloss, or maintained packages built on them when they fit. This audit makes that rule operational
before we add more comment composition, workflow actions, or command palette behavior.

## Goals

- Identify custom TUI primitives that should be replaced, wrapped, or left custom.
- Prefer maintained dependencies already in the project: Bubbles `viewport`, `textarea`,
  `textinput`, `table`, `list`, `help`, `key`, `paginator`, and Lip Gloss layout/table helpers.
- Choose one low-risk first implementation slice that reduces hand-rolled code without changing
  Jira behavior.
- Preserve the worker-backed Jira flow and metadata-driven create/edit behavior.
- Keep the audit usable as a repeatable decision record for future TUI work.

## Non-Goals

- Do not rewrite the whole TUI model in one pass.
- Do not replace custom Jira-specific behavior just because it is custom.
- Do not add new workflow features during the audit slice.
- Do not introduce a new UI framework outside the Bubble Tea ecosystem.

## Candidate Areas

### Footer And Help Rendering

`internal/tui/keymap.go` has useful structured binding metadata, but footer/help rendering is still
custom. Bubbles `key` and `help` are intended for generated key help and graceful truncation. This is
the best first app-wide slice because it can improve consistency across contexts without changing
Jira data flow.

Decision: make this the recommended first app-wide implementation candidate, behind a local adapter
that keeps the current key context metadata.

### Config Scalar Input

`internal/configui/model.go` originally used a manual single-line edit buffer for scalar text
fields. That duplicated cursor/input behavior from Bubbles `textinput` and was isolated from Jira
workflows, so it became the proof-of-pattern migration.

Decision: scalar fields now use Bubbles `textinput`. Keep custom boolean and color pickers for now.

### Create Field Pickers

Current behavior is app-specific code in `renderCreateDynamicField`, `updateCreateIssue`, and helper
functions for selection, filtering, windowing, and summary rendering. The behavior is useful, but it
duplicates pieces that Bubbles `list` already provides: selection, filtering, pagination/status, and
help. It remains important because the scope is bounded to Jira metadata option fields and recent
user friction came from long component lists, but it has more edge cases than key/help or config
text input.

Decision: keep as a near-term target, but do not start here unless we intentionally scope the work
to one dynamic picker after smaller migrations prove the pattern.

### Table And Rich Text Rendering

The rich description path includes custom Markdown-ish normalization, inline code styling, code
block styling, and fitted table rendering. Some of this remains intentionally custom because Jira
ADF conversion and terminal fitting have app-specific requirements. Lip Gloss table helpers may
replace parts of table drawing, but we should not remove local fixture coverage or the existing ADF
boundary.

Decision: keep the current ADF boundary and treat table drawing as a later proof-of-fit item against
the existing golden fixtures.

### Modal And Editor Configuration

The app already uses Bubbles `textarea` for text editors and Bubbles `viewport` for detail scrolling.
The custom part is repeated editor configuration and dialog layout. This is a wrapper/cleanup
candidate: a small internal dialog/editor factory could reduce duplication while still using the
same maintained primitives.

Decision: catalog duplication during the audit, but avoid starting here unless the picker adapter
proves too risky.

### Config UI Pickers

`internal/configui/model.go` has custom boolean/color picker rendering. These are simpler than the
main Jira TUI surfaces. They are worth recording, but they are not the current risk center.

Decision: defer until after main TUI picker and help surfaces are addressed.

## Approaches Considered

### Recommended: Audit Matrix Then One Replacement

Create a short audit matrix that lists custom component surfaces, current code locations, candidate
maintained replacements, risk, and recommendation. Then implement one low-risk replacement or
adapter, most likely footer/help rendering or config scalar input.

This keeps the diff small, creates durable guidance, and proves the replacement strategy with tests.

### Alternative: Documentation-Only Audit

Only write the matrix and defer implementation. This is safest, but it does not prove whether a
Bubbles replacement can preserve the existing UX.

### Alternative: Broad Component Refactor

Extract multiple TUI components at once. This could reduce `model.go` size faster, but it is too
risky because detail rendering, create flows, comment composition, and AI modals all share state and
tests.

## Design

Add `docs/tui-component-audit.md` as the living audit document. It will use one row per custom
surface with:

- Surface name.
- Current files/functions.
- User-facing risk or pain.
- Maintained component candidate.
- Recommendation: keep custom, wrap, replace, or investigate.
- Priority.
- Notes explaining why.

The first implementation slice should be one of:

- Footer/help adapter using Bubbles `key` and `help`, preserving existing key contexts and footer
  labels.
- Config scalar input using Bubbles `textinput`, preserving existing config field validation and
  custom bool/color controls.

Create metadata pickers remain the next target after the first migration proves the adapter pattern.
If Bubbles `list` does not fit create pickers cleanly, that later slice should standardize the
current picker behind a focused internal helper and record why `list` is not the right primitive yet.

## Testing

- Add tests before changing behavior.
- Preserve existing behavior tests for whichever surface is chosen first.
- For footer/help, assert context-specific footer labels and help output still include expected keys.
- For config input, assert text editing supports insertion, backspace, cursor movement, save, and
  existing validation.
- For later create picker work, preserve filtering, movement, optional unselected fields, AI
  component matching, and bounded rendering.
- Run focused TUI tests first, then `go test ./... -count=1`, then `make check`.

## Open Decision

The audit found two clearer lower-risk wins than create-field pickers: footer/help rendering and
config scalar input. Pick one of those for the first implementation slice, then return to create
metadata pickers with the migration pattern established.
