# Jira TUI

A terminal-first Jira client for people who do not want to live in the Jira web UI.

The first pass is intentionally narrow: authenticate with Jira Cloud, run a JQL query, and browse
matching issues in a Bubble Tea interface.

## Setup

```bash
go mod download
go run ./cmd/jira-tui
```

Set Jira credentials:

```bash
export JIRA_BASE_URL="https://your-domain.atlassian.net"
export JIRA_EMAIL="you@example.com"
export JIRA_API_TOKEN="your-api-token"
```

Optional:

```bash
export JIRA_JQL="assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC"
export JIRA_REFRESH_INTERVAL="2m"
export JIRA_REQUEST_TIMEOUT="20s"
export JIRA_WORKERS="2"
export JIRA_QUEUE_SIZE="16"
```

## Controls

| Key | Action |
| --- | --- |
| `j`, `down` | Move down |
| `k`, `up` | Move up |
| `r` | Refresh issues |
| `q`, `ctrl+c` | Quit |

## Build

```bash
make build
```

For a local binary:

```bash
make build-local
```

## Development

```bash
make check
```

## Project Memory

Planning, backlog, release notes, and decisions live in [docs/README.md](docs/README.md).
The project working agreement in [docs/working-agreement.md](docs/working-agreement.md) makes
doc updates part of the definition of done.
