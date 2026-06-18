# Changelog

All notable changes to this project should be recorded here.

## Unreleased

- Added view-scoped automatic child issue loading: the default Epics view opts into related child
  lookups while ordinary views avoid extra child-search Jira reads unless configured.
- Added metadata-backed transition-screen field support for required Resolution and Comment fields,
  including clear blocking feedback for unsupported required transition fields.
- Added comment composer formatting controls and Jira ADF mark conversion for bold, italic, and
  inline code while preserving detected links and selected Jira mentions.
- Closed the active Read/View backlog bucket by moving the remaining concrete follow-ups into
  Navigation/Query, Comments/Workflow, and Creation/Editing backlog sections.
- Added concrete `Edit Summary` and `Change Priority` rows to the ticket detail Actions workspace,
  routing them to the existing metadata-backed edit flows.
- Removed the stale linked-issues placeholder from the ticket detail Hierarchy workspace and added
  clearer hierarchy empty states for root issues and known-parent issues.
- Added read-only Jira issue links to the ticket detail Links workspace, including link badges,
  relationship/status/summary rows, open-in-browser, and copy-key behavior.
- Completed the active-context key binding audit by removing hidden detail `b` browser-open
  behavior and the no-AI detail `a` comment fallback, keeping help/footer metadata aligned with key
  handling.
- Added focused TUI regression coverage for issue-list first/last navigation rendering, detail
  escape navigation, and focused Links footer context.

## 0.2.1 - 2026-06-17

- Added a `cmd/jira` package for versioned `go install` installs, corrected the Go module path to
  the actual GitHub repository, and documented release binary, Go, and source install paths.

## 0.2.0 - 2026-06-17

- Added an in-app query modal with direct JQL entry and AI-assisted JQL generation, including
  generated-query preview, revision feedback, and explicit confirmation before running.
- Added persisted recent query history in the query modal, covering confirmed direct JQL and
  confirmed AI-generated JQL with source prompts, scoped to the active Jira cache namespace.
- Added saved-query promotion from the query modal, so a selected recent direct or AI-generated JQL
  query can be named, written to config, and used in normal saved-view rotation.
- Added a local issue-table Active filter with `f`, hiding loaded terminal-status tickets without
  changing saved views, JQL, Jira reads, or cached issue data.
- Added local issue-table subtree collapse with `z`, so dense loaded ticket branches can be hidden
  or revealed without changing Jira reads or the loaded issue data.
- Moved Assignee and mention picker filters onto Bubbles `textinput` while preserving Jira user
  search, cached lookup, and shared choice-list rendering behavior.
- Added a default `Epics` saved view scoped to the configured default project.
- Added sanitized API debug rows to Diagnostics, showing Jira operation family, request ID,
  issue/project scope, result class, safe counts, elapsed time, and categorized errors without raw
  request bodies, response bodies, tokens, or full JQL strings.
- Added cache refresh failure counts to Diagnostics by retaining the latest refresh error on active
  view, detail, comments, transition, edit metadata, create metadata, and expanded-child cache
  records.
- Added conservative persistent cache cleanup that removes SQLite cache rows not updated in the
  last seven days from a short background startup task.
- Patched retained issue-detail and current active-view cache records after confirmed summary,
  description, priority, assignee, and status writes, and invalidated cached transition options after
  status changes.
- Added Diagnostics cache-family summaries that show retained active view, detail, comments,
  transition, metadata, create, and expanded-child cache records as fresh/stale counts.
- Polished the ticket-detail header so the summary reads as the title, metadata uses compact
  label/value spacing, and the active section tab uses a plain selected marker instead of filled tab
  styling.
- Added a quiet header background activity indicator for active Jira refreshes, active AI work,
  recent background event bursts, and recent background errors.
- Bounded selected-issue detail prefetch from active-view refreshes and table navigation, and stopped
  list interactions from prefetching comments so large Jira views stay responsive while explicit
  detail opens still load detail and comments.
- Added provider-neutral AI task event payloads and routed existing Claude-backed AI requests through
  an event-publishing adapter for future Claude/Codex/auto provider routing.
