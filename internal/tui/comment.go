package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/mentiondetect"
)

func (m Model) commentEditorRows() int {
	reserved := 7
	if m.commentConfirm {
		reserved = 9
	}
	if m.detailNotice != "" {
		reserved += 2
	}
	reserved += m.commentLinkPreviewRows()
	reserved += m.commentMentionPreviewRows()
	reserved += m.mentionPickerRows()
	return max(2, m.boundedPanelBodyRows(reserved))
}

func (m Model) renderCommentComposer(layout browserLayout) string {
	selected := m.issues[m.selected]
	bodyWidth := max(32, layout.contentWidth-8)
	editorRows := m.commentEditorRows()
	var b strings.Builder
	b.WriteString(m.renderCommentComposerTitle(selected, bodyWidth))
	b.WriteString("\n\n")
	if m.commentSubmitting {
		b.WriteString(m.detailSectionHeader("comment-compose", m.commentSubmittingTitle(), "", bodyWidth))
		b.WriteString("\n")
		b.WriteString(m.theme.Muted.Render(m.commentSubmittingMessage()))
	} else if m.commentConfirm {
		b.WriteString(m.detailSectionHeader("comment-compose", m.commentReviewTitle(), "", bodyWidth))
		b.WriteString("\n")
		b.WriteString(m.renderCommentDraft(bodyWidth, editorRows))
		if preview := m.renderCommentLinkPreview(bodyWidth); preview != "" {
			b.WriteString("\n\n")
			b.WriteString(preview)
		}
		if preview := m.renderCommentMentionPreview(bodyWidth); preview != "" {
			b.WriteString("\n\n")
			b.WriteString(preview)
		}
		b.WriteString("\n\n")
		b.WriteString(m.theme.Warning.Render(m.commentConfirmPrompt(selected.Key)))
	} else {
		b.WriteString(m.detailSectionHeader("comment-compose", m.commentComposerTitle(), "", bodyWidth))
		b.WriteString("\n")
		b.WriteString(m.renderCommentDraft(bodyWidth, editorRows))
		if preview := m.renderCommentLinkPreview(bodyWidth); preview != "" {
			b.WriteString("\n\n")
			b.WriteString(preview)
		}
		if preview := m.renderCommentMentionPreview(bodyWidth); preview != "" {
			b.WriteString("\n\n")
			b.WriteString(preview)
		}
		if picker := m.renderMentionPicker(bodyWidth); picker != "" {
			b.WriteString("\n\n")
			b.WriteString(picker)
		}
	}
	if m.detailNotice != "" {
		b.WriteString("\n\n")
		b.WriteString(m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.theme.ActivePane.Width(layout.contentWidth).Render(b.String())
}

func (m Model) commentComposerTitle() string {
	if m.commentEditing {
		return "Edit Comment"
	}
	return "Add Comment"
}

func (m Model) commentReviewTitle() string {
	if m.commentEditing {
		return "Review Comment Edit"
	}
	return "Review Comment"
}

func (m Model) commentSubmittingTitle() string {
	if m.commentEditing {
		return "Updating Comment"
	}
	return "Posting Comment"
}

func (m Model) commentSubmittingMessage() string {
	if m.commentEditing {
		return "Updating comment..."
	}
	return "Posting comment..."
}

func (m Model) commentConfirmPrompt(issueKey string) string {
	if m.commentEditing {
		return "Update this comment on " + issueKey + "?"
	}
	return "Post this comment to " + issueKey + "?"
}

func (m Model) renderCommentComposerTitle(issue jira.Issue, width int) string {
	title := issue.Key
	if issue.Summary != "" {
		title += "  " + issue.Summary
	}
	return m.theme.Key.Render(truncate(title, width))
}

func (m Model) renderCommentDraft(width int, rows int) string {
	rows = max(1, rows)
	editor := m.configuredCommentEditor(width, rows)
	lineCount := editor.LineCount()
	if lineCount > rows && rows > 1 {
		editor.SetHeight(rows - 1)
		start := editor.ScrollYOffset() + 1
		end := min(lineCount, editor.ScrollYOffset()+editor.Height())
		indicator := fmt.Sprintf("Lines %d-%d of %d", start, end, lineCount)
		if start > 1 {
			indicator += "  PgUp previous"
		}
		if end < lineCount {
			indicator += "  PgDn next"
		}
		body := editor.View() + "\n" + m.theme.Muted.Render(truncate(indicator, width))
		return m.theme.Input.Width(width).Height(rows).Render(body)
	}
	return m.theme.Input.Width(width).Height(rows).Render(editor.View())
}

func (m Model) renderCommentLinkPreview(width int) string {
	links := collectDetailLinks(m.commentEditorValue())
	if len(links) == 0 {
		return ""
	}

	limit := min(3, len(links))
	lines := make([]string, 0, limit+2)
	lines = append(lines, m.detailSectionHeader("comment-links", "Detected Links", "", width))
	for index := 0; index < limit; index++ {
		link := links[index]
		prefix := fmt.Sprintf("[%d] %-5s ", index+1, link.Kind)
		lines = append(lines,
			m.theme.Key.Render(fmt.Sprintf("[%d]", index+1))+" "+
				m.theme.Muted.Render(fmt.Sprintf("%-5s", link.Kind))+" "+
				m.theme.Text.Render(truncate(linkDisplayText(link), max(12, width-lipgloss.Width(prefix)))),
		)
	}
	if len(links) > limit {
		lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("+%d more", len(links)-limit)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) commentLinkPreviewRows() int {
	links := collectDetailLinks(m.commentEditorValue())
	if len(links) == 0 {
		return 0
	}
	rows := 2 + min(3, len(links))
	if len(links) > 3 {
		rows++
	}
	return rows
}

func (m Model) renderCommentMentionPreview(width int) string {
	mentions := m.unresolvedCommentMentions()
	if len(mentions) == 0 {
		return ""
	}

	limit := min(3, len(mentions))
	lines := make([]string, 0, limit+2)
	lines = append(lines, m.detailSectionHeader("comment-mentions", "Unresolved Mentions", "", width))
	for index := 0; index < limit; index++ {
		mention := "@" + mentions[index].Query
		lines = append(lines, m.theme.Warning.Render(truncate(mention, width)))
	}
	if len(mentions) > limit {
		lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("+%d more", len(mentions)-limit)))
	}
	lines = append(lines, m.theme.Muted.Render("Use @ to select Jira users so mentions notify the right account."))
	return strings.Join(lines, "\n")
}

func (m Model) commentMentionPreviewRows() int {
	mentions := m.unresolvedCommentMentions()
	if len(mentions) == 0 {
		return 0
	}
	rows := 3 + min(3, len(mentions))
	if len(mentions) > 3 {
		rows++
	}
	return rows
}

func (m Model) unresolvedCommentMentions() []mentiondetect.Mention {
	detected := mentiondetect.Detect(m.commentEditorValue())
	if len(detected) == 0 {
		return nil
	}
	unresolved := make([]mentiondetect.Mention, 0, len(detected))
	for _, mention := range detected {
		if !m.isResolvedMention("@" + mention.Query) {
			unresolved = append(unresolved, mention)
		}
	}
	return unresolved
}

func (m Model) isResolvedMention(value string) bool {
	for _, mention := range m.commentMentions {
		if strings.HasPrefix(mention.Text, value) && strings.Contains(m.commentEditorValue(), mention.Text) {
			return true
		}
	}
	return false
}

func (m Model) mentionPickerRows() int {
	if !m.mentionPickerOpen {
		return 0
	}
	return 14
}

func (m Model) renderMentionPicker(width int) string {
	if !m.mentionPickerOpen {
		return ""
	}
	lines := []string{
		m.detailSectionHeader("mention-picker", "Mention User", "type to search Jira users", width),
		m.theme.Muted.Render("Filter: ") + m.theme.Text.Render(m.mentionQuery),
	}
	if m.mentionSearchLoading {
		lines = append(lines, m.theme.Muted.Render("Searching Jira users..."))
	} else if m.mentionSearchErr != nil {
		lines = append(lines, m.theme.Error.Render(truncate("User search failed: "+m.mentionSearchErr.Error(), width)))
	} else if strings.TrimSpace(m.mentionQuery) == "" {
		lines = append(lines, m.theme.Muted.Render("Start typing a name or email."))
	} else if len(m.mentionUsers) == 0 {
		lines = append(lines, m.theme.Muted.Render("No Jira users matched."))
	}
	lines = append(lines, m.renderMentionUsers(width)...)
	return m.theme.Panel.Width(width).Render(strings.TrimRight(strings.Join(lines, "\n"), "\n"))
}

func (m Model) renderMentionUsers(width int) []string {
	if len(m.mentionUsers) == 0 {
		return nil
	}
	rows := max(1, m.mentionPickerRows()-4)
	cursor := clamp(m.mentionCursor, 0, len(m.mentionUsers)-1)
	options := make([]choiceListOption, 0, len(m.mentionUsers))
	for _, user := range m.mentionUsers {
		options = append(options, choiceListOption{Label: m.mentionDisplayName(user)})
	}
	return m.renderChoiceList(options, cursor, width, rows)
}

func (m *Model) startCommentComposer() {
	m.mode = modeComment
	m.linkFocus = false
	m.hierarchyFocus = false
	m.commentFocus = false
	m.actionFocus = false
	m.commentDraft = ""
	m.commentEditor = newCommentEditor(m.commentDraft)
	m.commentEditorReady = true
	m.commentConfirm = false
	m.commentSubmitting = false
	m.commentEditing = false
	m.commentEditIssueKey = ""
	m.commentEditID = ""
	m.commentEditOriginal = ""
	m.commentMentions = nil
	m.closeMentionPicker()
	m.detailNotice = ""
}

func (m Model) startSelectedCommentEditor() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		m.detailNotice = "No issue selected."
		return m, nil
	}
	comment, ok := m.selectedCommentForEdit()
	if !ok || strings.TrimSpace(comment.ID) == "" {
		m.detailNotice = "Select a comment before editing."
		return m, nil
	}
	m.mode = modeComment
	m.linkFocus = false
	m.hierarchyFocus = false
	m.commentFocus = false
	m.actionFocus = false
	m.commentDraft = comment.Body
	m.commentEditor = newCommentEditor(comment.Body)
	m.commentEditorReady = true
	m.commentConfirm = false
	m.commentSubmitting = false
	m.commentEditing = true
	m.commentEditIssueKey = selected.Key
	m.commentEditID = comment.ID
	m.commentEditOriginal = comment.Body
	m.commentMentions = nil
	m.closeMentionPicker()
	m.detailNotice = ""
	return m, nil
}

