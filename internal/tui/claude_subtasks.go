package tui

import (
	"fmt"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

type claudeSubtaskReviewKind int

const (
	claudeSubtaskReviewKeep claudeSubtaskReviewKind = iota
	claudeSubtaskReviewAdd
	claudeSubtaskReviewModify
	claudeSubtaskReviewClose
)

type claudeSubtaskReviewItem struct {
	Kind    claudeSubtaskReviewKind
	Key     string
	Summary string
	Raw     string
	Done    bool
	Skipped bool
	Status  string
}

type claudeSubtaskReviewRequestKind int

const (
	claudeSubtaskReviewRequestNone claudeSubtaskReviewRequestKind = iota
	claudeSubtaskReviewRequestTransitions
	claudeSubtaskReviewRequestTransition
	claudeSubtaskReviewRequestComment
)

var claudeIssueKeyPattern = regexp.MustCompile(`\b[A-Z][A-Z0-9]+-\d+\b`)

func parseClaudeSubtaskReviewItems(recommendations string) []claudeSubtaskReviewItem {
	var items []claudeSubtaskReviewItem
	for _, line := range strings.Split(recommendations, "\n") {
		raw := strings.TrimSpace(line)
		raw = strings.TrimPrefix(raw, "-")
		raw = strings.TrimPrefix(raw, "*")
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if fields := strings.Fields(raw); len(fields) > 1 && strings.HasSuffix(fields[0], ".") {
			raw = strings.TrimSpace(strings.TrimPrefix(raw, fields[0]))
		}
		label, detail, ok := strings.Cut(raw, ":")
		if !ok {
			continue
		}
		kind, ok := claudeSubtaskReviewKindFromLabel(label)
		if !ok {
			continue
		}
		detail = strings.TrimSpace(detail)
		key := claudeIssueKeyPattern.FindString(detail)
		summary := strings.TrimSpace(detail)
		if key != "" {
			summary = strings.TrimSpace(strings.TrimPrefix(summary, key))
			summary = strings.TrimLeft(summary, " -:.,")
		}
		summary = strings.TrimSuffix(summary, ".")
		items = append(items, claudeSubtaskReviewItem{
			Kind:    kind,
			Key:     key,
			Summary: summary,
			Raw:     raw,
		})
	}
	return items
}

func claudeSubtaskReviewKindFromLabel(label string) (claudeSubtaskReviewKind, bool) {
	normalized := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(normalized, "keep"):
		return claudeSubtaskReviewKeep, true
	case strings.Contains(normalized, "add") || strings.Contains(normalized, "create"):
		return claudeSubtaskReviewAdd, true
	case strings.Contains(normalized, "rescope") || strings.Contains(normalized, "modify") || strings.Contains(normalized, "update") || strings.Contains(normalized, "change"):
		return claudeSubtaskReviewModify, true
	case strings.Contains(normalized, "remove") || strings.Contains(normalized, "defer") || strings.Contains(normalized, "close") || strings.Contains(normalized, "delete"):
		return claudeSubtaskReviewClose, true
	default:
		return claudeSubtaskReviewKeep, false
	}
}

func (k claudeSubtaskReviewKind) Label() string {
	switch k {
	case claudeSubtaskReviewAdd:
		return "Add child"
	case claudeSubtaskReviewModify:
		return "Modify"
	case claudeSubtaskReviewClose:
		return "Close invalid"
	default:
		return "Keep"
	}
}

func (m Model) openClaudeSubtaskReview(parentKey string, parentSummary string, recommendations string) Model {
	items := parseClaudeSubtaskReviewItems(recommendations)
	if len(items) == 0 {
		return m
	}
	m.claudeSubtaskReviewOpen = true
	m.claudeSubtaskReviewParentKey = strings.TrimSpace(parentKey)
	m.claudeSubtaskReviewParentSummary = strings.TrimSpace(parentSummary)
	m.claudeSubtaskReviewItems = items
	m.selectedClaudeSubtaskReview = 0
	m.claudeSubtaskReviewApplying = false
	m.claudeSubtaskReviewPendingIndex = -1
	m.claudeSubtaskReviewPendingKind = claudeSubtaskReviewRequestNone
	m.detailNotice = fmt.Sprintf("Ticket assist draft applied. Review %d subtask recommendations.", len(items))
	return m
}

