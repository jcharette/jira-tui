package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	lipglosstable "github.com/charmbracelet/lipgloss/table"
	"github.com/jon/jira-tui/internal/config"
	"github.com/jon/jira-tui/internal/jira"
	"github.com/jon/jira-tui/internal/linkdetect"
	"github.com/jon/jira-tui/internal/mentiondetect"
	"github.com/jon/jira-tui/internal/ui"
	"github.com/jon/jira-tui/internal/worker"
)

const (
	maxIssues             = 50
	maxComments           = 10
	initialRequestID      = 1
	defaultRequestTimeout = 20 * time.Second
	defaultWorkerCount    = 2
	defaultQueueSize      = 16
	minUsefulIssueRows    = 8
	appChromeRows         = 6
	panelFrameRows        = 4
	detailHeaderRows      = 6
	issueTreeRootGutter   = 2
	issueTreeMaxGutter    = 12
	issueTypeColumnWidth  = 2
)

type browserLayout struct {
	contentWidth int
	listWidth    int
	rows         int
}

type issueDisplayRow struct {
	issue          jira.Issue
	parent         *jira.Issue
	parentVisible  bool
	index          int
	childCount     int
	hiddenChildren int
}

type issueListColumns struct {
	width         int
	gutterWidth   int
	typeWidth     int
	keyWidth      int
	statusWidth   int
	priorityWidth int
	assigneeWidth int
	showStatus    bool
	showPriority  bool
	showAssignee  bool
	summaryWidth  int
}

type issueDisplayTree struct {
	issues          []jira.Issue
	roots           []issueDisplayRoot
	children        map[string][]int
	indexByKey      map[string]int
	missingParents  map[string]missingParentGroup
	missingParentOf map[string]bool
}

type issueDisplayRoot struct {
	issueIndex       int
	missingParentKey string
}

type missingParentGroup struct {
	key      string
	summary  string
	children []int
}

type issueSymbolMode string

const (
	symbolModeAuto    issueSymbolMode = "auto"
	symbolModePlain   issueSymbolMode = "plain"
	symbolModeSymbols issueSymbolMode = "symbols"
	symbolModeEmoji   issueSymbolMode = "emoji"
	symbolModeNerd    issueSymbolMode = "nerd"
)

type issueSymbols struct {
	Epic    string
	Story   string
	Task    string
	Bug     string
	Subtask string
	Issue   string
}

type Model struct {
	workers *worker.Pool
	jql     string
	views   []config.IssueView
	view    int
	mode    mode
	sort    sortMode

	issues              []jira.Issue
	selected            int
	offset              int
	detailOffset        int
	detailSectionOffset map[string]int
	detailFocus         int
	detailBackStack     []int
	hierarchyFocus      bool
	selectedHierarchy   int
	actionFocus         bool
	selectedAction      int
	detailViewport      viewport.Model
	detailViewportReady bool
	linkFocus           bool
	selectedLink        int
	width               int
	height              int
	helpOpen            bool
	helpOffset          int

	loading    bool
	refreshing bool
	err        error
	lastSynced time.Time

	details               map[string]jira.IssueDetail
	detailLoading         bool
	detailErr             error
	detailRequestKey      string
	comments              map[string][]jira.Comment
	commentsLoading       bool
	commentsErr           error
	commentsRequestKey    string
	commentDraft          string
	commentEditor         textarea.Model
	commentEditorReady    bool
	commentConfirm        bool
	commentSubmitting     bool
	commentRequestKey     string
	commentMentions       []jira.Mention
	mentionPickerOpen     bool
	mentionUsers          []jira.User
	mentionCursor         int
	mentionQuery          string
	mentionSearchLoading  bool
	mentionSearchErr      error
	mentionSearchReqID    int
	detailNotice          string
	activeDetailRequestID int
	activeCommentsReqID   int
	activeCommentReqID    int
	activeExpandReqID     int
	expandLoading         bool
	expandRequestKey      string
	expandMode            worker.ExpandMode

	refreshInterval time.Duration
	requestTimeout  time.Duration
	workerCount     int
	queueSize       int
	nextRequestID   int
	activeRequestID int
	theme           ui.Theme
	symbolMode      issueSymbolMode
}

type mode int
type sortMode int

const (
	modeTable mode = iota
	modeDetail
	modeComment
)

const (
	sortJira sortMode = iota
	sortPriority
	sortStatus
	sortAssignee
	sortType
	sortKey
)

type detailLink = linkdetect.Link

type detailAction struct {
	ID          string
	Label       string
	Description string
	Enabled     bool
}

type detailSection struct {
	ID    string
	Label string
	Short string
	Badge string
}

type hierarchyRow struct {
	Issue jira.Issue
	Group string
	Index int
}

type detailRenderContext struct {
	selected    jira.Issue
	display     jira.Issue
	detail      jira.IssueDetail
	hasDetail   bool
	description string
	links       []detailLink
}

var (
	openExternal    = defaultOpenExternal
	copyToClipboard = defaultCopyToClipboard
)

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

func WithTheme(theme config.Theme) Option {
	return func(m *Model) {
		m.theme = ui.NewTheme(theme)
	}
}

func WithDisplay(display config.Display) Option {
	return func(m *Model) {
		m.symbolMode = issueSymbolMode(strings.ToLower(strings.TrimSpace(display.SymbolMode)))
		if m.symbolMode == "" {
			m.symbolMode = symbolModeAuto
		}
	}
}

func WithViews(views []config.IssueView, active string) Option {
	return func(m *Model) {
		if len(views) == 0 {
			return
		}
		m.views = views
		m.view = 0
		for index, view := range views {
			if strings.EqualFold(view.Name, active) {
				m.view = index
				break
			}
		}
		m.jql = views[m.view].JQL
	}
}

type refreshTickMsg struct{}
type workSubmittedMsg struct{}
type noDetailRequestMsg struct{}
type workerStoppedMsg struct{}

type workerResultMsg struct {
	result worker.Result
}

type linkActionMsg struct {
	action string
	target string
	err    error
}

func NewModel(client worker.JiraClient, jql string, options ...Option) Model {
	model := Model{
		jql:                 jql,
		loading:             true,
		requestTimeout:      defaultRequestTimeout,
		workerCount:         defaultWorkerCount,
		queueSize:           defaultQueueSize,
		nextRequestID:       initialRequestID,
		activeRequestID:     initialRequestID,
		theme:               ui.NewTheme(config.DefaultTheme()),
		symbolMode:          symbolModeAuto,
		details:             make(map[string]jira.IssueDetail),
		comments:            make(map[string][]jira.Comment),
		detailSectionOffset: make(map[string]int),
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
		m.ensureSelectionVisible(m.browserLayout(msg.Width).rows)
		return m, nil
	case refreshTickMsg:
		var cmd tea.Cmd
		if !m.loading && !m.refreshing {
			m, cmd = m.startRefresh()
		}
		return m, tea.Batch(cmd, m.scheduleRefresh())
	case workerResultMsg:
		var cmd tea.Cmd
		m, cmd = m.handleWorkerResult(msg.result)
		return m, tea.Batch(cmd, m.waitForWorkerResult())
	case linkActionMsg:
		m.detailNotice = linkActionNotice(msg)
		return m, nil
	case workerStoppedMsg, workSubmittedMsg, noDetailRequestMsg:
		return m, nil
	case tea.PasteMsg:
		if m.mode == modeComment {
			return m.updateCommentComposer(msg)
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "?" {
			m.helpOpen = !m.helpOpen
			m.helpOffset = 0
			return m, nil
		}
		if m.helpOpen {
			switch msg.String() {
			case "esc":
				m.helpOpen = false
			case "j", "down":
				m.scrollHelp(1)
			case "k", "up":
				m.scrollHelp(-1)
			case "pgdown", "space", "ctrl+f":
				m.scrollHelp(m.helpRows())
			case "pgup", "ctrl+b":
				m.scrollHelp(-m.helpRows())
			case "g", "home":
				m.helpOffset = 0
			case "G", "end":
				m.scrollHelp(1 << 20)
			}
			return m, nil
		}
		if m.mode == modeComment {
			return m.updateCommentComposer(msg)
		}
		switch msg.String() {
		case "ctrl+c", "q":
			m.workers.Stop()
			return m, tea.Quit
		case "esc":
			if m.mode == modeDetail {
				if m.actionFocus {
					m.actionFocus = false
					m.detailNotice = ""
					return m, nil
				}
				if m.hierarchyFocus {
					m.hierarchyFocus = false
					m.detailNotice = ""
					return m, nil
				}
				if m.linkFocus {
					m.linkFocus = false
					m.detailNotice = ""
					return m, nil
				}
				if len(m.detailBackStack) > 0 {
					return m.popDetailBackStack()
				}
				m.mode = modeTable
				m.resetDetailScroll()
				m.linkFocus = false
				m.hierarchyFocus = false
				m.actionFocus = false
				m.detailNotice = ""
			}
		case "r":
			m.err = nil
			return m.startRefresh()
		case "x":
			if m.mode == modeTable {
				return m.startExpandSelectedIssue(worker.ExpandModeOpen)
			}
		case "X":
			if m.mode == modeTable {
				return m.startExpandSelectedIssue(worker.ExpandModeAll)
			}
		case "[", "shift+tab", "backtab":
			if m.mode == modeDetail {
				m.moveDetailFocus(-1)
				return m, nil
			}
			return m.switchView(-1)
		case "]", "tab":
			if m.mode == modeDetail {
				m.moveDetailFocus(1)
				return m, nil
			}
			return m.switchView(1)
		case "o":
			if m.mode == modeDetail && m.linkFocus {
				return m.openSelectedDetailLink()
			}
			m.switchSort(1)
		case "O":
			m.switchSort(-1)
		case "y":
			if m.mode == modeDetail && m.linkFocus {
				return m.copySelectedDetailLink()
			}
			if m.mode == modeDetail {
				return m.copySelectedIssueURL()
			}
		case "c":
			if m.mode == modeDetail {
				return m.copySelectedIssueKey()
			}
		case "b":
			if m.mode == modeDetail {
				return m.openSelectedIssue()
			}
		case "a":
			if m.mode == modeDetail {
				m.startCommentComposer()
				return m, nil
			}
		case "enter":
			if m.mode == modeDetail && m.linkFocus {
				return m.openSelectedDetailLink()
			}
			if m.mode == modeDetail && m.actionFocus {
				return m.runSelectedDetailAction()
			}
			if m.mode == modeDetail && m.hierarchyFocus {
				return m.openSelectedHierarchyIssue()
			}
			if m.mode == modeDetail {
				m.activateDetailFocus()
				return m, nil
			}
			if m.mode != modeDetail && len(m.issues) > 0 {
				m.mode = modeDetail
				m.resetDetailScroll()
				m.detailFocus = 0
				m.detailBackStack = nil
				m.linkFocus = false
				m.detailNotice = ""
				return m.startDetailRequestForSelected()
			}
		case "up", "k":
			if m.mode == modeDetail {
				if m.linkFocus {
					m.moveSelectedDetailLink(-1)
					return m, nil
				}
				if m.actionFocus {
					m.moveSelectedDetailAction(-1)
					return m, nil
				}
				if m.hierarchyFocus {
					m.moveSelectedHierarchyIssue(-1)
					return m, nil
				}
				if m.canMoveHierarchySelection() {
					m.moveSelectedHierarchyIssue(-1)
					return m, nil
				}
				m.scrollDetail(-1)
				return m, nil
			}
			m.moveSelection(-1)
			return m.startDetailRequestForSelected()
		case "down", "j":
			if m.mode == modeDetail {
				if m.linkFocus {
					m.moveSelectedDetailLink(1)
					return m, nil
				}
				if m.actionFocus {
					m.moveSelectedDetailAction(1)
					return m, nil
				}
				if m.hierarchyFocus {
					m.moveSelectedHierarchyIssue(1)
					return m, nil
				}
				if m.canMoveHierarchySelection() {
					m.moveSelectedHierarchyIssue(1)
					return m, nil
				}
				m.scrollDetail(1)
				return m, nil
			}
			m.moveSelection(1)
			return m.startDetailRequestForSelected()
		case "pgup", "ctrl+b":
			if m.mode == modeDetail {
				m.pageDetail(-1)
				return m, nil
			}
			m.pageSelection(-1)
			return m.startDetailRequestForSelected()
		case "pgdown", "ctrl+f", " ":
			if m.mode == modeDetail {
				m.pageDetail(1)
				return m, nil
			}
			m.pageSelection(1)
			return m.startDetailRequestForSelected()
		case "home", "g":
			if m.mode == modeDetail {
				m.setDetailOffset(0)
				m.linkFocus = false
				return m, nil
			}
			m.selected = 0
			m.offset = 0
			return m.startDetailRequestForSelected()
		case "end", "G":
			if m.mode == modeDetail {
				m.scrollDetailToBottom()
				m.linkFocus = false
				return m, nil
			}
			m.selected = max(0, len(m.issues)-1)
			m.ensureSelectionVisible(m.currentLayoutRows())
			return m.startDetailRequestForSelected()
		case "l":
			if m.mode == modeDetail {
				m.focusDetailLinks()
				return m, nil
			}
		case "d":
			if m.mode == modeDetail {
				m.linkFocus = false
				m.hierarchyFocus = false
				m.actionFocus = false
				m.jumpDetailSection("Description")
				return m, nil
			}
		case "h":
			if m.mode == modeDetail {
				m.linkFocus = false
				m.hierarchyFocus = false
				m.actionFocus = false
				m.jumpDetailSection("Hierarchy")
				return m, nil
			}
		case "m":
			if m.mode == modeDetail {
				m.linkFocus = false
				m.hierarchyFocus = false
				m.actionFocus = false
				m.jumpDetailSection("Comments")
				return m, nil
			}
		case "n":
			if m.mode == modeDetail {
				m.moveDetailFocus(1)
				return m, nil
			}
		case "p":
			if m.mode == modeDetail {
				m.moveDetailFocus(-1)
				return m, nil
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if m.mode == modeDetail {
				m.selectDetailLinkNumber(msg.String())
				return m, nil
			}
		}
	}
	return m, nil
}

func (m Model) handleWorkerResult(result worker.Result) (Model, tea.Cmd) {
	switch result.Kind {
	case worker.KindSearchIssues:
		return m.handleSearchResult(result)
	case worker.KindGetIssue:
		return m.handleDetailResult(result), nil
	case worker.KindGetComments:
		return m.handleCommentsResult(result), nil
	case worker.KindAddComment:
		return m.handleAddCommentResult(result)
	case worker.KindSearchUsers:
		return m.handleUserSearchResult(result), nil
	case worker.KindExpandIssues:
		return m.handleExpandIssuesResult(result), nil
	default:
		return m, nil
	}
}

func (m Model) handleExpandIssuesResult(result worker.Result) Model {
	if result.ID != m.activeExpandReqID {
		return m
	}
	m.expandLoading = false
	if result.Err != nil {
		m.detailNotice = "Expand failed: " + result.Err.Error()
		return m
	}
	if result.ExpandIssues == nil {
		m.detailNotice = "Expand failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	if result.ExpandIssues.ParentKey != m.expandRequestKey || result.ExpandIssues.Mode != m.expandMode {
		return m
	}
	added := m.mergeExpandedIssues(result.ExpandIssues.Issues)
	label := "open children"
	if result.ExpandIssues.Mode == worker.ExpandModeAll {
		label = "all children"
	}
	if added == 0 {
		m.detailNotice = "No new " + label + " found for " + result.ExpandIssues.ParentKey + "."
		return m
	}
	m.detailNotice = fmt.Sprintf("Loaded %d %s for %s.", added, label, result.ExpandIssues.ParentKey)
	m.ensureSelectionVisible(m.currentLayoutRows())
	return m
}

func (m Model) handleSearchResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeRequestID {
		return m, nil
	}
	m.loading = false
	m.refreshing = false
	if result.Err != nil {
		m.err = result.Err
		return m, nil
	}
	if result.SearchIssues == nil {
		m.err = worker.ErrInvalidRequest
		return m, nil
	}

	m.err = nil
	m.replaceIssues(result.SearchIssues.Issues)
	m.lastSynced = result.SearchIssues.SyncedAt
	return m.startDetailRequestForSelected()
}

func (m Model) handleDetailResult(result worker.Result) Model {
	if result.ID != m.activeDetailRequestID {
		return m
	}
	if m.detailRequestKey != "" {
		selected, ok := m.selectedIssue()
		if ok && selected.Key != m.detailRequestKey {
			return m
		}
	}

	m.detailLoading = false
	if result.Err != nil {
		m.detailErr = result.Err
		return m
	}
	if result.GetIssue == nil {
		m.detailErr = worker.ErrInvalidRequest
		return m
	}
	if m.details == nil {
		m.details = make(map[string]jira.IssueDetail)
	}
	m.details[result.GetIssue.Key] = result.GetIssue.Detail
	m.detailErr = nil
	return m
}

func (m Model) handleAddCommentResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeCommentReqID {
		return m, nil
	}
	if m.commentRequestKey != "" {
		selected, ok := m.selectedIssue()
		if ok && selected.Key != m.commentRequestKey {
			return m, nil
		}
	}
	m.commentSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Comment failed: " + result.Err.Error()
		m.commentConfirm = false
		return m, nil
	}
	if result.AddComment == nil {
		m.detailNotice = "Comment failed: " + worker.ErrInvalidRequest.Error()
		m.commentConfirm = false
		return m, nil
	}

	key := result.AddComment.Key
	m.mode = modeDetail
	m.commentDraft = ""
	m.commentMentions = nil
	m.commentConfirm = false
	m.commentRequestKey = ""
	m.detailNotice = "Comment posted."
	if m.comments != nil {
		delete(m.comments, key)
	}
	m.nextRequestID++
	m.activeCommentsReqID = m.nextRequestID
	m.commentsRequestKey = key
	m.commentsLoading = true
	m.commentsErr = nil
	return m, m.submitIssueComments(m.activeCommentsReqID, key)
}