func (m Model) updateCommentComposer(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c":
			m.workers.Stop()
			return m, tea.Quit
		case "esc":
			if m.mentionPickerOpen {
				return m.updateMentionPicker(msg)
			}
			m.mode = modeDetail
			m.commentDraft = ""
			m.commentEditor = newCommentEditor("")
			m.commentEditorReady = true
			m.commentConfirm = false
			m.commentSubmitting = false
			m.commentEditing = false
			m.commentEditIssueKey = ""
			m.commentEditID = ""
			m.commentEditOriginal = ""
			m.commentMentions = nil
			m.closeMentionPicker()
			m.detailNotice = "Comment canceled."
			return m, nil
		}
	}
	if m.commentSubmitting {
		return m, nil
	}
	if m.commentConfirm {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "y":
				return m.submitCommentDraft()
			case "n":
				m.commentConfirm = false
				m.detailNotice = ""
				m.ensureCommentEditor()
				m.commentEditor.Focus()
				return m, nil
			}
		}
		return m, nil
	}

	if m.mentionPickerOpen {
		return m.updateMentionPicker(msg)
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+b":
			m.insertCommentFormatting("**", "**")
			return m, nil
		case "ctrl+e":
			m.insertCommentFormatting("_", "_")
			return m, nil
		case "ctrl+g":
			m.insertCommentFormatting("`", "`")
			return m, nil
		case "ctrl+l":
			m.insertCommentBullet()
			return m, nil
		case "@":
			m.openMentionPicker()
			return m, nil
		case "tab", "ctrl+s":
			m.commentDraft = m.commentEditorValue()
			if strings.TrimSpace(m.commentDraft) == "" {
				m.detailNotice = "Write a comment before posting."
				return m, nil
			}
			m.commentConfirm = true
			m.ensureCommentEditor()
			m.commentEditor.Blur()
			m.detailNotice = ""
			return m, nil
		}
	}

	m.ensureCommentEditor()
	m.configureCommentEditor()
	editor, cmd := m.commentEditor.Update(msg)
	m.commentEditor = editor
	m.commentDraft = m.commentEditor.Value()
	if strings.TrimSpace(m.commentDraft) != "" {
		m.detailNotice = ""
	}
	return m, cmd
}

