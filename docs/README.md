# Project Docs

This folder is the working memory for Jira TUI. Read this first before planning or changing code.

## Start Here

- [project-state.md](project-state.md): what exists right now, current assumptions, and known constraints
- [roadmap.md](roadmap.md): feature inventory, dependencies, milestones, and implementation sequence
- [backlog.md](backlog.md): work that needs to be done
- [planning.md](planning.md): near-term implementation plan and sequencing
- [working-agreement.md](working-agreement.md): how we keep docs current as part of done work
- [releases/CHANGELOG.md](releases/CHANGELOG.md): release/change history
- [decisions/](decisions): architecture and product decisions we do not want to re-litigate

## Update Rules

Docs are part of the definition of done. Follow [working-agreement.md](working-agreement.md)
for every change.

Update these docs when a change affects future work:

- Add or move tasks in [backlog.md](backlog.md).
- Record completed user-visible changes in [releases/CHANGELOG.md](releases/CHANGELOG.md).
- Update [project-state.md](project-state.md) when the architecture, commands, env vars, or constraints change.
- Add a decision record when we choose a direction that future work should preserve.

## Completion Checklist

Before finishing a task, check whether the change requires updates to:

- [project-state.md](project-state.md)
- [backlog.md](backlog.md)
- [planning.md](planning.md)
- [releases/CHANGELOG.md](releases/CHANGELOG.md)
- [decisions/](decisions)
