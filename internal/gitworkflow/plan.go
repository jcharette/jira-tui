package gitworkflow

import (
	"fmt"
	"strings"

	"github.com/jcharette/jira-tui/internal/jira"
)

const maxNoteCommitLines = 6

type CommitPlan struct {
	IssueKey             string
	IssueSummary         string
	RepoPath             string
	Branch               string
	BaseBranch           string
	Changes              ChangeSummary
	UnreportedCommits    []Commit
	DefaultCommitMessage string
	JiraNote             string
	ShouldCommit         bool
	ShouldReport         bool
	ShouldPush           bool
}

type FinishPlan struct {
	CommitPlan    CommitPlan
	PRTitle       string
	PRBody        string
	FinalJiraNote string
}

func BuildCommitPlan(analysis Analysis, issue jira.Issue, reportedSHAs map[string]bool) CommitPlan {
	issueKey := strings.TrimSpace(issue.Key)
	if issueKey == "" {
		issueKey = analysis.IssueKey
	}
	issueKey = strings.ToUpper(strings.TrimSpace(issueKey))
	summary := oneLine(issue.Summary)
	unreported := unreportedCommits(analysis.Commits, reportedSHAs)
	defaultMessage := defaultCommitMessage(issueKey, summary)
	shouldCommit := analysis.Changes.Dirty
	shouldReport := shouldCommit || len(unreported) > 0
	return CommitPlan{
		IssueKey:             issueKey,
		IssueSummary:         summary,
		RepoPath:             analysis.Repo.Path,
		Branch:               analysis.Repo.CurrentBranch,
		BaseBranch:           analysis.BaseBranch,
		Changes:              analysis.Changes,
		UnreportedCommits:    unreported,
		DefaultCommitMessage: defaultMessage,
		JiraNote:             buildCommitJiraNote(issueKey, defaultMessage, unreported, shouldCommit),
		ShouldCommit:         shouldCommit,
		ShouldReport:         shouldReport,
		ShouldPush:           shouldCommit || len(unreported) > 0,
	}
}

func BuildFinishPlan(analysis Analysis, issue jira.Issue, reportedSHAs map[string]bool) FinishPlan {
	commitPlan := BuildCommitPlan(analysis, issue, reportedSHAs)
	title := defaultPRTitle(commitPlan.IssueKey, commitPlan.IssueSummary)
	body := defaultPRBody(commitPlan)
	return FinishPlan{
		CommitPlan:    commitPlan,
		PRTitle:       title,
		PRBody:        body,
		FinalJiraNote: buildFinalJiraNote(commitPlan, title),
	}
}

func ReportedSHAMap(shas []string) map[string]bool {
	reported := make(map[string]bool, len(shas))
	for _, sha := range shas {
		sha = strings.TrimSpace(sha)
		if sha != "" {
			reported[sha] = true
		}
	}
	return reported
}

func unreportedCommits(commits []Commit, reportedSHAs map[string]bool) []Commit {
	unreported := make([]Commit, 0, len(commits))
	for _, commit := range commits {
		if strings.TrimSpace(commit.SHA) == "" || reportedSHAs[commit.SHA] {
			continue
		}
		unreported = append(unreported, commit)
	}
	return unreported
}

func defaultCommitMessage(issueKey string, summary string) string {
	issueKey = strings.ToUpper(strings.TrimSpace(issueKey))
	summary = oneLine(summary)
	if summary == "" {
		summary = "update work"
	}
	if issueKey == "" {
		return summary
	}
	return issueKey + ": " + summary
}

func buildCommitJiraNote(issueKey string, defaultMessage string, commits []Commit, includePending bool) string {
	lines := []string{"Development update:"}
	if includePending {
		lines = append(lines, "- "+defaultMessage)
	}
	for _, commit := range commits {
		if len(lines)-1 >= maxNoteCommitLines {
			lines = append(lines, fmt.Sprintf("- ...and %d more commit(s)", len(commits)-(len(lines)-1)))
			break
		}
		subject := oneLine(commit.Subject)
		if subject == "" {
			subject = shortSHA(commit.SHA)
		}
		lines = append(lines, "- "+subject)
	}
	return strings.Join(lines, "\n")
}

func defaultPRTitle(issueKey string, summary string) string {
	return defaultCommitMessage(issueKey, summary)
}

func defaultPRBody(plan CommitPlan) string {
	var b strings.Builder
	if plan.IssueKey != "" {
		fmt.Fprintf(&b, "%s\n\n", plan.IssueKey)
	}
	b.WriteString("Summary:\n")
	if plan.IssueSummary != "" {
		fmt.Fprintf(&b, "- %s\n", plan.IssueSummary)
	} else {
		b.WriteString("- Update ticket work.\n")
	}
	if len(plan.UnreportedCommits) > 0 {
		b.WriteString("\nCommits:\n")
		for _, commit := range plan.UnreportedCommits {
			fmt.Fprintf(&b, "- %s\n", oneLine(displayValue(commit.Subject, shortSHA(commit.SHA))))
		}
	}
	return strings.TrimSpace(b.String())
}

func buildFinalJiraNote(plan CommitPlan, prTitle string) string {
	lines := []string{"Ready for review:"}
	if prTitle = oneLine(prTitle); prTitle != "" {
		lines = append(lines, "- "+prTitle)
	}
	for _, commit := range plan.UnreportedCommits {
		if len(lines)-1 >= maxNoteCommitLines {
			lines = append(lines, fmt.Sprintf("- ...and %d more commit(s)", len(plan.UnreportedCommits)-(len(lines)-1)))
			break
		}
		lines = append(lines, "- "+oneLine(displayValue(commit.Subject, shortSHA(commit.SHA))))
	}
	return strings.Join(lines, "\n")
}

func oneLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	fields := strings.Fields(value)
	return strings.Join(fields, " ")
}

func shortSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

func displayValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}
