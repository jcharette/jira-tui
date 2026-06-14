package tui

import "strings"

type keyContext string

const (
	keyContextTable          keyContext = "Issue Table"
	keyContextDetail         keyContext = "Ticket Detail"
	keyContextLinks          keyContext = "Links"
	keyContextHierarchy      keyContext = "Hierarchy"
	keyContextActions        keyContext = "Actions"
	keyContextComment        keyContext = "Add Comment"
	keyContextMentionPicker  keyContext = "Mention Picker"
	keyContextCommentConfirm keyContext = "Review Comment"
	keyContextHelp           keyContext = "Help"
)

type keyBinding struct {
	Keys        []string
	FooterKey   string
	Label       string
	Description string
	Group       string
	Footer      bool
}

func (b keyBinding) keyText() string {
	if b.FooterKey != "" {
		return b.FooterKey
	}
	return strings.Join(b.Keys, ", ")
}

func (b keyBinding) footerText() string {
	return b.keyText() + " " + b.Label
}

func activeKeyContext(m Model) keyContext {
	switch {
	case m.mode == modeComment && m.mentionPickerOpen:
		return keyContextMentionPicker
	case m.mode == modeComment && m.commentConfirm:
		return keyContextCommentConfirm
	case m.mode == modeComment:
		return keyContextComment
	case m.mode == modeDetail && m.linkFocus:
		return keyContextLinks
	case m.mode == modeDetail && m.actionFocus:
		return keyContextActions
	case m.mode == modeDetail && m.hierarchyFocus:
		return keyContextHierarchy
	case m.mode == modeDetail:
		return keyContextDetail
	default:
		return keyContextTable
	}
}

func footerBindings(context keyContext) []keyBinding {
	var bindings []keyBinding
	for _, binding := range keyBindings(context) {
		if binding.Footer {
			bindings = append(bindings, binding)
		}
	}
	return bindings
}

func keyBindings(context keyContext) []keyBinding {
	bindings := append([]keyBinding{}, globalBindings(context)...)
	switch context {
	case keyContextTable:
		bindings = append(bindings, tableBindings()...)
	case keyContextDetail:
		bindings = append(bindings, detailBindings()...)
	case keyContextLinks:
		bindings = append(bindings, linkBindings()...)
	case keyContextHierarchy:
		bindings = append(bindings, hierarchyBindings()...)
	case keyContextActions:
		bindings = append(bindings, actionBindings()...)
	case keyContextComment:
		bindings = append(bindings, commentBindings()...)
	case keyContextMentionPicker:
		bindings = append(bindings, mentionPickerBindings()...)
	case keyContextCommentConfirm:
		bindings = append(bindings, commentConfirmBindings()...)
	case keyContextHelp:
		bindings = append(bindings, helpBindings()...)
	}
	return bindings
}

func globalBindings(context keyContext) []keyBinding {
	if context == keyContextHelp {
		return nil
	}
	if context == keyContextComment || context == keyContextMentionPicker || context == keyContextCommentConfirm {
		return []keyBinding{
			{Keys: []string{"?"}, Label: "help", Description: "Open the keyboard help screen.", Group: "Global", Footer: true},
			{Keys: []string{"ctrl+c"}, Label: "quit", Description: "Quit Jira.", Group: "Global"},
		}
	}
	return []keyBinding{
		{Keys: []string{"?"}, Label: "help", Description: "Open the keyboard help screen.", Group: "Global", Footer: true},
		{Keys: []string{"q", "ctrl+c"}, FooterKey: "q", Label: "quit", Description: "Quit Jira.", Group: "Global"},
	}
}

