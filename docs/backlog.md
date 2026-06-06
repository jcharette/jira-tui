# Backlog

Use this as the source of truth for pending work. Keep items short and move finished work to the
changelog when it lands.

Docs are part of done. When an item here is completed, remove or move it and add a matching entry
to [releases/CHANGELOG.md](releases/CHANGELOG.md).

## Now

- Follow [working-agreement.md](working-agreement.md) for every future code/doc change.
- Use [roadmap.md](roadmap.md) as the milestone source of truth.
- Add an issue detail view for the selected issue.
- Fetch richer issue fields: description, priority, labels, components, fix versions, created date, updated date.
- Add tests around TUI navigation and rendering.
- Add incremental loading strategy for issue details, comments, and sprint data.

## Next

- Add a command/search input for changing JQL without restarting.
- Add issue comments view.
- Add comment creation.
- Add issue transition support: view available transitions and move issue status.
- Add assignment support.
- Add epic and subtask support.
- Add sprint and board support.
- Add open-in-browser command for the selected issue.
- Add configuration file support for saved profiles and default queries.
- Add lightweight caching for issue detail and related data.
- Add bounded concurrency controls for detail/comment/sprint fetches when parallel loading expands.

## Later

- Create issue flow.
- Edit issue fields.
- Worklog support.
- Sprint/board views.
- Saved filters.
- Multi-site Jira profiles.
- Packaging/install story: Homebrew tap, release binaries, or `go install`.

## Questions

- Should this optimize first for personal assigned work, team triage, or project/release management?
- Should commands be modal inside one TUI, or should we also expose subcommands like `jira-tui issue ABC-123`?
