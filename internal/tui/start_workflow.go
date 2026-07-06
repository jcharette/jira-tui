package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/events"
	"github.com/jcharette/jira-tui/internal/gitworkflow"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/startworkflow"
	"github.com/jcharette/jira-tui/internal/worker"
)

const maxStartPlanBytes = 3000

type startRepoDetectedMsg struct {
	id    int
	issue jira.Issue
	repo  gitworkflow.RepoStatus
	err   error
}

type startPlanResultMsg struct {
	id    int
	issue jira.Issue
	repo  gitworkflow.RepoStatus
	text  string
	err   error
}

type startBranchResultMsg struct {
	id              int
	result          startworkflow.Result
	outcome         startworkflow.Outcome
	branchSucceeded bool
}

func (m Model) startSelectedIssueWorkflow() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		m.detailNotice = "Select an issue before starting work."
		return m, nil
	}
	m.nextRequestID++
	m.activeStartRepoReqID = m.nextRequestID
	m.startWorkflowOpen = true
	m.startWorkflowPreparing = true
	m.startWorkflowPlanning = false
	m.startWorkflowApplying = false
	m.startWorkflowIssue = selected
	m.startWorkflowResult = startworkflow.Result{}
	m.startWorkflowOutcomes = nil
	m.startWorkflowBranchSucceeded = false
	m.startWorkflowErr = nil
	return m, m.detectStartRepo(m.activeStartRepoReqID, selected)
}

func (m Model) detectStartRepo(id int, issue jira.Issue) tea.Cmd {
	client := m.gitClient
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), startWorkflowTimeout(m.requestTimeout))
		defer cancel()
		repo, err := client.DetectRepo(ctx, ".")
		return startRepoDetectedMsg{id: id, issue: issue, repo: repo, err: err}
	}
}

func (m Model) handleStartRepoDetected(msg startRepoDetectedMsg) (Model, tea.Cmd) {
	if msg.id != m.activeStartRepoReqID {
		return m, nil
	}
	m.startWorkflowPreparing = false
	m.startWorkflowErr = msg.err
	cfg := configDefaultsForStart(m.gitConfig)
	if m.claudeStartPlanAvailable() {
		m.startWorkflowPreparing = true
		m.startWorkflowPlanning = true
		return m, m.submitStartPlan(msg.id, msg.issue, msg.repo, cfg)
	}
	m.startWorkflow = startworkflow.NewModel(startworkflow.Options{
		Config:        cfg,
		Issue:         &msg.issue,
		PreferredRepo: msg.repo,
	})
	return m, nil
}

func (m Model) handleStartPlanResult(msg startPlanResultMsg) (Model, tea.Cmd) {
	if msg.id != m.activeStartRepoReqID {
		return m, nil
	}
	m.startWorkflowPreparing = false
	m.startWorkflowPlanning = false
	cfg := configDefaultsForStart(m.gitConfig)
	planText := cleanStartPlanText(msg.text)
	planErr := ""
	if msg.err != nil {
		planErr = msg.err.Error()
	} else if strings.TrimSpace(msg.text) != "" && planText == "" {
		planErr = "empty plan"
	}
	m.startWorkflow = startworkflow.NewModel(startworkflow.Options{
		Config:        cfg,
		Issue:         &msg.issue,
		PreferredRepo: msg.repo,
		PlanText:      planText,
		PlanErr:       planErr,
	})
	return m, nil
}

func (m Model) updateStartWorkflow(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.startWorkflowPreparing {
		if msg.String() == "esc" {
			m.closeStartWorkflow()
		}
		return m, nil
	}
	if m.startWorkflowApplying {
		return m, nil
	}
	if m.startWorkflowResult.Confirmed || m.startWorkflowResult.Canceled {
		switch msg.String() {
		case "enter", "esc":
			m.closeStartWorkflow()
		}
		return m, nil
	}
	next, cmd := m.startWorkflow.Update(msg)
	if typed, ok := next.(startworkflow.Model); ok {
		m.startWorkflow = typed
	}
	result := m.startWorkflow.Result()
	if result.Canceled {
		m.closeStartWorkflow()
		return m, nil
	}
	if result.Confirmed {
		m.startWorkflowResult = result
		m.startWorkflowApplying = true
		m.nextRequestID++
		m.activeStartBranchReqID = m.nextRequestID
		return m, m.applyStartBranch(m.activeStartBranchReqID, result)
	}
	return m, cmd
}

