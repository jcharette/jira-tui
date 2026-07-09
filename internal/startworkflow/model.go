package startworkflow

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/gitworkflow"
	"github.com/jcharette/jira-tui/internal/jira"
)

type Step int

const (
	StepTicket Step = iota
	StepRepo
	StepBranch
	StepReview
	StepDone
)

type ActionKind string

const (
	ActionBranch     ActionKind = "branch"
	ActionAssign     ActionKind = "assign"
	ActionTransition ActionKind = "transition"
	ActionSprint     ActionKind = "sprint"
	ActionComment    ActionKind = "comment"
)

type ActionPlan struct {
	Kind     ActionKind
	Label    string
	Detail   string
	Required bool
	Skip     bool
}

type Options struct {
	Config        config.Config
	Issue         *jira.Issue
	Issues        []jira.Issue
	PreferredRepo gitworkflow.RepoStatus
	PlanText      string
	PlanErr       string
}

type Result struct {
	Confirmed  bool
	Canceled   bool
	Issue      jira.Issue
	BoardID    int
	RepoPath   string
	BranchName string
	Actions    []ActionPlan
}

type Model struct {
	cfg config.Config

	issues        []jira.Issue
	selectedIssue int
	issue         jira.Issue
	hasIssue      bool

	step Step

	repoInput   textinput.Model
	branchInput textinput.Model

	actions        []ActionPlan
	selectedAction int

	planText string
	planErr  string

	result Result
}

func NewModel(options Options) Model {
	cfg := options.Config
	if strings.TrimSpace(cfg.Git.BranchTemplate) == "" {
		cfg.Git.BranchTemplate = config.Defaults().Git.BranchTemplate
	}
	repo := newInput(displayRepoPath(options.PreferredRepo), "Repository path")
	branch := newInput("", "Branch name")
	model := Model{
		cfg:         cfg,
		issues:      append([]jira.Issue(nil), options.Issues...),
		step:        StepTicket,
		repoInput:   repo,
		branchInput: branch,
		planText:    strings.TrimSpace(options.PlanText),
		planErr:     strings.TrimSpace(options.PlanErr),
	}
	if options.Issue != nil && strings.TrimSpace(options.Issue.Key) != "" {
		model.issue = *options.Issue
		model.hasIssue = true
		model.step = StepRepo
		model.branchInput.SetValue(gitworkflow.RenderBranchName(cfg.Git.BranchTemplate, model.issue))
		model.branchInput.CursorEnd()
	}
	model.focusStepInput()
	return model
}

func Run(options Options) (Result, error) {
	final, err := tea.NewProgram(NewModel(options)).Run()
	if err != nil {
		return Result{}, err
	}
	model, ok := final.(Model)
	if !ok {
		return Result{}, fmt.Errorf("start workflow returned unexpected model")
	}
	return model.Result(), nil
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m Model) Result() Result {
	if m.result.Confirmed || m.result.Canceled {
		return m.result
	}
	return Result{}
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.result = Result{Canceled: true}
		m.step = StepDone
		return m, tea.Quit
	}
	switch m.step {
	case StepTicket:
		return m.updateTicket(msg)
	case StepRepo:
		return m.updateRepo(msg)
	case StepBranch:
		return m.updateBranch(msg)
	case StepReview:
		return m.updateReview(msg)
	default:
		return m, nil
	}
}

func (m Model) updateTicket(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.selectedIssue = clamp(m.selectedIssue-1, 0, len(m.issues)-1)
		return m, nil
	case "down", "j":
		m.selectedIssue = clamp(m.selectedIssue+1, 0, len(m.issues)-1)
		return m, nil
	case "enter":
		if len(m.issues) == 0 {
			return m, nil
		}
		m.issue = m.issues[clamp(m.selectedIssue, 0, len(m.issues)-1)]
		m.hasIssue = true
		m.branchInput.SetValue(gitworkflow.RenderBranchName(m.cfg.Git.BranchTemplate, m.issue))
		m.branchInput.CursorEnd()
		m.step = StepRepo
		m.focusStepInput()
		return m, nil
	}
	return m, nil
}

func (m Model) updateRepo(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if strings.TrimSpace(m.repoInput.Value()) == "" {
			return m, nil
		}
		m.step = StepBranch
		m.focusStepInput()
		return m, nil
	}
	var cmd tea.Cmd
	m.repoInput, cmd = m.repoInput.Update(msg)
	return m, cmd
}

func (m Model) updateBranch(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if strings.TrimSpace(m.branchInput.Value()) == "" {
			return m, nil
		}
		m.actions = m.reviewActions()
		m.selectedAction = 0
		m.step = StepReview
		m.focusStepInput()
		return m, nil
	}
	var cmd tea.Cmd
	m.branchInput, cmd = m.branchInput.Update(msg)
	return m, cmd
}

