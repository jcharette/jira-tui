# Task Plan

## Inline Description AI

- [x] Add TUI tests that Description focus exposes `a AI` only when Claude Ticket Assist is available.
- [x] Add TUI tests that `a` on Description opens the `AI for Description` picker and can cancel without running Claude.
- [x] Add TUI tests that inline Description AI actions submit scoped Claude prompts with ticket, Description, comments, and user instruction context.
- [x] Add TUI tests that inline Description AI results reuse the Ticket Assist modal zones.
- [x] Add TUI tests that inline Description AI apply updates Description only and never Summary.
- [x] Add TUI tests that inline Description AI drafts can still post as Jira comments through confirmation.
- [x] Implement inline Description AI state, picker, instruction editor, and prompt builder.
- [x] Implement Description-only Ticket Assist apply target while preserving normal Ticket Assist Summary+Description apply.
- [x] Update docs, changelog, and review notes.
- [x] Run focused tests, `go test ./internal/tui`, `make check`, and `make install-user`.

### Inline Description AI Scope

Implement the approved A+ hybrid pattern from
`docs/superpowers/specs/2026-06-15-inline-description-ai-design.md`. Normal ticket detail stays
simple; when Description is focused and Claude Ticket Assist is available, `a` opens a contextual
`AI for Description` picker. The picker supports improve clarity, extract acceptance criteria, ask
Claude a question, and draft clarifying comment. Results reuse the existing Ticket Assist modal and
draft/refine/copy/comment mechanics, but inline Description apply must update Description only.

### Inline Description AI Design

Follow `docs/superpowers/plans/2026-06-15-inline-description-ai-implementation.md`. Keep Claude IO
on the existing background runner and diagnostics path. Add only small scoped TUI state for the
inline picker/instruction flow and a draft target that distinguishes normal Ticket Assist from
Description-scoped output. If Description detail is not loaded, do not call Claude; show
`Description is not loaded yet.`

### Inline Description AI Review

- Added Description-scoped inline AI entry with `a` from the Description section.
- Added an `AI for Description` picker with improve clarity, extract acceptance criteria, ask
  Claude a question, and draft clarifying comment actions.
- Reused the existing Claude runner, Ticket Assist modal, refine, copy, comment, progress, and
  Diagnostics flows.
- Added a Description-only Ticket Assist draft target so inline AI apply updates Description and
  cannot change Summary.
- Preserved normal Ticket Assist Summary plus Description apply behavior behind the existing target.
- Final verification passed with focused inline AI/Ticket Assist tests, `go test ./internal/tui`,
  `make check`, and `make install-user`.

## Ticket Assist Output Clarity And Comment Path

- [x] Add TUI tests that Ticket Assist output renders distinct Claude Review, Local Draft, and action zones.
- [x] Add TUI tests that the action zone explains apply vs comment vs refine vs copy.
- [x] Add TUI tests that a Ticket Assist draft can be posted as a Jira comment without editing fields.
- [x] Implement the clearer Ticket Assist modal layout.
- [x] Implement the Ticket Assist comment confirmation and worker-backed post flow.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Ticket Assist Output Clarity And Comment Path Scope

Ticket Assist results must clearly separate Claude's review from the user's local editable draft and
from available actions. The modal should make it obvious what came from Claude, what is local and
not yet applied, and what the user can do next. Add a comment path for tickets the user should not
directly edit: from the draft modal, `c` opens a confirmation to post the current draft as a Jira
comment, while the existing gated `ctrl+s` path remains the direct Summary/Description edit flow.

### Ticket Assist Output Clarity And Comment Path Design

Render the modal as three zones: `Claude Review`, `Local Draft`, and `Available Actions`. `Claude
Review` is a bounded read-only preview. `Local Draft` is the large editable region and shows a `Not
Applied` state label. `Available Actions` is a short body-level action hint that explains `ctrl+s`
apply, `c` comment, `r` refine, `ctrl+y` copy, and `esc` close; if Jira writes are disabled, it
should say so. Comment posting uses the existing worker AddComment request/result path with
Ticket-Assist-specific state and confirmation, then refreshes comments on success.

### Ticket Assist Output Clarity And Comment Path Review

- Renamed the review area to `Claude Review` and the editable area to `Local Draft Not Applied`.
- Added an `Available Actions` zone on normal-height terminals so apply/comment/refine/copy choices
  are visible in the body, not only the footer.
- Kept cramped terminals within bounds by hiding the body action zone and relying on the footer.
- Added `c` from the Ticket Assist draft modal to open a `Post Draft As Comment` confirmation.
- Posting a comment uses the existing worker `AddComment` path with Ticket-Assist-specific state,
  closes the modal on success, and refreshes comments.
- Captured inline detail-section AI actions as the next design slice: field/comment scoped AI
  should reuse the same local draft, refinement, apply, and comment machinery.
- Final verification passed with `go test ./internal/tui -run 'TestClaudeTicketAssist'`,
  `go test ./internal/tui`, `make check`, and `make install-user`.

## Ticket Assist Refinement Loop

- [x] Add TUI tests that `r` opens a refinement instruction editor from Ticket Assist.
- [x] Add TUI tests that submitting refinement sends Claude the current user-edited draft plus instruction.
- [x] Add TUI tests that the refinement result replaces the editable draft and remains local until apply.
- [x] Implement the refinement instruction editor, loading state, prompt builder, and result handling.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Ticket Assist Refinement Scope

Make Ticket Assist iterative without turning it into a general chat app. From an existing Ticket
Assist draft, `r` opens a small instruction editor. The user can tell Claude how to adjust the
current draft, then `ctrl+s` sends the refinement request. The prompt must include the current
user-edited draft, not only the original Claude output, so Claude can revise instead of
reinventing. The returned draft replaces the editable draft and still requires explicit user review
and gated Jira apply.

### Ticket Assist Refinement Design

Refinement reuses the existing Claude Ticket Assist modal and background runner. The modal has
three non-write states: editable draft, instruction editor, and Claude loading. The instruction
editor is local and cancellable with `esc`; `ctrl+s` sends a read-only refinement prompt containing
the original ticket context, the current edited draft, and the user instruction. While running, the
normal calm Claude loading state is shown. On success, the returned draft replaces the editable
draft and the review text updates if Claude returned a Review section. On failure, the current draft
and instruction remain available.