func (m Model) handleUserSearchResult(result worker.Result) Model {
	if result.ID != m.mentionSearchReqID {
		return m
	}
	m.mentionSearchLoading = false
	if result.Err != nil {
		m.mentionSearchErr = result.Err
		return m
	}
	if result.SearchUsers == nil {
		m.mentionSearchErr = worker.ErrInvalidRequest
		return m
	}
	if result.SearchUsers.Query != m.mentionQuery {
		return m
	}
	m.mentionUsers = result.SearchUsers.Users
	m.mentionCursor = clamp(m.mentionCursor, 0, max(0, len(m.mentionUsers)-1))
	m.mentionSearchErr = nil
	return m
}

func (m Model) handleCommentsResult(result worker.Result) Model {
	if result.ID != m.activeCommentsReqID {
		return m
	}
	if m.commentsRequestKey != "" {
		selected, ok := m.selectedIssue()
		if ok && selected.Key != m.commentsRequestKey {
			return m
		}
	}

	m.commentsLoading = false
	if result.Err != nil {
		m.commentsErr = result.Err
		return m
	}
	if result.GetComments == nil {
		m.commentsErr = worker.ErrInvalidRequest
		return m
	}
	if m.comments == nil {
		m.comments = make(map[string][]jira.Comment)
	}
	m.comments[result.GetComments.Key] = result.GetComments.Comments
	m.commentsErr = nil
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
	layout := m.browserLayout(width)

	var b strings.Builder
	b.WriteString(m.renderHeader(layout))
	b.WriteString("\n")

	if ui.TerminalTooSmall(m.width, m.height) {
		b.WriteString(m.renderStatePanel(layout, "Terminal Size", m.theme.Warning.Render(ui.TerminalSizeMessage(m.width, m.height))))
		b.WriteString("\n\n")
		b.WriteString(m.renderFooterHelp(activeKeyContext(m), layout))
		return b.String()
	}

	b.WriteString(m.renderQuery(layout))
	b.WriteString("\n\n")

	if m.helpOpen {
		b.WriteString(m.renderHelp(layout))
		b.WriteString("\n\n")
		b.WriteString(m.renderFooterHelp(keyContextHelp, layout))
		return b.String()
	}

	if m.mode == modeComment && len(m.issues) > 0 {
		b.WriteString(m.renderCommentComposer(layout))
		b.WriteString("\n\n")
		b.WriteString(m.renderFooterHelp(activeKeyContext(m), layout))
		return b.String()
	}

	if m.mode == modeDetail && len(m.issues) > 0 {
		b.WriteString(m.renderFullDetail(layout))
		b.WriteString("\n\n")
		b.WriteString(m.renderFooterHelp(activeKeyContext(m), layout))
		return b.String()
	}

	switch {
	case m.loading:
		b.WriteString(m.renderStatePanel(layout, "Issues", m.theme.Muted.Render("Loading issues...")))
	case m.err != nil && len(m.issues) == 0:
		b.WriteString(m.renderStatePanel(layout, "Error", m.theme.Error.Render(truncate(m.err.Error(), max(20, layout.contentWidth-6)))))
	case len(m.issues) == 0:
		b.WriteString(m.renderStatePanel(layout, "No Issues", m.theme.Warning.Render("No issues matched this query.")))
	default:
		b.WriteString(m.renderIssueWorkspace(layout))
		if m.err != nil {
			b.WriteString("\n\n")
			b.WriteString(m.theme.Error.Render(truncate("Last refresh failed: "+m.err.Error(), layout.contentWidth)))
		}
	}
	if m.mode == modeTable && m.detailNotice != "" {
		b.WriteString("\n\n")
		b.WriteString(m.renderDetailNotice(m.detailNotice, max(32, layout.contentWidth-8)))
	}

	b.WriteString("\n\n")
	b.WriteString(m.renderFooterHelp(activeKeyContext(m), layout))
	return b.String()
}

func (m Model) renderHeader(layout browserLayout) string {
	status := "ready"
	if m.loading {
		status = "loading"
	} else if m.refreshing {
		status = "refreshing"
	}

	synced := "not synced"
	if !m.lastSynced.IsZero() {
		synced = "synced " + m.lastSynced.Format("15:04:05")
	}

	left := m.theme.Header.Render("Jira") + " " + m.theme.Subtitle.Render(status) + " " + m.theme.Selected.Render(m.activeViewName())
	right := m.theme.Muted.Render(fmt.Sprintf("%d issues  %s", len(m.issues), synced))
	rightColumn := lipgloss.PlaceHorizontal(
		max(0, layout.contentWidth-lipgloss.Width(left)-1),
		lipgloss.Right,
		right,
	)
	return lipgloss.NewStyle().Width(layout.contentWidth).Render(left + " " + rightColumn)
}

func (m Model) renderQuery(layout browserLayout) string {
	label := m.theme.PaneTitle.Render("Filter")
	labelWidth := lipgloss.Width("Filter  ")
	queryWidth := max(20, layout.contentWidth-labelWidth-2)
	query := m.theme.Muted.Render(truncate(m.filterSummary(), queryWidth))
	return label + "  " + query
}

func (m Model) renderStatePanel(layout browserLayout, title string, body string) string {
	return m.theme.Panel.Width(layout.contentWidth).Render(m.theme.PaneTitle.Render(title) + "\n\n" + body)
}

func (m Model) renderFooterHelp(context keyContext, layout browserLayout) string {
	available := max(20, layout.contentWidth)
	rendered := m.footerContextLabel(context, available)
	currentGroup := ""
	for _, binding := range footerBindings(context) {
		next := m.theme.Key.Render(binding.keyText()) + " " + m.theme.Muted.Render(binding.Label)
		if currentGroup != "" && binding.Group != currentGroup {
			next = m.theme.Muted.Render("|") + m.theme.Muted.Render("  ") + next
		}
		candidate := next
		if rendered != "" {
			candidate = rendered + m.theme.Muted.Render("  ") + next
		}
		if lipgloss.Width(candidate) > available {
			break
		}
		rendered = candidate
		currentGroup = binding.Group
	}
	return rendered
}

func (m Model) footerContextLabel(context keyContext, width int) string {
	label := string(context)
	if label == "" || lipgloss.Width(label)+2 > width {
		return ""
	}
	return m.theme.Muted.Render(label)
}

func (m Model) renderHelp(layout browserLayout) string {
	context := activeKeyContext(m)
	lines := m.helpLines(context)
	rows := m.helpRows()
	offset := clamp(m.helpOffset, 0, max(0, len(lines)-rows))
	end := min(len(lines), offset+rows)
	var b strings.Builder
	b.WriteString(m.detailSectionHeader("help", "Keyboard Help", string(context), max(32, layout.contentWidth-4)))
	b.WriteString("\n\n")
	if len(lines) > 0 {
		b.WriteString(strings.Join(lines[offset:end], "\n"))
	}
	if len(lines) > rows {
		indicator := m.theme.Muted.Render(fmt.Sprintf("Lines %d-%d of %d", offset+1, end, len(lines)))
		if offset > 0 {
			indicator += m.theme.Muted.Render("  PgUp previous")
		}
		if end < len(lines) {
			indicator += m.theme.Muted.Render("  PgDn next")
		}
		b.WriteString("\n")
		b.WriteString(truncate(indicator, layout.contentWidth-6))
	}
	return m.theme.ActivePane.Width(layout.contentWidth).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) helpLines(context keyContext) []string {
	bindings := append(helpBindings(), keyBindings(context)...)
	var lines []string
	currentGroup := ""
	for _, binding := range bindings {
		if binding.Group != currentGroup {
			if currentGroup != "" {
				lines = append(lines, "")
			}
			currentGroup = binding.Group
			lines = append(lines, m.theme.FieldLabel.Render(currentGroup))
		}
		keys := m.theme.Key.Render(binding.keyText())
		padding := strings.Repeat(" ", max(1, 16-lipgloss.Width(binding.keyText())))
		lines = append(lines, keys+m.theme.Muted.Render(padding)+m.theme.Text.Render(binding.Description))
	}
	return lines
}

func (m Model) helpRows() int {
	return m.boundedPanelBodyRows(3)
}

func (m Model) boundedPanelBodyRows(reservedInsidePanel int) int {
	if m.height <= 0 {
		return 18
	}
	return max(1, m.height-appChromeRows-panelFrameRows-reservedInsidePanel)
}

func (m Model) commentEditorRows() int {
	reserved := 7
	if m.commentConfirm {
		reserved = 9
	}
	if m.detailNotice != "" {
		reserved += 2
	}
	reserved += m.commentLinkPreviewRows()
	reserved += m.commentMentionPreviewRows()
	reserved += m.mentionPickerRows()
	return max(2, m.boundedPanelBodyRows(reserved))
}

func (m *Model) scrollHelp(delta int) {
	lines := m.helpLines(activeKeyContext(*m))
	m.helpOffset = clamp(m.helpOffset+delta, 0, max(0, len(lines)-m.helpRows()))
}

func (m Model) renderIssueWorkspace(layout browserLayout) string {
	return m.renderIssueList(layout)
}

func (m Model) renderIssueList(layout browserLayout) string {
	var b strings.Builder
	rowCount := layout.rows
	rows := m.issueRows(layout)
	start := clamp(m.offset, 0, max(0, len(rows)-rowCount))
	end := min(len(rows), start+rowCount)

	b.WriteString(m.issueListHeader(layout))
	b.WriteByte('\n')
	if len(m.issues) > 0 {
		vp := viewport.New(
			viewport.WithWidth(max(1, layout.listWidth-4)),
			viewport.WithHeight(max(1, rowCount)),
		)
		vp.SoftWrap = false
		vp.FillHeight = false
		vp.SetContent(strings.Join(rows, "\n"))
		vp.SetYOffset(start)
		b.WriteString(strings.TrimRight(vp.View(), "\n "))
	} else {
		b.WriteString(m.theme.Muted.Render("No issues found for this view."))
	}

	title := m.issueListTitle(len(rows), rowCount, start, end)
	content := m.issueListTitleLine(title, layout) + "\n" + strings.TrimRight(b.String(), "\n")
	if len(rows) > rowCount {
		content += "\n" + m.theme.Muted.Render(m.pageIndicator(start, end))
	}
	return m.theme.ActivePane.Width(layout.listWidth).Render(content)
}

func (m Model) issueListTitle(rowTotal int, rowCount int, start int, end int) string {
	title := "0 issues"
	if len(m.issues) > 0 {
		title = fmt.Sprintf("%d issues", len(m.issues))
		if rowTotal > rowCount {
			title = fmt.Sprintf("%d issues  rows %d-%d", len(m.issues), start+1, end)
		}
	}
	return title
}

func (m Model) issueListTitleLine(title string, layout browserLayout) string {
	line := m.theme.PaneTitle.Render(title) + " " + m.theme.Muted.Render("sorted by "+m.sortLabel())
	if m.useCompactIssueListChrome(layout) {
		return line
	}
	return line + "\n"
}

func (m Model) issueListHeader(layout browserLayout) string {
	columns := m.issueListColumns(layout)
	left := fmt.Sprintf("%s %s %-*s  %s", strings.Repeat(" ", columns.gutterWidth), padRight("T", columns.typeWidth), columns.keyWidth, "KEY", "SUMMARY")
	right := m.issueListMetaHeader(layout)
	spacer := max(1, columns.width-lipgloss.Width(left)-lipgloss.Width(right))
	return m.theme.Muted.Render(left + strings.Repeat(" ", spacer) + right)
}

func (m Model) issueListMetaHeader(layout browserLayout) string {
	columns := m.issueListColumns(layout)
	return m.issueListMetaPlain(columns, "STATUS", "PRI", "OWNER")
}

func (m Model) issueRows(layout browserLayout) []string {
	displayTree := buildIssueDisplayTree(m.issues)
	var rows []string
	for _, root := range displayTree.roots {
		if root.missingParentKey != "" {
			rows = append(rows, m.missingParentRows(displayTree, root.missingParentKey, layout)...)
			continue
		}
		rows = append(rows, m.issueTreeRows(displayTree, root.issueIndex, "", true, layout)...)
	}
	return rows
}

func buildIssueDisplayTree(issues []jira.Issue) issueDisplayTree {
	children := make(map[string][]int)
	indexByKey := make(map[string]int, len(issues))
	seen := make(map[string]bool, len(issues))
	for index, issue := range issues {
		seen[issue.Key] = true
		indexByKey[issue.Key] = index
	}
	var roots []issueDisplayRoot
	missingParents := make(map[string]missingParentGroup)
	missingParentOf := make(map[string]bool)
	seenMissingParent := make(map[string]bool)
	for index, issue := range issues {
		if issue.ParentKey != "" {
			if seen[issue.ParentKey] {
				children[issue.ParentKey] = append(children[issue.ParentKey], index)
				continue
			}
			group := missingParents[issue.ParentKey]
			group.key = issue.ParentKey
			if group.summary == "" {
				group.summary = issue.ParentSummary
			}
			group.children = append(group.children, index)
			missingParents[issue.ParentKey] = group
			missingParentOf[issue.ParentKey] = true
			if !seenMissingParent[issue.ParentKey] {
				roots = append(roots, issueDisplayRoot{missingParentKey: issue.ParentKey})
				seenMissingParent[issue.ParentKey] = true
			}
			continue
		}
		roots = append(roots, issueDisplayRoot{issueIndex: index})
	}
	return issueDisplayTree{
		issues:          issues,
		roots:           roots,
		children:        children,
		indexByKey:      indexByKey,
		missingParents:  missingParents,
		missingParentOf: missingParentOf,
	}
}