func (m Model) renderClaudeSubtaskReviewDialog(width int) string {
	bodyWidth := min(max(36, width-10), 100)
	lines := []string{
		m.theme.Text.Render("Review subtask recommendations before changing epic children."),
		m.theme.Muted.Render("Parent: ") + m.theme.Text.Render(displayValue(m.claudeSubtaskReviewParentKey, "selected ticket")),
		"",
	}
	if len(m.claudeSubtaskReviewItems) == 0 {
		lines = append(lines, m.detailEmptyState("No parsed recommendations.", bodyWidth))
	} else {
		selected := clamp(m.selectedClaudeSubtaskReview, 0, len(m.claudeSubtaskReviewItems)-1)
		for index, item := range m.claudeSubtaskReviewItems {
			cursor := " "
			if index == selected {
				cursor = ">"
			}
			status := item.Status
			if item.Done {
				status = "done"
			}
			if item.Skipped {
				status = "skipped"
			}
			target := item.Key
			if target == "" && item.Kind == claudeSubtaskReviewAdd {
				target = "new child"
			}
			if target == "" {
				target = "parent"
			}
			row := fmt.Sprintf("%s %-13s %-12s %s", cursor, item.Kind.Label(), target, displayValue(item.Summary, item.Raw))
			if status != "" {
				row += "  [" + status + "]"
			}
			lines = append(lines, truncate(row, bodyWidth))
		}
	}
	lines = append(lines, "", m.theme.Muted.Render("enter apply  s skip  esc done"))
	return m.renderDetailDialog(width, "Review Subtask Changes", m.claudeSubtaskReviewParentKey, strings.Join(lines, "\n"), "j/k select  enter apply  s skip  esc done")
}

