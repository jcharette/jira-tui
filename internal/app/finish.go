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
	"github.com/jcharette/jira-tui/internal/prprovider"
	"github.com/spf13/cobra"
)

type finishJiraClient interface {
	commitJiraClient
	GetTransitions(ctx context.Context, key string) ([]jira.Transition, error)
	TransitionIssue(ctx context.Context, key string, request jira.TransitionIssueRequest) error
}

type finishReview struct {
	Plan           gitworkflow.FinishPlan
	Transition     jira.Transition
	HasTransition  bool
	TransitionSkip string
}

type finishConfirmFunc func(out io.Writer, review finishReview) bool

func newFinishCommand(profile *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finish [ticket]",
		Short: "Push current work, open a pull request, and finish Jira work",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFinish(*profile, args, cmd.OutOrStdout(), gitworkflow.NewCLIClient(), prprovider.NewGitHubCLIProvider())
		},
	}
	return cmd
}

func runFinish(profile string, args []string, out io.Writer, gitClient gitworkflow.Client, prProvider prprovider.Provider) error {
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
	return runFinishWithDeps(ctx, args, out, gitClient, jira.NewClient(cfg), stateStore, prProvider, defaultFinishConfirm)
}

func runFinishWithDeps(ctx context.Context, args []string, out io.Writer, gitClient gitworkflow.Client, jiraClient finishJiraClient, stateStore commitStateStore, prProvider prprovider.Provider, confirm finishConfirmFunc) error {
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
	plan := gitworkflow.BuildFinishPlan(analysis, detail.Issue, reportedSHAs(reported))
	transition, hasTransition, transitionSkip := finishTransition(ctx, jiraClient, plan.CommitPlan.IssueKey)
	review := finishReview{
		Plan:           plan,
		Transition:     transition,
		HasTransition:  hasTransition,
		TransitionSkip: transitionSkip,
	}
	if confirm == nil {
		confirm = defaultFinishConfirm
	}
	if !confirm(out, review) {
		_, _ = fmt.Fprintln(out, "Finish canceled.")
		return nil
	}

	var reportedCommits []gitworkflow.Commit
	reportedCommits = append(reportedCommits, plan.CommitPlan.UnreportedCommits...)
	if plan.CommitPlan.ShouldCommit {
		commit, err := gitClient.CommitAll(ctx, plan.CommitPlan.RepoPath, plan.CommitPlan.DefaultCommitMessage)
		if err != nil {
			return fmt.Errorf("commit changes: %w", err)
		}
		reportedCommits = append(reportedCommits, commit)
		_, _ = fmt.Fprintf(out, "Committed %s %s\n", shortCommitSHA(commit.SHA), commit.Subject)
	}
	if plan.CommitPlan.ShouldReport {
		if _, err := jiraClient.AddComment(ctx, plan.CommitPlan.IssueKey, plan.CommitPlan.JiraNote, nil); err != nil {
			return fmt.Errorf("add Jira commit note: %w", err)
		}
		if err := stateStore.MarkReported(ctx, reportedCommitRecords(plan.CommitPlan, reportedCommits)); err != nil {
			return fmt.Errorf("save reported commit state: %w", err)
		}
		_, _ = fmt.Fprintf(out, "Reported progress to %s.\n", plan.CommitPlan.IssueKey)
	}
	if err := gitClient.PushCurrentBranch(ctx, plan.CommitPlan.RepoPath); err != nil {
		return fmt.Errorf("push branch: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Pushed %s.\n", displayValue(plan.CommitPlan.Branch, "current branch"))

	pr, err := prProvider.CreateOrUpdatePR(ctx, prprovider.Request{
		RepoPath:   plan.CommitPlan.RepoPath,
		Branch:     plan.CommitPlan.Branch,
		BaseBranch: plan.CommitPlan.BaseBranch,
		Title:      plan.PRTitle,
		Body:       plan.PRBody,
		Draft:      true,
	})
	if err != nil {
		return fmt.Errorf("open pull request: %w", err)
	}
	if pr.Created {
		_, _ = fmt.Fprintf(out, "Created pull request: %s\n", pr.URL)
	} else {
		_, _ = fmt.Fprintf(out, "Using pull request: %s\n", pr.URL)
	}
	finalNote := finishJiraNote(plan.FinalJiraNote, pr.URL)
	if _, err := jiraClient.AddComment(ctx, plan.CommitPlan.IssueKey, finalNote, nil); err != nil {
		return fmt.Errorf("add Jira finish note: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Posted final note to %s.\n", plan.CommitPlan.IssueKey)
	if hasTransition {
		if err := jiraClient.TransitionIssue(ctx, plan.CommitPlan.IssueKey, jira.TransitionIssueRequest{TransitionID: transition.ID}); err != nil {
			return fmt.Errorf("transition Jira issue: %w", err)
		}
		_, _ = fmt.Fprintf(out, "Transitioned %s to %s.\n", plan.CommitPlan.IssueKey, displayValue(transition.ToStatus, transition.Name))
	} else if transitionSkip != "" {
		_, _ = fmt.Fprintf(out, "Skipped transition: %s.\n", transitionSkip)
	}
	return nil
}

func defaultFinishConfirm(out io.Writer, review finishReview) bool {
	writeFinishReview(out, review)
	_, _ = fmt.Fprint(out, "\nApply these actions? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func writeFinishReview(out io.Writer, review finishReview) {
	plan := review.Plan
	_, _ = fmt.Fprintf(out, "Finish workflow for %s\n", plan.CommitPlan.IssueKey)
	_, _ = fmt.Fprintf(out, "Repo: %s\n", plan.CommitPlan.RepoPath)
	_, _ = fmt.Fprintf(out, "Branch: %s\n", displayValue(plan.CommitPlan.Branch, "current branch"))
	if plan.CommitPlan.ShouldCommit {
		_, _ = fmt.Fprintf(out, "Commit message: %s\n", plan.CommitPlan.DefaultCommitMessage)
	}
	if len(plan.CommitPlan.UnreportedCommits) > 0 {
		_, _ = fmt.Fprintln(out, "Unreported local commits:")
		for _, commit := range plan.CommitPlan.UnreportedCommits {
			_, _ = fmt.Fprintf(out, "- %s %s\n", shortCommitSHA(commit.SHA), displayValue(commit.Subject, "(no subject)"))
		}
	}
	if plan.CommitPlan.ShouldReport {
		_, _ = fmt.Fprintln(out, "Jira progress note:")
		_, _ = fmt.Fprintln(out, plan.CommitPlan.JiraNote)
	}
	_, _ = fmt.Fprintf(out, "Pull request title: %s\n", plan.PRTitle)
	_, _ = fmt.Fprintln(out, "Pull request body:")
	_, _ = fmt.Fprintln(out, plan.PRBody)
	_, _ = fmt.Fprintln(out, "Final Jira note:")
	_, _ = fmt.Fprintln(out, plan.FinalJiraNote)
	if review.HasTransition {
		_, _ = fmt.Fprintf(out, "Transition: %s\n", displayValue(review.Transition.ToStatus, review.Transition.Name))
	} else if review.TransitionSkip != "" {
		_, _ = fmt.Fprintf(out, "Transition: skipped (%s)\n", review.TransitionSkip)
	}
}

func finishTransition(ctx context.Context, jiraClient finishJiraClient, issueKey string) (jira.Transition, bool, string) {
	transitions, err := jiraClient.GetTransitions(ctx, issueKey)
	if err != nil {
		return jira.Transition{}, false, "could not load transitions"
	}
	if transition, ok := chooseFinishTransition(transitions); ok {
		return transition, true, ""
	}
	return jira.Transition{}, false, "no safe terminal transition available"
}

func chooseFinishTransition(transitions []jira.Transition) (jira.Transition, bool) {
	bestIndex := -1
	bestScore := 0
	for index, transition := range transitions {
		if !transition.IsAvailable || hasRequiredFields(transition) {
			continue
		}
		score := finishTransitionScore(transition)
		if score > bestScore {
			bestIndex = index
			bestScore = score
		}
	}
	if bestIndex < 0 {
		return jira.Transition{}, false
	}
	return transitions[bestIndex], true
}

func finishTransitionScore(transition jira.Transition) int {
	text := strings.ToLower(strings.Join([]string{transition.ToStatus, transition.Name}, " "))
	score := 0
	for _, keyword := range []string{"done", "closed", "resolved", "complete", "finished"} {
		if strings.Contains(text, keyword) {
			score += 10
		}
	}
	if strings.EqualFold(strings.TrimSpace(transition.ToStatus), "Done") {
		score += 5
	}
	return score
}

func hasRequiredFields(transition jira.Transition) bool {
	for _, field := range transition.Fields {
		if field.Required {
			return true
		}
	}
	return false
}

func finishJiraNote(note string, prURL string) string {
	note = strings.TrimSpace(note)
	prURL = strings.TrimSpace(prURL)
	if prURL == "" {
		return note
	}
	if note == "" {
		return "Ready for review:\n- Pull request: " + prURL
	}
	return note + "\n- Pull request: " + prURL
}
