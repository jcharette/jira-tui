package configui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/ui"
)

type Model struct {
	path string

	fields []field

	section  int
	selected int
	editing  bool
	editor   textinput.Model

	width  int
	height int

	problems []string
	err      error
	saved    bool
	cancel   bool
	theme    ui.Theme

	testing     bool
	testStatus  string
	testDetails string
}

type field struct {
	section int
	label   string
	value   string
	secret  bool
	boolean bool
	options []string
	help    string
}

type layout struct {
	stacked     bool
	sectionPane int
	fieldPane   int
}

type footerBinding struct {
	key   string
	label string
	group string
}

const (
	sectionAccount = iota
	sectionQueries
	sectionAppearance
	sectionDisplay
	sectionRuntime
	sectionGit
	sectionClaude
	sectionNotifications
	sectionTest
	sectionSave
	sectionQuit
)

var sectionLabels = []string{
	"Jira Account",
	"Queries",
	"Appearance",
	"Display",
	"Runtime",
	"Git",
	"Claude",
	"Notifications",
	"Test Connection",
	"Save and Exit",
	"Quit Without Saving",
}

type connectionTestMsg struct {
	status  string
	details string
	err     error
}

func NewModel(path string, cfg config.Config, problems []string) Model {
	return Model{
		path: path,
		fields: []field{
			{section: sectionAccount, label: "Active Profile", value: cfg.ActiveProfile},
			{section: sectionAccount, label: "Base URL", value: cfg.BaseURL},
			{section: sectionAccount, label: "Email", value: cfg.Email},
			{section: sectionAccount, label: "API Token", value: cfg.APIToken, secret: true, help: "Saved tokens are stored in the OS keychain: macOS Keychain, Windows Credential Manager, or Linux Secret Service. The config file keeps only a keyring reference."},
			{section: sectionQueries, label: "Default Project", value: cfg.DefaultProject},
			{section: sectionQueries, label: "Default JQL", value: cfg.DefaultJQL},
			{section: sectionAppearance, label: "Primary", value: cfg.Theme.Primary},
			{section: sectionAppearance, label: "Secondary", value: cfg.Theme.Secondary},
			{section: sectionAppearance, label: "Accent", value: cfg.Theme.Accent},
			{section: sectionAppearance, label: "Success", value: cfg.Theme.Success},
			{section: sectionAppearance, label: "Warning", value: cfg.Theme.Warning},
			{section: sectionAppearance, label: "Error", value: cfg.Theme.Error},
			{section: sectionAppearance, label: "Muted", value: cfg.Theme.Muted},
			{section: sectionAppearance, label: "Border", value: cfg.Theme.Border},
			{section: sectionAppearance, label: "Surface", value: cfg.Theme.Surface},
			{section: sectionAppearance, label: "Text", value: cfg.Theme.Text},
			{section: sectionDisplay, label: "Symbol Mode", value: cfg.Display.SymbolMode, options: []string{"auto", "symbols", "emoji", "nerd", "plain"}, help: "Auto detects Nerd-capable iTerm profiles, then falls back to colored safe glyphs.\nNerd setup: brew install --cask font-jetbrains-mono-nerd-font\nThen set your terminal profile font to JetBrainsMono Nerd Font, restart the terminal, and select nerd if auto does not switch."},
			{section: sectionRuntime, label: "Refresh Interval", value: cfg.RefreshInterval.String()},
			{section: sectionRuntime, label: "Request Timeout", value: cfg.RequestTimeout.String()},
			{section: sectionRuntime, label: "Workers", value: strconv.Itoa(cfg.WorkerCount)},
			{section: sectionRuntime, label: "Queue Size", value: strconv.Itoa(cfg.QueueSize)},
			{section: sectionGit, label: "Branch Template", value: cfg.Git.BranchTemplate, help: "Used by jira start before branch creation. Supported tokens: {key}, {summary_slug}, {summary}."},
			{section: sectionClaude, label: "Enabled", value: strconv.FormatBool(cfg.Claude.Enabled), boolean: true},
			{section: sectionClaude, label: "Command", value: cfg.Claude.Command},
			{section: sectionClaude, label: "Timeout", value: cfg.Claude.Timeout.String()},
			{section: sectionClaude, label: "Ticket Plan", value: strconv.FormatBool(cfg.Claude.Features.TicketPlan), boolean: true},
			{section: sectionClaude, label: "Ticket Assist", value: strconv.FormatBool(cfg.Claude.Features.TicketAssist), boolean: true},
			{section: sectionClaude, label: "Clarifying Questions", value: strconv.FormatBool(cfg.Claude.Features.ClarifyingQuestions), boolean: true},
			{section: sectionClaude, label: "Draft Comment", value: strconv.FormatBool(cfg.Claude.Features.DraftComment), boolean: true},
			{section: sectionClaude, label: "Draft Ticket", value: strconv.FormatBool(cfg.Claude.Features.DraftTicket), boolean: true},
			{section: sectionClaude, label: "Branch Plan", value: strconv.FormatBool(cfg.Claude.Features.BranchPlan), boolean: true},
			{section: sectionClaude, label: "Code Changes", value: strconv.FormatBool(cfg.Claude.Features.CodeChanges), boolean: true},
			{section: sectionClaude, label: "PR Creation", value: strconv.FormatBool(cfg.Claude.Features.PRCreation), boolean: true},
			{section: sectionClaude, label: "PR Review Response", value: strconv.FormatBool(cfg.Claude.Features.PRReviewResponse), boolean: true},
			{section: sectionClaude, label: "Require Confirmation", value: strconv.FormatBool(cfg.Claude.Gates.RequireConfirmation), boolean: true},
			{section: sectionClaude, label: "Allow Jira Writes", value: strconv.FormatBool(cfg.Claude.Gates.AllowJiraWrites), boolean: true},
			{section: sectionClaude, label: "Allow Git Writes", value: strconv.FormatBool(cfg.Claude.Gates.AllowGitWrites), boolean: true},
			{section: sectionClaude, label: "Allow GitHub Writes", value: strconv.FormatBool(cfg.Claude.Gates.AllowGitHubWrites), boolean: true},
			{section: sectionClaude, label: "Allow Code Edits", value: strconv.FormatBool(cfg.Claude.Gates.AllowCodeEdits), boolean: true},
			{section: sectionNotifications, label: "Notifications Enabled", value: strconv.FormatBool(cfg.Notifications.Enabled), boolean: true},
			{section: sectionNotifications, label: "System Enabled", value: strconv.FormatBool(cfg.Notifications.SystemEnabled), boolean: true, help: "Uses cross-platform desktop notifications through beeep when ticket events arrive."},
			{section: sectionNotifications, label: "System On New", value: strconv.FormatBool(cfg.Notifications.SystemOnNew), boolean: true},
			{section: sectionNotifications, label: "System On Updates", value: strconv.FormatBool(cfg.Notifications.SystemOnUpdates), boolean: true},
			{section: sectionNotifications, label: "Auto Open Panel", value: strconv.FormatBool(cfg.Notifications.AutoOpenPanel), boolean: true},
			{section: sectionNotifications, label: "Keep Panel Open", value: strconv.FormatBool(cfg.Notifications.KeepPanelOpenUntilCleared), boolean: true, help: "Keeps the notification center visible until notifications are cleared."},
			{section: sectionNotifications, label: "Max Items", value: strconv.Itoa(cfg.Notifications.MaxItems)},
		},
		problems: problems,
		theme:    ui.NewTheme(cfg.Theme),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if m.editing {
			return m.updateEditor(msg)
		}
		return m.updateMenu(msg)
	case tea.PasteMsg:
		if m.editing {
			pasted := sanitizePastedText(msg.Content)
			var cmd tea.Cmd
			m.editor, cmd = m.editor.Update(tea.PasteMsg{Content: pasted})
			m.problems = nil
			m.err = nil
			m.testStatus = ""
			m.testDetails = ""
			if m.currentField().secret && pasted != "" {
				m.testStatus = "Secret pasted"
				m.testDetails = fmt.Sprintf("Added %d characters to %s.", len(pasted), m.currentField().label)
			}
			return m, cmd
		}
		return m, nil
	case connectionTestMsg:
		m.testing = false
		m.testStatus = msg.status
		m.testDetails = msg.details
		m.err = msg.err
		return m, nil
	}
	return m, nil
}

