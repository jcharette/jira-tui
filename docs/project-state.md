# Project State

Last updated: 2026-06-22

## Goal

Build a terminal-first Jira client for people who hate Jira's web interface.

The app should make common Jira workflows fast from the command line, starting with issue browsing
and eventually expanding into issue details, comments, transitions, assignment, and creation.
The intended scope includes normal Jira user workflows such as editing tickets, creating tickets,
subtasks, epics, sprints, boards, comments, and transitions. Jira administration is out of scope.
The feature roadmap and dependency-aware milestones live in [roadmap.md](roadmap.md).

The Read/View, Creation/Editing, and first Git-backed developer workflow buckets are complete as of
2026-06-19. Current product work is focused on security/auth hardening and broader AI workflow
support.

## Current Stack

- Language: Go
- TUI framework: Bubble Tea v2 (`charm.land/bubbletea/v2`)
- TUI styling: Lip Gloss (`github.com/charmbracelet/lipgloss`)
- Jira API: Jira Cloud REST API v3 and Jira Agile REST API
- Jira library: `go-atlassian` behind `internal/jira`
- Concurrency: typed dispatcher and bounded worker pool powered by `ants`
- Authentication: email + Jira API token using Basic Auth
- Secret storage: saved Jira API tokens live in the OS keychain through `zalando/go-keyring`
- Configuration: TOML config file for non-secret settings and keyring references

## Current Commands

Install a tagged release with Go:

```bash
go install github.com/jcharette/jira-tui/cmd/jira@v1.0.10
```