### Ticket Assist Refinement Review

- Added a refinement instruction editor opened with `r` from the Ticket Assist draft modal.
- Submitting refinement with `ctrl+s` runs Claude through the existing background Ticket Assist
  request/progress path and calm loading modal.
- The refinement prompt includes original ticket context, the user's instruction, and the current
  user-edited draft so Claude revises from the actual draft state.
- Successful refinement replaces the editable draft and updates review text if Claude returns a
  Review section; Jira writes remain gated behind the existing apply flow.
- Verified so far with `go test ./internal/tui -run 'TestClaudeTicketAssistR|TestClaudeTicketAssistRefinement'`
  and `go test ./internal/tui -run 'TestClaudeTicketAssist'`.
- Final verification passed with `go test ./internal/tui`, `make check`, and `make install-user`.

## Ticket Assist Apply To Jira

## Ticket Assist Apply To Jira

- [x] Add Jira client tests and implementation for updating Description with ADF text.
- [x] Add worker request/result tests and implementation for Description updates.
- [x] Add TUI tests that Ticket Assist `ctrl+s` opens an apply confirmation when Jira writes are enabled.
- [x] Add TUI tests that disabled Jira write gates keep Ticket Assist local-only.
- [x] Add TUI tests that confirming applies parsed Summary and Description while preserving draft on failure.
- [x] Fix Ticket Assist draft modal sizing so the editable draft is visually distinct and gets primary space.
- [x] Implement Ticket Assist apply confirmation and worker-backed save flow.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Ticket Assist Apply Scope

Make Ticket Assist actionable for existing tickets without adding a pile of new keys. When the
edited draft is ready, `ctrl+s` is the single save/apply action if Jira writes are enabled; `esc`
remains the non-writing close/cancel path. The first apply slice updates Jira Summary and
Description only. Acceptance Criteria stay first-class in the editable draft and are written into
Description under their own heading for now.

### Ticket Assist Apply Design

Ticket Assist keeps the editable draft modal. `ctrl+s` parses the edited draft into a Summary value
and a Description body, then opens a confirmation state that names the issue and fields to update.
Confirming submits worker-backed Jira updates for Summary and Description. The modal remains open
while saving. Success refreshes the visible issue summary and cached detail description, records
Diagnostics, and closes the modal with a notice. Failure leaves the edited draft intact and shows
the Jira error. If `allow_jira_writes` is false, `ctrl+s` does not write; the footer and notice
explain that Jira writes are disabled and `ctrl+y` remains the copy path.

### Ticket Assist Apply Review

- Added Jira client support for updating Description using the same ADF text payload style as issue
  creation.
- Added worker request/result support for Description updates.
- Extended TUI Claude config with `require_confirmation` and `allow_jira_writes` gates from the
  existing app config.
- Changed Ticket Assist `ctrl+s` from close-only to the gated save/apply path; `esc` remains the
  non-writing close/cancel path.
- Added an apply confirmation that previews Summary and Description before Jira writes.
- Confirming applies Summary and Description through the worker pool, updates the visible issue
  summary and cached detail description, and closes with a notice after both writes complete.
- With Jira writes disabled, Ticket Assist stays local-only and keeps `ctrl+y` as the copy/export
  path.
- Enlarged and visually separated the editable draft block, and hides the review preview on cramped
  terminals so draft editing remains usable.
- Final verification passed with focused Jira, worker, and TUI tests,
  `go test ./internal/jira ./internal/worker ./internal/tui`, `make check`, and
  `make install-user`.

## Existing Ticket Assistance

- [x] Add config/config UI coverage for a `ticket_assist` Claude feature flag.
- [x] Add TUI regression coverage that the Claude section exposes ticket assistance for existing tickets.
- [x] Add TUI regression coverage that ticket assistance sends a read-only evaluation/sanitization prompt.
- [x] Add TUI regression coverage that Claude's returned draft opens in an editable review modal.
- [x] Add TUI regression coverage that long Ticket Assist drafts are bounded, paged, and copyable.
- [x] Add TUI regression coverage that Ticket Assist loading suppresses partial assistant text.
- [x] Add TUI regression coverage that `a` jumps to the AI/Claude section when available.
- [x] Implement the gated ticket-assist workflow without Jira writes.
- [x] Calm Claude loading modals so they show stable subprocess status instead of changing partial text.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Existing Ticket Assistance Scope

Add the first existing-ticket assistance workflow. From a focused ticket detail Claude section, users
can ask Claude to evaluate the current Jira ticket and produce a structured improved draft. The
workflow must be read-only: no Jira writes, no git/GitHub/code changes, and no external side effects
beyond running the user's local Claude CLI. Claude output should be editable before use because the
model can overreach. Acceptance Criteria must be a first-class draft section even if Jira stores it
inside the Description for now.

### Existing Ticket Assistance Design

The Claude section should expose two distinct actions when enabled: ticket plan and ticket assist.
Ticket plan keeps the current implementation/verification plan behavior. Ticket assist sends the
selected ticket metadata, description, and loaded comments to Claude with instructions to return:
review findings, a rewritten summary, a problem/goal section, acceptance criteria, test/verification
notes, implementation notes, and open questions. When the request finishes, the TUI opens a
bounded modal with the review and an editable draft editor. The editor is local only in this slice;
later work can add copy/apply-to-Jira behind confirmation and write gates.

### Existing Ticket Assistance Review

- Added a `ticket_assist` Claude feature flag to config load/save and the config editor.
- Added Ticket Assist as a selectable Claude section action alongside Ticket Plan.
- Added a read-only Claude prompt for evaluating existing tickets and drafting Summary, Problem /
  Goal, Acceptance Criteria, Test / Verification, Implementation Notes, and Open Questions.
- Rendered Claude ticket-assist results in a modal with read-only review findings plus an editable
  local draft textarea; closing the modal does not write to Jira.
- Bounded long Ticket Assist review/draft output, added draft line-range/page hints, routed
  `pgup`/`pgdn` through the editor, and added `ctrl+y` to copy the edited draft.
- Calmed the Ticket Assist and Ticket Plan loading modals so running AI calls show stable output
  states instead of constantly changing partial assistant text; detailed stream output remains in
  Diagnostics and final result/error views.
