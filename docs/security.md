# Security Overview

This document summarizes how `jira-tui` handles authentication, local data, external integrations,
and reviewer-relevant security controls.

## Scope

`jira-tui` is a local terminal application for Jira Cloud. It runs on the user's machine, talks to
Jira over HTTPS, and stores app state under the user's OS config/cache directories. It does not run a
server, host a web UI, or operate a project-owned authentication backend.

## Authentication

The supported Jira authentication path is Jira Cloud email plus Jira API token.

- Jira base URLs must use `https://`; plaintext `http://` URLs are rejected before Basic Auth is
  configured.
- Saved Jira API tokens are stored in the OS secret store through `github.com/zalando/go-keyring`.
- macOS uses Keychain, Windows uses Credential Manager, and Linux uses Secret Service.
- The TOML config file stores account metadata and an `api_token_source = "keyring"` reference, not
  the saved token value.
- Existing plaintext `api_token` configs still load for migration compatibility and are migrated to
  keyring storage the next time config is saved.
- OAuth/device authorization is intentionally not implemented. See
  [ADR 0010](decisions/0010-api-token-auth-over-oauth.md).

Users can revoke Jira API tokens from their Atlassian account settings. If a token is revoked or
deleted from the OS keychain, `jira-tui` requires the user to re-enter credentials through
`jira config`.

## Config Storage

Default config path:

```text
~/.config/jira/config.toml
```

Security controls:

- The config directory is created with `0700` permissions.
- The config file is written with `0600` permissions.
- The config file contains non-secret settings such as Jira base URL, email, default project,
  default Agile board ID, saved views, runtime settings, notification settings, and keyring
  token-source metadata.
- API tokens are not written to config after keyring migration.

## Cache Storage

Default cache path:

```text
<OS user cache dir>/jira-tui/cache.sqlite
```

The SQLite cache improves startup and navigation responsiveness. It can contain Jira business data
the user has permission to view, including:

- active view issue rows,
- issue details and descriptions,
- comments,
- transitions and edit metadata,
- create metadata,
- expanded child issue rows,
- recent query history and AI-generated query prompts.

Security controls:

- The cache directory is created with `0700` permissions.
- `cache.sqlite` is created with `0600` permissions.
- SQLite sidecar files, when present, are chmodded to `0600`.
- Old cache rows are cleaned up conservatively by age.
- The cache is local-only and is not uploaded by the app.

Security teams should treat the cache as local sensitive Jira data at rest. The current design uses
OS user isolation and owner-only permissions, not database encryption.

## Diagnostics And Bug Reports

Diagnostics exist to explain background activity without exposing raw Jira payloads.

Default persistent diagnostics path:

```text
<OS user cache dir>/jira-tui/diagnostics.jsonl
```

Diagnostics may include:

- worker request IDs,
- operation families,
- issue/project scope identifiers,
- result class,
- counts and timing,
- categorized errors,
- cache state summaries.

Diagnostics are designed not to include:

- Jira API tokens,
- raw request bodies,
- raw response bodies,
- descriptions,
- comments,
- full JQL strings.

Security controls:

- The diagnostics directory is created with `0700` permissions.
- The diagnostics log is written with `0600` permissions.
- The persistent log is size-bounded and rotated.
- Persistent Diagnostics and bug-report excerpts apply final redaction for token/password/secret-style
  key-value fields before writing or exporting text.
- Bug-report Diagnostics excerpts are explicit opt-in every time.
- Bug reports open a prefilled GitHub issue URL in the user's browser; the app does not store GitHub
  credentials or upload raw local log files.

## Notifications

In-app notifications are local process memory and are kept visible until the user clears them.

Optional system notifications use `github.com/gen2brain/beeep`. When enabled, the OS desktop
notification system may receive ticket keys, summaries, and changed field names. This can make Jira
content visible outside the terminal, including in OS notification history or lock-screen surfaces
depending on user settings.

Security-sensitive environments can leave system notifications disabled while still using the
in-app notification center.

## External Integrations

`jira-tui` can invoke local OS tools for user-requested actions:

- browser opening through `open`, `xdg-open`, `rundll32`, or equivalent,
- clipboard copy through platform clipboard tools,
- optional local Claude CLI execution when AI features are enabled,
- Git and GitHub CLI workflows for explicit `jira start`, `jira commit`, and `jira finish` actions.

Relevant controls:

- External browser and clipboard calls use argument-based process execution, not shell string
  interpolation.
- Git operations are routed through the `internal/gitworkflow` adapter boundary.
- GitHub PR operations are routed through the `internal/prprovider` boundary.
- Claude/AI features default disabled, require local CLI availability, and keep write gates closed
  unless explicitly enabled.
- Jira, Git, GitHub, and local write workflows present review/confirmation steps before writes.

## Network Behavior

Primary network traffic goes to the configured Jira Cloud site over HTTPS. OAuth is not used, so
normal API-token requests are made against the configured Jira base URL.

Release and support workflows may involve GitHub only when the user explicitly opens a bug report,
uses GitHub-backed finish workflows, or installs/downloads releases.

## Data Removal

Users can remove local app data with normal OS file/keychain controls:

- Delete `~/.config/jira/config.toml` to remove non-secret config.
- Delete the `jira-tui` directory under the OS user cache directory to remove cache and diagnostics.
- Delete `jira-tui` entries from the OS keychain/credential store to remove saved Jira API tokens.

## Security Review Summary

The main residual local data risk is that cache files intentionally contain Jira content for
responsiveness. The app mitigates that with owner-only files and bounded retention, but does not
encrypt the SQLite database. For most local developer-machine use, the intended security boundary is
the user's OS account plus OS keychain protection.