func tableBindings() []keyBinding {
	return []keyBinding{
		{Keys: []string{"j", "k", "up", "down"}, FooterKey: "j/k", Label: "move", Description: "Move the selected issue.", Group: "Navigation", Footer: true},
		{Keys: []string{"g", "G", "home", "end"}, FooterKey: "g/G", Label: "first/last", Description: "Jump to the first or last issue.", Group: "Navigation"},
		{Keys: []string{"enter"}, Label: "open", Description: "Open focused ticket detail.", Group: "Issue", Footer: true},
		{Keys: []string{"x"}, Label: "expand-open", Description: "Load open child issues for the selected parent.", Group: "Issue", Footer: true},
		{Keys: []string{"X"}, Label: "expand-all", Description: "Load all child issues for the selected parent, including resolved issues.", Group: "Issue"},
		{Keys: []string{"r"}, Label: "refresh", Description: "Refresh the active issue view.", Group: "Global", Footer: true},
		{Keys: []string{"tab", "]", "shift+tab", "["}, FooterKey: "tab", Label: "view", Description: "Switch saved issue views.", Group: "Views", Footer: true},
		{Keys: []string{"o", "O"}, Label: "sort", Description: "Cycle issue table sorting forward or backward.", Group: "Views", Footer: true},
		{Keys: []string{"pgup", "pgdn", "space", "ctrl+b", "ctrl+f"}, FooterKey: "pgup/pgdn", Label: "page", Description: "Page through the issue table.", Group: "Navigation", Footer: true},
	}
}

func detailBindings() []keyBinding {
	return []keyBinding{
		{Keys: []string{"esc"}, Label: "back", Description: "Return to the issue table.", Group: "Navigation", Footer: true},
		{Keys: []string{"j", "k", "up", "down"}, FooterKey: "j/k", Label: "scroll", Description: "Scroll ticket detail content.", Group: "Navigation", Footer: true},
		{Keys: []string{"pgup", "pgdn", "space", "ctrl+b", "ctrl+f"}, FooterKey: "pgup/pgdn", Label: "page", Description: "Page through ticket detail content.", Group: "Navigation"},
		{Keys: []string{"g", "G", "home", "end"}, FooterKey: "g/G", Label: "top/bottom", Description: "Jump to the top or bottom of ticket detail.", Group: "Navigation"},
		{Keys: []string{"tab", "shift+tab"}, FooterKey: "tab", Label: "section", Description: "Move focus across ticket detail sections.", Group: "Sections", Footer: true},
		{Keys: []string{"enter"}, Label: "select", Description: "Jump to or activate the focused ticket detail section.", Group: "Sections"},
		{Keys: []string{"n", "p"}, FooterKey: "n/p", Label: "section", Description: "Jump to the next or previous ticket detail section.", Group: "Sections"},
		{Keys: []string{"d"}, Label: "description", Description: "Jump to the Description section.", Group: "Sections"},
		{Keys: []string{"m"}, Label: "comments", Description: "Jump to the Comments section.", Group: "Sections"},
		{Keys: []string{"h"}, Label: "hierarchy", Description: "Jump to the Hierarchy section.", Group: "Sections"},
		{Keys: []string{"l"}, Label: "links", Description: "Jump to and focus the Links section.", Group: "Links"},
		{Keys: []string{"a"}, Label: "comment", Description: "Add a plain-text Jira comment.", Group: "Comments", Footer: true},
		{Keys: []string{"b"}, Label: "browser", Description: "Open the selected Jira issue in the browser.", Group: "Issue", Footer: true},
		{Keys: []string{"c"}, Label: "key", Description: "Copy the selected issue key.", Group: "Issue"},
		{Keys: []string{"y"}, Label: "url", Description: "Copy the selected issue URL.", Group: "Issue"},
		{Keys: []string{"r"}, Label: "refresh", Description: "Refresh the active issue view.", Group: "Global"},
	}
}

func linkBindings() []keyBinding {
	return []keyBinding{
		{Keys: []string{"esc"}, Label: "leave-links", Description: "Leave link focus and return to normal ticket detail navigation.", Group: "Navigation", Footer: true},
		{Keys: []string{"j", "k", "up", "down"}, FooterKey: "j/k", Label: "link", Description: "Select a discovered link.", Group: "Links", Footer: true},
		{Keys: []string{"o", "enter"}, FooterKey: "o/enter", Label: "open", Description: "Open the selected link.", Group: "Links", Footer: true},
		{Keys: []string{"y"}, Label: "copy", Description: "Copy the selected link or email address.", Group: "Links", Footer: true},
		{Keys: []string{"1-9"}, Label: "select", Description: "Select a link by number.", Group: "Links"},
		{Keys: []string{"r"}, Label: "refresh", Description: "Refresh the active issue view.", Group: "Global"},
	}
}