- Made `a` jump to the Claude/AI section when Claude actions are available, while preserving
  add-comment as the fallback when Claude is unavailable and through the Actions workflow.
- Verified so far with focused config, config UI, and TUI tests.
- Final verification passed with `go test ./internal/claude ./internal/config ./internal/configui ./internal/tui`,
  `go test ./internal/tui`, `make check`, and `make install-user`.

## Claude Loading Modal Cleanup

- [x] Add TUI regression coverage for the concise Claude loading modal.
- [x] Add TUI regression coverage for rendering Claude Markdown tables as bounded table blocks.
- [x] Replace debug-heavy loading copy with subprocess activity, elapsed progress, and stable output status.
- [x] Render final Claude Markdown pipe tables through the existing fitted table renderer.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Claude Loading Modal Cleanup Scope

The normal Claude ticket-plan loading modal should feel like a running product workflow, not a
debug pane. Keep clear feedback that the local Claude subprocess is alive, keep elapsed progress,
show assistant preview/status when stream output arrives, and keep `esc` cancellation. Move command,
request, start, and deadline detail out of the normal waiting modal; timeout/error states and
Diagnostics can still carry deeper troubleshooting context.

### Claude Loading Modal Cleanup Review

- Replaced normal Claude loading modal debug rows with an activity spinner-style subprocess line,
  elapsed progress, and stable output state.
- Kept timeout/error handling rich enough to show start/deadline/command evidence when something
  actually fails.
- Added Markdown pipe-table normalization for final Claude plans so existing fitted table rendering
  handles wide Claude tables inside the bounded modal.
- Captured ticket-to-local-workspace mapping as the next planned feature slice for future Claude and
  code-workflow context.
- Verified so far with `go test ./internal/tui -run 'TestClaudePlanShowsSubprocessActivityAndCancelHintWhileRunning'`
  and focused Claude modal/table tests.
- Final verification passed with `go test ./internal/claude`, `go test ./internal/tui`,
  `make check`, and `make install-user`.

## Claude Plan Interaction Fix

- [x] Add TUI regression coverage for long final Claude results staying inside the visible panel.
- [x] Bound final Claude result rendering and show a line-range/scroll hint.
- [x] Add TUI regression coverage for assembling assistant delta chunks into one preview.
- [x] Update rolling assistant preview logic to append deltas while still deduping repeated cumulative partials.
- [x] Add Claude runner tests for stream-json CLI progress and stderr events.
- [x] Add TUI tests that Claude progress/stderr appears in the modal before final completion.
- [x] Run Claude with stream-json partial output when progress reporting is requested.
- [x] Feed Claude progress events through Bubble Tea commands while the request is running.
- [x] Add TUI regression tests for elapsed loading state and cancel behavior.
- [x] Add a cancellable context for in-flight Claude ticket-plan requests.
- [x] Render elapsed time, configured timeout, and cancel guidance while Claude is running.
- [x] Ignore stale/cancelled results while still recording Diagnostics.
- [x] Add absolute start/deadline and command context to the Claude modal.
- [x] Make timeout failures say how long Claude ran before the deadline.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Claude Plan Interaction Fix Scope

The first Claude ticket-plan workflow can legitimately take time or fail inside the local Claude
CLI. The TUI must not look frozen while this happens. Keep the request backgrounded, show elapsed
time and timeout context in the modal, allow `esc` to cancel the running process, and make the final
state visible through the modal and Diagnostics. Do not add Jira/git/GitHub/code write behavior.

### Claude Plan Interaction Fix Review

- Added a Claude runner streaming path that runs `claude --verbose --output-format stream-json
  --include-partial-messages -p <prompt>` when progress reporting is requested.
- Parsed stream-json stdout and stderr into progress events while preserving the final result.
- Parsed nested Claude `stream_event` content deltas into readable output events and suppressed raw
  JSON envelopes in the modal.
- Filtered protocol/user events out of visible progress and changed partial output rendering to a
  single rolling assistant preview to avoid repeated overlapping chunks.
- Assembled true assistant delta chunks into one readable preview while still deduping repeated
  cumulative partials.
- Bounded long final Claude plans inside the visible ticket-detail panel and added a Claude line
  range hint.
- Added independent scroll state for final Claude plan results with `j`/`k`, arrows, page keys, and
  top/bottom jumps.
- Made the Claude plan dialog use a responsive percentage of available width so final plans are
  readable on wide terminals.
- Added a Bubble Tea progress channel/message path so the modal can repaint before final completion.
- Rendered recent Claude progress/stderr/output events in the ticket-plan modal.
- Added regression coverage for elapsed/timeout/cancel modal text while Claude is running.
- Added regression coverage that `esc` cancels the in-flight Claude runner context.
- Added a Claude runner regression proving `context.DeadlineExceeded` does not fire before the
  configured timeout.
- Added cancellable request context storage for active Claude ticket-plan requests.
- Added a one-second redraw tick while Claude is running so elapsed time can update.
- Changed loading modal footer from `esc close` to `esc cancel`; after cancel, the modal shows the
  cancelled state and stale runner results are ignored while Diagnostics still records them.
- Added request type, read-only mode, sanitized command, output wait state, start time, deadline,
  and timeout-specific failure detail to the Claude modal.

## Claude Ticket Plan Workflow

- [x] Add Claude runner tests for executing a read-only prompt through the local CLI.
- [x] Implement `claude.LocalRunner.Run` with bounded output and timeout support.
- [x] Add TUI tests that the Claude section is hidden unless enabled, available, and `ticket_plan` is flagged on.
- [x] Add TUI tests that activating the Claude section submits a ticket-plan request with selected ticket context.
- [x] Add TUI tests that successful results render in a modal and diagnostics record submit/result.
- [x] Implement gated Claude detail section, request command, prompt builder, result state, and modal rendering.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Claude Ticket Plan Workflow Scope

First read-only Claude workflow. From focused ticket detail, show a Claude section only when Claude
is enabled, preflight is available, and the `ticket_plan` feature flag is true. Activating it sends
selected ticket context to local Claude Code/CLI in the background and renders the returned plan in
a modal. No Jira, git, GitHub, or code writes are allowed in this slice.

### Claude Ticket Plan Workflow Review

