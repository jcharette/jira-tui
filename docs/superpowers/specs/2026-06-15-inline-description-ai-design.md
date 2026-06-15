# Inline Description AI Design

## Status

Approved direction: A+ hybrid.

This spec covers the first inline AI slice for focused ticket detail. It intentionally starts with
the Description section only. Comments should use the same pattern later, after the Description flow
proves the interaction model without adding complexity to comment composition.

## Goal

Let a user improve or reason about the current ticket Description from the Description section
itself, without requiring a trip through the Claude tab, while still preserving explicit user review
before any Jira write.

## User Experience

In normal ticket detail, the Description section stays simple. When Description is focused and
Claude Ticket Assist is enabled and available, the footer exposes `a AI`.

Pressing `a` opens a small contextual picker titled `AI for Description`. The picker is local UI
state; it does not start Claude until the user chooses an action. It uses the established modal
pattern with `j`/`k` or arrow selection, `enter` to run, and `esc` to cancel.

The first supported actions are:

- `Improve clarity`: rewrite the current Description for clearer intent, scope, acceptance
  criteria, and verification detail.
- `Extract acceptance criteria`: inspect the ticket and current Description, then draft explicit
  acceptance criteria and any missing questions.
- `Ask Claude a question`: open a short instruction editor. The user's question is sent with the
  full ticket context and current Description.
- `Draft clarifying comment`: produce a comment draft that can be posted to Jira without editing
  Summary or Description.

Running an action opens the existing Ticket Assist-style result modal. The modal must keep the same
clear zones:

- `Claude Review`: read-only summary of what Claude noticed.
- `Local Draft`: editable draft text that has not been applied to Jira.
- `Available Actions`: explicit next steps.

The user can refine the draft, copy it, post it as a comment, or apply it to Description when Jira
writes are enabled and confirmation gates allow it. For this slice, inline Description AI should not
update Summary. Description apply must reuse the existing worker-backed `UpdateDescription` path.

## Scope

Included:

- Description-only inline AI entry from focused ticket detail.
- A contextual AI picker opened with `a` when Description is focused.
- Action-specific Claude prompts built from selected ticket metadata, loaded Description, and loaded
  comments.
- Reuse of the existing Claude subprocess runner, progress handling, diagnostics, cancellation, and
  Ticket Assist result modal.
- Reuse of the existing local draft, refinement, copy, comment-post, and gated apply concepts.
- Tests for footer visibility, picker behavior, prompt content, result modal behavior, and
  Description-only apply semantics.

Excluded from this slice:

- Inline AI from Comments.
- General chat with Claude.
- Applying Summary changes from inline Description AI.
- Git, GitHub, branch, PR, or code edits.
- New Jira metadata discovery beyond the existing Description update path.
- Persisted training data or ticket-to-workspace mappings.

## Architecture

The TUI should add a small amount of AI action state alongside the existing Claude Ticket Assist
state. The state should identify:

- active inline AI scope, initially `description`;
- selected inline AI action;
- optional user instruction for question-style actions;
- whether the inline AI picker or instruction editor is open.

The Claude runner should stay unchanged. Inline Description AI should submit requests through the
same `submitClaudeTicketAssist` command path used by Ticket Assist and refinement, with a prompt
builder that names the selected inline action and constrains allowed side effects.

The result handling should reuse the existing `claudeAssist*` modal state where practical. Inline
Description AI can set `claudeAssistOpen`, `claudeAssistText`, `claudeAssistDraft`, and the draft
editor exactly like Ticket Assist, but it must also record that the current draft target is
Description-only. That target controls what `ctrl+s` means:

- For normal Ticket Assist: existing Summary plus Description apply behavior remains unchanged.
- For inline Description AI: `ctrl+s` opens a Description-only confirmation and submits only
  `UpdateDescription`.
- For draft clarifying comment: `c` remains available, and `ctrl+s` should not post without
  confirmation. Comment posting continues through the existing `AddComment` worker path.

This keeps the result modal and refinement loop consistent while preventing inline Description work
from accidentally changing Summary.

## Prompts

All prompts must explicitly state that Claude must not update Jira, run git, call GitHub, edit code,
or make external changes. The only output should be review text and a local draft for the TUI.

The prompt must include:

- issue key, summary, status, type, priority, assignee, reporter;
- loaded Description text;
- loaded comments, if available;
- the selected inline action;
- the current user-edited draft when refining;
- the user's question when using `Ask Claude a question`.

For Description improvement, the draft should be suitable as a replacement Description. For comment
drafting, the draft should be suitable as a Jira comment and should not pretend to be the ticket
description.

## Key Behavior

Normal detail mode:

- `a` continues to jump to the Claude section when focus is not on a supported inline AI section.
- When Description is focused and inline Description AI is available, `a` opens `AI for
  Description`.
- If Claude is disabled, unavailable, or `ticket_assist` is disabled, `a` keeps the existing
  behavior and no inline AI hint appears.

Inline AI picker:

- `j`/`k` and arrows move selection.
- `enter` runs the selected action or opens the instruction editor for `Ask Claude a question`.
- `esc` closes the picker and returns to normal detail view.

Result modal:

- `r` refines using the current local draft and the user's instruction.
- `ctrl+y` copies the current local draft.
- `c` opens the existing comment confirmation.
- `ctrl+s` applies only to Description when writes are enabled and confirmation passes.
- `esc` closes without writing.

## Error Handling

Unavailable Claude shows a non-blocking detail notice and does not open the picker.

If Description detail is still loading or unavailable, the picker can open, but running an action
must not call Claude. The TUI should show a non-blocking detail notice: `Description is not loaded
yet.` This keeps the first slice deterministic and prevents Claude from drafting against incomplete
field context.

Claude failures and cancellations should reuse the existing Ticket Assist loading/error states and
Diagnostics events. Jira write failures should preserve the local draft and show the existing
failure notice pattern.

## Testing

Add focused TUI tests before implementation:

- Description focus shows `a AI` only when Claude Ticket Assist is enabled and available.
- Pressing `a` on Description opens an `AI for Description` picker instead of jumping to the Claude
  section.
- The picker renders the four first-slice actions and supports selection/cancel.
- Running `Improve clarity` submits a Claude request with selected issue context and Description
  content.
- Running `Ask Claude a question` opens an instruction editor and sends the instruction with the
  prompt.
- A finished inline Description AI result opens the existing Ticket Assist-style modal with
  `Claude Review`, `Local Draft`, and `Available Actions`.
- Applying an inline Description AI draft updates Description only, not Summary.
- Posting an inline Description AI draft as a comment uses the existing comment confirmation and
  worker-backed `AddComment` flow.

Final verification for implementation should include focused TUI tests, `go test ./internal/tui`,
`make check`, and `make install-user`.

## Future Work

After this slice, extend the same scoped AI picker to Comments:

- improve a manually drafted comment before posting;
- ask Claude questions about the ticket from the Comments section;
- draft a clarifying comment from ticket context;
- preserve the existing mention-aware comment editor and confirmation flow.

The same model can later support ticket-to-local-workspace context, training-data capture, and
code/PR workflows, but those should remain behind separate feature flags and explicit write gates.