Release archives are published at
[GitHub Releases](https://github.com/jcharette/jira-tui/releases) with names such as
`jira-tui_1.0.10_darwin_arm64.tar.gz` and include a `jira` binary.

Run from the project root:

```bash
make run
```

Open the config editor:

```bash
jira config
```

Start work on a ticket:

```bash
jira start ABC-123
jira start
```

Run with a saved profile:

```bash
jira --profile work
jira --profile work config
jira --profile work start ABC-123
```

Build a local binary:

```bash
make build-local
```

Run tests:

```bash
make test
```

When Claude is enabled and the Branch Plan feature is on, `jira commit` can draft the Jira progress
note with Claude and still shows the generated note in the normal confirmation prompt. If Claude is
unavailable, disabled, times out, or returns an empty note, the command uses the deterministic note.

Run the standard verification loop:

```bash
make check
```

Current TUI navigation and rendering regression coverage includes issue-table paging and
first/last jumps, selected-window rendering, local active filtering, subtree collapse projection,
collapsed parent row affordances, issue-type glyph scanning, detail-mode section navigation and
scroll memory, context-specific footer hints, focused Links / Hierarchy / Actions behavior, help
overlay rendering, and minimum-terminal warnings. New TUI surfaces should keep adding focused
regression tests in the same workflow test files.

Sprint-oriented saved views now load Jira Agile metadata in the background through the worker pool:
boards are discovered for the configured default project, then active/future sprints are loaded for
returned boards through a bounded scheduler with at most two active sprint reads. This does not
alter issue JQL; the header shows compact sprint loading, error, or aggregate count state, and
Diagnostics records the Agile read family. Sprint ticket actions reuse this Agile metadata and can
add the selected issue to an active or future sprint through Jira Agile APIs. Set
`queries.default_board_id` to scope Sprint Actions when a project has multiple boards.

Diagnostics also mirrors sanitized background activity to a bounded persistent JSONL log in the OS
user cache directory, for example `~/Library/Caches/jira-tui/diagnostics.jsonl` on macOS. The
Diagnostics overlay shows the active log path. The log is intended for bug reports and records
worker/API/cache/state breadcrumbs without raw JQL, tokens, request/response bodies, comments, or
descriptions. The persistent log and bug-report excerpt apply final key-value redaction for
token/password/secret-style fields before writing or exporting Diagnostics text.

Pressing `B` opens an in-app bug report composer. The user provides a short title and description,
can opt into the bounded sanitized Diagnostics excerpt, and `jira-tui` opens a prefilled GitHub issue
URL in the browser. The app does not store GitHub credentials or upload raw local log files.

## Current Configuration

Config is stored at `~/.config/jira/config.toml` by default. The application creates the
directory with `0700` permissions and the config file with `0600` permissions. Saved Jira API tokens
are stored in the OS keychain; existing plaintext token configs still load and migrate on the next
config save.

The active config shape is:

```toml
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://example.atlassian.net"
email = "person@example.com"
api_token = ""
api_token_source = "keyring"

[profiles.work]
base_url = "https://work.atlassian.net"
email = "person@work.example"
api_token = ""
api_token_source = "keyring"

[queries]
default_project = "ABC"
default_board_id = 0
default_jql = "project = ABC AND assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC"

[views]
active = "Assigned"

[[views.saved]]
name = "Assigned"
jql = "project = ABC AND assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC"

[[views.saved]]
name = "Created/Reported"
jql = "project = ABC AND (creator = currentUser() OR reporter = currentUser()) AND resolution = Unresolved ORDER BY updated DESC"

[[views.saved]]
name = "Project Open"
jql = "project = ABC AND resolution = Unresolved ORDER BY updated DESC"

[[views.saved]]
name = "Current Sprint"
jql = "project = ABC AND sprint in openSprints() AND resolution = Unresolved ORDER BY assignee ASC, status ASC, priority DESC"

[[views.saved]]
name = "Watching"
jql = "project = ABC AND watcher = currentUser() AND resolution = Unresolved ORDER BY updated DESC"

[[views.saved]]
name = "Epics"
jql = "project = ABC AND issuetype = Epic AND resolution = Unresolved ORDER BY updated DESC"
include_children = true

[appearance]
theme = "default"
primary = "#7DD3FC"
secondary = "#A78BFA"
accent = "#F59E0B"
success = "#34D399"
warning = "#FBBF24"
error = "#F87171"
muted = "#6B7280"
border = "#374151"
surface = "#111827"
text = "#E5E7EB"

[display]
symbol_mode = "auto"

[runtime]
refresh_interval = "2m0s"
request_timeout = "20s"
workers = 2
queue_size = 16

[notifications]
enabled = true
system_enabled = false
system_on_new = true
system_on_updates = false
auto_open_panel = true
keep_panel_open_until_cleared = true
max_items = 50

[git]
branch_template = "{key}-{summary_slug}"

[claude]
enabled = false
command = ""
timeout = "2m0s"

[claude.features]
ticket_plan = false
ticket_assist = false
clarifying_questions = false
draft_comment = false
draft_ticket = false
branch_plan = false
code_changes = false
pr_creation = false
pr_review_response = false

[claude.gates]
require_confirmation = true
allow_jira_writes = false
allow_git_writes = false
allow_github_writes = false
allow_code_edits = false
```

If required config is missing or invalid at startup, the app launches the config editor before
starting the issue list. Runtime settings are updated through `jira config`, not command-line flags
or environment variables. Claude is configured in the same config editor. It defaults disabled; an
empty Claude command means auto-detect the local `claude` CLI with `exec.LookPath`, while a manual
path such as `/opt/homebrew/bin/claude` is honored as an override. This Claude Code path uses the
user's local CLI/session and does not require an Anthropic API key. Scalar config fields use Bubbles
`textinput` for cursor-aware editing, while boolean config fields render as true/false picker
controls instead of requiring users to type raw boolean strings.
Notifications are configured in the same editor. In-app notifications default enabled, auto-open the
notification center, and keep the center open until notifications are cleared; optional system
notifications use `github.com/gen2brain/beeep` and default to opt-in.

Default JQL:

```text
project = ABC AND assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC
```

## Current Behavior

- Loads up to 50 Jira issues matching the active saved view JQL.
- Refreshes issues in the background on a configurable interval.
- Routes Jira refresh work through a typed dispatcher and bounded worker pool powered by `ants`.
- The TUI is the only frontend and must remain responsive. Jira IO, cache refresh, TTL expiry
  handling, prefetching, and background sync work should run off the Bubble Tea update/render loop
  through bounded workers or well-maintained libraries. More background workers/threads are
  acceptable when they reduce UI blocking or repeated Jira calls.
- The Jira base URL must use `https://`; the app rejects plaintext `http://` base URLs before
  configuring Basic Auth.
- The SQLite app cache lives under the OS user cache directory and is created with owner-only file
  permissions because it stores Jira issue rows, comments, descriptions, metadata, and query
  history.
- Pressing `ctrl+d` opens a read-only Diagnostics overlay with recent background worker and
  detail-cache activity from a bounded in-memory buffer. The overlay includes summary counts,
  simple activity bars, active worker request counts, and labeled event rows for visibility and
  troubleshooting only; it does not make Jira calls or block the TUI. The same sanitized event stream
  is mirrored to the persistent Diagnostics log when logging is available. Create
  metadata results include sanitized result counts such as issue type and field counts, supported
  create-field counts, unsupported required counts, and short field ID/name/schema samples so empty
  or filtered Jira metadata responses can be diagnosed without logging raw response bodies or
  credentials. Worker submissions/results also produce bounded sanitized API debug rows with Jira
  operation family, request ID, issue/project scope, result class, safe counts, elapsed time, and
  categorized errors without recording raw request bodies, response bodies, tokens, or full JQL
  strings. Persistent Diagnostics and bug-report excerpts also redact sensitive key-value fields
  before writing or exporting.
- Uses request IDs to ignore stale Jira responses.
- Records new and updated ticket events into a persistent in-app notification queue. The main panel
  shows a compact notification alert, `ctrl+n` opens the notification center, `enter` opens the
  selected ticket, uncleared notifications remain visible until cleared, and focused ticket detail
  shows ticket-scoped notifications for the selected issue. Optional system notifications use
  `beeep`.
- Preserves the selected issue across refreshes when it still exists in the refreshed list.
- Bounds Jira calls with a configurable request timeout.
- Displays issues in a styled Bubble Tea alternate-screen TUI with compact control chips for saved
  view, local All/Active filter, layout, sort, issue count, freshness, and sprint metadata. The
  default layout is `Lanes`; `L` cycles local presentation through `Table`, `Workbench`, and `Lanes`
  without changing Jira queries, saved views, caches, or loaded
  issues. `enter` opens the focused detail view.
- In the issue table and focused ticket detail, `n` opens the first create-ticket flow. The modal
  loads Jira issue types for the active project, loads create fields for the selected issue type,
  renders Summary and Description editors plus supported metadata-backed fields, and submits
  creation through the worker pool. Supported extra create fields include Priority, Labels,
  Components, simple string/text/number fields, and single-select option fields with Jira-provided
  allowed values. Option-backed create fields support typeahead filtering while focused, and
  picker fields with metadata `autoCompleteUrl` can fetch Jira options through the worker pool as
  users type. Optional picker fields start unselected so Jira does not receive accidental
  first-option values.
  Metadata-owned Project and Issue Type required fields are satisfied by the selected project/type;
  unsupported required Jira fields are surfaced as blocking warnings before submit.
  If Jira returns zero creatable issue types, the modal reports that empty metadata response and
  keeps `ctrl+d` diagnostics available. Issue type and field discovery use go-atlassian's paged
  create metadata mapping endpoints first, then fall back to the expanded create metadata endpoint
  when the preferred endpoint succeeds with zero values. Long Jira option lists in create fields are
  rendered in bounded picker windows so the modal stays inside the visible terminal.
- From focused ticket detail, the Ticket Actions palette can create a subtask for the selected
  issue. This reuses the same create modal, filters the issue type picker to Jira metadata entries
  marked as subtasks, keeps required-field validation in the create form, and submits the selected
  issue key as the Jira parent only when the user explicitly creates the ticket.
- `jira start [ticket]` and focused ticket detail Ticket Actions -> Start Work share the same
  Bubble Tea workflow for selecting a ticket, choosing a local repo, editing the branch name, and
  reviewing writes before anything changes. The Git branch action runs through the single
  `internal/gitworkflow` adapter boundary. Confirmed Jira follow-ups assign to the current user,
  choose the best available In Progress-like transition when no required unsupported fields are
  present, and add a compact branch comment only after branch creation/switch succeeds.
- `jira commit [ticket]` analyzes the current Git repo through `internal/gitworkflow`, detects the
  Jira issue from the branch when possible, separates dirty work from already-reported and
  unreported local commits, previews the commit/Jira/push writes, stores reported commit SHAs under
  the OS cache directory, and skips duplicate Jira progress notes on later retries.
- `jira finish [ticket]` reuses the same commit/report state, pushes the branch, creates or reuses a
  GitHub draft pull request through `internal/prprovider`, posts a compact final Jira note with the
  PR URL, and applies only the best available terminal Jira transition when Jira metadata says no
  required extra fields are needed.
- Startup runs a local Claude Code/CLI preflight from config. Disabled Claude records a disabled
  status; enabled Claude resolves `claude` from PATH unless a command/path override is configured,
  runs a bounded `--version` check, and records ready/unavailable status in Diagnostics. When
  Claude is enabled, available, and the `ticket_plan` feature flag is true, focused ticket detail
  shows a Claude section that sends selected ticket context to the local Claude CLI in the
  background and renders the returned read-only implementation/verification plan in a modal. While
  Claude is running, the modal shows concise subprocess activity, elapsed progress, stable output
  state, and an `esc` cancel path that cancels the in-flight local CLI
  process. Timeout errors retain start/deadline/command evidence for troubleshooting. While
  running, the app requests Claude stream-json partial output, parses nested stream events, records
  progress in Diagnostics, and suppresses raw JSON/protocol noise from the loading modal. Loading
  modals show stable states such as waiting, receiving response, or receiving CLI messages instead
  of constantly updating partial assistant text. True delta chunks are assembled into final readable
  output while repeated cumulative partials are deduped. Auth failures or partial model output are
  still available through Diagnostics and final error/result states. Claude plan dialogs use a
  responsive percentage of the available detail
  width for readability. Final Claude plans are bounded inside the visible detail panel, render
  Markdown pipe tables through the existing fitted table renderer, scroll with `j`/`k`, arrows,
  page keys, and `g`/`G`, and show a Claude line-range hint. Future write-capable workflows must
  still check feature flags and write gates before showing actions, before enqueueing work, and
  before applying generated changes. A planned local workspace mapping feature should let a ticket
  map to one or more local repository folders so future Claude/code workflows can run with the right
  repo context.
- When Claude is enabled, available, and the `ticket_assist` feature flag is true, focused ticket
  detail also exposes a Ticket Assist action in the Claude section. Ticket Assist sends the current
  ticket context plus any loaded child/subtask hierarchy to the local Claude CLI in read-only mode
  and asks for review findings, a structured rewrite draft with Summary, Problem / Goal, Acceptance
  Criteria, Test / Verification, Implementation Notes, and Open Questions, plus first-class Subtask
  Recommendations for keeping, adding, removing, or rescoping child work. Returned drafts open in an
  editable local modal using a textarea editor; the draft receives the majority of modal space and
  is rendered as a distinct editable block from the bounded review preview. Parsed Open Questions
  appear below the draft; `enter` opens a local answer editor, `j`/`k` selects questions, and
  `ctrl+r` refines the current draft with saved answers. `pgup`/`pgdn` page long drafts and
  `ctrl+y` copies the edited draft. Printable letters always edit the focused local draft or answer
  editor; modal actions use modifier shortcuts so ordinary text entry cannot open another workflow.
  When `allow_jira_writes` is disabled, `ctrl+s` keeps the draft local and explains that writes are
  gated. When `allow_jira_writes` and confirmation are enabled, `ctrl+s` opens an apply confirmation
  and a second `ctrl+s` updates Jira Summary and Description through the worker pool. Pressing
  `ctrl+r` without parsed questions opens a refinement instruction editor; submitting it sends
  Claude the original ticket context, the current user-edited draft, and the user's instruction,
  then replaces the editable draft with the refined result while keeping Jira writes gated. Pressing
  `ctrl+c` opens a confirmation to post the current draft as a Jira comment without editing Summary
  or Description, useful for tickets owned by someone else. Ticket Assist result modals render
  distinct `Claude Review`, `Local Draft`, Open Questions, and `Available Actions` zones so
  generated review, local edits, clarifications, and next actions are easier to separate. Acceptance
  Criteria are treated as a first-class draft section and are written into Description for now.
  Ticket Assist draft and Open Question answer editors support simple local selection: terminals
  that report Shift+Arrow can extend a selection directly, and `ctrl+space` starts the same
  selection mode everywhere; arrows extend, `ctrl+y` copies, `delete`/`backspace` removes, and `esc`
  clears the selection. When applying a whole-ticket Ticket Assist draft, Summary and Description
  are updated through the worker pool first, then parsed Subtask Recommendations open a Review
  Subtask Changes modal for epic child management. The review modal supports keep, add, modify, and
  close recommendations one at a time: Add opens the existing metadata-backed subtask create flow
  with the recommendation prefilled, Modify/Rescope posts a review comment to the child ticket, and
  Remove/Defer attempts a close-as-invalid style transition only when Jira exposes a matching
  no-extra-fields transition. If the workflow requires fields or has no safe matching transition,
  the app posts a child-ticket comment instead of forcing a state change. When Description or Overview
  is the focused ticket-detail section and Claude Ticket Assist is enabled and available, `a` opens
  the `Ticket Assist` picker instead of jumping to the Claude tab. The first picker action starts the
  same whole-ticket guided Ticket Assist session; the remaining actions can extract acceptance
  criteria, answer a user question, or draft a clarifying comment. Description-scoped results still
  reuse the Ticket Assist modal, but Description-scoped apply writes only Description through the
  worker-backed Jira update path.
  Future inline AI actions for Comments should reuse this same draft/refine/apply/comment machinery
  without disrupting the existing comment composer. Pressing `a` elsewhere in ticket detail jumps
  to the Claude/AI section when any Claude action is available; otherwise it keeps the old
  add-comment fallback.
- The ticket Overview renders the full Description by default rather than a fixed preview. Long
  descriptions use the existing detail viewport and scroll indicator, so large terminals fill with
  ticket content and smaller panes can still page through the body with normal detail scrolling.
- The issue table is a compact tree-backed list with a fixed-width hierarchy gutter, icon-only
  issue type column, key, summary, status, priority, shortened assignee, and selected-row metadata.
  Returned child issues are rendered under their parent when both are present in the result set,
  while child and subtask rows keep their own status, priority, and owner values for scanability.
  Narrow terminals use explicit column breakpoints: owner is hidden below 96 columns, key/status
  widths compact below 90 columns, priority is hidden on very narrow issue lists, and deep
  hierarchy indentation is capped more aggressively. Supported terminal heights preserve at least
  eight useful issue rows.
- Issue search results are normalized in the worker pool by fetching missing parent issues for any
  returned children, so the visible list can stack children under real parent rows even when the
  saved JQL did not include the parent.
- Saved issue views can opt into automatic child loading with `include_children = true`. The default
  Epics view enables this so epic rows load direct child issues without changing the saved-view JQL;
  ordinary views leave it off to avoid extra child-search Jira reads. Active-view result caches are
  scoped by both normalized JQL and the child-loading mode so same-JQL views do not share incompatible
  result sets.
- When Jira includes subtask issue data on a visible parent, the worker adds those subtasks to the
  issue set so they render as nested rows instead of hidden-count metadata.
- Issue tables support explicit parent expansion without changing the active saved view: `x` loads
  open child issues for the selected parent and `X` loads all child issues, including resolved/done
  children. Expansion runs through the worker pool and merges new child rows into the current list.
- Issue tables support a local `f` Active filter that hides loaded tickets whose status text looks
  terminal (`done`, `closed`, `resolved`, `canceled`, or `cancelled`) without changing Jira reads,
  saved views, cache records, or loaded issue data.
- Issue tables support local layout modes with `L`. `Table` remains the dense scan view,
  `Workbench` adds a responsive selected-ticket context panel on wide terminals using selected-row
  context, latest loaded comments, hierarchy, and loaded description preview without exposing cache
  internals. Entering Workbench or moving selection inside Workbench starts a selected-issue comment
  load through the existing worker path. `Lanes` is the default layout, groups the current scoped
  visible issue set by status, shows the selected-row cursor, and places in-progress work before
  to-do work. Layout switching preserves selection, local filters, sort mode, active view, and Jira
  query state. Footer/help context labels follow the active presentation, so Lanes renders as
  `Issue Lanes`, Workbench as `Issue Workbench`, and Table as `Issue Table`.
- Issue tables support local subtree collapse with `z`: the selected node can hide or reveal its
  loaded descendants without changing Jira reads, saved views, cache records, issue ordering, or the
  loaded issue data. Collapsed rows show a compact hidden-descendant count, and navigation skips
  hidden rows until the ancestor is expanded.
- Issue tables support an in-app query modal with `/`. Users can enter direct raw JQL and run it
  with `ctrl+s`, or switch to AI-assisted JQL generation, preview the generated query, revise the
  prompt, and explicitly confirm before the generated JQL changes the active query or triggers Jira
  reads.
- Confirmed direct and AI-generated query runs are stored as recent query history in the SQLite app
  cache, scoped by active Jira cache namespace. The query modal has a `Recent` mode where users can
  select prior queries, copy one back into direct JQL for review, or run the selected recent query.
- Saved views can be created without editing config: `v` from the issue table saves the current JQL,
  `ctrl+v` from direct JQL or AI preview opens the same save prompt, and list modes can save selected
  recent queries or project-scoped templates. The prompt can toggle `include_children`; saving writes
  through the existing config file, appears in saved-view rotation immediately, and does not run Jira
  or change the active query.
- The query modal includes a `Views` mode for saved-view maintenance. Users can rename, reorder,
  delete, toggle `include_children`, or copy a saved view's JQL back into the direct editor for
  review without triggering Jira reads.
- Fetches read-only issue details for the selected issue through the worker pool, caches details by
  issue key, and ignores stale detail responses when selection changes. Issue detail uses
  short-lived TTL freshness tracking via `github.com/jellydator/ttlcache/v3`: fresh cached detail
  avoids duplicate Jira reads, while stale cached detail remains visible and refreshes in the
  background.
- Issue detail includes description text, reporter, creator, labels, components, fix versions,
  created date, and updated date when Jira returns those fields.
- Focused ticket detail loads the latest Jira comments for the selected issue through the worker
  pool and renders them with the same ADF-aware terminal formatting as descriptions.
- Focused ticket detail can add a Jira comment from the Comments section by pressing `enter`, or
  from the Ticket Actions palette. The composer uses `ctrl+s` to review, `y` to post, `n` to return
  to editing, and `esc` to cancel. Successful posts refresh the issue comments.
- Focused ticket detail can edit loaded comments from the Comments section. Pressing `enter` on a
  loaded Comments section focuses the comment list, `j`/`k` selects a comment, `e` opens the
  selected comment in the same editor surface, and the review flow updates Jira explicitly with
  `y`. Successful updates invalidate retained comment cache entries and refresh comments.
- Focused ticket detail can edit Summary directly from the header field: `s` focuses Summary,
  `enter` loads Jira edit metadata through the worker pool, and a focused edit dialog owns the
  summary draft, validation notice, save, and cancel controls. Successful updates refresh the
  visible issue row and cached detail summary immediately.
- Focused ticket detail exposes Status as a focusable control in the detail control strip. Selecting
  Status and pressing `enter` loads available Jira transitions through the worker pool, then opens
  the shared detail dialog pattern with Jira-populated transition choices. The Ticket Actions
  palette routes to the same transition flow. Successful transitions update the visible issue and
  cached detail status immediately.
- Focused ticket detail can edit Priority directly with `p`. The flow loads Jira edit metadata
  through the worker pool, opens a picker modal populated from Jira `allowedValues`, and submits the
  selected priority through the worker pool. Successful updates refresh the visible issue row and
  cached detail priority immediately.
- Focused ticket detail can edit Assignee directly from the detail control strip. Selecting Assignee and
  pressing `enter` opens a `Change Assignee` modal; typing filters Jira assignable users for the
  selected issue through the worker pool, arrow keys select a result, and `enter` assigns by Jira
  account ID. The Assignee modal owns its footer/help context, and the Ticket Actions palette routes
  to the same picker. Successful updates refresh the visible issue row and cached detail assignee
  immediately.
- Focused ticket detail can edit Labels through the Ticket Actions palette.
  Jira edit metadata confirms the `labels` field is editable, then a bounded comma-separated editor
  submits through the worker pool. Successful updates patch loaded issue detail and retained detail
  cache labels immediately.
- Focused ticket detail can edit Components through the Ticket Actions palette. Jira edit metadata
  confirms the `components` field is editable and provides `allowedValues`, then a bounded
  searchable multi-select picker submits selected components through the worker pool. Successful
  updates patch loaded issue detail and retained detail cache components immediately.
- Focused ticket detail can edit safe generic Jira custom fields through the Ticket Actions palette.
  The generic editor is enabled only for metadata-backed custom string/text/number/date/datetime
  fields, single-select option fields, multi-select option arrays with inline Jira `allowedValues`,
  and the supported standard fields Fix Versions, Affects Versions, and Due Date. Parent and Time
  Tracking have dedicated metadata-backed dialogs for parent key updates and original/remaining
  estimate edits. User, sprint, autocomplete-only, other standard, and unknown field schemas stay
  visible but disabled until a field-specific workflow exists.
- Jira field option lookup is available through the client and worker pool for metadata-provided
  `autoCompleteUrl` values. The create-ticket form uses it for option-backed picker fields; generic
  edit-field autocomplete remains gated until that option source has a safe selected-value submit
  path.
- Jira edit metadata now retains the full editable field catalog in addition to the supported
  Summary, Priority, Labels, and Components helpers. Retained edit fields include schema type,
  schema system/items/custom data, operations, default markers, inline allowed values, and
  autocomplete URLs. Ticket Actions enables safe generic custom fields plus supported standard
  version/date, parent, and estimate fields, and keeps unsupported editable fields as disabled,
  searchable rows with an `Unsupported` state and option-source context.
- Focused ticket detail treats Overview, Summary, Status, Assignee, and Priority as first-class
  focus targets before the remaining content destinations. `tab` and `shift+tab` move through
  focusable controls and sections, while `enter` opens the contextual edit modal, picker, or
  transition flow for the focused target. Single-letter shortcuts such as `s` and `p` remain
  accelerators, not the primary editing model.
- Key bindings should prefer one clear semantic path per workflow. Conventional navigation aliases
  are acceptable when they mean the same thing, but unrelated keys should not duplicate the same
  action; `tab` owns focus movement, `enter` acts on focus, and single-letter keys are distinct
  accelerators. The 2026-06-18 active-context audit removed the hidden detail `b` browser-open path
  and the no-AI detail `a` comment fallback: `o` is the ticket browser shortcut, Comments/`enter`
  opens comment composition, and `a` remains AI-only when AI is available.
- Ticket detail mutations should use focused modal dialogs as the default pattern: the selected
  field or section provides context, the dialog renders Jira-derived data and validation state, and
  submit/cancel controls stay keyboard-owned without relying on a generic Actions menu.
- Text-backed mutation dialogs, such as Summary and future comment/edit fields, should use a real
  editor surface in the modal. Enumerated mutation dialogs, such as Status and future Priority,
  should use a picker/dropdown-style list with arrow or `j`/`k` selection and a single apply action.
- The typed Jira client loads transition-screen field metadata with
  `expand=transitions.fields` and applies supported transition field values through the
  worker-backed status update path. Supported transition fields are Resolution, transition Comment,
  custom single-select option fields, text/string fields, date/datetime fields, user pickers,
  multi-select option/user arrays, and autocomplete-backed picker fields. Unsupported required
  fields block submission with a clear notice instead of sending a doomed Jira request.
- Comment composition and comment editing use a bounded multi-line editor surface with independent
  draft pagination; future rich comment work should build on that editor instead of reverting to
  single-line input.
- Comment composition shares the ticket detail section-header and notice-block grammar, so add,
  review, posting, detected-link, unresolved-mention, mention-picker, and validation feedback
  states read like the rest of focused detail.
- Comment composition detects embedded URLs, bare domains, `mailto:` links, and email addresses
  with `mvdan.cc/xurls/v2` and previews recognized links before posting.
- Comment composition includes keyboard formatting controls for bold, italic, inline code, and
  bullet markers. Comment submission converts visible `**bold**`, `_italic_`, and `` `code` ``
  tokens into Jira ADF marks, converts detected URLs and email addresses into Jira ADF link marks,
  and preserves the existing paragraph and hard-break behavior.
- Jira user search is available through the client and worker pool for comment mentions.
- Jira assignable-user search is available through the client and worker pool for issue-scoped
  assignee editing.
- Typing `@` in the comment composer opens a bounded Jira user-search picker. Selected users are
  inserted as visible `@Name` text and submitted as Jira ADF `mention` nodes with Atlassian account
  IDs. Mention results render through the shared Bubbles list-backed choice-list adapter. Raw typed
  `@mention` text is still previewed as unresolved until selected through the picker.
- TUI keyboard bindings are centralized by active context, and `?` opens a keyboard help screen for
  the current mode. Local key metadata adapts to Bubbles `key.Binding`, and footer help renders
  through Bubbles `help` while preserving the app's grouped task grammar. Footer help is kept to the
  high-value commands for the active screen. The footer starts with the active key context so users
  can tell whether they are in the issue table, ticket detail, hierarchy, links, actions, comment
  composition, or another focused mode.
- The keyboard help panel keeps its title and active context visible while the binding list scrolls,
  so paging help does not hide which mode is being documented.
- Empty and error state panels describe state only; recovery commands such as refresh stay in the
  grouped footer/help grammar instead of appearing as inline panel instructions.
- Bounded overlay/panel renderers account for shared app chrome, including header, query, blank
  separators, footer, panel borders, and panel padding, so future panels can reuse one height budget
  when the header changes.
- Ticket detail has its own scroll behavior in focused detail mode: `j`/`k`, `pgup`/`pgdn`, and
  `g`/`G` scroll the detail content instead of changing selected tickets. Focused detail scrolling
  is backed by Bubbles `viewport`; keep future detail scrolling work on that component. Each
  focused detail section remembers its own scroll offset while viewing the same issue, so switching
  between long descriptions, links, hierarchy, comments, and actions restores the last position for
  that section.
- Focused ticket detail keeps key ticket identity and title-style summary text in the panel header,
  then renders a compact control strip for Status, Priority, Assignee, and secondary metadata.
  Tickets open on Overview, which summarizes latest loaded activity, a description preview, hierarchy
  context, and the Ticket Actions palette affordance. Primary navigation is reserved for content
  destinations: Overview, Comments, Hierarchy, dynamic Links, and Claude when enabled. Section tabs
  and headers expose useful context badges such as child issue counts, discovered links, and comment
  count/loading/error state. The focused section is marked with a plain `>` tab marker and footer
  context label, while section headers remain content headings. Section scroll positions are driven
  by the same reusable section descriptors. Moving section focus with `tab`/`shift+tab` changes the
  selected section or control; `enter` activates the focused target. The body renders only the
  selected section, while inactive section names and badges stay in the tab bar.
- The ticket detail panel header separates issue identity, summary, focusable controls, and
  content-only tabs so navigation no longer competes with the ticket title.
- The always-visible ticket detail footer is intentionally limited to primary actions (`esc`,
  `j`/`k`, `tab`, `.`, `a`, and `o`); secondary section jumps and copy actions remain in `? help`.
  When a field control such as Status, Priority, Assignee, or Summary is focused, the footer names
  the matching `enter` action. When an interactive content section is selected but not yet
  activated, the footer swaps in the visible section commands, such as child or link selection and
  activation, and those keys operate on the visible section immediately.
- Long ticket detail views right-align their viewport line indicator at the bottom of the panel so
  pagination state reads as panel chrome instead of another content row. The indicator includes
  the active detail section name on the left and the line range on the right.
- The Links section uses a Lip Gloss table for Jira issue links and detected description links.
  Jira issue-link rows come from issue detail `issuelinks`, show the linked issue key,
  relationship, status, and summary, open the linked issue URL with `enter`/`o`, and copy the issue
  key with `y`. Comments render as repeated left-ruled blocks with author/date-first headers,
  secondary comment counts, and a clear header/body gap. Description content also separates its
  ruled header from rich body content, while Description and Comments loading, empty, and error
  messages share a consistent Status block. The Hierarchy section always renders a Path block for
  either the current issue or known parent context, separates visible child issues from subtasks,
  keeps the selected row marked before activation, and routes `j`/`k` and arrow keys to that
  selected row while the Hierarchy tab is selected. When no child rows are loaded, Hierarchy uses
  root-vs-known-parent empty-state copy and leaves Jira issue links to the Links workspace. Detail
  tables share one renderer so future pane/table styling stays centralized.
- Focused ticket detail does not append a permanent hierarchy/URL footer under every section;
  hierarchy data lives in the Hierarchy section and issue URL workflows live in key bindings or the
  Ticket Actions palette.
- Focused ticket detail has a searchable Ticket Actions palette on `.`, with filterable
  Action/Type/Detail rows that dispatch through Add Comment, browser/copy actions, Summary editing,
  Priority editing, Labels editing, Components editing, status transitions, assignment, and
  parent-scoped subtask creation. Detail notices render as a distinct styled block instead of plain
  inline text.
- Focused ticket detail supports section navigation with `tab`/`shift+tab`, direct jumps to
  comments with `m`, hierarchy with `h`, and links with `l`.
- Focused ticket detail supports selected issue actions: `o` opens the Jira issue URL, `c` copies
  the issue key, and `y` copies the issue URL when Links is not focused.
- Jira ADF descriptions and comments are rendered through `internal/adf`. The renderer wraps a
  maintained ADF-to-Markdown converter for text-oriented blocks, normalizes the output for the TUI,
  and keeps the app's custom `[table]` marker path for Jira tables. Renderer fixtures include
  sanitized real-shaped Jira description and comment payloads under `internal/adf/testdata`, and the
  dev-only `cmd/adf-fixture` helper can sanitize raw ADF JSON or capture selected issue/comment ADF
  through the existing Jira config. Rich ticket text renders fenced code blocks as compact
  foreground/background-highlighted lines instead of ASCII bordered blocks.
- Focused ticket detail extracts embedded URLs, bare domains, `mailto:` links, and plain email
  addresses through `internal/linkdetect` into a Links section; `l` jumps to and focuses that
  section, `j`/`k` selects links, `o`/`enter` opens the selected link, and `y` copies it.
- Long issue tables are rendered through a Bubbles `viewport` with visible range, page up/down
  controls, and first/last shortcuts.
- Issue tables can be locally sorted by Jira order, priority, status, assignee, type, or key with
  `o`/`O`; parent-child grouping is preserved when possible.
- `enter` opens a focused ticket detail view; `esc` returns to the table.
- Uses saved appearance skins and color overrides for the config editor and issue list.
- Each appearance skin provides a matching default issue-list symbol style.
- Uses `display.symbol_mode` to override issue-list symbols. `auto` is the default and detects
  supported Nerd-capable terminal setups before falling back to colored terminal-safe glyphs, with
  explicit `plain`, `symbols`, `emoji`, and `nerd` overrides available in config. `nerd` is the
  recommended premium icon mode after users install and select a Nerd Font such as JetBrainsMono
  Nerd Font or MesloLG Nerd Font.
- Supports moving through the issue list with `j`/`k` or arrow keys.
- Supports switching saved views with `tab`/`]` and `shift+tab`/`[`.
- Supports paging with `pgdn`/`space`/`ctrl+f` and `pgup`/`ctrl+b`, plus `g`/`G` for first/last.
- Supports refresh with `r`.
- Supports quit with `q` or `ctrl+c`.
- Config editor sections move with `left`/`right`, `h`/`l`, or `tab`; fields move with `j`/`k`;
  editing uses `enter` to accept and `esc` to cancel.
- Config editor includes `Test Connection`, which runs a live Jira query with the current unsaved
  settings and reports targeted feedback for network/base URL, auth, permission, project/JQL, and
  timeout failures.
- Config editor chrome now follows the issue browser grammar with a width-aware header, explicit
  `Config` / `Config Edit` footer context, grouped footer commands, and truncation by whole command.
- Config loading preserves multiple TOML profiles and uses `active_profile` by default. The main app
  can temporarily select a saved profile with `--profile <name>`, and the config editor exposes the
  selected active profile as an editable field.

## Known Constraints

- Saved views are generated from the default project today for assigned, created/reported, project
  open, current sprint, watching, and epics; custom view editing in config is not exposed yet.
- Jira write workflows must be metadata-driven. Creation, editing, assignment, transitions, board
  and sprint movement, priorities, statuses, issue types, field options, required fields, users, and
  custom fields should come from Jira metadata for the active site/project/issue, not hard-coded
  assumptions.
- Config supports saved profiles and CLI profile selection, but the config editor only edits the
  selected active profile rather than providing a full multi-profile management UI.
- There is no OAuth flow; API token auth is the only supported auth mode.
- There is no git repository initialized in this folder yet.
- The Jira client uses `go-atlassian` for search, issue detail, comment reads, comment creation,
  issue creation including parent-scoped subtask creation, edit metadata, create metadata discovery,
  summary updates, priority updates, labels updates, and issue transitions, including transition-screen metadata for Resolution/Comment fields and ADF payload
  construction for text, formatting marks, links, and selected user mentions. Jira Agile board and
  sprint metadata is exposed through worker-backed background reads for sprint-oriented views.
- Jira user display strings prefer real display/name/email/key fields over generic privacy aliases
  such as `User e31ec`; ticket detail also preserves a better selected-list assignee if the detail
  response returns a generic alias.
- User-search results are cached in memory with a short TTL using `github.com/jellydator/ttlcache/v3`.
  Mention search uses global query keys, while assignee typeahead uses issue-scoped assignable-user
  cache keys so global search results cannot appear as assignment candidates. Broader cache work
  should keep reads async, use maintained library hooks where possible, and refresh important
  ticket/view data in the background instead of blocking the TUI.
- The worker pool currently supports issue search, explicit parent expansion, issue detail, comment
  reads, comment creation, comment editing, Jira user search, issue-scoped assignable-user search,
  field option autocomplete lookup, edit metadata, create issue type metadata, create field
  metadata, issue creation including parent-scoped subtasks, summary updates, priority, labels
  updates, assignee updates, and issue transitions with supported transition field values, board
  discovery, and active/future sprint metadata loading.
