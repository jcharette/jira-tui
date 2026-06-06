# Working Agreement

This project uses docs as persistent memory. A change is not done until the relevant docs are updated.

## Before Starting Work

Read these files first:

1. [project-state.md](project-state.md)
2. [planning.md](planning.md)
3. [backlog.md](backlog.md)
4. [releases/CHANGELOG.md](releases/CHANGELOG.md)

Use them to avoid rediscovering previous decisions or repeating abandoned work.

## During Work

Keep changes aligned with the current milestone in [planning.md](planning.md).

If the implementation reveals a new task, add it to [backlog.md](backlog.md) before finishing.

If a decision would change how future work should be done, add a decision record under
[decisions/](decisions).

Before implementing non-core infrastructure, check for well-maintained third-party libraries and
prefer wrapping them behind internal boundaries when they fit.

## Definition Of Done

Before calling work complete:

- Code is implemented and formatted.
- Relevant tests pass, or the reason they could not run is recorded in the final note.
- [project-state.md](project-state.md) is updated if commands, env vars, architecture, behavior, or constraints changed.
- [backlog.md](backlog.md) is updated if work was completed, added, removed, or reprioritized.
- [planning.md](planning.md) is updated if milestone sequencing or near-term direction changed.
- [releases/CHANGELOG.md](releases/CHANGELOG.md) has an `Unreleased` entry for user-visible changes.
- A decision record is added for durable architecture/product choices.
- New infrastructure work either uses an appropriate maintained library or records why custom code
  is the better fit.

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

Move completed work out of the active backlog sections and describe it in the changelog.
