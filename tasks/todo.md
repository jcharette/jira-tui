# Task Plan

## Board Hygiene Audit and Fix - 2026-07-08

- [x] Add shared board hygiene checks for Epic-owned Sub-tasks, unassigned work, and sprint visibility.
- [x] Block Create Subtask from Epics and show TUI board hygiene warnings.
- [x] Add `jira ticket check-board [KEY]` and no-key current-user audit mode.
- [x] Add prompted `--fix` / scripted `--yes` behavior for safe fixes.
- [x] Document and verify the flow.

### Implementation Notes

- Reuse existing Jira search/detail, child expansion, sprint move, assignee update, and edit metadata paths.
- Attempt metadata-backed issue-type conversion before falling back to replacement/manual guidance.
- Keep default fixes interactive: print the plan, prompt once, default no.

### Review

- Added shared board hygiene checks plus TUI warnings and an Epic guard for Create Subtask.
- Added `jira ticket check-board [KEY]`, no-key current-user audit mode, prompted `--fix`, and `--yes`.
- Safe fixes assign to current user, add active sprint when `queries.default_board_id` has an active
  sprint, and attempt Story/Task conversion for Epic-owned Sub-tasks before reporting manual follow-up.

## Developer Workbench UX - 2026-07-06

- [x] Write the approved UX direction as a scoped design spec.
- [x] Review and approve the written spec.
- [x] Write the implementation plan for Pass 1: Developer Flow Polish.
- [x] Implement Pass 1 in small verified slices.
- [x] Reframe Developer Workbench as a ticket dashboard using existing Jira/Git/Claude context.
- [x] Reassess Pass 2 and Pass 3 sequencing after Pass 1 lands.

### Implementation Notes

- Use JiraTUI as a UX benchmark, not a source to copy.
- Keep Jira/Git/GitHub/Claude safe-write gates unchanged.
- Prefer existing detail sections, Ticket Actions, Claude actions, comments, worklog, and workflow
  code before adding new surfaces.

### Review

- Pass 1 added Developer Workbench and developer-first Ticket Actions.
- Follow-up cockpit slice kept the existing detail sections but made Workbench summarize developer
  actions plus Comments, Worklog, Hierarchy, and Links state.
- Ticket Dashboard slice reframed the Workbench detail section around ownership, latest comment
  signal, and next-action rows.
- Initial ticket-list follow-up added a selected-ticket orientation strip under the view controls.
- Search/filter clarity follow-up renamed the local issue-list filter chip to `Status All/Active`.
- Next useful slice is query modal clarity or deeper related-work detail polish.

## Release v1.0.11 - 2026-07-06

- [x] Move Unreleased changelog entries to v1.0.11.
- [x] Update app version and install docs for v1.0.11.
- [x] Verify docs, tests, project check, and install.
- [x] Switch GitHub auth to personal account, push release prep, dispatch and verify Release workflow.
- [x] Update Homebrew formula from published checksums and verify CI.
- [x] Switch GitHub auth back to joncha_floqast.

### Review

- Released v1.0.11 from `main` via GitHub Actions run `28792633219`.
- Updated `Formula/jira-tui.rb` from the published v1.0.11 `checksums.txt`.
- Fixed CI-only UX snapshot drift by making the snapshot harness own timezone and symbol mode.
- Verified local release gates, release publication, and formula follow-up CI run `28792948405`.
- Switched GitHub auth back to `joncha_floqast` after release operations.

## UX Snapshot Harness - 2026-07-06

- [x] Add failing UX snapshot coverage for representative TUI states.
- [x] Add an explicit golden update path using the existing test helper.
- [x] Generate stable fixtures for core screens and modals.
- [x] Document the UX snapshot workflow.
- [x] Verify focused snapshots, full tests, docs check, project check, and install.

### Implementation Notes

- Reuse `internal/tui` render tests and `testdata` golden files.
- Keep this as plain Go tests; no new snapshot dependency.
- Cover drift-prone text surfaces first: footers, modal titles, action labels, notices, and write gates.