- Added a Watermill GoChannel-backed event stream foundation for Jira/app events, with active-view
  ticket `new`/`updated` events routed into Diagnostics as the first consumer.
- Changed persisted active-view startup behavior to stale-while-revalidate: displayable cached rows
  render immediately, stale rows are marked refreshing, and Jira refresh happens in the background.
- Extended the SQLite persistent cache to explicit expanded child issue reads used by hierarchy
  expansion.
- Extended the SQLite persistent cache to create issue type and create field metadata used by the
  create ticket flow.
- Extended the SQLite persistent cache to status transitions and edit metadata used by summary and
  priority editors.
- Extended the SQLite persistent cache to issue detail and comments, including comment invalidation
  after posting.
- Added a SQLite-backed persistent cache for active Jira views using `modernc.org/sqlite`, so cached
  view rows can hydrate from the user's app cache directory across app restarts.
- Moved issue comments onto retained cache records so fresh comments skip Jira work, stale comments
  stay visible while refreshing, and comment posts invalidate retained comment data.
- Replaced marker-only issue detail freshness with retained detail cache records using the existing
  `ttlcache` dependency, preserving stale-visible detail refresh behavior.
- Added worker scheduler stats to Diagnostics so the overlay shows queue running, pending,
  coalesced, and capacity counts even before activity events exist.
- Added worker request priority and duplicate-read coalescing around the existing `ants` pool so
  foreground Jira work can be admitted ahead of queued background refresh.
- Added an in-memory active-view cache using `ttlcache` so fresh Jira views render without a new
  search, stale views render immediately while refreshing, and failed refreshes preserve stale rows
  with visible freshness state.
- Added a Jira cache/background refresh design for responsive large views, freshness labels,
  priority classes, stale-while-refresh behavior, write invalidation, diagnostics, and future
  persistent cache constraints.
- Styled Jira ADF panel/status markers, blockquotes, mentions, URLs, and email addresses in ticket
  detail rich text while preserving compact code blocks and fitted table rendering.
- Improved config editor scalar text fields with cursor-aware Bubbles text input while preserving
  existing boolean and color controls.
- Routed issue-browser footer command rendering through Bubbles `key`/`help` adapters while
  preserving existing context labels and grouped keyboard help.
- Added a shared Bubbles list-backed choice-list adapter and migrated comment mention results onto
  it as the first picker/list surface.
- Migrated Assignee search results onto the shared Bubbles list-backed choice-list adapter while
  preserving the existing Jira user search and assignment flow.
- Migrated focused dynamic create option fields, including Components, onto the shared Bubbles
  list-backed choice-list adapter while preserving existing filter and selection behavior.
- Migrated create issue type selection onto the shared Bubbles list-backed choice-list adapter
  while preserving keyboard movement and create-field metadata loading.
- Replaced manual create option filter input with Bubbles `textinput` while keeping existing
  create-field filter and selection state synchronized.
- Migrated the Priority picker onto the shared Bubbles list-backed choice-list adapter while
  preserving metadata-backed selection and submit behavior.
- Restored focused ticket-detail action consistency: pressing `enter` on Comments now opens Add
  Comment, the Actions menu routes implemented Status and Assignee workflows, and Assignee has its
  own footer/help context.
- Added ticket-detail contract tests that verify focused `enter` actions and section footer hints
  stay aligned across Summary, Assignee, Priority, Links, Hierarchy, Comments, Actions, and Status.
- Added a package/file boundary audit that identifies `internal/tui/model.go` and
  `internal/tui/model_test.go` as the main monolith risk and recommends same-package workflow file
  splits before new internal packages.
- Split the former TUI monolith into same-package workflow files for commands, diagnostics,
  issue-list rendering, comments, create issue, Claude assist, detail views, rich text, chrome,
  navigation, worker results, formatting, summary editing, and external actions.
- Split the broad TUI test file into matching workflow test files while keeping shared model fakes
  and core update tests in `model_test.go`.

## 0.1.0 - 2026-06-16