func (m Model) updateMentionPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c":
			m.workers.Stop()
			return m, tea.Quit
		case "esc":
			literal := "@"
			if m.mentionQuery != "" {
				literal += m.mentionQuery
			}
			m.closeMentionPicker()
			m.insertCommentText(literal)
			return m, nil
		case "enter":
			if user, ok := m.selectedMentionUser(); ok {
				mention := jira.Mention{
					AccountID: user.AccountID,
					Text:      m.mentionText(user),
				}
				m.closeMentionPicker()
				m.insertCommentText(mention.Text)
				m.commentMentions = append(m.commentMentions, mention)
				return m, nil
			}
			return m, nil
		case "up":
			m.moveMentionCursor(-1)
			return m, nil
		case "down":
			m.moveMentionCursor(1)
			return m, nil
		}
		if !m.mentionQueryEditorReady {
			m.mentionQueryEditor = newUserSearchInput(m.mentionQuery)
			m.mentionQueryEditorReady = true
		}
		previous := m.mentionQueryEditor.Value()
		editor, _ := m.mentionQueryEditor.Update(keyMsg)
		m.mentionQueryEditor = editor
		query := strings.TrimSpace(editor.Value())
		if strings.TrimSpace(previous) == query {
			return m, nil
		}
		m.setMentionQuery(query)
		if m.mentionQuery == "" {
			m.mentionSearchLoading = false
			m.mentionSearchErr = nil
			m.mentionUsers = nil
			m.mentionCursor = 0
			return m, nil
		}
		m.nextRequestID++
		m.mentionSearchReqID = m.nextRequestID
		m.mentionSearchLoading = true
		m.mentionSearchErr = nil
		return m, m.submitUserSearch(m.mentionSearchReqID, m.mentionQuery)
	}
	return m, nil
}

func newCommentEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Write a Jira comment..."
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func (m *Model) ensureCommentEditor() {
	if m.commentEditorReady {
		return
	}
	m.commentEditor = newCommentEditor(m.commentDraft)
	m.commentEditorReady = true
}

func (m *Model) configureCommentEditor() {
	m.ensureCommentEditor()
	width := max(32, m.browserLayout(m.width).contentWidth-8)
	rows := m.commentEditorRows()
	m.commentEditor.MaxHeight = max(rows, 1)
	m.commentEditor.MaxWidth = width
	m.commentEditor.SetWidth(width)
	m.commentEditor.SetHeight(rows)
}

func (m Model) configuredCommentEditor(width int, rows int) textarea.Model {
	editor := m.commentEditor
	if !m.commentEditorReady {
		editor = newCommentEditor(m.commentDraft)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	if !m.commentConfirm && !m.commentSubmitting && !m.mentionPickerOpen {
		editor.Focus()
	}
	return editor
}

func (m *Model) openMentionPicker() {
	m.mentionPickerOpen = true
	m.mentionSearchLoading = false
	m.mentionSearchErr = nil
	m.mentionUsers = nil
	m.mentionCursor = 0
	m.setMentionQuery("")
	m.mentionQueryEditor = newUserSearchInput("")
	m.mentionQueryEditorReady = true
	m.ensureCommentEditor()
	m.commentEditor.Blur()
}

func (m *Model) closeMentionPicker() {
	m.mentionPickerOpen = false
	m.mentionQuery = ""
	m.mentionQueryEditor = textinput.Model{}
	m.mentionQueryEditorReady = false
	m.mentionSearchLoading = false
	m.mentionSearchErr = nil
	m.ensureCommentEditor()
	m.commentEditor.Focus()
}

func (m *Model) setMentionQuery(query string) {
	m.mentionQuery = strings.TrimSpace(query)
	if m.mentionQueryEditorReady && strings.TrimSpace(m.mentionQueryEditor.Value()) != m.mentionQuery {
		m.mentionQueryEditor.SetValue(m.mentionQuery)
		m.mentionQueryEditor.CursorEnd()
	}
	m.mentionCursor = 0
}

func (m *Model) moveMentionCursor(delta int) {
	if len(m.mentionUsers) == 0 {
		m.mentionCursor = 0
		return
	}
	m.mentionCursor = clamp(m.mentionCursor+delta, 0, len(m.mentionUsers)-1)
}

func (m Model) selectedMentionUser() (jira.User, bool) {
	if len(m.mentionUsers) == 0 {
		return jira.User{}, false
	}
	index := clamp(m.mentionCursor, 0, len(m.mentionUsers)-1)
	return m.mentionUsers[index], true
}

func (m *Model) insertCommentText(value string) {
	m.ensureCommentEditor()
	m.configureCommentEditor()
	m.commentEditor.InsertString(value)
	m.commentDraft = m.commentEditor.Value()
	if strings.TrimSpace(m.commentDraft) != "" {
		m.detailNotice = ""
	}
}

func (m *Model) insertCommentFormatting(open string, close string) {
	m.ensureCommentEditor()
	m.configureCommentEditor()
	m.commentEditor.InsertString(open + close)
	if close != "" {
		m.commentEditor.SetCursorColumn(max(0, m.commentEditor.Column()-len(close)))
	}
	m.commentDraft = m.commentEditor.Value()
	if strings.TrimSpace(m.commentDraft) != "" {
		m.detailNotice = ""
	}
}

func (m *Model) insertCommentBullet() {
	m.ensureCommentEditor()
	m.configureCommentEditor()
	if value := m.commentEditor.Value(); value != "" && !strings.HasSuffix(value, "\n") {
		m.commentEditor.InsertString("\n")
	}
	m.commentEditor.InsertString("- ")
	m.commentDraft = m.commentEditor.Value()
	if strings.TrimSpace(m.commentDraft) != "" {
		m.detailNotice = ""
	}
}

func (m Model) mentionText(user jira.User) string {
	return "@" + m.mentionDisplayName(user)
}

func (m Model) mentionDisplayName(user jira.User) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	if user.Email != "" {
		return user.Email
	}
	return user.AccountID
}