func (m Model) updateReview(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.selectedAction = clamp(m.selectedAction-1, 0, len(m.actions)-1)
		return m, nil
	case "down", "j":
		m.selectedAction = clamp(m.selectedAction+1, 0, len(m.actions)-1)
		return m, nil
	case " ", "space":
		if len(m.actions) == 0 {
			return m, nil
		}
		index := clamp(m.selectedAction, 0, len(m.actions)-1)
		if !m.actions[index].Required {
			m.actions[index].Skip = !m.actions[index].Skip
		}
		return m, nil
	case "enter":
		m.result = Result{
			Confirmed:  true,
			Issue:      m.issue,
			BoardID:    m.cfg.DefaultBoardID,
			RepoPath:   strings.TrimSpace(m.repoInput.Value()),
			BranchName: strings.TrimSpace(m.branchInput.Value()),
			Actions:    append([]ActionPlan(nil), m.actions...),
		}
		m.step = StepDone
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) render() string {
	var b strings.Builder
	b.WriteString("Start Work\n\n")
	switch m.step {
	case StepTicket:
		b.WriteString("Choose ticket\n\n")
		if len(m.issues) == 0 {
			b.WriteString("No tickets loaded.\n")
			break
		}
		for index, issue := range m.issues {
			cursor := " "
			if index == m.selectedIssue {
				cursor = ">"
			}
			fmt.Fprintf(&b, "%s %s  %s\n", cursor, issue.Key, issue.Summary)
		}
		b.WriteString("\nenter choose  esc cancel")
	case StepRepo:
		fmt.Fprintf(&b, "Ticket: %s  %s\n\n", m.issue.Key, m.issue.Summary)
		b.WriteString("Repository\n")
		b.WriteString(m.repoInput.View())
		b.WriteString("\n\nenter continue  esc cancel")
	case StepBranch:
		fmt.Fprintf(&b, "Ticket: %s  %s\n", m.issue.Key, m.issue.Summary)
		fmt.Fprintf(&b, "Repo: %s\n\n", strings.TrimSpace(m.repoInput.Value()))
		b.WriteString("Branch\n")
		b.WriteString(m.branchInput.View())
		b.WriteString("\n\nenter review  esc cancel")
	case StepReview:
		fmt.Fprintf(&b, "Ticket: %s  %s\n", m.issue.Key, m.issue.Summary)
		fmt.Fprintf(&b, "Repo: %s\n", strings.TrimSpace(m.repoInput.Value()))
		fmt.Fprintf(&b, "Branch: %s\n\n", strings.TrimSpace(m.branchInput.Value()))
		b.WriteString("Review actions\n\n")
		for index, action := range m.actions {
			cursor := " "
			if index == m.selectedAction {
				cursor = ">"
			}
			state := "run"
			if action.Skip {
				state = "skip"
			}
			if action.Required {
				state = "required"
			}
			fmt.Fprintf(&b, "%s [%s] %s - %s\n", cursor, state, action.Label, action.Detail)
		}
		if m.planText != "" {
			b.WriteString("\nClaude plan:\n")
			b.WriteString(m.planText)
			b.WriteString("\n")
		} else if m.planErr != "" {
			b.WriteString("\nClaude plan unavailable: ")
			b.WriteString(m.planErr)
			b.WriteString("\n")
		}
		b.WriteString("\nspace skip optional  enter start  esc cancel")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) reviewActions() []ActionPlan {
	return DefaultActions(m.branchInput.Value())
}

func DefaultActions(branch string) []ActionPlan {
	return []ActionPlan{
		{Kind: ActionBranch, Label: "Create or switch branch", Detail: strings.TrimSpace(branch), Required: true},
		{Kind: ActionAssign, Label: "Assign to me", Detail: "Use current Jira user"},
		{Kind: ActionTransition, Label: "Move to In Progress", Detail: "Use best matching Jira transition"},
		{Kind: ActionSprint, Label: "Add to active sprint", Detail: "Use configured Jira board active sprint"},
		{Kind: ActionComment, Label: "Add branch comment", Detail: "Post compact branch note"},
	}
}

type PlanDraftRequest struct {
	Issue      jira.Issue
	RepoPath   string
	BranchName string
	Actions    []ActionPlan
}

func BuildPlanDraftPrompt(request PlanDraftRequest) string {
	var b strings.Builder
	b.WriteString("Create a read-only start-work implementation plan for this Jira ticket.\n")
	b.WriteString("Do not edit files, create branches, call Jira, run git commands, or make external changes.\n")
	b.WriteString("Return only the plan. Keep it concise and focused on first implementation steps, risks, and verification.\n\n")
	b.WriteString("Ticket:\n")
	writePromptField(&b, "Key", request.Issue.Key)
	writePromptField(&b, "Summary", request.Issue.Summary)
	writePromptField(&b, "Status", request.Issue.Status)
	writePromptField(&b, "Issue Type", request.Issue.IssueType)
	writePromptField(&b, "Priority", request.Issue.Priority)
	writePromptField(&b, "Assignee", request.Issue.Assignee)
	writePromptField(&b, "Repo", request.RepoPath)
	writePromptField(&b, "Branch", request.BranchName)
	if len(request.Actions) > 0 {
		b.WriteString("\nRequested writes after user confirmation:\n")
		for _, action := range request.Actions {
			state := "optional"
			if action.Required {
				state = "required"
			}
			fmt.Fprintf(&b, "- %s (%s): %s\n", action.Label, state, action.Detail)
		}
	}
	return strings.TrimSpace(b.String())
}

func writePromptField(b *strings.Builder, label string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(b, "- %s: %s\n", label, value)
}

func (m *Model) focusStepInput() {
	m.repoInput.Blur()
	m.branchInput.Blur()
	switch m.step {
	case StepRepo:
		m.repoInput.Focus()
	case StepBranch:
		m.branchInput.Focus()
	}
}

func newInput(value string, placeholder string) textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = placeholder
	input.SetValue(value)
	input.CharLimit = 240
	input.SetWidth(72)
	input.CursorEnd()
	return input
}

func displayRepoPath(status gitworkflow.RepoStatus) string {
	return strings.TrimSpace(status.Path)
}

func clamp(value, minValue, maxValue int) int {
	if maxValue < minValue {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

var _ tea.Model = Model{}
