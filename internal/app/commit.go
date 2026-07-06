package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jcharette/jira-tui/internal/claude"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/gitstate"
	"github.com/jcharette/jira-tui/internal/gitworkflow"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/spf13/cobra"
)

const maxCommitAINoteBytes = 1600

type commitJiraClient interface {
	GetIssue(ctx context.Context, key string) (jira.IssueDetail, error)
	AddComment(ctx context.Context, key string, body string, mentions []jira.Mention) (jira.Comment, error)
}

type commitStateStore interface {
	ReportedCommits(ctx context.Context, repoPath, branch, issueKey string) ([]gitstate.ReportedCommit, error)
	MarkReported(ctx context.Context, records []gitstate.ReportedCommit) error
}

type commitReview struct {
	Plan       gitworkflow.CommitPlan
	AIDrafted  bool
	AIDraftErr string
}

type commitConfirmFunc func(out io.Writer, review commitReview) bool

type commitOptions struct {
	NoteDrafter commitNoteDrafter
}

type commitNoteDrafter interface {
	DraftCommitNote(context.Context, commitNoteDraftRequest) (string, error)
}

type commitNoteDraftRequest struct {
	Plan  gitworkflow.CommitPlan
	Issue jira.Issue
}

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
	return runCommitWithDepsAndOptions(ctx, args, out, gitClient, jira.NewClient(cfg), stateStore, defaultCommitConfirm, commitOptions{
		NoteDrafter: commitNoteDrafterFromConfig(cfg),
	})
}

func runCommitWithDeps(ctx context.Context, args []string, out io.Writer, gitClient gitworkflow.Client, jiraClient commitJiraClient, stateStore commitStateStore, confirm commitConfirmFunc) error {
	return runCommitWithDepsAndOptions(ctx, args, out, gitClient, jiraClient, stateStore, confirm, commitOptions{})
}

func runCommitWithDepsAndOptions(ctx context.Context, args []string, out io.Writer, gitClient gitworkflow.Client, jiraClient commitJiraClient, stateStore commitStateStore, confirm commitConfirmFunc, options commitOptions) error {
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
	aiDrafted := false
	aiDraftErr := ""
	if options.NoteDrafter != nil && plan.ShouldReport {
		note, err := options.NoteDrafter.DraftCommitNote(ctx, commitNoteDraftRequest{Plan: plan, Issue: detail.Issue})
		if err != nil {
			aiDraftErr = err.Error()
		} else if cleaned := cleanCommitAINote(note); cleaned != "" {
			plan.JiraNote = cleaned
			aiDrafted = true
		} else {
			aiDraftErr = "empty note"
		}
	}
	review := commitReview{Plan: plan, AIDrafted: aiDrafted, AIDraftErr: aiDraftErr}
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
		if review.AIDrafted {
			_, _ = fmt.Fprintln(out, "AI drafted Jira note: yes")
		} else if strings.TrimSpace(review.AIDraftErr) != "" {
			_, _ = fmt.Fprintf(out, "AI drafted Jira note: no (%s)\n", review.AIDraftErr)
		}
		_, _ = fmt.Fprintln(out, "Jira note:")
		_, _ = fmt.Fprintln(out, plan.JiraNote)
	}
	if plan.ShouldPush {
		_, _ = fmt.Fprintln(out, "Push: yes")
	}
}

type claudeCommitNoteDrafter struct {
	Config claude.Config
	Runner claude.LocalRunner
}

func (d claudeCommitNoteDrafter) DraftCommitNote(ctx context.Context, request commitNoteDraftRequest) (string, error) {
	result, err := d.Runner.Run(ctx, claude.Request{
		Config: d.Config,
		Prompt: buildCommitNotePrompt(request),
	})
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func buildCommitNotePrompt(request commitNoteDraftRequest) string {
	plan := request.Plan
	var b strings.Builder
	fmt.Fprintf(&b, "Draft a compact Jira progress note for %s.\n", plan.IssueKey)
	b.WriteString("Do not edit files, create commits, call Jira, call GitHub, run git commands, or make external changes.\n")
	fmt.Fprintf(&b, "Ticket summary: %s\n", displayValue(plan.IssueSummary, request.Issue.Summary))
	b.WriteString("Return only the Jira note. Keep it under 6 bullets and under 1200 characters.\n")
	if plan.ShouldCommit {
		fmt.Fprintf(&b, "Pending commit message: %s\n", plan.DefaultCommitMessage)
	}
	if len(plan.Changes.Files) > 0 {
		b.WriteString("Changed files:\n")
		for _, file := range plan.Changes.Files {
			fmt.Fprintf(&b, "- %s %s\n", strings.TrimSpace(file.Status), file.Path)
		}
	}
	if len(plan.UnreportedCommits) > 0 {
		b.WriteString("Unreported commits:\n")
		for _, commit := range plan.UnreportedCommits {
			fmt.Fprintf(&b, "- %s %s\n", shortCommitSHA(commit.SHA), displayValue(commit.Subject, "(no subject)"))
		}
	}
	return strings.TrimSpace(b.String())
}

func cleanCommitAINote(note string) string {
	return cleanBoundedText(note, maxCommitAINoteBytes)
}

func oneLineBounded(value string, maxBytes int) string {
	return cleanBoundedText(strings.Join(strings.Fields(value), " "), maxBytes)
}

func cleanBoundedText(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if value == "" || maxBytes <= 0 {
		return ""
	}
	if len([]byte(value)) <= maxBytes {
		return value
	}
	var b strings.Builder
	for _, r := range value {
		if b.Len()+len(string(r)) > maxBytes {
			break
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func commitNoteDrafterFromConfig(cfg config.Config) commitNoteDrafter {
	if !cfg.Claude.Enabled || !cfg.Claude.Features.BranchPlan {
		return nil
	}
	claudeConfig, ok := checkedClaudeConfig(cfg)
	if !ok {
		return nil
	}
	return claudeCommitNoteDrafter{Config: claudeConfig}
}

func checkedClaudeConfig(cfg config.Config) (claude.Config, bool) {
	claudeConfig := claude.Config{
		Enabled: cfg.Claude.Enabled,
		Command: cfg.Claude.Command,
		Timeout: cfg.Claude.Timeout,
	}
	status := claude.LocalRunner{}.Check(context.Background(), claudeConfig)
	if !status.Enabled || !status.Available {
		return claude.Config{}, false
	}
	if status.Command != "" {
		claudeConfig.Command = status.Command
	}
	return claudeConfig, true
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
