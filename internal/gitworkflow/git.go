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
var issueKeyPattern = regexp.MustCompile(`(?i)\b([A-Z][A-Z0-9]+-\d+)\b`)

type RepoStatus struct {
	Path          string
	CurrentBranch string
	Dirty         bool
	Detected      bool
}

type Analysis struct {
	Repo           RepoStatus
	BaseBranch     string
	UpstreamBranch string
	IssueKey       string
	Changes        ChangeSummary
	Commits        []Commit
}

type ChangeSummary struct {
	Dirty bool
	Files []ChangedFile
}

type ChangedFile struct {
	Path   string
	Status string
}

type Commit struct {
	SHA     string
	Subject string
	Body    string
}

type Client interface {
	DetectRepo(ctx context.Context, path string) (RepoStatus, error)
	Analyze(ctx context.Context, path string) (Analysis, error)
	CreateOrSwitchBranch(ctx context.Context, repoPath, branch string) error
	CommitAll(ctx context.Context, repoPath, message string) (Commit, error)
	PushCurrentBranch(ctx context.Context, repoPath string) error
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

func (c CLIClient) Analyze(ctx context.Context, path string) (Analysis, error) {
	repo, err := c.DetectRepo(ctx, path)
	if err != nil {
		return Analysis{}, err
	}
	upstream, _ := gitOutput(ctx, repo.Path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	base := c.detectBaseBranch(ctx, repo.Path, repo.CurrentBranch, upstream)
	changes, err := c.changeSummary(ctx, repo.Path)
	if err != nil {
		return Analysis{}, err
	}
	commits, err := c.localCommits(ctx, repo.Path, base)
	if err != nil {
		return Analysis{}, err
	}
	return Analysis{
		Repo:           repo,
		BaseBranch:     base,
		UpstreamBranch: strings.TrimSpace(upstream),
		IssueKey:       DetectIssueKey(repo.CurrentBranch),
		Changes:        changes,
		Commits:        commits,
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

func (CLIClient) CommitAll(ctx context.Context, repoPath, message string) (Commit, error) {
	repoPath = strings.TrimSpace(repoPath)
	message = strings.TrimSpace(message)
	if repoPath == "" {
		return Commit{}, errors.New("repo path is required")
	}
	if message == "" {
		return Commit{}, errors.New("commit message is required")
	}
	if _, err := gitOutput(ctx, repoPath, "add", "-A"); err != nil {
		return Commit{}, fmt.Errorf("stage changes: %w", err)
	}
	if _, err := gitOutput(ctx, repoPath, "commit", "-m", message); err != nil {
		return Commit{}, fmt.Errorf("commit changes: %w", err)
	}
	return readCommit(ctx, repoPath, "HEAD")
}

func (CLIClient) PushCurrentBranch(ctx context.Context, repoPath string) error {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return errors.New("repo path is required")
	}
	if _, err := gitOutput(ctx, repoPath, "push", "-u", "origin", "HEAD"); err != nil {
		return fmt.Errorf("push branch: %w", err)
	}
	return nil
}

func (CLIClient) detectBaseBranch(ctx context.Context, repoPath, currentBranch, upstream string) string {
	for _, candidate := range []string{strings.TrimSpace(upstream), "origin/main", "main", "origin/master", "master"} {
		if candidate == "" {
			continue
		}
		if candidate == currentBranch {
			return candidate
		}
		if _, err := gitOutput(ctx, repoPath, "rev-parse", "--verify", "--quiet", candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func (CLIClient) changeSummary(ctx context.Context, repoPath string) (ChangeSummary, error) {
	status, err := gitOutput(ctx, repoPath, "status", "--porcelain")
	if err != nil {
		return ChangeSummary{}, fmt.Errorf("read git status: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(status), "\n")
	files := make([]ChangedFile, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		statusText := strings.TrimSpace(line[:min(len(line), 2)])
		path := strings.TrimSpace(line[min(len(line), 3):])
		if statusText == "" || path == "" {
			continue
		}
		files = append(files, ChangedFile{Path: path, Status: statusText})
	}
	return ChangeSummary{Dirty: len(files) > 0, Files: files}, nil
}

func (CLIClient) localCommits(ctx context.Context, repoPath, base string) ([]Commit, error) {
	if strings.TrimSpace(base) == "" {
		return nil, nil
	}
	output, err := gitOutput(ctx, repoPath, "log", base+"..HEAD", "--format=%H%x00%s%x00%b%x1e")
	if err != nil {
		return nil, fmt.Errorf("read local commits: %w", err)
	}
	return parseCommitLog(output), nil
}

func readCommit(ctx context.Context, repoPath, ref string) (Commit, error) {
	sha, err := gitOutput(ctx, repoPath, "rev-parse", ref)
	if err != nil {
		return Commit{}, fmt.Errorf("read commit sha: %w", err)
	}
	subject, err := gitOutput(ctx, repoPath, "log", "-1", "--format=%s", ref)
	if err != nil {
		return Commit{}, fmt.Errorf("read commit subject: %w", err)
	}
	body, err := gitOutput(ctx, repoPath, "log", "-1", "--format=%b", ref)
	if err != nil {
		return Commit{}, fmt.Errorf("read commit body: %w", err)
	}
	return Commit{SHA: strings.TrimSpace(sha), Subject: strings.TrimSpace(subject), Body: strings.TrimSpace(body)}, nil
}

func parseCommitLog(output string) []Commit {
	output = strings.Trim(output, "\x1e\n\r\t ")
	if output == "" {
		return nil
	}
	records := strings.Split(output, "\x1e")
	commits := make([]Commit, 0, len(records))
	for _, record := range records {
		record = strings.Trim(record, "\n\r\t ")
		if record == "" {
			continue
		}
		parts := strings.SplitN(record, "\x00", 3)
		if len(parts) < 2 {
			continue
		}
		commit := Commit{SHA: strings.TrimSpace(parts[0]), Subject: strings.TrimSpace(parts[1])}
		if len(parts) > 2 {
			commit.Body = strings.TrimSpace(parts[2])
		}
		if commit.SHA != "" {
			commits = append(commits, commit)
		}
	}
	return commits
}

func DetectIssueKey(value string) string {
	match := issueKeyPattern.FindStringSubmatch(strings.ToUpper(value))
	if len(match) < 2 {
		return ""
	}
	return match[1]
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