- Wrapped Jira ADF rendering with a maintained ADF-to-Markdown converter for richer descriptions
  and comments while preserving the existing fitted table path.
- Added sanitized real-shaped Jira ADF fixtures, a dev-only ADF fixture capture/sanitize helper, and
  compact code block rendering without ASCII border rows in rich ticket text.
- Added typeahead filtering for Jira create-ticket option fields, Claude-assisted Component
  recommendations from Jira metadata, and fixed create blocking on metadata-owned Project/Issue
  Type required fields.
- Added an interactive Open Questions panel to Claude-assisted ticket creation so users can answer
  draft blockers locally and feed those answers into the next AI refinement prompt.
- Widened create-ticket dialogs, made Summary/Description editing roomier, and simplified
  create-ticket AI loading output so partial Claude/debug details stay out of the normal modal.
- Made Claude-assisted ticket creation use only Jira-returned issue types, auto-apply matching AI
  type recommendations, and allow changing issue type from the create form without losing the draft.
- Added persistent TOML config with a Bubble Tea config editor and required default project setup.
- Renamed the installed binary to `jira` and added `make install-user` for updating `~/bin/jira`.
- Added configurable appearance colors and shared Lip Gloss styling for config and issue screens.
- Added explicit config editor navigation between sections and fields.
- Aligned config editor header and footer chrome with the issue browser, including context labels
  and grouped footer commands.
- Added live Jira connection testing in the config editor with targeted failure feedback.
- Reworked the issue browser into a responsive issue table plus selected-issue detail layout.
- Replaced the redundant issue-list side preview with a full-width, viewport-backed issue list.
- Condensed the main issue list into a Lip Gloss tree-backed hierarchy with compact issue symbols,
  status/priority/assignee columns, and selected-row metadata.
- Added configurable issue-list symbol modes with `auto`, `plain`, `symbols`, `emoji`, and `nerd`
  options.
- Replaced the raw JQL chrome in the main view with a readable Filter summary for common saved
  views.
- Reduced issue-list noise by shortening owner names and avoiding redundant selected-parent child
  counts when children are already visible.
- Added worker-side parent enrichment so issue lists fetch missing parent tickets and render
  children under real parent rows when the active JQL omits the parent.
- Added subtask hydration from Jira parent issue data so known subtasks render as nested rows
  instead of only appearing as hidden counts.
- Added worker-side child enrichment using Jira `parent in (...)` lookups so visible parent tickets
  can pull in subtasks and child rows that the active saved view JQL omitted.
- Added explicit issue-list parent expansion: `x` loads open children for the selected parent and
  `X` loads all children, including resolved/done issues, without changing the active saved view.
- Removed redundant selected-parent child counts when children are already visible in the issue tree.
- Indented issue-tree child connectors under parent rows so hierarchy alignment reads cleanly.
- Fixed issue-list header and nested tree alignment by giving the hierarchy gutter a fixed terminal
  width and sharing column sizing between headers and rows.
- Added explicit narrow issue-list column breakpoints so owner, key/status, and priority columns
  compact predictably below 96, 90, and 76 columns.
- Capped issue-list hierarchy indentation on narrow terminals and locked the supported minimum
  terminal height to at least eight useful issue rows.
- Improved issue-list hierarchy readability by widening the tree gutter, keeping connectors attached
  to nested rows, and preserving subtask status, priority, and owner values even when they match the
  parent.
- Reordered issue-table footer hints so opening the selected ticket stays visible before lower
  priority paging commands on constrained terminals.
- Grouped footer commands by task with subtle separators and trimmed secondary detail actions from
  the bottom chrome.
- Fixed footer fitting so narrow terminals omit overflowing commands instead of truncating them
  mid-label.
- Moved empty/error state recovery hints out of panel bodies and into the grouped footer command
  grammar.
- Added issue type, parent, and subtask metadata to search results for table hierarchy and detail views.
- Added focused ticket detail view opened with `enter` and closed with `esc`.
- Added tab-driven ticket detail navigation so `tab` and `shift+tab` move focus across Summary,
  Description, Hierarchy, Comments, Actions, and Links when available; `enter` activates the focused
  detail section.
