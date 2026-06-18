# Linked Issue Links Design

## Scope

Complete the next bounded `Now: Read And View` slice by showing Jira issue links in the existing
focused ticket detail Links workspace. The Links section should combine Jira issue links from issue
detail with detected URLs/email addresses from the description, while keeping the current keyboard
model: `l` focuses Links, `j/k` selects, `enter` or `o` opens, and `y` copies.

## Findings

The current Links section only renders links detected in the issue description. Jira issue detail
already includes the raw `issuelinks` field through `go-atlassian`, but `internal/jira.IssueDetail`
does not expose it. The hierarchy workspace already has worker-backed child expansion, so this slice
should not add another hierarchy read path.

## Design

Add a small `jira.IssueLink` domain type and parse raw inward/outward Jira issue links when issue
detail is parsed. Each parsed link should include direction text, relationship text, linked issue
key, summary, status, issue type, and URL.

In the TUI, extend the existing `detailLink` representation so Jira issue links render as first-class
rows in the Links section before detected description links. Opening an issue-link row opens the
linked issue URL; copying copies the issue key. Description links keep the existing open/copy
behavior.

## Non-Goals

Do not add issue-link editing, link-type metadata discovery, a new worker family, or Jira writes. Do
not change saved views, JQL, hierarchy expansion behavior, or Actions menu behavior.

## Verification

Use TDD for the Jira parser and TUI rendering/actions. Then run focused Jira/TUI tests,
`go test ./internal/jira ./internal/tui -count=1`, `go test ./... -count=1`, `make check`, and
`make install-user`.
