package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jcharette/jira-tui/internal/gitstate"
	"github.com/jcharette/jira-tui/internal/gitworkflow"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/spf13/cobra"
)

type commitJiraClient interface {
	GetIssue(ctx context.Context, key string) (jira.IssueDetail, error)
	AddComment(ctx context.Context, key string, body string, mentions []jira.Mention) (jira.Comment, error)
}

type commitStateStore interface {
	ReportedCommits(ctx context.Context, repoPath, branch, issueKey string) ([]gitstate.ReportedCommit, error)
	MarkReported(ctx context.Context, records []gitstate.ReportedCommit) error
}

type commitReview struct {
	Plan gitworkflow.CommitPlan
}

type commitConfirmFunc func(out io.Writer, review commitReview) bool

func newCommitCommand(profile *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit [ticket]",
		Short: "Commit current work and report progress to Jira",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommit(*profile, args, cmd.OutOrStdout(), gitworkflow.NewCLIClient())
		},
	}
	return cmd
}

func runCommit(profile string, args []string, out io.Writer, gitClient gitworkflow.Client) error {
	cfg, err := loadConfigOrConfigure(profile)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), maxDuration(cfg.RequestTimeout, 30*time.Second))
	defer cancel()

	stateStore, err := gitstate.OpenDefault()
	if err != nil {
		return fmt.Errorf("open git workflow state: %w", err)
	}
	return runCommitWithDeps(ctx, args, out, gitClient, jira.NewClient(cfg), stateStore, defaultCommitConfirm)
}

func runCommitWithDeps(ctx context.Context, args []string, out io.Writer, gitClient gitworkflow.Client, jiraClient commitJiraClient, stateStore commitStateStore, confirm commitConfirmFunc) error {
	analysis, err := gitClient.Analyze(ctx, ".")
	if err != nil {
		return fmt.Errorf("analyze git repo: %w", err)
	}
	issueKey := resolveCommitIssueKey(args, analysis)
	if issueKey == "" {
		return fmt.Errorf("ticket is required when the current branch does not include a Jira issue key")
	}
	detail, err := jiraClient.GetIssue(ctx, issueKey)
	if err != nil {
		return fmt.Errorf("load issue %s: %w", issueKey, err)
	}
	reported, err := stateStore.ReportedCommits(ctx, analysis.Repo.Path, analysis.Repo.CurrentBranch, issueKey)
	if err != nil {
		return fmt.Errorf("load reported commit state: %w", err)
	}
	plan := gitworkflow.BuildCommitPlan(analysis, detail.Issue, reportedSHAs(reported))
	if !plan.ShouldCommit && !plan.ShouldReport && !plan.ShouldPush {
		_, _ = fmt.Fprintf(out, "Nothing to commit or report for %s.\n", plan.IssueKey)
		return nil
	}
	review := commitReview{Plan: plan}
	if confirm == nil {
		confirm = defaultCommitConfirm
	}
	if !confirm(out, review) {
		_, _ = fmt.Fprintln(out, "Commit canceled.")
		return nil
	}

	var reportedCommits []gitworkflow.Commit
	reportedCommits = append(reportedCommits, plan.UnreportedCommits...)
	if plan.ShouldCommit {
		commit, err := gitClient.CommitAll(ctx, plan.RepoPath, plan.DefaultCommitMessage)
		if err != nil {
			return fmt.Errorf("commit changes: %w", err)
		}
		reportedCommits = append(reportedCommits, commit)
		_, _ = fmt.Fprintf(out, "Committed %s %s\n", shortCommitSHA(commit.SHA), commit.Subject)
	}
	if plan.ShouldReport {
		if _, err := jiraClient.AddComment(ctx, plan.IssueKey, plan.JiraNote, nil); err != nil {
			return fmt.Errorf("add Jira commit note: %w", err)
		}
		if err := stateStore.MarkReported(ctx, reportedCommitRecords(plan, reportedCommits)); err != nil {
			return fmt.Errorf("save reported commit state: %w", err)
		}
		_, _ = fmt.Fprintf(out, "Reported progress to %s.\n", plan.IssueKey)
	}
	if plan.ShouldPush {
		if err := gitClient.PushCurrentBranch(ctx, plan.RepoPath); err != nil {
			return fmt.Errorf("push branch: %w", err)
		}
		_, _ = fmt.Fprintf(out, "Pushed %s.\n", displayValue(plan.Branch, "current branch"))
	}
	return nil
}

func defaultCommitConfirm(out io.Writer, review commitReview) bool {
	writeCommitReview(out, review)
	_, _ = fmt.Fprint(out, "\nApply these actions? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func writeCommitReview(out io.Writer, review commitReview) {
	plan := review.Plan
	_, _ = fmt.Fprintf(out, "Commit workflow for %s\n", plan.IssueKey)
	_, _ = fmt.Fprintf(out, "Repo: %s\n", plan.RepoPath)
	_, _ = fmt.Fprintf(out, "Branch: %s\n", displayValue(plan.Branch, "current branch"))
	if plan.ShouldCommit {
		_, _ = fmt.Fprintf(out, "Commit message: %s\n", plan.DefaultCommitMessage)
		if len(plan.Changes.Files) > 0 {
			_, _ = fmt.Fprintln(out, "Changed files:")
			for _, file := range plan.Changes.Files {
				_, _ = fmt.Fprintf(out, "- %s %s\n", strings.TrimSpace(file.Status), file.Path)
			}
		}
	}
	if len(plan.UnreportedCommits) > 0 {
		_, _ = fmt.Fprintln(out, "Unreported local commits:")
		for _, commit := range plan.UnreportedCommits {
			_, _ = fmt.Fprintf(out, "- %s %s\n", shortCommitSHA(commit.SHA), displayValue(commit.Subject, "(no subject)"))
		}
	}
	if plan.ShouldReport {
		_, _ = fmt.Fprintln(out, "Jira note:")
		_, _ = fmt.Fprintln(out, plan.JiraNote)
	}
	if plan.ShouldPush {
		_, _ = fmt.Fprintln(out, "Push: yes")
	}
}

func resolveCommitIssueKey(args []string, analysis gitworkflow.Analysis) string {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return strings.ToUpper(strings.TrimSpace(args[0]))
	}
	return strings.ToUpper(strings.TrimSpace(analysis.IssueKey))
}

func reportedSHAs(records []gitstate.ReportedCommit) map[string]bool {
	shas := make([]string, 0, len(records))
	for _, record := range records {
		shas = append(shas, record.SHA)
	}
	return gitworkflow.ReportedSHAMap(shas)
}

func reportedCommitRecords(plan gitworkflow.CommitPlan, commits []gitworkflow.Commit) []gitstate.ReportedCommit {
	records := make([]gitstate.ReportedCommit, 0, len(commits))
	for _, commit := range commits {
		if strings.TrimSpace(commit.SHA) == "" {
			continue
		}
		records = append(records, gitstate.ReportedCommit{
			RepoPath: plan.RepoPath,
			Branch:   plan.Branch,
			IssueKey: plan.IssueKey,
			SHA:      strings.TrimSpace(commit.SHA),
			Subject:  strings.TrimSpace(commit.Subject),
		})
	}
	return records
}

func shortCommitSHA(sha string) string {
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
