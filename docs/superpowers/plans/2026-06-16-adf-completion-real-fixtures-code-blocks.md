# ADF Completion With Real Fixtures And Compact Code Blocks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the ADF renderer work with sanitized real-fixture tooling, offline fixture tests, and compact code-block rendering.

**Architecture:** Keep `internal/adf.Render` as the plain-text boundary. Add a focused `internal/adf/fixture` package for sanitizing/writing fixture JSON and a dev command that uses existing config/Jira client paths only when manually invoked. Improve visual code-block rendering in the TUI layer by replacing ASCII borders with styled padded lines.

**Tech Stack:** Go, Bubble Tea v2, Lip Gloss, go-atlassian Jira models, Cobra, existing config/Jira client packages.

---

### Task 1: Track Work

**Files:**
- Modify: `tasks/todo.md`

- [ ] Add an `ADF Completion With Real Fixtures And Compact Code Blocks` section with checkboxes for tests, fixture helper, compact code-block rendering, docs, and verification.

### Task 2: Compact Code Block Rendering

**Files:**
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/model.go`

- [ ] Add failing tests that `renderRichDescriptionBody` removes code fences and no longer emits `+---+` or side `|` ASCII borders.
- [ ] Add failing tests that leading/trailing blank code lines are trimmed and exactly one blank separator remains before a code block.
- [ ] Replace `renderCodeBlockLines` with compact padded Lip Gloss styling using `theme.CodeBlock`, no border rows, and no side border glyphs.
- [ ] Run `go test ./internal/tui -run 'TestRenderRichDescriptionFormatsCodeBlock|TestRenderRichDescriptionDoesNotLeaveExtraBlankLinesBeforeCodeBlock' -count=1`.

### Task 3: Sanitized ADF Fixture Helper

**Files:**
- Create: `internal/adf/fixture/fixture.go`
- Create: `internal/adf/fixture/fixture_test.go`
- Modify: `internal/jira/client.go`
- Modify: `internal/jira/client_test.go`

- [ ] Add failing tests for fixture sanitization of mentions, links, inline cards, account IDs, and deterministic JSON output.
- [ ] Implement a sanitizer that preserves node types, marks, nesting, table shape, and code text shape while replacing private identifiers/URLs with stable placeholders.
- [ ] Add Jira client methods for raw ADF description and comment retrieval so capture tooling can fetch payloads without using rendered description/comment strings.
- [ ] Add client tests proving raw description/comment ADF is returned without rendering.
- [ ] Run `go test ./internal/adf/fixture ./internal/jira -count=1`.

### Task 4: Dev Fixture Capture Command

**Files:**
- Create: `cmd/adf-fixture/main.go`

- [ ] Add a dev-only command that can sanitize a local raw ADF file or fetch a Jira issue description/comment using the existing config.
- [ ] The command writes formatted `*.adf.json` through `internal/adf/fixture` and does not run during normal app startup or tests.
- [ ] Run `go test ./cmd/adf-fixture -count=1`.

### Task 5: Real/Sanitized Fixtures And Golden Coverage

**Files:**
- Add: `internal/adf/testdata/real-description.adf.json`
- Add: `internal/adf/testdata/real-description.golden`
- Add: `internal/adf/testdata/real-comment-code.adf.json`
- Add: `internal/adf/testdata/real-comment-code.golden`
- Modify: `internal/adf/render_test.go`

- [ ] Add sanitized real-shaped fixtures for a description and a code-heavy comment.
- [ ] Extend `TestRenderJSONFixtures` to include the new fixture names.
- [ ] Run `go test ./internal/adf -count=1`.

### Task 6: Docs And Backlog

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/lessons.md`
- Modify: `tasks/todo.md`

- [ ] Remove or narrow the completed ADF real-fixture backlog item.
- [ ] Promote the hand-rolled TUI component audit as the next major initiative.
- [ ] Record compact code-block rendering and sanitized fixture capture workflow in docs/changelog.
- [ ] Mark task review complete in `tasks/todo.md`.

### Task 7: Final Verification

- [ ] Run `gofmt -w cmd/adf-fixture internal/adf/fixture internal/adf/render_test.go internal/jira/client.go internal/jira/client_test.go internal/tui/model.go internal/tui/model_test.go`.
- [ ] Run `go test ./internal/adf ./internal/adf/fixture ./internal/jira ./internal/tui -count=1`.
- [ ] Run `go test ./... -count=1`.
- [ ] Run `make check`.
- [ ] Run `make install-user`.
