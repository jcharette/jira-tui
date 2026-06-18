# Project State

Last updated: 2026-06-18

## Goal

Build a terminal-first Jira client for people who hate Jira's web interface.

The app should make common Jira workflows fast from the command line, starting with issue browsing
and eventually expanding into issue details, comments, transitions, assignment, and creation.
The intended scope includes normal Jira user workflows such as editing tickets, creating tickets,
subtasks, epics, sprints, boards, comments, and transitions. Jira administration is out of scope.
The feature roadmap and dependency-aware milestones live in [roadmap.md](roadmap.md).

## Current Stack

- Language: Go
- TUI framework: Bubble Tea v2 (`charm.land/bubbletea/v2`)
- TUI styling: Lip Gloss (`github.com/charmbracelet/lipgloss`)
- Jira API: Jira Cloud REST API v3
- Jira library: `go-atlassian` behind `internal/jira`
- Concurrency: typed dispatcher and bounded worker pool powered by `ants`
- Authentication: email + Jira API token from config using Basic Auth
- Configuration: TOML config file

## Current Commands

Install a tagged release with Go:

```bash
go install github.com/jcharette/jira-tui/cmd/jira@v0.2.1
```

Release archives are published at
[GitHub Releases](https://github.com/jcharette/jira-tui/releases) with names such as
`jira-tui_0.2.1_darwin_arm64.tar.gz` and include a `jira` binary.

Run from the project root:

```bash
make run
```

Open the config editor:

```bash
jira config
```

Build a local binary:

```bash
make build-local
```

Run tests:

```bash
make test
```

Run the standard verification loop:

```bash
make check
```

Current TUI navigation and rendering regression coverage includes issue-table paging and
first/last jumps, selected-window rendering, local active filtering, subtree collapse projection,
detail-mode section navigation and scroll memory, context-specific footer hints, focused Links /
Hierarchy / Actions behavior, help overlay rendering, and minimum-terminal warnings. New TUI
surfaces should keep adding focused regression tests in the same workflow test files.

## Current Configuration

Config is stored at `~/.config/jira/config.toml` by default. The application creates the
directory with `0700` permissions and the config file with `0600` permissions because the file stores
the Jira API token.

The active config shape is:

```toml
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://example.atlassian.net"
email = "person@example.com"
api_token = "secret"

[queries]
default_project = "ABC"
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

[appearance]
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
- Pressing `ctrl+d` opens a read-only Diagnostics overlay with recent background worker and
  detail-cache activity from a bounded in-memory buffer. The overlay includes summary counts,
  simple activity bars, active worker request counts, and labeled event rows for visibility and
  troubleshooting only; it does not make Jira calls, persist logs, or block the TUI. Create
  metadata results include sanitized result counts such as issue type and field counts, supported
  create-field counts, unsupported required counts, and short field ID/name/schema samples so empty
  or filtered Jira metadata responses can be diagnosed without logging raw response bodies or
  credentials. Worker submissions/results also produce bounded sanitized API debug rows with Jira
  operation family, request ID, issue/project scope, result class, safe counts, elapsed time, and
  categorized errors without recording raw request bodies, response bodies, tokens, or full JQL
  strings.
- Uses request IDs to ignore stale Jira responses.
- Preserves the selected issue across refreshes when it still exists in the refreshed list.
- Bounds Jira calls with a configurable request timeout.
- Displays issues in a styled Bubble Tea alternate-screen TUI with compact filter metadata and a
  full-width issue table. The redundant selected-issue side summary was removed; `enter` opens the
  focused detail view instead.
- In the issue table and focused ticket detail, `n` opens the first create-ticket flow. The modal
  loads Jira issue types for the active project, loads create fields for the selected issue type,
  renders Summary and Description editors plus supported metadata-backed fields, and submits
  creation through the worker pool. Supported extra create fields include Priority, Labels,
  Components, simple string/text/number fields, and single-select option fields with Jira-provided
  allowed values. Option-backed create fields support typeahead filtering while focused, and
  optional picker fields start unselected so Jira does not receive accidental first-option values.
  Metadata-owned Project and Issue Type required fields are satisfied by the selected project/type;
  unsupported required Jira fields are surfaced as blocking warnings before submit.
  If Jira returns zero creatable issue types, the modal reports that empty metadata response and
  keeps `ctrl+d` diagnostics available. Issue type and field discovery use go-atlassian's paged
  create metadata mapping endpoints first, then fall back to the expanded create metadata endpoint
  when the preferred endpoint succeeds with zero values. Long Jira option lists in create fields are
  rendered in bounded picker windows so the modal stays inside the visible terminal.
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
  ticket context to the local Claude CLI in read-only mode and asks for review findings plus a
  structured rewrite draft with Summary, Problem / Goal, Acceptance Criteria, Test / Verification,
  Implementation Notes, and Open Questions. Returned drafts open in an editable local modal using a
  textarea editor; the draft receives the majority of modal space and is rendered as a distinct
  editable block from the bounded review preview. `pgup`/`pgdn` page long drafts and `ctrl+y`
  copies the edited draft. When `allow_jira_writes` is disabled, `ctrl+s` keeps the draft local and
  explains that writes are gated. When `allow_jira_writes` and confirmation are enabled, `ctrl+s`
  opens an apply confirmation and a second `ctrl+s` updates Jira Summary and Description through the
  worker pool. Pressing `r` opens a refinement instruction editor; submitting it sends Claude the
  original ticket context, the current user-edited draft, and the user's instruction, then replaces
  the editable draft with the refined result while keeping Jira writes gated. Pressing `c` opens a
  confirmation to post the current draft as a Jira comment without editing Summary or Description,
  useful for tickets owned by someone else. Ticket Assist result modals render distinct `Claude
  Review`, `Local Draft`, and `Available Actions` zones so generated review, local edits, and next
  actions are easier to separate. Acceptance Criteria are treated as a first-class draft section and
  are written into Description for now. When Description is the focused ticket-detail section and
  Claude Ticket Assist is enabled and available, `a` opens `AI for Description` instead of jumping
  to the Claude tab. The picker can improve clarity, extract acceptance criteria, answer a user
  question, or draft a clarifying comment. Results reuse the Ticket Assist modal, but
  Description-scoped apply writes only Description through the worker-backed Jira update path.
  Future inline AI actions for Comments should reuse this same draft/refine/apply/comment machinery
  without disrupting the existing comment composer. Pressing `a` elsewhere in ticket detail jumps
  to the Claude/AI section when any Claude action is available; otherwise it keeps the old
  add-comment fallback.
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
- When Jira includes subtask issue data on a visible parent, the worker adds those subtasks to the
  issue set so they render as nested rows instead of hidden-count metadata.
- Issue tables support explicit parent expansion without changing the active saved view: `x` loads
  open child issues for the selected parent and `X` loads all child issues, including resolved/done
  children. Expansion runs through the worker pool and merges new child rows into the current list.
- Issue tables support a local `f` Active filter that hides loaded tickets whose status text looks
  terminal (`done`, `closed`, `resolved`, `canceled`, or `cancelled`) without changing Jira reads,
  saved views, cache records, or loaded issue data.
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
- In `Recent` mode, users can save the selected recent query as a named saved view. The view is
  written through the existing config file, appears in saved-view rotation immediately, and does not
  run Jira or change the active query while saving.
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
  from the Actions section. The composer uses `ctrl+s` to review, `y` to post, `n` to return to
  editing, and `esc` to cancel. Successful posts refresh the issue comments.
- Focused ticket detail can edit Summary directly from the header field: `s` focuses Summary,
  `enter` loads Jira edit metadata through the worker pool, and a focused edit dialog owns the
  summary draft, validation notice, save, and cancel controls. Successful updates refresh the
  visible issue row and cached detail summary immediately.
- Focused ticket detail includes a direct Status section. Selecting Status and pressing `enter`
  loads available Jira transitions through the worker pool, then opens the shared detail dialog
  pattern with Jira-populated transition choices. The Actions menu routes to the same transition
  flow. Successful transitions update the visible issue and cached detail status immediately.
- Focused ticket detail can edit Priority directly with `p`. The flow loads Jira edit metadata
  through the worker pool, opens a picker modal populated from Jira `allowedValues`, and submits the
  selected priority through the worker pool. Successful updates refresh the visible issue row and
  cached detail priority immediately.
- Focused ticket detail can edit Assignee directly from the header metadata. Selecting Assignee and
  pressing `enter` opens a `Change Assignee` modal; typing filters Jira users through the worker
  pool, arrow keys select a result, and `enter` assigns by Jira account ID. The Assignee modal owns
  its footer/help context, and the Actions menu routes to the same picker. Successful updates
  refresh the visible issue row and cached detail assignee immediately.
- Focused ticket detail treats Summary, Assignee, and Priority as first-class focus targets before
  the section tabs. `tab` and `shift+tab` move through editable fields and sections, while `enter`
  opens the contextual edit modal or picker for the focused target. Single-letter shortcuts such as
  `s` and `p` remain accelerators, not the primary editing model.
- Key bindings should prefer one clear semantic path per workflow. Conventional navigation aliases
  are acceptable when they mean the same thing, but unrelated keys should not duplicate the same
  action; `tab` owns focus movement, `enter` acts on focus, and single-letter keys are distinct
  accelerators.
- Ticket detail mutations should use focused modal dialogs as the default pattern: the selected
  field or section provides context, the dialog renders Jira-derived data and validation state, and
  submit/cancel controls stay keyboard-owned without relying on a generic Actions menu.
- Text-backed mutation dialogs, such as Summary and future comment/edit fields, should use a real
  editor surface in the modal. Enumerated mutation dialogs, such as Status and future Priority,
  should use a picker/dropdown-style list with arrow or `j`/`k` selection and a single apply action.
- The current typed Jira client supports listing and applying issue transitions, but does not
  expose Jira transition-screen field metadata. Future transition-field editing should add a
  lower-level Jira request for `expand=transitions.fields` instead of hard-coding presets.
- Comment composition uses a bounded multi-line editor surface with independent draft pagination;
  future rich comment work should build on that editor instead of reverting to single-line input.
- Comment composition shares the ticket detail section-header and notice-block grammar, so add,
  review, posting, detected-link, unresolved-mention, mention-picker, and validation feedback
  states read like the rest of focused detail.
- Comment composition detects embedded URLs, bare domains, `mailto:` links, and email addresses
  with `mvdan.cc/xurls/v2` and previews recognized links before posting.
- Comment submission converts detected URLs and email addresses into Jira ADF link marks while
  preserving the existing paragraph and hard-break behavior.
- Jira user search is available through the client and worker pool for comment mentions.
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
- Focused ticket detail keeps key ticket identity, title-style summary text, and compact metadata in
  the panel header, then
  renders Description, Links, Hierarchy, Comments, Actions, and Status as consistent workspace sections
  with shared ruled headers. Section tabs and headers expose useful
  context badges such as child issue counts, discovered links, and comment count/loading/error
  state. The focused section is marked with a plain `>` tab marker and footer context label, while section
  headers remain content headings. Section scroll positions are driven by the same reusable section descriptors. Moving section focus with
  `tab`/`shift+tab` changes the selected section; `enter` activates interactive sections. The
  body renders only the selected section, while inactive section names and badges stay in the tab
  bar.
- The ticket detail panel header separates issue identity (key, status, type), a standalone summary
  title, compact metadata, and plain-marker tabs so navigation no longer competes with the ticket
  title.
- The always-visible ticket detail footer is intentionally limited to primary actions (`esc`,
  `j`/`k`, `tab`, `a`, and `b`); secondary section jumps and copy actions remain in `? help`.
  When an interactive section is selected but not yet activated, the footer swaps in the visible
  section commands, such as child, link, or action selection and activation, and those keys operate
  on the visible section immediately.
- Long ticket detail views right-align their viewport line indicator at the bottom of the panel so
  pagination state reads as panel chrome instead of another content row. The indicator includes
  the active detail section name on the left and the line range on the right.
- The Links section uses a Lip Gloss table, and Comments render as repeated left-ruled blocks with
  author/date-first headers, secondary comment counts, and a clear header/body gap. Description
  content also separates its ruled header from rich body content, while Description and Comments
  loading, empty, and error messages share a consistent Status block. The Hierarchy section always
  renders a Path block for either the current issue or known parent context, separates visible child
  issues from subtasks, keeps the selected row marked before activation, routes `j`/`k` and arrow
  keys to that selected row while the Hierarchy tab is selected, and reserves a linked-issues
  placeholder until Jira linked issue data is loaded through a real API path. Detail tables share
  one renderer so future pane/table styling stays centralized.
- Focused ticket detail does not append a permanent hierarchy/URL footer under every section;
  hierarchy data lives in the Hierarchy section and issue URL workflows live in actions/key bindings.
- The Actions section uses a compact table with action state and detail columns. Detail notices
  render as a distinct styled block instead of plain inline text.
- Focused ticket detail supports section navigation with `tab`/`shift+tab`, direct jumps to
  description with `d`, comments with `m`, hierarchy with `h`, and links with `l`.
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
- Uses saved appearance colors for the config editor and issue list.
- Uses `display.symbol_mode` to control issue-list symbols. `auto` is the default and chooses a
  reliable terminal symbol set, with explicit `plain`, `symbols`, `emoji`, and `nerd` overrides
  available in config.
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

## Known Constraints

- Saved views are generated from the default project today for assigned, created/reported, project
  open, current sprint, watching, and epics; custom view editing in config is not exposed yet.
- Jira write workflows must be metadata-driven. Creation, editing, assignment, transitions, board
  and sprint movement, priorities, statuses, issue types, field options, required fields, users, and
  custom fields should come from Jira metadata for the active site/project/issue, not hard-coded
  assumptions.
- Config supports a single saved profile today, with a TOML shape designed for multiple profiles.
- There is no OAuth flow; API token auth is the only supported auth mode.
- There is no git repository initialized in this folder yet.
- The Jira client uses `go-atlassian` for search, issue detail, comment reads, comment creation,
  issue creation, edit metadata, create metadata discovery, summary updates, priority updates, and issue
  transitions, including ADF payload construction for text, links, and selected user mentions.
  Sprint and board APIs are not exposed through `internal/jira` yet.
- Jira user display strings prefer real display/name/email/key fields over generic privacy aliases
  such as `User e31ec`; ticket detail also preserves a better selected-list assignee if the detail
  response returns a generic alias.
- User-search results are cached in memory with a short TTL using `github.com/jellydator/ttlcache/v3`.
  The first uses are Assignee typeahead and issue-detail freshness. Broader cache work should keep
  reads async, use maintained library hooks where possible, and refresh important ticket/view data
  in the background instead of blocking the TUI.
- The worker pool currently supports issue search, explicit parent expansion, issue detail, comment
  reads, comment creation, Jira user search, edit metadata, create issue type metadata, create field
  metadata, issue creation, summary updates, priority updates, assignee updates, and issue
  transitions; sprint data is the next background workflow family to add.
