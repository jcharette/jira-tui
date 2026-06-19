package configui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/jcharette/jira-tui/internal/config"
)

func TestRenderShowsTerminalSizeWarningBelowMinimum(t *testing.T) {
	model := NewModel("/tmp/jira.toml", config.Config{}, nil)
	model.width = 87
	model.height = 23

	view := model.render()

	for _, want := range []string{"Terminal Size", "Terminal too small: 87x23", "at least 88x24", "120x30 is recommended"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "Jira Account") {
		t.Fatalf("small terminal warning should skip config fields: %q", view)
	}
}

func TestConfigDisplaySymbolModeGoldenSnapshot(t *testing.T) {
	cfg := config.Defaults()
	cfg.Display.SymbolMode = "auto"
	model := NewModel("/tmp/jira.toml", cfg, nil)
	model.width = 120
	model.height = 30
	model.section = sectionDisplay
	model.selected = fieldIndexForTest(model, sectionDisplay, "Symbol Mode")

	assertConfigGoldenSnapshot(t, "config_symbol_mode.golden", model.render())
}

func TestFooterHelpUsesContextAndGroupsCommands(t *testing.T) {
	model := NewModel("/tmp/jira.toml", config.Defaults(), nil)

	footer := model.renderFooterHelp(120)

	for _, want := range []string{"Config", "left/right section", "tab section", "j/k field", "enter edit/select", "t test", "s save", "q quit", "|"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("missing %q in %q", want, footer)
		}
	}
	if lipgloss.Width(footer) > 118 {
		t.Fatalf("footer width = %d, want <= 118: %q", lipgloss.Width(footer), footer)
	}
}

var configAnsiEscapeForTest = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func assertConfigGoldenSnapshot(t *testing.T, name string, rendered string) {
	t.Helper()
	got := normalizeConfigSnapshotForTest(rendered)
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden snapshot %s: %v\n\nrendered:\n%s", path, err, got)
	}
	want := strings.TrimRight(string(data), "\n")
	if got != want {
		t.Fatalf("golden snapshot %s mismatch\n\nwant:\n%s\n\ngot:\n%s", path, want, got)
	}
}

func normalizeConfigSnapshotForTest(rendered string) string {
	rendered = configAnsiEscapeForTest.ReplaceAllString(rendered, "")
	rendered = strings.ReplaceAll(rendered, "\r\n", "\n")
	rendered = strings.ReplaceAll(rendered, "\r", "\n")
	lines := strings.Split(rendered, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func TestFooterHelpUsesEditContext(t *testing.T) {
	model := NewModel("/tmp/jira.toml", config.Defaults(), nil)
	model.editing = true

	footer := model.renderFooterHelp(80)

	for _, want := range []string{"Config Edit", "enter accept", "esc cancel"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("missing %q in %q", want, footer)
		}
	}
	if strings.Contains(footer, "q quit") {
		t.Fatalf("edit footer should not show normal mode commands: %q", footer)
	}
}

func TestHeaderUsesAvailableWidth(t *testing.T) {
	model := NewModel("/tmp/jira.toml", config.Defaults(), nil)

	header := model.renderHeader(72)

	for _, want := range []string{"Jira Config", "editing", "/tmp/jira.toml"} {
		if !strings.Contains(header, want) {
			t.Fatalf("missing %q in %q", want, header)
		}
	}
	if lipgloss.Width(header) != 70 {
		t.Fatalf("header width = %d, want 70: %q", lipgloss.Width(header), header)
	}
}

func TestRenderShowsClaudeSection(t *testing.T) {
	cfg := config.Defaults()
	cfg.Claude.Enabled = true
	cfg.Claude.Command = "/opt/homebrew/bin/claude"
	cfg.Claude.Features.TicketPlan = true
	cfg.Claude.Features.TicketAssist = true
	cfg.Claude.Gates.AllowGitWrites = true
	model := NewModel("/tmp/jira.toml", cfg, nil)
	model.width = 120
	model.height = 30
	model.section = sectionClaude

	view := model.render()

	for _, want := range []string{"Claude", "Enabled", "/opt/homebrew/bin/claude", "Ticket Plan", "Ticket Assist", "Allow Git Writes"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestConfigFromFieldsIncludesClaudeSettings(t *testing.T) {
	cfg := config.Defaults()
	cfg.ActiveProfile = "work"
	cfg.BaseURL = "https://example.atlassian.net"
	cfg.Email = "person@example.com"
	cfg.APIToken = "secret"
	cfg.DefaultProject = "ABC"
	cfg.DefaultJQL = config.DefaultJQLForProject("ABC")
	cfg.Views = config.DefaultViews("ABC")
	cfg.ActiveView = cfg.Views[0].Name
	model := NewModel("/tmp/jira.toml", cfg, nil)
	setFieldForTest(&model, "Enabled", "true")
	setFieldForTest(&model, "Command", "/usr/local/bin/claude")
	setFieldForTest(&model, "Timeout", "45s")
	setFieldForTest(&model, "Ticket Plan", "true")
	setFieldForTest(&model, "Ticket Assist", "true")
	setFieldForTest(&model, "Clarifying Questions", "true")
	setFieldForTest(&model, "Allow Git Writes", "true")

	cfg, err := model.Config()
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}

	if !cfg.Claude.Enabled || cfg.Claude.Command != "/usr/local/bin/claude" {
		t.Fatalf("Claude = %#v", cfg.Claude)
	}
	if !cfg.Claude.Features.TicketPlan || !cfg.Claude.Features.TicketAssist || !cfg.Claude.Features.ClarifyingQuestions {
		t.Fatalf("Claude.Features = %#v", cfg.Claude.Features)
	}
	if !cfg.Claude.Gates.RequireConfirmation || !cfg.Claude.Gates.AllowGitWrites {
		t.Fatalf("Claude.Gates = %#v", cfg.Claude.Gates)
	}
	if cfg.ActiveProfile != "work" {
		t.Fatalf("ActiveProfile = %q", cfg.ActiveProfile)
	}
}

func TestConfigFromFieldsIncludesEditedActiveProfile(t *testing.T) {
	cfg := config.Defaults()
	cfg.ActiveProfile = "default"
	cfg.BaseURL = "https://example.atlassian.net"
	cfg.Email = "person@example.com"
	cfg.APIToken = "secret"
	cfg.DefaultProject = "ABC"
	cfg.DefaultJQL = config.DefaultJQLForProject("ABC")
	cfg.Views = config.DefaultViews("ABC")
	cfg.ActiveView = cfg.Views[0].Name
	model := NewModel("/tmp/jira.toml", cfg, nil)
	setFieldForTest(&model, "Active Profile", "work")

	next, err := model.Config()
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}

	if next.ActiveProfile != "work" {
		t.Fatalf("ActiveProfile = %q", next.ActiveProfile)
	}
	if value := fieldValueForTest(model, "Active Profile"); value != "work" {
		t.Fatalf("Active Profile field = %q", value)
	}
}

func TestBooleanFieldsToggleWithoutTextEditing(t *testing.T) {
	model := NewModel("/tmp/jira.toml", config.Defaults(), nil)
	model.section = sectionClaude
	model.selected = fieldIndexForTest(model, sectionClaude, "Enabled")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if next.editing {
		t.Fatal("boolean field should toggle without entering text edit mode")
	}
	if value := fieldValueForTest(next, "Enabled"); value != "true" {
		t.Fatalf("Enabled = %q", value)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "left", Code: tea.KeyLeft}))
	next = updated.(Model)
	if value := fieldValueForTest(next, "Enabled"); value != "false" {
		t.Fatalf("Enabled after left = %q", value)
	}

	view := next.render()
	if !strings.Contains(view, "false") || !strings.Contains(view, "true") {
		t.Fatalf("boolean field should render true/false options in %q", view)
	}
}