- Added `claude.LocalRunner.Run` for bounded local CLI prompt execution in print mode.
- Added a gated Claude ticket detail section that appears only when Claude is enabled, available,
  and `ticket_plan` is true.
- Built a read-only ticket-plan prompt from selected ticket metadata, description, and loaded
  comments.
- Submitted Claude plan requests asynchronously and rendered success/error output in the shared
  modal pattern.
- Recorded Diagnostics submit/result events for Claude ticket-plan requests.

## Config Boolean Pickers

- [x] Add config UI tests that boolean fields toggle without entering free-text edit mode.
- [x] Mark config fields with boolean value kind.
- [x] Render boolean fields as picker-style true/false values.
- [x] Route enter/space/left/right on boolean fields to toggle values directly.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Config Boolean Pickers Scope

Boolean config fields, especially Claude feature flags and write gates, should not require typing
`true` or `false`. Keep the config editor simple, but treat booleans as picker/toggle fields:
pressing enter or space toggles the selected value, and left/right chooses false/true while staying
in the config menu. Do not change text fields or the TOML representation.

### Config Boolean Pickers Review

- Added config UI regression coverage that boolean fields toggle without entering free-text edit
  mode.
- Marked Claude boolean fields as boolean value-kind fields in the config UI model.
- Rendered boolean values as `false / true` picker choices instead of plain text.
- Routed `enter` and `space` to toggle, `left` to false, and `right` to true while keeping focus in
  the config menu.
- Verified with focused config/config UI tests, `make check`, and `make install-user`.

## Claude Local Setup Foundation

- [x] Add config tests for Claude defaults, TOML load/save, feature flags, gates, and timeout validation.
- [x] Implement Claude config structs under the existing Jira config schema.
- [x] Add config UI tests that the Jira config menu exposes Claude settings.
- [x] Implement a Claude config section with enable/path/timeout/features/gates fields.
- [x] Add a local Claude runner/preflight package that uses `exec.LookPath("claude")` when no command override is set.
- [x] Add tests for Claude auto-detect, explicit command, version check, timeout, and not-found status.
- [x] Wire startup preflight status into the TUI model/diagnostics without blocking Jira startup when Claude is optional.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Claude Local Setup Foundation Scope

First infrastructure slice only: configure and verify local Claude Code/CLI availability. Default
Claude to disabled, default command to auto-detect `claude`, allow manual command/path override, and
record runtime status for diagnostics. Do not add Jira AI workflows yet. Claude workflows must later
check config feature flags and write gates before they are shown, before work is enqueued, and
before any generated action is applied.

### Claude Local Setup Foundation Review

- Added `[claude]`, `[claude.features]`, and `[claude.gates]` to the existing TOML config schema.
- Defaulted Claude to disabled, command auto-detect, a two-minute timeout, confirmation required,
  and all write gates closed.
- Added Claude fields to the existing `jira config` editor so users can enable Claude, override the
  command/path, set timeout, turn features on/off, and control write gates.
- Added `internal/claude.LocalRunner` for local Claude Code/CLI preflight using `exec.LookPath` when
  command is empty and `claude --version` with a bounded timeout.
- Wired startup preflight status into TUI Diagnostics without blocking Jira startup when optional
  Claude is disabled or unavailable.
- Preserved safe default confirmation while still honoring an explicit
  `require_confirmation = false` in TOML.
- Verified with focused config/config UI/Claude/TUI tests, `make check`, and `make install-user`.

## Bounded Create Modal

- [x] Add TUI regression tests for large create metadata lists staying inside a bounded modal.
- [x] Implement a scroll-window renderer for create dynamic fields and long picker option lists.
- [x] Keep focused create field and selected picker option visible while navigating.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Bounded Create Modal Scope

The create-ticket modal must not grow past the visible terminal when Jira returns many fields or a
long allowed-values list such as Components. Keep Summary and Description at the top, render Jira
metadata fields in a bounded area, and window long picker option lists around the selected option.
Do not introduce wizard pages yet and do not hard-code Jira fields.

### Bounded Create Modal Review

- Added a TUI regression test for a Components field with 30 Jira-provided options.
- Windowed long picker fields to six visible options plus an `Options x-y of n` range line.
- Kept the selected picker option visible as users move through long Jira option lists.
- Rendered inactive Summary and Description fields as compact one-line previews so large metadata
  forms fit the terminal while the focused text field still uses the editor surface.
- Verified with focused create modal tests, `make check`, and `make install-user`.

## Create Field Metadata Fallback

- [x] Add a Jira client regression test for empty field mappings falling back to expanded create metadata.
- [x] Implement fallback parsing for fields under `projects[].issuetypes[].fields`.
- [x] Update docs, changelog, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Create Field Metadata Fallback Scope

Jira is returning issue types from create metadata, but selected issue-type field mapping requests
are returning zero fields. Match the issue-type discovery pattern: keep the paged field-mapping
endpoint first, then fall back to expanded create metadata for the selected project and issue type
when the primary response is empty. Do not add UI presets or hard-code fields.

### Create Field Metadata Fallback Review

- Added a regression test proving empty `FetchFieldMappings` responses retry expanded create
  metadata with the selected project key and issue type ID.
- Added fallback parsing for `projects[].issuetypes[].fields`, reusing the same field parser as the
  paged field-mapping response.
- Sorted expanded field IDs before parsing so object-map response ordering stays deterministic.
- Verified with focused Jira/TUI create metadata tests, `make check`, and `make install-user`.

## Dynamic Create Fields

- [x] Add Jira client tests for create requests carrying priority and custom field values.
- [x] Extend typed create requests through Jira client and worker without raw body logging.
- [x] Add TUI tests for rendering Jira create metadata fields beyond Summary/Description.
- [x] Add TUI tests for picker/text field editing and submission payloads.
- [x] Implement dynamic create field state for supported text, labels, priority, and single-option fields.
- [x] Keep unsupported required Jira fields visible and block submit before Jira returns validation errors.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Dynamic Create Fields Scope

First slice: keep Summary and Description as dedicated editors, then render additional supported
fields returned by Jira create metadata. Supported fields are Priority, Labels, simple string/text
fields, and single-select option fields with Jira-provided allowed values. Unsupported required
fields are shown as blocking notices. Multi-value fields, cascading fields, rich user pickers, parent
issue pickers, components, fix versions, sprint, and asset fields remain follow-ups unless Jira
exposes them as simple allowed-value single-selects we can encode safely.

