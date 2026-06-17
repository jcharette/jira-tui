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
- Continue cache and Diagnostics polish only when new Jira read families are added. The current
  cache foundation uses maintained `ttlcache`, the bounded worker scheduler, and private SQLite
  persistence for active views, issue detail, comments, transitions, edit metadata, create metadata,
  and expanded children. Diagnostics now shows queue state, cache family fresh/stale/error counts,
  and sanitized API debug rows with operation family, request ID, issue/project scope, result class,
  result counts, timing, and safe error categories.
- Improve epic/subtask table grouping beyond current-result grouping by explicitly loading related
  children when the selected view needs them.

## Next: Navigation And Query

- Add epic and subtask support.
- Add sprint and board support.
- Add configuration file support for saved profiles and default queries.
- Add bounded concurrency controls for sprint and board fetches when parallel loading expands beyond
  the current issue/detail/comment/cache worker paths.

## Next: Comments And Workflow Actions

- Add transition field metadata support for transitions that require extra fields before moving
  issue status.

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
- Homebrew tap/formula after the release binary and `go install` paths are stable.

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
- Route new AI features through the provider-neutral `ai.task.*` event boundary so Claude, Codex,
  and future `auto` routing stay behind one command path.
- Keep git provider integrations isolated behind internal boundaries so GitHub/GitLab/Bitbucket
  support can be added without rewriting Jira UI flows.

## Questions

- Should this optimize first for personal assigned work, team triage, or project/release management?
- Should commands be modal inside one TUI, or should we also expose subcommands like `jira issue ABC-123`?