func (m Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m Model) Saved() bool {
	return m.saved
}

func (m Model) Cancelled() bool {
	return m.cancel
}

func (m Model) Config() (config.Config, error) {
	return m.configFromFields()
}

func (m Model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancel = true
		return m, tea.Quit
	case "up", "k":
		m.moveField(-1)
	case "down", "j":
		m.moveField(1)
	case "left", "shift+tab", "backtab", "h":
		m.switchSection(-1)
	case "right", "tab", "l":
		m.switchSection(1)
	case " ", "space":
		if m.currentField().boolean {
			m.toggleCurrentBool()
			return m, nil
		}
		if len(m.currentField().options) > 0 {
			m.cycleCurrentOption(1)
			return m, nil
		}
	case "enter":
		switch m.section {
		case sectionTest:
			return m.testConnection()
		case sectionSave:
			return m.save()
		case sectionQuit:
			m.cancel = true
			return m, tea.Quit
		default:
			if m.currentField().boolean {
				m.toggleCurrentBool()
				return m, nil
			}
			if len(m.currentField().options) > 0 {
				m.cycleCurrentOption(1)
				return m, nil
			}
			m.startEditingCurrentField()
		}
	case "s":
		return m.save()
	case "t":
		return m.testConnection()
	}
	return m, nil
}

