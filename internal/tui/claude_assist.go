package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/jon/jira-tui/internal/claude"
	"github.com/jon/jira-tui/internal/worker"
)

type ClaudeStatus struct {
	Enabled   bool
	Available bool
	Command   string
	Version   string
	Message   string
	Err       error
}

type ClaudeConfig struct {
	Enabled             bool
	TicketPlan          bool
	TicketAssist        bool
	DraftTicket         bool
	Command             string
	Timeout             time.Duration
	RequireConfirmation bool
	AllowJiraWrites     bool
}

type claudeRunner interface {
	Run(context.Context, claude.Request) (claude.Result, error)
}

func WithClaudeStatus(status ClaudeStatus) Option {
	return func(m *Model) {
		m.claudeStatus = status
		state := "disabled"
		if status.Enabled && status.Available {
			state = "ready"
		} else if status.Enabled {
			state = "unavailable"
		}
		detailParts := []string{state}
		if strings.TrimSpace(status.Command) != "" {
			detailParts = append(detailParts, strings.TrimSpace(status.Command))
		}
		if strings.TrimSpace(status.Version) != "" {
			detailParts = append(detailParts, strings.TrimSpace(status.Version))
		}
		if strings.TrimSpace(status.Message) != "" {
			detailParts = append(detailParts, strings.TrimSpace(status.Message))
		}
		if status.Err != nil {
			detailParts = append(detailParts, truncate(status.Err.Error(), 80))
		}
		eventStatus := "ok"
		if status.Enabled && !status.Available {
			eventStatus = "error"
		}
		m.recordDiagnosticEvent(diagnosticKindClaude, "claude", eventStatus, strings.Join(detailParts, " "))
	}
}

func WithClaudeConfig(config ClaudeConfig) Option {
	return func(m *Model) {
		m.claudeConfig = config
	}
}

func WithClaudeRunner(runner claudeRunner) Option {
	return func(m *Model) {
		m.claudeRunner = runner
	}
}

type claudePlanResultMsg struct {
	id   int
	key  string
	text string
	err  error
}

type claudePlanTickMsg struct {
	id int
}

type claudePlanProgressMsg struct {
	id    int
	key   string
	event claude.Event
}

type claudeAssistResultMsg struct {
	id   int
	key  string
	text string
	err  error
}

type claudeAssistTickMsg struct {
	id int
}

type claudeAssistProgressMsg struct {
	id    int
	key   string
	event claude.Event
}

func (m Model) handleClaudeAssistApplyResult(result worker.Result) Model {
	if !m.claudeAssistApplying {
		return m
	}
	switch result.Kind {
	case worker.KindUpdateSummary:
		if result.ID != m.activeClaudeAssistSummaryReqID {
			return m
		}
		if result.Err != nil {
			m.claudeAssistApplying = false
			m.claudeAssistConfirmApply = false
			m.detailNotice = "Ticket assist apply failed: " + result.Err.Error()
			return m
		}
		if result.UpdateSummary == nil {
			m.claudeAssistApplying = false
			m.detailNotice = "Ticket assist apply failed: " + worker.ErrInvalidRequest.Error()
			return m
		}
		m.updateIssueSummary(result.UpdateSummary.Key, result.UpdateSummary.Summary)
		m.claudeAssistSummaryApplied = true
	case worker.KindUpdateDescription:
		if result.ID != m.activeClaudeAssistDescriptionReqID {
			return m
		}
		if result.Err != nil {
			m.claudeAssistApplying = false
			m.claudeAssistConfirmApply = false
			m.detailNotice = "Ticket assist apply failed: " + result.Err.Error()
			return m
		}
		if result.UpdateDescription == nil {
			m.claudeAssistApplying = false
			m.detailNotice = "Ticket assist apply failed: " + worker.ErrInvalidRequest.Error()
			return m
		}
		m.updateIssueDescription(result.UpdateDescription.Key, result.UpdateDescription.Description)
		m.claudeAssistDescriptionApplied = true
	default:
		return m
	}
	if m.claudeAssistSummaryApplied && m.claudeAssistDescriptionApplied {
		m.claudeAssistApplying = false
		m.claudeAssistConfirmApply = false
		m.claudeAssistOpen = false
		m.activeClaudeAssistSummaryReqID = 0
		m.activeClaudeAssistDescriptionReqID = 0
		m.detailNotice = "Ticket assist draft applied."
	}
	return m
}

func (m Model) handleClaudeAssistCommentResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeClaudeAssistCommentReqID {
		return m, nil
	}
	m.claudeAssistPostingComment = false
	if result.Err != nil {
		m.detailNotice = "Ticket assist comment failed: " + result.Err.Error()
		return m, nil
	}
	if result.AddComment == nil {
		m.detailNotice = "Ticket assist comment failed: " + worker.ErrInvalidRequest.Error()
		return m, nil
	}
	key := result.AddComment.Key
	m.claudeAssistConfirmComment = false
	m.claudeAssistOpen = false
	m.activeClaudeAssistCommentReqID = 0
	m.detailNotice = "Ticket assist draft posted as a comment."
	m.invalidateIssueComments(key)
	m.nextRequestID++
	m.activeCommentsReqID = m.nextRequestID
	m.commentsRequestKey = key
	m.commentsLoading = true
	m.commentsErr = nil
	return m, m.submitIssueComments(m.activeCommentsReqID, key)
}

func (m Model) renderInlineAIDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 72)
	if m.inlineAIInstructionOpen {
		lines := []string{
			m.theme.Muted.Render("Question or instruction"),
			m.configuredInlineAIInstructionEditor(bodyWidth, 4).View(),
			"",
			m.theme.Muted.Render("Claude will receive the current ticket context and Description."),
		}
		return m.renderDetailDialog(width, "AI for Description", selected.Key, strings.Join(lines, "\n"), "ctrl+s send  esc cancel")
	}
	actions := inlineDescriptionAIActions()
	cursor := clamp(m.selectedInlineAIAction, 0, len(actions)-1)
	rows := make([][]string, 0, len(actions))
	for index, action := range actions {
		marker := " "
		labelStyle := m.theme.Text
		descStyle := m.theme.Muted
		if index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
		}
		rows = append(rows, []string{
			labelStyle.Render(marker),
			labelStyle.Render(action.Label),
			descStyle.Render(action.Description),
		})
	}
	body := m.detailTable(0, []string{"", "ACTION", "DETAIL"}, rows, nil)
	return m.renderDetailDialog(width, "AI for Description", selected.Key, body, "j/k select  enter run  esc cancel")
}

func (m Model) renderClaudePlanDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	dialogWidth := claudePlanDialogWidth(width)
	bodyWidth := max(24, dialogWidth-4)
	var lines []string
	footer := "esc close"
	switch {
	case m.claudePlanLoading:
		footer = "esc cancel"
		lines = append(lines, m.detailStatusBlock("Asking Claude for a ticket plan...", bodyWidth, false))
		lines = append(lines, "")
		lines = append(lines, m.renderClaudePlanLoading(bodyWidth, m.claudeNow()))
	case m.claudePlanErr != nil:
		lines = append(lines, m.renderClaudePlanError(bodyWidth, m.claudeNow()))
	case strings.TrimSpace(m.claudePlanText) != "":
		lines = append(lines, m.renderClaudePlanResult(bodyWidth))
		if m.claudePlanResultScrollable(bodyWidth) {
			footer = "j/k scroll  pgup/pgdn page  g/G jump  esc close"
		}
	default:
		lines = append(lines, m.detailEmptyState("No Claude plan yet.", bodyWidth))
	}
	return m.renderDetailDialogWithLimit(width, "Claude Ticket Plan", selected.Key, strings.Join(lines, "\n"), footer, dialogWidth)
}

