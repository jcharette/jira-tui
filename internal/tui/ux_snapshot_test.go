package tui

import (
	"testing"
	"time"

	"github.com/jcharette/jira-tui/internal/claude"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/events"
	"github.com/jcharette/jira-tui/internal/jira"
)

func TestUXSnapshots(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("EDT", -4*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	cases := []struct {
		name  string
		model func(t *testing.T) Model
	}{
		{name: "ux_issue_list.golden", model: uxSnapshotBaseModel},
		{name: "ux_ticket_detail.golden", model: uxSnapshotDetailModel},
		{name: "ux_action_palette.golden", model: uxSnapshotActionPaletteModel},
		{name: "ux_comment_composer.golden", model: uxSnapshotCommentComposerModel},
		{name: "ux_create_ticket.golden", model: uxSnapshotCreateTicketModel},
		{name: "ux_worklog_dialog.golden", model: uxSnapshotWorklogDialogModel},
		{name: "ux_claude_section.golden", model: uxSnapshotClaudeSectionModel},
		{name: "ux_diagnostics.golden", model: uxSnapshotDiagnosticsModel},
		{name: "ux_notifications.golden", model: uxSnapshotNotificationsModel},
		{name: "ux_bug_report.golden", model: uxSnapshotBugReportModel},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := tc.model(t)
			defer model.workers.Stop()
			assertGoldenSnapshot(t, tc.name, model.render())
		})
	}
}

func uxSnapshotBaseModel(t *testing.T) Model {
	t.Helper()
	now := time.Date(2026, 7, 6, 9, 30, 0, 0, time.UTC)
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC ORDER BY updated DESC",
		WithNow(func() time.Time { return now }),
		WithClaudeConfig(ClaudeConfig{
			Enabled:         true,
			TicketPlan:      true,
			TicketAssist:    true,
			DraftComment:    true,
			DraftTicket:     true,
			BranchPlan:      true,
			AllowJiraWrites: true,
		}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude", Version: "test"}),
		WithClaudeRunner(&fakeClaudeRunner{result: claude.Result{Text: "Draft output"}}),
		WithDisplay(config.Display{SymbolMode: "symbols"}),
	)
	model.width = 120
	model.height = 34
	model.loading = false
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Tighten deployment review flow", Status: "In Progress", Priority: "High", Assignee: "Jon", IssueType: "Story"},
		{Key: "ABC-2", Summary: "Document keyboard shortcuts", Status: "To Do", Priority: "Medium", Assignee: "Rae", IssueType: "Task"},
		{Key: "ABC-3", Summary: "Fix comment mention picker", Status: "Review", Priority: "Low", Assignee: "Ari", IssueType: "Bug"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Improve the review flow so generated text, local edits, and Jira writes are easy to distinguish.",
			Reporter:    "Rae",
			Creator:     "Ari",
			Labels:      []string{"ux", "claude"},
			Components:  []string{"tui"},
			Created:     now.Add(-48 * time.Hour),
			Updated:     now,
		},
	}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {{ID: "10001", Author: "Rae", Body: "Please make the write gates obvious before posting.", Created: now.Add(-2 * time.Hour)}},
	}
	model.worklogs = map[string][]jira.Worklog{
		"ABC-1": {{ID: "20001", Author: "Jon", TimeSpent: "45m", Comment: "Reviewed Claude flows", Started: now.Add(-3 * time.Hour)}},
	}
	return model
}

func uxSnapshotDetailModel(t *testing.T) Model {
	t.Helper()
	model := uxSnapshotBaseModel(t)
	model.mode = modeDetail
	return model
}

func uxSnapshotActionPaletteModel(t *testing.T) Model {
	t.Helper()
	model := uxSnapshotDetailModel(t)
	model.actionPaletteOpen = true
	model.actionPaletteEditor = newActionPaletteFilterInput("")
	model.actionPaletteEditorReady = true
	return model
}

func uxSnapshotCommentComposerModel(t *testing.T) Model {
	t.Helper()
	model := uxSnapshotDetailModel(t)
	model.startCommentComposer()
	model.commentDraft = "Can we confirm the rollout validation owner before this moves forward?"
	model.commentEditor = newCommentEditor(model.commentDraft)
	model.commentEditorReady = true
	return model
}

func uxSnapshotCreateTicketModel(t *testing.T) Model {
	t.Helper()
	model := uxSnapshotBaseModel(t)
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueTypes = []jira.CreateIssueType{{ID: "10001", Name: "Story"}, {ID: "10002", Name: "Task"}}
	return model
}

func uxSnapshotWorklogDialogModel(t *testing.T) Model {
	t.Helper()
	model := uxSnapshotDetailModel(t)
	updated, _ := model.startWorklogEditor()
	updated.worklogTimeDraft = "45m"
	updated.worklogTimeEditor = newWorklogTimeInput("45m")
	updated.worklogTimeEditorReady = true
	updated.worklogCommentDraft = "UX review pass"
	updated.worklogCommentEditor = newWorklogCommentEditor("UX review pass")
	updated.worklogCommentEditorReady = true
	return updated
}

func uxSnapshotClaudeSectionModel(t *testing.T) Model {
	t.Helper()
	model := uxSnapshotDetailModel(t)
	model.jumpDetailSection("Claude")
	return model
}

func uxSnapshotDiagnosticsModel(t *testing.T) Model {
	t.Helper()
	model := uxSnapshotDetailModel(t)
	model.diagnosticsOpen = true
	model.recordDiagnosticEvent(diagnosticKindWorker, "search_issues", "completed", "request_id=1 count=3")
	model.recordDiagnosticEvent(diagnosticKindClaude, "ticket_assist", "completed", "request_id=2 key=ABC-1")
	return model
}

func uxSnapshotNotificationsModel(t *testing.T) Model {
	t.Helper()
	model := uxSnapshotDetailModel(t)
	model.notificationPanelOpen = true
	model.notifications = []notification{{
		ID:            "notice-1",
		EventType:     events.TypeJiraTicketUpdated,
		IssueKey:      "ABC-1",
		Summary:       "Tighten deployment review flow",
		ChangedFields: []string{"status", "assignee"},
		ViewName:      "Current Work",
		At:            model.currentTime(),
	}}
	return model
}

func uxSnapshotBugReportModel(t *testing.T) Model {
	t.Helper()
	model := uxSnapshotDetailModel(t)
	model = model.startBugReport()
	model.bugReportTitleDraft = "Shortcut copy mismatch"
	model.bugReportTitleEditor = newBugReportTitleInput(model.bugReportTitleDraft)
	model.bugReportTitleEditorReady = true
	model.bugReportBodyDraft = "The footer says one thing but the keymap does another."
	model.bugReportBodyEditor = newBugReportBodyEditor(model.bugReportBodyDraft)
	model.bugReportBodyEditorReady = true
	model.bugReportIncludeDiagnostics = true
	return model
}