func (m Model) updateEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancel = true
		return m, tea.Quit
	case "esc":
		m.editing = false
		m.editor = textinput.Model{}
	case "enter":
		m.setCurrentValue(m.editor.Value())
		m.editing = false
		m.editor = textinput.Model{}
		m.problems = nil
		m.err = nil
		m.testStatus = ""
		m.testDetails = ""
	default:
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) startEditingCurrentField() {
	m.editing = true
	m.editor = newConfigTextInput(m.currentField())
}

func newConfigTextInput(field field) textinput.Model {
	editor := textinput.New()
	editor.Prompt = ""
	editor.SetValue(field.value)
	editor.CursorEnd()
	if field.secret {
		editor.EchoMode = textinput.EchoPassword
	}
	editor.Focus()
	return editor
}

func (m *Model) moveField(delta int) {
	items := m.itemsForSection()
	if len(items) == 0 {
		m.selected = 0
		return
	}
	m.selected = clamp(m.selected+delta, 0, len(items)-1)
}

func (m *Model) switchSection(delta int) {
	m.section = clamp(m.section+delta, 0, len(sectionLabels)-1)
	m.selected = 0
}

func (m Model) save() (tea.Model, tea.Cmd) {
	cfg, err := m.configFromFields()
	if err != nil {
		m.err = err
		var validationErr config.ValidationError
		if errors.As(err, &validationErr) {
			m.problems = validationErr.Problems
		}
		return m, nil
	}
	if err := config.Save(m.path, cfg); err != nil {
		m.err = err
		var validationErr config.ValidationError
		if errors.As(err, &validationErr) {
			m.problems = validationErr.Problems
		}
		return m, nil
	}
	m.saved = true
	return m, tea.Quit
}

