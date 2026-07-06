package startworkflow

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/gitworkflow"
	"github.com/jcharette/jira-tui/internal/jira"
)

func TestNewModelWithIssueStartsAtRepoAndPreviewsBranch(t *testing.T) {
	issue := jira.Issue{Key: "PROJ-123", Summary: "Add setup wizard"}
	model := NewModel(Options{
		Config:        config.Defaults(),
		Issue:         &issue,
		PreferredRepo: gitworkflow.RepoStatus{Path: "/tmp/repo", Detected: true},
	})

	if model.step != StepRepo {
		t.Fatalf("step = %v, want StepRepo", model.step)
	}
	if model.repoInput.Value() != "/tmp/repo" {
		t.Fatalf("repo = %q", model.repoInput.Value())
	}
	if model.branchInput.Value() != "proj-123-add-setup-wizard" {
		t.Fatalf("branch = %q", model.branchInput.Value())
	}
}

func TestModelWithoutIssueUsesTicketPicker(t *testing.T) {
	model := NewModel(Options{
		Config: config.Defaults(),
		Issues: []jira.Issue{
			{Key: "PROJ-100", Summary: "First ticket"},
			{Key: "PROJ-101", Summary: "Second ticket"},
		},
		PreferredRepo: gitworkflow.RepoStatus{Path: "/tmp/repo", Detected: true},
	})

	model = updateTest(t, model, "down")
	model = updateTest(t, model, "enter")

	if model.step != StepRepo {
		t.Fatalf("step = %v, want StepRepo", model.step)
	}
	if model.issue.Key != "PROJ-101" {
		t.Fatalf("issue = %#v", model.issue)
	}
	if model.branchInput.Value() != "proj-101-second-ticket" {
		t.Fatalf("branch = %q", model.branchInput.Value())
	}
}

func TestModelCanEditBranchAndConfirmResult(t *testing.T) {
	issue := jira.Issue{Key: "PROJ-123", Summary: "Add setup wizard"}
	model := NewModel(Options{
		Config:        config.Defaults(),
		Issue:         &issue,
		PreferredRepo: gitworkflow.RepoStatus{Path: "/tmp/repo", Detected: true},
	})

	model = updateTest(t, model, "enter")
	model.branchInput.SetValue("feature/proj-123-custom")
	model = updateTest(t, model, "enter")
	model = updateTest(t, model, "enter")

	result := model.Result()
	if !result.Confirmed || result.Canceled {
		t.Fatalf("Result = %#v", result)
	}
	if result.BranchName != "feature/proj-123-custom" {
		t.Fatalf("BranchName = %q", result.BranchName)
	}
	if len(result.Actions) != 4 || result.Actions[0].Kind != ActionBranch || !result.Actions[0].Required {
		t.Fatalf("Actions = %#v", result.Actions)
	}
}

func TestModelCanSkipOptionalReviewActions(t *testing.T) {
	issue := jira.Issue{Key: "PROJ-123", Summary: "Add setup wizard"}
	model := NewModel(Options{
		Config:        config.Defaults(),
		Issue:         &issue,
		PreferredRepo: gitworkflow.RepoStatus{Path: "/tmp/repo", Detected: true},
	})

	model = updateTest(t, model, "enter")
	model = updateTest(t, model, "enter")
	model = updateTest(t, model, "down")
	model = updateTest(t, model, " ")
	model = updateTest(t, model, "enter")

	result := model.Result()
	if !result.Confirmed {
		t.Fatalf("Result = %#v", result)
	}
	if !result.Actions[1].Skip {
		t.Fatalf("assign action not skipped: %#v", result.Actions)
	}
	if result.Actions[0].Skip {
		t.Fatalf("required branch action should not be skipped: %#v", result.Actions)
	}
}

func TestModelCancelBeforeWriteReturnsCanceledResult(t *testing.T) {
	issue := jira.Issue{Key: "PROJ-123", Summary: "Add setup wizard"}
	model := NewModel(Options{
		Config:        config.Defaults(),
		Issue:         &issue,
		PreferredRepo: gitworkflow.RepoStatus{Path: "/tmp/repo", Detected: true},
	})

	model = updateTest(t, model, "esc")

	result := model.Result()
	if !result.Canceled || result.Confirmed {
		t.Fatalf("Result = %#v", result)
	}
}

func TestRenderReviewShowsActionsAndFooter(t *testing.T) {
	issue := jira.Issue{Key: "PROJ-123", Summary: "Add setup wizard"}
	model := NewModel(Options{
		Config:        config.Defaults(),
		Issue:         &issue,
		PreferredRepo: gitworkflow.RepoStatus{Path: "/tmp/repo", Detected: true},
	})
	model = updateTest(t, model, "enter")
	model = updateTest(t, model, "enter")

	view := model.render()
	for _, want := range []string{"Review actions", "Create or switch branch", "Assign to me", "Move to In Progress", "space skip optional"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestRenderReviewShowsClaudePlanAndUnavailableState(t *testing.T) {
	issue := jira.Issue{Key: "PROJ-123", Summary: "Add setup wizard"}
	model := NewModel(Options{
		Config:        config.Defaults(),
		Issue:         &issue,
		PreferredRepo: gitworkflow.RepoStatus{Path: "/tmp/repo", Detected: true},
		PlanText:      "1. Inspect setup flow.\n2. Run focused tests.",
	})
	model = updateTest(t, model, "enter")
	model = updateTest(t, model, "enter")

	view := model.render()
	for _, want := range []string{"Claude plan:", "Inspect setup flow", "Run focused tests"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}

	model = NewModel(Options{
		Config:        config.Defaults(),
		Issue:         &issue,
		PreferredRepo: gitworkflow.RepoStatus{Path: "/tmp/repo", Detected: true},
		PlanErr:       "claude unavailable",
	})
	model = updateTest(t, model, "enter")
	model = updateTest(t, model, "enter")
	if view := model.render(); !strings.Contains(view, "Claude plan unavailable: claude unavailable") {
		t.Fatalf("missing unavailable state in %q", view)
	}
}

func TestBuildPlanDraftPromptIncludesRequestedWrites(t *testing.T) {
	prompt := BuildPlanDraftPrompt(PlanDraftRequest{
		Issue:      jira.Issue{Key: "PROJ-123", Summary: "Add setup wizard", Status: "To Do"},
		RepoPath:   "/tmp/repo",
		BranchName: "proj-123-add-setup-wizard",
		Actions:    DefaultActions("proj-123-add-setup-wizard"),
	})

	for _, want := range []string{"read-only start-work", "PROJ-123", "/tmp/repo", "proj-123-add-setup-wizard", "Requested writes", "Assign to me"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing %q in %q", want, prompt)
		}
	}
}

func updateTest(t *testing.T, model Model, key string) Model {
	t.Helper()
	msg := tea.KeyPressMsg(tea.Key{Text: key})
	switch key {
	case "enter":
		msg = tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "esc":
		msg = tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc})
	case "up":
		msg = tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})
	case "down":
		msg = tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})
	case " ":
		msg = tea.KeyPressMsg(tea.Key{Code: tea.KeySpace})
	}
	updated, _ := model.Update(msg)
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("updated model = %T", updated)
	}
	return next
}
