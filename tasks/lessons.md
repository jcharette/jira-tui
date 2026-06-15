# Lessons

- Prefer maintained third-party libraries for reusable infrastructure when they fit the Go TUI
  stack. Before feature or fix work, ask whether a library can reduce hand-rolled cache, sync,
  concurrency, parsing, or UI primitive code; hand-roll only when library options are a poor fit.
- The TUI must always stay responsive. It is acceptable and encouraged to use more background
  workers/threads for Jira reads, cache refresh, TTL expiry handling, and prefetch/sync work, as
  long as those flows are bounded, use maintained libraries where they fit, and never block the
  Bubble Tea update/render loop.
- Prefer stale-while-refresh behavior for Jira read caches: show useful cached data immediately,
  use TTL freshness to decide when to refresh, and perform refreshes through background workers
  instead of making the TUI wait.
- Diagnostics and observability surfaces should stay read-only and non-blocking. Record cheap
  model-local events from existing worker/cache paths before adding heavier logging, persistence,
  or new background requests.
- When the user says the existing dirty tree is working, treat those changes as intentional working
  state. Do not frame them as suspicious or unrelated caveats; verify the whole tree and commit the
  cohesive working set if asked.
- For TUI focusable lists, keep the selected row visible even before the sub-mode is activated.
  Hiding the cursor until focus mode makes a selected section look non-interactive and can make
  users think the feature disappeared.
- If a TUI section displays a selection cursor, route movement keys to that cursor immediately.
  Do not show a cursor while leaving `j/k` or arrow keys bound to unrelated panel scrolling.
- Do not commit every small bug fix by default. Commit only when the user asks, when creating an
  intentional checkpoint, or before a risky transition; otherwise leave verified changes in the
  working tree for review.
- For Jira write workflows, do not default to an Actions menu as the primary UX. Prefer making
  existing ticket detail sections and fields directly navigable: move to a field/section, press
  enter to edit or act, then open a metadata-backed dialog populated from Jira.
- For create/edit foundations, mirror the future user flow in the API shape. Fetch project issue
  types first, then fields for the selected issue type, instead of preloading every field for every
  possible type before the user chooses.
- In modal edit flows, key hints are not selectable controls; make the keyboard-owned action clear
  and keep instructional copy out of body notices when footer/dialog hints already explain it.
- Detail dialogs must be composed into the visible detail body, not appended after rendered
  scrollable content. Long descriptions should never push mutation modals below the visible panel.
- Text-backed modal fields must use a real editor surface, not a hand-rolled string renderer. Set
  value fields should use a picker/list surface with keyboard selection instead.
- A shortcut that means "edit this field" should start the modal/edit flow directly. Do not stop in
  a footer-only focus state that requires users to infer they need a second keypress.
- Run `gofmt` freely on any touched Go file. Do not wait for file-specific formatting approval.
- For ticket-detail focus tests, assert focused fields or sections by stable IDs/names instead of
  raw `detailFocus` indexes. The focus order now mixes editable fields and sections, so numeric
  indexes are brittle unless the test is explicitly about ordering.
- Avoid redundant semantic key bindings. Conventional navigation aliases are fine when they mean
  the same thing, but do not keep multiple conceptual paths for one workflow; `tab` should own focus
  movement, `enter` should act on focus, and single-letter keys should be distinct accelerators.
- User picker edit flows, such as Assignee, need type-to-filter input inside the picker. Do not
  rely only on opening with a prefilled/current-value search; users must be able to type characters
  like `jon` and refresh/reduce Jira user results as they go.
- Create issue forms should render and submit from Jira create metadata wherever the field shape is
  safely supported. Do not hide optional supported fields behind a Summary/Description-only form;
  unsupported required fields must be visible and block submit before Jira rejects the request.
- Jira create metadata paged endpoints can return empty mappings even when expanded create metadata
  has usable data. For issue types or fields, treat an empty successful mapping response as
  inconclusive and fall back to expanded `Issue.Metadata.Create` before concluding Jira returned no
  metadata.
- Claude/AI workflows must be feature-flagged and gate-checked from config before they are shown,
  before background work is enqueued, and before generated changes are applied. Default AI features
  disabled, require confirmation, and keep Jira/git/GitHub/code write gates closed until a workflow
  explicitly earns them.
