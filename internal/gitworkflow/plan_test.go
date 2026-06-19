package gitworkflow

import (
	"strings"
	"testing"

	"github.com/jcharette/jira-tui/internal/jira"
)

func TestBuildCommitPlanDirtyWork(t *testing.T) {
	analysis := Analysis{
		Repo:     RepoStatus{Path: "/tmp/repo", CurrentBranch: "abc-123-prepare-release"},
		IssueKey: "ABC-123",
		Changes:  ChangeSummary{Dirty: true, Files: []ChangedFile{{Path: "main.go", Status: "M"}}},
	}

	plan := BuildCommitPlan(analysis, jira.Issue{Key: "ABC-123", Summary: "Prepare release"}, nil)

	if !plan.ShouldCommit || !plan.ShouldReport || !plan.ShouldPush {
		t.Fatalf("plan flags = commit:%v report:%v push:%v", plan.ShouldCommit, plan.ShouldReport, plan.ShouldPush)
	}
	if plan.DefaultCommitMessage != "ABC-123: Prepare release" {
		t.Fatalf("DefaultCommitMessage = %q", plan.DefaultCommitMessage)
	}
	if !strings.Contains(plan.JiraNote, "ABC-123: Prepare release") {
		t.Fatalf("JiraNote = %q", plan.JiraNote)
	}
}

func TestBuildCommitPlanFiltersReportedCommits(t *testing.T) {
	analysis := Analysis{
		Repo: RepoStatus{Path: "/tmp/repo", CurrentBranch: "abc-123-prepare-release"},
		Commits: []Commit{
			{SHA: "1111111", Subject: "ABC-123 first"},
			{SHA: "2222222", Subject: "ABC-123 second"},
		},
	}

	plan := BuildCommitPlan(analysis, jira.Issue{Key: "ABC-123", Summary: "Prepare release"}, ReportedSHAMap([]string{"1111111"}))

	if plan.ShouldCommit {
		t.Fatal("ShouldCommit = true")
	}
	if !plan.ShouldReport || len(plan.UnreportedCommits) != 1 || plan.UnreportedCommits[0].SHA != "2222222" {
		t.Fatalf("unreported plan = %#v", plan)
	}
	if strings.Contains(plan.JiraNote, "first") || !strings.Contains(plan.JiraNote, "second") {
		t.Fatalf("JiraNote = %q", plan.JiraNote)
	}
}

func TestBuildCommitPlanNoop(t *testing.T) {
	plan := BuildCommitPlan(Analysis{
		Repo: RepoStatus{Path: "/tmp/repo", CurrentBranch: "abc-123-prepare-release"},
	}, jira.Issue{Key: "ABC-123", Summary: "Prepare release"}, nil)

	if plan.ShouldCommit || plan.ShouldReport || plan.ShouldPush {
		t.Fatalf("plan should be no-op: %#v", plan)
	}
}

func TestBuildFinishPlanIncludesPRAndFinalNote(t *testing.T) {
	analysis := Analysis{
		Repo:     RepoStatus{Path: "/tmp/repo", CurrentBranch: "abc-123-prepare-release"},
		IssueKey: "ABC-123",
		Commits:  []Commit{{SHA: "1111111", Subject: "ABC-123 prepare release"}},
	}

	plan := BuildFinishPlan(analysis, jira.Issue{Key: "ABC-123", Summary: "Prepare release"}, nil)

	if plan.PRTitle != "ABC-123: Prepare release" {
		t.Fatalf("PRTitle = %q", plan.PRTitle)
	}
	for _, want := range []string{"Summary:", "Prepare release", "ABC-123 prepare release"} {
		if !strings.Contains(plan.PRBody, want) {
			t.Fatalf("PRBody missing %q: %q", want, plan.PRBody)
		}
	}
	if !strings.Contains(plan.FinalJiraNote, "Ready for review") || !strings.Contains(plan.FinalJiraNote, "ABC-123: Prepare release") {
		t.Fatalf("FinalJiraNote = %q", plan.FinalJiraNote)
	}
}