### Review

- Added `TestUXSnapshots` with representative issue list, detail, action palette, comment, create,
  worklog, Claude, Diagnostics, Notifications, and bug-report states.
- Reused the existing golden snapshot helper and added `UPDATE_GOLDEN=1` refresh support.
- Normalized version strings in snapshots to avoid release-version churn.
- Verification: `go test ./internal/tui -run TestUXSnapshots -count=1`, `go test ./... -count=1`,
  `make docs-check`, `make check`, and `make install-user` passed.

## Tooling UX Review - 2026-07-06

- [x] Review CLI command/help UX and local workflow tooling.
- [x] Review TUI navigation, footer/help, modal, and keyboard consistency.
- [x] Review Claude-assisted workflow UX and confirmation boundaries.
- [x] Review docs/backlog/changelog consistency against shipped tooling.
- [x] Run verification commands and record findings.

### Review Notes

- Read-only review first; do not refactor until findings are concrete.
- Prioritize bugs, broken flows, stale help/docs, and high-friction UX.
- Keep fixes, if any, as separate bounded follow-up slices.

### Findings And Fixes

- Fixed Start Work so a required branch failure skips later Jira assignment, transition, and comment writes.
- Fixed Ticket Assist comment posting so `allow_jira_writes = false` blocks the Jira comment path.
- Wired the existing `draft_comment` feature flag into TUI Draft Comment availability.
- Added the same external-write prompt boundary to `jira commit` Claude note drafting used by other Claude prompts.
- Rejected unexpected positional args for `jira ticket toil` and `jira ticket create-toil`.
- Updated keyboard, command, project-state, and changelog docs for current tooling.
- Updated GitHub issue #6 through the GitHub connector after `gh issue edit` was blocked by EMU auth.
- Verification: focused regression tests passed, `go test ./... -count=1` passed,
  `make docs-check` passed, `make check` passed, and `make install-user` passed.

## Claude Comment Draft Refinement - 2026-07-06

- [x] Add focused failing tests for comment composer `ctrl+r` Claude refinement.
- [x] Implement local comment draft refinement in Add/Edit Comment without changing post/update paths.
- [x] Update footer/help/docs/changelog.
- [x] Verify focused tests, full tests, docs check, project check, and install.

### Implementation Notes

- Reuse the existing comment composer and `submitAIRequest` path.
- Keep Claude behind the existing `ticket_assist` feature flag.
- Keep Jira writes behind the existing comment review/post confirmation.

### Review

- Added `ctrl+r` Claude refinement for local Add/Edit Comment drafts.
- Kept Jira comment post/update writes behind the existing review confirmation.
- Updated keyboard help, workflows, project state, and Unreleased changelog.
- Verification: focused comment composer tests passed, `go test ./... -count=1` passed,
  `make docs-check` passed, `make check` passed, and `make install-user` passed.

## Claude Tooling Expansion - 2026-07-06

- [x] Add Claude drafting to `jira finish` PR title/body and final Jira note.
- [x] Add ticket-detail Claude Quality Review and Draft Comment actions.
- [x] Add read-only Claude start-work plans before branch/Jira writes.
- [x] Add create-ticket draft refinement.
- [x] Add Claude cleanup for bug report title/body.
- [x] Verify focused tests, docs checks, full local checks, and install.

### Implementation Notes

- Keep Claude as the only execution provider in this batch.
- Keep provider-neutral execution, Codex support, workspace mapping, and code edits deferred.
- Keep every generated result behind an existing review/edit/confirmation surface.

### Review

- Added optional read-only Claude Branch Plans to CLI and TUI Start Work review.
- Kept branch and Jira writes unchanged behind the existing Start Work confirmation.
- Verified focused start-work tests.
- Added `ctrl+r` Claude refinement for local Create Ticket Summary/Description drafts.
- Kept Jira creation unchanged behind the existing `ctrl+s` submit.
- Verified focused create-ticket refinement tests.
- Added `ctrl+r` Claude polish for local bug report title/body drafts.
- Kept GitHub issue opening unchanged behind the existing `ctrl+s` submit.
- Verified focused bug-report polish tests.
- Final verification passed: focused tests for each slice, `go test ./... -count=1`, `make docs-check`,
  `make check`, and `make install-user`.

