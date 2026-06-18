# Backlog

Use this as the source of truth for pending work. Keep items short and move finished work to the
changelog when it lands.

Docs are part of done. When an item here is completed, remove or move it and add a matching entry
to [releases/CHANGELOG.md](releases/CHANGELOG.md).

## Next: Navigation And Query

- Add bounded concurrency controls for sprint and board fetches when parallel loading expands beyond
  the current issue/detail/comment/cache worker paths.

## Later: Creation And Editing

- Extend Jira metadata discovery adapters beyond the current create/edit metadata foundation:
  assignable user search by project/issue and field options.
- Expand transition field handling beyond the current Resolution and Comment support to additional
  Jira field schemas when real workflow screens require them.
- Add a ticket action menu/command palette for edit workflows so growing actions do not overload
  single-key detail bindings.
- Add remaining metadata-backed implementations behind the Actions tab only as their underlying
  workflows land.
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