### Dynamic Create Fields Review

- Added typed `CreateIssueFieldValue` support to the Jira client and worker create request path.
- Encoded supported standard fields through go-atlassian issue fields: Priority, Labels, and
  Components.
- Encoded supported custom fields through go-atlassian `CustomFields.Raw`: simple text/number values
  and single-select option payloads.
- Added TUI state/rendering for supported create fields returned by Jira metadata, with `tab`
  navigation, `j`/`k` or arrow selection for pickers, and text input for simple fields.
- Unsupported required Jira fields now block submit with a clear local message instead of relying
  only on Jira validation errors.
- Added sanitized create-field diagnostics so `ctrl+d` shows total fields, supported fields,
  unsupported required fields, and a short field ID/name/schema sample for the selected issue type.
- Verified with focused Jira client, worker, and TUI create tests, then `make check`, then
  `make install-user`.

## Create Ticket Flow

- [x] Make `n` consistently create tickets from table and detail contexts while `tab` owns detail focus movement.
- [x] Add Jira client tests for creating an issue with project, issue type, summary, and ADF description.
- [x] Implement typed Jira create issue request/response support.
- [x] Add worker tests for background issue creation.
- [x] Implement worker create issue request/result support.
- [x] Add TUI tests for `n` opening create metadata, picking issue type, editing fields, and submitting.
- [x] Implement the first create-ticket modal flow.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Create Ticket Flow Scope

First usable ticket creation slice: `n` opens a modal, loads create issue types for the active
project, lets the user choose a Jira-provided issue type, loads fields for that type, and renders
Summary plus Description editors. Submitting creates the ticket through the worker pool. Additional
custom/required fields are not rendered yet; Jira errors should stay visible in the modal.

### Create Ticket Flow Review

- Added typed Jira issue creation support that builds project, issue type, summary, and ADF
  description payloads through the existing Jira client boundary.
- Added worker-backed create requests/results so ticket creation stays off the Bubble Tea
  update/render loop.
- Added a go-atlassian create metadata fallback: if `FetchIssueMappings` succeeds with zero issue
  types, the client retries `Issue.Metadata.Create` with `projects.issuetypes` expansion.
- Added a first create-ticket modal opened with `n` from the issue table or focused ticket detail,
  with Jira-provided issue type selection, Summary and Description editors, and worker-backed
  submission.
- Updated key behavior so `tab`/`shift+tab` own ticket-detail focus movement and `n` consistently
  means new ticket instead of also acting as section navigation.
- Added the keymap clarity review as its own follow-up so future shortcuts keep one clear semantic
  path per workflow.
- Added clearer empty create metadata handling: if Jira returns zero creatable issue types, the
  modal reports that result, hides inactive field/submit commands, and leaves `ctrl+d` diagnostics
  available with sanitized result counts.
- Verified with focused Jira client, worker, and TUI tests, then `make check`, then
  `make install-user`.

## Keymap Clarity Review

- [ ] Audit every active key context for redundant commands that do the same thing with different
      semantics.
- [ ] Keep conventional navigation aliases only where they reduce friction without changing meaning
      (`j`/`k` with arrows, page keys, home/end).
- [ ] Prefer one primary action path per workflow: `tab` moves focus, `enter` acts on focus, and
      single-letter commands are stable accelerators for distinct actions.
- [ ] Add tests for any removed or reassigned binding so help text and behavior stay aligned.
- [ ] Update docs/changelog with the keymap clarity rules and the resulting command changes.

## Sanitized API Debug Log

- [ ] Extend Diagnostics into an opt-in API debug/event log for Jira operations.
- [ ] Record request IDs, operation names, endpoint families, project/issue keys, status/error
      classes, timing, and result counts without raw response bodies or credentials.
- [ ] Surface empty metadata responses clearly, such as create issue types returning zero values.
- [ ] Add a future export path that can prefill a GitHub issue for this app with sanitized logs and
      app/version/context data.

## Create Metadata Discovery

- [x] Add Jira client tests for project create issue type discovery.
- [x] Add Jira client tests for issue-type create field metadata discovery.
- [x] Implement typed create metadata structs and Jira client methods.
- [x] Add worker request/result tests for create issue types and create fields.
- [x] Implement worker request/result support for create metadata discovery.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Create Metadata Discovery Scope

First slice for ticket creation: discover Jira create metadata without rendering a create form yet.
Fetch create issue types for a project, then fetch create fields for a selected issue type. Keep
these calls worker-backed for future TUI use, parse Jira-provided required/allowed/schema data, and
avoid hard-coded presets.

### Create Metadata Discovery Review

- Added typed `CreateIssueType` and `CreateField` domain structs in `internal/jira`.
- Added `GetCreateIssueTypes(projectKey)` using Jira create metadata issue-type mappings.
- Added `GetCreateFields(projectKey, issueTypeID)` using Jira create metadata field mappings.
- Parsed required/default flags, schema type/system/items/custom data, operations, allowed values,
  and autocomplete URLs for future create-form rendering.
- Added worker request/result support for project issue types and selected issue-type fields so the
  future TUI create flow can stay backgrounded.
- Verified so far with `go test ./internal/jira -run 'TestGetCreate' -count=1`,
  `go test ./internal/worker -run 'TestPoolGetCreate|TestPoolReturnsInvalidSummaryEditRequestResults' -count=1`,
  and `go test ./...`.
- Final verification passed with `make check` and `make install-user`.

## Diagnostics Overlay Polish

- [x] Add TUI tests for diagnostics summary counts and activity bars.
- [x] Add TUI tests that diagnostics rows include scan-friendly operation labels.
- [x] Implement a compact diagnostics summary strip without adding Jira IO or render-blocking work.
- [x] Update docs, changelog, and task review.
- [x] Run focused tests, `make check`, and `make install-user`.

### Diagnostics Overlay Polish Scope

Polish the existing read-only Diagnostics overlay so background activity scans faster. Add a compact
summary strip, simple activity bars, and clearer row detail labels. Do not add a timer, persistence,
new Jira requests, or animated state that is not tied to recorded worker/cache events.