func hierarchyBindings() []keyBinding {
	return []keyBinding{
		{Keys: []string{"esc"}, Label: "leave", Description: "Leave hierarchy focus and return to normal ticket detail navigation.", Group: "Navigation", Footer: true},
		{Keys: []string{"j", "k", "up", "down"}, FooterKey: "j/k", Label: "child", Description: "Select a child issue.", Group: "Hierarchy", Footer: true},
		{Keys: []string{"enter"}, Label: "open", Description: "Open the selected child issue.", Group: "Hierarchy", Footer: true},
	}
}

func actionBindings() []keyBinding {
	return []keyBinding{
		{Keys: []string{"esc"}, Label: "leave", Description: "Leave action focus and return to normal ticket detail navigation.", Group: "Navigation", Footer: true},
		{Keys: []string{"j", "k", "up", "down"}, FooterKey: "j/k", Label: "action", Description: "Select a ticket action.", Group: "Actions", Footer: true},
		{Keys: []string{"enter"}, Label: "run", Description: "Run the selected ticket action.", Group: "Actions", Footer: true},
	}
}

func commentBindings() []keyBinding {
	return []keyBinding{
		{Keys: []string{"enter", "ctrl+j"}, Label: "newline", Description: "Insert a newline in the comment draft.", Group: "Editing", Footer: true},
		{Keys: []string{"backspace", "ctrl+h"}, Label: "delete", Description: "Delete the previous character.", Group: "Editing"},
		{Keys: []string{"pgup", "pgdn", "ctrl+b", "ctrl+f"}, FooterKey: "pgup/pgdn", Label: "page", Description: "Page through a long comment draft.", Group: "Editing", Footer: true},
		{Keys: []string{"home", "end"}, Label: "top/bottom", Description: "Jump to the top or bottom of a long comment draft.", Group: "Editing"},
		{Keys: []string{"@"}, Label: "mention", Description: "Open Jira user search and insert a selected user mention.", Group: "Editing", Footer: true},
		{Keys: []string{"tab", "ctrl+s"}, FooterKey: "tab/ctrl+s", Label: "review", Description: "Review the draft before posting.", Group: "Comments", Footer: true},
		{Keys: []string{"esc"}, Label: "cancel", Description: "Cancel the comment draft.", Group: "Comments", Footer: true},
	}
}

func mentionPickerBindings() []keyBinding {
	return []keyBinding{
		{Keys: []string{"type"}, Label: "filter", Description: "Type to search Jira users.", Group: "Mention Picker", Footer: true},
		{Keys: []string{"up", "down"}, FooterKey: "up/down", Label: "select", Description: "Move through matching Jira users.", Group: "Mention Picker", Footer: true},
		{Keys: []string{"enter"}, Label: "insert", Description: "Insert the selected Jira user mention.", Group: "Mention Picker", Footer: true},
		{Keys: []string{"esc"}, Label: "cancel", Description: "Close user search and insert the typed literal mention text.", Group: "Mention Picker", Footer: true},
	}
}

func commentConfirmBindings() []keyBinding {
	return []keyBinding{
		{Keys: []string{"y"}, Label: "post", Description: "Post the comment to Jira.", Group: "Comments", Footer: true},
		{Keys: []string{"n"}, Label: "edit", Description: "Return to editing the comment draft.", Group: "Comments", Footer: true},
		{Keys: []string{"esc"}, Label: "cancel", Description: "Cancel the comment draft.", Group: "Comments", Footer: true},
	}
}

func helpBindings() []keyBinding {
	return []keyBinding{
		{Keys: []string{"?", "esc"}, FooterKey: "esc/?", Label: "close", Description: "Close this help screen.", Group: "Help", Footer: true},
		{Keys: []string{"j", "k", "up", "down"}, FooterKey: "j/k", Label: "scroll", Description: "Scroll the keyboard help screen.", Group: "Help", Footer: true},
		{Keys: []string{"pgup", "pgdn", "space", "ctrl+b", "ctrl+f"}, FooterKey: "pgup/pgdn", Label: "page", Description: "Page through the keyboard help screen.", Group: "Help", Footer: true},
		{Keys: []string{"g", "G", "home", "end"}, FooterKey: "g/G", Label: "top/bottom", Description: "Jump to the top or bottom of keyboard help.", Group: "Help"},
	}
}
