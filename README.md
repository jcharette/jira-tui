# Jira

A terminal-first Jira client for people who do not want to live in the Jira web UI.

The current app authenticates with Jira Cloud, loads saved JQL views, and browses matching issues in
a Bubble Tea table with a selected-issue detail pane.

## Install

Download a release archive from
[GitHub Releases](https://github.com/jcharette/jira-tui/releases), unpack it, and move the `jira`
binary somewhere on your `PATH`.

Example for Apple Silicon:

```bash
curl -LO https://github.com/jcharette/jira-tui/releases/download/v0.2.1/jira-tui_0.2.1_darwin_arm64.tar.gz
tar -xzf jira-tui_0.2.1_darwin_arm64.tar.gz
install -m 0755 jira ~/bin/jira
```

Or install with Go:

```bash
go install github.com/jcharette/jira-tui/cmd/jira@v0.2.1
```

Go installs the binary as `jira`.

From a source checkout:

```bash
go mod download
make install-user
```

## Setup

```bash
jira config
jira
```

The config command writes `~/.config/jira/config.toml` with owner-only file permissions. The
main command also opens config automatically when required settings are missing.

Required settings:

- Jira base URL, for example `https://your-domain.atlassian.net`
- Jira account email
- Jira API token
- Default Jira project key

The config editor also includes appearance colors and display symbol mode. Defaults are provided,
and saved colors/symbol settings are used by both the config editor and issue browser.

Recommended icon setup:

- `auto` tries to detect Nerd-capable terminal setups and otherwise uses colored terminal-safe
  glyphs.
- For the richest icon set, install a Nerd Font and set Symbol Mode to `nerd` in `jira config` if
  auto does not switch.
  Recommended fonts:
  - macOS/Homebrew: `brew install --cask font-jetbrains-mono-nerd-font`
  - Alternative: `brew install --cask font-meslo-lg-nerd-font`
- After installing, configure your terminal profile to use that Nerd Font for normal text and
  non-ASCII symbols, then restart the terminal and run `jira config`.

Terminal size:

- Minimum supported size: `88x24`
- Recommended size: `120x30` or larger

The issue browser starts with saved views for assigned work, created/reported work, project open
work, current sprint, and watched issues.

Selecting an issue fetches read-only detail data in the background, including description text,
reporter, creator, labels, components, fix versions, and created/updated dates when Jira returns
those fields.

## Controls

| Key | Action |
| --- | --- |
| `j`, `down` | Move down |
| `k`, `up` | Move up |
| `pgdn`, `space`, `ctrl+f` | Page down |
| `pgup`, `ctrl+b` | Page up |
| `g`, `home` | First issue |
| `G`, `end` | Last issue |
| `o` | Next table sort |
| `O` | Previous table sort |
| `tab`, `]` | Next view |
| `shift+tab`, `[` | Previous view |
| `enter` | Open ticket detail |
| `x` | Load open child issues for the selected parent |
| `X` | Load all child issues for the selected parent |
| `j`, `k`, `pgup`, `pgdn`, `g`, `G` in detail | Scroll ticket detail |
| `esc` | Return to table from detail |
| `r` | Refresh issues |
| `q`, `ctrl+c` | Quit |

## Commands

```bash
jira
jira config
```

## Build

```bash
make build
```

For a local binary:

```bash
make build-local
```

To update the everyday binary in `~/bin`:

```bash
make install-user
```

Both build targets produce a binary named `jira`.

## Development

```bash
make check
```

Docs workflow:

```bash
make docs-status
make milestone-complete M=M1
make release VERSION=0.1.0
```

## Project Memory

Planning, backlog, release notes, and decisions live in [docs/README.md](docs/README.md).
The project working agreement in [docs/working-agreement.md](docs/working-agreement.md) makes
doc updates part of the definition of done.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
