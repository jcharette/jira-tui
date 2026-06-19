package gitworkflow

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jcharette/jira-tui/internal/jira"
)

var nonBranchWord = regexp.MustCompile(`[^a-z0-9]+`)

type RepoStatus struct {
	Path          string
	CurrentBranch string
	Dirty         bool
	Detected      bool
}

type Client interface {
	DetectRepo(ctx context.Context, path string) (RepoStatus, error)
	CreateOrSwitchBranch(ctx context.Context, repoPath, branch string) error
}

type CLIClient struct{}

func NewCLIClient() CLIClient {
	return CLIClient{}
}

func (CLIClient) DetectRepo(ctx context.Context, path string) (RepoStatus, error) {
	if strings.TrimSpace(path) == "" {
		path = "."
	}
	root, err := gitOutput(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil {
		return RepoStatus{}, fmt.Errorf("detect git repo: %w", err)
	}
	branch, err := gitOutput(ctx, root, "branch", "--show-current")
	if err != nil {
		return RepoStatus{}, fmt.Errorf("detect git branch: %w", err)
	}
	status, err := gitOutput(ctx, root, "status", "--porcelain")
	if err != nil {
		return RepoStatus{}, fmt.Errorf("detect git status: %w", err)
	}
	return RepoStatus{
		Path:          root,
		CurrentBranch: strings.TrimSpace(branch),
		Dirty:         strings.TrimSpace(status) != "",
		Detected:      true,
	}, nil
}

func RenderBranchName(template string, issue jira.Issue) string {
	template = strings.TrimSpace(template)
	if template == "" {
		template = "{key}-{summary_slug}"
	}
	replacements := map[string]string{
		"{key}":          branchToken(issue.Key),
		"{summary_slug}": SummarySlug(issue.Summary),
		"{summary}":      branchToken(issue.Summary),
	}
	rendered := template
	for token, value := range replacements {
		rendered = strings.ReplaceAll(rendered, token, value)
	}
	return cleanBranchName(rendered)
}

func SummarySlug(summary string) string {
	summary = strings.ToLower(strings.TrimSpace(summary))
	summary = nonBranchWord.ReplaceAllString(summary, "-")
	summary = strings.Trim(summary, "-")
	if summary == "" {
		return "work"
	}
	return summary
}

func (CLIClient) CreateOrSwitchBranch(ctx context.Context, repoPath, branch string) error {
	repoPath = strings.TrimSpace(repoPath)
	branch = strings.TrimSpace(branch)
	if repoPath == "" {
		return errors.New("repo path is required")
	}
	if branch == "" {
		return errors.New("branch name is required")
	}
	if _, err := gitOutput(ctx, repoPath, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		_, err = gitOutput(ctx, repoPath, "switch", branch)
		if err != nil {
			return fmt.Errorf("switch branch %s: %w", branch, err)
		}
		return nil
	}
	if _, err := gitOutput(ctx, repoPath, "switch", "-c", branch); err != nil {
		return fmt.Errorf("create branch %s: %w", branch, err)
	}
	return nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = filepath.Clean(dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, detail)
	}
	return strings.TrimSpace(string(output)), nil
}

func branchToken(value string) string {
	return cleanBranchName(strings.TrimSpace(value))
}

func cleanBranchName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = nonBranchWord.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "work"
	}
	return value
}