func (m Model) missingParentRows(displayTree issueDisplayTree, parentKey string, layout browserLayout) []string {
	group := displayTree.missingParents[parentKey]
	label := parentKey
	if group.summary != "" {
		label += "  " + group.summary
	}
	gutterWidth := issueTreeGutterWidth(layout)
	rows := []string{m.theme.Muted.Render(padRight("  ◇", gutterWidth) + truncate(label, max(20, layout.listWidth-gutterWidth-2)))}
	for index, child := range group.children {
		rows = append(rows, m.issueTreeRows(displayTree, child, "  ", index == len(group.children)-1, layout)...)
	}
	return rows
}

func (m Model) issueTreeRows(displayTree issueDisplayTree, index int, prefix string, last bool, layout browserLayout) []string {
	row := displayTree.issueRow(index)
	gutter := issueTreeGutter(prefix, last, index == m.selected, layout)
	label := m.renderIssueDisplayRow(row, gutter, layout)
	if index == m.selected {
		label = m.theme.Selected.Render(label)
		if detail := m.selectedIssueListDetail(row, layout); detail != "" {
			label += "\n" + m.theme.Muted.Render(padRight("", issueTreeGutterWidth(layout)+issueTypeColumnWidth+1)+detail)
		}
	}
	rows := strings.Split(label, "\n")
	children := displayTree.children[row.issue.Key]
	nextPrefix := prefix
	if prefix != "" || len(children) > 0 {
		if last {
			nextPrefix += "  "
		} else {
			nextPrefix += "│ "
		}
	}
	for childPosition, childIndex := range children {
		rows = append(rows, m.issueTreeRows(displayTree, childIndex, nextPrefix, childPosition == len(children)-1, layout)...)
	}
	return rows
}

func issueTreeGutter(prefix string, last bool, selected bool, layout browserLayout) string {
	cursor := " "
	if selected {
		cursor = "▌"
	}
	if prefix == "" {
		return padRight(cursor, issueTreeRootGutter)
	}
	connector := prefix
	if last {
		connector += "╰─"
	} else {
		connector += "├─"
	}
	return cursor + fitLeft(connector, issueTreeGutterWidth(layout)-2) + " "
}

func (t issueDisplayTree) issueRow(index int) issueDisplayRow {
	issue := t.issues[index]
	childIndexes := t.children[issue.Key]
	hiddenChildren := issue.SubtaskCount - len(childIndexes)
	if hiddenChildren < 0 {
		hiddenChildren = 0
	}
	row := issueDisplayRow{
		issue:          issue,
		index:          index,
		childCount:     len(childIndexes),
		hiddenChildren: hiddenChildren,
	}
	if parentIndex, ok := t.indexByKey[issue.ParentKey]; ok {
		parent := t.issues[parentIndex]
		row.parent = &parent
		row.parentVisible = true
	}
	if t.missingParentOf[issue.ParentKey] {
		row.parentVisible = true
	}
	return row
}

