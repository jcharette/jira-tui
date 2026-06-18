# TUI Navigation And Rendering Test Coverage Design

## Scope

Complete the broad `Now: Read And View` backlog item for TUI navigation and rendering tests by
adding focused regression coverage around existing issue-list and detail behavior. This is a
test-hardening slice, not a UI behavior change.

## Approach

Keep tests in the existing `internal/tui` package and near the workflows they protect. Add
issue-list coverage in `issue_list_test.go` for table navigation plus rendered viewport behavior.
Add detail coverage in `detail_test.go` for returning from detail mode and keeping context-specific
footer/rendering stable.

## Non-Goals

Do not add new navigation keys, new rendering components, Jira reads, or worker flows. Do not create
a new TUI package; the package boundary audit recommends same-package workflow files for this work.

## Verification

Run focused `go test ./internal/tui -count=1`, then the normal project gate: `go test ./... -count=1`,
`make check`, and `make install-user`.
