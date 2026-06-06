package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jon/jira-tui/internal/jira"
	"github.com/jon/jira-tui/internal/worker"
)

const (
	maxIssues             = 50
	initialRequestID      = 1
	defaultRequestTimeout = 20 * time.Second
	defaultWorkerCount    = 2
	defaultQueueSize      = 16
)

type Model struct {
	workers *worker.Pool
	jql     string

	issues   []jira.Issue
	selected int
	width    int
	height   int

	loading    bool
	refreshing bool
	err        error
	lastSynced time.Time

	refreshInterval time.Duration
	requestTimeout  time.Duration
	workerCount     int
	queueSize       int
	nextRequestID   int
	activeRequestID int
}

type Option func(*Model)

func WithRefreshInterval(interval time.Duration) Option {
	return func(m *Model) {
		m.refreshInterval = interval
	}
}

func WithRequestTimeout(timeout time.Duration) Option {
	return func(m *Model) {
		m.requestTimeout = timeout
	}
}

func WithWorkerCount(count int) Option {
	return func(m *Model) {
		m.workerCount = count
	}
}

func WithQueueSize(size int) Option {
	return func(m *Model) {
		m.queueSize = size
	}
}

type refreshTickMsg struct{}
type workSubmittedMsg struct{}
type workerStoppedMsg struct{}

type workerResultMsg struct {
	result worker.Result
}

func NewModel(client worker.IssueSearcher, jql string, options ...Option) Model {
	model := Model{
		jql:             jql,
		loading:         true,
		requestTimeout:  defaultRequestTimeout,
		workerCount:     defaultWorkerCount,
		queueSize:       defaultQueueSize,
		nextRequestID:   initialRequestID,
		activeRequestID: initialRequestID,
	}
	for _, option := range options {
		option(&model)
	}
	model.workers = worker.NewPool(
		client,
		worker.WithWorkerCount(model.workerCount),
		worker.WithQueueSize(model.queueSize),
	)
	return model
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.submitIssueSearch(m.activeRequestID),
		m.waitForWorkerResult(),
		m.scheduleRefresh(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case refreshTickMsg:
		var cmd tea.Cmd
		if !m.loading && !m.refreshing {
			m, cmd = m.startRefresh()
		}
		return m, tea.Batch(cmd, m.scheduleRefresh())
	case workerResultMsg:
		return m.handleWorkerResult(msg.result), m.waitForWorkerResult()
	case workerStoppedMsg, workSubmittedMsg:
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.workers.Stop()
			return m, tea.Quit
		case "r":
			m.err = nil
			return m.startRefresh()
		case "up", "k":
			m.selected = max(0, m.selected-1)
		case "down", "j":
			m.selected = min(len(m.issues)-1, m.selected+1)
		}
	}
	return m, nil
}

func (m Model) handleWorkerResult(result worker.Result) Model {
	if result.Kind != worker.KindSearchIssues || result.ID != m.activeRequestID {
		return m
	}

	m.loading = false
	m.refreshing = false
	if result.Err != nil {
		m.err = result.Err
		return m
	}
	if result.SearchIssues == nil {
		m.err = worker.ErrInvalidRequest
		return m
	}

	m.err = nil
	m.replaceIssues(result.SearchIssues.Issues)
	m.lastSynced = result.SearchIssues.SyncedAt
	return m
}

func (m Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m Model) render() string {
	width := m.width
	if width <= 0 {
		width = 100
	}

	var b strings.Builder
	b.WriteString("Jira TUI")
	if m.refreshing {
		b.WriteString(" - refreshing")
	}
	if !m.lastSynced.IsZero() {
		b.WriteString(" - synced ")
		b.WriteString(m.lastSynced.Format("15:04:05"))
	}
	b.WriteByte('\n')
	b.WriteString(truncate(m.jql, width))
	b.WriteString("\n\n")

	switch {
	case m.loading:
		b.WriteString("Loading issues...\n")
	case m.err != nil && len(m.issues) == 0:
		b.WriteString("Error: ")
		b.WriteString(m.err.Error())
		b.WriteString("\n\nPress r to retry or q to quit.\n")
	case len(m.issues) == 0:
		b.WriteString("No issues matched this query.\n\nPress r to refresh or q to quit.\n")
	default:
		m.renderIssueList(&b, width)
		if m.err != nil {
			b.WriteString("\n")
			b.WriteString(truncate("Last refresh failed: "+m.err.Error(), width))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString("j/k move  r refresh  q quit")
	return b.String()
}

func (m Model) renderIssueList(b *strings.Builder, width int) {
	rows := m.visibleRows()
	start := clamp(m.selected-rows+1, 0, max(0, len(m.issues)-rows))
	for i, issue := range m.issues[start:min(len(m.issues), start+rows)] {
		index := start + i
		cursor := " "
		if index == m.selected {
			cursor = ">"
		}
		line := fmt.Sprintf("%s %-12s %-16s %s", cursor, issue.Key, issue.Status, issue.Summary)
		b.WriteString(truncate(line, width))
		b.WriteByte('\n')
	}

	selected := m.issues[m.selected]
	b.WriteString("\n")
	b.WriteString(truncate(fmt.Sprintf("%s | %s", selected.Assignee, selected.URL), width))
	b.WriteString("\n")
}

func (m Model) startRefresh() (Model, tea.Cmd) {
	m.nextRequestID++
	m.activeRequestID = m.nextRequestID
	if len(m.issues) == 0 {
		m.loading = true
	} else {
		m.refreshing = true
	}
	return m, m.submitIssueSearch(m.activeRequestID)
}

func (m Model) submitIssueSearch(requestID int) tea.Cmd {
	return func() tea.Msg {
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindSearchIssues,
			Timeout: m.requestTimeout,
			SearchIssues: &worker.SearchIssuesRequest{
				JQL:        m.jql,
				MaxResults: maxIssues,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindSearchIssues,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{}
	}
}

func (m Model) scheduleRefresh() tea.Cmd {
	if m.refreshInterval <= 0 {
		return nil
	}
	return tea.Tick(m.refreshInterval, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m Model) waitForWorkerResult() tea.Cmd {
	return func() tea.Msg {
		result, ok := <-m.workers.Results()
		if !ok {
			return workerStoppedMsg{}
		}
		return workerResultMsg{result: result}
	}
}

func (m *Model) replaceIssues(issues []jira.Issue) {
	selectedKey := ""
	if len(m.issues) > 0 && m.selected >= 0 && m.selected < len(m.issues) {
		selectedKey = m.issues[m.selected].Key
	}

	m.issues = issues
	if len(m.issues) == 0 {
		m.selected = 0
		return
	}

	if selectedKey != "" {
		for index, issue := range m.issues {
			if issue.Key == selectedKey {
				m.selected = index
				return
			}
		}
	}
	m.selected = clamp(m.selected, 0, len(m.issues)-1)
}

func (m Model) visibleRows() int {
	if m.height <= 7 {
		return 10
	}
	return max(1, m.height-7)
}

func truncate(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	if width <= 3 {
		return value[:width]
	}
	return value[:width-3] + "..."
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
