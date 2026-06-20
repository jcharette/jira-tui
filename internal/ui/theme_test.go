package ui

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/jcharette/jira-tui/internal/config"
)

func TestNewThemeKeepsInlineStylesForegroundOnly(t *testing.T) {
	cfg, _, ok := config.BuiltInTheme("ops")
	if !ok {
		t.Fatal("missing ops theme")
	}

	theme := NewTheme(cfg)

	for name, style := range map[string]lipgloss.Style{
		"Header":      theme.Header,
		"Subtitle":    theme.Subtitle,
		"PaneTitle":   theme.PaneTitle,
		"Selected":    theme.Selected,
		"TabInactive": theme.TabInactive,
		"Key":         theme.Key,
		"Muted":       theme.Muted,
		"Success":     theme.Success,
		"Warning":     theme.Warning,
		"Error":       theme.Error,
		"Text":        theme.Text,
		"FieldLabel":  theme.FieldLabel,
	} {
		if got := style.GetBackground(); fmt.Sprint(got) != "{}" {
			t.Fatalf("%s background = %q, want none", name, got)
		}
	}
}

func TestNewThemeAppliesSkinBackgroundsToBlockStyles(t *testing.T) {
	cfg, _, ok := config.BuiltInTheme("ops")
	if !ok {
		t.Fatal("missing ops theme")
	}

	theme := NewTheme(cfg)
	surface := lipgloss.Color(cfg.Surface)
	border := lipgloss.Color(cfg.Border)

	for name, style := range map[string]lipgloss.Style{
		"Panel":        theme.Panel,
		"ActivePane":   theme.ActivePane,
		"CodeBlock":    theme.CodeBlock,
		"CommentBlock": theme.CommentBlock,
		"NoticeBlock":  theme.NoticeBlock,
	} {
		if got := style.GetBackground(); got != surface {
			t.Fatalf("%s background = %q, want %q", name, got, surface)
		}
	}
	for name, style := range map[string]lipgloss.Style{
		"Code":  theme.Code,
		"Input": theme.Input,
	} {
		if got := style.GetBackground(); got != border {
			t.Fatalf("%s background = %q, want %q", name, got, border)
		}
	}
}