func (m Model) renderIssueDisplayRow(row issueDisplayRow, gutter string, layout browserLayout) string {
	issue := row.issue
	kind := m.issueKindSymbol(issue)
	columns := m.issueListColumns(layout)
	gutter = fitIssueTreeGutter(gutter, layout)
	kind = fitRight(kind, columns.typeWidth)
	keyText := truncate(issue.Key, columns.keyWidth)
	key := m.theme.Key.Render(fmt.Sprintf("%-*s", columns.keyWidth, keyText))
	statusText := truncate(issue.Status, columns.statusWidth)
	if statusText == "" {
		statusText = "Unknown"
	}
	status := statusStyle(m.theme, issue.Status).Render(fmt.Sprintf("%-*s", columns.statusWidth, statusText))
	priorityText := priorityBadge(issue.Priority)
	priority := priorityStyle(m.theme, issue.Priority).Render(fmt.Sprintf("%-*s", columns.priorityWidth, truncate(priorityText, columns.priorityWidth)))
	assigneeText := truncate(shortName(displayValue(issue.Assignee, "Unassigned")), columns.assigneeWidth)
	assignee := m.theme.Muted.Render(fmt.Sprintf("%-*s", columns.assigneeWidth, assigneeText))
	right := m.issueListMeta(columns, status, priority, assignee)

	leftPlain := fmt.Sprintf("%s %s %-*s  ", gutter, kind, columns.keyWidth, keyText)
	summaryWidth := max(12, columns.width-lipgloss.Width(leftPlain)-lipgloss.Width(m.issueListMetaPlain(columns, statusText, truncate(priorityText, columns.priorityWidth), assigneeText))-1)
	summary := truncate(issue.Summary, summaryWidth)
	if summary == "" {
		summary = "(no summary)"
	}
	left := fmt.Sprintf("%s %s %s  %s", m.theme.Muted.Render(gutter), m.theme.Muted.Render(kind), key, summary)
	spacer := max(1, columns.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", spacer) + right
}

func fitIssueTreeGutter(gutter string, layout browserLayout) string {
	maxWidth := issueTreeGutterWidth(layout)
	if lipgloss.Width(gutter) < issueTreeRootGutter {
		return padRight(gutter, issueTreeRootGutter)
	}
	if lipgloss.Width(gutter) > maxWidth {
		return fitRight(gutter, maxWidth)
	}
	return gutter
}

func issueTreeGutterWidth(layout browserLayout) int {
	switch {
	case layout.listWidth < 76:
		return 6
	case layout.listWidth < 90:
		return 8
	default:
		return issueTreeMaxGutter
	}
}

func (m Model) issueListColumns(layout browserLayout) issueListColumns {
	width := max(40, layout.listWidth-4)
	gutterWidth := issueTreeRootGutter
	typeWidth := issueTypeColumnWidth
	keyWidth := 12
	statusWidth := 14
	priorityWidth := 4
	assigneeWidth := 14
	showStatus := true
	showPriority := true
	showAssignee := layout.listWidth >= 96

	switch {
	case layout.listWidth < 64:
		keyWidth = 8
		statusWidth = 8
		showPriority = false
		showAssignee = false
	case layout.listWidth < 76:
		keyWidth = 10
		statusWidth = 10
		showPriority = false
		showAssignee = false
	case layout.listWidth < 90:
		keyWidth = 10
		statusWidth = 12
		showAssignee = false
	}

	rightPlain := m.issueListMetaPlain(issueListColumns{
		statusWidth:   statusWidth,
		priorityWidth: priorityWidth,
		assigneeWidth: assigneeWidth,
		showStatus:    showStatus,
		showPriority:  showPriority,
		showAssignee:  showAssignee,
	}, strings.Repeat("S", statusWidth), strings.Repeat("P", priorityWidth), strings.Repeat("O", assigneeWidth))
	leftWidth := gutterWidth + 1 + typeWidth + 1 + keyWidth + 2
	summaryWidth := max(12, width-leftWidth-lipgloss.Width(rightPlain)-1)
	return issueListColumns{
		width:         width,
		gutterWidth:   gutterWidth,
		typeWidth:     typeWidth,
		keyWidth:      keyWidth,
		statusWidth:   statusWidth,
		priorityWidth: priorityWidth,
		assigneeWidth: assigneeWidth,
		showStatus:    showStatus,
		showPriority:  showPriority,
		showAssignee:  showAssignee,
		summaryWidth:  summaryWidth,
	}
}

func (m Model) issueListMeta(columns issueListColumns, status, priority, assignee string) string {
	var parts []string
	if columns.showStatus {
		parts = append(parts, status)
	}
	if columns.showPriority {
		parts = append(parts, priority)
	}
	if columns.showAssignee {
		parts = append(parts, assignee)
	}
	return strings.Join(parts, " ")
}

func (m Model) issueListMetaPlain(columns issueListColumns, status, priority, assignee string) string {
	var parts []string
	if columns.showStatus {
		parts = append(parts, fmt.Sprintf("%-*s", columns.statusWidth, truncate(status, columns.statusWidth)))
	}
	if columns.showPriority {
		parts = append(parts, fmt.Sprintf("%-*s", columns.priorityWidth, truncate(priority, columns.priorityWidth)))
	}
	if columns.showAssignee {
		parts = append(parts, fmt.Sprintf("%-*s", columns.assigneeWidth, truncate(assignee, columns.assigneeWidth)))
	}
	return strings.Join(parts, " ")
}

func (m Model) selectedIssueListDetail(row issueDisplayRow, layout browserLayout) string {
	issue := row.issue
	width := max(40, layout.listWidth-4)
	var parts []string
	if issue.ParentKey != "" && !row.parentVisible {
		parts = append(parts, "parent "+issue.ParentKey)
	}
	if row.childCount > 0 && row.hiddenChildren > 0 {
		parts = append(parts, fmt.Sprintf("%d children", row.childCount))
	}
	if row.hiddenChildren > 0 {
		parts = append(parts, fmt.Sprintf("+%d hidden", row.hiddenChildren))
	}
	if issue.SubtaskCount > 0 && row.childCount == 0 {
		parts = append(parts, fmt.Sprintf("%d subtasks reported", issue.SubtaskCount))
	}
	if len(parts) == 0 {
		return ""
	}
	line := m.theme.Muted.Render(strings.Join(parts, "  |  "))
	return truncate(line, width)
}

func (m Model) issueKindSymbol(issue jira.Issue) string {
	symbols := m.issueSymbols()
	normalized := strings.ToLower(issue.IssueType)
	switch {
	case strings.Contains(normalized, "epic"):
		return symbols.Epic
	case issue.IsSubtask || strings.Contains(normalized, "sub-task") || strings.Contains(normalized, "subtask"):
		return symbols.Subtask
	case strings.Contains(normalized, "story"):
		return symbols.Story
	case strings.Contains(normalized, "bug"):
		return symbols.Bug
	case strings.Contains(normalized, "task"):
		return symbols.Task
	default:
		return symbols.Issue
	}
}

func (m Model) issueSymbols() issueSymbols {
	switch m.effectiveSymbolMode() {
	case symbolModePlain:
		return issueSymbols{Epic: "E", Story: "S", Task: "T", Bug: "B", Subtask: "-", Issue: "*"}
	case symbolModeEmoji:
		return issueSymbols{Epic: "🟣", Story: "🟦", Task: "🟨", Bug: "🐞", Subtask: "↳", Issue: "•"}
	case symbolModeNerd:
		return issueSymbols{Epic: "◆", Story: "󰧭", Task: "󰄬", Bug: "", Subtask: "↳", Issue: "•"}
	default:
		return issueSymbols{Epic: "◆", Story: "■", Task: "●", Bug: "!", Subtask: "↳", Issue: "•"}
	}
}

func (m Model) effectiveSymbolMode() issueSymbolMode {
	mode := m.symbolMode
	if mode == "" || mode == symbolModeAuto {
		return detectSymbolMode()
	}
	return mode
}

func detectSymbolMode() issueSymbolMode {
	term := strings.ToLower(os.Getenv("TERM"))
	lang := strings.ToLower(os.Getenv("LC_ALL") + os.Getenv("LC_CTYPE") + os.Getenv("LANG"))
	if term == "dumb" || (!strings.Contains(lang, "utf-8") && !strings.Contains(lang, "utf8")) {
		return symbolModePlain
	}
	return symbolModeSymbols
}

func (m Model) filterSummary() string {
	query := normalizeJQLForSummary(m.jql)
	var parts []string
	if project := jqlValueAfter(query, "project = "); project != "" {
		parts = append(parts, project)
	}
	switch {
	case strings.Contains(query, "assignee = currentuser()"):
		parts = append(parts, "assigned to me")
	case strings.Contains(query, "creator = currentuser()") && strings.Contains(query, "reporter = currentuser()"):
		parts = append(parts, "created/reported by me")
	case strings.Contains(query, "reporter = currentuser()"):
		parts = append(parts, "reported by me")
	case strings.Contains(query, "creator = currentuser()"):
		parts = append(parts, "created by me")
	case strings.Contains(query, "watcher = currentuser()"):
		parts = append(parts, "watching")
	}
	if strings.Contains(query, "sprint in opensprints()") {
		parts = append(parts, "current sprint")
	}
	if strings.Contains(query, "resolution = unresolved") {
		parts = append(parts, "unresolved")
	}
	if order := jqlOrderSummary(query); order != "" {
		parts = append(parts, order)
	}
	if len(parts) == 0 {
		return m.jql
	}
	return strings.Join(parts, " · ")
}

func normalizeJQLForSummary(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func jqlValueAfter(query string, marker string) string {
	index := strings.Index(query, marker)
	if index < 0 {
		return ""
	}
	value := strings.TrimSpace(query[index+len(marker):])
	if value == "" {
		return ""
	}
	end := len(value)
	for _, token := range []string{" and ", " or ", " order by ", ")"} {
		if tokenIndex := strings.Index(value, token); tokenIndex >= 0 && tokenIndex < end {
			end = tokenIndex
		}
	}
	return strings.ToUpper(strings.Trim(value[:end], " ()"))
}

func jqlOrderSummary(query string) string {
	index := strings.Index(query, " order by ")
	if index < 0 {
		return ""
	}
	order := strings.TrimSpace(query[index+len(" order by "):])
	switch {
	case strings.Contains(order, "updated desc"):
		return "updated desc"
	case strings.Contains(order, "updated asc"):
		return "updated asc"
	case strings.Contains(order, "priority desc"):
		return "priority desc"
	case order != "":
		return "sorted"
	default:
		return ""
	}
}

func priorityBadge(priority string) string {
	trimmed := strings.TrimSpace(priority)
	if trimmed == "" {
		return "P?"
	}
	rank := priorityRank(trimmed)
	switch rank {
	case 5:
		return "!!!"
	case 4:
		return "!!"
	case 3:
		return "P3"
	case 2:
		return "P4"
	case 1:
		return "P5"
	default:
		return truncate(trimmed, 6)
	}
}

func displayValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func shortName(value string) string {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return value
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + " " + string([]rune(parts[len(parts)-1])[0]) + "."
}

func (m Model) renderFullDetail(layout browserLayout) string {
	bodyWidth := max(32, layout.contentWidth-8)
	body := m.renderScrollableDetailBody(m.fullDetailContent(bodyWidth), bodyWidth)
	headerWidth := max(32, layout.contentWidth-4)
	header := m.renderDetailTitleLine(headerWidth) + "\n" +
		m.renderDetailSummaryLine(headerWidth) + "\n" +
		"\n" +
		m.renderDetailHeaderMeta(headerWidth) + "\n" +
		m.renderDetailHeaderDivider(headerWidth) + "\n" +
		m.renderDetailTabs(headerWidth)
	return m.theme.ActivePane.Width(layout.contentWidth).Render(header + "\n\n" + body)
}

func (m Model) renderCommentComposer(layout browserLayout) string {
	selected := m.issues[m.selected]
	bodyWidth := max(32, layout.contentWidth-8)
	editorRows := m.commentEditorRows()
	var b strings.Builder
	b.WriteString(m.renderCommentComposerTitle(selected, bodyWidth))
	b.WriteString("\n\n")
	if m.commentSubmitting {
		b.WriteString(m.detailSectionHeader("comment-compose", "Posting Comment", "", bodyWidth))
		b.WriteString("\n")
		b.WriteString(m.theme.Muted.Render("Posting comment..."))
	} else if m.commentConfirm {
		b.WriteString(m.detailSectionHeader("comment-compose", "Review Comment", "", bodyWidth))
		b.WriteString("\n")
		b.WriteString(m.renderCommentDraft(bodyWidth, editorRows))
		if preview := m.renderCommentLinkPreview(bodyWidth); preview != "" {
			b.WriteString("\n\n")
			b.WriteString(preview)
		}
		if preview := m.renderCommentMentionPreview(bodyWidth); preview != "" {
			b.WriteString("\n\n")
			b.WriteString(preview)
		}
		b.WriteString("\n\n")
		b.WriteString(m.theme.Warning.Render("Post this comment to " + selected.Key + "?"))
	} else {
		b.WriteString(m.detailSectionHeader("comment-compose", "Add Comment", "", bodyWidth))
		b.WriteString("\n")
		b.WriteString(m.renderCommentDraft(bodyWidth, editorRows))
		if preview := m.renderCommentLinkPreview(bodyWidth); preview != "" {
			b.WriteString("\n\n")
			b.WriteString(preview)
		}
		if preview := m.renderCommentMentionPreview(bodyWidth); preview != "" {
			b.WriteString("\n\n")
			b.WriteString(preview)
		}
		if picker := m.renderMentionPicker(bodyWidth); picker != "" {
			b.WriteString("\n\n")
			b.WriteString(picker)
		}
	}
	if m.detailNotice != "" {
		b.WriteString("\n\n")
		b.WriteString(m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.theme.ActivePane.Width(layout.contentWidth).Render(b.String())
}

func (m Model) renderCommentComposerTitle(issue jira.Issue, width int) string {
	title := issue.Key
	if issue.Summary != "" {
		title += "  " + issue.Summary
	}
	return m.theme.Key.Render(truncate(title, width))
}

func (m Model) renderCommentDraft(width int, rows int) string {
	rows = max(1, rows)
	editor := m.configuredCommentEditor(width, rows)
	lineCount := editor.LineCount()
	if lineCount > rows && rows > 1 {
		editor.SetHeight(rows - 1)
		start := editor.ScrollYOffset() + 1
		end := min(lineCount, editor.ScrollYOffset()+editor.Height())
		indicator := fmt.Sprintf("Lines %d-%d of %d", start, end, lineCount)
		if start > 1 {
			indicator += "  PgUp previous"
		}
		if end < lineCount {
			indicator += "  PgDn next"
		}
		body := editor.View() + "\n" + m.theme.Muted.Render(truncate(indicator, width))
		return m.theme.Input.Width(width).Height(rows).Render(body)
	}
	return m.theme.Input.Width(width).Height(rows).Render(editor.View())
}

func (m Model) renderCommentLinkPreview(width int) string {
	links := collectDetailLinks(m.commentEditorValue())
	if len(links) == 0 {
		return ""
	}

	limit := min(3, len(links))
	lines := make([]string, 0, limit+2)
	lines = append(lines, m.detailSectionHeader("comment-links", "Detected Links", "", width))
	for index := 0; index < limit; index++ {
		link := links[index]
		prefix := fmt.Sprintf("[%d] %-5s ", index+1, link.Kind)
		lines = append(lines,
			m.theme.Key.Render(fmt.Sprintf("[%d]", index+1))+" "+
				m.theme.Muted.Render(fmt.Sprintf("%-5s", link.Kind))+" "+
				m.theme.Text.Render(truncate(linkDisplayText(link), max(12, width-lipgloss.Width(prefix)))),
		)
	}
	if len(links) > limit {
		lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("+%d more", len(links)-limit)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) commentLinkPreviewRows() int {
	links := collectDetailLinks(m.commentEditorValue())
	if len(links) == 0 {
		return 0
	}
	rows := 2 + min(3, len(links))
	if len(links) > 3 {
		rows++
	}
	return rows
}

func (m Model) renderCommentMentionPreview(width int) string {
	mentions := m.unresolvedCommentMentions()
	if len(mentions) == 0 {
		return ""
	}

	limit := min(3, len(mentions))
	lines := make([]string, 0, limit+2)
	lines = append(lines, m.detailSectionHeader("comment-mentions", "Unresolved Mentions", "", width))
	for index := 0; index < limit; index++ {
		mention := "@" + mentions[index].Query
		lines = append(lines, m.theme.Warning.Render(truncate(mention, width)))
	}
	if len(mentions) > limit {
		lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("+%d more", len(mentions)-limit)))
	}
	lines = append(lines, m.theme.Muted.Render("Use @ to select Jira users so mentions notify the right account."))
	return strings.Join(lines, "\n")
}

func (m Model) commentMentionPreviewRows() int {
	mentions := m.unresolvedCommentMentions()
	if len(mentions) == 0 {
		return 0
	}
	rows := 3 + min(3, len(mentions))
	if len(mentions) > 3 {
		rows++
	}
	return rows
}

func (m Model) unresolvedCommentMentions() []mentiondetect.Mention {
	detected := mentiondetect.Detect(m.commentEditorValue())
	if len(detected) == 0 {
		return nil
	}
	unresolved := make([]mentiondetect.Mention, 0, len(detected))
	for _, mention := range detected {
		if !m.isResolvedMention("@" + mention.Query) {
			unresolved = append(unresolved, mention)
		}
	}
	return unresolved
}

func (m Model) isResolvedMention(value string) bool {
	for _, mention := range m.commentMentions {
		if strings.HasPrefix(mention.Text, value) && strings.Contains(m.commentEditorValue(), mention.Text) {
			return true
		}
	}
	return false
}

func (m Model) mentionPickerRows() int {
	if !m.mentionPickerOpen {
		return 0
	}
	return 14
}

func (m Model) renderMentionPicker(width int) string {
	if !m.mentionPickerOpen {
		return ""
	}
	lines := []string{
		m.detailSectionHeader("mention-picker", "Mention User", "type to search Jira users", width),
		m.theme.Muted.Render("Filter: ") + m.theme.Text.Render(m.mentionQuery),
	}
	if m.mentionSearchLoading {
		lines = append(lines, m.theme.Muted.Render("Searching Jira users..."))
	} else if m.mentionSearchErr != nil {
		lines = append(lines, m.theme.Error.Render(truncate("User search failed: "+m.mentionSearchErr.Error(), width)))
	} else if strings.TrimSpace(m.mentionQuery) == "" {
		lines = append(lines, m.theme.Muted.Render("Start typing a name or email."))
	} else if len(m.mentionUsers) == 0 {
		lines = append(lines, m.theme.Muted.Render("No Jira users matched."))
	}
	lines = append(lines, m.renderMentionUsers(width)...)
	return m.theme.Panel.Width(width).Render(strings.TrimRight(strings.Join(lines, "\n"), "\n"))
}

func (m Model) renderMentionUsers(width int) []string {
	if len(m.mentionUsers) == 0 {
		return nil
	}
	rows := max(1, m.mentionPickerRows()-4)
	cursor := clamp(m.mentionCursor, 0, len(m.mentionUsers)-1)
	start := clamp(cursor-rows+1, 0, max(0, len(m.mentionUsers)-rows))
	if cursor < start {
		start = cursor
	}
	end := min(len(m.mentionUsers), start+rows)
	lines := make([]string, 0, rows+1)
	for index := start; index < end; index++ {
		user := m.mentionUsers[index]
		prefix := "  "
		style := m.theme.Text
		if index == cursor {
			prefix = "> "
			style = m.theme.Selected
		}
		name := truncate(m.mentionDisplayName(user), max(10, width-8))
		lines = append(lines, style.Render(prefix+name))
	}
	if len(m.mentionUsers) > rows {
		lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("%d-%d of %d", start+1, end, len(m.mentionUsers))))
	}
	return lines
}

func (m Model) fullDetailContent(bodyWidth int) string {
	ctx, ok := m.detailRenderContext()
	if !ok {
		return m.detailEmptyState("No issue selected.", bodyWidth)
	}
	sections := m.detailSections()
	if len(sections) == 0 {
		return m.detailEmptyState("No detail sections available.", bodyWidth)
	}
	focus := clamp(m.detailFocus, 0, len(sections)-1)
	var b strings.Builder
	b.WriteString(m.renderDetailSection(sections[focus], ctx, bodyWidth))
	b.WriteString("\n\n")
	if m.detailNotice != "" {
		b.WriteString(m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return b.String()
}

func (m Model) detailRenderContext() (detailRenderContext, bool) {
	selected, ok := m.selectedIssue()
	if !ok {
		return detailRenderContext{}, false
	}
	detail, hasDetail := m.details[selected.Key]
	display := m.displayIssueForDetail(selected, detail, hasDetail)
	description := ""
	var links []detailLink
	if hasDetail {
		description = detail.Description
		if strings.TrimSpace(description) == "" {
			description = "No description."
		}
		links = collectDetailLinks(description)
	}
	return detailRenderContext{
		selected:    selected,
		display:     display,
		detail:      detail,
		hasDetail:   hasDetail,
		description: description,
		links:       links,
	}, true
}

func (m Model) renderDetailSection(section detailSection, ctx detailRenderContext, width int) string {
	switch section.ID {
	case "description":
		if ctx.hasDetail {
			return m.renderDescription(ctx.description, width)
		}
		if m.detailLoading && m.detailRequestKey == ctx.selected.Key {
			return m.renderDescriptionState("Loading issue detail...", width, false)
		}
		if m.detailErr != nil && m.detailRequestKey == ctx.selected.Key {
			return m.renderDescriptionState("Detail failed: "+m.detailErr.Error(), width, true)
		}
		return m.renderDescriptionState("Description not loaded.", width, false)
	case "links":
		return m.renderLinksSection(ctx.links, width)
	case "hierarchy":
		return m.renderHierarchySection(ctx.display, width)
	case "comments":
		return m.renderComments(ctx.display.Key, width)
	case "actions":
		return m.renderActionsSection(width)
	default:
		return m.detailSectionHeader(section.ID, section.Label, "", width) + "\n" + m.detailEmptyState("Section not available.", width)
	}
}

func (m Model) renderDetailIdentity(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return m.theme.Muted.Render("No issue selected")
	}
	detail, hasDetail := m.details[selected.Key]
	display := m.displayIssueForDetail(selected, detail, hasDetail)
	status := statusStyle(m.theme, display.Status).Render(displayValue(display.Status, "Unknown"))
	return m.theme.Key.Render(display.Key) + " " +
		status + " " +
		m.theme.Muted.Render(displayValue(display.IssueType, "Unknown"))
}

func (m Model) renderDetailTitleLine(width int) string {
	return m.renderDetailIdentity(width)
}

func (m Model) renderDetailSummaryLine(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	detail, hasDetail := m.details[selected.Key]
	display := m.displayIssueForDetail(selected, detail, hasDetail)
	return m.theme.Text.Render(truncate(displayValue(display.Summary, "No summary"), max(12, width)))
}

func (m Model) renderDetailHeaderMeta(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	detail, hasDetail := m.details[selected.Key]
	display := m.displayIssueForDetail(selected, detail, hasDetail)
	updated := "Unknown"
	if hasDetail {
		updated = formatTime(detail.Updated)
	}
	parts := []string{
		m.detailMetaPart("Assignee", shortName(displayValue(display.Assignee, "Unassigned"))),
		m.detailMetaPart("Priority", displayValue(display.Priority, "Unknown")),
		m.detailMetaPart("Updated", updated),
	}
	if hasDetail && strings.TrimSpace(detail.Reporter) != "" && detail.Reporter != "Unknown" {
		parts = append(parts, m.detailMetaPart("Reporter", shortName(detail.Reporter)))
	}
	separator := m.theme.Muted.Render("  |  ")
	for len(parts) > 0 {
		line := strings.Join(parts, separator)
		if lipgloss.Width(line) <= width {
			return line
		}
		parts = parts[:len(parts)-1]
	}
	return ""
}

func (m Model) renderDetailHeaderDivider(width int) string {
	if width <= 0 {
		return ""
	}
	return m.theme.Muted.Render(strings.Repeat("─", width))
}

func (m Model) detailMetaPart(label string, value string) string {
	return m.theme.Muted.Render(label+": ") + m.theme.Text.Render(value)
}

func (m Model) displayIssueForDetail(selected jira.Issue, detail jira.IssueDetail, hasDetail bool) jira.Issue {
	if !hasDetail {
		return selected
	}
	display := detail.Issue
	if display.Key == "" {
		display.Key = selected.Key
	}
	if jira.IsPrivacyUserAlias(display.Assignee) && !jira.IsPrivacyUserAlias(selected.Assignee) {
		display.Assignee = selected.Assignee
	}
	return display
}

func (m Model) detailSections() []detailSection {
	sections := []detailSection{
		{ID: "description", Label: "Description", Short: "Desc"},
		{ID: "hierarchy", Label: "Hierarchy", Short: "Tree"},
		{ID: "comments", Label: "Comments", Short: "Com"},
		{ID: "actions", Label: "Actions", Short: "Act"},
	}
	if selected, ok := m.selectedIssue(); ok {
		display := selected
		description := ""
		if detail, hasDetail := m.details[selected.Key]; hasDetail {
			display = detail.Issue
			if display.Key == "" {
				display.Key = selected.Key
			}
			description = detail.Description
		}
		if childCount := len(m.hierarchyRows(display.Key)); childCount > 0 {
			sections[1].Badge = fmt.Sprintf("%d", childCount)
		}
		if comments, loaded := m.comments[display.Key]; loaded {
			sections[2].Badge = fmt.Sprintf("%d", len(comments))
		} else if m.commentsLoading && m.commentsRequestKey == display.Key {
			sections[2].Badge = "..."
		} else if m.commentsErr != nil && m.commentsRequestKey == display.Key {
			sections[2].Badge = "!"
		}
		if linkCount := len(collectDetailLinks(description)); linkCount > 0 {
			links := detailSection{ID: "links", Label: "Links", Short: "Links", Badge: fmt.Sprintf("%d", linkCount)}
			sections = append(sections[:2], append([]detailSection{links}, sections[2:]...)...)
		}
	}
	return sections
}

func (m Model) detailTabs() []string {
	sections := m.detailSections()
	tabs := make([]string, 0, len(sections))
	for _, section := range sections {
		tabs = append(tabs, section.Label)
	}
	return tabs
}

func (m Model) detailSectionTitle(id string, fallback string, help string) string {
	return m.detailSectionHeader(id, fallback, help, 0)
}

func (m Model) detailSectionHeader(id string, fallback string, help string, width int) string {
	title := fallback
	badge := ""
	if section, ok := m.detailSection(id); ok {
		title = section.Label
		badge = section.Badge
	}
	leftStyle := m.theme.PaneTitle
	left := leftStyle.Render(title)
	if badge != "" {
		left += m.theme.Muted.Render(" " + badge)
	}
	right := ""
	if help != "" {
		right = m.theme.Muted.Render(help)
	}
	if width <= 0 {
		if right == "" {
			return left
		}
		return left + " " + right
	}
	ruleWidth := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if ruleWidth < 3 {
		if right == "" {
			return left
		}
		return left + " " + right
	}
	rule := m.theme.Muted.Render(strings.Repeat("─", ruleWidth))
	if right == "" {
		return left + " " + rule
	}
	return left + " " + rule + " " + right
}

func (m Model) detailSection(id string) (detailSection, bool) {
	for _, section := range m.detailSections() {
		if section.ID == id {
			return section, true
		}
	}
	return detailSection{}, false
}

func (m Model) renderDetailTabs(width int) string {
	sections := m.detailSections()
	tabs := m.detailTabsLine(sections, width, false)
	line := tabs
	if lipgloss.Width(line) <= width {
		return line
	}
	tabs = m.detailTabsLine(sections, width, true)
	line = tabs
	if lipgloss.Width(line) <= width {
		return line
	}
	return m.detailTabsWrapped(sections, max(8, width))
}

func (m Model) detailTabsLine(sections []detailSection, width int, compact bool) string {
	parts := make([]string, 0, len(sections))
	focus := clamp(m.detailFocus, 0, max(0, len(sections)-1))
	for index, section := range sections {
		label := section.Label
		if compact {
			label = section.Short
		}
		if section.Badge != "" {
			label += " " + section.Badge
		}
		if index == focus {
			parts = append(parts, m.theme.TabActive.Render(label))
		} else {
			parts = append(parts, m.theme.TabInactive.Render(label))
		}
	}
	return strings.Join(parts, m.theme.Muted.Render(" "))
}

func (m Model) detailTabsWrapped(sections []detailSection, width int) string {
	if len(sections) == 0 {
		return ""
	}
	separator := m.theme.Muted.Render(" ")
	var rows []string
	var current string
	focus := clamp(m.detailFocus, 0, max(0, len(sections)-1))
	for index, section := range sections {
		label := section.Short
		if section.Badge != "" {
			label += " " + section.Badge
		}
		part := m.theme.TabInactive.Render(label)
		if index == focus {
			part = m.theme.TabActive.Render(label)
		}
		candidate := part
		if current != "" {
			candidate = current + separator + part
		}
		if current != "" && lipgloss.Width(candidate) > width {
			rows = append(rows, current)
			current = part
			continue
		}
		current = candidate
	}
	if current != "" {
		rows = append(rows, current)
	}
	return strings.Join(rows, "\n")
}

func (m *Model) moveDetailFocus(delta int) {
	sections := m.detailSections()
	if len(sections) == 0 {
		m.detailFocus = 0
		return
	}
	m.saveDetailSectionOffset()
	m.detailFocus = (m.detailFocus + delta + len(sections)) % len(sections)
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.restoreDetailSectionOffset()
}

func (m *Model) activateDetailFocus() {
	sections := m.detailSections()
	if len(sections) == 0 {
		return
	}
	section := sections[clamp(m.detailFocus, 0, len(sections)-1)]
	switch section.ID {
	case "actions":
		m.focusActions()
	case "hierarchy":
		m.focusHierarchy()
	case "links":
		m.focusDetailLinks()
	default:
		m.linkFocus = false
		m.hierarchyFocus = false
		m.actionFocus = false
		m.jumpDetailSection(section.Label)
	}
}

func (m Model) renderIssueTitle(issue jira.Issue, width int) string {
	meta := m.theme.Key.Render(issue.Key) + "  " + statusStyle(m.theme, issue.Status).Render(displayValue(issue.Status, "Unknown")) + "  " + m.theme.Muted.Render(displayValue(issue.IssueType, "Unknown"))
	summary := wrapText(issue.Summary, width)
	if strings.TrimSpace(summary) == "" {
		summary = "No summary"
	}
	return meta + "\n" + m.theme.Text.Bold(true).Render(wrapText(summary, width))
}

func (m Model) renderDescription(description string, width int) string {
	width = max(24, width)
	return m.detailSectionHeader("description", "Description", "", width) + "\n" + m.renderRichDescriptionBody(wrapRichText(description, width), width)
}

func (m Model) renderDescriptionState(message string, width int, isError bool) string {
	state := m.detailEmptyState(message, width)
	if isError {
		state = m.theme.Error.Render(wrapText(message, max(12, width)))
	}
	return m.detailSectionHeader("description", "Description", "", width) + "\n" + state
}

func (m Model) renderComments(key string, width int) string {
	lines := []string{m.detailSectionHeader("comments", "Comments", "", width)}
	if comments, ok := m.comments[key]; ok {
		if len(comments) == 0 {
			lines = append(lines, m.detailEmptyState("No comments yet.", width))
			return strings.Join(lines, "\n")
		}
		for index, comment := range comments {
			if index > 0 {
				lines = append(lines, "")
			}
			body := comment.Body
			if strings.TrimSpace(body) == "" {
				body = "No comment body."
			}
			lines = append(lines, m.renderCommentBlock(index+1, len(comments), comment.Author, formatTime(comment.Created), body, width))
		}
		if len(comments) >= maxComments {
			lines = append(lines, "")
			lines = append(lines, m.detailEmptyState(fmt.Sprintf("Showing latest %d comments.", maxComments), width))
		}
		return strings.Join(lines, "\n")
	}
	if m.commentsLoading && m.commentsRequestKey == key {
		lines = append(lines, m.detailEmptyState("Loading comments...", width))
		return strings.Join(lines, "\n")
	}
	if m.commentsErr != nil && m.commentsRequestKey == key {
		lines = append(lines, m.theme.Error.Render(wrapText("Comments failed: "+m.commentsErr.Error(), width)))
		return strings.Join(lines, "\n")
	}
	lines = append(lines, m.detailEmptyState("Comments not loaded.", width))
	return strings.Join(lines, "\n")
}

func (m Model) renderCommentBlock(index int, total int, author string, created string, body string, width int) string {
	contentWidth := max(20, width-4)
	header := m.theme.Key.Render(fmt.Sprintf("Comment %d/%d", index, max(1, total))) +
		m.theme.Muted.Render("  "+displayValue(author, "Unknown")+"  "+created)
	bodyWidth := max(12, contentWidth-2)
	renderedBody := m.renderRichDescriptionBody(wrapRichText(body, bodyWidth), bodyWidth)
	renderedBody = indentLines(renderedBody, "  ")
	return m.theme.CommentBlock.Width(contentWidth + 2).Render(header + "\n" + renderedBody)
}

func (m Model) detailEmptyState(message string, width int) string {
	return m.theme.Muted.Render("  " + truncate(message, max(12, width-2)))
}

func (m Model) detailTable(width int, headers []string, rows [][]string, style func(row, col int) lipgloss.Style) string {
	table := lipglosstable.New().
		Border(lipgloss.HiddenBorder()).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == lipglosstable.HeaderRow {
				return m.theme.Muted
			}
			if style != nil {
				return style(row, col)
			}
			return m.theme.Text
		})
	if width > 0 {
		table = table.Width(width)
	}
	if len(headers) > 0 {
		table = table.Headers(headers...)
	}
	return table.String()
}

