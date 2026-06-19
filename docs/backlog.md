# Backlog

Use this as the source of truth for pending work. Keep items short and move finished work to the
changelog when it lands.

Docs are part of done. When an item here is completed, remove or move it and add a matching entry
to [releases/CHANGELOG.md](releases/CHANGELOG.md).

## Next: Creation And Editing

- Add remaining metadata-backed implementations beyond Labels and Components through the Ticket
  Actions palette only as their underlying workflows land.
- Extend generic edit issue support beyond safe custom text/number/date and inline option fields:
  add field-specific workflows for users, versions, sprints, autocomplete-only options, and other
  complex Jira schemas as real usage demands them.
- Add issue link removal/editing after parsed issue-link detail retains Jira link IDs.
- Worklog edit/delete support after add/list usage has proven the right terminal UX.
- Sprint/board views.
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

## Maybe Later

- Multi-site Jira profiles, if real usage shows people regularly need to switch between unrelated
  Jira tenants from the same install.

## Questions

- Should this optimize first for personal assigned work, team triage, or project/release management?
- Should commands be modal inside one TUI, or should we also expose subcommands like `jira issue ABC-123`?
