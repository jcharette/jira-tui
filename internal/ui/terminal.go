package ui

import "fmt"

const (
	MinTerminalWidth          = 88
	MinTerminalHeight         = 24
	RecommendedTerminalWidth  = 120
	RecommendedTerminalHeight = 30
)

func TerminalTooSmall(width, height int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	return width < MinTerminalWidth || height < MinTerminalHeight
}

func TerminalSizeMessage(width, height int) string {
	return fmt.Sprintf(
		"Terminal too small: %dx%d. Jira needs at least %dx%d; %dx%d is recommended.",
		width,
		height,
		MinTerminalWidth,
		MinTerminalHeight,
		RecommendedTerminalWidth,
		RecommendedTerminalHeight,
	)
}