func (m Model) commentEditorValue() string {
	if !m.commentEditorReady {
		return m.commentDraft
	}
	return m.commentEditor.Value()
}

func (m Model) submitCommentDraft() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		m.detailNotice = "No issue selected."
		m.commentConfirm = false
		return m, nil
	}
	m.commentDraft = m.commentEditorValue()
	body := strings.TrimSpace(m.commentDraft)
	if body == "" {
		m.detailNotice = "Write a comment before posting."
		m.commentConfirm = false
		return m, nil
	}
	m.nextRequestID++
	m.activeCommentReqID = m.nextRequestID
	m.commentRequestKey = selected.Key
	m.commentSubmitting = true
	m.detailNotice = ""
	if m.commentEditing {
		issueKey := strings.TrimSpace(m.commentEditIssueKey)
		commentID := strings.TrimSpace(m.commentEditID)
		if issueKey == "" || commentID == "" {
			m.commentSubmitting = false
			m.commentConfirm = false
			m.detailNotice = "Comment update failed: missing comment target."
			return m, nil
		}
		m.commentRequestKey = issueKey
		return m, m.submitUpdateComment(m.activeCommentReqID, issueKey, commentID, body, m.commentMentions)
	}
	return m, m.submitAddComment(m.activeCommentReqID, selected.Key, body, m.commentMentions)
}