- Made the ticket detail section navigation visible with an explicit `Sections` header, tab/enter
  hints, and a styled active tab.
- Reworked detail tabs around reusable section descriptors with full and compact labels so future
  detail panes can add sections without hard-coded abbreviation switches.
- Added context badges to ticket detail tabs and section headers, including hierarchy child counts,
  link counts, and comment loading/count/error state.
- Reworked ticket detail sections to use a shared ruled header treatment so Summary, Description,
  Links, Hierarchy, Comments, and Actions read as one workspace instead of unrelated text blocks.
- Reworked ticket detail section navigation around reusable section descriptors instead of a
  hard-coded section-name list.
- Changed ticket detail tab movement to select a single focused section body, while `enter` still
  activates interactive sections such as Links, Hierarchy, and Actions.
- Added a persistent ticket identity line to the ticket detail header with key, status, type, and
  summary so the selected issue stays visible while scrolling deeper sections.
- Condensed ticket detail chrome by making the panel header lead with ticket identity, moving
  Summary metadata into the header, removing the Summary body tab, and rendering section navigation
  on one compact line without redundant labels.
- Split the ticket detail header into separate identity, summary, metadata, and tab rows so long
  summaries no longer crowd the navigation controls.
- Added breathing room between the ticket summary and metadata rows so the detail header reads as
  ticket context first, then metadata, then navigation.
- Added a subtle divider between ticket metadata and detail tabs so the header reads as separate
  ticket context and navigation bands.
- Reduced the default ticket detail footer to primary actions while keeping secondary bindings in
  the paginated help screen.
- Added an active mode label to the footer so table, detail, link, hierarchy, action, and comment
  contexts are easier to distinguish.
- Added selected-section footer hints in ticket detail so visible Hierarchy, Links, and Actions
  rows advertise their movement and activation commands before entering a sub-mode.
- Routed selected-section Hierarchy, Links, and Actions commands before sub-mode activation so
  footer hints match actual key behavior.
- Clarified detail-mode key semantics so `o` opens the selected issue URL in browser, while
  `o`/`O` keep sorting behavior on the issue table.
- Added a direct Status section in ticket detail that loads available Jira transitions through the
  worker pool, renders a Jira-populated transition picker, and applies the selected transition.
- Added Jira client and worker support for listing and applying issue status transitions.
- Added direct Summary editing from the ticket detail header, backed by Jira edit metadata and
  worker-submitted summary updates.
- Added Jira client and worker support for issue edit metadata and summary updates.
- Moved Summary editing and Status transition selection into a shared ticket-detail modal dialog
  pattern so focused mutations have contextual data, explicit submit/cancel controls, and no inline
  draft fields in the normal detail layout.
- Fixed Summary modal editing for long values by showing the active input tail and cursor, and
  removed the pre-modal Summary instruction notice from the detail body.
- Changed the Summary shortcut so `s` starts the metadata-backed edit flow immediately and opens an
  editor-backed modal instead of stopping in a footer-only focus state.
- Added metadata-backed Priority editing with `p`, using Jira edit metadata allowed values in a
  picker modal and worker-submitted priority updates.
- Added direct Assignee editing from ticket detail with a type-to-filter Jira user picker, worker
  submitted assignment by account ID, and immediate visible/cached assignee updates on success.
- Added short-lived in-memory Jira user-search caching with `github.com/jellydator/ttlcache/v3` so
  repeated typeahead queries can avoid duplicate Jira calls.
- Added short-lived issue-detail freshness tracking with `github.com/jellydator/ttlcache/v3` so
  fresh cached details avoid duplicate Jira reads and stale details refresh through the worker pool
  while remaining visible.
- Added Jira client and worker support for create metadata discovery, including project issue types
  and selected issue-type create fields with required, schema, operation, and allowed-value data.
- Added a go-atlassian create metadata fallback so issue type discovery retries the expanded
  `Issue.Metadata.Create` response when the preferred paged issue type mapping endpoint returns
  zero values.
