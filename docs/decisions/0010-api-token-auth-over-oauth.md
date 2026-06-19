# 0010. Keep Jira API Token Authentication As The Supported Path

Date: 2026-06-19

## Status

Accepted

## Context

`jira-tui` authenticates to Jira Cloud with email plus Jira API token. Tokens are stored in the OS
keychain, config files keep only non-secret account metadata and keyring references, config files are
owner-only, Jira base URLs must use HTTPS, and local cache/Diagnostics storage has been hardened.

We evaluated adding Jira OAuth or device authorization for users who prefer not to create an API
token. Atlassian Jira Cloud supports OAuth 2.0 3LO authorization code flow, but public docs did not
show a Jira Cloud device authorization flow. The 3LO path requires OAuth client credentials and routes
REST calls through `api.atlassian.com/ex/jira/{cloudid}` instead of the tenant base URL.

For a distributed CLI/TUI binary, OAuth therefore requires one of two heavier models:

- Each user creates their own Atlassian developer app and stores client credentials locally.
- The project operates a hosted broker/backend that holds client credentials and exchanges tokens.

Both are more complex than asking a user to create a Jira API token, especially now that tokens are
stored in the OS keychain rather than plaintext config.

## Decision

Do not implement Jira OAuth or device authorization for now.

Keep API-token auth as the supported authentication path. Revisit OAuth only if Atlassian offers a
CLI-friendly device authorization flow, or if real users require OAuth/SSO because API tokens are
blocked by organization policy.

## Consequences

- `jira config` remains simpler: Jira base URL, email, API token, and default project.
- No project-hosted auth backend is required.
- No OAuth client secret is embedded in the app binary.
- API token fallback does not need to coexist with a second request-routing model.
- Security posture relies on OS keychain storage, HTTPS-only Jira URLs, owner-only config/cache
  files, and documented token creation/revocation.
