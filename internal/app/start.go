package app

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jcharette/jira-tui/internal/claude"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/gitworkflow"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/startworkflow"
	"github.com/spf13/cobra"
)

const startIssueSearchLimit = 25

var runStartWorkflow = startworkflow.Run

const maxStartPlanBytes = 3000

type startOptions struct {
	PlanDrafter startPlanDrafter
}

type startPlanDrafter interface {
	DraftStartPlan(context.Context, startworkflow.PlanDraftRequest) (string, error)
}

func newStartCommand(profile *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [ticket]",
		Short: "Start work on a Jira ticket",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(*profile, args, cmd.OutOrStdout(), gitworkflow.NewCLIClient())
		},
	}
	return cmd
}

func runStart(profile string, args []string, out io.Writer, gitClient gitworkflow.Client) error {
	cfg, err := loadConfigOrConfigure(profile)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), maxDuration(cfg.RequestTimeout, 30*time.Second))
	defer cancel()

	client := jira.NewClient(cfg)
	issue, issues, err := startIssues(ctx, client, cfg, args)
	if err != nil {
		return err
	}
	repo, _ := gitClient.DetectRepo(ctx, ".")
	options := startOptions{PlanDrafter: startPlanDrafterFromConfig(cfg)}
	planText, planErr := draftStartPlan(ctx, options.PlanDrafter, issue, repo, cfg)
	result, err := runStartWorkflow(startworkflow.Options{
		Config:        cfg,
		Issue:         issue,
		Issues:        issues,
		PreferredRepo: repo,
		PlanText:      planText,
		PlanErr:       planErr,
	})
	if err != nil {
		return fmt.Errorf("start workflow: %w", err)
	}
	if result.Canceled || !result.Confirmed {
		_, _ = fmt.Fprintln(out, "Start canceled.")
		return nil
	}
	outcomes := applyStartActions(ctx, gitClient, client, result)
	writeStartSummary(out, result, outcomes)
	if err := firstStartActionError(outcomes); err != nil {
		return err
	}
	return nil
}

func startIssues(ctx context.Context, client *jira.Client, cfg config.Config, args []string) (*jira.Issue, []jira.Issue, error) {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		key := strings.ToUpper(strings.TrimSpace(args[0]))
		detail, err := client.GetIssue(ctx, key)
		if err != nil {
			return nil, nil, fmt.Errorf("load issue %s: %w", key, err)
		}
		issue := detail.Issue
		return &issue, nil, nil
	}
	issues, err := client.SearchIssues(ctx, cfg.DefaultJQL, startIssueSearchLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("load start ticket picker: %w", err)
	}
	return nil, issues, nil
}

func draftStartPlan(ctx context.Context, drafter startPlanDrafter, issue *jira.Issue, repo gitworkflow.RepoStatus, cfg config.Config) (string, string) {
	if drafter == nil || issue == nil {
		return "", ""
	}
	branch := gitworkflow.RenderBranchName(cfg.Git.BranchTemplate, *issue)
	text, err := drafter.DraftStartPlan(ctx, startworkflow.PlanDraftRequest{
		Issue:      *issue,
		RepoPath:   repo.Path,
		BranchName: branch,
		Actions:    startworkflow.DefaultActions(branch),
	})
	if err != nil {
		return "", err.Error()
	}
	text = cleanBoundedText(text, maxStartPlanBytes)
	if text == "" {
		return "", "empty plan"
	}
	return text, ""
}

type claudeStartPlanDrafter struct {
	Config claude.Config
	Runner claude.LocalRunner
}

func (d claudeStartPlanDrafter) DraftStartPlan(ctx context.Context, request startworkflow.PlanDraftRequest) (string, error) {
	result, err := d.Runner.Run(ctx, claude.Request{
		Config: d.Config,
		Prompt: startworkflow.BuildPlanDraftPrompt(request),
	})
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func startPlanDrafterFromConfig(cfg config.Config) startPlanDrafter {
	if !cfg.Claude.Enabled || !cfg.Claude.Features.BranchPlan {
		return nil
	}
	claudeConfig, ok := checkedClaudeConfig(cfg)
	if !ok {
		return nil
	}
	return claudeStartPlanDrafter{Config: claudeConfig}
}

type startActionOutcome = startworkflow.Outcome

func applyStartActions(ctx context.Context, gitClient gitworkflow.Client, jiraClient *jira.Client, result startworkflow.Result) []startActionOutcome {
	return startworkflow.ApplyActions(ctx, gitClient, jiraClient, result)
}

func chooseStartTransition(transitions []jira.Transition) (jira.Transition, bool) {
	return startworkflow.ChooseStartTransition(transitions)
}

func writeStartSummary(out io.Writer, result startworkflow.Result, outcomes []startActionOutcome) {
	_, _ = fmt.Fprintf(out, "Started %s.\n", result.Issue.Key)
	_, _ = fmt.Fprintf(out, "Repo: %s\n", result.RepoPath)
	_, _ = fmt.Fprintf(out, "Branch: %s\n", result.BranchName)
	for _, outcome := range outcomes {
		if strings.TrimSpace(outcome.Label) == "" {
			continue
		}
		if outcome.Err != nil {
			_, _ = fmt.Fprintf(out, "%s: %s (%v)\n", outcome.Label, outcome.State, outcome.Err)
			continue
		}
		_, _ = fmt.Fprintf(out, "%s: %s\n", outcome.Label, outcome.State)
	}
}

func firstStartActionError(outcomes []startActionOutcome) error {
	for _, outcome := range outcomes {
		if outcome.Err != nil {
			return outcome.Err
		}
	}
	return nil
}

func maxDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