- Added the same expanded create metadata fallback for selected issue-type field discovery when the
  preferred paged field mapping endpoint returns zero fields.
- Added the first create-ticket workflow with `n` from the issue table and focused ticket detail:
  choose a Jira-provided issue type, fill Summary and Description in a modal, and submit issue
  creation through the worker pool.
- Added dynamic create-field rendering and submission for supported Jira metadata fields beyond
  Summary and Description, including Priority, Labels, Components, simple text/number fields, and
  single-select option fields.
- Bounded long create-field picker lists so Jira metadata with many options, such as Components,
  stays inside the visible create-ticket modal.
- Added sanitized create-field diagnostics with total, supported, unsupported-required, and
  field ID/name/schema samples to explain why a create form renders only certain Jira fields.
- Added the first Claude Code/CLI setup foundation: config schema, config menu fields, local command
  auto-detection with optional manual path override, startup `--version` preflight, and Diagnostics
  status. Claude workflows use the user's local CLI/session and do not require an Anthropic API key.
- Added the first read-only Claude workflow: when enabled, available, and `ticket_plan` is flagged
  on, ticket detail shows a Claude section that asks the local CLI for an implementation and
  verification plan using selected ticket context, then renders the result in a modal with
  Diagnostics submit/result events.
- Improved the Claude ticket-plan modal so long-running local CLI calls show elapsed time,
  configured timeout, and an `esc` cancel path instead of appearing stuck.
- Added more Claude ticket-plan modal evidence: request type, read-only mode, sanitized command,
  output wait state, start time, deadline, and timeout-specific failure copy.
- Switched Claude ticket-plan runs to stream-json progress when the TUI requests progress, rendering
  recent stdout/stderr/partial output events in the modal before the final result.
- Parsed Claude nested `stream_event` envelopes into readable output/status rows and suppressed raw
  JSON progress payloads in the modal.
- Changed Claude progress rendering to a single rolling assistant preview so overlapping partial
  chunks do not repeat as multiple `output:` rows.
- Assembled Claude text-delta chunks into the rolling preview and bounded long final Claude plan
  results inside the ticket-detail panel with a line-range hint.
- Added scrolling for final Claude plan results with `j`/`k`, arrow keys, page keys, and top/bottom
  jumps.
- Made the Claude plan dialog use a responsive percentage of the available detail width instead of
  the narrow default edit-dialog cap.
- Simplified the Claude loading modal to concise subprocess activity, elapsed progress, stable output
  state, and cancel; detailed command/timing evidence remains for timeout
  troubleshooting instead of the normal waiting view.
- Calmed Claude Plan and Ticket Assist loading modals by replacing constantly changing partial
  assistant text with stable output states such as waiting, receiving response, and receiving CLI
  messages; detailed stream events still flow to Diagnostics and final results/errors.
- Rendered Markdown pipe tables in final Claude output through the existing fitted table renderer so
  wide table rows no longer leak as raw, clipped pipe text.
- Added a `ticket_assist` Claude feature flag and existing-ticket assistant workflow. The Claude
  section can now evaluate a selected ticket, call out clarity/acceptance/test gaps, and return an
  editable structured draft with first-class Acceptance Criteria before any Jira write exists.
- Made the Ticket Assist draft modal usable for long Claude output: review text is bounded, draft
  editor overflow shows line ranges and pages with `pgup`/`pgdn`, and `ctrl+y` copies the edited
  draft to the clipboard.
- Gave the Ticket Assist draft more primary modal space, rendered it as a distinct editable block,
  and hid review preview on cramped terminals so the draft remains usable.
- Added gated Ticket Assist apply-to-Jira support: when Claude Jira writes are enabled,
  `ctrl+s` opens confirmation and then updates Summary and Description through the worker pool;
  with writes disabled, the draft remains local and copyable.
- Added an iterative Ticket Assist refinement loop: `r` opens an instruction editor, and submitting
  it sends Claude the original ticket context, the current user-edited draft, and the user's
  instruction before replacing the editable draft with the refined result.
