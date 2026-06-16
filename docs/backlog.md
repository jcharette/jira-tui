# Backlog

Use this as the source of truth for pending work. Keep items short and move finished work to the
changelog when it lands.

Docs are part of done. When an item here is completed, remove or move it and add a matching entry
to [releases/CHANGELOG.md](releases/CHANGELOG.md).

## Now: Read And View

- Follow [working-agreement.md](working-agreement.md) for every future code/doc change.
- Use [roadmap.md](roadmap.md) as the milestone source of truth.
- Use [package-boundary-audit.md](package-boundary-audit.md) before adding new TUI workflow
  surfaces: keep additions in the existing same-package workflow files, avoid putting unrelated
  behavior back into `model.go`, and defer new packages until there is a clear second consumer or
  dependency boundary.
- Continue replacing hand-rolled TUI rendering and input components with maintained Bubble Tea,
  Bubbles, Lip Gloss, or compatible public libraries when the package/file boundary audit identifies
  a clear, low-risk adapter opportunity. Do not force multi-column action/state tables into the
  simple choice-list adapter without a design pass.
- Keep ticket detail rich rendering covered as new Jira ADF shapes appear: links, mentions, inline
  code, code blocks, lists, blockquotes, panels/statuses, and tables now have focused TUI coverage.
- Continue rich comment composition support with Jira mentions and formatting controls on top of
  the bounded multi-line comment editor and detected-link/ADF-link support.
- Add tests around TUI navigation and rendering.
- Audit key bindings across every active context so commands stay clear: keep conventional
  navigation aliases, but remove redundant semantic paths where two unrelated keys do the same
  workflow.
- Continue focused ticket detail workspace work: real linked issue data in the Hierarchy/Links
  workspace, contextual footer commands, and metadata-backed implementations behind the Actions tab.
- Keep detail tabs and focused sections pane-compatible: new sections should expose their own
  focus state, key context, and activation behavior so they can move into real panes without a
  rewrite.
- Define explicit ticket detail modes before adding field writes: view mode for reading/scrolling,
  compose mode for comments, edit-field mode for metadata-backed forms, transition mode for status
  changes, and an action menu/command palette for less common operations.
- Add incremental loading strategy for sprint data and future expanded comment/detail workflows.
- Continue [jira-cache-performance-design.md](jira-cache-performance-design.md) with scheduler
  priority/coalescing: user writes and foreground reads must outrank background refresh, duplicate
  read jobs should coalesce, and background jobs should be dropped before foreground jobs when the
  queue is full.
- Add a generic in-memory TTL cache policy around Jira reads using maintained library support where
  possible: cache typeahead/metadata/detail/view data with per-data TTLs, refresh important entries
  asynchronously on expiry or view timers, use bounded background workers/threads where helpful,
  and merge new ticket rows into active views without blocking the TUI.
- Extend the Diagnostics overlay with queue depth, per-view refresh timestamps, cache expiry/refresh
  events, and background sync summaries as cache and prefetch tooling grows.
- Add an opt-in sanitized API debug log built on the Diagnostics model: record Jira operation,
  endpoint family, request ID, project/issue keys, result class, status/error summary, timing, and
  empty/paged result counts without storing tokens or raw response bodies. Use this later as the
  source for creating GitHub issues for this app with attached sanitized debugging context.
- Add saved issue views for assigned to me, reported/created by me, project open, watching, and
  epic-focused drill-down.
- Improve epic/subtask table grouping beyond current-result grouping by explicitly loading related
  children when the selected view needs them.

## Next: Navigation And Query

- Add a command/search input for changing JQL without restarting.
- Add epic and subtask support.
- Add sprint and board support.
- Add configuration file support for saved profiles and default queries.
- Add lightweight caching for issue detail and related data.
- Add bounded concurrency controls for detail/comment/sprint fetches when parallel loading expands.

## Next: Comments And Workflow Actions

- Add issue transition support that fetches available transitions and transition field metadata for
  the selected issue before rendering choices or moving issue status.

## Later: Creation And Editing

- Extend Jira metadata discovery adapters beyond the current create/edit metadata foundation:
  transition field metadata, assignable user search by project/issue, field options, boards, and
  sprints.
- Add a ticket action menu/command palette for edit workflows so growing actions do not overload
  single-key detail bindings.
- Expand the first create issue flow beyond Summary/Description: render supported required fields
  from Jira create metadata, validate allowed values, and add custom field shapes without hard-coded
  presets.
- Edit issue flow that uses Jira edit metadata, supports all editable fields cleanly, and validates
  against Jira-provided options before submitting updates.
- Add issue link management using Jira metadata/API responses for link types and valid targets.
- Add subtask creation from a selected issue using Jira create metadata for required fields.
- Add comment editing with explicit confirmation and clear failure feedback.
- Add assignment flow with Jira assignable-user lookup/search, account ID handling,
  disambiguation, and permission-aware errors.
- Worklog support.
- Sprint/board views.
- Saved filters.
- Multi-site Jira profiles.
- Packaging/install story: Homebrew tap, release binaries, or `go install`.

## Later: Security And Auth

- Add browser-based Jira OAuth or device authorization flow so users can authenticate without
  storing long-lived API tokens directly in the config file.
- Store OAuth credentials in the OS keychain or another secure credential store, with config holding
  only non-secret profile metadata.
- Keep API token auth as the current fallback until OAuth scopes, callback handling, token refresh,
  and enterprise Jira compatibility are designed intentionally.

## Later: Git And AI Workflows

- Add git integration for opening a branch from the selected or assigned ticket with configurable
  branch naming.
- Detect the current git branch and related ticket key so Jira actions can attach to the right
  issue intentionally.
- Add PR helper workflow that drafts PR titles and bodies from ticket context and local git state.
- Add Jira update helpers for branch/PR links, ticket comments, and status suggestions with explicit
  confirmation before every write.
- Add AI-assisted ticket summaries, implementation notes, PR prep, and comment drafts with visible
  source context and user approval before posting or changing Jira.
- Keep git provider integrations isolated behind internal boundaries so GitHub/GitLab/Bitbucket
  support can be added without rewriting Jira UI flows.

## Questions

- Should this optimize first for personal assigned work, team triage, or project/release management?
- Should commands be modal inside one TUI, or should we also expose subcommands like `jira issue ABC-123`?
