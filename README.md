# Jira TUI

A terminal-first Jira client for engineers who want Jira to feel like part of their development
workflow instead of a browser chore.

`jira-tui` gives you fast saved views, status-oriented issue lanes, focused ticket detail, comments,
workflow transitions, ticket creation, metadata-backed edits, worklogs, issue links, and sanitized
diagnostics from one keyboard-driven TUI. It is built for Jira Cloud and keeps Jira reads/writes on
bounded background workers so the interface stays responsive while data loads.

## Highlights

- **Issue views that stay useful:** saved JQL views, direct JQL, AI-assisted JQL generation, recent
  query history, saved-view creation, and local active-ticket filtering without rewriting your Jira
  queries.
- **Multiple ways to scan work:** Lanes is the default status-grouped view, with Table and Workbench
  layouts available through `L`. Loaded parent/child issue trees can be expanded, collapsed, and
  navigated without changing the saved view.
- **Focused ticket detail:** tickets open into a compact Overview with comments, hierarchy, links,
  summary, status, assignee, priority, and actions organized around the work you are likely to do
  next.
- **Real Jira edits:** create tickets and subtasks, edit summary/priority/assignee/labels/components,
  apply status transitions with required fields, add/edit comments, create/remove issue links, and
  add/edit/delete worklogs through worker-backed Jira requests.
- **Rich comments and readable details:** Jira ADF descriptions and comments render in the terminal,
  links are detected, comments support basic formatting controls, and Jira mentions use account IDs
  selected through search.
- **Operationally debuggable:** `ctrl+d` opens Diagnostics, and `B` opens an in-app GitHub bug report
  composer with an opt-in sanitized Diagnostics excerpt. Tokens, raw request/response bodies,
  descriptions, comments, and full JQL are not included.
- **Practical install story:** release archives, `go install`, source installs, and a repo-managed
  Homebrew formula are all documented.

## Install

Homebrew:

```bash
brew install --formula https://raw.githubusercontent.com/jcharette/jira-tui/main/Formula/jira-tui.rb
```

