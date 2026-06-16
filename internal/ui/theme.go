package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jon/jira-tui/internal/config"
)

type Theme struct {
	Header       lipgloss.Style
	Subtitle     lipgloss.Style
	Panel        lipgloss.Style
	ActivePane   lipgloss.Style
	PaneTitle    lipgloss.Style
	Selected     lipgloss.Style
	TabActive    lipgloss.Style
	TabInactive  lipgloss.Style
	Key          lipgloss.Style
	Muted        lipgloss.Style
	Success      lipgloss.Style
	Warning      lipgloss.Style
	Error        lipgloss.Style
	Text         lipgloss.Style
	FieldLabel   lipgloss.Style
	Code         lipgloss.Style
	CodeBlock    lipgloss.Style
	CommentBlock lipgloss.Style
	NoticeBlock  lipgloss.Style
	Input        lipgloss.Style
}

func NewTheme(cfg config.Theme) Theme {
	primary := lipgloss.Color(cfg.Primary)
	secondary := lipgloss.Color(cfg.Secondary)
	accent := lipgloss.Color(cfg.Accent)
	success := lipgloss.Color(cfg.Success)
	warning := lipgloss.Color(cfg.Warning)
	errorColor := lipgloss.Color(cfg.Error)
	muted := lipgloss.Color(cfg.Muted)
	border := lipgloss.Color(cfg.Border)
	surface := lipgloss.Color(cfg.Surface)
	text := lipgloss.Color(cfg.Text)

	return Theme{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(primary).
			Padding(0, 1),
		Subtitle: lipgloss.NewStyle().
			Foreground(muted),
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Foreground(text).
			Background(surface).
			Padding(1, 2),
		ActivePane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primary).
			Foreground(text).
			Background(surface).
			Padding(1, 2),
		PaneTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(secondary),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent),
		TabActive: lipgloss.NewStyle().
			Bold(true).
			Underline(true).
			Foreground(accent).
			Background(border).
			Padding(0, 1),
		TabInactive: lipgloss.NewStyle().
			Foreground(muted).
			Padding(0, 1),
		Key: lipgloss.NewStyle().
			Bold(true).
			Foreground(primary),
		Muted: lipgloss.NewStyle().
			Foreground(muted),
		Success: lipgloss.NewStyle().
			Foreground(success),
		Warning: lipgloss.NewStyle().
			Foreground(warning),
		Error: lipgloss.NewStyle().
			Bold(true).
			Foreground(errorColor),
		Text: lipgloss.NewStyle().
			Foreground(text),
		FieldLabel: lipgloss.NewStyle().
			Bold(true).
			Foreground(secondary),
		Code: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			Background(lipgloss.Color("#1F2937")),
		CodeBlock: lipgloss.NewStyle().
			Foreground(text).
			Background(lipgloss.Color("#0B1220")).
			Padding(0, 1),
		CommentBlock: lipgloss.NewStyle().
			Foreground(text).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(border).
			Padding(0, 1),
		NoticeBlock: lipgloss.NewStyle().
			Foreground(text).
			Background(lipgloss.Color("#1F2937")).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(accent).
			Padding(0, 1),
		Input: lipgloss.NewStyle().
			Foreground(text).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1),
	}
}

func Help(theme Theme, bindings ...string) string {
	var parts []string
	for _, binding := range bindings {
		key, label, ok := strings.Cut(binding, " ")
		if !ok {
			parts = append(parts, theme.Key.Render(binding))
			continue
		}
		parts = append(parts, theme.Key.Render(key)+" "+theme.Muted.Render(label))
	}
	return strings.Join(parts, theme.Muted.Render("  "))
}

func PanelWidth(width int) int {
	if width <= 0 {
		return 96
	}
	if width < 48 {
		return max(28, width-2)
	}
	return min(width-2, 110)
}