- Split Ticket Assist results into clearer `Claude Review`, `Local Draft`, and `Available Actions`
  zones so generated review, local edits, and next actions do not read as one text blob.
- Added a Ticket Assist comment path: `c` opens confirmation to post the current draft as a Jira
  comment without editing Summary or Description, then refreshes comments through the worker pool.
- Added inline Description AI from ticket detail: pressing `a` on Description opens a scoped AI
  picker and returns editable drafts that can be refined, copied, posted as comments, or applied to
  Description behind existing write gates.
- Added Jira client and worker support for updating issue Description with ADF text.
- Changed ticket-detail `a` to jump to the Claude/AI section when AI actions are available; add
  comment remains available through the Actions workflow and as the fallback when Claude is not
  available.
- Changed boolean config fields, including Claude feature flags and write gates, to true/false
  picker controls so users no longer type raw boolean strings.
- Added clearer create-ticket empty metadata handling: when Jira returns zero creatable issue types,
  the modal says so, keeps only relevant footer commands, and exposes `ctrl+d` diagnostics with
  sanitized create metadata result counts.
- Removed redundant ticket-detail `n` section navigation so `tab`/`shift+tab` own detail focus
  movement and `n` consistently means new ticket.
- Added a `ctrl+d` Diagnostics overlay that shows recent worker submit/result activity,
  issue-detail cache hit/miss/stale-refresh decisions, summary counts, activity bars, and labeled
  event rows from a bounded in-memory buffer.
- Added ticket-detail field focus so `tab` moves through Summary, Assignee, Priority, and sections, and
  `enter` opens the focused field or section mutation flow.
- Reworked ticket detail into a focused-section workspace: the selected tab owns the body, and
  inactive section names and badges stay in the tab bar.
- Removed inactive section previews from the focused ticket detail body; inactive section names and
  badges now live only in the tab bar.
- Removed the always-on Hierarchy/URL footer from focused ticket detail so inactive metadata no
  longer appears under every selected section.
- Right-aligned the focused ticket detail viewport line indicator so paging state is visually
  separate from ticket content.
- Added the active detail section name to the focused ticket detail pager so long views show what
  section is currently being scrolled.
- Decluttered ticket detail by removing duplicate key hints from the header tab row, dropping the
  active-section `>` prefix, and aligning inactive preview rows with their labels.
- Made the ticket detail tab bar the single strong section-focus indicator; body section headers now
  render as content headings instead of a second selected control.
- Removed an extra blank line between the global app header and Filter row to reduce top-level
  vertical chrome.
- Kept the Description section visible while issue detail is loading or failed, with consistent
  loading/error state rendering.
- Reworked ticket detail comments into repeated left-ruled blocks with inset bodies and made comments/hierarchy
  empty states consistent.
- Reworked ticket detail comment blocks to lead with author/date, keep comment counts secondary,
  and add clearer spacing before the comment body.
- Added a clearer gap between Description section headers and rich body content.
- Aligned Description and Comments loading, empty, and error messages around a shared Status block.
- Added comment numbering inside ticket detail comment blocks so longer threads have clearer
  landmarks.
- Reworked the ticket detail Links section into a compact Lip Gloss table with cleaner selected-row
  treatment.
- Tightened the ticket detail Summary headline and rendered Hierarchy parent information as a
  structured table row instead of loose inline text.
- Centralized ticket detail table rendering so Summary metadata, Links, Hierarchy, and Actions use
  one shared Lip Gloss table path.
- Added a ticket detail comment limit hint when the visible comments reach the current fetch cap.
- Fixed Jira privacy fallback names like `User e31ec` leaking into ticket detail metadata when
  better user fields or the selected issue's assignee are available.
- Reworked the ticket detail Actions section into a compact table with action state and detail
  columns, preparing it for future metadata-backed edit workflows.
- Added a styled ticket detail notice block so action feedback and failures do not disappear into
  the normal detail text stream.
- Added a focused ticket detail Hierarchy section that renders visible child issues in a compact
  status/priority/owner table for parent and epic workflows.
