package prprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type commandRunner func(ctx context.Context, dir string, name string, args ...string) ([]byte, error)

type GitHubCLIProvider struct {
	run commandRunner
}

func NewGitHubCLIProvider() GitHubCLIProvider {
	return GitHubCLIProvider{run: runCommand}
}

func NewGitHubCLIProviderWithRunner(run commandRunner) GitHubCLIProvider {
	if run == nil {
		run = runCommand
	}
	return GitHubCLIProvider{run: run}
}

func (p GitHubCLIProvider) CurrentPR(ctx context.Context, repoPath string) (PullRequest, bool, error) {
	output, err := p.run(ctx, repoPath, "gh", "pr", "view", "--json", "url,title,state")
	if err != nil {
		if isNoCurrentPR(output) {
			return PullRequest{}, false, nil
		}
		return PullRequest{}, false, fmt.Errorf("view GitHub pull request: %w", commandError{Err: err, Output: output})
	}
	var raw struct {
		URL   string `json:"url"`
		Title string `json:"title"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return PullRequest{}, false, fmt.Errorf("parse GitHub pull request: %w", err)
	}
	if strings.TrimSpace(raw.URL) == "" {
		return PullRequest{}, false, nil
	}
	return PullRequest{
		URL:   strings.TrimSpace(raw.URL),
		Title: strings.TrimSpace(raw.Title),
		State: strings.TrimSpace(raw.State),
	}, true, nil
}

func (p GitHubCLIProvider) CreateOrUpdatePR(ctx context.Context, request Request) (PullRequest, error) {
	if existing, ok, err := p.CurrentPR(ctx, request.RepoPath); err != nil {
		return PullRequest{}, err
	} else if ok {
		return existing, nil
	}
	args := []string{"pr", "create", "--title", request.Title, "--body", request.Body}
	if request.Draft {
		args = append(args, "--draft")
	}
	if strings.TrimSpace(request.BaseBranch) != "" {
		args = append(args, "--base", strings.TrimSpace(request.BaseBranch))
	}
	output, err := p.run(ctx, request.RepoPath, "gh", args...)
	if err != nil {
		return PullRequest{}, fmt.Errorf("create GitHub pull request: %w", commandError{Err: err, Output: output})
	}
	url := firstURL(string(output))
	if url == "" {
		return PullRequest{}, fmt.Errorf("create GitHub pull request: gh returned no pull request URL")
	}
	return PullRequest{
		URL:     url,
		Title:   strings.TrimSpace(request.Title),
		State:   "OPEN",
		Created: true,
	}, nil
}

func runCommand(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

type commandError struct {
	Err    error
	Output []byte
}

func (e commandError) Error() string {
	output := strings.TrimSpace(string(e.Output))
	if output == "" {
		return e.Err.Error()
	}
	return e.Err.Error() + ": " + output
}

func (e commandError) Unwrap() error {
	return e.Err
}

func isNoCurrentPR(output []byte) bool {
	text := strings.ToLower(string(output))
	return strings.Contains(text, "no pull request") ||
		strings.Contains(text, "no pull requests") ||
		strings.Contains(text, "not found")
}

func firstURL(output string) string {
	for _, field := range strings.Fields(output) {
		field = strings.TrimSpace(field)
		if strings.HasPrefix(field, "https://") || strings.HasPrefix(field, "http://") {
			return strings.TrimRight(field, ".,")
		}
	}
	return ""
}
