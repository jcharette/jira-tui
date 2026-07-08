package boardcheck

import (
	"fmt"
	"strings"

	"github.com/jcharette/jira-tui/internal/jira"
)

type Severity string

const (
	SeverityError Severity = "ERROR"
	SeverityWarn  Severity = "WARN"
)

type Code string

const (
	CodeSubtaskUnderEpic    Code = "subtask-under-epic"
	CodeUnassigned          Code = "unassigned"
	CodeMissingActiveSprint Code = "missing-active-sprint"
)

type Finding struct {
	Severity Severity
	Code     Code
	IssueKey string
	Message  string
	Fix      string
}

type Options struct {
	Parent              *jira.Issue
	RequireActiveSprint bool
	ActiveSprintKnown   bool
	InActiveSprint      bool
}

func CheckIssue(issue jira.Issue, opts Options) []Finding {
	var findings []Finding
	if isSubtask(issue) && opts.Parent != nil && isEpic(*opts.Parent) {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Code:     CodeSubtaskUnderEpic,
			IssueKey: issue.Key,
			Message:  fmt.Sprintf("%s is a Sub-task directly under Epic %s", issue.Key, opts.Parent.Key),
			Fix:      "convert to Story/Task under the Epic or create a replacement Story/Task",
		})
	}
	if isInProgress(issue) && isUnassigned(issue) {
		findings = append(findings, Finding{
			Severity: SeverityWarn,
			Code:     CodeUnassigned,
			IssueKey: issue.Key,
			Message:  issue.Key + " is in progress but unassigned",
			Fix:      "assign the ticket",
		})
	}
	if opts.RequireActiveSprint && opts.ActiveSprintKnown && !opts.InActiveSprint && isInProgress(issue) {
		findings = append(findings, Finding{
			Severity: SeverityWarn,
			Code:     CodeMissingActiveSprint,
			IssueKey: issue.Key,
			Message:  issue.Key + " is in progress but not in the active sprint",
			Fix:      "add the ticket to the active sprint",
		})
	}
	return findings
}

func IsEpic(issue jira.Issue) bool {
	return isEpic(issue)
}

func IsSubtask(issue jira.Issue) bool {
	return isSubtask(issue)
}

func IsInProgress(issue jira.Issue) bool {
	return isInProgress(issue)
}

func isEpic(issue jira.Issue) bool {
	return strings.EqualFold(strings.TrimSpace(issue.IssueType), "Epic") || issue.HierarchyLevel == 1
}

func isSubtask(issue jira.Issue) bool {
	issueType := strings.ToLower(strings.TrimSpace(issue.IssueType))
	return issue.IsSubtask || issue.HierarchyLevel < 0 || issueType == "subtask" || issueType == "sub-task"
}

func isInProgress(issue jira.Issue) bool {
	status := strings.ToLower(strings.TrimSpace(issue.Status))
	if status == "" {
		return false
	}
	for _, done := range []string{"done", "closed", "resolved", "cancelled", "canceled"} {
		if status == done {
			return false
		}
	}
	return strings.Contains(status, "progress") || strings.Contains(status, "review") || strings.Contains(status, "blocked")
}

func isUnassigned(issue jira.Issue) bool {
	assignee := strings.TrimSpace(issue.Assignee)
	return assignee == "" || strings.EqualFold(assignee, "Unassigned")
}