func (m Model) testConnection() (tea.Model, tea.Cmd) {
	cfg, err := m.configFromFields()
	if err != nil {
		m.err = err
		var validationErr config.ValidationError
		if errors.As(err, &validationErr) {
			m.problems = validationErr.Problems
			m.testStatus = "Local validation failed"
			m.testDetails = "Fix the highlighted config values before testing Jira."
		}
		return m, nil
	}

	m.testing = true
	m.err = nil
	m.problems = nil
	m.testStatus = "Testing Jira connection..."
	m.testDetails = fmt.Sprintf("Checking %s with project %s.", cfg.BaseURL, cfg.DefaultProject)

	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.RequestTimeout)
		defer cancel()

		issues, err := jira.NewClient(cfg).SearchIssues(ctx, cfg.DefaultJQL, 1)
		if err != nil {
			status, details := explainConnectionError(err, cfg)
			return connectionTestMsg{status: status, details: details, err: err}
		}

		details := fmt.Sprintf("Authenticated successfully. Query is valid for project %s.", cfg.DefaultProject)
		if len(issues) == 0 {
			details += " No matching issues were returned, which is okay."
		} else {
			details += " Jira returned at least one matching issue."
		}
		return connectionTestMsg{
			status:  "Jira connection verified",
			details: details,
		}
	}
}

func (m Model) render() string {
	width := m.width
	if width <= 0 {
		width = 100
	}

	var b strings.Builder
	b.WriteString(m.renderHeader(width))
	b.WriteString("\n\n")

	if ui.TerminalTooSmall(m.width, m.height) {
		b.WriteString(m.theme.ActivePane.Width(max(32, width-2)).Render(
			m.theme.PaneTitle.Render("Terminal Size") + "\n\n" + m.theme.Warning.Render(ui.TerminalSizeMessage(m.width, m.height)),
		))
		b.WriteString("\n\n")
		b.WriteString(m.renderFooterHelp(width))
		return b.String()
	}

	if len(m.problems) > 0 {
		b.WriteString(m.theme.Warning.Render("Needs attention"))
		b.WriteByte('\n')
		for _, problem := range m.problems {
			b.WriteString(m.theme.Error.Render("  - " + truncate(problem, width-4)))
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	} else if m.err != nil {
		b.WriteString(m.theme.Error.Render(truncate("Error: "+m.err.Error(), width)))
		b.WriteString("\n\n")
	}
	if m.testStatus != "" {
		style := m.theme.Muted
		switch {
		case strings.Contains(strings.ToLower(m.testStatus), "verified"):
			style = m.theme.Success
		case strings.Contains(strings.ToLower(m.testStatus), "failed"), m.err != nil:
			style = m.theme.Error
		case m.testing:
			style = m.theme.Warning
		}
		b.WriteString(style.Render(m.testStatus))
		if m.testDetails != "" {
			b.WriteString("\n")
			b.WriteString(m.theme.Muted.Render(truncate(m.testDetails, width)))
		}
		b.WriteString("\n\n")
	}

	layout := configLayout(width)
	menu := m.renderMenu(layout)
	fields := m.renderFields(layout)
	if !layout.stacked {
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, menu, "  ", fields))
	} else {
		b.WriteString(menu)
		b.WriteString("\n\n")
		b.WriteString(fields)
	}

	b.WriteString("\n\n")
	b.WriteString(m.renderFooterHelp(width))
	return b.String()
}

func (m Model) renderHeader(width int) string {
	contentWidth := max(32, width-2)
	status := "editing"
	if m.testing {
		status = "testing"
	} else if m.saved {
		status = "saved"
	}
	left := m.theme.Header.Render("Jira Config") + " " + m.theme.Subtitle.Render(status)
	right := m.theme.Muted.Render(truncate(m.path, max(20, contentWidth-lipgloss.Width(left)-2)))
	rightColumn := lipgloss.PlaceHorizontal(
		max(0, contentWidth-lipgloss.Width(left)-1),
		lipgloss.Right,
		right,
	)
	return lipgloss.NewStyle().Width(contentWidth).Render(left + " " + rightColumn)
}

func (m Model) renderFooterHelp(width int) string {
	available := max(20, width-2)
	rendered := m.footerContextLabel(available)
	currentGroup := ""
	for _, binding := range m.footerBindings() {
		next := m.theme.Key.Render(binding.key) + " " + m.theme.Muted.Render(binding.label)
		if currentGroup != "" && binding.group != currentGroup {
			next = m.theme.Muted.Render("|") + m.theme.Muted.Render("  ") + next
		}
		candidate := next
		if rendered != "" {
			candidate = rendered + m.theme.Muted.Render("  ") + next
		}
		if lipgloss.Width(candidate) > available {
			break
		}
		rendered = candidate
		currentGroup = binding.group
	}
	return rendered
}

