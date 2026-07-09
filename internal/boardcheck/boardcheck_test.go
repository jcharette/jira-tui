package boardcheck

import (
	"testing"

	"github.com/jcharette/jira-tui/internal/jira"
)

func TestCheckIssueFlagsSubtaskUnderEpic(t *testing.T) {
	findings := CheckIssue(jira.Issue{
		Key:       "DEVOPS-2",
		IssueType: "Sub-task",
		IsSubtask: true,
		ParentKey: "DEVOPS-1",
	}, Options{Parent: &jira.Issue{Key: "DEVOPS-1", IssueType: "Epic"}})

	if len(findings) != 1 || findings[0].Code != CodeSubtaskUnderEpic || findings[0].Severity != SeverityError {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestCheckIssueFlagsInProgressUnassignedAndMissingSprint(t *testing.T) {
	findings := CheckIssue(jira.Issue{
		Key:       "DEVOPS-3",
		Status:    "In Progress",
		Assignee:  "Unassigned",
		IssueType: "Story",
	}, Options{RequireActiveSprint: true, ActiveSprintKnown: true})

	if len(findings) != 2 {
		t.Fatalf("findings = %#v", findings)
	}
	if findings[0].Code != CodeUnassigned || findings[1].Code != CodeMissingActiveSprint {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestCheckIssueAllowsCleanStory(t *testing.T) {
	findings := CheckIssue(jira.Issue{
		Key:       "DEVOPS-4",
		Status:    "In Progress",
		Assignee:  "Jon",
		IssueType: "Story",
	}, Options{RequireActiveSprint: true, ActiveSprintKnown: true, InActiveSprint: true})

	if len(findings) != 0 {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestCheckIssueFlagsAssignedTodoMissingSprint(t *testing.T) {
	findings := CheckIssue(jira.Issue{
		Key:       "DEVOPS-5",
		Status:    "To Do",
		Assignee:  "Jon",
		IssueType: "Story",
	}, Options{RequireActiveSprint: true, ActiveSprintKnown: true})

	if len(findings) != 1 || findings[0].Code != CodeMissingActiveSprint {
		t.Fatalf("findings = %#v", findings)
	}
}
