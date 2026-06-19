package gitworkflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jcharette/jira-tui/internal/jira"
)

func TestRenderBranchNameUsesDefaultTemplateAndSanitizedSummary(t *testing.T) {
	issue := jira.Issue{Key: "PROJ-123", Summary: "Add setup wizard: phase 1!"}

	got := RenderBranchName("", issue)

	if got != "proj-123-add-setup-wizard-phase-1" {
		t.Fatalf("RenderBranchName() = %q", got)
	}
}

func TestRenderBranchNameUsesConfiguredTemplate(t *testing.T) {
	issue := jira.Issue{Key: "PROJ-123", Summary: "Start Work"}

	got := RenderBranchName("feature/{key}/{summary_slug}", issue)

	if got != "feature-proj-123-start-work" {
		t.Fatalf("RenderBranchName() = %q", got)
	}
}

func TestDetectRepoReadsRootBranchAndDirtyState(t *testing.T) {
	repo := initTempRepo(t)
	writeFile(t, filepath.Join(repo, "notes.txt"), "draft")

	status, err := NewCLIClient().DetectRepo(context.Background(), filepath.Join(repo, "subdir"))
	if err != nil {
		t.Fatalf("DetectRepo() error = %v", err)
	}

	if !status.Detected {
		t.Fatal("Detected = false")
	}
	wantPath, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("EvalSymlinks(repo) error = %v", err)
	}
	gotPath, err := filepath.EvalSymlinks(status.Path)
	if err != nil {
		t.Fatalf("EvalSymlinks(status.Path) error = %v", err)
	}
	if gotPath != wantPath {
		t.Fatalf("Path = %q, want %q", status.Path, repo)
	}
	if status.CurrentBranch == "" {
		t.Fatal("CurrentBranch is empty")
	}
	if !status.Dirty {
		t.Fatal("Dirty = false, want true")
	}
}

func TestCreateOrSwitchBranchCreatesThenSwitchesExistingBranch(t *testing.T) {
	repo := initTempRepo(t)
	client := NewCLIClient()

	if err := client.CreateOrSwitchBranch(context.Background(), repo, "proj-123-start-work"); err != nil {
		t.Fatalf("CreateOrSwitchBranch(create) error = %v", err)
	}
	if branch := gitTestOutput(t, repo, "branch", "--show-current"); branch != "proj-123-start-work" {
		t.Fatalf("branch after create = %q", branch)
	}

	gitTest(t, repo, "switch", "-")
	if err := client.CreateOrSwitchBranch(context.Background(), repo, "proj-123-start-work"); err != nil {
		t.Fatalf("CreateOrSwitchBranch(switch) error = %v", err)
	}
	if branch := gitTestOutput(t, repo, "branch", "--show-current"); branch != "proj-123-start-work" {
		t.Fatalf("branch after switch = %q", branch)
	}
}

func initTempRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitTest(t, repo, "init")
	gitTest(t, repo, "config", "user.email", "person@example.test")
	gitTest(t, repo, "config", "user.name", "Person")
	writeFile(t, filepath.Join(repo, "README.md"), "fixture\n")
	gitTest(t, repo, "add", "README.md")
	gitTest(t, repo, "commit", "-m", "initial")
	if err := os.Mkdir(filepath.Join(repo, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	return repo
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func gitTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	_ = gitTestOutput(t, dir, args...)
}

func gitTestOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
	return strings.TrimSpace(string(output))
}