func (m Model) renderDetailNotice(message string, width int) string {
	contentWidth := max(20, width-4)
	return m.theme.NoticeBlock.Width(contentWidth + 2).Render(m.theme.Muted.Render("Notice") + "\n" + m.theme.Text.Render(wrapText(message, contentWidth)))
}

func (m Model) renderHierarchySection(issue jira.Issue, width int) string {
	rows := m.hierarchyRows(issue.Key)
	children, subtasks := splitHierarchyRows(rows)
	var lines []string
	help := ""
	switch {
	case m.hierarchyFocus:
		help = "j/k select  enter open  esc leave"
	case len(rows) > 0:
		help = "enter focus"
	}
	lines = append(lines, m.detailSectionHeader("hierarchy", "Hierarchy", help, width))
	pathRel := "Current"
	pathIssue := issue.Key
	if issue.ParentKey != "" {
		pathRel = "Parent"
		pathIssue = issue.ParentKey
		if issue.ParentSummary != "" {
			pathIssue += "  " + issue.ParentSummary
		}
	} else if issue.Summary != "" {
		pathIssue += "  " + issue.Summary
	}
	lines = append(lines, m.renderHierarchyPath(pathRel, pathIssue, width))
	if len(rows) == 0 {
		if issue.ParentKey == "" {
			lines = append(lines, m.detailEmptyState("No parent or child issues in the current result.", width))
		}
		lines = append(lines, "")
		lines = append(lines, m.renderLinkedIssuesPlaceholder(width))
		return strings.Join(lines, "\n")
	}
	if len(children) > 0 {
		lines = append(lines, "")
		lines = append(lines, m.renderHierarchyGroup("Children", children, width))
	}
	if len(subtasks) > 0 {
		lines = append(lines, "")
		lines = append(lines, m.renderHierarchyGroup("Subtasks", subtasks, width))
	}
	lines = append(lines, "")
	lines = append(lines, m.renderLinkedIssuesPlaceholder(width))
	return strings.Join(lines, "\n")
}

func (m Model) renderHierarchyGroup(label string, groupRows []hierarchyRow, width int) string {
	lines := []string{m.theme.Muted.Render(fmt.Sprintf("%s %d", label, len(groupRows)))}
	tableRows := make([][]string, 0, len(groupRows))
	cursor := clamp(m.selectedHierarchy, 0, max(0, len(m.currentHierarchyChildren())-1))
	for _, row := range groupRows {
		child := row.Issue
		key := child.Key
		selected := row.Index == cursor
		if selected {
			key = "> " + key
		} else {
			key = "  " + key
		}
		tableRows = append(tableRows, []string{
			key,
			truncate(child.Summary, max(12, width-52)),
			displayValue(child.Status, "Unknown"),
			truncate(priorityBadge(child.Priority), 4),
			truncate(shortName(displayValue(child.Assignee, "Unassigned")), 14),
		})
	}
	lines = append(lines, m.detailTable(0, []string{"KEY", "SUMMARY", "STATUS", "PRI", "OWNER"}, tableRows, func(row, col int) lipgloss.Style {
		if col == 0 {
			if m.hierarchyFocus && row >= 0 && row < len(groupRows) && groupRows[row].Index == cursor {
				return m.theme.Selected
			}
			return m.theme.Key
		}
		return m.theme.Text
	}))
	return strings.Join(lines, "\n")
}

func (m Model) renderHierarchyPath(rel string, issue string, width int) string {
	label := m.theme.Muted.Render("Path")
	relation := m.theme.Muted.Render(rel + ": ")
	return label + "\n" + relation + m.theme.Text.Render(truncate(issue, max(12, width-lipgloss.Width(rel)-8)))
}

func (m Model) renderLinkedIssuesPlaceholder(width int) string {
	return m.theme.Muted.Render("Linked Issues") + "\n" + m.detailEmptyState("Linked issue data is not loaded yet.", width)
}

func (m Model) renderActionsSection(width int) string {
	actions := m.detailActions()
	lines := make([]string, 0, len(actions)+2)
	help := ""
	if m.actionFocus {
		help = "j/k select  enter run  esc leave"
	}
	lines = append(lines, m.detailSectionHeader("actions", "Actions", help, width))
	cursor := clamp(m.selectedAction, 0, max(0, len(actions)-1))
	rows := make([][]string, 0, len(actions))
	for index, action := range actions {
		marker := " "
		labelStyle := m.theme.Text
		stateStyle := m.theme.Success
		descStyle := m.theme.Muted
		state := "ready"
		if m.actionFocus && index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
		} else if !action.Enabled {
			labelStyle = m.theme.Muted
			stateStyle = m.theme.Muted
			descStyle = m.theme.Muted
			state = "metadata"
		}
		rows = append(rows, []string{
			labelStyle.Render(marker),
			labelStyle.Render(action.Label),
			stateStyle.Render(state),
			descStyle.Render(truncate(action.Description, max(16, width-46))),
		})
	}
	lines = append(lines, m.detailTable(0, []string{"", "ACTION", "STATE", "DETAIL"}, rows, nil))
	return strings.Join(lines, "\n")
}

func (m Model) detailActions() []detailAction {
	return []detailAction{
		{ID: "comment", Label: "Add Comment", Description: "Write a Jira comment.", Enabled: true},
		{ID: "browser", Label: "Open In Browser", Description: "Open this ticket in Jira.", Enabled: true},
		{ID: "copy-key", Label: "Copy Key", Description: "Copy the ticket key.", Enabled: true},
		{ID: "copy-url", Label: "Copy URL", Description: "Copy the Jira URL.", Enabled: true},
		{ID: "edit-fields", Label: "Edit Fields", Description: "Will use Jira edit metadata before rendering fields.", Enabled: false},
		{ID: "transition", Label: "Transition Status", Description: "Will load available Jira transitions and fields.", Enabled: false},
		{ID: "assign", Label: "Assign", Description: "Will use Jira assignable-user lookup.", Enabled: false},
		{ID: "subtask", Label: "Create Subtask", Description: "Will use Jira create metadata for required fields.", Enabled: false},
	}
}

func (m *Model) focusActions() {
	actions := m.detailActions()
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = true
	m.selectedAction = clamp(m.selectedAction, 0, max(0, len(actions)-1))
	m.jumpDetailSection("Actions")
}

func (m *Model) moveSelectedDetailAction(delta int) {
	actions := m.detailActions()
	if len(actions) == 0 {
		m.selectedAction = 0
		return
	}
	m.selectedAction = clamp(m.selectedAction+delta, 0, len(actions)-1)
}

func (m Model) runSelectedDetailAction() (Model, tea.Cmd) {
	actions := m.detailActions()
	if len(actions) == 0 {
		return m, nil
	}
	action := actions[clamp(m.selectedAction, 0, len(actions)-1)]
	if !action.Enabled {
		m.detailNotice = action.Label + " needs Jira metadata before it can be edited safely."
		return m, nil
	}
	switch action.ID {
	case "comment":
		m.startCommentComposer()
		return m, nil
	case "browser":
		return m.openSelectedIssue()
	case "copy-key":
		return m.copySelectedIssueKey()
	case "copy-url":
		return m.copySelectedIssueURL()
	default:
		return m, nil
	}
}

func (m *Model) focusHierarchy() {
	children := m.currentHierarchyChildren()
	m.linkFocus = false
	m.hierarchyFocus = len(children) > 0
	m.selectedHierarchy = clamp(m.selectedHierarchy, 0, max(0, len(children)-1))
	m.jumpDetailSection("Hierarchy")
	if len(children) == 0 {
		m.detailNotice = "No child issues are available for this ticket."
	}
}

func (m *Model) moveSelectedHierarchyIssue(delta int) {
	children := m.currentHierarchyChildren()
	if len(children) == 0 {
		m.selectedHierarchy = 0
		return
	}
	m.selectedHierarchy = clamp(m.selectedHierarchy+delta, 0, len(children)-1)
}

func (m Model) canMoveHierarchySelection() bool {
	if m.mode != modeDetail {
		return false
	}
	section, ok := m.focusedDetailSection()
	if !ok || section.ID != "hierarchy" {
		return false
	}
	return len(m.currentHierarchyChildren()) > 0
}

func (m Model) currentHierarchyChildren() []jira.Issue {
	selected, ok := m.selectedIssue()
	if !ok {
		return nil
	}
	display := selected
	if detail, hasDetail := m.details[selected.Key]; hasDetail {
		display = detail.Issue
	}
	rows := m.hierarchyRows(display.Key)
	children := make([]jira.Issue, 0, len(rows))
	for _, row := range rows {
		children = append(children, row.Issue)
	}
	return children
}