func (m Model) updateClaudeSubtaskReview(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.claudeSubtaskReviewApplying {
		if msg.String() == "esc" {
			m.claudeSubtaskReviewOpen = false
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.claudeSubtaskReviewOpen = false
		m.detailNotice = "Subtask recommendation review closed."
		return m, nil
	case "j", "down":
		m.selectedClaudeSubtaskReview = clamp(m.selectedClaudeSubtaskReview+1, 0, max(0, len(m.claudeSubtaskReviewItems)-1))
		return m, nil
	case "k", "up":
		m.selectedClaudeSubtaskReview = clamp(m.selectedClaudeSubtaskReview-1, 0, max(0, len(m.claudeSubtaskReviewItems)-1))
		return m, nil
	case "s":
		m.markSelectedClaudeSubtaskReviewSkipped()
		return m, nil
	case "enter":
		return m.applySelectedClaudeSubtaskReview()
	default:
		return m, nil
	}
}

func (m *Model) markSelectedClaudeSubtaskReviewSkipped() {
	if len(m.claudeSubtaskReviewItems) == 0 {
		return
	}
	index := clamp(m.selectedClaudeSubtaskReview, 0, len(m.claudeSubtaskReviewItems)-1)
	m.claudeSubtaskReviewItems[index].Skipped = true
	m.claudeSubtaskReviewItems[index].Status = "skipped"
	m.detailNotice = "Subtask recommendation skipped."
}

func (m Model) applySelectedClaudeSubtaskReview() (Model, tea.Cmd) {
	if len(m.claudeSubtaskReviewItems) == 0 {
		return m, nil
	}
	index := clamp(m.selectedClaudeSubtaskReview, 0, len(m.claudeSubtaskReviewItems)-1)
	item := m.claudeSubtaskReviewItems[index]
	if item.Done || item.Skipped {
		m.detailNotice = "Subtask recommendation already handled."
		return m, nil
	}
	switch item.Kind {
	case claudeSubtaskReviewKeep:
		m.claudeSubtaskReviewItems[index].Done = true
		m.claudeSubtaskReviewItems[index].Status = "kept"
		m.detailNotice = "Subtask recommendation marked kept."
		return m, nil
	case claudeSubtaskReviewAdd:
		m.claudeSubtaskReviewItems[index].Done = true
		m.claudeSubtaskReviewItems[index].Status = "create opened"
		m.claudeSubtaskReviewOpen = false
		summary := displayValue(item.Summary, "New child ticket")
		description := claudeSubtaskReviewCommentBody(item, "Create this child ticket under "+m.claudeSubtaskReviewParentKey+".")
		m, cmd := m.startCreateIssueWithParentDraft(m.claudeSubtaskReviewParentKey, m.claudeSubtaskReviewParentSummary, summary, description)
		m.detailNotice = "Opened child ticket create flow from Ticket Assist recommendation."
		return m, cmd
	case claudeSubtaskReviewModify:
		if item.Key == "" {
			m.detailNotice = "Modify recommendation needs an issue key."
			return m, nil
		}
		return m.submitClaudeSubtaskReviewComment(index, item, "Review this recommended scope change.")
	case claudeSubtaskReviewClose:
		if item.Key == "" {
			m.detailNotice = "Close recommendation needs an issue key."
			return m, nil
		}
		if transitions := m.transitions[item.Key]; len(transitions) > 0 {
			return m.applyClaudeSubtaskReviewClose(index, item, transitions)
		}
		m.nextRequestID++
		m.activeClaudeSubtaskReviewReqID = m.nextRequestID
		m.claudeSubtaskReviewPendingIndex = index
		m.claudeSubtaskReviewPendingKind = claudeSubtaskReviewRequestTransitions
		m.claudeSubtaskReviewApplying = true
		m.claudeSubtaskReviewItems[index].Status = "loading transitions"
		m.detailNotice = "Loading close transitions for " + item.Key + "."
		return m, m.submitIssueTransitions(m.activeClaudeSubtaskReviewReqID, item.Key)
	default:
		return m, nil
	}
}

func (m Model) applyClaudeSubtaskReviewClose(index int, item claudeSubtaskReviewItem, transitions []jira.Transition) (Model, tea.Cmd) {
	transition, ok := preferredClaudeSubtaskCloseTransition(transitions)
	if !ok {
		return m.submitClaudeSubtaskReviewComment(index, item, "No safe close-as-invalid transition was available; review this child manually.")
	}
	m.nextRequestID++
	m.activeClaudeSubtaskReviewReqID = m.nextRequestID
	m.claudeSubtaskReviewPendingIndex = index
	m.claudeSubtaskReviewPendingKind = claudeSubtaskReviewRequestTransition
	m.claudeSubtaskReviewApplying = true
	m.claudeSubtaskReviewItems[index].Status = "closing"
	m.detailNotice = "Closing " + item.Key + " with " + displayValue(transition.Name, transition.ToStatus) + "."
	return m, m.submitIssueTransition(m.activeClaudeSubtaskReviewReqID, item.Key, transition, nil)
}

func (m Model) submitClaudeSubtaskReviewComment(index int, item claudeSubtaskReviewItem, note string) (Model, tea.Cmd) {
	target := item.Key
	if target == "" {
		target = m.claudeSubtaskReviewParentKey
	}
	m.nextRequestID++
	m.activeClaudeSubtaskReviewReqID = m.nextRequestID
	m.claudeSubtaskReviewPendingIndex = index
	m.claudeSubtaskReviewPendingKind = claudeSubtaskReviewRequestComment
	m.claudeSubtaskReviewApplying = true
	m.claudeSubtaskReviewItems[index].Status = "commenting"
	m.detailNotice = "Posting subtask recommendation comment to " + target + "."
	return m, m.submitAddComment(m.activeClaudeSubtaskReviewReqID, target, claudeSubtaskReviewCommentBody(item, note), nil)
}

func (m Model) handleClaudeSubtaskReviewResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeClaudeSubtaskReviewReqID {
		return m, nil
	}
	index := m.claudeSubtaskReviewPendingIndex
	if index < 0 || index >= len(m.claudeSubtaskReviewItems) {
		m.claudeSubtaskReviewApplying = false
		return m, nil
	}
	item := m.claudeSubtaskReviewItems[index]
	if result.Err != nil {
		m.claudeSubtaskReviewApplying = false
		m.claudeSubtaskReviewItems[index].Status = "failed"
		m.detailNotice = "Subtask recommendation failed: " + result.Err.Error()
		return m, nil
	}
	switch result.Kind {
	case worker.KindGetTransitions:
		if result.GetTransitions == nil {
			m.claudeSubtaskReviewApplying = false
			m.claudeSubtaskReviewItems[index].Status = "failed"
			m.detailNotice = "Subtask recommendation failed: " + worker.ErrInvalidRequest.Error()
			return m, nil
		}
		m.cacheIssueTransitions(result.GetTransitions.Key, result.GetTransitions.Transitions, result.GetTransitions.SyncedAt)
		m.claudeSubtaskReviewApplying = false
		return m.applyClaudeSubtaskReviewClose(index, item, result.GetTransitions.Transitions)
	case worker.KindTransitionIssue:
		if result.TransitionIssue == nil {
			m.claudeSubtaskReviewApplying = false
			m.claudeSubtaskReviewItems[index].Status = "failed"
			m.detailNotice = "Subtask recommendation failed: " + worker.ErrInvalidRequest.Error()
			return m, nil
		}
		m.updateIssueStatus(result.TransitionIssue.Key, result.TransitionIssue.ToStatus)
		m.claudeSubtaskReviewItems[index].Done = true
		m.claudeSubtaskReviewItems[index].Status = "closed"
		m.detailNotice = "Closed " + result.TransitionIssue.Key + " from Ticket Assist recommendation."
	case worker.KindAddComment:
		if result.AddComment == nil {
			m.claudeSubtaskReviewApplying = false
			m.claudeSubtaskReviewItems[index].Status = "failed"
			m.detailNotice = "Subtask recommendation failed: " + worker.ErrInvalidRequest.Error()
			return m, nil
		}
		m.invalidateIssueComments(result.AddComment.Key)
		m.claudeSubtaskReviewItems[index].Done = true
		m.claudeSubtaskReviewItems[index].Status = "commented"
		m.detailNotice = "Posted subtask recommendation comment to " + result.AddComment.Key + "."
	default:
		return m, nil
	}
	m.claudeSubtaskReviewApplying = false
	m.activeClaudeSubtaskReviewReqID = 0
	m.claudeSubtaskReviewPendingIndex = -1
	m.claudeSubtaskReviewPendingKind = claudeSubtaskReviewRequestNone
	return m, nil
}

func preferredClaudeSubtaskCloseTransition(transitions []jira.Transition) (jira.Transition, bool) {
	names := []string{"invalid", "won't do", "wont do", "won\u2019t do", "cancelled", "canceled", "closed", "done"}
	for _, name := range names {
		for _, transition := range transitions {
			if !transition.IsAvailable {
				continue
			}
			label := strings.ToLower(transition.Name + " " + transition.ToStatus)
			if !strings.Contains(label, name) {
				continue
			}
			if unsupported := unsupportedRequiredTransitionFields(transition.Fields); len(unsupported) > 0 {
				continue
			}
			if transitionNeedsFieldForm(transition) {
				continue
			}
			if transition.ID != "" {
				return transition, true
			}
		}
	}
	return jira.Transition{}, false
}

func claudeSubtaskReviewCommentBody(item claudeSubtaskReviewItem, note string) string {
	parts := []string{"Ticket Assist Subtask Recommendation"}
	if note = strings.TrimSpace(note); note != "" {
		parts = append(parts, "", note)
	}
	parts = append(parts, "", item.Raw)
	return strings.Join(parts, "\n")
}