## Claude AI Workflow Cleanup - 2026-07-06

- [x] Sync the local backlog and roadmap with the decision to backburner provider-neutral AI.
- [x] Add optional Claude-assisted Jira note drafting to `jira commit`.
- [x] Verify focused tests, docs checks, and full local checks.

### Implementation Notes

- Keep Claude as the only execution provider in this slice.
- Keep `ai.task.*` event names for Diagnostics, but do not add Codex execution.
- Keep generated Jira notes reviewable before posting.

### Review

- Reframed the active AI backlog around Claude workflow cleanup and removed stale closed Ticket
  Actions issue #12 from the active local index.
- Added optional Claude-assisted Jira progress note drafting to `jira commit` behind the existing
  Claude Branch Plan feature flag.
- Kept provider-neutral execution, Codex support, and workspace mapping deferred.
- Verified focused app tests, `go test ./... -count=1`, `make docs-check`, and `make check`.
- GitHub issue #6 update is blocked by account policy: `Unauthorized: As an Enterprise Managed User,
  you cannot access this content`.

## Release v1.0.10 - 2026-06-30

- [x] Update version references and release notes for the notification-center fix.
- [x] Run release-gate local verification.
- [x] Commit and push release prep to `main`.
- [x] Dispatch and verify the GitHub Release workflow for v1.0.10.
- [x] Update the Homebrew formula from published v1.0.10 checksums and verify follow-up CI.

### Review

- Released v1.0.10 from `main` via GitHub Actions run `28437020463`.
- Updated `Formula/jira-tui.rb` from the published v1.0.10 `checksums.txt`.
- Verified local release gates, release publication, and formula follow-up CI run `28437306320`.

## Notification Center Enter Open Bug - 2026-06-30

- [x] Reproduce the notification-center Enter key bug with a focused regression test.
- [x] Fix the notification panel key handling so the advertised footer action opens the selected ticket.
- [x] Verify the focused notification tests, broader TUI package tests, docs check, and full local check.

### Review

- Added regression coverage for opening a notification whose ticket is not in the current issue list.
- Notification `enter` now selects or appends the notified ticket, closes the notification panel,
  and reuses the existing detail-loading worker path.
- Updated notification shortcut docs to include `enter open`.
- Verified with focused notification tests, `go test ./internal/tui -count=1`, `make docs-check`,
  `go test ./... -count=1`, and `make check`.

## Toil Ticket CLI And Quick Picker - 2026-06-29

- [x] Add a shared toil-ticket helper in the app layer for create, update, close, and picker flows.
- [x] Add `jira ticket create-toil`, `jira ticket update-toil`, `jira ticket close-toil`, and `jira ticket toil`.
- [x] Make `create-toil` label new tickets with `toil`, prefer Jira issue type `Toil` when available, and otherwise use the first safe non-subtask issue type.
- [x] Make `update-toil` and `close-toil` accept an issue key or open a quick picker of open assigned toil tickets using `labels = toil OR issuetype = Toil`.
- [x] Make `close-toil` log any supplied time first, then apply the best safe terminal transition with no required fields; if no safe transition exists, leave the ticket open and report why.
- [x] Add a TUI "Create Toil Ticket" fast path that mirrors the same summary/time/note/close-after-create workflow without bypassing Jira metadata.
- [x] Update footer/help and durable docs for the new commands and shortcuts.
- [x] Verify the CLI and TUI slices with focused app/TUI tests, `go test ./... -count=1`, `make docs-check`, and `make check`.

### Implementation Notes

- Keep Jira writes on existing client/worker patterns. Do not introduce a second Jira API path.
- Reuse existing create metadata, worklog validation, worklog payloads, and terminal-transition scoring where possible.
- CLI commands live under the existing Cobra root as `jira ticket ...`; do not move existing `start`, `commit`, or `finish` commands.
- The portable toil marker is label `toil`; Jira issue type `Toil` is opportunistic, not required.
- No standalone binaries named `create-toil`, `update-toil`, or `close-toil` in this slice.

### Review

- Implemented the `jira ticket ...` CLI namespace for toil creation, picker-backed update, and
  picker-backed close.
- Added the TUI `T` Create Toil Ticket modal with Summary, Duration, Note, and Close after create.
- Verified with focused app/TUI tests, `go test ./... -count=1`, `make docs-check`, and
  `make check`.

## Epic Subtask Recommendation Review - 2026-06-22

- [x] Parse Ticket Assist `Subtask Recommendations` into reviewable actions.
- [x] Add an epic-management review modal for keep/add/modify/close recommendations.
- [x] For remove/defer actions, safely close the child only when Jira exposes a matching no-extra-fields transition; otherwise add a review comment to the child.
- [x] For add actions, open the existing child-ticket create flow prefilled from the recommendation.
- [x] Update footer/help/docs/changelog for the new review flow.
- [x] Verify focused tests, full checks, docs checks, and installed binary.

### Review

- Added a parsed Review Subtask Changes flow after whole-ticket Ticket Assist apply.
- Add recommendations open the metadata-backed child-ticket create form prefilled from the recommendation.
- Modify/Rescope recommendations post a child-ticket review comment.
- Remove/Defer recommendations attempt a close-as-invalid style transition only when Jira exposes a safe no-extra-fields transition; otherwise they post a child-ticket review comment.
- Updated the modal footer, keyboard docs, workflow docs, project state, and changelog.
- Verified with focused Ticket Assist tests, `go test ./internal/tui -count=1`, `go test ./... -count=1`, `make docs-check`, `make check`, `make install-user`, and binary string checks.

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
- [x] Push release prep to GitHub.
- [x] Create GitHub release v1.0.3.
- [x] Update Homebrew formula checksums from published assets.
- [x] Capture the next metadata-backed Ticket Actions backlog slice.

## Metadata-Backed Ticket Actions - 2026-06-20

- [x] Add RED tests for Fix Version, Affects Version, and Due Date actions.
- [x] Allow supported standard Jira edit fields through the generic edit writer.
- [x] Surface the three actions only when Jira edit metadata exposes them as editable.
- [x] Reuse option pickers for version fields and text input for due date.
- [x] Update docs for the new Ticket Actions.
- [x] Verify focused tests and full project checks.

## Release v1.0.4 - 2026-06-20

- [x] Audit docs for metadata-backed Ticket Actions before shipping.
- [x] Move release notes from Unreleased to v1.0.4.
- [x] Run docs and full project checks.
- [x] Push release prep to GitHub.
- [x] Create GitHub release v1.0.4.
- [x] Update Homebrew formula checksums from published assets.
- [x] Update GitHub issue #12 with shipped first-slice scope.

## Metadata-Backed Ticket Actions Completion - 2026-06-21

- [x] Add RED client tests for parent and time tracking payloads.
- [x] Add RED worker tests for parent and time tracking writes.
- [x] Add RED TUI tests for metadata-gated Parent and Estimates actions.
- [x] Implement Jira parent and time tracking writes.
- [x] Implement worker request/result routing.
- [x] Implement TUI dialogs for Set Parent and Edit Estimates.
- [x] Update docs and issue #12 completion notes.
- [x] Verify focused tests and full project checks.

## Release v1.0.5 - 2026-06-21

- [x] Audit docs for Parent and Estimates Ticket Actions before shipping.
- [x] Move release notes from Unreleased to v1.0.5.
- [x] Run docs and full project checks.
- [ ] Push release prep to GitHub.
- [x] Create GitHub release v1.0.5.
- [x] Update Homebrew formula checksums from published assets.
- [x] Close GitHub issue #12 after release.

## Review Fixes - Parent Cache and Time Tracking - 2026-06-22

- [x] Add failing regression tests for parent cache patching and time tracking read/prefill behavior.
- [x] Patch parent updates through retained detail and active view cache.
- [x] Add Jira detail estimate fields and parse current time tracking values.
- [x] Prefill estimates editor and patch refreshed detail state after successful updates.
- [x] Run focused tests and full verification.
- [x] Document review results.

Review results:
- Parent updates now patch retained issue detail cache and active view cache, matching existing summary/priority/status cache behavior.
- Ticket detail now reads Jira time tracking estimates through the raw REST detail response, because the upstream typed issue model omits `fields.timetracking`.
- The estimate editor now prefills current values and patches cached detail state after a successful update.
- Verification: focused Jira/TUI tests, `go test ./... -count=1`, `GOCACHE=/tmp/jira-tui-gocache go vet ./...`, and `make check` passed.

## Release v1.0.6 - 2026-06-22

- [x] Audit docs for parent cache and time tracking detail fixes before shipping.
- [x] Move release notes from Unreleased to v1.0.6.
- [x] Run docs and full project checks.
- [x] Push release prep to GitHub.
- [x] Create GitHub release v1.0.6.
- [x] Update Homebrew formula checksums from published assets.
- [x] Verify final main CI.

## Ticket Assist Text Entry Shortcut Safety - 2026-06-22

- [x] Add a failing regression test proving printable letters typed in the Ticket Assist local draft editor are inserted as text and do not open modal actions.
- [x] Replace bare Ticket Assist draft actions with explicit modifier shortcuts while keeping save/copy/escape behavior intact.
- [x] Update user-facing AI Ticket Assist docs/changelog and rendered footers for the changed shortcuts.
- [x] Verify focused Claude Assist tests and the broader project check.

### Review

- Root cause: `updateClaudeAssistEditor` handled bare `r` and `c` before passing keys to the focused textarea, so normal typing could open refine/comment actions.
- Ticket Assist draft actions now use `ctrl+r refine` and `ctrl+c comment`; printable letters stay in the editor.
- Updated Ticket Assist modal footers, available-action hints, README key copy, `docs/keyboard.md`, `docs/project-state.md`, and the Unreleased changelog.
- Verification: RED test failed on `r` opening refine, focused Claude Assist tests passed, `go test ./internal/tui -count=1` passed, `go test ./... -count=1` passed, `make check` passed, and `make docs-check` passed after the final task-review edit.

## Release v1.0.7 - 2026-06-22

- [x] Confirm `v1.0.6` is the latest GitHub release and `v1.0.7` is unused.
- [x] Move Unreleased changelog entry to `1.0.7`.
- [x] Update app version, install docs, and Homebrew formula version references for `1.0.7`.
- [x] Run full release verification.
- [x] Commit release changes and push `main`.
- [x] Tag and create GitHub release `v1.0.7`.
- [x] Update Homebrew formula checksums from published assets.
- [x] Verify final release state.

### Review

- Published `v1.0.7` for the Ticket Assist text-entry shortcut fix.
- Release commit: `e64e86c`.
- Release URL: https://github.com/jcharette/jira-tui/releases/tag/v1.0.7
- Assets uploaded: macOS arm64/amd64 tarballs, Linux arm64/amd64 tarballs, Windows amd64 zip, and `checksums.txt`.
- Formula checksums match the published GitHub asset digests.
- Verification: `make check`, `make install-user`, `make docs-check`, GitHub release metadata, and `gh release list --limit 5`.

## Guided Ticket Assist Session - 2026-06-22

- [x] Add RED tests for the whole-ticket prompt including loaded children/subtasks and explicit subtask recommendation output.
- [x] Add RED tests for parsing Ticket Assist Open Questions into local answer state after Claude results.
- [x] Add RED tests for answering Open Questions in the Ticket Assist modal and refining with the current draft plus answers.
- [x] Implement minimal Ticket Assist question state, rendering, key handling, and prompt feedback.
- [x] Update Ticket Assist copy/docs for guided whole-ticket drafting, Open Questions, and Subtask Recommendations.
- [x] Verify focused Claude Assist tests, full TUI tests, full project checks, and docs checks.

### Proposed Design

- `Ticket Assist` becomes a guided whole-ticket drafting session, entered either from the Claude section or contextual Description AI.
- Claude output should include one editable draft plus `Open Questions` and `Subtask Recommendations`.
- Open Questions are parsed into local answer state. Users answer them in the same modal, then run `ctrl+r refine with answers`.
- Prompt context includes loaded hierarchy rows so Claude can recommend keeping, adding, removing, or rescoping subtasks without directly writing Jira changes.
- Jira writes remain explicit and limited to the existing apply/comment paths; subtask recommendations are local text only in this slice.

### Review

- Contextual `a` action now opens `Ticket Assist`; the first action starts the same whole-ticket guided session as the Claude section.
- Ticket Assist prompts include loaded children/subtasks and require `Subtask Recommendations` output without making Jira writes.
- Parsed `Open Questions` render in the modal, can be answered locally, and `ctrl+r` refines with saved answers while printable answer text stays in the editor.
- Verified with focused Ticket Assist tests, `go test ./internal/tui -count=1`, `go test ./... -count=1`, `make check`, and `make docs-check`.

## Detail Description Expansion and Simple Text Selection - 2026-06-22

- [x] Add RED detail tests proving Overview renders the full description instead of a fixed three-line preview.
- [x] Update Overview description rendering to expand by default and rely on the existing detail viewport for overflow.
- [x] Add RED editor tests for simple mark/select/copy/delete behavior in Ticket Assist text boxes.
- [x] Implement a minimal selection layer using `shift+arrow` when available, plus `ctrl+space` mark, normal arrows to extend, `ctrl+y` copy, and `delete`/`backspace` delete.
- [x] Update keyboard/docs/footer copy for expanded descriptions and simple selection keys.
- [x] Verify focused tests, full TUI tests, full project checks, and docs checks.

### Review

- Root cause: Overview intentionally rendered only a three-line Description preview and Description was not a primary detail tab, so long epics could not expose their full body from the default view.
- Overview now renders the full Description by default; long bodies use the existing detail viewport and scroll indicator.
- Ticket Assist draft and Open Question answer editors now support simple local selection with `shift+arrow` when the terminal sends it, or `ctrl+space` plus arrows everywhere.
- Selection actions: `ctrl+y` copies selected text, `delete`/`backspace` removes it, paste replaces it, and `esc` clears it before closing/canceling the editor.
- Verification: focused detail/selection tests, `go test ./internal/tui -count=1`, `go test ./... -count=1`, `make check`, and `make docs-check`.

## Ticket Assist Apply Writes and Subtask Recommendations - 2026-06-22

- [x] Add RED tests proving Ticket Assist apply with Open Questions writes Summary and Description after confirmation.
- [x] Add RED tests proving Subtask Recommendations are carried into Jira instead of being local-only.
- [x] Make apply confirmation/result copy explicit about Summary, Description, and Subtask Recommendations.
- [x] Post parsed Subtask Recommendations as a Jira comment during Ticket Assist apply.
- [x] Update docs/changelog for the non-destructive subtask recommendation apply behavior.
- [x] Verify focused Ticket Assist tests, full TUI tests, full project checks, docs checks, and reinstall the local binary.

### Review

- Confirmed local config has `allow_jira_writes = true` and `require_confirmation = true`; the write gate was not the blocker.
- Ticket Assist apply already updated Summary and Description after confirmation, but Subtask Recommendations were never enqueued and remained local-only.
- Whole-ticket apply now posts parsed Subtask Recommendations as a Jira comment after the Summary and Description worker updates.
- Apply progress/confirmation now shows Subtask Recommendations as part of the Jira apply operation when present.
- Destructive child-ticket removal/rebuild remains intentionally manual; the app now persists the recommendation plan in Jira instead of losing it when the modal closes.
- Verification: focused Ticket Assist tests, `go test ./internal/tui -count=1`, `go test ./... -count=1`, `make check`, `make docs-check`, and `make install-user`.