func (m Model) renderClaudeAssistDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	dialogWidth := claudePlanDialogWidth(width)
	bodyWidth := max(24, dialogWidth-4)
	footer := "esc close"
	var lines []string
	switch {
	case m.claudeAssistLoading:
		footer = "esc cancel"
		lines = append(lines, m.detailStatusBlock("Asking Claude to evaluate this ticket...", bodyWidth, false))
		lines = append(lines, "")
		lines = append(lines, m.renderClaudeAssistLoading(bodyWidth, m.claudeNow()))
	case m.claudeAssistApplying:
		footer = "esc close"
		lines = append(lines, m.detailStatusBlock("Applying Ticket Assist draft to Jira...", bodyWidth, false))
		lines = append(lines, "")
		lines = append(lines, m.theme.Muted.Render("Summary: ")+m.theme.Text.Render(applyStatusLabel(m.claudeAssistSummaryApplied)))
		lines = append(lines, m.theme.Muted.Render("Description: ")+m.theme.Text.Render(applyStatusLabel(m.claudeAssistDescriptionApplied)))
	case m.claudeAssistPostingComment:
		footer = "esc close"
		lines = append(lines, m.detailStatusBlock("Posting Ticket Assist draft as a Jira comment...", bodyWidth, false))
	case m.claudeAssistConfirmComment:
		footer = "ctrl+s post  esc cancel"
		lines = append(lines, m.renderClaudeAssistCommentConfirmation(bodyWidth))
	case m.claudeAssistConfirmApply:
		footer = "ctrl+s apply  esc cancel"
		lines = append(lines, m.renderClaudeAssistApplyConfirmation(bodyWidth))
	case m.claudeAssistRefining:
		footer = "ctrl+s send  esc cancel"
		lines = append(lines, m.renderClaudeAssistRefinementEditor(bodyWidth))
	case m.claudeAssistErr != nil:
		lines = append(lines, m.renderClaudeAssistError(bodyWidth, m.claudeNow()))
	default:
		if m.claudeConfig.AllowJiraWrites {
			footer = "ctrl+s apply  c comment  r refine  ctrl+y copy  pgup/pgdn page  esc close"
		} else {
			footer = "c comment  r refine  ctrl+y copy  pgup/pgdn page  esc close"
		}
		lines = append(lines, m.renderClaudeAssistEditor(bodyWidth))
	}
	return m.renderDetailDialogWithLimit(width, "Claude Ticket Assist", selected.Key, strings.Join(lines, "\n"), footer, dialogWidth)
}

func applyStatusLabel(done bool) string {
	if done {
		return "done"
	}
	return "saving"
}

func claudePlanDialogWidth(width int) int {
	if width <= 0 {
		width = 72
	}
	return clamp((width*88)/100, 72, max(72, width))
}

func (m Model) renderClaudePlanResult(width int) string {
	lines := m.claudePlanResultLines(width)
	if len(lines) == 0 {
		return ""
	}
	rows := m.claudePlanResultRows()
	if len(lines) <= rows {
		return strings.Join(lines, "\n")
	}
	offset := clamp(m.claudePlanOffset, 0, max(0, len(lines)-rows))
	end := min(len(lines), offset+rows)
	visible := append([]string(nil), lines[offset:end]...)
	visible = append(visible, m.theme.Muted.Render(fmt.Sprintf("Claude Lines %d-%d of %d", offset+1, end, len(lines))))
	return strings.Join(visible, "\n")
}

func (m Model) claudePlanResultRows() int {
	return max(1, m.fullDetailRows()-9)
}

func (m Model) claudePlanResultScrollable(width int) bool {
	return len(m.claudePlanResultLines(width)) > m.claudePlanResultRows()
}

func (m *Model) scrollClaudePlanResult(delta int) {
	lines := m.claudePlanResultLines(m.currentClaudePlanBodyWidth())
	rows := m.claudePlanResultRows()
	m.claudePlanOffset = clamp(m.claudePlanOffset+delta, 0, max(0, len(lines)-rows))
}

func (m Model) claudePlanResultLines(width int) []string {
	rendered := m.renderRichDescriptionBody(wrapRichText(markdownTablesToRichMarkers(m.claudePlanText), width), width)
	return strings.Split(strings.TrimRight(rendered, "\n"), "\n")
}

func (m Model) renderClaudeAssistLoading(width int, now time.Time) string {
	elapsed := formatClaudeDuration(now.Sub(m.claudeAssistStartedAt))
	if m.claudeConfig.Timeout > 0 {
		elapsed += " of " + m.claudeConfig.Timeout.String()
	}
	lines := []string{
		m.theme.Muted.Render("Activity: ") + m.theme.Text.Render(claudeActivityFrame(now.Sub(m.claudeAssistStartedAt))+" Claude subprocess running"),
		m.theme.Muted.Render("Elapsed: ") + m.theme.Text.Render(elapsed),
	}
	lines = append(lines, m.renderClaudeProgressStatus(m.claudeAssistProgress)...)
	return strings.Join(lines, "\n")
}

func (m Model) renderClaudeAssistProgressLines(width int) []string {
	return m.renderClaudeEventProgressLines(m.claudeAssistProgress, width)
}