### Diagnostics Overlay Polish Review

- Added a compact summary strip with worker/cache/error/active counts and the last recorded event.
- Added simple worker/cache activity bars computed from the retained diagnostics buffer.
- Added operation labels to event details so rows read as `get_issue #7 ABC-1` or
  `issue_detail ABC-1` instead of bare IDs.
- Kept the overlay read-only and render-local: no timers, persistence, new Jira calls, or background
  work were added for this visual polish.
- Verified so far with `go test ./internal/tui -run 'TestDiagnostics' -count=1` and
  `go test ./internal/tui -count=1`.
- Final verification passed with `make check` and `make install-user`.

## Diagnostics Overlay

- [x] Add TUI tests that the diagnostics overlay toggles without changing the active workflow.
- [x] Add TUI tests that worker submit/result activity is recorded in a bounded in-memory buffer.
- [x] Add TUI tests that detail-cache hit/miss/stale-refresh decisions are recorded.
- [x] Implement the read-only diagnostics overlay with `ctrl+d` and `esc` close behavior.
- [x] Wire diagnostics events to existing worker messages and detail TTL cache decisions.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Diagnostics Overlay Scope

First slice: add a toggleable, read-only overlay for recent background activity. It should show
worker submit/result events and detail-cache hit/miss/stale refresh decisions from an in-memory
bounded buffer. It must not make Jira calls, block rendering, persist logs, or interfere with
active text-editing flows.

### Diagnostics Overlay Review

- Added a bounded in-memory diagnostics event buffer to the TUI model.
- Added `ctrl+d` to open a read-only Diagnostics overlay and `esc`/`ctrl+d` to close it.
- Recorded worker submit/result events from the existing worker message path.
- Recorded issue-detail cache hit, miss, and stale-refresh decisions from the existing TTL cache
  decision path.
- Verified so far with `go test ./internal/tui -run 'TestDiagnostics' -count=1` and
  `go test ./internal/tui -count=1`.
- Final verification passed with `make check` and `make install-user`.

## Detail TTL Cache Refresh

- [x] Add TUI tests that fresh cached issue detail avoids duplicate Jira detail requests.
- [x] Add TUI tests that stale cached issue detail stays visible while a background refresh starts.
- [x] Add TUI tests that loaded detail marks the cache entry fresh.
- [x] Implement issue-detail freshness tracking with `github.com/jellydator/ttlcache/v3`.
- [x] Keep stale detail display non-blocking and refresh only through the worker pool.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Detail TTL Cache Scope

First slice: keep the existing in-memory issue detail map as the stale display cache, and add a
short-lived TTL freshness marker using `ttlcache`. Fresh detail avoids duplicate Jira reads. Stale
detail remains visible immediately, but the worker pool refreshes it in the background. This keeps
the TUI responsive while making detail data less permanently stale.

### Detail TTL Cache Review

- Added issue-detail freshness tracking with `github.com/jellydator/ttlcache/v3`.
- Kept the existing detail map as the stale display cache so already-loaded details remain visible.
- Fresh details skip duplicate Jira detail requests.
- Stale details start a worker-backed background refresh without blocking the detail view.
- Loaded detail responses mark the selected issue detail fresh for a short TTL.
- Verified with `go test ./internal/tui -run 'Test(FreshCachedDetailSkipsDetailRefresh|StaleCachedDetailStartsBackgroundRefresh|LoadedDetailMarksDetailCacheFresh)' -count=1`,
  `make check`, and `make install-user`.

## Assignee Edit Workflow

- [x] Add Jira client tests for assigning an issue by Jira account ID.
- [x] Implement minimal Jira assignee update support through `Issue.Assign`.
- [x] Add worker request/result tests for assignee updates.
- [x] Implement background worker assignee update flow.
- [x] Add TUI tests for Assignee as a focusable detail field.
- [x] Add TUI tests for opening a Jira-user picker and applying the selected assignee.
- [x] Implement the Assignee picker without relying on the Actions menu.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Assignee Edit Scope

First slice: make Assignee a first-class ticket-detail focus target after Summary and before
Priority. `tab`/`shift+tab` moves to it, and `enter` opens a modal picker with type-to-filter Jira
user search. Typing characters such as `jon` updates the query and refreshes Jira user results
through the worker pool. Selecting a user submits an assignee update by Jira account ID through the
worker pool and updates the visible issue row plus cached detail assignee on success.

This intentionally does not build the full generic edit-fields framework yet. It proves the next
metadata-backed field shape with the smallest useful workflow: a user picker populated from Jira,
keyboard selection, submit, cancel, and worker result handling.

### Assignee Edit Review

- Added Jira client support for assigning an issue by Jira account ID through `Issue.Assign`.
- Added worker request/result support for assignee updates so assignment stays backgrounded.
- Added Assignee as a first-class ticket-detail focus target between Summary and Priority.
- Added a `Change Assignee` modal with type-to-filter Jira user search, arrow-key selection,
  worker-submitted assignment, and cancel behavior.
- Added a short-lived in-memory TTL cache for Jira user-search results using
  `github.com/jellydator/ttlcache/v3`, with case-insensitive query keys.
- Deferred broader background cache refresh tooling to the next cache-framework slice: use
  `ttlcache` eviction/loader hooks where they fit, and keep ticket refresh async/merge-based rather
  than blocking the UI.
- Verified with `go test ./internal/jira ./internal/worker ./internal/tui -run 'Test(UpdateAssignee|PoolUpdateAssignee|PoolReturnsInvalidSummaryEditRequestResults|DetailTabFocusesEditableFieldsBeforeSections|DetailEnterOnFocusedAssigneeOpensTypeaheadPicker|AssigneePickerSubmitsSelectedUser|AssigneePickerUsesCachedUserSearch|AssigneeUpdateSuccessUpdatesIssueAndDetailAssignee)' -count=1`,
  `make check`, and `make install-user`.

## Detail Field Focus Workflow

- [x] Add TUI tests for tabbing through Summary and Priority fields before section tabs.
- [x] Add TUI tests that `enter` on focused Summary opens the Summary editor modal.
- [x] Add TUI tests that `enter` on focused Priority opens the Priority picker modal.
- [x] Implement unified detail focus targets for header fields plus existing sections.
- [x] Keep `s` and `p` as temporary accelerators that use the same activation path.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Detail Field Focus Scope

