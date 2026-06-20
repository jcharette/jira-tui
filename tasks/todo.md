# Task Plan

## 1.0.0 Initial Release Baseline

- [x] Consolidate the app into a clean initial release baseline.
- [x] Keep README, project docs, security overview, backlog, and changelog aligned with the current
  product state.
- [x] Publish `v1.0.0` as the first visible release after history reset.

### 1.0.0 Scope

This release is the clean baseline for the full Jira TUI app: Jira browsing, ticket detail,
metadata-backed Jira writes, Git/GitHub developer workflows, local Claude assistance, notifications,
cache/Diagnostics infrastructure, and the current security model.

### Backlog

- GitHub issue #6 remains the active public backlog item for provider-neutral AI workflow expansion.

## Show App Version In Header

- [x] Add a shared app version source.
- [x] Show the version in the main app header.
- [x] Show the version in the config header.
- [x] Verify focused header tests.
- [x] Prepare the `v1.0.1` release notes and install docs.

### Review

- Added `internal/version` as the single source for the visible app version.
- Main TUI header now shows the version with the right-side runtime metadata.
- Config UI header now shows the same version next to its status.
- Updated the config UI golden snapshot for the new versioned header.
- Verified with `go test ./internal/tui ./internal/configui -run 'TestHeaderUsesAvailableWidth' -count=1`.
- Verified full suite with `go test ./... -count=1`, `make check`, and `make install-user`.

## Built-In Color Skins

- [x] Add named built-in skins with matching color palettes and default symbol modes.
- [x] Support `[appearance] theme = "name"` in config load/save/validation while preserving explicit color overrides.
- [x] Let explicit `[display] symbol_mode = "..."` override the skin's default icon set.
- [x] Add a Theme picker to Jira Config.
- [x] Document the available built-in skins.
- [x] Add focused config/config UI tests.
- [x] Verify focused tests and full local check.

### Review

- Added built-in skins: `default`, `focus`, `ops`, and `high-contrast`.
- Each skin includes a color palette and default issue-list symbol mode.
- Existing explicit color fields and `[display] symbol_mode` still override the selected skin.
- Jira Config now has a Theme picker that updates color fields and Symbol Mode together.
- Focused config/config UI tests pass.
- Full `make check` passes.

## Skin Coverage Follow-Up

- [x] Move remaining fixed dark block backgrounds to skin-derived backgrounds.
- [x] Make shared text/status styles carry the skin surface background.
- [x] Add focused theme tests for skin-derived backgrounds.
- [x] Add skin-owned issue icon sets with explicit emoji/nerd/plain override behavior.
- [x] Verify focused theme tests and full local check.

### Review

- Shared theme text/status styles now carry the selected skin surface background.
- Code, notice, comment, and input blocks now derive backgrounds from the selected skin instead of fixed dark colors.
- Built-in skins now include issue icon sets, not only a symbol-mode default.
- Focused tests pass for theme backgrounds and skin icon behavior.
- Full `make check` passes.

## Skin Inline Background Cleanup

- [x] Remove background painting from inline text styles.
- [x] Keep skin-derived backgrounds on block/container styles.
- [x] Update theme tests to catch rectangular inline background artifacts.
- [x] Verify focused UI tests and full local check.

### Review

- Inline styles are foreground-only again to avoid rectangular artifacts in tables and footers.
- Block styles still use skin-derived backgrounds.
- Verified with `go test ./internal/ui -count=1` and `make check`.

## Theme Quality Pass

- [x] Add theme-specific status styles for To Do, In Progress, Review, Done, and Blocked.
- [x] Add theme-specific priority styles for high/medium/low priority display.
- [x] Add an Appearance preview in Jira Config.
- [x] Add focused tests for themed status/priority styles and preview rendering.
- [x] Verify focused tests and full local check.

### Review

- Status rendering now uses dedicated theme slots instead of reusing generic warning/success/error.
- Priority rendering now uses dedicated high/medium/low theme slots.
- Jira Config Appearance now shows a small two-row preview sample.
- Focused theme quality tests pass.
- Full `make check` passes.

## Theme Gallery UX Redesign