func (m Model) renderClaudeAssistEditor(width int) string {
	var lines []string
	if review := strings.TrimSpace(m.claudeAssistReviewText()); review != "" && m.claudeAssistReviewRows() > 0 {
		lines = append(lines, m.theme.FieldLabel.Render("Claude Review"))
		lines = append(lines, m.renderClaudeAssistReview(review, width)...)
		if m.height > 32 {
			lines = append(lines, "")
		}
	}
	lines = append(lines, m.theme.FieldLabel.Render("Local Draft")+" "+m.theme.Muted.Render("Not Applied"))
	lines = append(lines, m.renderClaudeAssistDraftEditor(width))
	if m.height == 0 || m.height > 32 {
		lines = append(lines, "")
		lines = append(lines, m.renderClaudeAssistActionHint(width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderClaudeAssistActionHint(width int) string {
	var action string
	if m.claudeConfig.AllowJiraWrites {
		action = "ctrl+s apply  |  c comment  |  r refine  |  ctrl+y copy"
	} else {
		action = "Jira writes disabled  |  c comment  |  r refine  |  ctrl+y copy"
	}
	return m.theme.FieldLabel.Render("Available Actions") + "\n" + m.theme.Muted.Render(truncate(action, width))
}

func (m Model) claudeAssistEditorRows() int {
	reviewRows := m.claudeAssistReviewRows()
	available := max(1, m.fullDetailRows()-reviewRows-13)
	if m.height > 0 && m.height <= 32 {
		return max(2, min(4, available/2))
	}
	return max(6, min(18, available/2))
}

func (m Model) claudeAssistReviewRows() int {
	if m.height > 0 && m.height <= 32 {
		return 0
	}
	return max(1, min(3, m.fullDetailRows()/8))
}

func (m Model) renderClaudeAssistReview(review string, width int) []string {
	rendered := m.renderRichDescriptionBody(wrapRichText(markdownTablesToRichMarkers(review), width), width)
	reviewLines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	if len(reviewLines) == 1 && strings.TrimSpace(reviewLines[0]) == "" {
		return nil
	}
	if len(reviewLines) > 0 && strings.EqualFold(strings.Trim(strings.TrimSpace(reviewLines[0]), "#: "), "Review") {
		reviewLines = reviewLines[1:]
	}
	rows := min(len(reviewLines), m.claudeAssistReviewRows())
	lines := append([]string(nil), reviewLines[:rows]...)
	if len(reviewLines) > rows {
		end := max(1, rows)
		lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("Review Lines 1-%d of %d", end, len(reviewLines))))
	}
	return lines
}

func (m Model) renderClaudeAssistDraftEditor(width int) string {
	rows := m.claudeAssistEditorRows()
	editorWidth := max(20, width-3)
	editor := m.configuredClaudeAssistEditor(editorWidth, rows)
	lineCount := editor.LineCount()
	body := editor.View()
	if lineCount > rows {
		start := editor.ScrollYOffset() + 1
		end := min(lineCount, editor.ScrollYOffset()+editor.Height())
		indicator := fmt.Sprintf("Draft Lines %d-%d of %d  PgUp/PgDn page", start, end, lineCount)
		body += "\n" + m.theme.Muted.Render(truncate(indicator, editorWidth))
	}
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(m.theme.Muted.GetForeground()).
		Padding(0, 1).
		Width(max(24, width)).
		Render(body)
}

func (m Model) renderClaudeAssistApplyConfirmation(width int) string {
	var lines []string
	if m.claudeAssistTarget == claudeAssistTargetDescription {
		lines = append(lines, m.theme.FieldLabel.Render("Apply Description Draft"))
		lines = append(lines, m.theme.Muted.Render("Issue: ")+m.theme.Text.Render(displayValue(m.claudeAssistKey, "selected ticket")))
		lines = append(lines, "")
		lines = append(lines, m.theme.Muted.Render("Description"))
		descriptionLines := strings.Split(strings.TrimSpace(m.claudeAssistApplyDescription), "\n")
		for i, line := range descriptionLines {
			if i >= 4 {
				lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("Description Lines 1-4 of %d", len(descriptionLines))))
				break
			}
			lines = append(lines, m.theme.Text.Render(truncate(line, width)))
		}
		return strings.Join(lines, "\n")
	}
	lines = append(lines, m.theme.FieldLabel.Render("Apply Ticket Assist Draft"))
	lines = append(lines, m.theme.Muted.Render("Issue: ")+m.theme.Text.Render(displayValue(m.claudeAssistKey, "selected ticket")))
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("Summary"))
	lines = append(lines, m.theme.Text.Render(truncate(displayValue(m.claudeAssistApplySummary, "unchanged"), width)))
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("Description"))
	descriptionLines := strings.Split(strings.TrimSpace(m.claudeAssistApplyDescription), "\n")
	for i, line := range descriptionLines {
		if i >= 4 {
			lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("Description Lines 1-4 of %d", len(descriptionLines))))
			break
		}
		lines = append(lines, m.theme.Text.Render(truncate(line, width)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderClaudeAssistCommentConfirmation(width int) string {
	var lines []string
	lines = append(lines, m.theme.FieldLabel.Render("Post Draft As Comment"))
	lines = append(lines, m.theme.Muted.Render("Issue: ")+m.theme.Text.Render(displayValue(m.claudeAssistKey, "selected ticket")))
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("This will add the local Ticket Assist draft as a Jira comment without editing Summary or Description."))
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("Comment Preview"))
	draftLines := strings.Split(strings.TrimSpace(m.claudeAssistDraftValue()), "\n")
	for i, line := range draftLines {
		if i >= 4 {
			lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("Comment Lines 1-4 of %d", len(draftLines))))
			break
		}
		lines = append(lines, m.theme.Text.Render(truncate(line, width)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderClaudeAssistRefinementEditor(width int) string {
	var lines []string
	lines = append(lines, m.theme.FieldLabel.Render("Refine Draft"))
	lines = append(lines, m.theme.Muted.Render("Instruction"))
	editor := m.configuredClaudeAssistRefineEditor(max(24, width-3), 4)
	body := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(m.theme.Muted.GetForeground()).
		Padding(0, 1).
		Width(max(24, width)).
		Render(editor.View())
	lines = append(lines, body)
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("Claude will receive the current edited draft and this instruction."))
	return strings.Join(lines, "\n")
}

func (m Model) renderClaudeAssistError(width int, now time.Time) string {
	if errors.Is(m.claudeAssistErr, context.DeadlineExceeded) {
		lines := []string{
			m.renderDetailNotice("Claude ticket assist timed out after "+displayValue(m.claudeConfig.Timeout.String(), "the configured timeout"), width),
			"",
			m.theme.Muted.Render("Started: ") + m.theme.Text.Render(formatClockTime(m.claudeAssistStartedAt)),
		}
		if m.claudeConfig.Timeout > 0 {
			lines = append(lines, m.theme.Muted.Render("Deadline: ")+m.theme.Text.Render(formatClockTime(m.claudeAssistStartedAt.Add(m.claudeConfig.Timeout))))
		}
		lines = append(lines, m.theme.Muted.Render("Elapsed: ")+m.theme.Text.Render(formatClaudeDuration(now.Sub(m.claudeAssistStartedAt))))
		lines = append(lines, m.theme.Muted.Render("Command: ")+m.theme.Text.Render(m.claudeCommandLabel()))
		return strings.Join(lines, "\n")
	}
	return m.renderDetailNotice("Claude ticket assist failed: "+m.claudeAssistErr.Error(), width)
}

func (m Model) currentClaudePlanBodyWidth() int {
	layout := m.browserLayout(m.width)
	dialogWidth := max(32, layout.contentWidth-12)
	return min(max(24, dialogWidth-12), 64)
}

func (m Model) claudeTicketPlanAvailable() bool {
	return m.claudeConfig.Enabled &&
		m.claudeConfig.TicketPlan &&
		m.claudeStatus.Enabled &&
		m.claudeStatus.Available
}

func (m Model) claudeTicketAssistAvailable() bool {
	return m.claudeConfig.Enabled &&
		m.claudeConfig.TicketAssist &&
		m.claudeStatus.Enabled &&
		m.claudeStatus.Available
}

func (m Model) claudeCreateTicketDraftAvailable() bool {
	return m.claudeCreateTicketDraftEnabled() && m.claudeStatus.Available
}

func (m Model) claudeCreateTicketDraftEnabled() bool {
	return m.claudeConfig.Enabled &&
		m.claudeConfig.DraftTicket &&
		m.claudeStatus.Enabled
}

func (m Model) claudeAvailable() bool {
	return m.claudeTicketPlanAvailable() || m.claudeTicketAssistAvailable()
}

func (m Model) inlineDescriptionAIAvailable() bool {
	if !m.claudeTicketAssistAvailable() {
		return false
	}
	section, ok := m.focusedDetailSection()
	return ok && section.ID == "description"
}

func (m Model) openInlineDescriptionAI() (Model, tea.Cmd) {
	if !m.inlineDescriptionAIAvailable() {
		m.detailNotice = "Claude ticket assistance is not enabled or available."
		return m, nil
	}
	m.inlineAIOpen = true
	m.selectedInlineAIAction = clamp(m.selectedInlineAIAction, 0, len(inlineDescriptionAIActions())-1)
	m.detailNotice = ""
	return m, nil
}

type claudeAction struct {
	ID          string
	Label       string
	Description string
	Enabled     bool
}

type inlineAIAction struct {
	ID          string
	Label       string
	Description string
}

type claudeAssistTarget int

func inlineDescriptionAIActions() []inlineAIAction {
	return []inlineAIAction{
		{ID: "improve_clarity", Label: "Improve clarity", Description: "Rewrite the Description for clearer scope and verification."},
		{ID: "extract_acceptance", Label: "Extract acceptance criteria", Description: "Draft explicit acceptance criteria and open questions."},
		{ID: "ask_question", Label: "Ask Claude a question", Description: "Ask about this ticket and draft a local answer."},
		{ID: "draft_comment", Label: "Draft clarifying comment", Description: "Draft a Jira comment without editing fields."},
	}
}

func (m Model) claudeActions() []claudeAction {
	actions := []claudeAction{
		{ID: "ticket_plan", Label: "Ticket Plan", Description: "Create a read-only implementation and verification plan.", Enabled: m.claudeTicketPlanAvailable()},
		{ID: "ticket_assist", Label: "Ticket Assist", Description: "Evaluate and rewrite this ticket with editable acceptance criteria.", Enabled: m.claudeTicketAssistAvailable()},
	}
	filtered := make([]claudeAction, 0, len(actions))
	for _, action := range actions {
		if action.Enabled {
			filtered = append(filtered, action)
		}
	}
	return filtered
}

func (m Model) renderClaudeSection(ctx detailRenderContext, width int) string {
	help := "j/k select  enter run"
	if (m.claudePlanLoading && m.claudePlanKey == ctx.display.Key) || (m.claudeAssistLoading && m.claudeAssistKey == ctx.display.Key) {
		help = "running"
	}
	lines := []string{m.detailSectionHeader("claude", "Claude", help, width), ""}
	actions := m.claudeActions()
	if len(actions) == 0 {
		lines = append(lines, m.detailEmptyState("Claude ticket assistance is not enabled or available.", width))
		return strings.Join(lines, "\n")
	}
	if m.claudePlanLoading && m.claudePlanKey == ctx.display.Key {
		lines = append(lines, m.detailStatusBlock("Asking Claude for a read-only ticket plan...", width, false))
		return strings.Join(lines, "\n")
	}
	if m.claudeAssistLoading && m.claudeAssistKey == ctx.display.Key {
		lines = append(lines, m.detailStatusBlock("Asking Claude to evaluate this ticket...", width, false))
		return strings.Join(lines, "\n")
	}
	cursor := clamp(m.selectedClaudeAction, 0, len(actions)-1)
	rows := make([][]string, 0, len(actions))
	for index, action := range actions {
		marker := " "
		labelStyle := m.theme.Text
		descStyle := m.theme.Muted
		if index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
		}
		rows = append(rows, []string{
			labelStyle.Render(marker),
			labelStyle.Render(action.Label),
			descStyle.Render(action.Description),
		})
	}
	lines = append(lines, m.detailTable(0, []string{"", "ACTION", "DETAIL"}, rows, nil))
	if strings.TrimSpace(m.claudePlanText) != "" && m.claudePlanKey == ctx.display.Key {
		lines = append(lines, "", m.theme.Muted.Render("Latest ticket plan is ready. Select Ticket Plan to refresh it."))
	}
	if strings.TrimSpace(m.claudeAssistDraftValue()) != "" && m.claudeAssistKey == ctx.display.Key {
		lines = append(lines, "", m.theme.Muted.Render("Latest ticket assist draft is ready. Select Ticket Assist to refresh it."))
	}
	return strings.Join(lines, "\n")
}

