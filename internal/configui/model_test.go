package configui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/jon/jira-tui/internal/config"
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