- [x] Replace the inline Theme option picker with a vertical Theme Gallery.
- [x] Reduce primary themes to curated, job-oriented choices: `Default`, `Focus`, `Ops`, and `High Contrast`.
- [x] Show each theme with a short use-case description.
- [x] Make preview rows the primary Appearance screen content.
- [x] Move raw color fields behind an `Advanced Colors` entry.
- [x] Keep theme-owned icon packs and explicit symbol-mode overrides.
- [x] Add tests for theme gallery rendering, navigation, selection, and config output.
- [x] Verify focused config UI tests and full local check.

### Proposed UX

- `Appearance` opens a gallery list instead of raw color fields.
- `j/k` moves through theme cards.
- `enter` selects the focused theme and updates the preview immediately.
- `Advanced Colors` remains available for manual overrides, but is not the default first impression.
- Preview includes selected row, hierarchy, status, priority, footer sample, and notification sample.

### Review

- Appearance now renders a vertical Theme Gallery instead of a wrapped inline option list.
- Primary themes are `default`, `focus`, `ops`, and `high-contrast`.
- Raw color overrides now live in `Advanced Colors`.
- The gallery preview includes issue rows, hierarchy, notification, and footer samples.
- Focused config/config UI/TUI/theme tests pass.
- Full `make check` passes.

## Markdown Escape Rendering Fix

- [x] Add regression coverage for escaped Markdown punctuation in rich descriptions.
- [x] Add regression coverage for escaped Markdown punctuation in compact single-line previews.
- [x] Unescape Markdown punctuation in terminal text rendering while preserving rich tokens.
- [x] Verify focused tests and full local check.

### Review

- Escaped punctuation such as `\(` and `\)` no longer leaks into rendered description text.
- Compact previews using `singleLine` normalize the same punctuation escapes.
- Focused rich-text tests pass.
- Full `make check` passes.

## User Docs And Key Help Cleanup

- [x] Add `docs/quickstart.md` for first-run onboarding.
- [x] Add `docs/workflows.md` for task-oriented recipes.
- [x] Add `docs/keyboard.md` for screen-grouped key reference.
- [x] Link new docs from README and docs index.
- [x] Include new docs in `make docs-check`.
- [x] Rename the `x` footer hint from `expand-open` to `children`.
- [x] Verify docs checks and full local check.

### Review

- Added a short onboarding doc, day-to-day workflow recipes, and a keyboard reference.
- README now points new users to quickstart/workflows/keyboard docs.
- The docs index now makes user-facing docs easier to find before internal project-state docs.
- The issue-list footer now uses `x children` for the child-loading action.
- Verified with `make docs-check`, focused TUI help/footer tests, and `make check`.

## Release v1.0.2 - 2026-06-20

- [x] Audit docs for the theme, keyboard, and rich-text UX changes before shipping.
- [x] Move release notes from Unreleased to v1.0.2.
- [x] Run docs and full project checks.
- [x] Push release prep to GitHub.
- [x] Create GitHub release v1.0.2.
- [x] Update Homebrew formula checksums from published assets.


## Sprint Actions - 2026-06-20

- [x] Add Jira Agile API support for discovering boards/sprints and moving issues into a sprint.
- [x] Add config support for a default Agile board used by sprint actions.
- [x] Add Ticket Actions flow for Sprint -> Add to active sprint / Choose sprint / Configure board.
- [x] Keep sprint writes worker-backed and report clear success/failure status.
- [x] Update docs/backlog and user docs for sprint actions before shipping.
- [x] Verify with focused tests, make check, and make install-user.


### Sprint Actions Review

- Implemented: `queries.default_board_id`, config UI field, Jira Agile sprint move API, worker request/result, Ticket Actions Sprint picker, Diagnostics classification, and user docs.
- Verified: focused package tests, `make docs-check`, `make check`, and `make install-user` passed on 2026-06-20.


## Release v1.0.3 - 2026-06-20

- [x] Audit docs for Sprint Actions before shipping.
- [x] Move release notes from Unreleased to v1.0.3.
- [x] Run docs and full project checks.
- [ ] Push release prep to GitHub.
- [ ] Create GitHub release v1.0.3.
- [ ] Update Homebrew formula checksums from published assets.
- [ ] Capture the next metadata-backed Ticket Actions backlog slice.