func (m Model) startClaudeTicketPlan() (Model, tea.Cmd) {
	ctx, ok := m.detailRenderContext()
	if !ok {
		return m, nil
	}
	if !m.claudeTicketPlanAvailable() {
		m.detailNotice = "Claude ticket planning is not enabled or available."
		return m, nil
	}
	key := ctx.display.Key
	if key == "" {
		key = ctx.selected.Key
	}
	m.nextRequestID++
	reqID := m.nextRequestID
	m.activeClaudePlanReqID = reqID
	m.claudePlanKey = key
	m.claudePlanText = ""
	m.claudePlanErr = nil
	m.claudePlanOffset = 0
	m.claudePlanLoading = true
	m.claudePlanOpen = true
	m.claudePlanStartedAt = m.claudeNow()
	m.claudePlanProgress = nil
	m.claudePlanEvents = make(chan claude.Event, 16)
	runCtx, cancel := context.WithCancel(context.Background())
	m.claudePlanCancel = cancel
	m.detailNotice = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_plan", "submit", workerDiagnosticDetail(reqID, key, nil))
	return m, tea.Batch(
		m.submitClaudeTicketPlan(runCtx, reqID, key, m.buildClaudeTicketPlanPrompt(ctx), m.claudePlanEvents),
		m.waitForClaudePlanProgress(reqID, key),
		m.scheduleClaudePlanTick(reqID),
	)
}

func (m Model) startClaudeTicketAssist() (Model, tea.Cmd) {
	ctx, ok := m.detailRenderContext()
	if !ok {
		return m, nil
	}
	if !m.claudeTicketAssistAvailable() {
		m.detailNotice = "Claude ticket assistance is not enabled or available."
		return m, nil
	}
	key := ctx.display.Key
	if key == "" {
		key = ctx.selected.Key
	}
	m.nextRequestID++
	reqID := m.nextRequestID
	m.activeClaudeAssistReqID = reqID
	m.claudeAssistKey = key
	m.claudeAssistText = ""
	m.claudeAssistErr = nil
	m.claudeAssistLoading = true
	m.claudeAssistOpen = true
	m.claudeAssistStartedAt = m.claudeNow()
	m.claudeAssistProgress = nil
	m.claudeAssistDraft = ""
	m.claudeAssistEditor = newClaudeAssistEditor("")
	m.claudeAssistEditorReady = true
	m.claudeAssistTarget = claudeAssistTargetTicket
	m.claudeAssistEvents = make(chan claude.Event, 16)
	runCtx, cancel := context.WithCancel(context.Background())
	m.claudeAssistCancel = cancel
	m.detailNotice = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_assist", "submit", workerDiagnosticDetail(reqID, key, nil))
	return m, tea.Batch(
		m.submitClaudeTicketAssist(runCtx, reqID, key, m.buildClaudeTicketAssistPrompt(ctx), m.claudeAssistEvents),
		m.waitForClaudeAssistProgress(reqID, key),
		m.scheduleClaudeAssistTick(reqID),
	)
}

func (m Model) submitClaudeTicketPlan(ctx context.Context, reqID int, key string, prompt string, events chan<- claude.Event) tea.Cmd {
	return m.submitClaudeRequest(ctx, reqID, key, prompt, events, func(id int, key string, text string, err error) tea.Msg {
		return claudePlanResultMsg{id: id, key: key, text: text, err: err}
	})
}

func (m Model) submitClaudeTicketAssist(ctx context.Context, reqID int, key string, prompt string, events chan<- claude.Event) tea.Cmd {
	return m.submitClaudeRequest(ctx, reqID, key, prompt, events, func(id int, key string, text string, err error) tea.Msg {
		return claudeAssistResultMsg{id: id, key: key, text: text, err: err}
	})
}

func (m Model) submitClaudeRequest(ctx context.Context, reqID int, key string, prompt string, events chan<- claude.Event, resultMsg func(int, string, string, error) tea.Msg) tea.Cmd {
	runner := m.claudeRunner
	if runner == nil {
		runner = claude.LocalRunner{}
	}
	config := claude.Config{
		Enabled: m.claudeConfig.Enabled,
		Command: m.claudeConfig.Command,
		Timeout: m.claudeConfig.Timeout,
	}
	return func() tea.Msg {
		defer closeClaudeEvents(events)
		result, err := runner.Run(ctx, claude.Request{
			Config: config,
			Prompt: prompt,
			Progress: func(event claude.Event) {
				if strings.TrimSpace(event.Text) == "" {
					return
				}
				select {
				case events <- event:
				case <-ctx.Done():
				}
			},
		})
		if err != nil {
			return resultMsg(reqID, key, "", err)
		}
		return resultMsg(reqID, key, result.Text, nil)
	}
}

func closeClaudeEvents(events chan<- claude.Event) {
	if events != nil {
		close(events)
	}
}

func (m Model) waitForClaudePlanProgress(reqID int, key string) tea.Cmd {
	events := m.claudePlanEvents
	return waitForClaudeProgress(events, reqID, key, func(id int, key string, event claude.Event) tea.Msg {
		return claudePlanProgressMsg{id: id, key: key, event: event}
	})
}

func (m Model) waitForClaudeAssistProgress(reqID int, key string) tea.Cmd {
	events := m.claudeAssistEvents
	return waitForClaudeProgress(events, reqID, key, func(id int, key string, event claude.Event) tea.Msg {
		return claudeAssistProgressMsg{id: id, key: key, event: event}
	})
}

func waitForClaudeProgress(events <-chan claude.Event, reqID int, key string, progressMsg func(int, string, claude.Event) tea.Msg) tea.Cmd {
	if events == nil {
		return nil
	}
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return noDetailRequestMsg{}
		}
		return progressMsg(reqID, key, event)
	}
}

func (m Model) scheduleClaudePlanTick(reqID int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return claudePlanTickMsg{id: reqID}
	})
}

func (m Model) scheduleClaudeAssistTick(reqID int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return claudeAssistTickMsg{id: reqID}
	})
}

func (m Model) cancelClaudeTicketPlan() Model {
	if m.claudePlanCancel != nil {
		m.claudePlanCancel()
	}
	reqID := m.activeClaudePlanReqID
	key := m.claudePlanKey
	m.claudePlanCancel = nil
	m.claudePlanEvents = nil
	m.activeClaudePlanReqID = 0
	m.claudePlanLoading = false
	m.claudePlanErr = errors.New("Claude ticket plan cancelled")
	m.claudePlanText = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_plan", "cancel", workerDiagnosticDetail(reqID, key, m.claudePlanErr))
	return m
}

func (m Model) cancelClaudeTicketAssist() Model {
	if m.claudeAssistCancel != nil {
		m.claudeAssistCancel()
	}
	reqID := m.activeClaudeAssistReqID
	key := m.claudeAssistKey
	m.claudeAssistCancel = nil
	m.claudeAssistEvents = nil
	m.activeClaudeAssistReqID = 0
	m.claudeAssistLoading = false
	m.claudeAssistErr = errors.New("Claude ticket assist cancelled")
	m.claudeAssistText = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_assist", "cancel", workerDiagnosticDetail(reqID, key, m.claudeAssistErr))
	return m
}

func (m Model) handleClaudePlanProgress(msg claudePlanProgressMsg) Model {
	if msg.id != m.activeClaudePlanReqID || msg.key != m.claudePlanKey {
		return m
	}
	if strings.TrimSpace(msg.event.Text) == "" {
		return m
	}
	m.claudePlanProgress = append(m.claudePlanProgress, msg.event)
	if len(m.claudePlanProgress) > 6 {
		m.claudePlanProgress = append([]claude.Event(nil), m.claudePlanProgress[len(m.claudePlanProgress)-6:]...)
	}
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_plan", "progress", truncate(msg.event.Kind+" "+msg.event.Text, 100))
	return m
}

func (m Model) handleClaudeAssistProgress(msg claudeAssistProgressMsg) Model {
	if msg.id != m.activeClaudeAssistReqID || msg.key != m.claudeAssistKey {
		return m
	}
	if strings.TrimSpace(msg.event.Text) == "" {
		return m
	}
	m.claudeAssistProgress = append(m.claudeAssistProgress, msg.event)
	if len(m.claudeAssistProgress) > 6 {
		m.claudeAssistProgress = append([]claude.Event(nil), m.claudeAssistProgress[len(m.claudeAssistProgress)-6:]...)
	}
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_assist", "progress", truncate(msg.event.Kind+" "+msg.event.Text, 100))
	return m
}

