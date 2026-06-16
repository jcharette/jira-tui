# Hand-Rolled TUI Component Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce a concrete audit of custom TUI components and implement one small, tested simplification toward maintained Bubble Tea/Bubbles/Lip Gloss primitives.

**Architecture:** Keep Jira behavior and existing model state stable. Add documentation first, then introduce at most one narrow internal abstraction or Bubbles-backed adapter after tests prove the behavior boundary.

**Tech Stack:** Go, Bubble Tea v2, Bubbles v2, Lip Gloss, existing TUI tests, `make check`.

---

### Task 1: Complete The Audit Matrix

**Files:**
- Create: `docs/tui-component-audit.md`
- Modify: `tasks/todo.md`

- [ ] **Step 1: Create the audit document**

Add `docs/tui-component-audit.md` with a table containing the audited surfaces and the first-slice
recommendation. Use the committed audit document as the source of truth:

```markdown
# TUI Component Audit

This audit tracks custom TUI rendering and input code that may be replaced or wrapped with
maintained Bubble Tea, Bubbles, Lip Gloss, or compatible ecosystem libraries.

| Surface | Current code | Pain / risk | Candidate maintained primitive | Recommendation | Priority | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| Footer and keyboard help | `internal/tui/keymap.go`; `internal/tui/model.go`: `renderFooterHelpWithBindings`, `renderHelp`, `helpLines` | Custom binding type, footer truncation, help grouping, and manual help scrolling | Bubbles `key` and `help`, with a local adapter for context grouping | First implementation candidate | P0 | Lower product risk than create forms because key metadata is already centralized and data-driven. |
| Config scalar text input | `internal/configui/model.go`: `updateEditor`, `renderFields` | Manual single-line edit buffer with append/backspace only and no cursor model | Bubbles `textinput` | First implementation candidate | P0 | Separate config model makes this easy to test. Keep custom bool/color picker rendering for now. |
| Create metadata option picker | `internal/tui/model.go`: `renderCreateIssueTypePickerLines`, `renderCreateDynamicField`, `updateCreateIssue` | Long Jira option lists, custom filtering/windowing, repeated selection logic | Bubbles `list`, Bubbles `textinput`, or a small app picker wrapper | Investigate later or scope to one dynamic picker | P1 | User pain is real, but Jira metadata forms have many edge cases. |
| Rich text and fitted tables | `internal/tui/model.go`: `wrapRichText`, `renderFittedTable`, `renderWrappedTableRow` | Custom table fitting and Markdown-ish parsing | Lip Gloss table helpers where fixture-compatible | Investigate later | P1 | Keep ADF fixture tests and `internal/adf.Render` boundary. |
| Dialog layout and editor configuration | `internal/tui/model.go`: `renderDetailDialog*`, `configured*Editor`, `new*Editor` | Repeated sizing/styling logic | Internal wrapper around Bubbles `textarea` and Lip Gloss layout | Wrap later | P1 | Good cleanup target after picker risk is reduced. |
| Detail scrolling | `internal/tui/model.go`: `newDetailViewport`, `renderScrollableDetailBody` | Mostly already uses Bubbles viewport, custom line indicator | Bubbles `viewport` | Keep with light wrapper | P2 | Existing primitive is appropriate; avoid churn. |
| Config UI booleans/colors | `internal/configui/model.go`: `renderBoolPicker`, `renderColorSwatch` | Small custom renderers | Bubbles list/textinput or internal shared picker | Defer | P2 | Not current user pain. |
```

- [ ] **Step 2: Update `tasks/todo.md` progress**

Mark the audit-document task complete after the file exists.

### Task 2: Choose And Prove The First Low-Risk Boundary

**Files:**
- Modify one of: `internal/tui/model_test.go`, `internal/configui/model_test.go`

- [ ] **Step 1: Choose the first code slice**

Choose one:

```text
A. Footer/help adapter with Bubbles key/help.
B. Config scalar text input with Bubbles textinput.
```

Use A if the goal is app-wide TUI consistency. Use B if the goal is the smallest possible proof of
moving manual input buffers to maintained Bubbles primitives.

- [ ] **Step 2: Add or identify focused behavior tests**

For A, ensure tests cover:

```text
table context footer still includes move, open, new, refresh, and quit/help where expected
detail context footer still includes back, scroll, section, new, ai, open, summary, and priority where expected
help view still groups or lists context bindings and can scroll when content exceeds height
```

For B, ensure tests cover:

```text
typing inserts text into a scalar config field
backspace deletes text
left/right movement changes cursor position before insertion
save persists the edited value through the existing validation path
boolean and color controls keep their current picker/swatch behavior
```

- [ ] **Step 3: Run focused tests before implementation**

Run one command based on the chosen slice:

```bash
go test ./internal/tui -run 'Test.*(Footer|Help|Key)' -count=1
go test ./internal/configui -count=1
```

Expected: pass before refactor. If the regex misses existing tests, run the exact relevant test names
found in the package test file.

### Task 3: Implement One Small Maintained-Primitive Migration

**Files:**
- Modify one of: `internal/tui/model.go`, `internal/tui/keymap.go`, `internal/configui/model.go`
- Test one of: `internal/tui/model_test.go`, `internal/configui/model_test.go`

- [ ] **Step 1: Choose the smaller valid implementation**

Pick one after reading the audit:

```text
A. Convert footer/help rendering metadata to Bubbles key/help through a local adapter while keeping
   existing key contexts and labels.
B. Replace the config UI scalar edit buffer with Bubbles textinput while keeping bool/color controls
   custom.
```

Use B if A requires broad changes to key context semantics.

- [ ] **Step 2: Implement the smallest passing change**

Do not change Jira behavior, config file shape, key meanings, or command names.

- [ ] **Step 3: Format touched Go files**

Run:

```bash
gofmt -w internal/tui/model.go internal/tui/keymap.go internal/tui/model_test.go internal/configui/model.go internal/configui/model_test.go
```

### Task 4: Verify And Document

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/planning.md` if sequencing changes
- Modify: `docs/releases/CHANGELOG.md` if behavior changes
- Modify: `tasks/todo.md`

- [ ] **Step 1: Run focused tests**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: pass.

- [ ] **Step 2: Run full verification**

Run:

```bash
go test ./... -count=1
make check
```

Expected: pass.

- [ ] **Step 3: Update docs**

If code behavior changes, add a short `Unreleased` changelog entry. If the audit reprioritizes the
backlog, update `docs/backlog.md`. Add a review note to `tasks/todo.md`.

- [ ] **Step 4: Stop for review**

Summarize the audit outcome, implementation choice, files changed, and verification output before
starting the next backlog item.
