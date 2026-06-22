package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

type textSelection struct {
	active bool
	anchor int
	cursor int
}

func (s *textSelection) Mark(editor textarea.Model) {
	offset := textareaCursorOffset(editor)
	s.active = true
	s.anchor = offset
	s.cursor = offset
}

func (s *textSelection) UpdateCursor(editor textarea.Model) {
	if !s.active {
		return
	}
	s.cursor = textareaCursorOffset(editor)
}

func (s *textSelection) Clear() {
	*s = textSelection{}
}

func (s textSelection) Range(editor textarea.Model) (int, int, bool) {
	if !s.active || s.anchor == s.cursor {
		return 0, 0, false
	}
	value := []rune(editor.Value())
	start := clamp(min(s.anchor, s.cursor), 0, len(value))
	end := clamp(max(s.anchor, s.cursor), 0, len(value))
	return start, end, start < end
}

func (s textSelection) SelectedText(editor textarea.Model) string {
	start, end, ok := s.Range(editor)
	if !ok {
		return ""
	}
	return string([]rune(editor.Value())[start:end])
}

func deleteTextareaSelection(editor *textarea.Model, selection *textSelection) bool {
	start, end, ok := selection.Range(*editor)
	if !ok {
		return false
	}
	value := []rune(editor.Value())
	next := string(append(append([]rune{}, value[:start]...), value[end:]...))
	setTextareaValueAndCursor(editor, next, start)
	selection.Clear()
	return true
}

func replaceTextareaSelection(editor *textarea.Model, selection *textSelection, replacement string) bool {
	start, end, ok := selection.Range(*editor)
	if !ok {
		return false
	}
	value := []rune(editor.Value())
	insert := []rune(replacement)
	next := make([]rune, 0, len(value)-(end-start)+len(insert))
	next = append(next, value[:start]...)
	next = append(next, insert...)
	next = append(next, value[end:]...)
	setTextareaValueAndCursor(editor, string(next), start+len(insert))
	selection.Clear()
	return true
}

func textareaCursorOffset(editor textarea.Model) int {
	value := editor.Value()
	lines := strings.Split(value, "\n")
	if len(lines) == 0 {
		return 0
	}
	row := clamp(editor.Line(), 0, len(lines)-1)
	offset := 0
	for index := 0; index < row; index++ {
		offset += len([]rune(lines[index])) + 1
	}
	column := clamp(editor.LineInfo().CharOffset, 0, len([]rune(lines[row])))
	return offset + column
}

func setTextareaValueAndCursor(editor *textarea.Model, value string, offset int) {
	offset = clamp(offset, 0, len([]rune(value)))
	editor.SetValue(value)
	editor.MoveToBegin()
	for range offset {
		next, _ := editor.Update(tea.KeyPressMsg(tea.Key{Text: "right", Code: tea.KeyRight}))
		*editor = next
	}
	editor.Focus()
}

func textareaSelectionExtendingKey(key string) bool {
	switch key {
	case "left", "right", "up", "down", "home", "end", "ctrl+a", "ctrl+e", "alt+left", "alt+right", "ctrl+b", "ctrl+f", "ctrl+p", "ctrl+n", "pgup", "pgdown":
		return true
	default:
		return false
	}
}

func updateTextareaShiftSelection(editor *textarea.Model, selection *textSelection, key string) bool {
	movement, ok := shiftSelectionMovementKey(key)
	if !ok {
		return false
	}
	if !selection.active {
		selection.Mark(*editor)
	}
	next, _ := editor.Update(tea.KeyPressMsg(tea.Key{Text: movement, Code: movementKeyCode(movement)}))
	*editor = next
	selection.UpdateCursor(*editor)
	return true
}

func shiftSelectionMovementKey(key string) (string, bool) {
	switch key {
	case "shift+left":
		return "left", true
	case "shift+right":
		return "right", true
	case "shift+up":
		return "up", true
	case "shift+down":
		return "down", true
	default:
		return "", false
	}
}

func movementKeyCode(key string) rune {
	switch key {
	case "left":
		return tea.KeyLeft
	case "right":
		return tea.KeyRight
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	default:
		return 0
	}
}