func (m Model) handleClaudePlanResult(msg claudePlanResultMsg) Model {
	status := "ok"
	if msg.err != nil {
		status = "error"
		if errors.Is(msg.err, context.Canceled) {
			status = "cancel"
		} else if errors.Is(msg.err, context.DeadlineExceeded) {
			status = "timeout"
		}
	}
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_plan", status, workerDiagnosticDetail(msg.id, msg.key, msg.err))
	if msg.id != m.activeClaudePlanReqID || msg.key != m.claudePlanKey {
		return m
	}
	m.claudePlanLoading = false
	m.claudePlanCancel = nil
	m.claudePlanEvents = nil
	m.claudePlanOpen = true
	m.claudePlanErr = msg.err
	if msg.err == nil {
		m.claudePlanText = strings.TrimSpace(msg.text)
		m.claudePlanOffset = 0
	}
	return m
}

func (m Model) handleClaudeAssistResult(msg claudeAssistResultMsg) Model {
	status := "ok"
	if msg.err != nil {
		status = "error"
		if errors.Is(msg.err, context.Canceled) {
			status = "cancel"
		} else if errors.Is(msg.err, context.DeadlineExceeded) {
			status = "timeout"
		}
	}
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_assist", status, workerDiagnosticDetail(msg.id, msg.key, msg.err))
	if msg.id != m.activeClaudeAssistReqID || msg.key != m.claudeAssistKey {
		return m
	}
	m.claudeAssistLoading = false
	m.claudeAssistCancel = nil
	m.claudeAssistEvents = nil
	m.claudeAssistOpen = true
	m.claudeAssistErr = msg.err
	if msg.err == nil {
		m.claudeAssistText = strings.TrimSpace(msg.text)
		m.claudeAssistDraft = claudeAssistDraftFromText(m.claudeAssistText)
		m.claudeAssistEditor = newClaudeAssistEditor(m.claudeAssistDraft)
		m.claudeAssistEditorReady = true
	}
	return m
}

func (m Model) renderClaudePlanLoading(width int, now time.Time) string {
	elapsed := formatClaudeDuration(now.Sub(m.claudePlanStartedAt))
	if m.claudeConfig.Timeout > 0 {
		elapsed += " of " + m.claudeConfig.Timeout.String()
	}
	lines := []string{
		m.theme.Muted.Render("Activity: ") + m.theme.Text.Render(claudeActivityFrame(now.Sub(m.claudePlanStartedAt))+" Claude subprocess running"),
		m.theme.Muted.Render("Elapsed: ") + m.theme.Text.Render(elapsed),
	}
	lines = append(lines, m.renderClaudeProgressStatus(m.claudePlanProgress)...)
	return strings.Join(lines, "\n")
}

func claudeActivityFrame(elapsed time.Duration) string {
	frames := []string{"|", "/", "-", "\\"}
	if elapsed < 0 {
		elapsed = 0
	}
	return frames[int(elapsed/time.Second)%len(frames)]
}

func (m Model) renderClaudeProgressLines(width int) []string {
	return m.renderClaudeEventProgressLines(m.claudePlanProgress, width)
}

func (m Model) renderClaudeProgressStatus(events []claude.Event) []string {
	status := "waiting for first response"
	if len(events) > 0 {
		status = "receiving response"
		if claudeAssistantPreview(events) == "" {
			status = "receiving CLI messages"
		}
	}
	return []string{m.theme.Muted.Render("Output: ") + m.theme.Text.Render(status)}
}

func (m Model) renderClaudeEventProgressLines(events []claude.Event, width int) []string {
	preview := claudeAssistantPreview(events)
	if preview == "" {
		if len(events) > 0 {
			return []string{m.theme.Muted.Render("Output: ") + m.theme.Text.Render("waiting for assistant text")}
		}
		return []string{m.theme.Muted.Render("Output: ") + m.theme.Text.Render("waiting for first response")}
	}
	prefix := "Assistant: "
	return []string{
		m.theme.Muted.Render("Output: ") + m.theme.Text.Render("assistant text"),
		m.theme.Muted.Render(prefix) + m.theme.Text.Render(truncate(preview, max(16, width-lipgloss.Width(prefix)))),
	}
}

func claudeAssistantPreview(events []claude.Event) string {
	var preview string
	for _, event := range events {
		if event.Kind != "output" && event.Kind != "result" && event.Kind != "stderr" {
			continue
		}
		text := strings.Join(strings.Fields(strings.TrimSpace(event.Text)), " ")
		if text == "" || looksLikeJSONEvent(text) {
			continue
		}
		if preview == "" {
			preview = text
			continue
		}
		switch {
		case text == preview:
			continue
		case strings.HasPrefix(text, preview):
			preview = text
		case strings.HasPrefix(preview, text):
			continue
		case strings.Contains(preview, text):
			continue
		default:
			joiner := " "
			if strings.HasSuffix(preview, " ") || strings.HasPrefix(text, " ") {
				joiner = ""
			}
			preview += joiner + text
		}
	}
	return preview
}

func looksLikeJSONEvent(text string) bool {
	return strings.HasPrefix(text, "{") && strings.Contains(text, `"type"`)
}

func (m Model) renderClaudePlanError(width int, now time.Time) string {
	if errors.Is(m.claudePlanErr, context.DeadlineExceeded) {
		lines := []string{
			m.renderDetailNotice("Claude plan timed out after "+displayValue(m.claudeConfig.Timeout.String(), "the configured timeout"), width),
			"",
			m.theme.Muted.Render("Started: ") + m.theme.Text.Render(formatClockTime(m.claudePlanStartedAt)),
		}
		if m.claudeConfig.Timeout > 0 {
			lines = append(lines, m.theme.Muted.Render("Deadline: ")+m.theme.Text.Render(formatClockTime(m.claudePlanStartedAt.Add(m.claudeConfig.Timeout))))
		}
		lines = append(lines, m.theme.Muted.Render("Elapsed: ")+m.theme.Text.Render(formatClaudeDuration(now.Sub(m.claudePlanStartedAt))))
		lines = append(lines, m.theme.Muted.Render("Command: ")+m.theme.Text.Render(m.claudeCommandLabel()))
		return strings.Join(lines, "\n")
	}
	return m.renderDetailNotice("Claude plan failed: "+m.claudePlanErr.Error(), width)
}

func (m Model) claudeCommandLabel() string {
	command := strings.TrimSpace(m.claudeConfig.Command)
	if command == "" {
		command = strings.TrimSpace(m.claudeStatus.Command)
	}
	if command == "" {
		command = "claude"
	}
	return command + " -p <prompt>"
}

func (m Model) claudeNow() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

func (m Model) buildClaudeTicketPlanPrompt(ctx detailRenderContext) string {
	issue := ctx.display
	if issue.Key == "" {
		issue.Key = ctx.selected.Key
	}
	var b strings.Builder
	b.WriteString("Create a read-only implementation and verification plan for this Jira ticket.\n")
	b.WriteString("Do not edit files, create branches, run git commands, call Jira, or make external changes.\n")
	b.WriteString("Focus on likely code areas, risks, test strategy, and questions to clarify before implementation.\n\n")
	b.WriteString("Ticket:\n")
	writePromptField(&b, "Key", issue.Key)
	writePromptField(&b, "Summary", issue.Summary)
	writePromptField(&b, "Status", issue.Status)
	writePromptField(&b, "Issue Type", issue.IssueType)
	writePromptField(&b, "Priority", issue.Priority)
	writePromptField(&b, "Assignee", issue.Assignee)
	writePromptField(&b, "Reporter", ctx.detail.Reporter)
	if len(ctx.detail.Labels) > 0 {
		writePromptField(&b, "Labels", strings.Join(ctx.detail.Labels, ", "))
	}
	if len(ctx.detail.Components) > 0 {
		writePromptField(&b, "Components", strings.Join(ctx.detail.Components, ", "))
	}
	description := strings.TrimSpace(ctx.description)
	if description == "" {
		description = strings.TrimSpace(ctx.detail.Description)
	}
	if description != "" {
		b.WriteString("\nDescription:\n")
		b.WriteString(description)
		b.WriteString("\n")
	}
	comments := m.comments[issue.Key]
	if len(comments) > 0 {
		b.WriteString("\nLoaded comments:\n")
		for index, comment := range comments {
			author := displayValue(comment.Author, "Unknown")
			body := strings.TrimSpace(comment.Body)
			if body == "" {
				continue
			}
			fmt.Fprintf(&b, "%d. %s: %s\n", index+1, author, body)
		}
	}
	return strings.TrimSpace(b.String())
}