Download a release archive from
[GitHub Releases](https://github.com/jcharette/jira-tui/releases), unpack it, and move the `jira`
binary somewhere on your `PATH`.

Apple Silicon example:

```bash
curl -LO https://github.com/jcharette/jira-tui/releases/download/v0.2.2/jira-tui_0.2.2_darwin_arm64.tar.gz
tar -xzf jira-tui_0.2.2_darwin_arm64.tar.gz
install -m 0755 jira ~/bin/jira
```

Or install with Go:

```bash
go install github.com/jcharette/jira-tui/cmd/jira@v0.2.2
```

Go installs the binary as `jira`.

From a source checkout:

```bash
go mod download
make install-user
```

More install options are documented in [docs/install.md](docs/install.md).

## Setup

```bash
jira config
jira
```

The config command writes `~/.config/jira/config.toml` with owner-only file permissions. The main
command also opens config automatically when required settings are missing.

Required settings:

- Jira base URL, for example `https://your-domain.atlassian.net`
- Jira account email
- Jira API token
- Default Jira project key

The config editor also includes saved views, profiles, runtime settings, appearance colors, and
display symbol mode.

## Recommended Terminal Setup

- Minimum supported size: `88x24`
- Recommended size: `120x30` or larger
- Default Symbol Mode: `auto`

`auto` tries to detect Nerd-capable terminal setups and otherwise uses colored terminal-safe glyphs.
For the richest icon set, install a Nerd Font and set Symbol Mode to `nerd` in `jira config` if auto
does not switch.

Recommended fonts:

```bash
brew install --cask font-jetbrains-mono-nerd-font
```

Alternative:

```bash
brew install --cask font-meslo-lg-nerd-font
```

After installing, configure your terminal profile to use that Nerd Font for normal text and
non-ASCII symbols, then restart the terminal and run `jira config`.

## Core Workflows

### Browse And Triage

- Saved views for assigned work, created/reported work, project open work, current sprint, watched
  issues, and epics.
- `L` switches local layouts: Lanes, Table, Workbench, and Planning where sprint metadata is useful.
- `f` toggles the local active-ticket filter without changing JQL or cached issue data.
- `x` loads open children for the focused parent; `X` loads all children, including resolved work.
- `z` collapses or expands loaded descendants locally.
- `o`/`O` cycles local sort order while preserving parent/child grouping where possible.

### Query And Views

- `/` opens direct JQL and AI-assisted JQL generation.
- Generated JQL is previewed before it changes the active query.
- Recent direct and AI-generated queries can be rerun or copied back into the editor.
- `v` saves the current query as a named view from inside the app.
- Saved views can be renamed, reordered, deleted, and configured for automatic child loading.

### Read And Act On Tickets

- `enter` opens focused ticket detail with Overview, Comments, Hierarchy, Links, and Claude/AI when
  enabled.
- Comments load through the worker pool and render Jira ADF in the terminal.
- Links include Jira issue links, detected URLs, email addresses, and copy/open actions.
- Ticket Actions (`.`) gives a searchable command surface for edits, comments, status, assignment,
  links, subtasks, and worklogs.

### Create And Edit

- `n` opens Jira-backed ticket creation with issue type, Summary, Description, and supported
  metadata fields.
- Ticket detail supports summary, priority, assignee, labels, components, safe generic custom fields,
  and workflow transitions through Jira edit metadata.
- Transition screens support required Resolution, Comment, text/date fields, user pickers,
  multi-select fields, and autocomplete-backed options when Jira metadata supplies safe values.
- Subtask creation reuses the same Jira metadata-backed create flow with the current ticket as
  parent.

### Comments, Links, And Worklogs

- Add and edit Jira comments from ticket detail.
- Comment composition supports bold, italic, inline code, bullets, detected links, and selected Jira
  mentions.
- Create and remove Jira issue links using Jira link types.
- Add, edit, and delete Jira worklogs from the Worklog section.

### Diagnostics And Bug Reports

- `ctrl+d` opens Diagnostics with bounded worker/API/cache/state activity.
- Diagnostics are mirrored to a bounded persistent JSONL log under the OS user cache directory.
- `B` opens a GitHub bug report composer from the app.
- Bug reports can include a sanitized excerpt only when the user opts in.

## Controls

| Key | Action |
| --- | --- |
| `j`, `k`, arrows | Move selection or focused picker |
| `pgdn`, `space`, `ctrl+f` | Page down |
| `pgup`, `ctrl+b` | Page up |
| `g`, `G` | First or last row |
| `tab`, `shift+tab` | Switch views or move ticket-detail focus |
| `enter` | Open or activate the focused item |
| `esc` | Back or cancel |
| `r` | Refresh issues |
| `L` | Switch issue-list layout |
| `f` | Toggle active-ticket filter |
| `/` | Query modal |
| `v` | Save current query as a view |
| `n` | Create ticket |
| `.` | Ticket Actions |
| `x`, `X` | Load child issues |
| `z` | Collapse or expand loaded descendants |
| `o`, `O` | Sort forward or backward |
| `ctrl+d` | Diagnostics |
| `B` | Report a bug |
| `?` | Contextual keyboard help |
| `q`, `ctrl+c` | Quit |

The footer shows the most relevant commands for the active screen. Press `?` for the full contextual
keymap.

## Commands

```bash
jira
jira config
jira --profile work
jira --profile work config
```

## Development

```bash
make check
make install-user
```

Useful targets:

```bash
make build
make build-local
make docs-status
make milestone-complete M=M1
make release VERSION=0.2.2
```

Planning, backlog, release notes, and decisions live in [docs/README.md](docs/README.md). The project
working agreement in [docs/working-agreement.md](docs/working-agreement.md) makes doc updates part of
the definition of done.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
