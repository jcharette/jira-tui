# Changelog

All notable changes to this project should be recorded here.

## Unreleased

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
- Planned saved issue views for assigned, reported/created, project open, watching, and epic workflows.
- Added default saved views for assigned, created/reported, project open, current sprint, and watching.
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