func (m Model) footerContextLabel(width int) string {
	label := "Config"
	if m.editing {
		label = "Config Edit"
	}
	if label == "" || lipgloss.Width(label)+2 > width {
		return ""
	}
	return m.theme.Muted.Render(label)
}

func (m Model) footerBindings() []footerBinding {
	if m.editing {
		return []footerBinding{
			{key: "enter", label: "accept", group: "Editing"},
			{key: "esc", label: "cancel", group: "Editing"},
		}
	}
	return []footerBinding{
		{key: "left/right", label: "section", group: "Navigation"},
		{key: "tab", label: "section", group: "Navigation"},
		{key: "j/k", label: "field", group: "Navigation"},
		{key: "enter", label: "edit/select", group: "Editing"},
		{key: "t", label: "test", group: "Actions"},
		{key: "s", label: "save", group: "Actions"},
		{key: "q", label: "quit", group: "Global"},
	}
}

func (m Model) renderMenu(layout layout) string {
	var b strings.Builder
	for section := range sectionLabels {
		cursor := " "
		if section == m.section && (section == sectionSave || section == sectionQuit) {
			cursor = ">"
		}
		line := fmt.Sprintf("%s %s", cursor, sectionLabels[section])
		if section == m.section {
			line = m.theme.Selected.Render(line)
		} else if section == sectionSave {
			line = m.theme.Success.Render(line)
		} else if section == sectionQuit {
			line = m.theme.Muted.Render(line)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return m.theme.Panel.Width(layout.sectionPane).Render(m.theme.PaneTitle.Render("Sections") + " " + m.theme.Muted.Render("left/right") + "\n\n" + strings.TrimRight(b.String(), "\n"))
}

func (m Model) renderFields(layout layout) string {
	if m.section == sectionTest {
		body := "Run a live Jira check with the current account, token, base URL, project, and JQL before saving. Saving stores the token in the OS keychain."
		if m.testing {
			body = "Testing Jira now..."
		}
		if m.testDetails != "" {
			body += "\n\n" + m.testDetails
		}
		return m.theme.ActivePane.Width(layout.fieldPane).Render(m.theme.Warning.Render("Test Connection") + "\n\n" + body + "\n\nPress enter or t to run.")
	}
	if m.section == sectionSave {
		return m.theme.ActivePane.Width(layout.fieldPane).Render(m.theme.Success.Render("Save and Exit") + "\n\nPress enter or s to save settings. The Jira API token is stored in the OS keychain, not plaintext TOML. Saving runs local validation; use Test Connection for a live Jira check.")
	}
	if m.section == sectionQuit {
		return m.theme.ActivePane.Width(layout.fieldPane).Render(m.theme.Warning.Render("Quit Without Saving") + "\n\nPress enter or q to leave config unchanged.")
	}

	var b strings.Builder
	for index, field := range m.fieldsForSection(m.section) {
		prefix := " "
		if index == m.selected {
			prefix = ">"
		}
		value := displayValue(field)
		if m.editing && index == m.selected {
			editor := m.editor
			editor.SetWidth(max(1, layout.fieldPane-24))
			value = m.theme.Input.Render(editor.View())
		} else if field.section == sectionAppearance {
			value = renderColorSwatch(m.theme, value)
		} else if field.boolean {
			value = renderBoolPicker(m.theme, field.value)
		} else if len(field.options) > 0 {
			value = renderOptionPicker(m.theme, field.value, field.options)
		}

		label := m.theme.FieldLabel.Render(field.label + ":")
		line := fmt.Sprintf("%s %-18s %s", prefix, label, value)
		if index == m.selected {
			line = m.theme.Selected.Render(line)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}

	content := m.theme.PaneTitle.Render(sectionLabels[m.section]) + " " + m.theme.Muted.Render("j/k") + "\n\n" + strings.TrimRight(b.String(), "\n")
	if help := strings.TrimSpace(m.currentField().help); help != "" {
		content += "\n\n" + m.theme.Muted.Render(wrapConfigHelp(help, max(20, layout.fieldPane-6)))
	}
	return m.theme.ActivePane.Width(layout.fieldPane).Render(content)
}

func configLayout(width int) layout {
	if width <= 0 {
		width = 100
	}

	contentWidth := max(32, width-2)
	if width < 88 {
		return layout{
			stacked:     true,
			sectionPane: min(contentWidth, max(32, width-2)),
			fieldPane:   min(contentWidth, max(32, width-2)),
		}
	}

	if width < 120 {
		sectionWidth := 32
		return layout{
			sectionPane: sectionWidth,
			fieldPane:   max(42, width-sectionWidth-8),
		}
	}

	sectionWidth := clamp(width/4, 32, 40)
	return layout{
		sectionPane: sectionWidth,
		fieldPane:   min(88, max(56, width-sectionWidth-8)),
	}
}

func (m Model) configFromFields() (config.Config, error) {
	cfg := config.Defaults()
	for _, field := range m.fields {
		value := strings.TrimSpace(field.value)
		switch field.label {
		case "Active Profile":
			cfg.ActiveProfile = value
		case "Base URL":
			cfg.BaseURL = strings.TrimRight(value, "/")
		case "Email":
			cfg.Email = value
		case "API Token":
			cfg.APIToken = value
		case "Default Project":
			cfg.DefaultProject = value
		case "Default JQL":
			cfg.DefaultJQL = value
		case "Primary":
			cfg.Theme.Primary = value
		case "Secondary":
			cfg.Theme.Secondary = value
		case "Accent":
			cfg.Theme.Accent = value
		case "Success":
			cfg.Theme.Success = value
		case "Warning":
			cfg.Theme.Warning = value
		case "Error":
			cfg.Theme.Error = value
		case "Muted":
			cfg.Theme.Muted = value
		case "Border":
			cfg.Theme.Border = value
		case "Surface":
			cfg.Theme.Surface = value
		case "Text":
			cfg.Theme.Text = value
		case "Symbol Mode":
			cfg.Display.SymbolMode = strings.ToLower(value)
		case "Refresh Interval":
			duration, err := parseDuration(value)
			if err != nil {
				return config.Config{}, config.ValidationError{Problems: []string{"refresh interval must be a valid Go duration"}}
			}
			cfg.RefreshInterval = duration
		case "Request Timeout":
			duration, err := parseDuration(value)
			if err != nil {
				return config.Config{}, config.ValidationError{Problems: []string{"request timeout must be a valid Go duration"}}
			}
			cfg.RequestTimeout = duration
		case "Workers":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return config.Config{}, config.ValidationError{Problems: []string{"worker count must be an integer"}}
			}
			cfg.WorkerCount = parsed
		case "Queue Size":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return config.Config{}, config.ValidationError{Problems: []string{"queue size must be an integer"}}
			}
			cfg.QueueSize = parsed
		case "Branch Template":
			cfg.Git.BranchTemplate = value
		case "Enabled":
			parsed, err := parseBoolField("Claude enabled", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Enabled = parsed
		case "Command":
			cfg.Claude.Command = value
		case "Timeout":
			duration, err := parseDuration(value)
			if err != nil {
				return config.Config{}, config.ValidationError{Problems: []string{"Claude timeout must be a valid Go duration"}}
			}
			cfg.Claude.Timeout = duration
		case "Ticket Plan":
			parsed, err := parseBoolField("ticket plan", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Features.TicketPlan = parsed
		case "Ticket Assist":
			parsed, err := parseBoolField("ticket assist", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Features.TicketAssist = parsed
		case "Clarifying Questions":
			parsed, err := parseBoolField("clarifying questions", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Features.ClarifyingQuestions = parsed
		case "Draft Comment":
			parsed, err := parseBoolField("draft comment", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Features.DraftComment = parsed
		case "Draft Ticket":
			parsed, err := parseBoolField("draft ticket", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Features.DraftTicket = parsed
		case "Branch Plan":
			parsed, err := parseBoolField("branch plan", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Features.BranchPlan = parsed
		case "Code Changes":
			parsed, err := parseBoolField("code changes", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Features.CodeChanges = parsed
		case "PR Creation":
			parsed, err := parseBoolField("PR creation", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Features.PRCreation = parsed
		case "PR Review Response":
			parsed, err := parseBoolField("PR review response", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Features.PRReviewResponse = parsed
		case "Require Confirmation":
			parsed, err := parseBoolField("require confirmation", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Gates.RequireConfirmation = parsed
		case "Allow Jira Writes":
			parsed, err := parseBoolField("allow Jira writes", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Gates.AllowJiraWrites = parsed
		case "Allow Git Writes":
			parsed, err := parseBoolField("allow git writes", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Gates.AllowGitWrites = parsed
		case "Allow GitHub Writes":
			parsed, err := parseBoolField("allow GitHub writes", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Gates.AllowGitHubWrites = parsed
		case "Allow Code Edits":
			parsed, err := parseBoolField("allow code edits", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Claude.Gates.AllowCodeEdits = parsed
		case "Notifications Enabled":
			parsed, err := parseBoolField("notifications enabled", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Notifications.Enabled = parsed
		case "System Enabled":
			parsed, err := parseBoolField("system notifications enabled", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Notifications.SystemEnabled = parsed
		case "System On New":
			parsed, err := parseBoolField("system notifications on new tickets", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Notifications.SystemOnNew = parsed
		case "System On Updates":
			parsed, err := parseBoolField("system notifications on ticket updates", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Notifications.SystemOnUpdates = parsed
		case "Auto Open Panel":
			parsed, err := parseBoolField("auto open notification panel", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Notifications.AutoOpenPanel = parsed
		case "Keep Panel Open":
			parsed, err := parseBoolField("keep notification panel open", value)
			if err != nil {
				return config.Config{}, err
			}
			cfg.Notifications.KeepPanelOpenUntilCleared = parsed
		case "Max Items":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return config.Config{}, config.ValidationError{Problems: []string{"notification max items must be an integer"}}
			}
			cfg.Notifications.MaxItems = parsed
		}
	}
	if cfg.DefaultJQL == "" || cfg.DefaultJQL == config.Defaults().DefaultJQL {
		cfg.DefaultJQL = config.DefaultJQLForProject(cfg.DefaultProject)
	}
	cfg.Views = config.DefaultViews(cfg.DefaultProject)
	if len(cfg.Views) > 0 {
		cfg.ActiveView = cfg.Views[0].Name
	}
	return cfg, config.Validate(cfg)
}

func (m Model) itemsForSection() []field {
	return m.fieldsForSection(m.section)
}

func (m Model) fieldsForSection(section int) []field {
	var fields []field
	for _, field := range m.fields {
		if field.section == section {
			fields = append(fields, field)
		}
	}
	return fields
}

func (m Model) currentField() field {
	return m.fieldsForSection(m.section)[m.selected]
}

func (m *Model) setCurrentValue(value string) {
	index := 0
	for i, field := range m.fields {
		if field.section != m.section {
			continue
		}
		if index == m.selected {
			m.fields[i].value = value
			return
		}
		index++
	}
}

func (m *Model) toggleCurrentBool() {
	current := strings.EqualFold(strings.TrimSpace(m.currentField().value), "true")
	m.setCurrentValue(strconv.FormatBool(!current))
}

func (m *Model) cycleCurrentOption(delta int) {
	field := m.currentField()
	if len(field.options) == 0 {
		return
	}
	current := strings.ToLower(strings.TrimSpace(field.value))
	selected := 0
	for index, option := range field.options {
		if strings.EqualFold(option, current) {
			selected = index
			break
		}
	}
	selected = (selected + delta + len(field.options)) % len(field.options)
	m.setCurrentValue(field.options[selected])
}

func displayValue(field field) string {
	if field.secret && field.value != "" {
		return strings.Repeat("*", len(field.value))
	}
	return field.value
}

func renderColorSwatch(theme ui.Theme, value string) string {
	style := lipgloss.NewStyle().Background(lipgloss.Color(value)).Foreground(lipgloss.Color(value))
	return style.Render("  ") + " " + theme.Text.Render(value)
}

func renderBoolPicker(theme ui.Theme, value string) string {
	enabled := strings.EqualFold(strings.TrimSpace(value), "true")
	falseValue := theme.Text.Render("false")
	trueValue := theme.Text.Render("true")
	if enabled {
		trueValue = theme.Selected.Render("true")
	} else {
		falseValue = theme.Selected.Render("false")
	}
	return falseValue + theme.Muted.Render(" / ") + trueValue
}

func renderOptionPicker(theme ui.Theme, value string, options []string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	rendered := make([]string, 0, len(options))
	for _, option := range options {
		text := option
		if strings.EqualFold(option, value) {
			text = theme.Selected.Render(option)
		} else {
			text = theme.Text.Render(option)
		}
		rendered = append(rendered, text)
	}
	return strings.Join(rendered, theme.Muted.Render(" / "))
}

func wrapConfigText(value string, width int) string {
	if width <= 0 {
		return value
	}
	words := strings.Fields(value)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if lipgloss.Width(line)+1+lipgloss.Width(word) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		line += " " + word
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}

func wrapConfigHelp(value string, width int) string {
	parts := strings.Split(value, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		lines = append(lines, wrapConfigText(part, width))
	}
	return strings.Join(lines, "\n")
}

func sanitizePastedText(value string) string {
	value = strings.TrimSpace(value)
	return strings.Map(func(char rune) rune {
		if char == '\n' || char == '\r' || char == '\t' {
			return -1
		}
		if char < 0x20 || char == 0x7f {
			return -1
		}
		return char
	}, value)
}

func explainConnectionError(err error, cfg config.Config) (string, string) {
	message := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, context.DeadlineExceeded), strings.Contains(message, "timeout"), strings.Contains(message, "deadline exceeded"):
		return "Jira connection timed out", fmt.Sprintf("No response from %s within %s. Check VPN/network access or increase request_timeout.", cfg.BaseURL, cfg.RequestTimeout)
	case strings.Contains(message, "no such host"), strings.Contains(message, "connection refused"), strings.Contains(message, "tls"), strings.Contains(message, "certificate"), strings.Contains(message, "server misbehaving"):
		return "Jira base URL or network check failed", fmt.Sprintf("Could not reach %s. Confirm the Jira base URL, DNS/VPN access, and TLS certificate.", cfg.BaseURL)
	case strings.Contains(message, "401"), strings.Contains(message, "unauthorized"), strings.Contains(message, "authentication"):
		return "Jira authentication failed", "The email/API token pair was rejected. Confirm the account email and generate a fresh Jira API token if needed."
	case strings.Contains(message, "403"), strings.Contains(message, "forbidden"):
		return "Jira permission check failed", fmt.Sprintf("The credentials worked, but Jira denied this query. Confirm the account can browse project %s.", cfg.DefaultProject)
	case strings.Contains(message, "400"), strings.Contains(message, "jql"), strings.Contains(message, "project"):
		return "Jira query or project check failed", fmt.Sprintf("Jira rejected the configured JQL. Confirm project %s exists and the query is valid: %s", cfg.DefaultProject, cfg.DefaultJQL)
	default:
		return "Jira connection test failed", fmt.Sprintf("Jira returned an unexpected error while testing %s. Raw error: %v", cfg.BaseURL, err)
	}
}

func parseDuration(value string) (time.Duration, error) {
	return time.ParseDuration(value)
}

func parseBoolField(name string, value string) (bool, error) {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, config.ValidationError{Problems: []string{name + " must be true or false"}}
	}
	return parsed, nil
}

func truncate(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 3 {
		return value[:width]
	}
	return value[:width-3] + "..."
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