- Reworked the ticket detail Hierarchy section into grouped Path, Children, Subtasks, and Linked
  Issues areas so parent and subtask relationships scan more clearly.
- Fixed the Hierarchy section so the selected child row remains visibly marked before activation
  and root issues still show a Path block for the current issue.
- Fixed Hierarchy tab navigation so `j`/`k` and arrow keys move the visible child selection instead
  of scrolling the surrounding detail panel.
- Added hierarchy focus inside ticket detail: activating the Hierarchy tab lets users select child
  issues with `j`/`k` and open the selected child with `enter`, using a dedicated key context that
  can later map cleanly to real panes.
- Added a detail back stack so opening a child issue from the Hierarchy section returns to the
  parent ticket detail on `esc` before leaving detail mode.
- Added a focused Actions section in ticket detail with selectable actions for comment, browser,
  copy key, and copy URL, plus disabled metadata-required placeholders for future edit,
  transition, assignment, and subtask creation workflows.
- Added read-only selected issue detail fetching through the worker pool with stale-response
  protection and per-key caching.
- Added issue detail rendering for description text, reporter, creator, labels, components, fix
  versions, created date, and updated date.
- Improved ticket description rendering with preserved Jira line breaks, list markers, paragraph
  spacing, and wrapped continuation indentation.
- Added a dedicated Jira ADF renderer for terminal-friendly links, mentions, inline code, code
  blocks, lists, blockquotes, panels/statuses, hard breaks, and simple tables.
- Added ADF fixture coverage for nested lists and rich table cells, and improved table rendering so
  hard breaks inside cells do not break the terminal table shape.
- Switched ticket/comment table display to Lip Gloss table rendering so Jira tables use styled
  terminal borders instead of raw ASCII separators.
- Switched focused ticket detail scrolling to Bubbles `viewport` while preserving existing section
  jumps and line indicators.
- Added per-section ticket detail scroll memory so returning to a long detail section restores its
  previous position for the selected issue.
- Reworked the issue browser header spacing with Lip Gloss placement so the fixed chrome is less
  hand-rolled.
- Styled inline code in ticket descriptions as terminal code spans instead of leaving plain
  backtick-delimited text.
- Improved code block rendering in ticket descriptions by hiding raw fences and styling block lines
  separately from prose.
- Added a dedicated code block style with a stronger background and left rule so multi-line code
  reads as a block instead of highlighted prose.
- Reworked ticket code blocks as full-width terminal blocks with visible ASCII boundaries and
  padded code lines.
- Collapsed excess blank rows around ticket description code blocks and trimmed empty fenced-code
  lines so blocks render tightly in detail view.
- Added focused ticket detail link discovery for embedded `http(s)` URLs, `mailto:` links, and
  plain email addresses, plus an `l` jump to the Links section.
- Cleaned up email link rendering so Jira `mailto:` marks do not duplicate visible email addresses
  in descriptions or the detail Links section.
- Added focused Links section actions: select links with `j`/`k`, open with `o` or `enter`, and
  copy with `y`.
- Added focused ticket detail section navigation with `n`/`p`, `d`, `h`, and `l`, plus selected
  issue actions for opening the Jira issue, copying the key, and copying the URL.
- Added read-only Jira comments in focused ticket detail, fetched through the worker pool and
  rendered with the existing ADF-aware description formatter.
- Added the first Jira write workflow: plain-text comment creation from ticket detail with explicit
  review/confirmation, worker-backed posting, failure feedback, and comment refresh after success.
- Centralized TUI key bindings by active context and added a `?` keyboard help screen with footer
  help rendered from the same keymap source.
- Bounded the keyboard help screen to the available terminal height and added help pagination using
  the shared app chrome budget.
- Kept the keyboard help title and active context visible while paging through long help content.
- Fixed comment composer input so space and newline keys are accepted while drafting comments.
- Reworked comment composition into a bounded multi-line editor with independent draft pagination
  and a compact composer header.
- Replaced the custom comment draft input with the maintained Bubbles textarea editor for cursor
  movement, multiline editing, deletion, and paste support.