- First-pass AI workflows should prove the interaction model with read-only outputs, bounded local
  CLI execution, and Diagnostics breadcrumbs before adding Jira, git, GitHub, or code write paths.
- Long-running AI calls must show visible progress context and offer cancellation. A modal that only
  says "asking..." is indistinguishable from a dead CLI or frozen TUI.
- Timeout errors need wall-clock evidence in the UI. Show start time, deadline, elapsed time,
  timeout value, and command shape so users can tell whether the app timed out early or the external
  process exceeded its configured deadline.
- External CLI integrations should stream stdout/stderr/progress into the TUI when possible. Waiting
  for final process exit hides auth prompts, plugin startup, partial model output, and useful errors.
- Streaming protocol envelopes must be parsed into user-facing statuses or text before rendering.
  Do not dump raw JSON stream events into a TUI modal unless the user explicitly opened a debug log.
- Partial assistant streams can repeat or overlap. Render a rolling preview of the latest useful
  assistant text instead of a chronological list of every partial chunk.
- Final AI responses can be much longer than the terminal panel. Always bound modal result bodies
  and provide line-range context instead of letting the overlay grow off-screen.
- Bounded modal results also need their own scroll state and footer hints. Truncation alone fixes
  layout but still leaves the user unable to inspect the full result.
- Large read-only result modals should use responsive percentage-based width with min/max guards,
  not the narrow fixed cap intended for small edit dialogs.
- Long-running subprocess modals should separate product progress from debug evidence. The normal
  waiting state should show activity, elapsed progress, useful streamed output, and cancel; command,
  request, start, and deadline details belong in diagnostics or timeout/error states unless the user
  explicitly opens a debug surface.
- Do not stream partial AI assistant text into normal loading modals. Constantly changing partial
  output is distracting and hard to read; show stable statuses such as waiting or receiving response,
  and keep detailed stream content in Diagnostics and final result/error views.
- AI result rendering needs Markdown-aware handling for common structures. Pipe tables should route
  through the fitted table renderer instead of being wrapped as ordinary prose.
- Future AI/code workflows need ticket-to-local-workspace mapping. A ticket should be able to map to
  one or more local repository folders so Claude can receive the right repo context without guessing
  from the process working directory.
- Do not treat ticket-to-local-workspace mapping as the next AI feature by default. The next AI
  product slice should focus on ticket assistance first: generating better ticket content and
  evaluating/sanitizing existing Jira tickets, with workspace mapping feeding later code-workflow
  context.
- AI-generated ticket content must pass through a user-editable draft review before any Jira create
  or update workflow. Acceptance Criteria should be a first-class draft section, not buried in the
  description prose, even if Jira stores the final text inside Description for now.
- AI draft review modals must be usable before write/apply workflows exist. Long drafts need
  visible line ranges, paging keys, and a copy/export command; otherwise the user cannot practically
  do anything with the generated content.
- In AI draft review modals, the editable draft is the primary artifact. Do not cap it like a small
  inline field or let review text visually merge with it; give the draft primary modal space and a
  distinct editable block, hiding review preview first on cramped terminals.
- AI result modals should separate generated review, local draft, and available actions as distinct
  zones. When those concepts share one text flow, users cannot tell what Claude produced, what they
  can edit, or what will happen next.
- AI ticket assistance needs both edit and comment outputs. Direct field edits are useful for owned
  tickets; comment posting is the safer default for clarifying tickets created or owned by someone
  else.
- AI refinement prompts must include the current user-edited draft, not only the original ticket or
  prior Claude output. Otherwise Claude has to reinvent context and can discard user corrections.
- In ticket detail, `a` is the AI/Claude accelerator when AI actions are available. Avoid reusing it
  for add-comment as the primary path; comment creation remains available through the Actions
  workflow and focused comment composer.
- Local Claude integration should prefer `exec.LookPath("claude")` for auto-detection and allow a
  manual command/path override for users whose terminal PATH does not expose the CLI.
- Claude Code workflows in this app must work without an Anthropic API key. Use the user's local
  `claude` CLI/session by default; treat direct API SDK integration as a separate optional provider.
- Binary config values should use picker/toggle behavior in the TUI, not free-text editing. Users
  should not have to type raw `true` or `false` for feature flags, gates, or other boolean settings.