func (m Model) buildClaudeTicketAssistPrompt(ctx detailRenderContext) string {
	var b strings.Builder
	b.WriteString("Evaluate and sanitize this existing Jira ticket.\n")
	b.WriteString("Do not update Jira, create tickets, edit files, create branches, run git commands, call GitHub, or make external changes.\n")
	b.WriteString("Return practical ticket-writing help only. Do not invent product decisions; list unknowns as Open Questions.\n")
	b.WriteString("Acceptance Criteria must be a first-class section in the draft, not buried inside prose.\n")
	b.WriteString("Use this exact high-level structure:\n")
	b.WriteString("Review\n")
	b.WriteString("- Clarity issues\n")
	b.WriteString("- Missing acceptance criteria\n")
	b.WriteString("- Conflicting or stale context\n")
	b.WriteString("- Implementation or test gaps\n")
	b.WriteString("- Open questions\n\n")
	b.WriteString("Draft\n")
	b.WriteString("Summary: <one concise summary>\n\n")
	b.WriteString("Problem / Goal\n<clear user/business goal>\n\n")
	b.WriteString("Acceptance Criteria\n- [ ] <testable criterion>\n\n")
	b.WriteString("Test / Verification\n- <verification step>\n\n")
	b.WriteString("Implementation Notes\n- <notes or constraints>\n\n")
	b.WriteString("Open Questions\n- <question or None>\n\n")
	b.WriteString("Ticket:\n")
	m.writeClaudeTicketContext(&b, ctx)
	return strings.TrimSpace(b.String())
}

func (m Model) buildClaudeTicketAssistRefinementPrompt(ctx detailRenderContext, currentDraft string, instruction string) string {
	var b strings.Builder
	b.WriteString("Refine this Jira ticket draft using the user's instruction.\n")
	b.WriteString("Do not update Jira, create tickets, edit files, create branches, run git commands, call GitHub, or make external changes.\n")
	b.WriteString("Do not reinvent the draft from scratch. Preserve useful user edits from the current draft unless the instruction asks otherwise.\n")
	b.WriteString("Acceptance Criteria must remain a first-class section in the draft, not buried inside prose.\n")
	b.WriteString("Return the same high-level structure as Ticket Assist:\n")
	b.WriteString("Review\n")
	b.WriteString("- What changed and why\n")
	b.WriteString("- Remaining risks or open questions\n\n")
	b.WriteString("Draft\n")
	b.WriteString("Summary: <one concise summary>\n\n")
	b.WriteString("Problem / Goal\n<clear user/business goal>\n\n")
	b.WriteString("Acceptance Criteria\n- [ ] <testable criterion>\n\n")
	b.WriteString("Test / Verification\n- <verification step>\n\n")
	b.WriteString("Implementation Notes\n- <notes or constraints>\n\n")
	b.WriteString("Open Questions\n- <question or None>\n\n")
	b.WriteString("User instruction:\n")
	b.WriteString(strings.TrimSpace(instruction))
	b.WriteString("\n\nCurrent user-edited draft:\n")
	b.WriteString(strings.TrimSpace(currentDraft))
	b.WriteString("\n\nOriginal ticket context:\n")
	m.writeClaudeTicketContext(&b, ctx)
	return strings.TrimSpace(b.String())
}

func (m Model) buildInlineDescriptionAIPrompt(ctx detailRenderContext, action inlineAIAction, instruction string) string {
	var b strings.Builder
	b.WriteString("Provide Description-scoped Jira ticket assistance.\n")
	b.WriteString("Do not update Jira, create tickets, edit files, create branches, run git commands, call GitHub, edit code, or make external changes.\n")
	b.WriteString("Return practical writing help only. The draft must be local TUI text for user review.\n")
	b.WriteString("Selected inline action: ")
	b.WriteString(action.Label)
	b.WriteString("\n")
	if strings.TrimSpace(instruction) != "" {
		b.WriteString("User question/instruction:\n")
		b.WriteString(strings.TrimSpace(instruction))
		b.WriteString("\n")
	}
	b.WriteString("Use this exact high-level structure:\n")
	b.WriteString("Review\n")
	b.WriteString("- What changed or what you noticed\n")
	b.WriteString("- Risks or open questions\n\n")
	b.WriteString("Draft\n")
	if action.ID == "draft_comment" {
		b.WriteString("<Jira comment draft>\n\n")
	} else {
		b.WriteString("<replacement Description draft>\n\n")
	}
	b.WriteString("Ticket:\n")
	m.writeClaudeTicketContext(&b, ctx)
	return strings.TrimSpace(b.String())
}

func (m Model) writeClaudeTicketContext(b *strings.Builder, ctx detailRenderContext) {
	issue := ctx.display
	if issue.Key == "" {
		issue.Key = ctx.selected.Key
	}
	writePromptField(b, "Key", issue.Key)
	writePromptField(b, "Summary", issue.Summary)
	writePromptField(b, "Status", issue.Status)
	writePromptField(b, "Issue Type", issue.IssueType)
	writePromptField(b, "Priority", issue.Priority)
	writePromptField(b, "Assignee", issue.Assignee)
	writePromptField(b, "Reporter", ctx.detail.Reporter)
	if len(ctx.detail.Labels) > 0 {
		writePromptField(b, "Labels", strings.Join(ctx.detail.Labels, ", "))
	}
	if len(ctx.detail.Components) > 0 {
		writePromptField(b, "Components", strings.Join(ctx.detail.Components, ", "))
	}
	description := strings.TrimSpace(ctx.description)
	if description == "" {
		description = strings.TrimSpace(ctx.detail.Description)
	}
	if description != "" {
		b.WriteString("\nDescription:\n")
		b.WriteString(description)
		b.WriteString("\n")
	}
	comments := m.comments[issue.Key]
	if len(comments) > 0 {
		b.WriteString("\nLoaded comments:\n")
		for index, comment := range comments {
			author := displayValue(comment.Author, "Unknown")
			body := strings.TrimSpace(comment.Body)
			if body == "" {
				continue
			}
			fmt.Fprintf(b, "%d. %s: %s\n", index+1, author, body)
		}
	}
}

func (m Model) claudeAssistReviewText() string {
	review, _ := splitClaudeAssistText(m.claudeAssistText)
	return review
}

func claudeAssistDraftFromText(text string) string {
	_, draft := splitClaudeAssistText(text)
	if strings.TrimSpace(draft) == "" {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(draft)
}

func splitClaudeAssistText(text string) (string, string) {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	draftIndex := -1
	for index, line := range lines {
		normalized := strings.Trim(strings.TrimSpace(line), "#: ")
		if strings.EqualFold(normalized, "Draft") {
			draftIndex = index
			break
		}
	}
	if draftIndex < 0 {
		return "", strings.TrimSpace(text)
	}
	review := strings.TrimSpace(strings.Join(lines[:draftIndex], "\n"))
	draft := strings.TrimSpace(strings.Join(lines[draftIndex+1:], "\n"))
	return review, draft
}

func writePromptField(b *strings.Builder, label string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(b, "- %s: %s\n", label, value)
}

func formatClaudeDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	return duration.Round(time.Second).String()
}

func formatClockTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("15:04:05")
}

func (m Model) canUseClaudeSelection() bool {
	if m.mode != modeDetail {
		return false
	}
	section, ok := m.focusedDetailSection()
	return ok && section.ID == "claude"
}

func (m *Model) moveSelectedClaudeAction(delta int) {
	actions := m.claudeActions()
	if len(actions) == 0 {
		m.selectedClaudeAction = 0
		return
	}
	m.selectedClaudeAction = clamp(m.selectedClaudeAction+delta, 0, len(actions)-1)
}

func (m Model) runSelectedClaudeAction() (Model, tea.Cmd) {
	actions := m.claudeActions()
	if len(actions) == 0 {
		m.detailNotice = "Claude ticket assistance is not enabled or available."
		return m, nil
	}
	action := actions[clamp(m.selectedClaudeAction, 0, len(actions)-1)]
	switch action.ID {
	case "ticket_plan":
		return m.startClaudeTicketPlan()
	case "ticket_assist":
		return m.startClaudeTicketAssist()
	default:
		return m, nil
	}
}

func newClaudeAssistEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Edit Claude ticket draft..."
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func newClaudeAssistRefineEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Tell Claude how to refine this draft..."
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func (m *Model) ensureClaudeAssistEditor() {
	if m.claudeAssistEditorReady {
		return
	}
	m.claudeAssistEditor = newClaudeAssistEditor(m.claudeAssistDraft)
	m.claudeAssistEditorReady = true
}

func (m *Model) configureClaudeAssistEditor(width int, rows int) {
	m.ensureClaudeAssistEditor()
	m.claudeAssistEditor.MaxHeight = max(rows, 1)
	m.claudeAssistEditor.MaxWidth = width
	m.claudeAssistEditor.SetWidth(width)
	m.claudeAssistEditor.SetHeight(rows)
	m.claudeAssistEditor.Focus()
}