func (m Model) applyStartBranch(id int, result startworkflow.Result) tea.Cmd {
	client := m.gitClient
	return func() tea.Msg {
		outcome := startworkflow.Outcome{
			Kind:  startworkflow.ActionBranch,
			Label: startActionLabel(result, startworkflow.ActionBranch),
		}
		branchAction, ok := startAction(result, startworkflow.ActionBranch)
		if ok && branchAction.Skip {
			outcome.State = "skipped"
			return startBranchResultMsg{id: id, result: result, outcome: outcome}
		}
		ctx, cancel := context.WithTimeout(context.Background(), startWorkflowTimeout(m.requestTimeout))
		defer cancel()
		if err := client.CreateOrSwitchBranch(ctx, result.RepoPath, result.BranchName); err != nil {
			outcome.State = "failed"
			outcome.Err = err
			return startBranchResultMsg{id: id, result: result, outcome: outcome}
		}
		outcome.State = "completed"
		return startBranchResultMsg{id: id, result: result, outcome: outcome, branchSucceeded: true}
	}
}

func (m Model) handleStartBranchResult(msg startBranchResultMsg) (Model, tea.Cmd) {
	if msg.id != m.activeStartBranchReqID {
		return m, nil
	}
	m.startWorkflowBranchSucceeded = msg.branchSucceeded
	m.startWorkflowOutcomes = append(m.startWorkflowOutcomes, msg.outcome)
	if !hasRunnableJiraStartActions(msg.result) {
		m.startWorkflowApplying = false
		m.startWorkflowOutcomes = append(m.startWorkflowOutcomes, skippedJiraStartOutcomes(msg.result)...)
		m.detailNotice = startWorkflowNotice(msg.result, m.startWorkflowOutcomes)
		return m, nil
	}
	m.nextRequestID++
	m.activeStartIssueReqID = m.nextRequestID
	return m, m.submitStartIssue(m.activeStartIssueReqID, msg.result, msg.branchSucceeded)
}

func (m Model) handleStartIssueResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeStartIssueReqID {
		return m, nil
	}
	m.startWorkflowApplying = false
	if result.Err != nil {
		m.startWorkflowOutcomes = append(m.startWorkflowOutcomes, startworkflow.Outcome{
			Kind:  startworkflow.ActionTransition,
			Label: "Apply Jira updates",
			State: "failed",
			Err:   result.Err,
		})
		m.detailNotice = "Start Work failed: " + result.Err.Error()
		return m, nil
	}
	if result.StartIssue == nil {
		m.detailNotice = "Start Work failed: " + worker.ErrInvalidRequest.Error()
		return m, nil
	}
	m.startWorkflowOutcomes = append(m.startWorkflowOutcomes, result.StartIssue.Outcomes...)
	m.detailNotice = startWorkflowNotice(m.startWorkflowResult, m.startWorkflowOutcomes)
	if startWorkflowChangedJira(result.StartIssue.Outcomes) {
		key := strings.TrimSpace(result.StartIssue.Key)
		if key != "" {
			delete(m.details, key)
			if m.detailCache != nil {
				m.detailCache.Delete(key)
			}
		}
		return m.startDetailRequestForSelected()
	}
	return m, nil
}

func (m Model) renderStartWorkflowDialog(width int) string {
	selectedKey := displayValue(m.startWorkflowIssue.Key, "selected ticket")
	if m.startWorkflowPreparing {
		label := "Detecting repository..."
		if m.startWorkflowPlanning {
			label = "Asking Claude for start plan..."
		}
		body := m.detailStatusBlock(label, max(24, min(width-12, 64)), false)
		return m.renderDetailDialogWithLimit(width, "Start Work", selectedKey, body, "esc cancel", 84)
	}
	if m.startWorkflowApplying {
		lines := []string{
			m.detailStatusBlock("Applying confirmed Start actions...", max(24, min(width-12, 64)), false),
		}
		if len(m.startWorkflowOutcomes) > 0 {
			lines = append(lines, "", m.renderStartWorkflowOutcomes(max(24, min(width-12, 64))))
		}
		return m.renderDetailDialogWithLimit(width, "Start Work", selectedKey, strings.Join(lines, "\n"), "applying", 84)
	}
	if m.startWorkflowResult.Confirmed {
		body := m.renderStartWorkflowOutcomes(max(24, min(width-12, 64)))
		return m.renderDetailDialogWithLimit(width, "Start Work", selectedKey, body, "enter close", 84)
	}
	body := m.startWorkflow.View().Content
	if m.startWorkflowErr != nil {
		body = m.theme.Warning.Render("Repo detection failed: "+m.startWorkflowErr.Error()) + "\n\n" + body
	}
	return m.renderDetailDialogWithLimit(width, "Start Work", selectedKey, body, "enter continue  esc cancel", 92)
}