func (m Model) openSelectedHierarchyIssue() (Model, tea.Cmd) {
	children := m.currentHierarchyChildren()
	if len(children) == 0 {
		return m, nil
	}
	child := children[clamp(m.selectedHierarchy, 0, len(children)-1)]
	for index, issue := range m.issues {
		if issue.Key == child.Key {
			m.detailBackStack = append(m.detailBackStack, m.selected)
			m.selected = index
			m.resetDetailScroll()
			m.detailFocus = 0
			m.hierarchyFocus = false
			m.linkFocus = false
			m.detailNotice = ""
			return m.startDetailRequestForSelected()
		}
	}
	return m, nil
}

func (m Model) popDetailBackStack() (Model, tea.Cmd) {
	if len(m.detailBackStack) == 0 {
		return m, nil
	}
	previous := m.detailBackStack[len(m.detailBackStack)-1]
	m.detailBackStack = m.detailBackStack[:len(m.detailBackStack)-1]
	m.selected = clamp(previous, 0, max(0, len(m.issues)-1))
	m.resetDetailScroll()
	m.detailFocus = 0
	m.hierarchyFocus = false
	m.linkFocus = false
	m.detailNotice = ""
	return m.startDetailRequestForSelected()
}

func (m Model) detailChildren(parentKey string) []jira.Issue {
	children := make([]jira.Issue, 0)
	for _, issue := range m.issues {
		if issue.ParentKey == parentKey {
			children = append(children, issue)
		}
	}
	return children
}

func (m Model) hierarchyRows(parentKey string) []hierarchyRow {
	related := m.detailChildren(parentKey)
	children := make([]jira.Issue, 0, len(related))
	subtasks := make([]jira.Issue, 0, len(related))
	for _, issue := range related {
		if isSubtaskIssue(issue) {
			subtasks = append(subtasks, issue)
			continue
		}
		children = append(children, issue)
	}
	rows := make([]hierarchyRow, 0, len(related))
	for _, issue := range children {
		rows = append(rows, hierarchyRow{
			Issue: issue,
			Group: "children",
			Index: len(rows),
		})
	}
	for _, issue := range subtasks {
		rows = append(rows, hierarchyRow{
			Issue: issue,
			Group: "subtasks",
			Index: len(rows),
		})
	}
	return rows
}

func splitHierarchyRows(rows []hierarchyRow) ([]hierarchyRow, []hierarchyRow) {
	children := make([]hierarchyRow, 0, len(rows))
	subtasks := make([]hierarchyRow, 0, len(rows))
	for _, row := range rows {
		if row.Group == "subtasks" {
			subtasks = append(subtasks, row)
			continue
		}
		children = append(children, row)
	}
	return children, subtasks
}

func isSubtaskIssue(issue jira.Issue) bool {
	issueType := strings.ToLower(strings.TrimSpace(issue.IssueType))
	return issueType == "subtask" || issueType == "sub-task"
}

func (m Model) renderLinksSection(links []detailLink, width int) string {
	lines := make([]string, 0, len(links)+1)
	lines = append(lines, m.detailSectionHeader("links", "Links", "", width))
	rows := make([][]string, 0, len(links))
	for index, link := range links {
		display := linkDisplayText(link)
		targetWidth := max(16, width-18)
		key := fmt.Sprintf("[%d]", index+1)
		keyStyle := m.theme.Key
		kindStyle := m.theme.Muted
		targetStyle := m.theme.Text
		if m.linkFocus && index == clamp(m.selectedLink, 0, len(links)-1) {
			key = "> " + key
			keyStyle = m.theme.Selected
			targetStyle = m.theme.Selected
		} else {
			key = "  " + key
		}
		rows = append(rows, []string{
			keyStyle.Render(key),
			kindStyle.Render(link.Kind),
			targetStyle.Render(truncate(display, targetWidth)),
		})
	}
	lines = append(lines, m.detailTable(0, []string{"", "TYPE", "TARGET"}, rows, nil))
	return strings.Join(lines, "\n")
}

func linkDisplayText(link detailLink) string {
	if link.Kind == linkdetect.KindEmail {
		if address := linkdetect.MailtoAddress(link.Target); address != "" {
			return address
		}
	}
	if link.Label != "" {
		return link.Label
	}
	return link.Target
}

func linkCopyText(link detailLink) string {
	if link.Kind == linkdetect.KindEmail {
		return linkDisplayText(link)
	}
	return link.Target
}

func collectDetailLinks(value string) []detailLink {
	return linkdetect.Detect(value)
}

