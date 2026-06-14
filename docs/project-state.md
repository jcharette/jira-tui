# Project State

Last updated: 2026-06-13

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
```

If required config is missing or invalid at startup, the app launches the config editor before
starting the issue list. Runtime settings are updated through `jira config`, not command-line flags
or environment variables.

Default JQL:

```text
project = ABC AND assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC
```

## Current Behavior

- Loads up to 50 Jira issues matching the active saved view JQL.
- Refreshes issues in the background on a configurable interval.
- Routes Jira refresh work through a typed dispatcher and bounded worker pool powered by `ants`.
- Uses request IDs to ignore stale Jira responses.
- Preserves the selected issue across refreshes when it still exists in the refreshed list.
- Bounds Jira calls with a configurable request timeout.
- Displays issues in a styled Bubble Tea alternate-screen TUI with compact filter metadata and a
  full-width issue table. The redundant selected-issue side summary was removed; `enter` opens the
  focused detail view instead.
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
- Fetches read-only issue details for the selected issue through the worker pool, caches details by
  issue key, and ignores stale detail responses when selection changes.
- Issue detail includes description text, reporter, creator, labels, components, fix versions,
  created date, and updated date when Jira returns those fields.
- Focused ticket detail loads the latest Jira comments for the selected issue through the worker
  pool and renders them with the same ADF-aware terminal formatting as descriptions.
- Focused ticket detail can add a Jira comment with `a`; the composer uses `ctrl+s` to
  review, `y` to post, `n` to return to editing, and `esc` to cancel. Successful posts refresh the
  issue comments.
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
  IDs. Raw typed `@mention` text is still previewed as unresolved until selected through the picker.
- TUI keyboard bindings are centralized by active context, and `?` opens a keyboard help screen for
  the current mode. Footer help is rendered from the same keymap source, grouped by task, and kept
  to the high-value commands for the active screen. The footer starts with the active key context
  so users can tell whether they are in the issue table, ticket detail, hierarchy, links, actions,
  comment composition, or another focused mode.
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
- Focused ticket detail keeps key ticket identity and compact metadata in the panel header, then
  renders Description, Links, Hierarchy, Comments, and Actions as consistent workspace sections
  with shared ruled headers. Section tabs and headers expose useful
  context badges such as child issue counts, discovered links, and comment count/loading/error
  state. The focused section is marked in the tab bar and footer context label, while section
  headers remain content headings. Section scroll positions are driven by the same reusable section descriptors. Moving section focus with
  `tab`/`shift+tab` changes the selected section; `enter` activates interactive sections. The
  body renders only the selected section, while inactive section names and badges stay in the tab
  bar.
- The ticket detail panel header separates issue identity (key, status, type), a standalone summary
  band, metadata, a subtle divider, and tabs so navigation no longer competes with the ticket title.
- The always-visible ticket detail footer is intentionally limited to primary actions (`esc`,
  `j`/`k`, `tab`, `a`, and `b`); secondary section jumps and copy actions remain in `? help`.
- Long ticket detail views right-align their viewport line indicator at the bottom of the panel so
  pagination state reads as panel chrome instead of another content row. The indicator includes
  the active detail section name on the left and the line range on the right.
- The Links section uses a Lip Gloss table, and Comments render as numbered, repeated left-ruled
  blocks with inset bodies so long detail views have clearer visual hierarchy. The Hierarchy section
  always renders a Path block for either the current issue or known parent context, separates
  visible child issues from subtasks, keeps the selected row marked before activation, routes
  `j`/`k` and arrow keys to that selected row while the Hierarchy tab is selected, and reserves a
  linked-issues placeholder until Jira linked issue data is loaded through a real API path. Detail
  tables share one renderer so future pane/table styling stays centralized.
- Focused ticket detail does not append a permanent hierarchy/URL footer under every section;
  hierarchy data lives in the Hierarchy section and issue URL workflows live in actions/key bindings.
- The Actions section uses a compact table with action state and detail columns. Detail notices
  render as a distinct styled block instead of plain inline text.
- Focused ticket detail supports section navigation with `n`/`p`, direct jumps to description with
  `d`, comments with `m`, hierarchy with `h`, and links with `l`.
- Focused ticket detail supports selected issue actions: `b` opens the Jira issue URL, `c` copies
  the issue key, and `y` copies the issue URL when Links is not focused.
- Jira ADF descriptions are rendered through `internal/adf`, which handles readable terminal output
  for links, mentions, inline code, code blocks, lists, blockquotes, panels/statuses, hard breaks,
  and simple tables.
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

- Saved views are generated from the default project today; custom view editing in config is not
  exposed yet.
- Jira write workflows must be metadata-driven. Creation, editing, assignment, transitions, board
  and sprint movement, priorities, statuses, issue types, field options, required fields, users, and
  custom fields should come from Jira metadata for the active site/project/issue, not hard-coded
  assumptions.
- Config supports a single saved profile today, with a TOML shape designed for multiple profiles.
- There is no OAuth flow; API token auth is the only supported auth mode.
- There is no git repository initialized in this folder yet.
- The Jira client uses `go-atlassian` for search, issue detail, comment reads, and comment
  creation, including ADF payload construction for text, links, and selected user mentions. Transitions,
  metadata, sprint, and board APIs are not exposed through
  `internal/jira` yet.
- Jira user display strings prefer real display/name/email/key fields over generic privacy aliases
  such as `User e31ec`; ticket detail also preserves a better selected-list assignee if the detail
  response returns a generic alias.
- The worker pool currently supports issue search, explicit parent expansion, issue detail, comment
  reads, comment creation, and Jira user search; sprint data is the next background workflow family
  to add.
