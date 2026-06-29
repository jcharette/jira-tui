# Keyboard Reference

Press `?` in the app for context-aware help. This page explains the main keys by screen.

## Global

| Key | Action |
| --- | --- |
| `?` | Open help for the current screen |
| `q` / `ctrl+c` | Quit or cancel, depending on screen |
| `esc` | Back or cancel |
| `ctrl+d` | Open Diagnostics |
| `ctrl+n` | Open Notifications |
| `B` | Open bug report composer |

## Issue List

| Key | Action |
| --- | --- |
| `j/k`, arrows | Move selected issue |
| `pgup` / `pgdn` | Page through issues |
| `g` / `G` | Jump first or last |
| `enter` | Open selected ticket detail |
| `r` | Refresh active view |
| `/` | Open query screen |
| `tab` / `shift+tab` | Switch saved views |
| `L` | Cycle layouts |
| `f` | Toggle local active-ticket filter |
| `o` / `O` | Cycle sort |
| `n` | Create ticket |
| `T` | Create toil ticket |
| `x` | Load open children for selected parent |
| `X` | Load all children for selected parent |
| `z` | Collapse or expand loaded descendants |

## Ticket Detail

| Key | Action |
| --- | --- |
| `tab` / `shift+tab` | Move through fields and sections |
| `j/k` | Scroll detail content or move inside focused lists |
| `enter` | Edit or activate focused field/section |
| `.` | Open Ticket Actions |
| `s` | Edit Summary |
| `p` | Edit Priority |
| `o` | Open Jira issue URL |
| `c` | Copy Jira issue URL |
| `n` | Create ticket or subtask from context |
| `T` | Create toil ticket |
| `a` | Open AI/Claude actions when enabled |
| `esc` | Return to issue list |

Ticket Actions includes Sprint, which lists the active sprint first and future sprints below it. It
also includes Fix Version, Affects Version, Due Date, Parent, and Estimates when Jira edit metadata
exposes those fields for the selected issue.

When a Ticket Assist draft modal is open, printable letters stay in the focused draft or answer
editor. Use `enter` to answer parsed Open Questions, `j`/`k` to select questions, `ctrl+r` to
refine with Claude or refine with saved answers, `ctrl+c` to post the draft as a comment, `ctrl+s`
to apply when Jira writes are enabled, and `ctrl+y` to copy the draft. In Ticket Assist text boxes,
`shift+arrow` selects text when supported by the terminal; `ctrl+space` starts selection everywhere,
normal arrows extend it, `ctrl+y` copies it, `delete`/`backspace` removes it, and `esc` clears it.
After applying a whole-ticket draft with Subtask Recommendations, the Review Subtask Changes modal
uses `j`/`k` to select a recommendation, `enter` to apply it, `s` to skip it, and `esc` to finish.

## Query

| Key | Action |
| --- | --- |
| `tab` | Switch query mode |
| `ctrl+s` | Run direct JQL or generate AI JQL |
| `s` / `ctrl+v` | Save query as a view |
| `j/k` | Move through templates, recent queries, or saved views |
| `r` / `d` / `i` / `J` / `K` | Manage saved views when in Views mode |
| `esc` | Close query screen |

## Create Ticket

| Key | Action |
| --- | --- |
| `j/k` | Move through issue types or fields |
| `enter` | Continue or activate selected field |
| `tab` | Move fields or switch create mode when AI is enabled |
| `ctrl+s` | Generate or submit, depending on mode |
| `ctrl+r` | Refine a generated draft with answered Open Questions |
| `esc` | Cancel |

## Create Toil

| Key | Action |
| --- | --- |
| `tab` | Move between Summary, Duration, Note, and Close after create |
| `space` | Toggle Close after create when focused |
| `ctrl+s` | Create the toil ticket and log work |
| `esc` | Cancel |

## Comments

| Key | Action |
| --- | --- |
| `enter` | Add or focus comment action |
| `e` | Edit selected comment when focused |
| `@` | Open mention picker while composing |
| `ctrl+s` | Submit after review |
| `esc` | Cancel or leave comment focus |

## Notifications

| Key | Action |
| --- | --- |
| `j/k` | Move selected notification |
| `enter` | Open related ticket when available |
| `x` | Clear selected notification |
| `ctrl+x` | Clear all notifications |
| `esc` | Close notifications |

## Diagnostics

| Key | Action |
| --- | --- |
| `j/k`, `pgup` / `pgdn` | Scroll diagnostics |
| `esc` | Close diagnostics |

## Config

| Key | Action |
| --- | --- |
| `left/right` | Switch config sections |
| `tab` / `shift+tab` | Switch config sections |
| `j/k` | Move fields or theme cards |
| `enter` | Edit, select, or toggle |
| `space` | Toggle or cycle picker fields |
| `t` | Test Jira connection |
| `s` | Save and exit |
| `q` | Quit without saving |
