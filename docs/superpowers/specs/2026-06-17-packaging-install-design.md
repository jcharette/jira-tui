# Packaging Install Design

## Scope

This slice completes the easy install story without building a Homebrew tap. Users should be able
to install from GitHub release binaries, install from source with the existing Makefile, or use a
versioned `go install` command that creates a `jira` executable.

## Approach

The current command package lives at `cmd/jira-tui`, so `go install .../cmd/jira-tui@<version>`
would install a binary named `jira-tui` even though the app and Makefile build a binary named
`jira`. The least surprising fix is to add `cmd/jira` as the package intended for Go installs.
To avoid duplicating Cobra and startup code, the current command wiring moves into `internal/app`;
both command packages call the same `app.Execute()` function.

The current module path is stale: `github.com/jon/jira-tui` does not resolve, while the repository
remote and existing tags are under `github.com/jcharette/jira-tui`. The installable module path
must match the actual remote so `go install github.com/jcharette/jira-tui/cmd/jira@<version>` can
download tagged releases.

## Non-Goals

Homebrew remains a follow-up. The docs should mention it only as future work, because publishing a
tap or formula should wait until the release binary and `go install` paths are stable.

## Verification

Verify the module path with `go list -m -versions github.com/jcharette/jira-tui`. Verify the new
command package with focused Go tests and a direct `go build ./cmd/jira`, then run the standard
project verification loop: `go test ./... -count=1`, `make check`, and `make install-user`. A patch
release should be cut after merging because versioned `go install` only works when the target
package exists in a tagged module version.