First slice: make Summary and Priority first-class detail focus targets before the existing
Description/Hierarchy/Comments/Actions/Status section targets. `tab` and `shift+tab` move through
fields and sections; `enter` activates the focused target. Text fields still open editor modals,
set-value fields still open picker modals, and existing single-letter shortcuts remain aliases
rather than the primary workflow.

### Detail Field Focus Review

- Added Summary and Priority as first-class ticket-detail focus targets before the section tabs.
- `tab` and `shift+tab` now move through editable fields and sections; `enter` activates the
  focused target.
- `s` and `p` remain accelerators into the same Summary and Priority modal flows.
- Status activation now follows the same direct-edit rule: selecting Status and pressing `enter`
  starts the Jira-backed transition picker/load instead of requiring an intermediate focus step.
- Verified with `go test ./internal/tui -count=1`, `make check`, and `make install-user`.

## Priority Edit Workflow

- [x] Add Jira client tests for parsing priority edit metadata and allowed values.
- [x] Add Jira client tests for updating the selected issue priority.
- [x] Implement minimal Jira priority metadata and update support.
- [x] Add worker request/result tests for priority update.
- [x] Implement background worker priority update flow.
- [x] Add TUI tests for direct Priority picker modal editing.
- [x] Implement the Priority picker without relying on the Actions menu.
- [x] Update docs and changelog.
- [x] Run focused tests, `make check`, and `make install-user`.

### Priority Edit Scope

First slice: pressing `p` in ticket detail starts a metadata-backed Priority edit flow for the
selected issue. The TUI reuses cached edit metadata or loads it through the worker pool; if Jira
reports priority as editable and supplies allowed values, the modal renders a picker/dropdown-style
list. `j`/`k` or arrow keys select the new priority, `enter` submits the selected value through the
worker pool, and `esc` cancels.

This follows the modal split: Priority is an enumerated field, so it uses a picker rather than a
text editor. Jira allowed values must come from edit metadata; do not hard-code a preset list.

### Priority Edit Review

- Extended Jira edit metadata parsing with Priority and Jira-provided allowed values.
- Added Jira and worker support for priority updates using Jira priority ID/name values.
- Added `p` as the direct Priority edit shortcut in ticket detail; previous-section movement remains
  available through `shift+tab` and `[`.
- Added a `Change Priority` picker modal with current priority, Jira-loaded options, selection
  movement, apply, and cancel behavior.
- Successful priority updates refresh the visible issue row and cached detail priority immediately.
- Verified with `go test ./internal/jira ./internal/worker ./internal/tui -run 'Test(GetEditMetadataParsesPriorityAllowedValues|UpdatePriority|PoolUpdatePriority|PoolReturnsInvalidSummaryEditRequestResults|Priority)' -count=1`,
  `go test ./internal/tui -count=1`, `make check`, and `make install-user`.

## Detail Overlay Dialogs

- [x] Add failing TUI tests for Summary edit rendering as a centered overlay dialog.
- [x] Add failing TUI tests for Status transition rendering as the same overlay dialog pattern.
- [x] Implement a small reusable detail overlay renderer in `internal/tui/model.go`.
- [x] Move Summary edit draft rendering out of the header row and into the overlay.
- [x] Move Status transition picker rendering out of the Status section body and into the overlay.
- [x] Update docs and changelog.
- [x] Run focused tests, `make check`, and `make install-user`.

### Detail Overlay Scope

First slice: keep the current detail screen as the background and render focused edit/selection
workflows in a layered dialog. Summary edit owns the draft text, loading/saving/error notice, and
`enter`/`esc` hints inside the overlay. Status transitions reuse the same shell for Jira-loaded
transition choices and preserve `j`/`k`, `enter`, and `esc` behavior.

This is a rendering and interaction cleanup only. Jira metadata, summary updates, transition loads,
and transition submissions stay worker-backed and unchanged.

### Detail Overlay Review

- Added a shared ticket-detail dialog shell for focused mutation workflows.
- Summary editing now keeps the saved summary in the detail header and renders the unsaved draft in
  an `Edit Summary` dialog with explicit save/cancel hints.
- Summary focus no longer renders an instructional Notice block before the dialog opens; the footer
  owns the `enter`/`esc` guidance in that intermediate state.
- Long Summary drafts now render as an input-style line biased toward the edit cursor so typing at
  the end of a long value is visible.
- Pressing `s` now starts the metadata-backed Summary edit flow immediately and renders loading/edit
  states in the modal shell instead of stopping in a footer-only focus state.
- Text-backed modal fields use editor surfaces; enumerated modal fields keep picker/list selection.
- Status transition selection now renders in a `Change Status` dialog with current status,
  Jira-loaded transition choices, selection cursor, and apply/cancel hints.
- Verified with `go test ./internal/tui -run 'Test(SummaryEditorRendersAsOverlayDialog|StatusTransitionPickerRendersAsOverlayDialog)' -count=1`,
  `make check`, and `make install-user`.

## Summary Edit Workflow

- [x] Add Jira client tests for fetching edit metadata for the selected issue.
- [x] Add Jira client tests for updating the issue summary.
- [x] Implement minimal Jira edit metadata and summary update client methods.
- [x] Add worker request/result tests for metadata loading and summary update.
- [x] Implement background worker edit metadata and summary update flows.
- [x] Add TUI tests for direct Summary section editing.
- [x] Implement Summary section editor without relying on the Actions menu.
- [x] Update docs and changelog.
- [x] Run focused tests, `make check`, and `make install-user`.

### Summary Edit Review

- Added Jira client methods and tests for issue edit metadata and summary updates.
- Added worker request/result handlers so metadata loading and summary updates stay backgrounded.
- Added direct Summary field focus in ticket detail with `s`; focused `enter` loads metadata and
  opens a prefilled one-line editor, then `enter` submits the update.
- Fixed duplicate/queued `enter` handling so Summary drafts cannot submit until the user actually
  edits the opened draft.
- Successful summary updates refresh the visible issue row and cached detail summary immediately.
- Verified with `go test ./internal/jira ./internal/worker ./internal/tui -count=1`,
  `make check`, and `make install-user`.

### Summary Edit Scope

