package tui

import "charm.land/bubbles/v2/textinput"

func newUserSearchInput(value string) textinput.Model {
	editor := textinput.New()
	editor.Prompt = ""
	editor.SetValue(value)
	editor.CursorEnd()
	editor.Focus()
	return editor
}