- Aligned comment composer add/review/posting states with ticket detail section headers and styled
  notice blocks.
- Aligned comment composer detected-link, unresolved-mention, and mention-picker panels with the
  same ruled section-header grammar.
- Added comment composer detection for embedded URLs, bare domains, mailto links, and email
  addresses using `mvdan.cc/xurls/v2`, with a compact preview before posting.
- Posts detected comment URLs and email addresses as Jira ADF link marks so they render as links in
  Jira.
- Added Jira user-search support through the client and worker pool as groundwork for resolvable
  comment mentions.
- Added type-forward Jira mention selection in the comment composer. Typing `@` opens Jira user
  search, arrow keys choose a user, and selected users are posted as Jira ADF `mention` nodes using
  Atlassian account IDs.
- Added unresolved `@mention` detection in the comment composer so raw typed mentions are flagged
  until selected through Jira user search.
- Fixed Jira comment creation payloads by setting the required ADF document version on plain-text
  comments.
- Fixed comment composer input so typing `q` inserts text instead of quitting the app; `ctrl+c`
  remains the quit key in text-entry modes.
- Reworked focused ticket detail layout with a compact issue title, metadata band, primary
  description area, and muted hierarchy/URL footer.
- Recorded the decision to keep ADF rendering behind `internal/adf` after checking for maintained
  Go formatter options.
- Added focused detail scrolling so long ticket descriptions can be read without changing the
  selected ticket.
- Prioritized rich ticket detail rendering and navigation before additional workflow features.
- Added default saved views for assigned, created/reported, project open, current sprint, watching,
  and epics.
- Added watch UI view switching with `tab`/`]` and `shift+tab`/`[`.
- Added explicit issue table pagination with visible ranges, page up/down, and first/last controls.
- Added local issue table sorting by Jira order, priority, status, assignee, type, and key.
- Fixed issue table pagination row counts to reserve rendered headers, query, help text, borders,
  padding, and selected-detail panel space.
- Recorded a metadata-driven Jira workflow rule to avoid hard-coded assumptions about issue types,
  fields, priorities, statuses, transitions, users, boards, sprints, and custom fields.
- Added backlog and roadmap tracks for git integration and AI-assisted ticket/PR workflows.
- Added future backlog and roadmap items for browser-based Jira OAuth and secure credential storage.
- Recorded a TUI UX baseline requiring visible navigation, structure, and styling before completion.
- Added Makefile docs workflow targets for milestone status, milestone completion, release creation, and docs checks.
- Marked M0 Foundation complete in the roadmap.
- Added a dependency-aware product roadmap with milestones for issue details, comments, workflow actions, config, planning views, creation/editing, and power-user workflows.
- Adopted `ants/v2` as the execution engine under the internal worker dispatcher.
- Added a library-first planning rule for non-core infrastructure.
- Added an internal typed dispatcher and bounded worker pool for Jira background work.
- Routed issue search refreshes through the worker pool.
- Added configurable Jira worker count and queue size.
- Planned a small typed dispatcher and bounded worker pool as the next concurrency step.
- Added a Makefile for repeatable format, test, build, run, tidy, clean, and check workflows.
- Made issue refresh use an explicit goroutine and result channel at the TUI boundary.
- Recorded channel-backed background work as the concurrency pattern for future Jira workflows.
- Added configurable background issue refresh with stale-response protection.
- Added configurable Jira request timeout.
- Preserved selected issue across refreshed issue lists.
- Removed generated build artifact from the project tree.
- Migrated Jira issue search from custom HTTP code to `go-atlassian` Jira v3.
- Accepted `go-atlassian` as the preferred Jira Cloud library direction for broad Jira support.
- Added a working agreement that makes docs, planning, backlog, and changelog updates part of done work.
- Started Go/Bubble Tea Jira TUI project.
- Added Jira Cloud issue search client.
- Added env-based configuration.
- Added first issue list TUI with navigation and refresh.
- Added initial tests for config and Jira client.
- Added project docs for planning, backlog, release notes, and decisions.
