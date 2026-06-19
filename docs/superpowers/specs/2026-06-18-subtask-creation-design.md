# Subtask Creation Design

## Goal

Enable creating a Jira subtask from the selected issue without introducing a second create form.

## UX

- The existing `n` shortcut keeps opening normal ticket creation.
- The focused ticket detail Actions tab enables `Create Subtask`.
- Running `Create Subtask` opens the existing create modal with title `Create Subtask` and subtitle
  containing the parent issue key.
- The issue type picker shows only Jira issue types marked as subtasks by create metadata.
- The form keeps the current metadata-backed Summary, Description, supported fields, required-field
  validation, AI draft, bounded rendering, and explicit `ctrl+s` submit behavior.

## Data Flow

- Subtask creation starts from the selected issue in detail mode.
- The project key comes from the parent issue key first, then the existing active-view project
  fallback.
- Create issue type and field metadata use the existing worker-backed create metadata requests.
- Create submission adds the selected parent issue key to the worker create request.
- The Jira client sends `fields.parent.key` in the typed go-atlassian create payload.

## Error Handling

- If no selected parent issue exists, the action leaves the user in detail with a notice.
- If Jira returns no subtask issue types, the create modal shows an empty metadata state and leaves
  diagnostics available.
- Unsupported required create fields continue to block submit before Jira rejects the write.

## Out Of Scope

- Subtask-of-subtask prevention beyond Jira's own metadata/API validation.
- Link management, bulk child creation, or sprint/board placement.
- A separate command palette redesign.