func (m Model) renderRichDescriptionBody(value string, width int) string {
	source := strings.Split(value, "\n")
	lines := make([]string, 0, len(source))
	inCodeBlock := false
	inTable := false
	var codeLines []string
	var tableLines []string
	for _, line := range source {
		if line == "[table]" || line == "[/table]" {
			if line == "[table]" {
				inTable = true
				tableLines = nil
			} else {
				inTable = false
				lines = append(lines, m.renderTableBlock(tableLines, width))
				tableLines = nil
			}
			continue
		}
		if strings.TrimSpace(line) == "```" {
			if inCodeBlock {
				inCodeBlock = false
				lines = appendCodeBlock(lines, m.renderCodeBlockLines(codeLines, width))
				codeLines = nil
			} else {
				inCodeBlock = true
				codeLines = nil
			}
			continue
		}
		if inTable {
			tableLines = append(tableLines, line)
			continue
		}
		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}
		lines = append(lines, renderInlineCode(m.theme, line))
	}
	if inCodeBlock && len(codeLines) > 0 {
		lines = appendCodeBlock(lines, m.renderCodeBlockLines(codeLines, width))
	}
	if inTable && len(tableLines) > 0 {
		lines = append(lines, m.renderTableBlock(tableLines, width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderTableBlock(tableLines []string, width int) string {
	rows := parseTableRows(tableLines)
	if len(rows) == 0 {
		return ""
	}
	headers := rows[0]
	body := rows[1:]
	tableWidth := max(24, width)
	t := lipglosstable.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(m.theme.Muted).
		Headers(headers...).
		Rows(body...).
		Width(tableWidth).
		StyleFunc(func(row, _ int) lipgloss.Style {
			switch {
			case row == lipglosstable.HeaderRow:
				return m.theme.FieldLabel.Padding(0, 1)
			case row%2 == 0:
				return m.theme.Text.Padding(0, 1)
			default:
				return m.theme.Muted.Padding(0, 1)
			}
		})
	rendered := t.String()
	if rendered == "" {
		return ""
	}
	return rendered
}

func (m Model) renderCodeBlockLines(lines []string, width int) string {
	lines = trimBlankCodeLines(lines)
	if len(lines) == 0 {
		return ""
	}
	blockWidth := max(12, width)
	contentWidth := max(1, blockWidth-4)
	rule := "+" + strings.Repeat("-", blockWidth-2) + "+"
	rendered := make([]string, 0, len(lines)+2)
	rendered = append(rendered, m.theme.Muted.Render(rule))
	for _, line := range lines {
		line = truncate(line, contentWidth)
		padded := line + strings.Repeat(" ", contentWidth-len(line))
		rendered = append(rendered, m.theme.Muted.Render("| ")+m.theme.CodeBlock.Width(contentWidth).Render(padded)+m.theme.Muted.Render(" |"))
	}
	rendered = append(rendered, m.theme.Muted.Render(rule))
	return strings.Join(rendered, "\n")
}

func appendCodeBlock(lines []string, block string) []string {
	if block == "" {
		return trimTrailingBlankLines(lines)
	}
	lines = trimTrailingBlankLines(lines)
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	return append(lines, block)
}

func trimTrailingBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func trimBlankCodeLines(lines []string) []string {
	start := 0
	end := len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

func renderTableLine(theme ui.Theme, line string) string {
	if strings.TrimSpace(line) == "" {
		return ""
	}
	if isTableSeparator(line) {
		return theme.Muted.Render(line)
	}
	cells := strings.Split(line, "|")
	for index, cell := range cells {
		if index == 0 || index == len(cells)-1 {
			continue
		}
		cells[index] = renderInlineCode(theme, cell)
	}
	return theme.Muted.Render("|") + strings.Join(cells[1:len(cells)-1], theme.Muted.Render("|")) + theme.Muted.Render("|")
}

func isTableSeparator(line string) bool {
	trimmed := strings.Trim(line, "| ")
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		if r != '-' && r != '|' {
			return false
		}
	}
	return strings.Contains(trimmed, "-")
}

func renderInlineCode(theme ui.Theme, line string) string {
	var b strings.Builder
	remaining := line
	for {
		start := strings.Index(remaining, "`")
		if start < 0 {
			b.WriteString(theme.Text.Render(remaining))
			break
		}
		b.WriteString(theme.Text.Render(remaining[:start]))
		remaining = remaining[start+1:]
		end := strings.Index(remaining, "`")
		if end < 0 {
			b.WriteString(theme.Text.Render("`" + remaining))
			break
		}
		code := remaining[:end]
		if code == "" {
			b.WriteString(theme.Text.Render("``"))
		} else {
			b.WriteString(theme.Code.Render(code))
		}
		remaining = remaining[end+1:]
	}
	return b.String()
}

func issueTypeLabel(issue jira.Issue) string {
	prefix := ""
	if issue.ParentKey != "" || issue.IsSubtask {
		prefix = "|-"
	}
	if issue.IssueType == "" || issue.IssueType == "Unknown" {
		return prefix + "Issue"
	}
	return prefix + issue.IssueType
}

func listText(values []string) string {
	if len(values) == 0 {
		return "None"
	}
	return strings.Join(values, ", ")
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "Unknown"
	}
	return value.Local().Format("2006-01-02 15:04")
}

func statusStyle(theme ui.Theme, status string) lipgloss.Style {
	normalized := strings.ToLower(status)
	switch {
	case strings.Contains(normalized, "done"), strings.Contains(normalized, "closed"), strings.Contains(normalized, "resolved"):
		return theme.Success
	case strings.Contains(normalized, "block"), strings.Contains(normalized, "fail"):
		return theme.Error
	case strings.Contains(normalized, "progress"), strings.Contains(normalized, "review"):
		return theme.Warning
	default:
		return theme.Muted
	}
}

func priorityStyle(theme ui.Theme, priority string) lipgloss.Style {
	normalized := strings.ToLower(priority)
	switch {
	case strings.Contains(normalized, "highest"), strings.Contains(normalized, "critical"), strings.Contains(normalized, "blocker"):
		return theme.Error
	case strings.Contains(normalized, "high"):
		return theme.Warning
	case strings.Contains(normalized, "medium"):
		return theme.Text
	default:
		return theme.Muted
	}
}

func (m Model) sortLabel() string {
	switch m.sort {
	case sortPriority:
		return "priority"
	case sortStatus:
		return "status"
	case sortAssignee:
		return "assignee"
	case sortType:
		return "type"
	case sortKey:
		return "key"
	default:
		return "jira"
	}
}

func (m Model) pageIndicator(start, end int) string {
	left := ""
	right := ""
	if start > 0 {
		left = "PgUp previous"
	}
	if end < len(m.issues) {
		right = "PgDn next"
	}
	switch {
	case left != "" && right != "":
		return left + "  " + right
	case left != "":
		return left
	case right != "":
		return right
	default:
		return ""
	}
}

func (m Model) browserLayout(width int) browserLayout {
	if width <= 0 {
		width = 100
	}
	contentWidth := max(42, width-6)
	return browserLayout{
		contentWidth: contentWidth,
		listWidth:    contentWidth,
		rows:         m.tableRows(),
	}
}

func wrapText(value string, width int) string {
	if width <= 0 {
		return value
	}
	words := strings.Fields(value)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		line += " " + word
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}

func (m Model) renderScrollableDetailBody(content string, width int) string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	rows := m.fullDetailRows()
	if len(lines) <= rows {
		return strings.Join(lines, "\n")
	}

	bodyRows := max(1, rows-1)
	vp := m.newDetailViewport(content, width, bodyRows)
	offset := vp.YOffset()
	end := min(len(lines), offset+bodyRows)
	indicator := m.detailScrollIndicator(offset+1, end, len(lines), width)
	body := strings.TrimRight(vp.View(), "\n ")
	return body + "\n" + indicator
}

func (m Model) detailScrollIndicator(start int, end int, total int, width int) string {
	left := "Detail"
	if section, ok := m.focusedDetailSection(); ok {
		left = section.Label
	}
	right := fmt.Sprintf("Lines %d-%d of %d", start, end, total)
	if start > 1 {
		right += "  PgUp previous"
	}
	if end < total {
		right += "  PgDn next"
	}
	left = truncate(left, max(0, width-lipgloss.Width(right)-2))
	spacer := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return m.theme.Muted.Render(left + strings.Repeat(" ", spacer) + right)
}

func (m Model) focusedDetailSection() (detailSection, bool) {
	sections := m.detailSections()
	if len(sections) == 0 {
		return detailSection{}, false
	}
	return sections[clamp(m.detailFocus, 0, len(sections)-1)], true
}

func (m Model) newDetailViewport(content string, width int, rows int) viewport.Model {
	vp := viewport.New(
		viewport.WithWidth(max(1, width)),
		viewport.WithHeight(max(1, rows)),
	)
	vp.SoftWrap = false
	vp.FillHeight = false
	vp.SetContent(strings.TrimRight(content, "\n"))
	vp.SetYOffset(m.detailOffset)
	return vp
}

func (m Model) fullDetailRows() int {
	return max(3, m.boundedPanelBodyRows(detailHeaderRows+1))
}

func (m Model) currentDetailLineCount() int {
	return len(strings.Split(strings.TrimRight(m.currentDetailContent(), "\n"), "\n"))
}

func (m Model) currentDetailContent() string {
	return m.fullDetailContent(m.currentDetailBodyWidth())
}

func (m Model) currentDetailBodyWidth() int {
	width := m.width
	if width <= 0 {
		width = 100
	}
	return max(32, m.browserLayout(width).contentWidth-8)
}

func wrapRichText(value string, width int) string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	blocks := strings.Split(normalized, "\n")

	var lines []string
	previousBlank := false
	inCodeBlock := false
	for index := 0; index < len(blocks); index++ {
		block := strings.TrimSpace(blocks[index])
		block = strings.TrimSpace(block)
		if block == "```" {
			inCodeBlock = !inCodeBlock
			lines = append(lines, block)
			previousBlank = false
			continue
		}
		if inCodeBlock {
			lines = append(lines, fitCodeLine(block, width))
			previousBlank = false
			continue
		}
		if block == "[table]" {
			var tableLines []string
			for index++; index < len(blocks); index++ {
				tableBlock := strings.TrimSpace(blocks[index])
				if tableBlock == "[/table]" {
					break
				}
				if tableBlock != "" {
					tableLines = append(tableLines, tableBlock)
				}
			}
			if len(tableLines) > 0 {
				lines = append(lines, "[table]")
				lines = append(lines, renderFittedTable(tableLines, width)...)
				lines = append(lines, "[/table]")
			}
			previousBlank = false
			continue
		}
		if block == "" {
			if !previousBlank && len(lines) > 0 {
				lines = append(lines, "")
			}
			previousBlank = true
			continue
		}
		previousBlank = false
		lines = append(lines, wrapRichLine(block, width)...)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func fitCodeLine(line string, width int) string {
	if width <= 0 || len(line) <= width {
		return line
	}
	return truncate(line, width)
}

func renderFittedTable(lines []string, width int) []string {
	rows := parseTableRows(lines)
	if len(rows) == 0 {
		return nil
	}
	widths := fittedTableWidths(rows, width)
	var rendered []string
	for index, row := range rows {
		rendered = append(rendered, renderWrappedTableRow(row, widths)...)
		if index == 0 {
			rendered = append(rendered, renderTableSeparatorLine(widths))
		}
	}
	return rendered
}

func parseTableRows(lines []string) [][]string {
	var rows [][]string
	for _, line := range lines {
		if isTableSeparator(line) {
			continue
		}
		parts := strings.Split(strings.Trim(line, "|"), "|")
		row := make([]string, 0, len(parts))
		for _, part := range parts {
			row = append(row, strings.TrimSpace(part))
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}

func fittedTableWidths(rows [][]string, width int) []int {
	columns := 0
	for _, row := range rows {
		columns = max(columns, len(row))
	}
	if columns == 0 {
		return nil
	}

	available := width - columns*2 - (columns + 1)
	if available < columns {
		available = columns
	}
	widths := make([]int, columns)
	for _, row := range rows {
		for index, cell := range row {
			widths[index] = max(widths[index], len(cell))
		}
	}

	for index := range widths {
		widths[index] = clamp(widths[index], 1, max(1, available/columns))
	}
	for remaining := available - sumInts(widths); remaining > 0; remaining-- {
		target := -1
		targetNeed := 0
		for index := range widths {
			need := naturalTableWidth(rows, index) - widths[index]
			if need > targetNeed {
				target = index
				targetNeed = need
			}
		}
		if target < 0 {
			break
		}
		widths[target]++
	}
	return widths
}

func naturalTableWidth(rows [][]string, column int) int {
	natural := 0
	for _, row := range rows {
		if column < len(row) {
			natural = max(natural, len(row[column]))
		}
	}
	return natural
}

func renderWrappedTableRow(row []string, widths []int) []string {
	wrappedCells := make([][]string, len(widths))
	height := 1
	for index, width := range widths {
		cell := ""
		if index < len(row) {
			cell = row[index]
		}
		wrapped := strings.Split(wrapText(cell, width), "\n")
		if len(wrapped) == 0 {
			wrapped = []string{""}
		}
		wrappedCells[index] = wrapped
		height = max(height, len(wrapped))
	}

	lines := make([]string, 0, height)
	for rowLine := 0; rowLine < height; rowLine++ {
		cells := make([]string, len(widths))
		for index, width := range widths {
			cell := ""
			if rowLine < len(wrappedCells[index]) {
				cell = wrappedCells[index][rowLine]
			}
			cell = truncate(cell, width)
			cells[index] = " " + cell + strings.Repeat(" ", width-len(cell)) + " "
		}
		lines = append(lines, "|"+strings.Join(cells, "|")+"|")
	}
	return lines
}

func renderTableSeparatorLine(widths []int) string {
	parts := make([]string, len(widths))
	for index, width := range widths {
		parts[index] = strings.Repeat("-", width+2)
	}
	return "|" + strings.Join(parts, "|") + "|"
}

func sumInts(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func wrapRichLine(line string, width int) []string {
	marker, body := richLineMarker(line)
	if marker == "" {
		return strings.Split(wrapText(line, width), "\n")
	}

	bodyWidth := max(12, width-len(marker))
	wrapped := strings.Split(wrapText(body, bodyWidth), "\n")
	if len(wrapped) == 0 {
		return []string{marker}
	}
	lines := make([]string, 0, len(wrapped))
	lines = append(lines, marker+wrapped[0])
	indent := strings.Repeat(" ", len(marker))
	for _, continuation := range wrapped[1:] {
		lines = append(lines, indent+continuation)
	}
	return lines
}

func richLineMarker(line string) (string, string) {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		return line[:2], strings.TrimSpace(line[2:])
	}
	if strings.HasPrefix(line, "> ") {
		return line[:2], strings.TrimSpace(line[2:])
	}
	for index, r := range line {
		if r == '.' && index > 0 && index < 4 {
			prefix := line[:index+1]
			for _, digit := range prefix[:len(prefix)-1] {
				if digit < '0' || digit > '9' {
					return "", line
				}
			}
			rest := strings.TrimSpace(line[index+1:])
			if rest != "" {
				return prefix + " ", rest
			}
		}
		if r < '0' || r > '9' {
			break
		}
	}
	return "", line
}

func (m Model) startRefresh() (Model, tea.Cmd) {
	m.nextRequestID++
	m.activeRequestID = m.nextRequestID
	m.expandLoading = false
	m.expandRequestKey = ""
	if len(m.issues) == 0 {
		m.loading = true
	} else {
		m.refreshing = true
	}
	return m, m.submitIssueSearch(m.activeRequestID)
}

func (m Model) startExpandSelectedIssue(mode worker.ExpandMode) (Model, tea.Cmd) {
	issue, ok := m.selectedIssue()
	if !ok || issue.Key == "" {
		m.detailNotice = "No issue selected."
		return m, nil
	}
	m.nextRequestID++
	m.activeExpandReqID = m.nextRequestID
	m.expandRequestKey = issue.Key
	m.expandMode = mode
	m.expandLoading = true
	label := "open children"
	if mode == worker.ExpandModeAll {
		label = "all children"
	}
	m.detailNotice = "Loading " + label + " for " + issue.Key + "."
	return m, m.submitExpandIssues(m.activeExpandReqID, issue.Key, mode)
}

func (m Model) switchView(delta int) (Model, tea.Cmd) {
	if len(m.views) == 0 {
		return m, nil
	}
	m.view = (m.view + delta + len(m.views)) % len(m.views)
	m.jql = m.views[m.view].JQL
	m.selected = 0
	m.offset = 0
	m.issues = nil
	m.err = nil
	m.mode = modeTable
	m.loading = true
	m.refreshing = false
	m.expandLoading = false
	m.expandRequestKey = ""
	m.detailNotice = ""
	return m.startRefresh()
}

func (m *Model) mergeExpandedIssues(children []jira.Issue) int {
	if len(children) == 0 {
		return 0
	}
	selectedKey := ""
	if selected, ok := m.selectedIssue(); ok {
		selectedKey = selected.Key
	}
	seen := make(map[string]bool, len(m.issues)+len(children))
	for _, issue := range m.issues {
		seen[issue.Key] = true
	}
	added := 0
	for _, child := range children {
		if child.Key == "" || seen[child.Key] {
			continue
		}
		m.issues = append(m.issues, child)
		seen[child.Key] = true
		added++
	}
	if added == 0 {
		return 0
	}
	m.issues = orderIssues(m.issues, m.sort)
	if selectedKey != "" {
		for index, issue := range m.issues {
			if issue.Key == selectedKey {
				m.selected = index
				break
			}
		}
	}
	return added
}

func (m *Model) switchSort(delta int) {
	sortCount := int(sortKey) + 1
	m.sort = sortMode((int(m.sort) + delta + sortCount) % sortCount)
	selectedKey := ""
	if len(m.issues) > 0 && m.selected >= 0 && m.selected < len(m.issues) {
		selectedKey = m.issues[m.selected].Key
	}
	m.issues = orderIssues(m.issues, m.sort)
	if selectedKey != "" {
		for index, issue := range m.issues {
			if issue.Key == selectedKey {
				m.selected = index
				break
			}
		}
	}
	m.ensureSelectionVisible(m.currentLayoutRows())
}

func (m Model) activeViewName() string {
	if len(m.views) == 0 || m.view < 0 || m.view >= len(m.views) {
		return "Default"
	}
	return m.views[m.view].Name
}

func (m *Model) moveSelection(delta int) {
	if len(m.issues) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	m.selected = clamp(m.selected+delta, 0, len(m.issues)-1)
	m.resetDetailScroll()
	m.ensureSelectionVisible(m.currentLayoutRows())
}

func (m *Model) pageSelection(delta int) {
	if len(m.issues) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	rows := m.currentLayoutRows()
	step := max(1, rows-1)
	m.selected = clamp(m.selected+(delta*step), 0, len(m.issues)-1)
	m.resetDetailScroll()
	m.ensureSelectionVisible(rows)
}

func (m *Model) scrollDetail(delta int) {
	content := m.currentDetailContent()
	rows := max(1, m.fullDetailRows()-1)
	width := m.currentDetailBodyWidth()
	vp := m.newDetailViewport(content, width, rows)
	if delta > 0 {
		vp.ScrollDown(delta)
	} else if delta < 0 {
		vp.ScrollUp(-delta)
	}
	m.detailOffset = vp.YOffset()
	m.saveDetailSectionOffset()
}

func (m *Model) pageDetail(delta int) {
	content := m.currentDetailContent()
	rows := max(1, m.fullDetailRows()-1)
	width := m.currentDetailBodyWidth()
	vp := m.newDetailViewport(content, width, rows)
	if delta > 0 {
		vp.PageDown()
	} else if delta < 0 {
		vp.PageUp()
	}
	m.detailOffset = vp.YOffset()
	m.saveDetailSectionOffset()
}

func (m *Model) scrollDetailToBottom() {
	content := m.currentDetailContent()
	rows := max(1, m.fullDetailRows()-1)
	width := m.currentDetailBodyWidth()
	vp := m.newDetailViewport(content, width, rows)
	vp.GotoBottom()
	m.detailOffset = vp.YOffset()
	m.saveDetailSectionOffset()
}

func (m *Model) startCommentComposer() {
	m.mode = modeComment
	m.linkFocus = false
	m.commentDraft = ""
	m.commentEditor = newCommentEditor(m.commentDraft)
	m.commentEditorReady = true
	m.commentConfirm = false
	m.commentSubmitting = false
	m.commentMentions = nil
	m.closeMentionPicker()
	m.detailNotice = ""
}

func (m Model) updateCommentComposer(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c":
			m.workers.Stop()
			return m, tea.Quit
		case "esc":
			if m.mentionPickerOpen {
				return m.updateMentionPicker(msg)
			}
			m.mode = modeDetail
			m.commentDraft = ""
			m.commentEditor = newCommentEditor("")
			m.commentEditorReady = true
			m.commentConfirm = false
			m.commentSubmitting = false
			m.commentMentions = nil
			m.closeMentionPicker()
			m.detailNotice = "Comment canceled."
			return m, nil
		}
	}
	if m.commentSubmitting {
		return m, nil
	}
	if m.commentConfirm {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "y":
				return m.submitCommentDraft()
			case "n":
				m.commentConfirm = false
				m.detailNotice = ""
				m.ensureCommentEditor()
				m.commentEditor.Focus()
				return m, nil
			}
		}
		return m, nil
	}

	if m.mentionPickerOpen {
		return m.updateMentionPicker(msg)
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "@":
			m.openMentionPicker()
			return m, nil
		case "tab", "ctrl+s":
			m.commentDraft = m.commentEditorValue()
			if strings.TrimSpace(m.commentDraft) == "" {
				m.detailNotice = "Write a comment before posting."
				return m, nil
			}
			m.commentConfirm = true
			m.ensureCommentEditor()
			m.commentEditor.Blur()
			m.detailNotice = ""
			return m, nil
		}
	}

	m.ensureCommentEditor()
	m.configureCommentEditor()
	editor, cmd := m.commentEditor.Update(msg)
	m.commentEditor = editor
	m.commentDraft = m.commentEditor.Value()
	if strings.TrimSpace(m.commentDraft) != "" {
		m.detailNotice = ""
	}
	return m, cmd
}

func (m Model) updateMentionPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c":
			m.workers.Stop()
			return m, tea.Quit
		case "esc":
			literal := "@"
			if m.mentionQuery != "" {
				literal += m.mentionQuery
			}
			m.closeMentionPicker()
			m.insertCommentText(literal)
			return m, nil
		case "enter":
			if user, ok := m.selectedMentionUser(); ok {
				mention := jira.Mention{
					AccountID: user.AccountID,
					Text:      m.mentionText(user),
				}
				m.closeMentionPicker()
				m.insertCommentText(mention.Text)
				m.commentMentions = append(m.commentMentions, mention)
				return m, nil
			}
			return m, nil
		case "up":
			m.moveMentionCursor(-1)
			return m, nil
		case "down":
			m.moveMentionCursor(1)
			return m, nil
		}
		if query, changed := nextMentionQuery(m.mentionQuery, keyMsg); changed {
			m.setMentionQuery(query)
			if m.mentionQuery == "" {
				m.mentionSearchLoading = false
				m.mentionSearchErr = nil
				m.mentionUsers = nil
				m.mentionCursor = 0
				return m, nil
			}
			m.nextRequestID++
			m.mentionSearchReqID = m.nextRequestID
			m.mentionSearchLoading = true
			m.mentionSearchErr = nil
			return m, m.submitUserSearch(m.mentionSearchReqID, m.mentionQuery)
		}
	}
	return m, nil
}

func nextMentionQuery(current string, keyMsg tea.KeyMsg) (string, bool) {
	switch keyMsg.String() {
	case "backspace", "ctrl+h":
		runes := []rune(current)
		if len(runes) == 0 {
			return "", false
		}
		return string(runes[:len(runes)-1]), true
	}
	text := keyMsg.Key().Text
	if text == "" {
		return current, false
	}
	runes := []rune(text)
	if len(runes) != 1 || runes[0] < 32 || runes[0] == 127 {
		return current, false
	}
	return current + text, true
}

func newCommentEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Write a Jira comment..."
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func (m *Model) ensureCommentEditor() {
	if m.commentEditorReady {
		return
	}
	m.commentEditor = newCommentEditor(m.commentDraft)
	m.commentEditorReady = true
}

func (m *Model) configureCommentEditor() {
	m.ensureCommentEditor()
	width := max(32, m.browserLayout(m.width).contentWidth-8)
	rows := m.commentEditorRows()
	m.commentEditor.MaxHeight = max(rows, 1)
	m.commentEditor.MaxWidth = width
	m.commentEditor.SetWidth(width)
	m.commentEditor.SetHeight(rows)
}

func (m Model) configuredCommentEditor(width int, rows int) textarea.Model {
	editor := m.commentEditor
	if !m.commentEditorReady {
		editor = newCommentEditor(m.commentDraft)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	if !m.commentConfirm && !m.commentSubmitting && !m.mentionPickerOpen {
		editor.Focus()
	}
	return editor
}

func (m *Model) openMentionPicker() {
	m.mentionPickerOpen = true
	m.mentionSearchLoading = false
	m.mentionSearchErr = nil
	m.mentionUsers = nil
	m.mentionCursor = 0
	m.setMentionQuery("")
	m.ensureCommentEditor()
	m.commentEditor.Blur()
}

func (m *Model) closeMentionPicker() {
	m.mentionPickerOpen = false
	m.mentionQuery = ""
	m.mentionSearchLoading = false
	m.mentionSearchErr = nil
	m.ensureCommentEditor()
	m.commentEditor.Focus()
}

func (m *Model) setMentionQuery(query string) {
	m.mentionQuery = strings.TrimSpace(query)
	m.mentionCursor = 0
}

func (m *Model) moveMentionCursor(delta int) {
	if len(m.mentionUsers) == 0 {
		m.mentionCursor = 0
		return
	}
	m.mentionCursor = clamp(m.mentionCursor+delta, 0, len(m.mentionUsers)-1)
}

func (m Model) selectedMentionUser() (jira.User, bool) {
	if len(m.mentionUsers) == 0 {
		return jira.User{}, false
	}
	index := clamp(m.mentionCursor, 0, len(m.mentionUsers)-1)
	return m.mentionUsers[index], true
}

func (m *Model) insertCommentText(value string) {
	m.ensureCommentEditor()
	m.configureCommentEditor()
	m.commentEditor.InsertString(value)
	m.commentDraft = m.commentEditor.Value()
	if strings.TrimSpace(m.commentDraft) != "" {
		m.detailNotice = ""
	}
}

func (m Model) mentionText(user jira.User) string {
	return "@" + m.mentionDisplayName(user)
}

func (m Model) mentionDisplayName(user jira.User) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	if user.Email != "" {
		return user.Email
	}
	return user.AccountID
}

