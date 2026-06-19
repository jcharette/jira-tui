package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type choiceListOption struct {
	Label string
}

func (o choiceListOption) FilterValue() string {
	return o.Label
}

type choiceListDelegate struct {
	model Model
}

func (d choiceListDelegate) Height() int {
	return 1
}

func (d choiceListDelegate) Spacing() int {
	return 0
}

func (d choiceListDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d choiceListDelegate) Render(w io.Writer, listModel list.Model, index int, item list.Item) {
	option, ok := item.(choiceListOption)
	if !ok {
		return
	}
	prefix := "  "
	style := d.model.theme.Text
	if index == listModel.Index() {
		prefix = "> "
		style = d.model.theme.Selected
	}
	label := truncate(option.Label, max(1, listModel.Width()-len(prefix)))
	fmt.Fprint(w, style.Render(prefix+label))
}

func (m Model) renderChoiceList(options []choiceListOption, selected int, width int, rows int) []string {
	if len(options) == 0 {
		return nil
	}
	items := make([]list.Item, 0, len(options))
	for _, option := range options {
		items = append(items, option)
	}
	listModel := list.New(items, choiceListDelegate{model: m}, max(1, width), max(1, rows))
	listModel.SetFilteringEnabled(false)
	listModel.SetShowTitle(false)
	listModel.SetShowFilter(false)
	listModel.SetShowStatusBar(false)
	listModel.SetShowPagination(false)
	listModel.SetShowHelp(false)
	listModel.Select(clamp(selected, 0, len(options)-1))

	view := strings.TrimRight(listModel.View(), "\n ")
	lines := strings.Split(view, "\n")
	start, end := listModel.Paginator.GetSliceBounds(len(listModel.VisibleItems()))
	if len(options) > listModel.Paginator.PerPage {
		lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("%d-%d of %d", start+1, end, len(options))))
	}
	return lines
}