func TestSymbolModeFieldCyclesOptionsWithoutTextEditing(t *testing.T) {
	cfg := config.Defaults()
	cfg.Display.SymbolMode = "auto"
	model := NewModel("/tmp/jira.toml", cfg, nil)
	model.section = sectionDisplay
	model.selected = fieldIndexForTest(model, sectionDisplay, "Symbol Mode")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if next.editing {
		t.Fatal("symbol mode should cycle options without entering text edit mode")
	}
	if value := fieldValueForTest(next, "Symbol Mode"); value != "symbols" {
		t.Fatalf("Symbol Mode after enter = %q, want symbols", value)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "left", Code: tea.KeyLeft}))
	next = updated.(Model)
	if value := fieldValueForTest(next, "Symbol Mode"); value != "auto" {
		t.Fatalf("Symbol Mode after left = %q, want auto", value)
	}
}

func TestSymbolModeHelpShowsNerdFontSetupCommand(t *testing.T) {
	model := NewModel("/tmp/jira.toml", config.Defaults(), nil)
	model.width = 120
	model.height = 30
	model.section = sectionDisplay
	model.selected = fieldIndexForTest(model, sectionDisplay, "Symbol Mode")

	view := model.render()

	for _, want := range []string{
		"Auto detects Nerd-capable iTerm profiles",
		"brew install --cask font-jetbrains-mono-nerd-font",
		"set your terminal profile font to JetBrainsMono Nerd Font",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestScalarFieldEditingUsesCursorAwareTextInput(t *testing.T) {
	cfg := config.Defaults()
	cfg.DefaultProject = "ABC"
	model := NewModel("/tmp/jira.toml", cfg, nil)
	model.section = sectionQueries
	model.selected = fieldIndexForTest(model, sectionQueries, "Default Project")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.editing {
		t.Fatal("expected scalar field to enter edit mode")
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "left", Code: tea.KeyLeft}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x'}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if next.editing {
		t.Fatal("expected enter to accept scalar edit")
	}
	if value := fieldValueForTest(next, "Default Project"); value != "ABxC" {
		t.Fatalf("Default Project = %q, want cursor-aware insert into ABxC", value)
	}
}

func setFieldForTest(model *Model, label string, value string) {
	for index, field := range model.fields {
		if field.label == label {
			model.fields[index].value = value
			return
		}
	}
}

func fieldValueForTest(model Model, label string) string {
	for _, field := range model.fields {
		if field.label == label {
			return field.value
		}
	}
	return ""
}

func fieldIndexForTest(model Model, section int, label string) int {
	index := 0
	for _, field := range model.fields {
		if field.section != section {
			continue
		}
		if field.label == label {
			return index
		}
		index++
	}
	return 0
}
