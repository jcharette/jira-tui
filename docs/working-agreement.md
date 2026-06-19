# Working Agreement

This project uses docs as persistent memory. A change is not done until the relevant docs are updated.

## Before Starting Work

Read these files first:

1. [project-state.md](project-state.md)
2. [security.md](security.md)
3. [backlog.md](backlog.md)
4. [releases/CHANGELOG.md](releases/CHANGELOG.md)

Use them to avoid rediscovering previous decisions or repeating abandoned work.

## During Work

Keep changes aligned with the current direction in [roadmap.md](roadmap.md).

If the implementation reveals a user-visible backlog task, create or update the matching GitHub
issue before finishing, then keep [backlog.md](backlog.md) as the local curated index. Use local docs
only for architecture context, implementation plans, lessons, and handoff notes that should stay
close to the codebase.

If a decision would change how future work should be done, add a decision record under
[decisions/](decisions).

Before implementing non-core infrastructure, check for well-maintained third-party libraries and
prefer wrapping them behind internal boundaries when they fit.

For Jira write workflows, do not assume a generic Jira shape. Issue creation, editing, transitions,
assignment, sprint movement, priorities, statuses, issue types, required fields, field options,
workflow actions, user identifiers, and custom fields must be discovered from Jira metadata for the
configured site/project/issue before building forms or submitting updates. If the metadata endpoint
or adapter method does not exist yet, add that API boundary first or record the missing discovery
work in the backlog. Do not hard-code values from one Jira instance as product behavior.

For TUI work, treat interaction clarity and visual organization as part of the feature, not polish
to defer. Before calling a TUI screen done, make sure navigation between panes/sections is visible
on screen, keybindings match the layout, and the view uses the shared styling system for color,
borders, spacing, and status emphasis. Pane layouts should respond to terminal width with explicit
breakpoints instead of relying on one fixed size. Table/list row counts must subtract all rendered
chrome: headers, query bars, help footers, borders, padding, titles, and sibling panels.

Use maintained Bubble Tea ecosystem components for standard TUI primitives by default: Bubbles
`viewport` for scrollable content, `table`/`list` for selectable collections, `help`/`key` for key
help, `textinput`/`textarea` for text entry, `paginator` for paging, and `spinner` for loading
states. Lip Gloss table/list/tree/rendering helpers should be preferred over hand-built terminal
markup. Hand-roll only when the maintained component cannot meet the interaction or rendering
requirement, and record that reason in the code or docs.

## Definition Of Done

Before calling work complete:

- Code is implemented and formatted.
- Relevant tests pass, or the reason they could not run is recorded in the final note.
- [project-state.md](project-state.md) is updated if commands, env vars, architecture, behavior, or constraints changed.
- GitHub Issues are updated if user-visible backlog work was completed, added, removed, or reprioritized.
- [backlog.md](backlog.md) is updated as the local issue index when public backlog work changes.
- [roadmap.md](roadmap.md) is updated if product direction or near-term sequencing changed.
- [releases/CHANGELOG.md](releases/CHANGELOG.md) has an `Unreleased` entry for user-visible changes.
- A decision record is added for durable architecture/product choices.
- New infrastructure work either uses an appropriate maintained library or records why custom code
  is the better fit.
- TUI primitives use Bubble Tea, Bubbles, Lip Gloss, or a maintained package built on them when one
  fits the behavior.
- TUI changes have explicit on-screen navigation hints, clear section/pane organization, and
  styled loading, empty, error, and selected states where applicable.

## Changelog Style

Use short bullets under `Unreleased`.

Prefer user-visible wording:

- Good: `Added issue detail view for selected issues.`
- Avoid: `Refactored model struct.`

Internal-only changes belong in the changelog only when they affect future maintenance.

## Backlog Style

Keep backlog items actionable. Prefer verbs:

- `Add issue comments view.`
- `Persist the last successful query.`

Keep user-visible backlog items in GitHub Issues and link them from [backlog.md](backlog.md). Move
completed work out of the local active index, close or update the GitHub issue, and describe shipped
behavior in the changelog.