func (m Model) configuredClaudeAssistEditor(width int, rows int) textarea.Model {
	editor := m.claudeAssistEditor
	if !m.claudeAssistEditorReady {
		editor = newClaudeAssistEditor(m.claudeAssistDraft)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	editor.Focus()
	return editor
}

func (m *Model) ensureClaudeAssistRefineEditor() {
	if m.claudeAssistRefineEditorReady {
		return
	}
	m.claudeAssistRefineEditor = newClaudeAssistRefineEditor(m.claudeAssistRefineInstruction)
	m.claudeAssistRefineEditorReady = true
}

func (m *Model) configureClaudeAssistRefineEditor(width int, rows int) {
	m.ensureClaudeAssistRefineEditor()
	m.claudeAssistRefineEditor.MaxHeight = max(rows, 1)
	m.claudeAssistRefineEditor.MaxWidth = width
	m.claudeAssistRefineEditor.SetWidth(width)
	m.claudeAssistRefineEditor.SetHeight(rows)
	m.claudeAssistRefineEditor.Focus()
}

func (m Model) configuredClaudeAssistRefineEditor(width int, rows int) textarea.Model {
	editor := m.claudeAssistRefineEditor
	if !m.claudeAssistRefineEditorReady {
		editor = newClaudeAssistRefineEditor(m.claudeAssistRefineInstruction)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	editor.Focus()
	return editor
}

func (m *Model) ensureInlineAIInstructionEditor() {
	if m.inlineAIInstructionReady {
		return
	}
	m.inlineAIInstructionEditor = newClaudeAssistRefineEditor(m.inlineAIInstruction)
	m.inlineAIInstructionReady = true
}

func (m *Model) configureInlineAIInstructionEditor(width int, rows int) {
	m.ensureInlineAIInstructionEditor()
	m.inlineAIInstructionEditor.MaxHeight = max(rows, 1)
	m.inlineAIInstructionEditor.MaxWidth = width
	m.inlineAIInstructionEditor.SetWidth(width)
	m.inlineAIInstructionEditor.SetHeight(rows)
	m.inlineAIInstructionEditor.Focus()
}

func (m Model) configuredInlineAIInstructionEditor(width int, rows int) textarea.Model {
	editor := m.inlineAIInstructionEditor
	if !m.inlineAIInstructionReady {
		editor = newClaudeAssistRefineEditor(m.inlineAIInstruction)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	editor.Focus()
	return editor
}

func (m Model) claudeAssistDraftValue() string {
	if !m.claudeAssistEditorReady {
		return m.claudeAssistDraft
	}
	return m.claudeAssistEditor.Value()
}

func (m Model) claudeAssistRefineInstructionValue() string {
	if !m.claudeAssistRefineEditorReady {
		return m.claudeAssistRefineInstruction
	}
	return m.claudeAssistRefineEditor.Value()
}

func (m Model) inlineAIInstructionValue() string {
	if !m.inlineAIInstructionReady {
		return m.inlineAIInstruction
	}
	return m.inlineAIInstructionEditor.Value()
}

func (m Model) updateClaudeAssistEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.claudeAssistPostingComment {
		if msg.String() == "esc" {
			m.claudeAssistOpen = false
		}
		return m, nil
	}
	if m.claudeAssistApplying {
		if msg.String() == "esc" {
			m.claudeAssistOpen = false
		}
		return m, nil
	}
	if m.claudeAssistConfirmComment {
		switch msg.String() {
		case "esc":
			m.claudeAssistConfirmComment = false
			return m, nil
		case "ctrl+s":
			return m.submitClaudeAssistComment()
		}
		return m, nil
	}
	if m.claudeAssistRefining {
		switch msg.String() {
		case "esc":
			m.claudeAssistRefineInstruction = m.claudeAssistRefineInstructionValue()
			m.claudeAssistRefining = false
			return m, nil
		case "ctrl+s":
			return m.submitClaudeAssistRefinement()
		}
		m.configureClaudeAssistRefineEditor(max(32, m.browserLayout(m.width).contentWidth-16), 4)
		editor, cmd := m.claudeAssistRefineEditor.Update(msg)
		m.claudeAssistRefineEditor = editor
		m.claudeAssistRefineInstruction = m.claudeAssistRefineEditor.Value()
		return m, cmd
	}
	if m.claudeAssistConfirmApply {
		switch msg.String() {
		case "esc":
			m.claudeAssistConfirmApply = false
			return m, nil
		case "ctrl+s":
			return m.submitClaudeAssistApply()
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.claudeAssistDraft = m.claudeAssistDraftValue()
		m.claudeAssistOpen = false
		return m, nil
	case "r":
		m.claudeAssistDraft = m.claudeAssistDraftValue()
		m.claudeAssistRefineInstruction = ""
		m.claudeAssistRefineEditor = newClaudeAssistRefineEditor("")
		m.claudeAssistRefineEditorReady = true
		m.claudeAssistRefining = true
		m.detailNotice = ""
		return m, nil
	case "c":
		m.claudeAssistDraft = m.claudeAssistDraftValue()
		if strings.TrimSpace(m.claudeAssistDraft) == "" {
			m.detailNotice = "No Ticket Assist draft to post as a comment."
			return m, nil
		}
		if selected, ok := m.selectedIssue(); ok {
			m.claudeAssistKey = selected.Key
		}
		m.claudeAssistConfirmComment = true
		m.detailNotice = ""
		return m, nil
	case "ctrl+s":
		return m.beginClaudeAssistApply()
	case "ctrl+y":
		return m.copyClaudeAssistDraft()
	}
	m.configureClaudeAssistEditor(max(32, m.browserLayout(m.width).contentWidth-16), m.claudeAssistEditorRows())
	editor, cmd := m.claudeAssistEditor.Update(msg)
	m.claudeAssistEditor = editor
	m.claudeAssistDraft = m.claudeAssistEditor.Value()
	return m, cmd
}

func (m Model) updateInlineAIPicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.inlineAIInstructionOpen {
		switch msg.String() {
		case "esc":
			m.inlineAIInstruction = m.inlineAIInstructionValue()
			m.inlineAIInstructionOpen = false
			return m, nil
		case "ctrl+s":
			actions := inlineDescriptionAIActions()
			action := actions[clamp(m.selectedInlineAIAction, 0, len(actions)-1)]
			return m.submitInlineDescriptionAI(action, m.inlineAIInstructionValue())
		}
		m.configureInlineAIInstructionEditor(max(32, m.browserLayout(m.width).contentWidth-16), 4)
		editor, cmd := m.inlineAIInstructionEditor.Update(msg)
		m.inlineAIInstructionEditor = editor
		m.inlineAIInstruction = m.inlineAIInstructionEditor.Value()
		return m, cmd
	}
	actions := inlineDescriptionAIActions()
	switch msg.String() {
	case "esc":
		m.inlineAIOpen = false
		return m, nil
	case "j", "down":
		m.selectedInlineAIAction = clamp(m.selectedInlineAIAction+1, 0, len(actions)-1)
		return m, nil
	case "k", "up":
		m.selectedInlineAIAction = clamp(m.selectedInlineAIAction-1, 0, len(actions)-1)
		return m, nil
	case "enter":
		return m.runSelectedInlineAIAction()
	default:
		return m, nil
	}
}

func (m Model) runSelectedInlineAIAction() (Model, tea.Cmd) {
	actions := inlineDescriptionAIActions()
	action := actions[clamp(m.selectedInlineAIAction, 0, len(actions)-1)]
	if action.ID == "ask_question" {
		m.inlineAIInstructionOpen = true
		m.inlineAIInstruction = ""
		m.inlineAIInstructionEditor = newClaudeAssistRefineEditor("")
		m.inlineAIInstructionReady = true
		return m, nil
	}
	return m.submitInlineDescriptionAI(action, "")
}

func (m Model) submitInlineDescriptionAI(action inlineAIAction, instruction string) (Model, tea.Cmd) {
	ctx, ok := m.detailRenderContext()
	if !ok {
		m.detailNotice = "No selected ticket for inline AI."
		return m, nil
	}
	if strings.TrimSpace(ctx.description) == "" && strings.TrimSpace(ctx.detail.Description) == "" {
		m.detailNotice = "Description is not loaded yet."
		return m, nil
	}
	key := ctx.display.Key
	if key == "" {
		key = ctx.selected.Key
	}
	m.nextRequestID++
	reqID := m.nextRequestID
	m.activeClaudeAssistReqID = reqID
	m.claudeAssistKey = key
	m.claudeAssistText = ""
	m.claudeAssistErr = nil
	m.claudeAssistLoading = true
	m.claudeAssistOpen = true
	m.claudeAssistStartedAt = m.claudeNow()
	m.claudeAssistProgress = nil
	m.claudeAssistDraft = ""
	m.claudeAssistEditor = newClaudeAssistEditor("")
	m.claudeAssistEditorReady = true
	m.claudeAssistTarget = claudeAssistTargetDescription
	m.inlineAIOpen = false
	m.inlineAIInstructionOpen = false
	m.claudeAssistEvents = make(chan claude.Event, 16)
	runCtx, cancel := context.WithCancel(context.Background())
	m.claudeAssistCancel = cancel
	m.detailNotice = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "inline_description_ai", "submit", workerDiagnosticDetail(reqID, key, nil))
	return m, tea.Batch(
		m.submitClaudeTicketAssist(runCtx, reqID, key, m.buildInlineDescriptionAIPrompt(ctx, action, instruction), m.claudeAssistEvents),
		m.waitForClaudeAssistProgress(reqID, key),
		m.scheduleClaudeAssistTick(reqID),
	)
}

func (m Model) beginClaudeAssistApply() (Model, tea.Cmd) {
	m.claudeAssistDraft = m.claudeAssistDraftValue()
	if !m.claudeConfig.AllowJiraWrites {
		m.detailNotice = "Jira writes are disabled for Claude Ticket Assist. Use ctrl+y to copy the draft."
		return m, nil
	}
	selected, ok := m.selectedIssue()
	if !ok {
		m.detailNotice = "No selected ticket to update."
		return m, nil
	}
	if m.claudeAssistTarget == claudeAssistTargetDescription {
		description := strings.TrimSpace(m.claudeAssistDraft)
		if description == "" {
			m.detailNotice = "Claude description draft has no text to apply."
			return m, nil
		}
		m.claudeAssistKey = selected.Key
		m.claudeAssistApplySummary = ""
		m.claudeAssistApplyDescription = description
		m.claudeAssistConfirmApply = m.claudeConfig.RequireConfirmation
		if m.claudeAssistConfirmApply {
			return m, nil
		}
		return m.submitClaudeAssistApply()
	}
	draft := parseClaudeAssistApplyDraft(m.claudeAssistDraft, selected.Summary)
	if strings.TrimSpace(draft.Description) == "" {
		m.detailNotice = "Claude ticket draft has no description to apply."
		return m, nil
	}
	m.claudeAssistKey = selected.Key
	m.claudeAssistApplySummary = draft.Summary
	m.claudeAssistApplyDescription = draft.Description
	m.claudeAssistConfirmApply = m.claudeConfig.RequireConfirmation
	if m.claudeAssistConfirmApply {
		return m, nil
	}
	return m.submitClaudeAssistApply()
}

func (m Model) submitClaudeAssistApply() (Model, tea.Cmd) {
	if !m.claudeConfig.AllowJiraWrites {
		m.detailNotice = "Jira writes are disabled for Claude Ticket Assist. Use ctrl+y to copy the draft."
		return m, nil
	}
	key := strings.TrimSpace(m.claudeAssistKey)
	if key == "" {
		if selected, ok := m.selectedIssue(); ok {
			key = selected.Key
		}
	}
	if key == "" {
		m.detailNotice = "No selected ticket to update."
		return m, nil
	}
	summary := strings.TrimSpace(m.claudeAssistApplySummary)
	description := strings.TrimSpace(m.claudeAssistApplyDescription)
	if m.claudeAssistTarget == claudeAssistTargetDescription {
		if description == "" {
			m.detailNotice = "Claude description draft needs text before applying."
			return m, nil
		}
		m.nextRequestID++
		m.activeClaudeAssistDescriptionReqID = m.nextRequestID
		m.activeClaudeAssistSummaryReqID = 0
		m.claudeAssistApplying = true
		m.claudeAssistConfirmApply = false
		m.claudeAssistSummaryApplied = true
		m.claudeAssistDescriptionApplied = false
		m.detailNotice = ""
		return m, m.submitUpdateDescription(m.activeClaudeAssistDescriptionReqID, key, description)
	}
	if summary == "" || description == "" {
		m.detailNotice = "Claude ticket draft needs both summary and description before applying."
		return m, nil
	}
	m.nextRequestID++
	m.activeClaudeAssistSummaryReqID = m.nextRequestID
	m.nextRequestID++
	m.activeClaudeAssistDescriptionReqID = m.nextRequestID
	m.claudeAssistApplying = true
	m.claudeAssistConfirmApply = false
	m.claudeAssistSummaryApplied = false
	m.claudeAssistDescriptionApplied = false
	m.detailNotice = ""
	return m, tea.Batch(
		m.submitUpdateSummary(m.activeClaudeAssistSummaryReqID, key, summary),
		m.submitUpdateDescription(m.activeClaudeAssistDescriptionReqID, key, description),
	)
}

func (m Model) submitClaudeAssistRefinement() (Model, tea.Cmd) {
	ctx, ok := m.detailRenderContext()
	if !ok {
		m.detailNotice = "No selected ticket to refine."
		return m, nil
	}
	instruction := strings.TrimSpace(m.claudeAssistRefineInstructionValue())
	if instruction == "" {
		m.detailNotice = "Write a refinement instruction before sending."
		return m, nil
	}
	currentDraft := strings.TrimSpace(m.claudeAssistDraftValue())
	if currentDraft == "" {
		m.detailNotice = "No Claude ticket draft to refine."
		return m, nil
	}
	key := ctx.display.Key
	if key == "" {
		key = ctx.selected.Key
	}
	m.nextRequestID++
	reqID := m.nextRequestID
	m.activeClaudeAssistReqID = reqID
	m.claudeAssistKey = key
	m.claudeAssistErr = nil
	m.claudeAssistLoading = true
	m.claudeAssistRefining = false
	m.claudeAssistOpen = true
	m.claudeAssistStartedAt = m.claudeNow()
	m.claudeAssistProgress = nil
	m.claudeAssistDraft = currentDraft
	m.claudeAssistEvents = make(chan claude.Event, 16)
	runCtx, cancel := context.WithCancel(context.Background())
	m.claudeAssistCancel = cancel
	m.detailNotice = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_assist_refine", "submit", workerDiagnosticDetail(reqID, key, nil))
	return m, tea.Batch(
		m.submitClaudeTicketAssist(runCtx, reqID, key, m.buildClaudeTicketAssistRefinementPrompt(ctx, currentDraft, instruction), m.claudeAssistEvents),
		m.waitForClaudeAssistProgress(reqID, key),
		m.scheduleClaudeAssistTick(reqID),
	)
}

func (m Model) submitClaudeAssistComment() (Model, tea.Cmd) {
	key := strings.TrimSpace(m.claudeAssistKey)
	if key == "" {
		if selected, ok := m.selectedIssue(); ok {
			key = selected.Key
		}
	}
	body := strings.TrimSpace(m.claudeAssistDraftValue())
	if key == "" {
		m.detailNotice = "No selected ticket for comment."
		return m, nil
	}
	if body == "" {
		m.detailNotice = "No Ticket Assist draft to post as a comment."
		return m, nil
	}
	m.nextRequestID++
	m.activeClaudeAssistCommentReqID = m.nextRequestID
	m.claudeAssistConfirmComment = false
	m.claudeAssistPostingComment = true
	m.detailNotice = ""
	return m, m.submitAddComment(m.activeClaudeAssistCommentReqID, key, body, nil)
}

type claudeAssistApplyDraft struct {
	Summary     string
	Description string
}

func parseClaudeAssistApplyDraft(draft string, fallbackSummary string) claudeAssistApplyDraft {
	var descriptionLines []string
	summary := strings.TrimSpace(fallbackSummary)
	for _, line := range strings.Split(draft, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "summary:") {
			if value := strings.TrimSpace(strings.TrimPrefix(trimmed, trimmed[:len("summary:")])); value != "" {
				summary = value
			}
			continue
		}
		descriptionLines = append(descriptionLines, line)
	}
	return claudeAssistApplyDraft{
		Summary:     summary,
		Description: strings.TrimSpace(strings.Join(descriptionLines, "\n")),
	}
}

func (m Model) copyClaudeAssistDraft() (Model, tea.Cmd) {
	draft := strings.TrimSpace(m.claudeAssistDraftValue())
	if draft == "" {
		m.detailNotice = "No Claude ticket draft to copy."
		return m, nil
	}
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "copy-draft",
			target: "Claude ticket draft",
			err:    copyToClipboard(draft),
		}
	}
}
