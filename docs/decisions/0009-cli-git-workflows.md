# 0009: Use CLI-First Git Workflows With A Shared TUI Workflow Surface

Date: 2026-06-19

## Status

Accepted

## Context

The app already supports rich Jira browsing and write workflows inside the TUI. The next power-user
slice needs to connect Jira tickets to local development work without forcing every action through a
full-screen app session.

The target workflows are:

- `jira start [ticket]` to choose or confirm a ticket, choose a local repo, create/switch to a
  ticket branch, and optionally update Jira.
- `jira commit` to create a commit or reconcile local commits made outside the tool, then optionally
  write a compact Jira update.
- `jira finish` to prepare final commits, push, open a pull request, and optionally transition Jira
  with a concise final note.
- An in-app Start action from the selected ticket that follows the same workflow as
  `jira start <ticket>`.

## Decision

Use a hybrid command model:

- `jira start`, `jira commit`, and `jira finish` are first-class CLI commands.
- `jira start` without a ticket opens a focused picker for the user's tickets, not the full app.
- The TUI can launch Start from a selected ticket.
- CLI and TUI Start launch points share one Bubble Tea workflow component so validation, previews,
  confirmation, cancellation, and error handling stay consistent.
- Commit and Finish are CLI-first for now because they are repo/branch workflows. In-app versions are
  deferred until the app has a broader workspace model.

## Start Workflow

Start prefers the current git repository when one is available, then recent local repositories, then
an explicit repo picker. Repo choices are stored locally and are not written into Jira. This matters
because a Jira project can map to many repos.

Branch naming is configurable in the app config. The default template is:

```text
{key}-{summary_slug}
```

The user sees and can edit the final branch name before any write happens.

Before writing, Start shows a review screen. The default write set is conservative:

- create or switch to the ticket branch;
- assign the ticket to the current Jira user when available;
- transition to the best available "in progress" status when Jira metadata supports it;
- add a short Jira comment with the branch/work link when available.

Every external write is confirmable and skippable.

## Commit Workflow

`jira commit` is an actual commit workflow, not just a Jira note helper.

It detects the ticket from the branch name or explicit context, compares local branch commits against
the upstream/base branch, and separates:

- uncommitted changes;
- local commits that were already reported to Jira;
- local commits that have not been reported yet.

The app stores reported commit SHAs in local state keyed by repo, branch, and ticket. Jira is not
parsed as the primary source of truth for reported commits. If users made commits outside the tool,
the workflow can still offer to summarize and report them.

AI assistance is optional and must produce compact implementation notes. Jira updates should be
clean, bounded, and reviewable, not long generated narratives.

## Finish Workflow

`jira finish` detects the current ticket and repo state, inspects Jira transition metadata, and
offers the best terminal transition rather than hard-coding a status name.

Finish can offer AI help for:

- pull request title and body;
- final Jira completion note;
- commit message cleanup when local changes still need a commit.

The final Jira note may be more of an overall synopsis than a commit log, but it must remain bounded
and reviewed before posting.

GitHub is the first pull-request provider. Provider-specific code must sit behind an interface so
GitLab, Bitbucket, or other providers can be added later without leaking provider details into the
workflow UI.

## Linking Work

V1 links work by:

- including the Jira key in branch names, commits, and pull requests;
- posting short Jira comments with branch or pull-request URLs after explicit confirmation.

Full Jira development-panel integration is intentionally deferred because it is brittle and not
needed for the first useful version.

## Cancellation And Recovery

Before confirmation, cancellation has no side effects.

After partial writes, workflows show a completed/skipped/failed summary. Local workflow state should
prevent duplicate Jira commit notes when a user retries or finishes later.

## Consequences

- The public backlog can split the work into Start, Commit, Finish, and shared AI support issues.
- The CLI package needs a command surface that can launch focused Bubble Tea workflows.
- Git operations, Jira writes, provider writes, and AI output all need review screens before they
  mutate external systems.
- The TUI should not grow separate custom Start behavior; it should call the same workflow path as
  the CLI.