First slice: add a direct Summary section in ticket detail. Selecting Summary and pressing `enter`
loads Jira edit metadata through the worker pool; if Jira reports `summary` as editable, the TUI
opens a small prefilled editor. Submitting updates only the summary through Jira, then updates the
visible issue row and cached detail.

The first slice intentionally edits only Summary. Priority, assignee, labels, and custom fields
should reuse this metadata/edit framework in later slices instead of sharing a generic Actions menu.

## Status Transition Workflow

- [x] Add Jira client tests for listing available issue transitions.
- [x] Add Jira client tests for applying a selected transition.
- [x] Implement minimal Jira transition client methods with local DTOs.
- [x] Add worker request/result tests for transition loading and submission.
- [x] Implement background worker transition flows.
- [x] Add TUI tests for direct status-field transition workflow.
- [x] Implement the status transition picker without relying on the Actions menu.
- [x] Update docs and changelog.
- [x] Run focused tests, `make check`, and `make install-user`.

### Status Transition Review

- Added Jira client methods and tests for listing and applying issue transitions.
- Added worker request/result handlers so transition loading and submission stay backgrounded.
- Added a direct ticket detail Status section with a Jira-populated transition picker; `enter`
  loads transitions, `j`/`k` selects, and focused `enter` applies the transition.
- Successful transitions update the visible issue row and cached detail status immediately.
- Verified with `go test ./internal/jira ./internal/worker ./internal/tui -count=1`,
  `make check`, and `make install-user`.

### Status Transition Scope

First slice: from a focused ticket detail status field/section, `enter` loads available Jira
transitions in the background, renders a small picker populated from Jira, and submits the selected
transition through the worker pool. On success, the visible issue status is updated or refreshed.

The pinned go-atlassian client can list and apply transitions, but does not expose
`expand=transitions.fields`; transition-screen field metadata remains a follow-up lower-level Jira
request rather than a hard-coded preset list.

## Detail Footer Section Commands

- [x] Confirm UX scope for selected-but-not-activated section footer commands.
- [x] Add failing footer tests for Hierarchy, Links, and Actions selected sections.
- [x] Add section-aware footer bindings while preserving active sub-mode footers.
- [x] Update docs and changelog.
- [x] Run focused tests, `make check`, and `make install-user`.

## Description And Comments UX Rhythm

- [x] Confirm ordered UX scope: comments, description, then states.
- [x] Add failing tests for lighter comment headers and body spacing.
- [x] Improve comment block rendering.
- [x] Add failing tests for description section spacing around rich content.
- [x] Improve description rendering rhythm.
- [x] Add failing tests for aligned detail state blocks.
- [x] Align description/comment loading, empty, and error states.
- [x] Update docs and changelog.
- [x] Run focused tests, `make check`, and `make install-user`.

## Hierarchy Detail Polish

- [x] Explore project context and backlog.
- [x] Confirm hierarchy detail design direction.
- [x] Write implementation plan after design approval.
- [x] Add failing TUI tests for grouped hierarchy rendering and selection.
- [x] Implement grouped hierarchy rows in `internal/tui/model.go`.
- [x] Keep hierarchy activation/back-stack behavior working across grouped rows.
- [x] Run focused TUI tests.
- [x] Update docs and changelog for the hierarchy tab behavior.
- [x] Run `make check`.

## Review

- Implemented grouped Hierarchy tab rendering for Path, Children, Subtasks, and a linked-issues placeholder.
- Added focused TUI coverage for grouped hierarchy rendering and opening a selected grouped subtask.
- Verified with `go test ./internal/tui -count=1`, `make check`, and `make install-user`.

## Implementation Plan

### Scope

Improve the focused ticket detail Hierarchy tab so it renders a grouped tree workspace:

- `Path` section for known parent context.
- `Children` section for visible non-subtask children.
- `Subtasks` section for visible subtasks.
- `Linked Issues` placeholder section only, because linked issue data is not currently exposed through the Jira client.

No new Jira API calls are part of this slice. Existing table expansion through `x` and `X` remains table-mode behavior.

### Files

- Modify `internal/tui/model.go`
  - Add a small internal hierarchy row model.
  - Split visible related issues into children and subtasks.
  - Render grouped sections with the existing section header, empty-state, and detail table helpers.
  - Map `selectedHierarchy` to selectable issue rows across groups.
- Modify `internal/tui/model_test.go`
  - Add RED tests for grouped Path/Children/Subtasks rendering.
  - Add RED tests that `enter` opens the selected grouped subtask/child and preserves the detail back stack.
- Modify docs after code is green:
  - `docs/project-state.md`
  - `docs/backlog.md`
  - `docs/releases/CHANGELOG.md`

### TDD Steps

- [ ] Write `TestHierarchySectionRendersGroupedTree`.
  - Build a detail-mode model with selected parent `ABC-1`, one child story, and one subtask.
  - Assert the rendered hierarchy contains `Path`, `Children 1`, `Subtasks 1`, the child key, and the subtask key.
  - Assert it does not use the old flat empty-state wording when grouped rows exist.
- [ ] Run `go test ./internal/tui -run TestHierarchySectionRendersGroupedTree -count=1`.
  - Expected: FAIL because the current renderer has no grouped section labels/counts.
- [ ] Implement the smallest grouped renderer in `internal/tui/model.go`.
  - Add `hierarchyRow` and grouping helpers near the hierarchy renderer.
  - Render parent path first, then child and subtask tables.
  - Keep selectable rows flat internally so existing `selectedHierarchy` behavior stays simple.
- [ ] Run `go test ./internal/tui -run TestHierarchySectionRendersGroupedTree -count=1`.
  - Expected: PASS.
- [ ] Write `TestHierarchyEnterOpensSelectedGroupedSubtask`.
  - Focus Hierarchy, select the subtask row after the child row, press `enter`.
  - Assert selected issue changes to the subtask and previous selection is pushed to the back stack.
- [ ] Run `go test ./internal/tui -run TestHierarchyEnterOpensSelectedGroupedSubtask -count=1`.
  - Expected: PASS if row mapping was implemented cleanly; otherwise fix mapping only.
- [ ] Run `go test ./internal/tui -count=1`.
- [ ] Update docs and changelog.
- [ ] Run `make check`.
