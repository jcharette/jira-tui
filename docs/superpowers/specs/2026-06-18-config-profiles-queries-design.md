# Config Profiles And Default Queries

## Goal

Complete the Navigation/Query backlog slice for saved profiles and default queries by making the
existing TOML profile shape durable and selectable at runtime.

## Scope

- Preserve multiple `[profiles.<name>]` entries when loading and saving config.
- Track the active profile name in `config.Config`.
- Allow `jira --profile <name>` and `jira config --profile <name>` to select a saved profile.
- Keep default project/default JQL behavior under `[queries]`, including generated default JQL when
  a project is configured and no explicit JQL is provided.
- Keep the config editor focused on the active profile. It may edit the active profile name and
  credentials, but it will not add a full multi-profile management surface in this slice.

## Non-Goals

- Do not add OAuth or keychain-backed credential storage.
- Do not add a visual profile switcher in the main TUI.
- Do not add per-profile saved views or per-profile query sets yet; those need a broader multi-site
  workspace design.
- Do not change Jira IO paths or worker behavior.

## Design

`internal/config.Config` becomes the durable in-memory representation for the selected profile plus
the full saved profile map. `LoadOptions` gains `Profile string`; when set, the loader uses that
profile for account credentials while leaving the saved `active_profile` unchanged. When omitted,
the loader uses `active_profile`, falling back to `default`.

`Save` writes all known profiles and updates the selected active profile with the flat account fields
currently edited by the config editor. This prevents editing default queries or saving a generated
view from deleting non-active profiles.

The CLI adds a persistent `--profile` flag. App startup, config editing, and saved-view writes all
flow through the selected profile context. Running the main app with `--profile` is temporary;
running the config editor can persist the selected `Active Profile` field when the user saves. The
cache namespace should include the base URL and active profile name so profiles on the same Jira site
do not collide.

## Testing

- Config tests cover loading a named profile override, rejecting unknown profile overrides, and
  preserving non-active profiles across save/load.
- Config UI tests cover active profile field round-trip.
- App command tests cover the `--profile` flag being accepted on root and config commands.
- Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.