func (m Model) commentEditorValue() string {
	if !m.commentEditorReady {
		return m.commentDraft
	}
	return m.commentEditor.Value()
}

func (m Model) submitCommentDraft() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		m.detailNotice = "No issue selected."
		m.commentConfirm = false
		return m, nil
	}
	m.commentDraft = m.commentEditorValue()
	body := strings.TrimSpace(m.commentDraft)
	if body == "" {
		m.detailNotice = "Write a comment before posting."
		m.commentConfirm = false
		return m, nil
	}
	m.nextRequestID++
	m.activeCommentReqID = m.nextRequestID
	m.commentRequestKey = selected.Key
	m.commentSubmitting = true
	m.detailNotice = ""
	return m, m.submitAddComment(m.activeCommentReqID, selected.Key, body, m.commentMentions)
}

func isPrintableKey(value string) bool {
	runes := []rune(value)
	if len(runes) != 1 {
		return false
	}
	return runes[0] >= 32 && runes[0] != 127
}

func (m *Model) focusDetailLinks() {
	links := m.currentDetailLinks()
	if len(links) == 0 {
		m.linkFocus = false
		m.detailNotice = "No links found in this ticket description."
		return
	}
	m.linkFocus = true
	m.selectedLink = clamp(m.selectedLink, 0, len(links)-1)
	m.detailNotice = ""
	m.jumpDetailSection("Links")
}

func (m *Model) moveSelectedDetailLink(delta int) {
	links := m.currentDetailLinks()
	if len(links) == 0 {
		m.linkFocus = false
		m.selectedLink = 0
		return
	}
	m.selectedLink = clamp(m.selectedLink+delta, 0, len(links)-1)
}

func (m *Model) selectDetailLinkNumber(value string) {
	links := m.currentDetailLinks()
	if len(links) == 0 {
		return
	}
	number := int(value[0] - '0')
	if number <= 0 || number > len(links) {
		return
	}
	m.linkFocus = true
	m.selectedLink = number - 1
	m.detailNotice = ""
	m.jumpDetailSection("Links")
}

func (m Model) openSelectedDetailLink() (Model, tea.Cmd) {
	link, ok := m.selectedDetailLink()
	if !ok {
		m.detailNotice = "No link selected."
		return m, nil
	}
	target := link.Target
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "open",
			target: target,
			err:    openExternal(target),
		}
	}
}

func (m Model) copySelectedDetailLink() (Model, tea.Cmd) {
	link, ok := m.selectedDetailLink()
	if !ok {
		m.detailNotice = "No link selected."
		return m, nil
	}
	target := linkCopyText(link)
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "copy",
			target: target,
			err:    copyToClipboard(target),
		}
	}
}

func (m Model) openSelectedIssue() (Model, tea.Cmd) {
	issue, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(issue.URL) == "" {
		m.detailNotice = "No issue URL available."
		return m, nil
	}
	target := issue.URL
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "open",
			target: target,
			err:    openExternal(target),
		}
	}
}

func (m Model) copySelectedIssueKey() (Model, tea.Cmd) {
	issue, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(issue.Key) == "" {
		m.detailNotice = "No issue key available."
		return m, nil
	}
	target := issue.Key
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "copy",
			target: target,
			err:    copyToClipboard(target),
		}
	}
}

func (m Model) copySelectedIssueURL() (Model, tea.Cmd) {
	issue, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(issue.URL) == "" {
		m.detailNotice = "No issue URL available."
		return m, nil
	}
	target := issue.URL
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "copy",
			target: target,
			err:    copyToClipboard(target),
		}
	}
}

func (m Model) selectedDetailLink() (detailLink, bool) {
	links := m.currentDetailLinks()
	if len(links) == 0 {
		return detailLink{}, false
	}
	index := clamp(m.selectedLink, 0, len(links)-1)
	return links[index], true
}

func (m Model) currentDetailLinks() []detailLink {
	if len(m.issues) == 0 || m.selected < 0 || m.selected >= len(m.issues) {
		return nil
	}
	detail, ok := m.details[m.issues[m.selected].Key]
	if !ok {
		return nil
	}
	return collectDetailLinks(detail.Description)
}

func (m *Model) jumpDetailSection(title string) {
	m.saveDetailSectionOffset()
	for index, section := range m.detailSections() {
		if strings.EqualFold(section.Label, title) || strings.EqualFold(section.ID, title) {
			m.detailFocus = index
			m.restoreDetailSectionOffset()
			return
		}
	}
}

func (m *Model) setDetailOffset(offset int) {
	content := m.currentDetailContent()
	rows := max(1, m.fullDetailRows()-1)
	width := m.currentDetailBodyWidth()
	vp := m.newDetailViewport(content, width, rows)
	vp.SetYOffset(offset)
	m.detailOffset = vp.YOffset()
	m.saveDetailSectionOffset()
}

func (m *Model) saveDetailSectionOffset() {
	section, ok := m.focusedDetailSection()
	if !ok {
		return
	}
	if m.detailSectionOffset == nil {
		m.detailSectionOffset = make(map[string]int)
	}
	m.detailSectionOffset[section.ID] = m.detailOffset
}

func (m *Model) restoreDetailSectionOffset() {
	section, ok := m.focusedDetailSection()
	if !ok {
		m.detailOffset = 0
		return
	}
	offset := 0
	if m.detailSectionOffset != nil {
		offset = m.detailSectionOffset[section.ID]
	}
	m.setDetailOffset(offset)
}

func (m *Model) resetDetailScroll() {
	m.detailOffset = 0
	m.detailSectionOffset = make(map[string]int)
}

func linkActionNotice(msg linkActionMsg) string {
	if msg.err != nil {
		return fmt.Sprintf("Could not %s %s: %v", msg.action, msg.target, msg.err)
	}
	switch msg.action {
	case "copy":
		return "Copied " + msg.target
	case "open":
		return "Opened " + msg.target
	default:
		return msg.target
	}
}

func defaultOpenExternal(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Run()
	default:
		return exec.Command("xdg-open", target).Run()
	}
}

func defaultCopyToClipboard(value string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	}
	cmd.Stdin = strings.NewReader(value)
	return cmd.Run()
}

func (m *Model) ensureSelectionVisible(rows int) {
	rows = max(1, rows)
	renderedRows := m.issueRows(m.browserLayout(m.width))
	selectedRow := m.selectedRenderedRowIndex(renderedRows)
	maxOffset := max(0, len(renderedRows)-rows)
	if selectedRow < m.offset {
		m.offset = selectedRow
	}
	if selectedRow >= m.offset+rows {
		m.offset = selectedRow - rows + 1
	}
	m.offset = clamp(m.offset, 0, maxOffset)
}

func (m Model) selectedRenderedRowIndex(rows []string) int {
	if len(m.issues) == 0 || m.selected < 0 || m.selected >= len(m.issues) {
		return 0
	}
	key := m.issues[m.selected].Key
	for index, row := range rows {
		if strings.Contains(row, key) {
			return index
		}
	}
	return clamp(m.selected, 0, max(0, len(rows)-1))
}

func (m Model) currentLayoutRows() int {
	width := m.width
	if width <= 0 {
		width = 100
	}
	return m.browserLayout(width).rows
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

func (m Model) submitExpandIssues(requestID int, parentKey string, mode worker.ExpandMode) tea.Cmd {
	return func() tea.Msg {
		if parentKey == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindExpandIssues,
			Timeout: m.requestTimeout,
			ExpandIssues: &worker.ExpandIssuesRequest{
				ParentKey:  parentKey,
				Mode:       mode,
				MaxResults: maxIssues,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindExpandIssues,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{}
	}
}

func (m Model) startDetailRequestForSelected() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		m.detailLoading = false
		m.detailErr = nil
		m.detailRequestKey = ""
		m.commentsLoading = false
		m.commentsErr = nil
		m.commentsRequestKey = ""
		return m, nil
	}
	var cmds []tea.Cmd
	if _, ok := m.details[selected.Key]; ok {
		m.detailLoading = false
		m.detailErr = nil
		m.detailRequestKey = ""
	} else if !(m.detailLoading && m.detailRequestKey == selected.Key) {
		m.nextRequestID++
		m.activeDetailRequestID = m.nextRequestID
		m.detailRequestKey = selected.Key
		m.detailLoading = true
		m.detailErr = nil
		cmds = append(cmds, m.submitIssueDetail(m.activeDetailRequestID, selected.Key))
	}

	if _, ok := m.comments[selected.Key]; ok {
		m.commentsLoading = false
		m.commentsErr = nil
		m.commentsRequestKey = ""
	} else if !(m.commentsLoading && m.commentsRequestKey == selected.Key) {
		m.nextRequestID++
		m.activeCommentsReqID = m.nextRequestID
		m.commentsRequestKey = selected.Key
		m.commentsLoading = true
		m.commentsErr = nil
		cmds = append(cmds, m.submitIssueComments(m.activeCommentsReqID, selected.Key))
	}

	return m, tea.Batch(cmds...)
}

func (m Model) submitIssueDetail(requestID int, key string) tea.Cmd {
	return func() tea.Msg {
		if key == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindGetIssue,
			Timeout: m.requestTimeout,
			GetIssue: &worker.GetIssueRequest{
				Key: key,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindGetIssue,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{}
	}
}

func (m Model) submitIssueComments(requestID int, key string) tea.Cmd {
	return func() tea.Msg {
		if key == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindGetComments,
			Timeout: m.requestTimeout,
			GetComments: &worker.GetCommentsRequest{
				Key:        key,
				MaxResults: maxComments,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindGetComments,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{}
	}
}

func (m Model) submitAddComment(requestID int, key string, body string, mentions []jira.Mention) tea.Cmd {
	return func() tea.Msg {
		if key == "" || strings.TrimSpace(body) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindAddComment,
			Timeout: m.requestTimeout,
			AddComment: &worker.AddCommentRequest{
				Key:      key,
				Body:     body,
				Mentions: mentions,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindAddComment,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{}
	}
}

func (m Model) submitUserSearch(requestID int, query string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(query) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindSearchUsers,
			Timeout: m.requestTimeout,
			SearchUsers: &worker.SearchUsersRequest{
				Query:      query,
				MaxResults: 20,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindSearchUsers,
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

func (m Model) selectedIssue() (jira.Issue, bool) {
	if len(m.issues) == 0 || m.selected < 0 || m.selected >= len(m.issues) {
		return jira.Issue{}, false
	}
	return m.issues[m.selected], true
}

func (m *Model) replaceIssues(issues []jira.Issue) {
	selectedKey := ""
	if len(m.issues) > 0 && m.selected >= 0 && m.selected < len(m.issues) {
		selectedKey = m.issues[m.selected].Key
	}

	m.issues = orderIssues(issues, m.sort)
	if len(m.issues) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}

	if selectedKey != "" {
		for index, issue := range m.issues {
			if issue.Key == selectedKey {
				m.selected = index
				m.ensureSelectionVisible(m.currentLayoutRows())
				return
			}
		}
	}
	m.selected = clamp(m.selected, 0, len(m.issues)-1)
	m.ensureSelectionVisible(m.currentLayoutRows())
}

func (m Model) tableRows() int {
	if m.height <= 0 {
		return 10
	}

	// Header, query, footer, panel borders, padding, title, and table header all
	// consume vertical space outside the viewport-backed issue rows.
	reserved := 15
	if m.height >= ui.MinTerminalHeight && m.height-reserved < minUsefulIssueRows && !ui.TerminalTooSmall(m.width, m.height) {
		reserved--
	}
	return max(1, m.height-reserved)
}

func (m Model) useCompactIssueListChrome(layout browserLayout) bool {
	return layout.rows <= minUsefulIssueRows
}

func orderIssues(issues []jira.Issue, mode sortMode) []jira.Issue {
	byParent := make(map[string][]jira.Issue)
	topLevel := make([]jira.Issue, 0, len(issues))
	seen := make(map[string]bool, len(issues))
	for _, issue := range issues {
		seen[issue.Key] = true
	}
	for _, issue := range issues {
		if issue.ParentKey != "" && seen[issue.ParentKey] {
			byParent[issue.ParentKey] = append(byParent[issue.ParentKey], issue)
			continue
		}
		topLevel = append(topLevel, issue)
	}

	sortIssueGroup(topLevel, mode)
	for parent := range byParent {
		sortIssueGroup(byParent[parent], mode)
	}

	ordered := make([]jira.Issue, 0, len(issues))
	for _, issue := range topLevel {
		ordered = append(ordered, issue)
		ordered = append(ordered, byParent[issue.Key]...)
	}
	return ordered
}

func sortIssueGroup(issues []jira.Issue, mode sortMode) {
	if mode == sortJira {
		return
	}
	sort.SliceStable(issues, func(i, j int) bool {
		left := issues[i]
		right := issues[j]
		switch mode {
		case sortPriority:
			if priorityRank(left.Priority) != priorityRank(right.Priority) {
				return priorityRank(left.Priority) > priorityRank(right.Priority)
			}
		case sortStatus:
			if left.Status != right.Status {
				return left.Status < right.Status
			}
		case sortAssignee:
			if left.Assignee != right.Assignee {
				return left.Assignee < right.Assignee
			}
		case sortType:
			if left.IssueType != right.IssueType {
				return left.IssueType < right.IssueType
			}
		case sortKey:
			if left.Key != right.Key {
				return left.Key < right.Key
			}
		}
		return left.Key < right.Key
	})
}

func priorityRank(priority string) int {
	normalized := strings.ToLower(priority)
	switch {
	case strings.Contains(normalized, "highest"), strings.Contains(normalized, "blocker"), strings.Contains(normalized, "critical"):
		return 5
	case strings.Contains(normalized, "high"):
		return 4
	case strings.Contains(normalized, "medium"):
		return 3
	case strings.Contains(normalized, "low"):
		return 2
	case strings.Contains(normalized, "lowest"):
		return 1
	default:
		return 0
	}
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

func indentLines(value string, prefix string) string {
	if value == "" || prefix == "" {
		return value
	}
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		if line == "" {
			continue
		}
		lines[index] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func padRight(value string, width int) string {
	padding := width - lipgloss.Width(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

func fitRight(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return padRight(value, width)
	}
	result := ""
	for _, r := range reverseRunes(value) {
		candidate := string(r) + result
		if lipgloss.Width(candidate) > width {
			continue
		}
		result = candidate
		if lipgloss.Width(result) == width {
			break
		}
	}
	return padRight(result, width)
}

func fitLeft(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	result := ""
	for _, r := range value {
		candidate := result + string(r)
		if lipgloss.Width(candidate) > width {
			break
		}
		result = candidate
	}
	return result
}

func reverseRunes(value string) []rune {
	runes := []rune(value)
	for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
		runes[left], runes[right] = runes[right], runes[left]
	}
	return runes
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