func (m Model) renderStartWorkflowOutcomes(width int) string {
	if len(m.startWorkflowOutcomes) == 0 {
		return m.theme.Muted.Render("No actions completed.")
	}
	lines := make([]string, 0, len(m.startWorkflowOutcomes))
	for _, outcome := range m.startWorkflowOutcomes {
		label := displayValue(outcome.Label, string(outcome.Kind))
		state := displayValue(outcome.State, "pending")
		line := state + "  " + label
		if outcome.Err != nil {
			line += "  " + outcome.Err.Error()
		}
		lines = append(lines, truncate(line, width))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) closeStartWorkflow() {
	m.startWorkflowOpen = false
	m.startWorkflowPreparing = false
	m.startWorkflowPlanning = false
	m.startWorkflowApplying = false
	m.startWorkflow = startworkflow.Model{}
	m.startWorkflowIssue = jira.Issue{}
	m.startWorkflowResult = startworkflow.Result{}
	m.startWorkflowOutcomes = nil
	m.startWorkflowBranchSucceeded = false
	m.startWorkflowErr = nil
	m.activeStartRepoReqID = 0
	m.activeStartBranchReqID = 0
	m.activeStartIssueReqID = 0
}

func (m Model) claudeStartPlanAvailable() bool {
	return m.claudeConfig.Enabled &&
		m.claudeConfig.BranchPlan &&
		m.claudeStatus.Enabled &&
		m.claudeStatus.Available
}

func (m Model) submitStartPlan(id int, issue jira.Issue, repo gitworkflow.RepoStatus, cfg config.Config) tea.Cmd {
	branch := gitworkflow.RenderBranchName(cfg.Git.BranchTemplate, issue)
	prompt := startworkflow.BuildPlanDraftPrompt(startworkflow.PlanDraftRequest{
		Issue:      issue,
		RepoPath:   repo.Path,
		BranchName: branch,
		Actions:    startworkflow.DefaultActions(branch),
	})
	ctx, cancel := context.WithTimeout(context.Background(), startWorkflowTimeout(m.claudeConfig.Timeout))
	return tea.Sequence(
		m.submitAIRequest(ctx, aiTaskRequest{
			RequestID: id,
			Operation: events.AIOperationImplementationPlan,
			IssueKey:  issue.Key,
			Prompt:    prompt,
			ResultMsg: func(id int, _ string, text string, err error) tea.Msg {
				cancel()
				return startPlanResultMsg{id: id, issue: issue, repo: repo, text: text, err: err}
			},
		}),
	)
}

func cleanStartPlanText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len([]byte(value)) <= maxStartPlanBytes {
		return value
	}
	var b strings.Builder
	for _, r := range value {
		if b.Len()+len(string(r)) > maxStartPlanBytes {
			break
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func configDefaultsForStart(git config.Git) config.Config {
	cfg := config.Defaults()
	if strings.TrimSpace(git.BranchTemplate) != "" {
		cfg.Git = git
	}
	return cfg
}

func startWorkflowTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return defaultRequestTimeout
	}
	return timeout
}

func startAction(result startworkflow.Result, kind startworkflow.ActionKind) (startworkflow.ActionPlan, bool) {
	for _, action := range result.Actions {
		if action.Kind == kind {
			return action, true
		}
	}
	return startworkflow.ActionPlan{}, false
}

func startActionLabel(result startworkflow.Result, kind startworkflow.ActionKind) string {
	action, ok := startAction(result, kind)
	if !ok {
		return string(kind)
	}
	return action.Label
}

func hasRunnableJiraStartActions(result startworkflow.Result) bool {
	for _, action := range result.Actions {
		if action.Kind != startworkflow.ActionBranch && !action.Skip {
			return true
		}
	}
	return false
}

func skippedJiraStartOutcomes(result startworkflow.Result) []startworkflow.Outcome {
	outcomes := make([]startworkflow.Outcome, 0, len(result.Actions))
	for _, action := range result.Actions {
		if action.Kind == startworkflow.ActionBranch {
			continue
		}
		outcomes = append(outcomes, startworkflow.Outcome{Kind: action.Kind, Label: action.Label, State: "skipped"})
	}
	return outcomes
}

func startWorkflowChangedJira(outcomes []startworkflow.Outcome) bool {
	for _, outcome := range outcomes {
		if outcome.State != "completed" {
			continue
		}
		switch outcome.Kind {
		case startworkflow.ActionAssign, startworkflow.ActionTransition, startworkflow.ActionComment:
			return true
		}
	}
	return false
}

func startWorkflowNotice(result startworkflow.Result, outcomes []startworkflow.Outcome) string {
	completed := 0
	failed := 0
	for _, outcome := range outcomes {
		switch outcome.State {
		case "completed":
			completed++
		case "failed":
			failed++
		}
	}
	if failed > 0 {
		return "Start Work finished with failures for " + displayValue(result.Issue.Key, "ticket") + "."
	}
	if completed == 0 {
		return "Start Work finished with no writes for " + displayValue(result.Issue.Key, "ticket") + "."
	}
	return "Started work on " + displayValue(result.Issue.Key, "ticket") + "."
}
