package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	lipglosstable "github.com/charmbracelet/lipgloss/table"
	"github.com/jellydator/ttlcache/v3"
	"github.com/jon/jira-tui/internal/claude"
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
	userSearchCacheTTL    = 2 * time.Minute
	issueDetailCacheTTL   = 45 * time.Second
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
	createPickerMaxRows   = 6
)

const (
	createTypeFieldIndex = iota
	createSummaryFieldIndex
	createDescriptionFieldIndex
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

	issues                             []jira.Issue
	selected                           int
	offset                             int
	detailOffset                       int
	detailSectionOffset                map[string]int
	detailFocus                        int
	detailBackStack                    []int
	hierarchyFocus                     bool
	selectedHierarchy                  int
	actionFocus                        bool
	selectedAction                     int
	transitionFocus                    bool
	selectedTransition                 int
	priorityFocus                      bool
	selectedPriority                   int
	assigneeFocus                      bool
	selectedAssignee                   int
	assigneeUsers                      []jira.User
	assigneeQuery                      string
	assigneeSearchLoading              bool
	assigneeSearchErr                  error
	assigneeSearchReqID                int
	assigneeSubmitting                 bool
	assigneeSubmitKey                  string
	assigneeSubmitValue                jira.User
	userSearchCache                    *ttlcache.Cache[string, []jira.User]
	summaryFocus                       bool
	summaryEditing                     bool
	summaryDraft                       string
	summaryDirty                       bool
	summaryEditor                      textarea.Model
	summaryEditorReady                 bool
	createOpen                         bool
	createProjectKey                   string
	createIssueTypes                   []jira.CreateIssueType
	selectedCreateIssueType            int
	createIssueTypesLoading            bool
	createIssueTypesErr                error
	createFields                       []jira.CreateField
	createFieldsLoading                bool
	createFieldsErr                    error
	createIssueType                    jira.CreateIssueType
	createChangingType                 bool
	createAIGeneratedMode              bool
	createFieldFocus                   int
	createSummaryDraft                 string
	createDescriptionDraft             string
	createSummaryEditor                textarea.Model
	createSummaryEditorReady           bool
	createDescriptionEditor            textarea.Model
	createDescriptionEditorReady       bool
	createAIPromptLoading              bool
	createAIPromptErr                  error
	createAIPromptStartedAt            time.Time
	createAIPromptCancel               context.CancelFunc
	createAIPromptEvents               chan claude.Event
	createAIPromptProgress             []claude.Event
	createAIPromptOpen                 bool
	createAIPrompt                     string
	createAIPromptEditor               textarea.Model
	createAIPromptEditorReady          bool
	createAIFieldDrafts                map[string]string
	createAIQuestions                  []createAIQuestion
	selectedCreateAIQuestion           int
	createAIQuestionAnswering          bool
	createAIQuestionEditor             textarea.Model
	createAIQuestionEditorReady        bool
	createSubmitting                   bool
	createSubmitSummary                string
	createSubmitDescription            string
	createDynamicValues                map[string]string
	createDynamicSelections            map[string]int
	createDynamicFilters               map[string]string
	createSubmitFields                 []jira.CreateIssueFieldValue
	detailViewport                     viewport.Model
	detailViewportReady                bool
	linkFocus                          bool
	selectedLink                       int
	width                              int
	height                             int
	helpOpen                           bool
	helpOffset                         int
	diagnosticsOpen                    bool
	diagnosticsEvents                  []diagnosticEvent
	claudeConfig                       ClaudeConfig
	claudeStatus                       ClaudeStatus
	claudeRunner                       claudeRunner
	selectedClaudeAction               int
	inlineAIOpen                       bool
	selectedInlineAIAction             int
	inlineAIInstructionOpen            bool
	inlineAIInstruction                string
	inlineAIInstructionEditor          textarea.Model
	inlineAIInstructionReady           bool
	claudePlanLoading                  bool
	claudePlanOpen                     bool
	claudePlanText                     string
	claudePlanErr                      error
	claudePlanKey                      string
	claudePlanOffset                   int
	claudePlanStartedAt                time.Time
	claudePlanCancel                   context.CancelFunc
	claudePlanEvents                   chan claude.Event
	claudePlanProgress                 []claude.Event
	activeClaudePlanReqID              int
	claudeAssistLoading                bool
	claudeAssistOpen                   bool
	claudeAssistText                   string
	claudeAssistErr                    error
	claudeAssistKey                    string
	claudeAssistStartedAt              time.Time
	claudeAssistCancel                 context.CancelFunc
	claudeAssistEvents                 chan claude.Event
	claudeAssistProgress               []claude.Event
	activeClaudeAssistReqID            int
	claudeAssistDraft                  string
	claudeAssistEditor                 textarea.Model
	claudeAssistEditorReady            bool
	claudeAssistTarget                 claudeAssistTarget
	claudeAssistConfirmApply           bool
	claudeAssistApplying               bool
	claudeAssistApplySummary           string
	claudeAssistApplyDescription       string
	activeClaudeAssistSummaryReqID     int
	activeClaudeAssistDescriptionReqID int
	claudeAssistSummaryApplied         bool
	claudeAssistDescriptionApplied     bool
	claudeAssistConfirmComment         bool
	claudeAssistPostingComment         bool
	activeClaudeAssistCommentReqID     int
	claudeAssistRefining               bool
	claudeAssistRefineInstruction      string
	claudeAssistRefineEditor           textarea.Model
	claudeAssistRefineEditorReady      bool
	activeCreateAIPromptReqID          int
	now                                func() time.Time

	loading    bool
	refreshing bool
	err        error
	lastSynced time.Time

	details                     map[string]jira.IssueDetail
	detailFreshnessCache        *ttlcache.Cache[string, struct{}]
	detailLoading               bool
	detailErr                   error
	detailRequestKey            string
	comments                    map[string][]jira.Comment
	commentsLoading             bool
	commentsErr                 error
	commentsRequestKey          string
	commentDraft                string
	commentEditor               textarea.Model
	commentEditorReady          bool
	commentConfirm              bool
	commentSubmitting           bool
	commentRequestKey           string
	commentMentions             []jira.Mention
	mentionPickerOpen           bool
	mentionUsers                []jira.User
	mentionCursor               int
	mentionQuery                string
	mentionSearchLoading        bool
	mentionSearchErr            error
	mentionSearchReqID          int
	detailNotice                string
	activeDetailRequestID       int
	activeCommentsReqID         int
	activeCommentReqID          int
	activeExpandReqID           int
	activeTransitionsReqID      int
	activeTransitionReqID       int
	activeSummaryMetadataReqID  int
	activeSummaryReqID          int
	activePriorityMetadataReqID int
	activePriorityReqID         int
	activeAssigneeReqID         int
	activeCreateIssueTypesReqID int
	activeCreateFieldsReqID     int
	activeCreateIssueReqID      int
	expandLoading               bool
	expandRequestKey            string
	expandMode                  worker.ExpandMode
	transitions                 map[string][]jira.Transition
	transitionLoading           bool
	transitionSubmitting        bool
	transitionRequestKey        string
	transitionSubmitKey         string
	transitionSubmitToStatus    string
	transitionErr               error
	editMetadata                map[string]jira.EditMetadata
	summaryMetadataLoading      bool
	summaryMetadataRequestKey   string
	summaryMetadataErr          error
	summarySubmitting           bool
	summarySubmitKey            string
	summarySubmitValue          string
	priorityMetadataLoading     bool
	priorityMetadataRequestKey  string
	priorityMetadataErr         error
	prioritySubmitting          bool
	prioritySubmitKey           string
	prioritySubmitValue         jira.FieldOption

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

const maxDiagnosticsEvents = 80

type diagnosticKind string

const (
	diagnosticKindWorker diagnosticKind = "worker"
	diagnosticKindCache  diagnosticKind = "cache"
	diagnosticKindClaude diagnosticKind = "claude"
)

type diagnosticEvent struct {
	At     time.Time
	Kind   diagnosticKind
	Label  string
	Status string
	Detail string
}

type ClaudeStatus struct {
	Enabled   bool
	Available bool
	Command   string
	Version   string
	Message   string
	Err       error
}

type ClaudeConfig struct {
	Enabled             bool
	TicketPlan          bool
	TicketAssist        bool
	DraftTicket         bool
	Command             string
	Timeout             time.Duration
	RequireConfirmation bool
	AllowJiraWrites     bool
}

type claudeRunner interface {
	Run(context.Context, claude.Request) (claude.Result, error)
}

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

type detailTargetKind string

const (
	detailTargetField   detailTargetKind = "field"
	detailTargetSection detailTargetKind = "section"
)

type detailTarget struct {
	ID      string
	Label   string
	Kind    detailTargetKind
	Section detailSection
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

func WithClaudeStatus(status ClaudeStatus) Option {
	return func(m *Model) {
		m.claudeStatus = status
		state := "disabled"
		if status.Enabled && status.Available {
			state = "ready"
		} else if status.Enabled {
			state = "unavailable"
		}
		detailParts := []string{state}
		if strings.TrimSpace(status.Command) != "" {
			detailParts = append(detailParts, strings.TrimSpace(status.Command))
		}
		if strings.TrimSpace(status.Version) != "" {
			detailParts = append(detailParts, strings.TrimSpace(status.Version))
		}
		if strings.TrimSpace(status.Message) != "" {
			detailParts = append(detailParts, strings.TrimSpace(status.Message))
		}
		if status.Err != nil {
			detailParts = append(detailParts, truncate(status.Err.Error(), 80))
		}
		eventStatus := "ok"
		if status.Enabled && !status.Available {
			eventStatus = "error"
		}
		m.recordDiagnosticEvent(diagnosticKindClaude, "claude", eventStatus, strings.Join(detailParts, " "))
	}
}

func WithClaudeConfig(config ClaudeConfig) Option {
	return func(m *Model) {
		m.claudeConfig = config
	}
}

func WithClaudeRunner(runner claudeRunner) Option {
	return func(m *Model) {
		m.claudeRunner = runner
	}
}

type refreshTickMsg struct{}
type workSubmittedMsg struct {
	kind worker.Kind
	id   int
	key  string
}
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

type claudePlanResultMsg struct {
	id   int
	key  string
	text string
	err  error
}

type claudePlanTickMsg struct {
	id int
}

type claudePlanProgressMsg struct {
	id    int
	key   string
	event claude.Event
}

type claudeAssistResultMsg struct {
	id   int
	key  string
	text string
	err  error
}

type claudeAssistTickMsg struct {
	id int
}

type claudeAssistProgressMsg struct {
	id    int
	key   string
	event claude.Event
}

type createAIPromptResultMsg struct {
	id   int
	text string
	err  error
}

type createAIPromptTickMsg struct {
	id int
}

type createAIPromptProgressMsg struct {
	id    int
	event claude.Event
}

type createAIQuestion struct {
	Question string
	Answer   string
}

func NewModel(client worker.JiraClient, jql string, options ...Option) Model {
	model := Model{
		jql:                  jql,
		loading:              true,
		requestTimeout:       defaultRequestTimeout,
		workerCount:          defaultWorkerCount,
		queueSize:            defaultQueueSize,
		nextRequestID:        initialRequestID,
		activeRequestID:      initialRequestID,
		theme:                ui.NewTheme(config.DefaultTheme()),
		symbolMode:           symbolModeAuto,
		details:              make(map[string]jira.IssueDetail),
		detailFreshnessCache: ttlcache.New[string, struct{}](ttlcache.WithTTL[string, struct{}](issueDetailCacheTTL)),
		comments:             make(map[string][]jira.Comment),
		transitions:          make(map[string][]jira.Transition),
		editMetadata:         make(map[string]jira.EditMetadata),
		detailSectionOffset:  make(map[string]int),
		userSearchCache:      ttlcache.New[string, []jira.User](ttlcache.WithTTL[string, []jira.User](userSearchCacheTTL)),
		claudeRunner:         claude.LocalRunner{},
		now:                  time.Now,
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
		m.recordWorkerResult(resultDiagnosticEvent(msg.result))
		m, cmd = m.handleWorkerResult(msg.result)
		return m, tea.Batch(cmd, m.waitForWorkerResult())
	case claudePlanResultMsg:
		m = m.handleClaudePlanResult(msg)
		return m, nil
	case claudePlanTickMsg:
		if m.claudePlanLoading && msg.id == m.activeClaudePlanReqID {
			return m, m.scheduleClaudePlanTick(msg.id)
		}
		return m, nil
	case claudePlanProgressMsg:
		m = m.handleClaudePlanProgress(msg)
		if m.claudePlanLoading && msg.id == m.activeClaudePlanReqID {
			return m, m.waitForClaudePlanProgress(msg.id, msg.key)
		}
		return m, nil
	case claudeAssistResultMsg:
		m = m.handleClaudeAssistResult(msg)
		return m, nil
	case claudeAssistTickMsg:
		if m.claudeAssistLoading && msg.id == m.activeClaudeAssistReqID {
			return m, m.scheduleClaudeAssistTick(msg.id)
		}
		return m, nil
	case claudeAssistProgressMsg:
		m = m.handleClaudeAssistProgress(msg)
		if m.claudeAssistLoading && msg.id == m.activeClaudeAssistReqID {
			return m, m.waitForClaudeAssistProgress(msg.id, msg.key)
		}
		return m, nil
	case createAIPromptResultMsg:
		return m.handleCreateAIPromptResult(msg)
	case createAIPromptTickMsg:
		if m.createAIPromptLoading && msg.id == m.activeCreateAIPromptReqID {
			return m, m.scheduleCreateAIPromptTick(msg.id)
		}
		return m, nil
	case createAIPromptProgressMsg:
		m = m.handleCreateAIPromptProgress(msg)
		if m.createAIPromptLoading && msg.id == m.activeCreateAIPromptReqID {
			return m, m.waitForCreateAIPromptProgress(msg.id)
		}
		return m, nil
	case linkActionMsg:
		m.detailNotice = linkActionNotice(msg)
		return m, nil
	case workSubmittedMsg:
		m.recordDiagnosticEvent(diagnosticKindWorker, string(msg.kind), "submit", workerDiagnosticDetail(msg.id, msg.key, nil))
		return m, nil
	case workerStoppedMsg, noDetailRequestMsg:
		return m, nil
	case tea.PasteMsg:
		if m.createOpen {
			return m.updateCreatePaste(msg)
		}
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
		if m.diagnosticsOpen {
			switch msg.String() {
			case "esc", "ctrl+d":
				m.diagnosticsOpen = false
			}
			return m, nil
		}
		if msg.String() == "ctrl+d" {
			m.diagnosticsOpen = true
			return m, nil
		}
		if m.claudePlanOpen && msg.String() == "esc" {
			if m.claudePlanLoading {
				m = m.cancelClaudeTicketPlan()
				return m, nil
			}
			m.claudePlanOpen = false
			return m, nil
		}
		if m.claudePlanOpen && !m.claudePlanLoading && strings.TrimSpace(m.claudePlanText) != "" {
			switch msg.String() {
			case "j", "down":
				m.scrollClaudePlanResult(1)
				return m, nil
			case "k", "up":
				m.scrollClaudePlanResult(-1)
				return m, nil
			case "pgdown", "space", "ctrl+f":
				m.scrollClaudePlanResult(m.claudePlanResultRows())
				return m, nil
			case "pgup", "ctrl+b":
				m.scrollClaudePlanResult(-m.claudePlanResultRows())
				return m, nil
			case "g", "home":
				m.claudePlanOffset = 0
				return m, nil
			case "G", "end":
				m.scrollClaudePlanResult(1 << 20)
				return m, nil
			}
		}
		if m.claudeAssistOpen {
			if m.claudeAssistLoading && msg.String() == "esc" {
				m = m.cancelClaudeTicketAssist()
				return m, nil
			}
			if !m.claudeAssistLoading {
				return m.updateClaudeAssistEditor(msg)
			}
		}
		if m.inlineAIOpen {
			return m.updateInlineAIPicker(msg)
		}
		if m.createOpen {
			if m.createAIPromptOpen {
				return m.updateCreateAIPrompt(msg)
			}
			return m.updateCreateIssue(msg)
		}
		if m.mode == modeComment {
			return m.updateCommentComposer(msg)
		}
		if m.mode == modeDetail && m.summaryEditing {
			return m.updateSummaryEditor(msg)
		}
		if m.mode == modeDetail && m.assigneeFocus {
			return m.updateAssigneePicker(msg)
		}
		switch msg.String() {
		case "ctrl+c", "q":
			m.workers.Stop()
			return m, tea.Quit
		case "esc":
			if m.mode == modeDetail {
				if m.summaryFocus {
					m.summaryFocus = false
					m.detailNotice = ""
					return m, nil
				}
				if m.actionFocus {
					m.actionFocus = false
					m.detailNotice = ""
					return m, nil
				}
				if m.transitionFocus {
					m.transitionFocus = false
					m.detailNotice = ""
					return m, nil
				}
				if m.priorityFocus {
					m.priorityFocus = false
					m.detailNotice = ""
					return m, nil
				}
				if m.assigneeFocus {
					m.assigneeFocus = false
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
				m.transitionFocus = false
				m.priorityFocus = false
				m.assigneeFocus = false
				m.summaryFocus = false
				m.detailNotice = ""
			}
		case "r":
			m.err = nil
			return m.startRefresh()
		case "n":
			return m.startCreateIssue()
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
			if m.mode == modeDetail {
				return m.openSelectedIssue()
			}
			m.switchSort(1)
		case "O":
			if m.mode == modeTable {
				m.switchSort(-1)
			}
		case "y":
			if m.mode == modeDetail && m.linkFocus {
				return m.copySelectedDetailLink()
			}
			if m.canUseLinkSelection() {
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
				if m.inlineDescriptionAIAvailable() {
					return m.openInlineDescriptionAI()
				}
				if m.claudeAvailable() {
					m.jumpDetailSection("Claude")
					return m, nil
				}
				m.startCommentComposer()
				return m, nil
			}
		case "s":
			if m.mode == modeDetail {
				return m.startSummaryEditor()
			}
		case "p":
			if m.mode == modeDetail {
				return m.startPriorityEditor()
			}
		case "enter":
			if m.mode == modeDetail && m.linkFocus {
				return m.openSelectedDetailLink()
			}
			if m.mode == modeDetail && m.actionFocus {
				return m.runSelectedDetailAction()
			}
			if m.mode == modeDetail && m.transitionFocus {
				return m.submitSelectedTransition()
			}
			if m.mode == modeDetail && m.priorityFocus {
				return m.submitSelectedPriority()
			}
			if m.mode == modeDetail && m.assigneeFocus {
				return m.submitSelectedAssignee()
			}
			if m.mode == modeDetail && m.summaryFocus {
				return m.startSummaryEditor()
			}
			if m.mode == modeDetail && m.hierarchyFocus {
				return m.openSelectedHierarchyIssue()
			}
			if m.mode == modeDetail {
				return m.activateFocusedDetailTarget()
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
				if m.transitionFocus {
					m.moveSelectedTransition(-1)
					return m, nil
				}
				if m.priorityFocus {
					m.moveSelectedPriority(-1)
					return m, nil
				}
				if m.assigneeFocus {
					m.moveSelectedAssignee(-1)
					return m, nil
				}
				if m.hierarchyFocus {
					m.moveSelectedHierarchyIssue(-1)
					return m, nil
				}
				if m.canUseLinkSelection() {
					m.moveSelectedDetailLink(-1)
					return m, nil
				}
				if m.canUseActionSelection() {
					m.moveSelectedDetailAction(-1)
					return m, nil
				}
				if m.canUseClaudeSelection() {
					m.moveSelectedClaudeAction(-1)
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
				if m.transitionFocus {
					m.moveSelectedTransition(1)
					return m, nil
				}
				if m.priorityFocus {
					m.moveSelectedPriority(1)
					return m, nil
				}
				if m.assigneeFocus {
					m.moveSelectedAssignee(1)
					return m, nil
				}
				if m.hierarchyFocus {
					m.moveSelectedHierarchyIssue(1)
					return m, nil
				}
				if m.canUseLinkSelection() {
					m.moveSelectedDetailLink(1)
					return m, nil
				}
				if m.canUseActionSelection() {
					m.moveSelectedDetailAction(1)
					return m, nil
				}
				if m.canUseClaudeSelection() {
					m.moveSelectedClaudeAction(1)
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
				m.transitionFocus = false
				m.priorityFocus = false
				m.assigneeFocus = false
				return m, nil
			}
			m.selected = 0
			m.offset = 0
			return m.startDetailRequestForSelected()
		case "end", "G":
			if m.mode == modeDetail {
				m.scrollDetailToBottom()
				m.linkFocus = false
				m.transitionFocus = false
				m.priorityFocus = false
				m.assigneeFocus = false
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
				m.transitionFocus = false
				m.priorityFocus = false
				m.assigneeFocus = false
				m.jumpDetailSection("Description")
				return m, nil
			}
		case "h":
			if m.mode == modeDetail {
				m.linkFocus = false
				m.hierarchyFocus = false
				m.actionFocus = false
				m.transitionFocus = false
				m.priorityFocus = false
				m.assigneeFocus = false
				m.jumpDetailSection("Hierarchy")
				return m, nil
			}
		case "m":
			if m.mode == modeDetail {
				m.linkFocus = false
				m.hierarchyFocus = false
				m.actionFocus = false
				m.transitionFocus = false
				m.priorityFocus = false
				m.assigneeFocus = false
				m.jumpDetailSection("Comments")
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
		if result.ID == m.activeClaudeAssistCommentReqID {
			return m.handleClaudeAssistCommentResult(result)
		}
		return m.handleAddCommentResult(result)
	case worker.KindSearchUsers:
		return m.handleUserSearchResult(result), nil
	case worker.KindExpandIssues:
		return m.handleExpandIssuesResult(result), nil
	case worker.KindGetTransitions:
		return m.handleGetTransitionsResult(result), nil
	case worker.KindTransitionIssue:
		return m.handleTransitionIssueResult(result), nil
	case worker.KindGetEditMetadata:
		return m.handleEditMetadataResult(result), nil
	case worker.KindUpdateSummary:
		if result.ID == m.activeClaudeAssistSummaryReqID {
			return m.handleClaudeAssistApplyResult(result), nil
		}
		return m.handleUpdateSummaryResult(result), nil
	case worker.KindUpdateDescription:
		return m.handleClaudeAssistApplyResult(result), nil
	case worker.KindUpdatePriority:
		return m.handleUpdatePriorityResult(result), nil
	case worker.KindUpdateAssignee:
		return m.handleUpdateAssigneeResult(result), nil
	case worker.KindGetCreateIssueTypes:
		return m.handleGetCreateIssueTypesResult(result), nil
	case worker.KindGetCreateFields:
		return m.handleGetCreateFieldsResult(result), nil
	case worker.KindCreateIssue:
		return m.handleCreateIssueResult(result), nil
	default:
		return m, nil
	}
}

func (m Model) handleAssigneeSearchResult(result worker.Result) Model {
	if result.ID != m.assigneeSearchReqID {
		return m
	}
	m.assigneeSearchLoading = false
	if result.Err != nil {
		m.assigneeSearchErr = result.Err
		m.detailNotice = "Assignee search failed: " + result.Err.Error()
		return m
	}
	if result.SearchUsers == nil {
		m.assigneeSearchErr = worker.ErrInvalidRequest
		m.detailNotice = "Assignee search failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	if result.SearchUsers.Query != m.assigneeQuery {
		return m
	}
	m.cacheUserSearch(result.SearchUsers.Query, result.SearchUsers.Users)
	m.assigneeUsers = result.SearchUsers.Users
	m.selectedAssignee = clamp(m.selectedAssignee, 0, max(0, len(m.assigneeUsers)-1))
	m.assigneeSearchErr = nil
	m.detailNotice = ""
	return m
}

func (m Model) cachedUserSearch(query string) ([]jira.User, bool) {
	if m.userSearchCache == nil {
		return nil, false
	}
	item := m.userSearchCache.Get(userSearchCacheKey(query))
	if item == nil {
		return nil, false
	}
	return item.Value(), true
}

func (m Model) cacheUserSearch(query string, users []jira.User) {
	if m.userSearchCache == nil || strings.TrimSpace(query) == "" {
		return
	}
	m.userSearchCache.Set(userSearchCacheKey(query), users, ttlcache.DefaultTTL)
}

func userSearchCacheKey(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

func (m Model) handleEditMetadataResult(result worker.Result) Model {
	if result.ID != m.activeSummaryMetadataReqID && result.ID != m.activePriorityMetadataReqID {
		return m
	}
	isPriorityRequest := result.ID == m.activePriorityMetadataReqID
	if isPriorityRequest {
		m.priorityMetadataLoading = false
	} else {
		m.summaryMetadataLoading = false
	}
	if result.Err != nil {
		if isPriorityRequest {
			m.priorityMetadataErr = result.Err
			m.detailNotice = "Priority metadata failed: " + result.Err.Error()
		} else {
			m.summaryMetadataErr = result.Err
			m.detailNotice = "Summary metadata failed: " + result.Err.Error()
		}
		return m
	}
	if result.GetEditMetadata == nil {
		if isPriorityRequest {
			m.priorityMetadataErr = worker.ErrInvalidRequest
			m.detailNotice = "Priority metadata failed: " + worker.ErrInvalidRequest.Error()
		} else {
			m.summaryMetadataErr = worker.ErrInvalidRequest
			m.detailNotice = "Summary metadata failed: " + worker.ErrInvalidRequest.Error()
		}
		return m
	}
	requestKey := m.summaryMetadataRequestKey
	if isPriorityRequest {
		requestKey = m.priorityMetadataRequestKey
	}
	if result.GetEditMetadata.Key != requestKey {
		return m
	}
	selected, ok := m.selectedIssue()
	if !ok || selected.Key != result.GetEditMetadata.Key {
		return m
	}
	if m.editMetadata == nil {
		m.editMetadata = make(map[string]jira.EditMetadata)
	}
	m.editMetadata[result.GetEditMetadata.Key] = result.GetEditMetadata.Metadata
	if isPriorityRequest {
		m.priorityMetadataErr = nil
		return m.beginPriorityEditing(result.GetEditMetadata.Metadata)
	}
	m.summaryMetadataErr = nil
	if !result.GetEditMetadata.Metadata.Summary.Editable {
		m.detailNotice = "Summary is not editable for " + result.GetEditMetadata.Key + "."
		return m
	}
	m.beginSummaryEditing()
	return m
}

func (m Model) handleUpdateSummaryResult(result worker.Result) Model {
	if result.ID != m.activeSummaryReqID {
		return m
	}
	m.summarySubmitting = false
	if result.Err != nil {
		m.detailNotice = "Summary update failed: " + result.Err.Error()
		return m
	}
	if result.UpdateSummary == nil {
		m.detailNotice = "Summary update failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	if result.UpdateSummary.Key != m.summarySubmitKey {
		return m
	}
	m.updateIssueSummary(result.UpdateSummary.Key, result.UpdateSummary.Summary)
	m.summaryEditing = false
	m.summaryDirty = false
	m.summarySubmitKey = ""
	m.summarySubmitValue = ""
	m.detailNotice = "Summary updated."
	return m
}

func (m Model) handleClaudeAssistApplyResult(result worker.Result) Model {
	if !m.claudeAssistApplying {
		return m
	}
	switch result.Kind {
	case worker.KindUpdateSummary:
		if result.ID != m.activeClaudeAssistSummaryReqID {
			return m
		}
		if result.Err != nil {
			m.claudeAssistApplying = false
			m.claudeAssistConfirmApply = false
			m.detailNotice = "Ticket assist apply failed: " + result.Err.Error()
			return m
		}
		if result.UpdateSummary == nil {
			m.claudeAssistApplying = false
			m.detailNotice = "Ticket assist apply failed: " + worker.ErrInvalidRequest.Error()
			return m
		}
		m.updateIssueSummary(result.UpdateSummary.Key, result.UpdateSummary.Summary)
		m.claudeAssistSummaryApplied = true
	case worker.KindUpdateDescription:
		if result.ID != m.activeClaudeAssistDescriptionReqID {
			return m
		}
		if result.Err != nil {
			m.claudeAssistApplying = false
			m.claudeAssistConfirmApply = false
			m.detailNotice = "Ticket assist apply failed: " + result.Err.Error()
			return m
		}
		if result.UpdateDescription == nil {
			m.claudeAssistApplying = false
			m.detailNotice = "Ticket assist apply failed: " + worker.ErrInvalidRequest.Error()
			return m
		}
		m.updateIssueDescription(result.UpdateDescription.Key, result.UpdateDescription.Description)
		m.claudeAssistDescriptionApplied = true
	default:
		return m
	}
	if m.claudeAssistSummaryApplied && m.claudeAssistDescriptionApplied {
		m.claudeAssistApplying = false
		m.claudeAssistConfirmApply = false
		m.claudeAssistOpen = false
		m.activeClaudeAssistSummaryReqID = 0
		m.activeClaudeAssistDescriptionReqID = 0
		m.detailNotice = "Ticket assist draft applied."
	}
	return m
}

func (m Model) handleClaudeAssistCommentResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeClaudeAssistCommentReqID {
		return m, nil
	}
	m.claudeAssistPostingComment = false
	if result.Err != nil {
		m.detailNotice = "Ticket assist comment failed: " + result.Err.Error()
		return m, nil
	}
	if result.AddComment == nil {
		m.detailNotice = "Ticket assist comment failed: " + worker.ErrInvalidRequest.Error()
		return m, nil
	}
	key := result.AddComment.Key
	m.claudeAssistConfirmComment = false
	m.claudeAssistOpen = false
	m.activeClaudeAssistCommentReqID = 0
	m.detailNotice = "Ticket assist draft posted as a comment."
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

func (m Model) handleUpdatePriorityResult(result worker.Result) Model {
	if result.ID != m.activePriorityReqID {
		return m
	}
	m.prioritySubmitting = false
	if result.Err != nil {
		m.detailNotice = "Priority update failed: " + result.Err.Error()
		return m
	}
	if result.UpdatePriority == nil {
		m.detailNotice = "Priority update failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	if result.UpdatePriority.Key != m.prioritySubmitKey {
		return m
	}
	priorityName := displayValue(result.UpdatePriority.Priority.Name, result.UpdatePriority.Priority.ID)
	m.updateIssuePriority(result.UpdatePriority.Key, priorityName)
	m.priorityFocus = false
	m.prioritySubmitKey = ""
	m.prioritySubmitValue = jira.FieldOption{}
	m.detailNotice = "Priority updated to " + displayValue(priorityName, "Unknown") + "."
	return m
}

func (m Model) handleUpdateAssigneeResult(result worker.Result) Model {
	if result.ID != m.activeAssigneeReqID {
		return m
	}
	m.assigneeSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Assignee update failed: " + result.Err.Error()
		return m
	}
	if result.UpdateAssignee == nil {
		m.detailNotice = "Assignee update failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	if result.UpdateAssignee.Key != m.assigneeSubmitKey {
		return m
	}
	assigneeName := displayValue(result.UpdateAssignee.Assignee.DisplayName, result.UpdateAssignee.Assignee.Email)
	m.updateIssueAssignee(result.UpdateAssignee.Key, assigneeName)
	m.assigneeFocus = false
	m.assigneeSubmitKey = ""
	m.assigneeSubmitValue = jira.User{}
	m.detailNotice = "Assignee updated to " + displayValue(assigneeName, "Unknown") + "."
	return m
}

func (m Model) handleGetCreateIssueTypesResult(result worker.Result) Model {
	if result.ID != m.activeCreateIssueTypesReqID {
		return m
	}
	m.createIssueTypesLoading = false
	if result.Err != nil {
		m.createIssueTypesErr = result.Err
		return m
	}
	if result.GetCreateIssueTypes == nil {
		m.createIssueTypesErr = worker.ErrInvalidRequest
		return m
	}
	if result.GetCreateIssueTypes.ProjectKey != m.createProjectKey {
		return m
	}
	m.createIssueTypes = result.GetCreateIssueTypes.IssueTypes
	m.selectedCreateIssueType = clamp(m.selectedCreateIssueType, 0, max(0, len(m.createIssueTypes)-1))
	m.createIssueTypesErr = nil
	return m
}

func (m Model) handleGetCreateFieldsResult(result worker.Result) Model {
	if result.ID != m.activeCreateFieldsReqID {
		return m
	}
	m.createFieldsLoading = false
	if result.Err != nil {
		m.createFieldsErr = result.Err
		return m
	}
	if result.GetCreateFields == nil {
		m.createFieldsErr = worker.ErrInvalidRequest
		return m
	}
	if result.GetCreateFields.ProjectKey != m.createProjectKey || result.GetCreateFields.IssueTypeID != m.createIssueType.ID {
		return m
	}
	m.createFields = result.GetCreateFields.Fields
	m.createFieldsErr = nil
	m.beginCreateForm()
	m.applyCreateAIFieldDrafts()
	return m
}

func (m Model) handleCreateIssueResult(result worker.Result) Model {
	if result.ID != m.activeCreateIssueReqID {
		return m
	}
	m.createSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Create issue failed: " + result.Err.Error()
		return m
	}
	if result.CreateIssue == nil {
		m.detailNotice = "Create issue failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	created := result.CreateIssue.Issue
	if strings.TrimSpace(created.Key) == "" {
		m.detailNotice = "Create issue failed: empty Jira response."
		return m
	}
	m.createOpen = false
	m.issues = prependIssue(m.issues, created)
	m.selected = 0
	m.offset = 0
	m.mode = modeTable
	m.detailNotice = "Created " + created.Key + "."
	m.resetCreateIssueState()
	return m
}

func (m Model) handleGetTransitionsResult(result worker.Result) Model {
	if result.ID != m.activeTransitionsReqID {
		return m
	}
	m.transitionLoading = false
	if result.Err != nil {
		m.transitionErr = result.Err
		m.detailNotice = "Transitions failed: " + result.Err.Error()
		return m
	}
	if result.GetTransitions == nil {
		m.transitionErr = worker.ErrInvalidRequest
		m.detailNotice = "Transitions failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	if result.GetTransitions.Key != m.transitionRequestKey {
		return m
	}
	selected, ok := m.selectedIssue()
	if !ok || selected.Key != result.GetTransitions.Key {
		return m
	}
	if m.transitions == nil {
		m.transitions = make(map[string][]jira.Transition)
	}
	m.transitions[result.GetTransitions.Key] = result.GetTransitions.Transitions
	m.selectedTransition = clamp(m.selectedTransition, 0, max(0, len(result.GetTransitions.Transitions)-1))
	m.transitionFocus = len(result.GetTransitions.Transitions) > 0
	m.transitionErr = nil
	if len(result.GetTransitions.Transitions) == 0 {
		m.detailNotice = "No available status transitions for " + result.GetTransitions.Key + "."
	} else {
		m.detailNotice = fmt.Sprintf("Loaded %d status transitions for %s.", len(result.GetTransitions.Transitions), result.GetTransitions.Key)
	}
	return m
}

func (m Model) handleTransitionIssueResult(result worker.Result) Model {
	if result.ID != m.activeTransitionReqID {
		return m
	}
	m.transitionSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Status update failed: " + result.Err.Error()
		return m
	}
	if result.TransitionIssue == nil {
		m.detailNotice = "Status update failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	if result.TransitionIssue.Key != m.transitionSubmitKey {
		return m
	}
	m.updateIssueStatus(result.TransitionIssue.Key, result.TransitionIssue.ToStatus)
	m.transitionFocus = false
	m.transitionSubmitKey = ""
	m.transitionSubmitToStatus = ""
	m.detailNotice = "Status updated to " + displayValue(result.TransitionIssue.ToStatus, "Unknown") + "."
	return m
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
	m.markIssueDetailFresh(result.GetIssue.Key)
	m.detailErr = nil
	return m
}

func (m Model) isIssueDetailFresh(key string) bool {
	if m.detailFreshnessCache == nil || strings.TrimSpace(key) == "" {
		return false
	}
	return m.detailFreshnessCache.Get(key) != nil
}

func (m Model) markIssueDetailFresh(key string) {
	if m.detailFreshnessCache == nil || strings.TrimSpace(key) == "" {
		return
	}
	m.detailFreshnessCache.Set(key, struct{}{}, ttlcache.DefaultTTL)
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
	if result.ID == m.assigneeSearchReqID {
		return m.handleAssigneeSearchResult(result)
	}
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

	if m.diagnosticsOpen {
		b.WriteString(m.renderDiagnostics(layout))
		b.WriteString("\n\n")
		b.WriteString(m.renderFooterHelp(keyContextDiagnostics, layout))
		return b.String()
	}

	if m.createOpen {
		b.WriteString(m.renderCreateIssue(layout))
		b.WriteString("\n\n")
		b.WriteString(m.renderModelFooterHelp(layout))
		return b.String()
	}

	if m.mode == modeComment && len(m.issues) > 0 {
		b.WriteString(m.renderCommentComposer(layout))
		b.WriteString("\n\n")
		b.WriteString(m.renderModelFooterHelp(layout))
		return b.String()
	}

	if m.mode == modeDetail && len(m.issues) > 0 {
		b.WriteString(m.renderFullDetail(layout))
		b.WriteString("\n\n")
		b.WriteString(m.renderModelFooterHelp(layout))
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
	return m.renderFooterHelpWithBindings(context, footerBindings(context), layout)
}

func (m Model) renderModelFooterHelp(layout browserLayout) string {
	context := activeKeyContext(m)
	bindings := footerBindings(context)
	if context == keyContextDetail {
		bindings = m.detailFooterBindings()
	}
	if context == keyContextCreate {
		bindings = m.createFooterBindings()
	}
	return m.renderFooterHelpWithBindings(context, bindings, layout)
}

func (m Model) createFooterBindings() []keyBinding {
	bindings := []keyBinding{
		{Keys: []string{"?"}, Label: "help", Description: "Open the keyboard help screen.", Group: "Global", Footer: true},
		{Keys: []string{"ctrl+d"}, Label: "diagnostics", Description: "Open recent background worker and cache activity.", Group: "Global", Footer: true},
		{Keys: []string{"esc"}, Label: "cancel", Description: "Cancel ticket creation.", Group: "Create", Footer: true},
	}
	switch {
	case m.createIssueTypesLoading, m.createIssueTypesErr != nil:
		return bindings
	case len(m.createIssueTypes) == 0 && m.createIssueType.ID == "":
		return bindings
	case m.createIssueType.ID == "":
		if m.claudeCreateTicketDraftEnabled() {
			bindings = append(bindings, keyBinding{Keys: []string{"tab"}, Label: "mode", Description: "Switch between manual and AI generated ticket creation.", Group: "Create", Footer: true})
		}
		if m.createAIGeneratedMode {
			return append(bindings,
				keyBinding{Keys: []string{"ctrl+s"}, Label: "generate", Description: "Ask Claude to generate a local ticket draft.", Group: "Create", Footer: true},
			)
		}
		return append(bindings,
			keyBinding{Keys: []string{"up", "down", "j", "k"}, FooterKey: "j/k", Label: "type", Description: "Select an issue type while choosing ticket type.", Group: "Create", Footer: true},
			keyBinding{Keys: []string{"enter"}, Label: "continue", Description: "Continue with the selected issue type.", Group: "Create", Footer: true},
		)
	case m.createFieldsLoading, m.createFieldsErr != nil:
		return bindings
	default:
		return append(bindings,
			keyBinding{Keys: []string{"tab"}, Label: "field", Description: "Move between Summary and Description.", Group: "Create", Footer: true},
			keyBinding{Keys: []string{"ctrl+s"}, Label: "submit", Description: "Create the ticket.", Group: "Create", Footer: true},
		)
	}
}

func (m Model) renderFooterHelpWithBindings(context keyContext, bindings []keyBinding, layout browserLayout) string {
	available := max(20, layout.contentWidth)
	rendered := m.footerContextLabel(context, available)
	currentGroup := ""
	for _, binding := range bindings {
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

func (m Model) detailFooterBindings() []keyBinding {
	base := footerBindings(keyContextDetail)
	if target, ok := m.focusedDetailTarget(); ok && target.Kind == detailTargetField {
		fieldBindings := []keyBinding{
			{Keys: []string{"enter"}, Label: "edit", Group: "Field", Footer: true},
		}
		filtered := make([]keyBinding, 0, len(base)+len(fieldBindings))
		for _, binding := range base {
			if binding.keyText() == "enter" || binding.keyText() == "s" || binding.keyText() == "p" {
				continue
			}
			filtered = append(filtered, binding)
		}
		if len(filtered) == 0 {
			return fieldBindings
		}
		result := make([]keyBinding, 0, len(filtered)+len(fieldBindings))
		result = append(result, filtered[0])
		result = append(result, fieldBindings...)
		result = append(result, filtered[1:]...)
		return result
	}
	section, ok := m.focusedDetailSection()
	if !ok {
		return base
	}
	var sectionBindings []keyBinding
	switch section.ID {
	case "description":
		if m.inlineDescriptionAIAvailable() {
			sectionBindings = []keyBinding{
				{Keys: []string{"a"}, Label: "AI", Group: "Section", Footer: true},
			}
		}
	case "hierarchy":
		if len(m.currentHierarchyChildren()) > 0 {
			sectionBindings = []keyBinding{
				{Keys: []string{"j", "k", "up", "down"}, FooterKey: "j/k", Label: "child", Group: "Section", Footer: true},
				{Keys: []string{"enter"}, Label: "focus", Group: "Section", Footer: true},
			}
		}
	case "links":
		if len(m.currentDetailLinks()) > 0 {
			sectionBindings = []keyBinding{
				{Keys: []string{"j", "k", "up", "down"}, FooterKey: "j/k", Label: "link", Group: "Section", Footer: true},
				{Keys: []string{"enter"}, Label: "focus", Group: "Section", Footer: true},
				{Keys: []string{"y"}, Label: "copy", Group: "Section", Footer: true},
			}
		}
	case "actions":
		sectionBindings = []keyBinding{
			{Keys: []string{"j", "k", "up", "down"}, FooterKey: "j/k", Label: "action", Group: "Section", Footer: true},
			{Keys: []string{"enter"}, Label: "focus", Group: "Section", Footer: true},
		}
	case "status":
		sectionBindings = []keyBinding{
			{Keys: []string{"enter"}, Label: "transition", Group: "Section", Footer: true},
		}
	}
	if len(sectionBindings) == 0 {
		return base
	}
	filtered := make([]keyBinding, 0, len(base)+len(sectionBindings))
	for _, binding := range base {
		if binding.FooterKey == "j/k" || binding.keyText() == "enter" {
			continue
		}
		filtered = append(filtered, binding)
	}
	result := make([]keyBinding, 0, len(filtered)+len(sectionBindings))
	if len(filtered) == 0 {
		return append(result, sectionBindings...)
	}
	result = append(result, filtered[0])
	result = append(result, sectionBindings...)
	result = append(result, filtered[1:]...)
	return result
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

func (m Model) renderDiagnostics(layout browserLayout) string {
	rows := m.boundedPanelBodyRows(12)
	events := m.recentDiagnosticEvents(rows)
	var b strings.Builder
	b.WriteString(m.detailSectionHeader("diagnostics", "Diagnostics", "Background Activity", max(32, layout.contentWidth-4)))
	b.WriteString("\n\n")
	if len(events) == 0 {
		b.WriteString(m.theme.Muted.Render("No background activity recorded yet."))
		return m.theme.ActivePane.Width(layout.contentWidth).Render(strings.TrimRight(b.String(), "\n"))
	}
	b.WriteString(m.renderDiagnosticsSummary(events, max(20, layout.contentWidth-6)))
	b.WriteString("\n\n")
	b.WriteString(m.theme.Muted.Render(fmt.Sprintf("%-8s  %-8s  %-8s  %s", "TIME", "KIND", "STATUS", "DETAIL")))
	for _, event := range events {
		line := fmt.Sprintf("%-8s  %-8s  %-8s  %s", event.At.Format("15:04:05"), event.Kind, event.Status, diagnosticEventDetail(event))
		b.WriteString("\n")
		b.WriteString(truncate(line, max(20, layout.contentWidth-6)))
	}
	return m.theme.ActivePane.Width(layout.contentWidth).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) renderDiagnosticsSummary(events []diagnosticEvent, width int) string {
	stats := diagnosticStatsFor(events)
	last := "none"
	if len(events) > 0 {
		event := events[len(events)-1]
		last = strings.TrimSpace(event.Label + " " + event.Status)
	}
	summary := fmt.Sprintf("Workers %d   Cache %d   Errors %d   Active %d   Last %s", stats.Workers, stats.Cache, stats.Errors, stats.Active, last)
	bars := fmt.Sprintf("Activity  worker %s  cache  %s", diagnosticActivityBar(stats.Workers, len(events), 12), diagnosticActivityBar(stats.Cache, len(events), 12))
	return truncate(summary, width) + "\n" + truncate(bars, width)
}

type diagnosticStats struct {
	Workers int
	Cache   int
	Errors  int
	Active  int
}

func diagnosticStatsFor(events []diagnosticEvent) diagnosticStats {
	var stats diagnosticStats
	activeRequests := make(map[string]struct{})
	for _, event := range events {
		switch event.Kind {
		case diagnosticKindWorker:
			stats.Workers++
			switch event.Status {
			case "submit":
				if requestID := diagnosticEventRequestID(event); requestID != "" {
					activeRequests[requestID] = struct{}{}
				} else {
					stats.Active++
				}
			case "ok", "error":
				if requestID := diagnosticEventRequestID(event); requestID != "" {
					delete(activeRequests, requestID)
				} else {
					stats.Active = max(0, stats.Active-1)
				}
			}
		case diagnosticKindCache:
			stats.Cache++
		}
		if event.Status == "error" {
			stats.Errors++
		}
	}
	stats.Active += len(activeRequests)
	return stats
}

func diagnosticActivityBar(count int, total int, width int) string {
	if width <= 0 {
		return "[]"
	}
	filled := 0
	if total > 0 && count > 0 {
		filled = max(1, min(width, count*width/total))
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", max(0, width-filled)) + "]"
}

func diagnosticEventDetail(event diagnosticEvent) string {
	detail := strings.TrimSpace(event.Detail)
	label := strings.TrimSpace(event.Label)
	switch {
	case label == "":
		return detail
	case detail == "":
		return label
	case strings.HasPrefix(detail, label+" "):
		return detail
	default:
		return label + " " + detail
	}
}

func diagnosticEventRequestID(event diagnosticEvent) string {
	for _, field := range strings.Fields(event.Detail) {
		if strings.HasPrefix(field, "#") {
			return field
		}
	}
	return ""
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

func (m *Model) recordDiagnosticEvent(kind diagnosticKind, label string, status string, detail string) {
	if label == "" && detail == "" {
		return
	}
	m.diagnosticsEvents = append(m.diagnosticsEvents, diagnosticEvent{
		At:     time.Now(),
		Kind:   kind,
		Label:  label,
		Status: status,
		Detail: detail,
	})
	if len(m.diagnosticsEvents) > maxDiagnosticsEvents {
		start := len(m.diagnosticsEvents) - maxDiagnosticsEvents
		m.diagnosticsEvents = append([]diagnosticEvent(nil), m.diagnosticsEvents[start:]...)
	}
}

func (m Model) recentDiagnosticEvents(limit int) []diagnosticEvent {
	if limit <= 0 || len(m.diagnosticsEvents) == 0 {
		return nil
	}
	start := max(0, len(m.diagnosticsEvents)-limit)
	events := append([]diagnosticEvent(nil), m.diagnosticsEvents[start:]...)
	return events
}

func resultDiagnosticEvent(result worker.Result) diagnosticEvent {
	status := "ok"
	if result.Err != nil {
		status = "error"
	}
	detailParts := []string{workerDiagnosticDetail(result.ID, resultDiagnosticKey(result), result.Err)}
	if metrics := resultDiagnosticMetrics(result); metrics != "" {
		detailParts = append(detailParts, metrics)
	}
	return diagnosticEvent{
		Kind:   diagnosticKindWorker,
		Label:  string(result.Kind),
		Status: status,
		Detail: strings.Join(detailParts, " "),
	}
}

func (m *Model) recordWorkerResult(event diagnosticEvent) {
	m.recordDiagnosticEvent(event.Kind, event.Label, event.Status, event.Detail)
}

func workerDiagnosticDetail(id int, key string, err error) string {
	var parts []string
	if id > 0 {
		parts = append(parts, fmt.Sprintf("#%d", id))
	}
	if key != "" {
		parts = append(parts, key)
	}
	if err != nil {
		parts = append(parts, truncate(err.Error(), 80))
	}
	return strings.Join(parts, " ")
}

func resultDiagnosticKey(result worker.Result) string {
	switch {
	case result.GetIssue != nil:
		return result.GetIssue.Key
	case result.GetComments != nil:
		return result.GetComments.Key
	case result.AddComment != nil:
		return result.AddComment.Key
	case result.SearchUsers != nil:
		return result.SearchUsers.Query
	case result.ExpandIssues != nil:
		return result.ExpandIssues.ParentKey
	case result.GetTransitions != nil:
		return result.GetTransitions.Key
	case result.TransitionIssue != nil:
		return result.TransitionIssue.Key
	case result.GetEditMetadata != nil:
		return result.GetEditMetadata.Key
	case result.GetCreateIssueTypes != nil:
		return result.GetCreateIssueTypes.ProjectKey
	case result.GetCreateFields != nil:
		return strings.TrimSpace(result.GetCreateFields.ProjectKey + " " + result.GetCreateFields.IssueTypeID)
	case result.CreateIssue != nil:
		return result.CreateIssue.Issue.Key
	case result.UpdateSummary != nil:
		return result.UpdateSummary.Key
	case result.UpdateDescription != nil:
		return result.UpdateDescription.Key
	case result.UpdatePriority != nil:
		return result.UpdatePriority.Key
	case result.UpdateAssignee != nil:
		return result.UpdateAssignee.Key
	default:
		return ""
	}
}

func resultDiagnosticMetrics(result worker.Result) string {
	switch {
	case result.GetCreateIssueTypes != nil:
		return fmt.Sprintf("types=%d", len(result.GetCreateIssueTypes.IssueTypes))
	case result.GetCreateFields != nil:
		fields := result.GetCreateFields.Fields
		return fmt.Sprintf(
			"fields=%d supported=%d required_unsupported=%d sample=%s",
			len(fields),
			len(supportedCreateFields(fields)),
			len(unsupportedRequiredCreateFields(fields)),
			createFieldDiagnosticSample(fields, 6),
		)
	default:
		return ""
	}
}

func createFieldDiagnosticSample(fields []jira.CreateField, limit int) string {
	if limit <= 0 || len(fields) == 0 {
		return "-"
	}
	var parts []string
	for index, field := range fields {
		if index >= limit {
			parts = append(parts, "...")
			break
		}
		id := displayValue(field.ID, field.Key)
		name := displayValue(field.Name, id)
		schema := displayValue(field.SchemaSystem, displayValue(field.SchemaType, "unknown"))
		parts = append(parts, strings.ReplaceAll(fmt.Sprintf("%s/%s/%s", id, name, schema), " ", "_"))
	}
	return strings.Join(parts, ",")
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

func indexFieldOptionByName(options []jira.FieldOption, name string) int {
	name = strings.TrimSpace(name)
	for index, option := range options {
		if strings.EqualFold(strings.TrimSpace(option.Name), name) {
			return index
		}
	}
	return 0
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
	if overlay := m.renderDetailOverlay(layout); overlay != "" {
		body = placeDetailOverlay(body, overlay, m.fullDetailRows())
	}
	content := header + "\n\n" + body
	return m.theme.ActivePane.Width(layout.contentWidth).Render(content)
}

func placeDetailOverlay(body string, overlay string, rows int) string {
	if strings.TrimSpace(overlay) == "" {
		return body
	}
	rows = max(1, rows)
	overlayLines := strings.Split(strings.TrimRight(overlay, "\n"), "\n")
	rows = max(rows, len(overlayLines))
	bodyLines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for len(bodyLines) < rows {
		bodyLines = append(bodyLines, "")
	}
	if len(bodyLines) > rows {
		bodyLines = bodyLines[:rows]
	}
	if len(overlayLines) == 0 {
		return strings.Join(bodyLines, "\n")
	}
	start := max(0, (rows-len(overlayLines))/2)
	for index, line := range overlayLines {
		target := start + index
		if target >= rows {
			break
		}
		bodyLines[target] = line
	}
	return strings.Join(bodyLines, "\n")
}

func (m Model) renderDetailOverlay(layout browserLayout) string {
	width := max(32, layout.contentWidth-12)
	if m.summaryMetadataLoading {
		return m.renderSummaryLoadingDialog(width)
	}
	if m.summaryEditing || m.summarySubmitting {
		return m.renderSummaryDialog(width)
	}
	if m.priorityMetadataLoading {
		return m.renderPriorityLoadingDialog(width)
	}
	if m.priorityFocus || m.prioritySubmitting {
		return m.renderPriorityDialog(width)
	}
	if m.assigneeFocus || m.assigneeSubmitting {
		return m.renderAssigneeDialog(width)
	}
	if m.transitionFocus || m.transitionSubmitting {
		return m.renderStatusTransitionDialog(width)
	}
	if m.inlineAIOpen {
		return m.renderInlineAIDialog(width)
	}
	if m.claudeAssistLoading || m.claudeAssistOpen {
		return m.renderClaudeAssistDialog(width)
	}
	if m.claudePlanLoading || m.claudePlanOpen {
		return m.renderClaudePlanDialog(width)
	}
	return ""
}

func (m Model) renderSummaryLoadingDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	body := m.detailStatusBlock("Loading summary metadata...", bodyWidth, false)
	return m.renderDetailDialog(width, "Edit Summary", selected.Key, body, "esc cancel")
}

func (m Model) renderPriorityLoadingDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	body := m.detailStatusBlock("Loading priority metadata...", bodyWidth, false)
	return m.renderDetailDialog(width, "Change Priority", selected.Key, body, "esc cancel")
}

func (m Model) renderDetailDialog(width int, title, subtitle, body, footer string) string {
	return m.renderDetailDialogWithLimit(width, title, subtitle, body, footer, 72)
}

func (m Model) renderDetailDialogWithLimit(width int, title, subtitle, body, footer string, maxDialogWidth int) string {
	dialogWidth := min(max(44, width), maxDialogWidth)
	contentWidth := max(24, dialogWidth-4)
	var lines []string
	lines = append(lines, m.theme.PaneTitle.Render(truncate(title, contentWidth)))
	if strings.TrimSpace(subtitle) != "" {
		lines = append(lines, m.theme.Muted.Render(truncate(subtitle, contentWidth)))
	}
	if strings.TrimSpace(body) != "" {
		lines = append(lines, "")
		lines = append(lines, body)
	}
	if strings.TrimSpace(footer) != "" {
		lines = append(lines, "")
		lines = append(lines, m.theme.Muted.Render(truncate(footer, contentWidth)))
	}
	dialog := lipgloss.NewStyle().
		Width(contentWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Muted.GetForeground()).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, dialog)
}

func (m Model) renderCreateIssue(layout browserLayout) string {
	if m.createAIPromptOpen {
		return m.renderCreateAIPrompt(layout)
	}
	width := layout.contentWidth
	dialogWidth := m.createDialogMaxWidth(width)
	bodyWidth := max(24, dialogWidth-4)
	var lines []string
	focusLine := -1
	subtitle := displayValue(m.createProjectKey, "Project unknown")
	footer := "esc cancel"
	switch {
	case m.createIssueTypesLoading:
		lines = append(lines, m.detailStatusBlock("Loading issue types...", bodyWidth, false))
	case m.createIssueTypesErr != nil:
		lines = append(lines, m.renderDetailNotice("Issue type metadata failed: "+m.createIssueTypesErr.Error(), bodyWidth))
	case len(m.createIssueTypes) == 0 && m.createIssueType.ID == "":
		lines = append(lines, m.detailEmptyState("Jira returned 0 creatable issue types for "+displayValue(m.createProjectKey, "this project")+". Press ctrl+d for request diagnostics.", bodyWidth))
	case m.createIssueType.ID == "":
		if m.claudeCreateTicketDraftEnabled() {
			lines = append(lines, m.renderCreateModeTabs(bodyWidth), "")
		}
		if m.createAIGeneratedMode {
			lines = append(lines, m.renderCreateAIPromptBody(bodyWidth)...)
			if m.createAIPromptLoading {
				footer = "esc cancel"
			} else {
				footer = "tab mode  ctrl+s generate  esc cancel"
			}
		} else {
			lines = append(lines, m.renderCreateIssueTypePickerLines()...)
			if m.detailNotice != "" {
				lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
			}
			if m.claudeCreateTicketDraftEnabled() {
				footer = "tab mode  j/k select  enter continue  esc cancel"
			} else {
				footer = "j/k select  enter continue  esc cancel"
			}
		}
	case m.createFieldsLoading:
		lines = append(lines, m.detailStatusBlock("Loading create fields...", bodyWidth, false))
	case m.createFieldsErr != nil:
		lines = append(lines, m.renderDetailNotice("Create fields failed: "+m.createFieldsErr.Error(), bodyWidth))
	case m.createChangingType:
		lines = append(lines, m.renderCreateIssueTypePickerLines()...)
		if m.detailNotice != "" {
			lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
		}
		footer = "j/k select  enter change  esc keep"
	default:
		if m.createFieldFocus == createTypeFieldIndex {
			focusLine = len(lines)
		}
		lines = append(lines, m.createFieldLabel("Type", createTypeFieldIndex))
		lines = append(lines, m.theme.Text.Render(displayValue(m.createIssueType.Name, m.createIssueType.ID)))
		if m.createFieldFocus == createTypeFieldIndex {
			lines = append(lines, m.theme.Muted.Render("Press enter to change issue type."))
		}
		lines = append(lines, "")
		if m.createFieldFocus == createSummaryFieldIndex {
			focusLine = len(lines)
		}
		lines = append(lines, m.createFieldLabel("Summary", createSummaryFieldIndex))
		lines = append(lines, m.renderCreateSummaryValue(bodyWidth))
		lines = append(lines, "")
		if m.createFieldFocus == createDescriptionFieldIndex {
			focusLine = len(lines)
		}
		lines = append(lines, m.createFieldLabel("Description", createDescriptionFieldIndex))
		lines = append(lines, m.renderCreateDescriptionValue(bodyWidth))
		if questionsIndex := m.createQuestionsFieldIndex(); questionsIndex >= 0 {
			lines = append(lines, "")
			if m.createFieldFocus == questionsIndex {
				focusLine = len(lines)
			}
			lines = append(lines, m.createFieldLabel("Open Questions", questionsIndex))
			lines = append(lines, m.renderCreateQuestions(bodyWidth))
		}
		if aiFieldIndex := m.createAIPromptFieldIndex(); aiFieldIndex >= 0 {
			lines = append(lines, "")
			if m.createFieldFocus == aiFieldIndex {
				focusLine = len(lines)
			}
			lines = append(lines, m.createFieldLabel("Generate Draft", aiFieldIndex))
			lines = append(lines, m.theme.Muted.Render("Press enter to improve the current draft with AI."))
		}
		for index, field := range supportedCreateFields(m.createFields) {
			focusIndex := m.createDynamicFieldFocusIndex(index)
			lines = append(lines, "")
			if m.createFieldFocus == focusIndex {
				focusLine = len(lines)
				lines = append(lines, m.createFieldLabel(displayValue(field.Name, field.ID), focusIndex))
				lines = append(lines, m.renderCreateDynamicField(field, bodyWidth))
				continue
			}
			lines = append(lines, m.renderCreateDynamicField(field, bodyWidth))
		}
		if unsupported := unsupportedRequiredCreateFields(m.createFields); len(unsupported) > 0 {
			lines = append(lines, "", m.renderDetailNotice("Jira may require more fields: "+strings.Join(unsupported, ", "), bodyWidth))
		}
		if m.createSubmitting {
			lines = append(lines, "", m.detailStatusBlock("Creating ticket...", bodyWidth, false))
		}
		if m.detailNotice != "" {
			lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
		}
		if m.claudeCreateTicketDraftEnabled() {
			footer = "tab field  enter generate  ctrl+s create  esc cancel"
		} else {
			footer = "tab field  ctrl+s create  esc cancel"
		}
	}
	body := m.windowCreateBody(lines, bodyWidth, focusLine)
	return m.renderDetailDialogWithLimit(width, "Create Ticket", subtitle, body, footer, dialogWidth)
}

func (m Model) windowCreateBody(lines []string, width int, focusLine int) string {
	rows := m.createBodyRows()
	if len(lines) <= rows {
		return strings.Join(lines, "\n")
	}
	offset := 0
	if focusLine >= 0 {
		offset = clamp(focusLine-rows/2, 0, max(0, len(lines)-rows))
	}
	end := min(len(lines), offset+rows)
	visible := append([]string(nil), lines[offset:end]...)
	indicator := fmt.Sprintf("Create Lines %d-%d of %d", offset+1, end, len(lines))
	visible = append(visible, m.theme.Muted.Render(truncate(indicator, width)))
	return strings.Join(visible, "\n")
}

func (m Model) createBodyRows() int {
	return max(6, m.boundedPanelBodyRows(11))
}

func (m Model) createDescriptionEditorRows() int {
	return clamp((m.createBodyRows()*2)/3, 10, 18)
}

func (m Model) createSummaryEditorRows() int {
	return 3
}

func (m Model) createDialogMaxWidth(width int) int {
	if width <= 0 {
		return 72
	}
	return clamp((width*86)/100, 72, min(width, 132))
}

func (m Model) renderCreateModeTabs(width int) string {
	manual := "Manual"
	generated := "AI Generated"
	if m.createAIGeneratedMode {
		manual = m.theme.Muted.Render("  " + manual)
		generated = m.theme.Selected.Render("> " + generated)
	} else {
		manual = m.theme.Selected.Render("> " + manual)
		generated = m.theme.Muted.Render("  " + generated)
	}
	line := manual + "  " + generated
	if width > 0 && lipgloss.Width(line) > width {
		if m.createAIGeneratedMode {
			return m.theme.Selected.Render("> AI")
		}
		return m.theme.Selected.Render("> Manual")
	}
	return line
}

func (m Model) renderCreateIssueTypePickerLines() []string {
	rows := make([][]string, 0, len(m.createIssueTypes))
	cursor := clamp(m.selectedCreateIssueType, 0, len(m.createIssueTypes)-1)
	for index, issueType := range m.createIssueTypes {
		marker := " "
		labelStyle := m.theme.Text
		if index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
		}
		rows = append(rows, []string{
			labelStyle.Render(marker),
			labelStyle.Render(displayValue(issueType.Name, issueType.ID)),
		})
	}
	return []string{m.detailTable(0, []string{"", "ISSUE TYPE"}, rows, nil)}
}

func (m Model) renderCreateAIPrompt(layout browserLayout) string {
	width := layout.contentWidth
	dialogWidth := m.createDialogMaxWidth(width)
	bodyWidth := max(24, dialogWidth-4)
	lines := m.renderCreateAIPromptBody(bodyWidth)
	subtitle := m.createProjectKey
	if strings.TrimSpace(m.createIssueType.Name) != "" {
		subtitle = subtitle + "  " + m.createIssueType.Name
	}
	footer := "ctrl+s generate  esc cancel"
	if m.createAIPromptLoading {
		footer = "esc cancel"
	} else if m.createAIPromptErr == nil {
		footer = "ctrl+s generate  esc cancel"
	}
	return m.renderDetailDialogWithLimit(width, "Generate Ticket Draft", displayValue(subtitle, "No project"), strings.Join(lines, "\n"), footer, dialogWidth)
}

func (m Model) renderCreateAIPromptBody(bodyWidth int) []string {
	lines := []string{
		m.theme.FieldLabel.Render("Prompt"),
		"",
	}
	if m.createAIPromptLoading {
		lines = append(lines, m.renderClaudeProgressStatus(m.createAIPromptProgress)...)
		lines = append(lines, "", m.theme.Muted.Render("Elapsed: "+formatClaudeDuration(m.claudeNow().Sub(m.createAIPromptStartedAt))))
		lines = append(lines, m.theme.Muted.Render("Claude is drafting. Press esc to cancel."))
	} else if m.createAIPromptErr != nil {
		lines = append(lines, m.renderDetailNotice("Draft generation failed: "+m.createAIPromptErr.Error(), bodyWidth))
		lines = append(lines, "")
		lines = append(lines, m.configuredCreateAIPromptEditor(bodyWidth, 8).View())
	} else {
		lines = append(lines, m.configuredCreateAIPromptEditor(bodyWidth, 8).View())
	}
	if m.detailNotice != "" && !m.createAIPromptLoading {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return lines
}

func (m Model) createFieldLabel(label string, index int) string {
	style := m.theme.Muted
	if m.createFieldFocus == index {
		style = m.theme.PaneTitle
	}
	return style.Render(label)
}

func (m Model) renderCreateSummaryValue(width int) string {
	if m.createFieldFocus == createSummaryFieldIndex {
		return m.configuredCreateSummaryEditor(width, m.createSummaryEditorRows()).View()
	}
	value := strings.TrimSpace(m.createSummaryDraft)
	if value == "" {
		value = "Edit summary..."
	}
	return m.theme.Muted.Render(truncate(value, width))
}

func (m Model) renderCreateDescriptionValue(width int) string {
	if m.createFieldFocus == createDescriptionFieldIndex {
		return m.configuredCreateDescriptionEditor(width, m.createDescriptionEditorRows()).View()
	}
	value := strings.TrimSpace(m.createDescriptionDraft)
	if value == "" {
		value = "Write a Jira comment..."
	}
	return m.theme.Muted.Render(truncate(value, width))
}

func (m Model) renderCreateQuestions(width int) string {
	if len(m.createAIQuestions) == 0 {
		return ""
	}
	selected := clamp(m.selectedCreateAIQuestion, 0, len(m.createAIQuestions)-1)
	var lines []string
	for index, question := range m.createAIQuestions {
		prefix := " "
		style := m.theme.Text
		if m.createFieldFocus == m.createQuestionsFieldIndex() && index == selected {
			prefix = ">"
			style = m.theme.Selected
		}
		status := ""
		if strings.TrimSpace(question.Answer) != "" {
			status = "answered"
		}
		parts := []string{prefix}
		if status != "" {
			parts = append(parts, status)
		}
		parts = append(parts, question.Question)
		line := strings.TrimSpace(strings.Join(parts, " "))
		lines = append(lines, style.Render(truncate(line, width)))
		if m.createAIQuestionAnswering && index == selected {
			lines = append(lines, m.configuredCreateQuestionAnswerEditor(width, 4).View())
		}
	}
	if m.createFieldFocus == m.createQuestionsFieldIndex() && !m.createAIQuestionAnswering {
		lines = append(lines, m.theme.Muted.Render("enter answer  j/k select  ctrl+r refine with answers"))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderCreateDynamicField(field jira.CreateField, width int) string {
	if focused, ok := m.focusedCreateDynamicField(); !ok || createFieldValueKey(focused) != createFieldValueKey(field) {
		return m.renderCreateDynamicFieldSummary(field, width)
	}
	if createFieldUsesPicker(field) {
		if len(field.AllowedValues) == 0 {
			return m.detailEmptyState("No Jira options available.", width)
		}
		key := createFieldValueKey(field)
		filter := m.createDynamicFilters[key]
		matches := filteredCreateFieldOptionIndexes(field.AllowedValues, filter)
		if len(matches) == 0 {
			return strings.Join([]string{
				m.theme.Muted.Render("Filter: " + filter),
				m.detailEmptyState("No Jira options matched.", width),
			}, "\n")
		}
		selected := m.createDynamicSelections[key]
		matchPosition := createOptionMatchPosition(matches, selected)
		if matchPosition < 0 {
			matchPosition = 0
		}
		start, end := boundedSelectionWindow(len(matches), matchPosition, createPickerMaxRows)
		var lines []string
		if strings.TrimSpace(filter) != "" {
			lines = append(lines, m.theme.Muted.Render("Filter: "+filter))
		}
		for index := start; index < end; index++ {
			optionIndex := matches[index]
			option := field.AllowedValues[optionIndex]
			marker := " "
			style := m.theme.Text
			if optionIndex == selected {
				marker = ">"
				style = m.theme.Selected
			}
			lines = append(lines, style.Render(marker+" "+displayValue(option.Name, option.ID)))
		}
		if len(matches) > end-start {
			lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("Options %d-%d of %d", start+1, end, len(matches))))
		}
		return strings.Join(lines, "\n")
	}
	value := m.createDynamicValues[createFieldValueKey(field)]
	if strings.TrimSpace(value) == "" {
		value = " "
	}
	return m.theme.Text.Render(truncate(value, width))
}

func (m Model) renderCreateDynamicFieldSummary(field jira.CreateField, width int) string {
	label := displayValue(field.Name, field.ID)
	value := ""
	if createFieldUsesPicker(field) {
		selected := m.createDynamicSelections[createFieldValueKey(field)]
		if selected >= 0 && selected < len(field.AllowedValues) {
			value = displayValue(field.AllowedValues[selected].Name, field.AllowedValues[selected].ID)
		}
	} else {
		value = strings.TrimSpace(m.createDynamicValues[createFieldValueKey(field)])
	}
	if value == "" {
		value = "-"
	}
	line := label + ": " + value
	return m.theme.Muted.Render(truncate(line, width))
}

func (m Model) renderSummaryDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	lines := []string{
		m.theme.Muted.Render("Summary"),
		m.configuredSummaryEditor(bodyWidth, 3).View(),
	}
	if m.summarySubmitting && m.summarySubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Updating summary...", bodyWidth, false))
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Edit Summary", selected.Key, strings.Join(lines, "\n"), "enter save  esc cancel")
}

func (m Model) renderPriorityDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	current := m.theme.Muted.Render("Current: ") + priorityStyle(m.theme, selected.Priority).Render(displayValue(selected.Priority, "Unknown"))
	lines := []string{current}
	if m.prioritySubmitting && m.prioritySubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Updating priority...", bodyWidth, false))
	} else {
		options := m.priorityOptions(selected.Key)
		if len(options) == 0 {
			lines = append(lines, "", m.detailEmptyState("No Jira priority values are available.", bodyWidth))
		} else {
			cursor := clamp(m.selectedPriority, 0, len(options)-1)
			rows := make([][]string, 0, len(options))
			for index, option := range options {
				marker := " "
				labelStyle := m.theme.Text
				if index == cursor {
					marker = ">"
					labelStyle = m.theme.Selected
				}
				rows = append(rows, []string{
					labelStyle.Render(marker),
					labelStyle.Render(displayValue(option.Name, option.ID)),
				})
			}
			lines = append(lines, "", m.detailTable(0, []string{"", "PRIORITY"}, rows, nil))
		}
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Change Priority", selected.Key, strings.Join(lines, "\n"), "j/k select  enter apply  esc cancel")
}

func (m Model) renderAssigneeDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	current := m.theme.Muted.Render("Current: ") + m.theme.Text.Render(displayValue(selected.Assignee, "Unassigned"))
	lines := []string{
		current,
		m.theme.Muted.Render("Filter: ") + m.theme.Text.Render(displayValue(m.assigneeQuery, "type to search")),
	}
	if m.assigneeSubmitting && m.assigneeSubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Updating assignee...", bodyWidth, false))
	} else if m.assigneeSearchLoading {
		lines = append(lines, "", m.detailStatusBlock("Searching Jira users...", bodyWidth, false))
	} else if m.assigneeSearchErr != nil {
		lines = append(lines, "", m.renderDetailNotice("Assignee search failed: "+m.assigneeSearchErr.Error(), bodyWidth))
	} else if len(m.assigneeUsers) == 0 {
		lines = append(lines, "", m.detailEmptyState("Type a name to search Jira users.", bodyWidth))
	} else {
		cursor := clamp(m.selectedAssignee, 0, len(m.assigneeUsers)-1)
		rows := make([][]string, 0, len(m.assigneeUsers))
		for index, user := range m.assigneeUsers {
			marker := " "
			labelStyle := m.theme.Text
			if index == cursor {
				marker = ">"
				labelStyle = m.theme.Selected
			}
			rows = append(rows, []string{
				labelStyle.Render(marker),
				labelStyle.Render(displayValue(user.DisplayName, user.Email)),
			})
		}
		lines = append(lines, "", m.detailTable(0, []string{"", "USER"}, rows, nil))
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Change Assignee", selected.Key, strings.Join(lines, "\n"), "type filter  up/down select  enter apply  esc cancel")
}

func (m Model) renderInlineAIDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 72)
	if m.inlineAIInstructionOpen {
		lines := []string{
			m.theme.Muted.Render("Question or instruction"),
			m.configuredInlineAIInstructionEditor(bodyWidth, 4).View(),
			"",
			m.theme.Muted.Render("Claude will receive the current ticket context and Description."),
		}
		return m.renderDetailDialog(width, "AI for Description", selected.Key, strings.Join(lines, "\n"), "ctrl+s send  esc cancel")
	}
	actions := inlineDescriptionAIActions()
	cursor := clamp(m.selectedInlineAIAction, 0, len(actions)-1)
	rows := make([][]string, 0, len(actions))
	for index, action := range actions {
		marker := " "
		labelStyle := m.theme.Text
		descStyle := m.theme.Muted
		if index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
		}
		rows = append(rows, []string{
			labelStyle.Render(marker),
			labelStyle.Render(action.Label),
			descStyle.Render(action.Description),
		})
	}
	body := m.detailTable(0, []string{"", "ACTION", "DETAIL"}, rows, nil)
	return m.renderDetailDialog(width, "AI for Description", selected.Key, body, "j/k select  enter run  esc cancel")
}

func (m Model) renderClaudePlanDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	dialogWidth := claudePlanDialogWidth(width)
	bodyWidth := max(24, dialogWidth-4)
	var lines []string
	footer := "esc close"
	switch {
	case m.claudePlanLoading:
		footer = "esc cancel"
		lines = append(lines, m.detailStatusBlock("Asking Claude for a ticket plan...", bodyWidth, false))
		lines = append(lines, "")
		lines = append(lines, m.renderClaudePlanLoading(bodyWidth, m.claudeNow()))
	case m.claudePlanErr != nil:
		lines = append(lines, m.renderClaudePlanError(bodyWidth, m.claudeNow()))
	case strings.TrimSpace(m.claudePlanText) != "":
		lines = append(lines, m.renderClaudePlanResult(bodyWidth))
		if m.claudePlanResultScrollable(bodyWidth) {
			footer = "j/k scroll  pgup/pgdn page  g/G jump  esc close"
		}
	default:
		lines = append(lines, m.detailEmptyState("No Claude plan yet.", bodyWidth))
	}
	return m.renderDetailDialogWithLimit(width, "Claude Ticket Plan", selected.Key, strings.Join(lines, "\n"), footer, dialogWidth)
}

func (m Model) renderClaudeAssistDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	dialogWidth := claudePlanDialogWidth(width)
	bodyWidth := max(24, dialogWidth-4)
	footer := "esc close"
	var lines []string
	switch {
	case m.claudeAssistLoading:
		footer = "esc cancel"
		lines = append(lines, m.detailStatusBlock("Asking Claude to evaluate this ticket...", bodyWidth, false))
		lines = append(lines, "")
		lines = append(lines, m.renderClaudeAssistLoading(bodyWidth, m.claudeNow()))
	case m.claudeAssistApplying:
		footer = "esc close"
		lines = append(lines, m.detailStatusBlock("Applying Ticket Assist draft to Jira...", bodyWidth, false))
		lines = append(lines, "")
		lines = append(lines, m.theme.Muted.Render("Summary: ")+m.theme.Text.Render(applyStatusLabel(m.claudeAssistSummaryApplied)))
		lines = append(lines, m.theme.Muted.Render("Description: ")+m.theme.Text.Render(applyStatusLabel(m.claudeAssistDescriptionApplied)))
	case m.claudeAssistPostingComment:
		footer = "esc close"
		lines = append(lines, m.detailStatusBlock("Posting Ticket Assist draft as a Jira comment...", bodyWidth, false))
	case m.claudeAssistConfirmComment:
		footer = "ctrl+s post  esc cancel"
		lines = append(lines, m.renderClaudeAssistCommentConfirmation(bodyWidth))
	case m.claudeAssistConfirmApply:
		footer = "ctrl+s apply  esc cancel"
		lines = append(lines, m.renderClaudeAssistApplyConfirmation(bodyWidth))
	case m.claudeAssistRefining:
		footer = "ctrl+s send  esc cancel"
		lines = append(lines, m.renderClaudeAssistRefinementEditor(bodyWidth))
	case m.claudeAssistErr != nil:
		lines = append(lines, m.renderClaudeAssistError(bodyWidth, m.claudeNow()))
	default:
		if m.claudeConfig.AllowJiraWrites {
			footer = "ctrl+s apply  c comment  r refine  ctrl+y copy  pgup/pgdn page  esc close"
		} else {
			footer = "c comment  r refine  ctrl+y copy  pgup/pgdn page  esc close"
		}
		lines = append(lines, m.renderClaudeAssistEditor(bodyWidth))
	}
	return m.renderDetailDialogWithLimit(width, "Claude Ticket Assist", selected.Key, strings.Join(lines, "\n"), footer, dialogWidth)
}

func applyStatusLabel(done bool) string {
	if done {
		return "done"
	}
	return "saving"
}

func claudePlanDialogWidth(width int) int {
	if width <= 0 {
		width = 72
	}
	return clamp((width*88)/100, 72, max(72, width))
}

func (m Model) renderClaudePlanResult(width int) string {
	lines := m.claudePlanResultLines(width)
	if len(lines) == 0 {
		return ""
	}
	rows := m.claudePlanResultRows()
	if len(lines) <= rows {
		return strings.Join(lines, "\n")
	}
	offset := clamp(m.claudePlanOffset, 0, max(0, len(lines)-rows))
	end := min(len(lines), offset+rows)
	visible := append([]string(nil), lines[offset:end]...)
	visible = append(visible, m.theme.Muted.Render(fmt.Sprintf("Claude Lines %d-%d of %d", offset+1, end, len(lines))))
	return strings.Join(visible, "\n")
}

func (m Model) claudePlanResultRows() int {
	return max(1, m.fullDetailRows()-9)
}

func (m Model) claudePlanResultScrollable(width int) bool {
	return len(m.claudePlanResultLines(width)) > m.claudePlanResultRows()
}

func (m *Model) scrollClaudePlanResult(delta int) {
	lines := m.claudePlanResultLines(m.currentClaudePlanBodyWidth())
	rows := m.claudePlanResultRows()
	m.claudePlanOffset = clamp(m.claudePlanOffset+delta, 0, max(0, len(lines)-rows))
}

func (m Model) claudePlanResultLines(width int) []string {
	rendered := m.renderRichDescriptionBody(wrapRichText(markdownTablesToRichMarkers(m.claudePlanText), width), width)
	return strings.Split(strings.TrimRight(rendered, "\n"), "\n")
}

func (m Model) renderClaudeAssistLoading(width int, now time.Time) string {
	elapsed := formatClaudeDuration(now.Sub(m.claudeAssistStartedAt))
	if m.claudeConfig.Timeout > 0 {
		elapsed += " of " + m.claudeConfig.Timeout.String()
	}
	lines := []string{
		m.theme.Muted.Render("Activity: ") + m.theme.Text.Render(claudeActivityFrame(now.Sub(m.claudeAssistStartedAt))+" Claude subprocess running"),
		m.theme.Muted.Render("Elapsed: ") + m.theme.Text.Render(elapsed),
	}
	lines = append(lines, m.renderClaudeProgressStatus(m.claudeAssistProgress)...)
	return strings.Join(lines, "\n")
}

func (m Model) renderClaudeAssistProgressLines(width int) []string {
	return m.renderClaudeEventProgressLines(m.claudeAssistProgress, width)
}

func (m Model) renderClaudeAssistEditor(width int) string {
	var lines []string
	if review := strings.TrimSpace(m.claudeAssistReviewText()); review != "" && m.claudeAssistReviewRows() > 0 {
		lines = append(lines, m.theme.FieldLabel.Render("Claude Review"))
		lines = append(lines, m.renderClaudeAssistReview(review, width)...)
		if m.height > 32 {
			lines = append(lines, "")
		}
	}
	lines = append(lines, m.theme.FieldLabel.Render("Local Draft")+" "+m.theme.Muted.Render("Not Applied"))
	lines = append(lines, m.renderClaudeAssistDraftEditor(width))
	if m.height == 0 || m.height > 32 {
		lines = append(lines, "")
		lines = append(lines, m.renderClaudeAssistActionHint(width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderClaudeAssistActionHint(width int) string {
	var action string
	if m.claudeConfig.AllowJiraWrites {
		action = "ctrl+s apply  |  c comment  |  r refine  |  ctrl+y copy"
	} else {
		action = "Jira writes disabled  |  c comment  |  r refine  |  ctrl+y copy"
	}
	return m.theme.FieldLabel.Render("Available Actions") + "\n" + m.theme.Muted.Render(truncate(action, width))
}

func (m Model) claudeAssistEditorRows() int {
	reviewRows := m.claudeAssistReviewRows()
	available := max(1, m.fullDetailRows()-reviewRows-13)
	if m.height > 0 && m.height <= 32 {
		return max(2, min(4, available/2))
	}
	return max(6, min(18, available/2))
}

func (m Model) claudeAssistReviewRows() int {
	if m.height > 0 && m.height <= 32 {
		return 0
	}
	return max(1, min(3, m.fullDetailRows()/8))
}

func (m Model) renderClaudeAssistReview(review string, width int) []string {
	rendered := m.renderRichDescriptionBody(wrapRichText(markdownTablesToRichMarkers(review), width), width)
	reviewLines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	if len(reviewLines) == 1 && strings.TrimSpace(reviewLines[0]) == "" {
		return nil
	}
	if len(reviewLines) > 0 && strings.EqualFold(strings.Trim(strings.TrimSpace(reviewLines[0]), "#: "), "Review") {
		reviewLines = reviewLines[1:]
	}
	rows := min(len(reviewLines), m.claudeAssistReviewRows())
	lines := append([]string(nil), reviewLines[:rows]...)
	if len(reviewLines) > rows {
		end := max(1, rows)
		lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("Review Lines 1-%d of %d", end, len(reviewLines))))
	}
	return lines
}

func (m Model) renderClaudeAssistDraftEditor(width int) string {
	rows := m.claudeAssistEditorRows()
	editorWidth := max(20, width-3)
	editor := m.configuredClaudeAssistEditor(editorWidth, rows)
	lineCount := editor.LineCount()
	body := editor.View()
	if lineCount > rows {
		start := editor.ScrollYOffset() + 1
		end := min(lineCount, editor.ScrollYOffset()+editor.Height())
		indicator := fmt.Sprintf("Draft Lines %d-%d of %d  PgUp/PgDn page", start, end, lineCount)
		body += "\n" + m.theme.Muted.Render(truncate(indicator, editorWidth))
	}
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(m.theme.Muted.GetForeground()).
		Padding(0, 1).
		Width(max(24, width)).
		Render(body)
}

func (m Model) renderClaudeAssistApplyConfirmation(width int) string {
	var lines []string
	if m.claudeAssistTarget == claudeAssistTargetDescription {
		lines = append(lines, m.theme.FieldLabel.Render("Apply Description Draft"))
		lines = append(lines, m.theme.Muted.Render("Issue: ")+m.theme.Text.Render(displayValue(m.claudeAssistKey, "selected ticket")))
		lines = append(lines, "")
		lines = append(lines, m.theme.Muted.Render("Description"))
		descriptionLines := strings.Split(strings.TrimSpace(m.claudeAssistApplyDescription), "\n")
		for i, line := range descriptionLines {
			if i >= 4 {
				lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("Description Lines 1-4 of %d", len(descriptionLines))))
				break
			}
			lines = append(lines, m.theme.Text.Render(truncate(line, width)))
		}
		return strings.Join(lines, "\n")
	}
	lines = append(lines, m.theme.FieldLabel.Render("Apply Ticket Assist Draft"))
	lines = append(lines, m.theme.Muted.Render("Issue: ")+m.theme.Text.Render(displayValue(m.claudeAssistKey, "selected ticket")))
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("Summary"))
	lines = append(lines, m.theme.Text.Render(truncate(displayValue(m.claudeAssistApplySummary, "unchanged"), width)))
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("Description"))
	descriptionLines := strings.Split(strings.TrimSpace(m.claudeAssistApplyDescription), "\n")
	for i, line := range descriptionLines {
		if i >= 4 {
			lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("Description Lines 1-4 of %d", len(descriptionLines))))
			break
		}
		lines = append(lines, m.theme.Text.Render(truncate(line, width)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderClaudeAssistCommentConfirmation(width int) string {
	var lines []string
	lines = append(lines, m.theme.FieldLabel.Render("Post Draft As Comment"))
	lines = append(lines, m.theme.Muted.Render("Issue: ")+m.theme.Text.Render(displayValue(m.claudeAssistKey, "selected ticket")))
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("This will add the local Ticket Assist draft as a Jira comment without editing Summary or Description."))
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("Comment Preview"))
	draftLines := strings.Split(strings.TrimSpace(m.claudeAssistDraftValue()), "\n")
	for i, line := range draftLines {
		if i >= 4 {
			lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("Comment Lines 1-4 of %d", len(draftLines))))
			break
		}
		lines = append(lines, m.theme.Text.Render(truncate(line, width)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderClaudeAssistRefinementEditor(width int) string {
	var lines []string
	lines = append(lines, m.theme.FieldLabel.Render("Refine Draft"))
	lines = append(lines, m.theme.Muted.Render("Instruction"))
	editor := m.configuredClaudeAssistRefineEditor(max(24, width-3), 4)
	body := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(m.theme.Muted.GetForeground()).
		Padding(0, 1).
		Width(max(24, width)).
		Render(editor.View())
	lines = append(lines, body)
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("Claude will receive the current edited draft and this instruction."))
	return strings.Join(lines, "\n")
}

func (m Model) renderClaudeAssistError(width int, now time.Time) string {
	if errors.Is(m.claudeAssistErr, context.DeadlineExceeded) {
		lines := []string{
			m.renderDetailNotice("Claude ticket assist timed out after "+displayValue(m.claudeConfig.Timeout.String(), "the configured timeout"), width),
			"",
			m.theme.Muted.Render("Started: ") + m.theme.Text.Render(formatClockTime(m.claudeAssistStartedAt)),
		}
		if m.claudeConfig.Timeout > 0 {
			lines = append(lines, m.theme.Muted.Render("Deadline: ")+m.theme.Text.Render(formatClockTime(m.claudeAssistStartedAt.Add(m.claudeConfig.Timeout))))
		}
		lines = append(lines, m.theme.Muted.Render("Elapsed: ")+m.theme.Text.Render(formatClaudeDuration(now.Sub(m.claudeAssistStartedAt))))
		lines = append(lines, m.theme.Muted.Render("Command: ")+m.theme.Text.Render(m.claudeCommandLabel()))
		return strings.Join(lines, "\n")
	}
	return m.renderDetailNotice("Claude ticket assist failed: "+m.claudeAssistErr.Error(), width)
}

func (m Model) currentClaudePlanBodyWidth() int {
	layout := m.browserLayout(m.width)
	dialogWidth := max(32, layout.contentWidth-12)
	return min(max(24, dialogWidth-12), 64)
}

func (m Model) renderStatusTransitionDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	current := m.theme.Muted.Render("Current: ") + statusStyle(m.theme, selected.Status).Render(displayValue(selected.Status, "Unknown"))
	lines := []string{current}
	if m.transitionSubmitting && m.transitionSubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Applying transition...", bodyWidth, false))
	} else {
		transitions := m.transitions[selected.Key]
		if len(transitions) == 0 {
			lines = append(lines, "", m.detailEmptyState("No available Jira transitions.", bodyWidth))
		} else {
			cursor := clamp(m.selectedTransition, 0, len(transitions)-1)
			rows := make([][]string, 0, len(transitions))
			for index, transition := range transitions {
				marker := " "
				labelStyle := m.theme.Text
				if index == cursor {
					marker = ">"
					labelStyle = m.theme.Selected
				}
				toStatus := displayValue(transition.ToStatus, "Unknown")
				rows = append(rows, []string{
					labelStyle.Render(marker),
					labelStyle.Render(transition.Name),
					statusStyle(m.theme, toStatus).Render(toStatus),
				})
			}
			lines = append(lines, "", m.detailTable(0, []string{"", "TRANSITION", "TO"}, rows, nil))
		}
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Change Status", selected.Key, strings.Join(lines, "\n"), "j/k select  enter apply  esc cancel")
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
	section := sections[0]
	if focused, ok := m.focusedDetailSection(); ok {
		section = focused
	}
	var b strings.Builder
	b.WriteString(m.renderDetailSection(section, ctx, bodyWidth))
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
	case "status":
		return m.renderStatusSection(ctx.display, width)
	case "claude":
		return m.renderClaudeSection(ctx, width)
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
	label := "Summary: "
	value := displayValue(display.Summary, "No summary")
	if strings.TrimSpace(value) == "" {
		value = " "
	}
	style := m.theme.Text
	if m.summaryFocus || m.focusedDetailTargetID() == "summary" {
		style = m.theme.Selected
	}
	if m.summaryMetadataLoading && m.summaryMetadataRequestKey == display.Key {
		value = "Loading edit metadata..."
	}
	if m.summarySubmitting && m.summarySubmitKey == display.Key {
		value = "Updating summary..."
	}
	available := max(12, width-lipgloss.Width(label))
	return m.theme.Muted.Render(label) + style.Render(truncate(value, available))
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
		m.detailMetaPartWithStyle("Assignee", shortName(displayValue(display.Assignee, "Unassigned")), m.focusedDetailTargetID() == "assignee"),
		m.detailMetaPartWithStyle("Priority", displayValue(display.Priority, "Unknown"), m.focusedDetailTargetID() == "priority"),
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

func (m Model) detailMetaPartWithStyle(label string, value string, selected bool) string {
	style := m.theme.Text
	if selected {
		style = m.theme.Selected
	}
	return m.theme.Muted.Render(label+": ") + style.Render(value)
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
		{ID: "status", Label: "Status", Short: "Stat"},
	}
	if m.claudeAvailable() {
		sections = append(sections, detailSection{ID: "claude", Label: "Claude", Short: "AI"})
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

func (m Model) detailTargets() []detailTarget {
	sections := m.detailSections()
	targets := []detailTarget{
		{ID: "summary", Label: "Summary", Kind: detailTargetField},
		{ID: "assignee", Label: "Assignee", Kind: detailTargetField},
		{ID: "priority", Label: "Priority", Kind: detailTargetField},
	}
	for _, section := range sections {
		targets = append(targets, detailTarget{
			ID:      section.ID,
			Label:   section.Label,
			Kind:    detailTargetSection,
			Section: section,
		})
	}
	return targets
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
	focusedSectionID := ""
	if section, ok := m.focusedDetailSection(); ok {
		focusedSectionID = section.ID
	}
	for _, section := range sections {
		label := section.Label
		if compact {
			label = section.Short
		}
		if section.Badge != "" {
			label += " " + section.Badge
		}
		if section.ID == focusedSectionID {
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
	focusedSectionID := ""
	if section, ok := m.focusedDetailSection(); ok {
		focusedSectionID = section.ID
	}
	for _, section := range sections {
		label := section.Short
		if section.Badge != "" {
			label += " " + section.Badge
		}
		part := m.theme.TabInactive.Render(label)
		if section.ID == focusedSectionID {
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
	targets := m.detailTargets()
	if len(targets) == 0 {
		m.detailFocus = 0
		return
	}
	m.saveDetailSectionOffset()
	m.detailFocus = (m.detailFocus + delta + len(targets)) % len(targets)
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.priorityFocus = false
	m.assigneeFocus = false
	m.summaryFocus = false
	m.restoreDetailSectionOffset()
}

func (m *Model) moveDetailSectionFocus(delta int) {
	targets := m.detailTargets()
	if len(targets) == 0 {
		m.detailFocus = 0
		return
	}
	m.saveDetailSectionOffset()
	start := clamp(m.detailFocus, 0, len(targets)-1)
	for step := 1; step <= len(targets); step++ {
		index := (start + delta*step + len(targets)*step) % len(targets)
		if targets[index].Kind == detailTargetSection {
			m.detailFocus = index
			break
		}
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.priorityFocus = false
	m.assigneeFocus = false
	m.summaryFocus = false
	m.restoreDetailSectionOffset()
}

func (m Model) activateFocusedDetailTarget() (Model, tea.Cmd) {
	target, ok := m.focusedDetailTarget()
	if !ok {
		return m, nil
	}
	if target.Kind == detailTargetField {
		switch target.ID {
		case "summary":
			return m.startSummaryEditor()
		case "assignee":
			return m.startAssigneePicker()
		case "priority":
			return m.startPriorityEditor()
		}
		return m, nil
	}
	section := target.Section
	switch section.ID {
	case "actions":
		m.focusActions()
	case "status":
		return m.startStatusTransitionPicker()
	case "claude":
		return m.runSelectedClaudeAction()
	case "hierarchy":
		m.focusHierarchy()
	case "links":
		m.focusDetailLinks()
	default:
		m.linkFocus = false
		m.hierarchyFocus = false
		m.actionFocus = false
		m.transitionFocus = false
		m.priorityFocus = false
		m.assigneeFocus = false
		m.jumpDetailSection(section.Label)
	}
	return m, nil
}

func (m *Model) activateDetailFocus() {
	updated, _ := m.activateFocusedDetailTarget()
	*m = updated
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
	return m.detailSectionHeader("description", "Description", "", width) + "\n\n" + m.renderRichDescriptionBody(wrapRichText(description, width), width)
}

func (m Model) renderDescriptionState(message string, width int, isError bool) string {
	return m.detailSectionHeader("description", "Description", "", width) + "\n\n" + m.detailStatusBlock(message, width, isError)
}

func (m Model) renderComments(key string, width int) string {
	lines := []string{m.detailSectionHeader("comments", "Comments", "", width)}
	if comments, ok := m.comments[key]; ok {
		if len(comments) == 0 {
			lines = append(lines, "")
			lines = append(lines, m.detailStatusBlock("No comments yet.", width, false))
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
		lines = append(lines, "")
		lines = append(lines, m.detailStatusBlock("Loading comments...", width, false))
		return strings.Join(lines, "\n")
	}
	if m.commentsErr != nil && m.commentsRequestKey == key {
		lines = append(lines, "")
		lines = append(lines, m.detailStatusBlock("Comments failed: "+m.commentsErr.Error(), width, true))
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "")
	lines = append(lines, m.detailStatusBlock("Comments not loaded.", width, false))
	return strings.Join(lines, "\n")
}

func (m Model) renderCommentBlock(index int, total int, author string, created string, body string, width int) string {
	contentWidth := max(20, width-4)
	header := m.theme.Key.Render(displayValue(author, "Unknown")) +
		m.theme.Muted.Render("  "+created+"  "+fmt.Sprintf("Comment %d/%d", index, max(1, total)))
	bodyWidth := max(12, contentWidth-2)
	renderedBody := m.renderRichDescriptionBody(wrapRichText(body, bodyWidth), bodyWidth)
	renderedBody = indentLines(renderedBody, "  ")
	return m.theme.CommentBlock.Width(contentWidth + 2).Render(header + "\n\n" + renderedBody)
}

func (m Model) detailEmptyState(message string, width int) string {
	return m.theme.Muted.Render("  " + truncate(message, max(12, width-2)))
}

func (m Model) detailStatusBlock(message string, width int, isError bool) string {
	header := m.detailSectionHeader("detail-status", "Status", "", width)
	body := m.theme.Text.Render(wrapText(message, max(12, width)))
	if isError {
		body = m.theme.Error.Render(wrapText(message, max(12, width)))
	}
	return header + "\n" + body
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

func (m Model) renderStatusSection(issue jira.Issue, width int) string {
	help := "enter load transitions"
	if m.transitionFocus {
		help = "dialog open"
	} else if m.transitionLoading && m.transitionRequestKey == issue.Key {
		help = "loading"
	} else if m.transitionSubmitting && m.transitionSubmitKey == issue.Key {
		help = "applying"
	}
	lines := []string{m.detailSectionHeader("status", "Status", help, width)}
	current := statusStyle(m.theme, issue.Status).Render(displayValue(issue.Status, "Unknown"))
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("Current: ")+current)
	if m.transitionLoading && m.transitionRequestKey == issue.Key {
		lines = append(lines, "")
		lines = append(lines, m.detailStatusBlock("Loading available transitions...", width, false))
		return strings.Join(lines, "\n")
	}
	if m.transitionErr != nil && m.transitionRequestKey == issue.Key {
		lines = append(lines, "")
		lines = append(lines, m.detailStatusBlock("Transitions failed: "+m.transitionErr.Error(), width, true))
	}
	if m.transitionFocus || (m.transitionSubmitting && m.transitionSubmitKey == issue.Key) {
		lines = append(lines, "")
		lines = append(lines, m.detailEmptyState("Status change dialog open.", width))
		return strings.Join(lines, "\n")
	}
	transitions := m.transitions[issue.Key]
	if len(transitions) == 0 {
		lines = append(lines, "")
		lines = append(lines, m.detailEmptyState("Press enter to load available Jira transitions.", width))
		return strings.Join(lines, "\n")
	}
	cursor := clamp(m.selectedTransition, 0, len(transitions)-1)
	rows := make([][]string, 0, len(transitions))
	for index, transition := range transitions {
		marker := " "
		labelStyle := m.theme.Text
		if m.transitionFocus && index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
		}
		toStatus := displayValue(transition.ToStatus, "Unknown")
		rows = append(rows, []string{
			labelStyle.Render(marker),
			labelStyle.Render(transition.Name),
			statusStyle(m.theme, toStatus).Render(toStatus),
		})
	}
	lines = append(lines, "")
	lines = append(lines, m.detailTable(0, []string{"", "TRANSITION", "TO"}, rows, nil))
	return strings.Join(lines, "\n")
}

func (m Model) claudeTicketPlanAvailable() bool {
	return m.claudeConfig.Enabled &&
		m.claudeConfig.TicketPlan &&
		m.claudeStatus.Enabled &&
		m.claudeStatus.Available
}

func (m Model) claudeTicketAssistAvailable() bool {
	return m.claudeConfig.Enabled &&
		m.claudeConfig.TicketAssist &&
		m.claudeStatus.Enabled &&
		m.claudeStatus.Available
}

func (m Model) claudeCreateTicketDraftAvailable() bool {
	return m.claudeCreateTicketDraftEnabled() && m.claudeStatus.Available
}

func (m Model) claudeCreateTicketDraftEnabled() bool {
	return m.claudeConfig.Enabled &&
		m.claudeConfig.DraftTicket &&
		m.claudeStatus.Enabled
}

func (m Model) claudeAvailable() bool {
	return m.claudeTicketPlanAvailable() || m.claudeTicketAssistAvailable()
}

func (m Model) inlineDescriptionAIAvailable() bool {
	if !m.claudeTicketAssistAvailable() {
		return false
	}
	section, ok := m.focusedDetailSection()
	return ok && section.ID == "description"
}

func (m Model) openInlineDescriptionAI() (Model, tea.Cmd) {
	if !m.inlineDescriptionAIAvailable() {
		m.detailNotice = "Claude ticket assistance is not enabled or available."
		return m, nil
	}
	m.inlineAIOpen = true
	m.selectedInlineAIAction = clamp(m.selectedInlineAIAction, 0, len(inlineDescriptionAIActions())-1)
	m.detailNotice = ""
	return m, nil
}

type claudeAction struct {
	ID          string
	Label       string
	Description string
	Enabled     bool
}

type inlineAIAction struct {
	ID          string
	Label       string
	Description string
}

type claudeAssistTarget int

const (
	claudeAssistTargetTicket claudeAssistTarget = iota
	claudeAssistTargetDescription
)

func inlineDescriptionAIActions() []inlineAIAction {
	return []inlineAIAction{
		{ID: "improve_clarity", Label: "Improve clarity", Description: "Rewrite the Description for clearer scope and verification."},
		{ID: "extract_acceptance", Label: "Extract acceptance criteria", Description: "Draft explicit acceptance criteria and open questions."},
		{ID: "ask_question", Label: "Ask Claude a question", Description: "Ask about this ticket and draft a local answer."},
		{ID: "draft_comment", Label: "Draft clarifying comment", Description: "Draft a Jira comment without editing fields."},
	}
}

func (m Model) claudeActions() []claudeAction {
	actions := []claudeAction{
		{ID: "ticket_plan", Label: "Ticket Plan", Description: "Create a read-only implementation and verification plan.", Enabled: m.claudeTicketPlanAvailable()},
		{ID: "ticket_assist", Label: "Ticket Assist", Description: "Evaluate and rewrite this ticket with editable acceptance criteria.", Enabled: m.claudeTicketAssistAvailable()},
	}
	filtered := make([]claudeAction, 0, len(actions))
	for _, action := range actions {
		if action.Enabled {
			filtered = append(filtered, action)
		}
	}
	return filtered
}

func (m Model) renderClaudeSection(ctx detailRenderContext, width int) string {
	help := "j/k select  enter run"
	if (m.claudePlanLoading && m.claudePlanKey == ctx.display.Key) || (m.claudeAssistLoading && m.claudeAssistKey == ctx.display.Key) {
		help = "running"
	}
	lines := []string{m.detailSectionHeader("claude", "Claude", help, width), ""}
	actions := m.claudeActions()
	if len(actions) == 0 {
		lines = append(lines, m.detailEmptyState("Claude ticket assistance is not enabled or available.", width))
		return strings.Join(lines, "\n")
	}
	if m.claudePlanLoading && m.claudePlanKey == ctx.display.Key {
		lines = append(lines, m.detailStatusBlock("Asking Claude for a read-only ticket plan...", width, false))
		return strings.Join(lines, "\n")
	}
	if m.claudeAssistLoading && m.claudeAssistKey == ctx.display.Key {
		lines = append(lines, m.detailStatusBlock("Asking Claude to evaluate this ticket...", width, false))
		return strings.Join(lines, "\n")
	}
	cursor := clamp(m.selectedClaudeAction, 0, len(actions)-1)
	rows := make([][]string, 0, len(actions))
	for index, action := range actions {
		marker := " "
		labelStyle := m.theme.Text
		descStyle := m.theme.Muted
		if index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
		}
		rows = append(rows, []string{
			labelStyle.Render(marker),
			labelStyle.Render(action.Label),
			descStyle.Render(action.Description),
		})
	}
	lines = append(lines, m.detailTable(0, []string{"", "ACTION", "DETAIL"}, rows, nil))
	if strings.TrimSpace(m.claudePlanText) != "" && m.claudePlanKey == ctx.display.Key {
		lines = append(lines, "", m.theme.Muted.Render("Latest ticket plan is ready. Select Ticket Plan to refresh it."))
	}
	if strings.TrimSpace(m.claudeAssistDraftValue()) != "" && m.claudeAssistKey == ctx.display.Key {
		lines = append(lines, "", m.theme.Muted.Render("Latest ticket assist draft is ready. Select Ticket Assist to refresh it."))
	}
	return strings.Join(lines, "\n")
}

func (m Model) startClaudeTicketPlan() (Model, tea.Cmd) {
	ctx, ok := m.detailRenderContext()
	if !ok {
		return m, nil
	}
	if !m.claudeTicketPlanAvailable() {
		m.detailNotice = "Claude ticket planning is not enabled or available."
		return m, nil
	}
	key := ctx.display.Key
	if key == "" {
		key = ctx.selected.Key
	}
	m.nextRequestID++
	reqID := m.nextRequestID
	m.activeClaudePlanReqID = reqID
	m.claudePlanKey = key
	m.claudePlanText = ""
	m.claudePlanErr = nil
	m.claudePlanOffset = 0
	m.claudePlanLoading = true
	m.claudePlanOpen = true
	m.claudePlanStartedAt = m.claudeNow()
	m.claudePlanProgress = nil
	m.claudePlanEvents = make(chan claude.Event, 16)
	runCtx, cancel := context.WithCancel(context.Background())
	m.claudePlanCancel = cancel
	m.detailNotice = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_plan", "submit", workerDiagnosticDetail(reqID, key, nil))
	return m, tea.Batch(
		m.submitClaudeTicketPlan(runCtx, reqID, key, m.buildClaudeTicketPlanPrompt(ctx), m.claudePlanEvents),
		m.waitForClaudePlanProgress(reqID, key),
		m.scheduleClaudePlanTick(reqID),
	)
}

func (m Model) startClaudeTicketAssist() (Model, tea.Cmd) {
	ctx, ok := m.detailRenderContext()
	if !ok {
		return m, nil
	}
	if !m.claudeTicketAssistAvailable() {
		m.detailNotice = "Claude ticket assistance is not enabled or available."
		return m, nil
	}
	key := ctx.display.Key
	if key == "" {
		key = ctx.selected.Key
	}
	m.nextRequestID++
	reqID := m.nextRequestID
	m.activeClaudeAssistReqID = reqID
	m.claudeAssistKey = key
	m.claudeAssistText = ""
	m.claudeAssistErr = nil
	m.claudeAssistLoading = true
	m.claudeAssistOpen = true
	m.claudeAssistStartedAt = m.claudeNow()
	m.claudeAssistProgress = nil
	m.claudeAssistDraft = ""
	m.claudeAssistEditor = newClaudeAssistEditor("")
	m.claudeAssistEditorReady = true
	m.claudeAssistTarget = claudeAssistTargetTicket
	m.claudeAssistEvents = make(chan claude.Event, 16)
	runCtx, cancel := context.WithCancel(context.Background())
	m.claudeAssistCancel = cancel
	m.detailNotice = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_assist", "submit", workerDiagnosticDetail(reqID, key, nil))
	return m, tea.Batch(
		m.submitClaudeTicketAssist(runCtx, reqID, key, m.buildClaudeTicketAssistPrompt(ctx), m.claudeAssistEvents),
		m.waitForClaudeAssistProgress(reqID, key),
		m.scheduleClaudeAssistTick(reqID),
	)
}

func (m Model) submitClaudeTicketPlan(ctx context.Context, reqID int, key string, prompt string, events chan<- claude.Event) tea.Cmd {
	return m.submitClaudeRequest(ctx, reqID, key, prompt, events, func(id int, key string, text string, err error) tea.Msg {
		return claudePlanResultMsg{id: id, key: key, text: text, err: err}
	})
}

func (m Model) submitClaudeTicketAssist(ctx context.Context, reqID int, key string, prompt string, events chan<- claude.Event) tea.Cmd {
	return m.submitClaudeRequest(ctx, reqID, key, prompt, events, func(id int, key string, text string, err error) tea.Msg {
		return claudeAssistResultMsg{id: id, key: key, text: text, err: err}
	})
}

func (m Model) submitClaudeRequest(ctx context.Context, reqID int, key string, prompt string, events chan<- claude.Event, resultMsg func(int, string, string, error) tea.Msg) tea.Cmd {
	runner := m.claudeRunner
	if runner == nil {
		runner = claude.LocalRunner{}
	}
	config := claude.Config{
		Enabled: m.claudeConfig.Enabled,
		Command: m.claudeConfig.Command,
		Timeout: m.claudeConfig.Timeout,
	}
	return func() tea.Msg {
		defer closeClaudeEvents(events)
		result, err := runner.Run(ctx, claude.Request{
			Config: config,
			Prompt: prompt,
			Progress: func(event claude.Event) {
				if strings.TrimSpace(event.Text) == "" {
					return
				}
				select {
				case events <- event:
				case <-ctx.Done():
				}
			},
		})
		if err != nil {
			return resultMsg(reqID, key, "", err)
		}
		return resultMsg(reqID, key, result.Text, nil)
	}
}

func closeClaudeEvents(events chan<- claude.Event) {
	if events != nil {
		close(events)
	}
}

func (m Model) waitForClaudePlanProgress(reqID int, key string) tea.Cmd {
	events := m.claudePlanEvents
	return waitForClaudeProgress(events, reqID, key, func(id int, key string, event claude.Event) tea.Msg {
		return claudePlanProgressMsg{id: id, key: key, event: event}
	})
}

func (m Model) waitForClaudeAssistProgress(reqID int, key string) tea.Cmd {
	events := m.claudeAssistEvents
	return waitForClaudeProgress(events, reqID, key, func(id int, key string, event claude.Event) tea.Msg {
		return claudeAssistProgressMsg{id: id, key: key, event: event}
	})
}

func waitForClaudeProgress(events <-chan claude.Event, reqID int, key string, progressMsg func(int, string, claude.Event) tea.Msg) tea.Cmd {
	if events == nil {
		return nil
	}
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return noDetailRequestMsg{}
		}
		return progressMsg(reqID, key, event)
	}
}

func (m Model) scheduleClaudePlanTick(reqID int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return claudePlanTickMsg{id: reqID}
	})
}

func (m Model) scheduleClaudeAssistTick(reqID int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return claudeAssistTickMsg{id: reqID}
	})
}

func (m Model) cancelClaudeTicketPlan() Model {
	if m.claudePlanCancel != nil {
		m.claudePlanCancel()
	}
	reqID := m.activeClaudePlanReqID
	key := m.claudePlanKey
	m.claudePlanCancel = nil
	m.claudePlanEvents = nil
	m.activeClaudePlanReqID = 0
	m.claudePlanLoading = false
	m.claudePlanErr = errors.New("Claude ticket plan cancelled")
	m.claudePlanText = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_plan", "cancel", workerDiagnosticDetail(reqID, key, m.claudePlanErr))
	return m
}

func (m Model) cancelClaudeTicketAssist() Model {
	if m.claudeAssistCancel != nil {
		m.claudeAssistCancel()
	}
	reqID := m.activeClaudeAssistReqID
	key := m.claudeAssistKey
	m.claudeAssistCancel = nil
	m.claudeAssistEvents = nil
	m.activeClaudeAssistReqID = 0
	m.claudeAssistLoading = false
	m.claudeAssistErr = errors.New("Claude ticket assist cancelled")
	m.claudeAssistText = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_assist", "cancel", workerDiagnosticDetail(reqID, key, m.claudeAssistErr))
	return m
}

func (m Model) handleClaudePlanProgress(msg claudePlanProgressMsg) Model {
	if msg.id != m.activeClaudePlanReqID || msg.key != m.claudePlanKey {
		return m
	}
	if strings.TrimSpace(msg.event.Text) == "" {
		return m
	}
	m.claudePlanProgress = append(m.claudePlanProgress, msg.event)
	if len(m.claudePlanProgress) > 6 {
		m.claudePlanProgress = append([]claude.Event(nil), m.claudePlanProgress[len(m.claudePlanProgress)-6:]...)
	}
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_plan", "progress", truncate(msg.event.Kind+" "+msg.event.Text, 100))
	return m
}

func (m Model) handleClaudeAssistProgress(msg claudeAssistProgressMsg) Model {
	if msg.id != m.activeClaudeAssistReqID || msg.key != m.claudeAssistKey {
		return m
	}
	if strings.TrimSpace(msg.event.Text) == "" {
		return m
	}
	m.claudeAssistProgress = append(m.claudeAssistProgress, msg.event)
	if len(m.claudeAssistProgress) > 6 {
		m.claudeAssistProgress = append([]claude.Event(nil), m.claudeAssistProgress[len(m.claudeAssistProgress)-6:]...)
	}
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_assist", "progress", truncate(msg.event.Kind+" "+msg.event.Text, 100))
	return m
}

func (m Model) handleClaudePlanResult(msg claudePlanResultMsg) Model {
	status := "ok"
	if msg.err != nil {
		status = "error"
		if errors.Is(msg.err, context.Canceled) {
			status = "cancel"
		} else if errors.Is(msg.err, context.DeadlineExceeded) {
			status = "timeout"
		}
	}
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_plan", status, workerDiagnosticDetail(msg.id, msg.key, msg.err))
	if msg.id != m.activeClaudePlanReqID || msg.key != m.claudePlanKey {
		return m
	}
	m.claudePlanLoading = false
	m.claudePlanCancel = nil
	m.claudePlanEvents = nil
	m.claudePlanOpen = true
	m.claudePlanErr = msg.err
	if msg.err == nil {
		m.claudePlanText = strings.TrimSpace(msg.text)
		m.claudePlanOffset = 0
	}
	return m
}

func (m Model) handleClaudeAssistResult(msg claudeAssistResultMsg) Model {
	status := "ok"
	if msg.err != nil {
		status = "error"
		if errors.Is(msg.err, context.Canceled) {
			status = "cancel"
		} else if errors.Is(msg.err, context.DeadlineExceeded) {
			status = "timeout"
		}
	}
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_assist", status, workerDiagnosticDetail(msg.id, msg.key, msg.err))
	if msg.id != m.activeClaudeAssistReqID || msg.key != m.claudeAssistKey {
		return m
	}
	m.claudeAssistLoading = false
	m.claudeAssistCancel = nil
	m.claudeAssistEvents = nil
	m.claudeAssistOpen = true
	m.claudeAssistErr = msg.err
	if msg.err == nil {
		m.claudeAssistText = strings.TrimSpace(msg.text)
		m.claudeAssistDraft = claudeAssistDraftFromText(m.claudeAssistText)
		m.claudeAssistEditor = newClaudeAssistEditor(m.claudeAssistDraft)
		m.claudeAssistEditorReady = true
	}
	return m
}

func (m Model) renderClaudePlanLoading(width int, now time.Time) string {
	elapsed := formatClaudeDuration(now.Sub(m.claudePlanStartedAt))
	if m.claudeConfig.Timeout > 0 {
		elapsed += " of " + m.claudeConfig.Timeout.String()
	}
	lines := []string{
		m.theme.Muted.Render("Activity: ") + m.theme.Text.Render(claudeActivityFrame(now.Sub(m.claudePlanStartedAt))+" Claude subprocess running"),
		m.theme.Muted.Render("Elapsed: ") + m.theme.Text.Render(elapsed),
	}
	lines = append(lines, m.renderClaudeProgressStatus(m.claudePlanProgress)...)
	return strings.Join(lines, "\n")
}

func claudeActivityFrame(elapsed time.Duration) string {
	frames := []string{"|", "/", "-", "\\"}
	if elapsed < 0 {
		elapsed = 0
	}
	return frames[int(elapsed/time.Second)%len(frames)]
}

func (m Model) renderClaudeProgressLines(width int) []string {
	return m.renderClaudeEventProgressLines(m.claudePlanProgress, width)
}

func (m Model) renderClaudeProgressStatus(events []claude.Event) []string {
	status := "waiting for first response"
	if len(events) > 0 {
		status = "receiving response"
		if claudeAssistantPreview(events) == "" {
			status = "receiving CLI messages"
		}
	}
	return []string{m.theme.Muted.Render("Output: ") + m.theme.Text.Render(status)}
}

func (m Model) renderClaudeEventProgressLines(events []claude.Event, width int) []string {
	preview := claudeAssistantPreview(events)
	if preview == "" {
		if len(events) > 0 {
			return []string{m.theme.Muted.Render("Output: ") + m.theme.Text.Render("waiting for assistant text")}
		}
		return []string{m.theme.Muted.Render("Output: ") + m.theme.Text.Render("waiting for first response")}
	}
	prefix := "Assistant: "
	return []string{
		m.theme.Muted.Render("Output: ") + m.theme.Text.Render("assistant text"),
		m.theme.Muted.Render(prefix) + m.theme.Text.Render(truncate(preview, max(16, width-lipgloss.Width(prefix)))),
	}
}

func claudeAssistantPreview(events []claude.Event) string {
	var preview string
	for _, event := range events {
		if event.Kind != "output" && event.Kind != "result" && event.Kind != "stderr" {
			continue
		}
		text := strings.Join(strings.Fields(strings.TrimSpace(event.Text)), " ")
		if text == "" || looksLikeJSONEvent(text) {
			continue
		}
		if preview == "" {
			preview = text
			continue
		}
		switch {
		case text == preview:
			continue
		case strings.HasPrefix(text, preview):
			preview = text
		case strings.HasPrefix(preview, text):
			continue
		case strings.Contains(preview, text):
			continue
		default:
			joiner := " "
			if strings.HasSuffix(preview, " ") || strings.HasPrefix(text, " ") {
				joiner = ""
			}
			preview += joiner + text
		}
	}
	return preview
}

func looksLikeJSONEvent(text string) bool {
	return strings.HasPrefix(text, "{") && strings.Contains(text, `"type"`)
}

func (m Model) renderClaudePlanError(width int, now time.Time) string {
	if errors.Is(m.claudePlanErr, context.DeadlineExceeded) {
		lines := []string{
			m.renderDetailNotice("Claude plan timed out after "+displayValue(m.claudeConfig.Timeout.String(), "the configured timeout"), width),
			"",
			m.theme.Muted.Render("Started: ") + m.theme.Text.Render(formatClockTime(m.claudePlanStartedAt)),
		}
		if m.claudeConfig.Timeout > 0 {
			lines = append(lines, m.theme.Muted.Render("Deadline: ")+m.theme.Text.Render(formatClockTime(m.claudePlanStartedAt.Add(m.claudeConfig.Timeout))))
		}
		lines = append(lines, m.theme.Muted.Render("Elapsed: ")+m.theme.Text.Render(formatClaudeDuration(now.Sub(m.claudePlanStartedAt))))
		lines = append(lines, m.theme.Muted.Render("Command: ")+m.theme.Text.Render(m.claudeCommandLabel()))
		return strings.Join(lines, "\n")
	}
	return m.renderDetailNotice("Claude plan failed: "+m.claudePlanErr.Error(), width)
}

func (m Model) claudeCommandLabel() string {
	command := strings.TrimSpace(m.claudeConfig.Command)
	if command == "" {
		command = strings.TrimSpace(m.claudeStatus.Command)
	}
	if command == "" {
		command = "claude"
	}
	return command + " -p <prompt>"
}

func (m Model) claudeNow() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

func (m Model) buildClaudeTicketPlanPrompt(ctx detailRenderContext) string {
	issue := ctx.display
	if issue.Key == "" {
		issue.Key = ctx.selected.Key
	}
	var b strings.Builder
	b.WriteString("Create a read-only implementation and verification plan for this Jira ticket.\n")
	b.WriteString("Do not edit files, create branches, run git commands, call Jira, or make external changes.\n")
	b.WriteString("Focus on likely code areas, risks, test strategy, and questions to clarify before implementation.\n\n")
	b.WriteString("Ticket:\n")
	writePromptField(&b, "Key", issue.Key)
	writePromptField(&b, "Summary", issue.Summary)
	writePromptField(&b, "Status", issue.Status)
	writePromptField(&b, "Issue Type", issue.IssueType)
	writePromptField(&b, "Priority", issue.Priority)
	writePromptField(&b, "Assignee", issue.Assignee)
	writePromptField(&b, "Reporter", ctx.detail.Reporter)
	if len(ctx.detail.Labels) > 0 {
		writePromptField(&b, "Labels", strings.Join(ctx.detail.Labels, ", "))
	}
	if len(ctx.detail.Components) > 0 {
		writePromptField(&b, "Components", strings.Join(ctx.detail.Components, ", "))
	}
	description := strings.TrimSpace(ctx.description)
	if description == "" {
		description = strings.TrimSpace(ctx.detail.Description)
	}
	if description != "" {
		b.WriteString("\nDescription:\n")
		b.WriteString(description)
		b.WriteString("\n")
	}
	comments := m.comments[issue.Key]
	if len(comments) > 0 {
		b.WriteString("\nLoaded comments:\n")
		for index, comment := range comments {
			author := displayValue(comment.Author, "Unknown")
			body := strings.TrimSpace(comment.Body)
			if body == "" {
				continue
			}
			fmt.Fprintf(&b, "%d. %s: %s\n", index+1, author, body)
		}
	}
	return strings.TrimSpace(b.String())
}

func (m Model) buildClaudeTicketAssistPrompt(ctx detailRenderContext) string {
	var b strings.Builder
	b.WriteString("Evaluate and sanitize this existing Jira ticket.\n")
	b.WriteString("Do not update Jira, create tickets, edit files, create branches, run git commands, call GitHub, or make external changes.\n")
	b.WriteString("Return practical ticket-writing help only. Do not invent product decisions; list unknowns as Open Questions.\n")
	b.WriteString("Acceptance Criteria must be a first-class section in the draft, not buried inside prose.\n")
	b.WriteString("Use this exact high-level structure:\n")
	b.WriteString("Review\n")
	b.WriteString("- Clarity issues\n")
	b.WriteString("- Missing acceptance criteria\n")
	b.WriteString("- Conflicting or stale context\n")
	b.WriteString("- Implementation or test gaps\n")
	b.WriteString("- Open questions\n\n")
	b.WriteString("Draft\n")
	b.WriteString("Summary: <one concise summary>\n\n")
	b.WriteString("Problem / Goal\n<clear user/business goal>\n\n")
	b.WriteString("Acceptance Criteria\n- [ ] <testable criterion>\n\n")
	b.WriteString("Test / Verification\n- <verification step>\n\n")
	b.WriteString("Implementation Notes\n- <notes or constraints>\n\n")
	b.WriteString("Open Questions\n- <question or None>\n\n")
	b.WriteString("Ticket:\n")
	m.writeClaudeTicketContext(&b, ctx)
	return strings.TrimSpace(b.String())
}

func (m Model) buildClaudeTicketAssistRefinementPrompt(ctx detailRenderContext, currentDraft string, instruction string) string {
	var b strings.Builder
	b.WriteString("Refine this Jira ticket draft using the user's instruction.\n")
	b.WriteString("Do not update Jira, create tickets, edit files, create branches, run git commands, call GitHub, or make external changes.\n")
	b.WriteString("Do not reinvent the draft from scratch. Preserve useful user edits from the current draft unless the instruction asks otherwise.\n")
	b.WriteString("Acceptance Criteria must remain a first-class section in the draft, not buried inside prose.\n")
	b.WriteString("Return the same high-level structure as Ticket Assist:\n")
	b.WriteString("Review\n")
	b.WriteString("- What changed and why\n")
	b.WriteString("- Remaining risks or open questions\n\n")
	b.WriteString("Draft\n")
	b.WriteString("Summary: <one concise summary>\n\n")
	b.WriteString("Problem / Goal\n<clear user/business goal>\n\n")
	b.WriteString("Acceptance Criteria\n- [ ] <testable criterion>\n\n")
	b.WriteString("Test / Verification\n- <verification step>\n\n")
	b.WriteString("Implementation Notes\n- <notes or constraints>\n\n")
	b.WriteString("Open Questions\n- <question or None>\n\n")
	b.WriteString("User instruction:\n")
	b.WriteString(strings.TrimSpace(instruction))
	b.WriteString("\n\nCurrent user-edited draft:\n")
	b.WriteString(strings.TrimSpace(currentDraft))
	b.WriteString("\n\nOriginal ticket context:\n")
	m.writeClaudeTicketContext(&b, ctx)
	return strings.TrimSpace(b.String())
}

func (m Model) buildInlineDescriptionAIPrompt(ctx detailRenderContext, action inlineAIAction, instruction string) string {
	var b strings.Builder
	b.WriteString("Provide Description-scoped Jira ticket assistance.\n")
	b.WriteString("Do not update Jira, create tickets, edit files, create branches, run git commands, call GitHub, edit code, or make external changes.\n")
	b.WriteString("Return practical writing help only. The draft must be local TUI text for user review.\n")
	b.WriteString("Selected inline action: ")
	b.WriteString(action.Label)
	b.WriteString("\n")
	if strings.TrimSpace(instruction) != "" {
		b.WriteString("User question/instruction:\n")
		b.WriteString(strings.TrimSpace(instruction))
		b.WriteString("\n")
	}
	b.WriteString("Use this exact high-level structure:\n")
	b.WriteString("Review\n")
	b.WriteString("- What changed or what you noticed\n")
	b.WriteString("- Risks or open questions\n\n")
	b.WriteString("Draft\n")
	if action.ID == "draft_comment" {
		b.WriteString("<Jira comment draft>\n\n")
	} else {
		b.WriteString("<replacement Description draft>\n\n")
	}
	b.WriteString("Ticket:\n")
	m.writeClaudeTicketContext(&b, ctx)
	return strings.TrimSpace(b.String())
}

func (m Model) writeClaudeTicketContext(b *strings.Builder, ctx detailRenderContext) {
	issue := ctx.display
	if issue.Key == "" {
		issue.Key = ctx.selected.Key
	}
	writePromptField(b, "Key", issue.Key)
	writePromptField(b, "Summary", issue.Summary)
	writePromptField(b, "Status", issue.Status)
	writePromptField(b, "Issue Type", issue.IssueType)
	writePromptField(b, "Priority", issue.Priority)
	writePromptField(b, "Assignee", issue.Assignee)
	writePromptField(b, "Reporter", ctx.detail.Reporter)
	if len(ctx.detail.Labels) > 0 {
		writePromptField(b, "Labels", strings.Join(ctx.detail.Labels, ", "))
	}
	if len(ctx.detail.Components) > 0 {
		writePromptField(b, "Components", strings.Join(ctx.detail.Components, ", "))
	}
	description := strings.TrimSpace(ctx.description)
	if description == "" {
		description = strings.TrimSpace(ctx.detail.Description)
	}
	if description != "" {
		b.WriteString("\nDescription:\n")
		b.WriteString(description)
		b.WriteString("\n")
	}
	comments := m.comments[issue.Key]
	if len(comments) > 0 {
		b.WriteString("\nLoaded comments:\n")
		for index, comment := range comments {
			author := displayValue(comment.Author, "Unknown")
			body := strings.TrimSpace(comment.Body)
			if body == "" {
				continue
			}
			fmt.Fprintf(b, "%d. %s: %s\n", index+1, author, body)
		}
	}
}

func (m Model) claudeAssistReviewText() string {
	review, _ := splitClaudeAssistText(m.claudeAssistText)
	return review
}

func claudeAssistDraftFromText(text string) string {
	_, draft := splitClaudeAssistText(text)
	if strings.TrimSpace(draft) == "" {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(draft)
}

func splitClaudeAssistText(text string) (string, string) {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	draftIndex := -1
	for index, line := range lines {
		normalized := strings.Trim(strings.TrimSpace(line), "#: ")
		if strings.EqualFold(normalized, "Draft") {
			draftIndex = index
			break
		}
	}
	if draftIndex < 0 {
		return "", strings.TrimSpace(text)
	}
	review := strings.TrimSpace(strings.Join(lines[:draftIndex], "\n"))
	draft := strings.TrimSpace(strings.Join(lines[draftIndex+1:], "\n"))
	return review, draft
}

func parseCreateIssueDraft(text string) (summary string, description string) {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	summaryIndex := -1
	descriptionIndex := -1
	for index, line := range lines {
		header := strings.TrimLeft(strings.TrimSpace(line), "#")
		header = strings.TrimSpace(strings.Trim(header, ":"))
		header = strings.ToLower(header)
		switch {
		case strings.HasPrefix(header, "summary"):
			if summaryIndex < 0 {
				summaryIndex = index
			}
		case strings.HasPrefix(header, "description"):
			if descriptionIndex < 0 {
				descriptionIndex = index
			}
		}
	}
	if summaryIndex < 0 && descriptionIndex < 0 {
		return "", ""
	}
	extractAfterHeader := func(start int) string {
		if start < 0 {
			return ""
		}
		line := strings.TrimSpace(lines[start])
		if i := strings.Index(line, ":"); i >= 0 {
			if value := strings.TrimSpace(line[i+1:]); value != "" {
				return value
			}
		}
		parts := make([]string, 0, len(lines)-start-1)
		for index := start + 1; index < len(lines); index++ {
			candidate := strings.TrimSpace(lines[index])
			lower := strings.ToLower(strings.Trim(candidate, "#:"))
			if strings.HasPrefix(lower, "summary") || strings.HasPrefix(lower, "description") || strings.HasPrefix(lower, "acceptance criteria") || strings.HasPrefix(lower, "open questions") || strings.HasPrefix(lower, "test / verification") || strings.HasPrefix(lower, "implementation notes") {
				break
			}
			if candidate != "" || len(parts) > 0 {
				parts = append(parts, lines[index])
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	summary = strings.TrimSpace(extractAfterHeader(summaryIndex))
	description = strings.TrimSpace(extractAfterHeader(descriptionIndex))
	if summary == "" && summaryIndex >= 0 {
		summary = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[summaryIndex]), "Summary"))
	}
	if description == "" && summaryIndex >= 0 && descriptionIndex < 0 && summaryIndex+1 < len(lines) {
		description = strings.TrimSpace(strings.TrimSpace(strings.Join(lines[summaryIndex+1:], "\n")))
	}
	return summary, description
}

func parseCreateIssueDraftFields(text string) map[string]string {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	sections := map[string]string{}
	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if line == "" || strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			continue
		}
		label := ""
		value := ""
		if before, after, ok := strings.Cut(line, ":"); ok && len(strings.TrimSpace(before)) <= 40 {
			label = strings.TrimSpace(strings.Trim(before, "#* "))
			value = strings.TrimSpace(after)
		} else if isCreateDraftFieldHeader(line) {
			label = strings.TrimSpace(strings.Trim(line, "#* "))
		}
		if label == "" {
			continue
		}
		var values []string
		if value != "" {
			values = append(values, value)
		}
		for next := index + 1; next < len(lines); next++ {
			candidate := strings.TrimSpace(lines[next])
			if candidate == "" {
				if len(values) > 0 {
					break
				}
				continue
			}
			if isCreateDraftFieldHeader(candidate) || isCreateDraftInlineField(candidate) {
				break
			}
			values = append(values, strings.TrimPrefix(strings.TrimPrefix(candidate, "- "), "* "))
			index = next
		}
		if len(values) > 0 {
			sections[normalizeCreateDraftFieldName(label)] = strings.TrimSpace(strings.Join(values, "\n"))
		}
	}
	return sections
}

func parseCreateIssueOpenQuestions(text string) []createAIQuestion {
	fields := parseCreateIssueDraftFields(text)
	value := strings.TrimSpace(fields["openquestions"])
	if value == "" {
		return nil
	}
	var questions []createAIQuestion
	for _, line := range strings.Split(value, "\n") {
		question := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "))
		if question == "" || strings.EqualFold(question, "none") || strings.EqualFold(question, "n/a") {
			continue
		}
		questions = append(questions, createAIQuestion{Question: question})
	}
	return questions
}

func isCreateDraftInlineField(line string) bool {
	before, _, ok := strings.Cut(strings.TrimSpace(line), ":")
	return ok && len(strings.TrimSpace(before)) <= 40
}

func isCreateDraftFieldHeader(line string) bool {
	line = strings.TrimSpace(strings.Trim(line, "#* "))
	if line == "" || len(line) > 48 {
		return false
	}
	if strings.ContainsAny(line, ".?!") {
		return false
	}
	switch normalizeCreateDraftFieldName(line) {
	case "issuetype", "summary", "description", "components", "component", "priority", "labels", "label", "investmentcategory", "releaseinstructions", "openquestions":
		return true
	default:
		return false
	}
}

func normalizeCreateDraftFieldName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func writePromptField(b *strings.Builder, label string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(b, "- %s: %s\n", label, value)
}

func formatClaudeDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	return duration.Round(time.Second).String()
}

func formatClockTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("15:04:05")
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

func (m Model) canUseClaudeSelection() bool {
	if m.mode != modeDetail {
		return false
	}
	section, ok := m.focusedDetailSection()
	return ok && section.ID == "claude"
}

func (m *Model) moveSelectedClaudeAction(delta int) {
	actions := m.claudeActions()
	if len(actions) == 0 {
		m.selectedClaudeAction = 0
		return
	}
	m.selectedClaudeAction = clamp(m.selectedClaudeAction+delta, 0, len(actions)-1)
}

func (m Model) runSelectedClaudeAction() (Model, tea.Cmd) {
	actions := m.claudeActions()
	if len(actions) == 0 {
		m.detailNotice = "Claude ticket assistance is not enabled or available."
		return m, nil
	}
	action := actions[clamp(m.selectedClaudeAction, 0, len(actions)-1)]
	switch action.ID {
	case "ticket_plan":
		return m.startClaudeTicketPlan()
	case "ticket_assist":
		return m.startClaudeTicketAssist()
	default:
		return m, nil
	}
}

func (m *Model) focusStatusTransitions() {
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = true
	m.jumpDetailSection("Status")
}

func (m Model) startStatusTransitionPicker() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.jumpDetailSection("Status")
	if transitions := m.transitions[selected.Key]; len(transitions) > 0 {
		m.transitionFocus = true
		m.selectedTransition = clamp(m.selectedTransition, 0, len(transitions)-1)
		m.detailNotice = ""
		return m, nil
	}
	if m.transitionLoading && m.transitionRequestKey == selected.Key {
		return m, nil
	}
	m.nextRequestID++
	m.activeTransitionsReqID = m.nextRequestID
	m.transitionRequestKey = selected.Key
	m.transitionLoading = true
	m.transitionErr = nil
	m.transitionFocus = false
	m.detailNotice = "Loading status transitions for " + selected.Key + "."
	return m, m.submitIssueTransitions(m.activeTransitionsReqID, selected.Key)
}

func (m *Model) moveSelectedTransition(delta int) {
	selected, ok := m.selectedIssue()
	if !ok {
		m.selectedTransition = 0
		return
	}
	transitions := m.transitions[selected.Key]
	if len(transitions) == 0 {
		m.selectedTransition = 0
		return
	}
	m.selectedTransition = clamp(m.selectedTransition+delta, 0, len(transitions)-1)
}

func (m Model) submitSelectedTransition() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	transitions := m.transitions[selected.Key]
	if len(transitions) == 0 {
		return m.startStatusTransitionPicker()
	}
	transition := transitions[clamp(m.selectedTransition, 0, len(transitions)-1)]
	if transition.ID == "" {
		m.detailNotice = "Status update failed: missing transition ID."
		return m, nil
	}
	m.nextRequestID++
	m.activeTransitionReqID = m.nextRequestID
	m.transitionSubmitting = true
	m.transitionSubmitKey = selected.Key
	m.transitionSubmitToStatus = transition.ToStatus
	m.detailNotice = "Updating status to " + displayValue(transition.ToStatus, transition.Name) + "."
	return m, m.submitIssueTransition(m.activeTransitionReqID, selected.Key, transition)
}

func (m Model) startPriorityEditor() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.summaryFocus = false
	m.priorityFocus = true
	if metadata, ok := m.editMetadata[selected.Key]; ok {
		return m.beginPriorityEditing(metadata), nil
	}
	if m.priorityMetadataLoading && m.priorityMetadataRequestKey == selected.Key {
		return m, nil
	}
	m.nextRequestID++
	m.activePriorityMetadataReqID = m.nextRequestID
	m.priorityMetadataRequestKey = selected.Key
	m.priorityMetadataLoading = true
	m.priorityMetadataErr = nil
	m.detailNotice = ""
	return m, m.submitEditMetadata(m.activePriorityMetadataReqID, selected.Key)
}

func (m Model) beginPriorityEditing(metadata jira.EditMetadata) Model {
	selected, ok := m.selectedIssue()
	if !ok {
		return m
	}
	if !metadata.Priority.Editable {
		m.priorityFocus = false
		m.detailNotice = "Priority is not editable for " + selected.Key + "."
		return m
	}
	if len(metadata.Priority.AllowedValues) == 0 {
		m.priorityFocus = false
		m.detailNotice = "Priority metadata returned no allowed values for " + selected.Key + "."
		return m
	}
	m.priorityFocus = true
	m.selectedPriority = indexFieldOptionByName(metadata.Priority.AllowedValues, selected.Priority)
	m.detailNotice = ""
	return m
}

func (m Model) priorityOptions(key string) []jira.FieldOption {
	if metadata, ok := m.editMetadata[key]; ok {
		return metadata.Priority.AllowedValues
	}
	return nil
}

func (m *Model) moveSelectedPriority(delta int) {
	selected, ok := m.selectedIssue()
	if !ok {
		m.selectedPriority = 0
		return
	}
	options := m.priorityOptions(selected.Key)
	if len(options) == 0 {
		m.selectedPriority = 0
		return
	}
	m.selectedPriority = clamp(m.selectedPriority+delta, 0, len(options)-1)
}

func (m Model) submitSelectedPriority() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	options := m.priorityOptions(selected.Key)
	if len(options) == 0 {
		return m.startPriorityEditor()
	}
	priority := options[clamp(m.selectedPriority, 0, len(options)-1)]
	priorityName := displayValue(priority.Name, priority.ID)
	if strings.TrimSpace(priorityName) == "" {
		m.detailNotice = "Priority update failed: missing priority value."
		return m, nil
	}
	if priorityName == strings.TrimSpace(selected.Priority) {
		m.detailNotice = "Priority unchanged."
		return m, nil
	}
	m.nextRequestID++
	m.activePriorityReqID = m.nextRequestID
	m.prioritySubmitting = true
	m.prioritySubmitKey = selected.Key
	m.prioritySubmitValue = priority
	m.detailNotice = "Updating priority to " + priorityName + "."
	return m, m.submitUpdatePriority(m.activePriorityReqID, selected.Key, priority)
}

func (m Model) startAssigneePicker() (Model, tea.Cmd) {
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.priorityFocus = false
	m.summaryFocus = false
	m.assigneeFocus = true
	m.assigneeQuery = ""
	m.assigneeUsers = nil
	m.selectedAssignee = 0
	m.assigneeSearchLoading = false
	m.assigneeSearchErr = nil
	m.detailNotice = ""
	return m, nil
}

func (m Model) updateAssigneePicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.assigneeFocus = false
		m.assigneeQuery = ""
		m.assigneeUsers = nil
		m.selectedAssignee = 0
		m.assigneeSearchLoading = false
		m.assigneeSearchErr = nil
		m.detailNotice = ""
		return m, nil
	case "enter":
		return m.submitSelectedAssignee()
	case "up":
		m.moveSelectedAssignee(-1)
		return m, nil
	case "down":
		m.moveSelectedAssignee(1)
		return m, nil
	}
	query, changed := nextMentionQuery(m.assigneeQuery, msg)
	if !changed {
		return m, nil
	}
	m.assigneeQuery = query
	m.selectedAssignee = 0
	m.assigneeSearchErr = nil
	if strings.TrimSpace(query) == "" {
		m.assigneeUsers = nil
		m.assigneeSearchLoading = false
		return m, nil
	}
	if users, ok := m.cachedUserSearch(query); ok {
		m.assigneeUsers = users
		m.assigneeSearchLoading = false
		m.selectedAssignee = clamp(m.selectedAssignee, 0, max(0, len(users)-1))
		return m, nil
	}
	m.nextRequestID++
	m.assigneeSearchReqID = m.nextRequestID
	m.assigneeSearchLoading = true
	return m, m.submitUserSearch(m.assigneeSearchReqID, query)
}

func (m *Model) moveSelectedAssignee(delta int) {
	if len(m.assigneeUsers) == 0 {
		m.selectedAssignee = 0
		return
	}
	m.selectedAssignee = clamp(m.selectedAssignee+delta, 0, len(m.assigneeUsers)-1)
}

func (m Model) submitSelectedAssignee() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	if len(m.assigneeUsers) == 0 {
		m.detailNotice = "Search for a Jira user before assigning."
		return m, nil
	}
	assignee := m.assigneeUsers[clamp(m.selectedAssignee, 0, len(m.assigneeUsers)-1)]
	if strings.TrimSpace(assignee.AccountID) == "" {
		m.detailNotice = "Assignee update failed: missing account ID."
		return m, nil
	}
	assigneeName := displayValue(assignee.DisplayName, assignee.Email)
	if assigneeName == strings.TrimSpace(selected.Assignee) {
		m.detailNotice = "Assignee unchanged."
		return m, nil
	}
	m.nextRequestID++
	m.activeAssigneeReqID = m.nextRequestID
	m.assigneeSubmitting = true
	m.assigneeSubmitKey = selected.Key
	m.assigneeSubmitValue = assignee
	m.detailNotice = "Updating assignee to " + displayValue(assigneeName, "Unknown") + "."
	return m, m.submitUpdateAssignee(m.activeAssigneeReqID, selected.Key, assignee)
}

func (m Model) startCreateIssue() (Model, tea.Cmd) {
	projectKey := projectKeyFromJQL(m.jql)
	if projectKey == "" {
		projectKey = projectKeyFromJQL(m.filterSummary())
	}
	if projectKey == "" {
		m.detailNotice = "Create ticket needs a project key in the active view JQL."
		return m, nil
	}
	m.resetCreateIssueState()
	m.createOpen = true
	m.createProjectKey = projectKey
	m.createIssueTypesLoading = true
	m.nextRequestID++
	m.activeCreateIssueTypesReqID = m.nextRequestID
	return m, m.submitCreateIssueTypes(m.activeCreateIssueTypesReqID, projectKey)
}

func (m Model) updateCreatePaste(msg tea.PasteMsg) (Model, tea.Cmd) {
	if !m.createFormReady() || m.createSubmitting {
		return m, nil
	}
	if m.createAIQuestionAnswering && m.createQuestionsFieldFocused() {
		m.ensureCreateQuestionAnswerEditor()
		m.createAIQuestionEditor.InsertString(msg.String())
	} else if m.createFieldFocus == createSummaryFieldIndex {
		m.ensureCreateSummaryEditor()
		m.createSummaryEditor.InsertString(msg.String())
		m.createSummaryDraft = m.createSummaryEditor.Value()
	} else if m.createFieldFocus == createDescriptionFieldIndex {
		m.ensureCreateDescriptionEditor()
		m.createDescriptionEditor.InsertString(msg.String())
		m.createDescriptionDraft = m.createDescriptionEditor.Value()
	} else if field, ok := m.focusedCreateDynamicField(); ok && !createFieldUsesPicker(field) {
		m.setCreateDynamicValue(field, m.createDynamicValues[createFieldValueKey(field)]+msg.String())
	}
	m.detailNotice = ""
	return m, nil
}

func (m Model) updateCreateIssue(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" && m.createIssueType.ID == "" && m.createAIGeneratedMode && m.createAIPromptLoading {
		return m.cancelCreateAIPrompt(), nil
	}
	if msg.String() == "esc" && m.createAIQuestionAnswering {
		m.createAIQuestionAnswering = false
		m.createAIQuestionEditorReady = false
		m.detailNotice = ""
		return m, nil
	}
	if msg.String() == "esc" && m.createChangingType {
		m.createChangingType = false
		m.detailNotice = ""
		return m, nil
	}
	if msg.String() == "esc" {
		if field, ok := m.focusedCreateDynamicField(); ok && createFieldUsesPicker(field) {
			key := createFieldValueKey(field)
			if strings.TrimSpace(m.createDynamicFilters[key]) != "" {
				m.clearCreateDynamicFilter(field)
				m.detailNotice = ""
				return m, nil
			}
		}
	}
	switch msg.String() {
	case "esc":
		m.resetCreateIssueState()
		return m, nil
	}
	if m.createIssueType.ID == "" {
		if msg.String() == "tab" && m.claudeCreateTicketDraftEnabled() {
			m.createAIGeneratedMode = !m.createAIGeneratedMode
			m.detailNotice = ""
			if m.createAIGeneratedMode {
				m.ensureCreateAIPromptEditor()
			}
			return m, nil
		}
		if m.createAIGeneratedMode {
			if m.createAIPromptLoading {
				return m, nil
			}
			if msg.String() == "ctrl+s" {
				return m.submitCreateAIPrompt()
			}
			m.configureCreateAIPromptEditor(max(32, m.browserLayout(m.width).contentWidth-16), 8)
			editor, cmd := m.createAIPromptEditor.Update(msg)
			m.createAIPromptEditor = editor
			m.createAIPrompt = m.createAIPromptEditor.Value()
			return m, cmd
		}
		switch msg.String() {
		case "up", "k":
			m.moveSelectedCreateIssueType(-1)
			return m, nil
		case "down", "j":
			m.moveSelectedCreateIssueType(1)
			return m, nil
		case "g", "G":
			m.detailNotice = "Select an issue type before generating a ticket draft."
			return m, nil
		case "enter":
			return m.selectCreateIssueType()
		default:
			return m, nil
		}
	}
	if m.createChangingType {
		switch msg.String() {
		case "esc":
			m.createChangingType = false
			m.detailNotice = ""
			return m, nil
		case "up", "k":
			m.moveSelectedCreateIssueType(-1)
			return m, nil
		case "down", "j":
			m.moveSelectedCreateIssueType(1)
			return m, nil
		case "enter":
			m.createChangingType = false
			return m.selectCreateIssueType()
		default:
			return m, nil
		}
	}
	if !m.createFormReady() || m.createSubmitting {
		return m, nil
	}
	if m.createQuestionsFieldFocused() && m.createAIQuestionAnswering {
		return m.updateCreateQuestions(msg)
	}
	if m.createQuestionsFieldFocused() {
		switch msg.String() {
		case "up", "k", "down", "j":
			return m.updateCreateQuestions(msg)
		case "ctrl+r":
			return m.submitCreateQuestionRefinement()
		}
	}
	if field, ok := m.focusedCreateDynamicField(); ok && createFieldUsesPicker(field) {
		switch msg.String() {
		case "up", "k":
			m.moveCreateDynamicSelection(field, -1)
			return m, nil
		case "down", "j":
			m.moveCreateDynamicSelection(field, 1)
			return m, nil
		case "backspace", "ctrl+h":
			m.backspaceCreateDynamicFilter(field)
			return m, nil
		case "enter":
			m.clearCreateDynamicFilter(field)
			return m, nil
		}
		if len(msg.String()) == 1 {
			m.appendCreateDynamicFilter(field, msg.String())
			return m, nil
		}
	}
	switch msg.String() {
	case "tab":
		m.moveCreateFieldFocus(1)
		return m, nil
	case "shift+tab", "backtab":
		m.moveCreateFieldFocus(-1)
		return m, nil
	case "enter":
		if m.createFieldFocus == createTypeFieldIndex {
			m.startCreateIssueTypeChange()
			return m, nil
		}
		if m.createAIPromptFieldFocused() {
			return m.startCreateAIPrompt()
		}
		if m.createQuestionsFieldFocused() {
			return m.updateCreateQuestions(msg)
		}
	case "ctrl+s":
		return m.submitCreateIssueDraft()
	}
	if m.createFieldFocus == createSummaryFieldIndex {
		m.ensureCreateSummaryEditor()
		m.configureCreateSummaryEditor()
		editor, cmd := m.createSummaryEditor.Update(msg)
		m.createSummaryEditor = editor
		m.createSummaryDraft = m.createSummaryEditor.Value()
		m.detailNotice = ""
		return m, cmd
	}
	if field, ok := m.focusedCreateDynamicField(); ok {
		if createFieldUsesPicker(field) {
			return m, nil
		}
		switch msg.String() {
		case "backspace", "ctrl+h":
			value := []rune(m.createDynamicValues[createFieldValueKey(field)])
			if len(value) > 0 {
				m.setCreateDynamicValue(field, string(value[:len(value)-1]))
			}
			return m, nil
		}
		if len(msg.String()) == 1 {
			m.setCreateDynamicValue(field, m.createDynamicValues[createFieldValueKey(field)]+msg.String())
			return m, nil
		}
		return m, nil
	}
	m.ensureCreateDescriptionEditor()
	m.configureCreateDescriptionEditor()
	editor, cmd := m.createDescriptionEditor.Update(msg)
	m.createDescriptionEditor = editor
	m.createDescriptionDraft = m.createDescriptionEditor.Value()
	m.detailNotice = ""
	return m, cmd
}

func (m Model) updateCreateQuestions(msg tea.KeyMsg) (Model, tea.Cmd) {
	if len(m.createAIQuestions) == 0 {
		return m, nil
	}
	if m.createAIQuestionAnswering {
		switch msg.String() {
		case "enter":
			m.saveCreateQuestionAnswer()
			if m.selectedCreateAIQuestion < len(m.createAIQuestions)-1 {
				m.selectedCreateAIQuestion++
				m.createAIQuestionEditor = newCreateQuestionAnswerEditor(m.createAIQuestions[m.selectedCreateAIQuestion].Answer)
				m.createAIQuestionEditorReady = true
				m.createAIQuestionAnswering = true
				m.detailNotice = "Saved answer. Next question."
				return m, nil
			}
			m.createAIQuestionAnswering = false
			m.createAIQuestionEditorReady = false
			m.detailNotice = "Saved final question answer locally."
			return m, nil
		case "ctrl+s":
			m.saveCreateQuestionAnswer()
			m.createAIQuestionAnswering = false
			m.createAIQuestionEditorReady = false
			m.detailNotice = "Saved question answer locally."
			return m, nil
		}
		m.ensureCreateQuestionAnswerEditor()
		editor, cmd := m.createAIQuestionEditor.Update(msg)
		m.createAIQuestionEditor = editor
		m.detailNotice = ""
		return m, cmd
	}
	switch msg.String() {
	case "up", "k":
		m.selectedCreateAIQuestion = clamp(m.selectedCreateAIQuestion-1, 0, len(m.createAIQuestions)-1)
		return m, nil
	case "down", "j":
		m.selectedCreateAIQuestion = clamp(m.selectedCreateAIQuestion+1, 0, len(m.createAIQuestions)-1)
		return m, nil
	case "enter":
		selected := clamp(m.selectedCreateAIQuestion, 0, len(m.createAIQuestions)-1)
		m.createAIQuestionEditor = newCreateQuestionAnswerEditor(m.createAIQuestions[selected].Answer)
		m.createAIQuestionEditorReady = true
		m.createAIQuestionAnswering = true
		m.detailNotice = ""
		return m, nil
	}
	return m, nil
}

func (m Model) submitCreateQuestionRefinement() (Model, tea.Cmd) {
	m.createAIPrompt = "Refine the current ticket draft using my answers to the Open Questions."
	return m.submitCreateAIPrompt()
}

func (m *Model) saveCreateQuestionAnswer() {
	if len(m.createAIQuestions) == 0 {
		return
	}
	selected := clamp(m.selectedCreateAIQuestion, 0, len(m.createAIQuestions)-1)
	m.createAIQuestions[selected].Answer = strings.TrimSpace(m.createAIQuestionEditor.Value())
}

func (m *Model) moveSelectedCreateIssueType(delta int) {
	if len(m.createIssueTypes) == 0 {
		m.selectedCreateIssueType = 0
		return
	}
	m.selectedCreateIssueType = clamp(m.selectedCreateIssueType+delta, 0, len(m.createIssueTypes)-1)
}

func (m Model) selectCreateIssueType() (Model, tea.Cmd) {
	if len(m.createIssueTypes) == 0 {
		m.detailNotice = "No Jira issue types are available."
		return m, nil
	}
	m.createIssueType = m.createIssueTypes[clamp(m.selectedCreateIssueType, 0, len(m.createIssueTypes)-1)]
	m.createChangingType = false
	if strings.TrimSpace(m.createIssueType.ID) == "" {
		m.detailNotice = "Create ticket failed: missing issue type ID."
		return m, nil
	}
	m.createFieldsLoading = true
	m.createFieldsErr = nil
	m.nextRequestID++
	m.activeCreateFieldsReqID = m.nextRequestID
	return m, m.submitCreateFields(m.activeCreateFieldsReqID, m.createProjectKey, m.createIssueType.ID)
}

func (m *Model) startCreateIssueTypeChange() {
	m.createChangingType = true
	m.detailNotice = ""
	for index, issueType := range m.createIssueTypes {
		if createIssueTypeMatches(issueType, m.createIssueType.Name) || createIssueTypeMatches(issueType, m.createIssueType.ID) {
			m.selectedCreateIssueType = index
			return
		}
	}
}

func (m *Model) beginCreateForm() {
	m.createChangingType = false
	m.createFieldFocus = createSummaryFieldIndex
	m.createSummaryEditor = newSummaryEditor(m.createSummaryDraft)
	m.createSummaryEditorReady = true
	m.createDescriptionEditor = newCommentEditor(m.createDescriptionDraft)
	m.createDescriptionEditorReady = true
	m.createDynamicValues = map[string]string{}
	m.createDynamicSelections = map[string]int{}
	m.createDynamicFilters = map[string]string{}
	for _, field := range supportedCreateFields(m.createFields) {
		key := createFieldValueKey(field)
		m.createDynamicValues[key] = ""
		m.createDynamicSelections[key] = defaultCreateFieldSelection(field)
		m.createDynamicFilters[key] = ""
	}
	m.detailNotice = ""
}

func (m *Model) applyCreateAIFieldDrafts() {
	if len(m.createAIFieldDrafts) == 0 {
		return
	}
	for _, field := range supportedCreateFields(m.createFields) {
		value, ok := m.createAIFieldDraftFor(field)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		key := createFieldValueKey(field)
		if createFieldUsesPicker(field) {
			if index, ok := bestCreateFieldOptionMatch(value, field.AllowedValues); ok {
				m.createDynamicSelections[key] = index
			}
			continue
		}
		m.setCreateDynamicValue(field, value)
	}
}

func (m Model) createAIFieldDraftFor(field jira.CreateField) (string, bool) {
	for _, candidate := range []string{field.Name, field.ID, field.Key, field.SchemaSystem} {
		key := normalizeCreateDraftFieldName(candidate)
		if key == "" {
			continue
		}
		if value, ok := m.createAIFieldDrafts[key]; ok {
			return value, true
		}
	}
	return "", false
}

func bestCreateFieldOptionMatch(value string, options []jira.FieldOption) (int, bool) {
	normalizedValue := normalizeCreateDraftFieldName(value)
	if normalizedValue == "" {
		return 0, false
	}
	for index, option := range options {
		optionName := normalizeCreateDraftFieldName(option.Name)
		optionID := normalizeCreateDraftFieldName(option.ID)
		if optionName != "" && (strings.Contains(normalizedValue, optionName) || strings.Contains(optionName, normalizedValue)) {
			return index, true
		}
		if optionID != "" && (strings.Contains(normalizedValue, optionID) || strings.Contains(optionID, normalizedValue)) {
			return index, true
		}
	}
	return 0, false
}

func createIssueTypeMatches(issueType jira.CreateIssueType, value string) bool {
	normalizedValue := normalizeCreateDraftFieldName(value)
	if normalizedValue == "" {
		return false
	}
	for _, candidate := range []string{issueType.Name, issueType.ID} {
		if normalizeCreateDraftFieldName(candidate) == normalizedValue {
			return true
		}
	}
	return false
}

func (m *Model) moveCreateFieldFocus(delta int) {
	total := 3 + len(supportedCreateFields(m.createFields))
	if len(m.createAIQuestions) > 0 {
		total++
	}
	if m.claudeCreateTicketDraftEnabled() {
		total++
	}
	if total <= 0 {
		m.createFieldFocus = createSummaryFieldIndex
		return
	}
	m.createFieldFocus = (m.createFieldFocus + delta + total) % total
}

func (m Model) focusedCreateDynamicField() (jira.CreateField, bool) {
	index := m.createFieldFocus - m.createDynamicFieldStartIndex()
	fields := supportedCreateFields(m.createFields)
	if index < 0 || index >= len(fields) {
		return jira.CreateField{}, false
	}
	return fields[index], true
}

func (m *Model) moveCreateDynamicSelection(field jira.CreateField, delta int) {
	if len(field.AllowedValues) == 0 {
		return
	}
	key := createFieldValueKey(field)
	matches := filteredCreateFieldOptionIndexes(field.AllowedValues, m.createDynamicFilters[key])
	if len(matches) == 0 {
		return
	}
	position := createOptionMatchPosition(matches, m.createDynamicSelections[key])
	if position < 0 {
		position = 0
	} else {
		position = clamp(position+delta, 0, len(matches)-1)
	}
	m.createDynamicSelections[key] = matches[position]
}

func (m *Model) appendCreateDynamicFilter(field jira.CreateField, value string) {
	key := createFieldValueKey(field)
	if m.createDynamicFilters == nil {
		m.createDynamicFilters = map[string]string{}
	}
	m.createDynamicFilters[key] += value
	m.selectFirstFilteredCreateOption(field)
}

func (m *Model) backspaceCreateDynamicFilter(field jira.CreateField) {
	key := createFieldValueKey(field)
	value := []rune(m.createDynamicFilters[key])
	if len(value) == 0 {
		return
	}
	m.createDynamicFilters[key] = string(value[:len(value)-1])
	m.selectFirstFilteredCreateOption(field)
}

func (m *Model) clearCreateDynamicFilter(field jira.CreateField) {
	if m.createDynamicFilters == nil {
		return
	}
	m.createDynamicFilters[createFieldValueKey(field)] = ""
}

func (m *Model) selectFirstFilteredCreateOption(field jira.CreateField) {
	key := createFieldValueKey(field)
	matches := filteredCreateFieldOptionIndexes(field.AllowedValues, m.createDynamicFilters[key])
	if len(matches) == 0 {
		m.createDynamicSelections[key] = -1
		return
	}
	m.createDynamicSelections[key] = matches[0]
}

func (m *Model) setCreateDynamicValue(field jira.CreateField, value string) {
	if m.createDynamicValues == nil {
		m.createDynamicValues = map[string]string{}
	}
	m.createDynamicValues[createFieldValueKey(field)] = value
}

func (m Model) createAIPromptFieldIndex() int {
	if !m.claudeCreateTicketDraftEnabled() {
		return -1
	}
	return m.createDynamicFieldStartIndex() - 1
}

func (m Model) createAIPromptFieldFocused() bool {
	aiIndex := m.createAIPromptFieldIndex()
	return aiIndex >= 0 && m.createFieldFocus == aiIndex
}

func (m Model) createFormReady() bool {
	return m.createIssueType.ID != "" && !m.createFieldsLoading && m.createFieldsErr == nil
}

func (m Model) createDynamicFieldFocusIndex(index int) int {
	return m.createDynamicFieldStartIndex() + index
}

func (m Model) createDynamicFieldStartIndex() int {
	start := 3
	if len(m.createAIQuestions) > 0 {
		start++
	}
	if m.claudeCreateTicketDraftEnabled() {
		start++
	}
	return start
}

func (m Model) createQuestionsFieldIndex() int {
	if len(m.createAIQuestions) == 0 {
		return -1
	}
	return 3
}

func (m Model) createQuestionsFieldFocused() bool {
	index := m.createQuestionsFieldIndex()
	return index >= 0 && m.createFieldFocus == index
}

func (m Model) createIssueFieldValues() ([]jira.CreateIssueFieldValue, error) {
	if unsupported := unsupportedRequiredCreateFields(m.createFields); len(unsupported) > 0 {
		return nil, fmt.Errorf("Jira requires unsupported fields: %s", strings.Join(unsupported, ", "))
	}
	var values []jira.CreateIssueFieldValue
	for _, field := range supportedCreateFields(m.createFields) {
		key := createFieldValueKey(field)
		value := jira.CreateIssueFieldValue{
			FieldID:      displayValue(field.ID, field.Key),
			SchemaType:   field.SchemaType,
			SchemaSystem: field.SchemaSystem,
		}
		if createFieldUsesPicker(field) {
			if len(field.AllowedValues) == 0 {
				if field.Required {
					return nil, fmt.Errorf("%s has no Jira options.", displayValue(field.Name, value.FieldID))
				}
				continue
			}
			selected := m.createDynamicSelections[key]
			if selected < 0 || selected >= len(field.AllowedValues) {
				if field.Required {
					return nil, fmt.Errorf("%s cannot be empty.", displayValue(field.Name, value.FieldID))
				}
				continue
			}
			value.Option = field.AllowedValues[selected]
		} else {
			value.Text = strings.TrimSpace(m.createDynamicValues[key])
			if field.Required && value.Text == "" {
				return nil, fmt.Errorf("%s cannot be empty.", displayValue(field.Name, value.FieldID))
			}
			if value.Text == "" {
				continue
			}
		}
		values = append(values, value)
	}
	return values, nil
}

func (m Model) submitCreateIssueDraft() (Model, tea.Cmd) {
	summary := strings.TrimSpace(m.createSummaryDraft)
	if summary == "" {
		m.detailNotice = "Summary cannot be empty."
		return m, nil
	}
	if strings.TrimSpace(m.createProjectKey) == "" || strings.TrimSpace(m.createIssueType.ID) == "" {
		m.detailNotice = "Create ticket failed: missing project or issue type."
		return m, nil
	}
	m.nextRequestID++
	m.activeCreateIssueReqID = m.nextRequestID
	m.createSubmitting = true
	m.createSubmitSummary = summary
	m.createSubmitDescription = strings.TrimSpace(m.createDescriptionDraft)
	fields, err := m.createIssueFieldValues()
	if err != nil {
		m.createSubmitting = false
		m.detailNotice = err.Error()
		return m, nil
	}
	m.createSubmitFields = fields
	m.detailNotice = ""
	return m, m.submitCreateIssue(m.activeCreateIssueReqID, worker.CreateIssueRequest{
		ProjectKey:  m.createProjectKey,
		IssueTypeID: m.createIssueType.ID,
		Summary:     summary,
		Description: m.createSubmitDescription,
		Fields:      fields,
	})
}

func (m Model) startCreateAIPrompt() (Model, tea.Cmd) {
	if !m.claudeCreateTicketDraftEnabled() {
		m.detailNotice = "Claude ticket draft generation is not enabled."
		return m, nil
	}
	if !m.claudeCreateTicketDraftAvailable() {
		m.detailNotice = "Claude ticket draft generation is currently unavailable."
		return m, nil
	}
	if !m.createFormReady() {
		m.detailNotice = "Select an issue type before generating a ticket draft."
		return m, nil
	}
	m.createAIPromptOpen = true
	m.createAIPromptLoading = false
	m.createAIPromptErr = nil
	m.createAIPromptProgress = nil
	m.createAIPrompt = strings.TrimSpace(m.createAIPrompt)
	m.createAIPromptEditor = newCreateAIPromptEditor(m.createAIPrompt)
	m.createAIPromptEditorReady = true
	m.detailNotice = ""
	return m, nil
}

func (m Model) updateCreateAIPrompt(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.createAIPromptLoading {
		if msg.String() == "esc" {
			return m.cancelCreateAIPrompt(), nil
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.createAIPromptOpen = false
		m.createAIPrompt = strings.TrimSpace(m.createAIPromptEditorValue())
		return m, nil
	case "ctrl+s":
		return m.submitCreateAIPrompt()
	}
	m.configureCreateAIPromptEditor(max(32, m.browserLayout(m.width).contentWidth-16), 8)
	editor, cmd := m.createAIPromptEditor.Update(msg)
	m.createAIPromptEditor = editor
	m.createAIPrompt = m.createAIPromptEditor.Value()
	return m, cmd
}

func (m Model) submitCreateAIPrompt() (Model, tea.Cmd) {
	if !m.claudeCreateTicketDraftEnabled() {
		m.detailNotice = "Claude ticket draft generation is not enabled."
		return m, nil
	}
	if !m.claudeCreateTicketDraftAvailable() {
		m.detailNotice = "Claude ticket draft generation is currently unavailable."
		return m, nil
	}
	request := strings.TrimSpace(m.createAIPrompt)
	if request == "" {
		request = "Draft a ticket from the selected project."
		if strings.TrimSpace(m.createIssueType.ID) != "" {
			request = "Draft a ticket from the selected project and issue type."
		}
		m.createAIPrompt = request
	}
	m.nextRequestID++
	m.activeCreateAIPromptReqID = m.nextRequestID
	m.createAIPromptLoading = true
	m.createAIPromptStartedAt = m.claudeNow()
	m.createAIPromptErr = nil
	m.createAIPromptProgress = nil
	m.createAIPromptEvents = make(chan claude.Event, 16)
	m.createAIPromptEditor = newCreateAIPromptEditor(request)
	m.createAIPromptEditorReady = true
	runCtx, cancel := context.WithCancel(context.Background())
	m.createAIPromptCancel = cancel
	m.recordDiagnosticEvent(diagnosticKindClaude, "create_ticket_draft", "submit", workerDiagnosticDetail(m.activeCreateAIPromptReqID, m.createProjectKey, nil))
	return m, tea.Batch(
		m.submitCreatePrompt(
			runCtx,
			m.activeCreateAIPromptReqID,
			m.buildCreateIssueDraftPrompt(request),
			m.createAIPromptEvents,
		),
		m.waitForCreateAIPromptProgress(m.activeCreateAIPromptReqID),
		m.scheduleCreateAIPromptTick(m.activeCreateAIPromptReqID),
	)
}

func (m Model) submitCreatePrompt(ctx context.Context, reqID int, prompt string, events chan<- claude.Event) tea.Cmd {
	return m.submitClaudeRequest(ctx, reqID, m.createProjectKey, prompt, events, func(id int, _ string, text string, err error) tea.Msg {
		return createAIPromptResultMsg{id: id, text: text, err: err}
	})
}

func (m Model) buildCreateIssueDraftPrompt(request string) string {
	var b strings.Builder
	if strings.TrimSpace(m.createIssueType.ID) == "" {
		b.WriteString("Draft a new Jira ticket for this project. The user has not selected the Jira issue type yet.\n")
	} else {
		b.WriteString("Draft a new Jira ticket for this project and issue type.\n")
	}
	b.WriteString("Return plain text in this exact format so it can be parsed:\n")
	b.WriteString("Issue Type: <one of the Available Jira Issue Types, or Unknown if not enough context>\n")
	b.WriteString("Summary: <one concise summary>\n")
	b.WriteString("Description: <full ticket description text>\n")
	if components := createComponentsField(m.createFields); len(components.AllowedValues) > 0 {
		b.WriteString("Components: <one of the Available Components, or Unknown if not enough context>\n")
	}
	b.WriteString("Do not edit files, create branches, run git commands, call Jira, or make external changes.\n")
	b.WriteString("Do not mention assumptions without flagging them as Open Questions.\n")
	b.WriteString("Focus on creating a clear, ready-to-use ticket. If scope is unclear, include questions in the Description under Open Questions.\n\n")
	b.WriteString("Project: ")
	b.WriteString(displayValue(m.createProjectKey, "Unknown"))
	b.WriteString("\nIssue Type: ")
	if strings.TrimSpace(m.createIssueType.ID) == "" {
		b.WriteString("Not selected yet")
	} else {
		b.WriteString(displayValue(m.createIssueType.Name, m.createIssueType.ID))
	}
	if len(m.createIssueTypes) > 0 {
		b.WriteString("\n\nAvailable Jira Issue Types:\n")
		for _, issueType := range m.createIssueTypes {
			name := strings.TrimSpace(displayValue(issueType.Name, issueType.ID))
			if name == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(name)
			b.WriteString("\n")
		}
	}
	if components := createComponentsField(m.createFields); len(components.AllowedValues) > 0 {
		b.WriteString("\n\nAvailable Components:\n")
		for _, option := range components.AllowedValues {
			name := strings.TrimSpace(displayValue(option.Name, option.ID))
			if name == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(name)
			b.WriteString("\n")
		}
	}
	if current := strings.TrimSpace(m.createIssueAICurrentDraft()); current != "" {
		b.WriteString("\n\nCurrent draft:\n")
		b.WriteString(current)
	}
	if feedback := strings.TrimSpace(m.createAIQuestionFeedback()); feedback != "" {
		b.WriteString("\n\n")
		b.WriteString(feedback)
	}
	b.WriteString("\n\nUser request:\n")
	b.WriteString(strings.TrimSpace(request))
	return strings.TrimSpace(b.String())
}

func (m Model) createIssueAICurrentDraft() string {
	var parts []string
	if summary := strings.TrimSpace(m.createSummaryDraft); summary != "" {
		parts = append(parts, "Summary: "+summary)
	}
	if description := strings.TrimSpace(m.createDescriptionDraft); description != "" {
		parts = append(parts, "Description:\n"+description)
	}
	return strings.Join(parts, "\n\n")
}

func (m Model) createAIQuestionFeedback() string {
	if len(m.createAIQuestions) == 0 {
		return ""
	}
	var answered []string
	var unanswered []string
	for _, question := range m.createAIQuestions {
		q := strings.TrimSpace(question.Question)
		if q == "" {
			continue
		}
		answer := strings.TrimSpace(question.Answer)
		if answer == "" {
			unanswered = append(unanswered, "- "+q)
			continue
		}
		answered = append(answered, "Q: "+q+"\nA: "+answer)
	}
	var parts []string
	if len(answered) > 0 {
		parts = append(parts, "User answers to Open Questions:\n"+strings.Join(answered, "\n\n"))
	}
	if len(unanswered) > 0 {
		parts = append(parts, "Still unanswered Open Questions:\n"+strings.Join(unanswered, "\n"))
	}
	return strings.Join(parts, "\n\n")
}

func (m Model) waitForCreateAIPromptProgress(reqID int) tea.Cmd {
	events := m.createAIPromptEvents
	return waitForClaudeProgress(events, reqID, "", func(id int, _ string, event claude.Event) tea.Msg {
		return createAIPromptProgressMsg{id: id, event: event}
	})
}

func (m Model) scheduleCreateAIPromptTick(reqID int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return createAIPromptTickMsg{id: reqID}
	})
}

func (m Model) cancelCreateAIPrompt() Model {
	if m.createAIPromptCancel != nil {
		m.createAIPromptCancel()
	}
	reqID := m.activeCreateAIPromptReqID
	m.createAIPromptCancel = nil
	m.createAIPromptEvents = nil
	m.createAIPromptLoading = false
	m.createAIPromptErr = errors.New("Claude ticket draft generation cancelled")
	m.createAIPromptOpen = false
	m.recordDiagnosticEvent(diagnosticKindClaude, "create_ticket_draft", "cancel", workerDiagnosticDetail(reqID, m.createProjectKey, m.createAIPromptErr))
	return m
}

func (m *Model) createAIPromptEditorValue() string {
	if m.createAIPromptEditorReady {
		return m.createAIPromptEditor.Value()
	}
	return m.createAIPrompt
}

func newCreateAIPromptEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Describe the ticket you want Claude to draft."
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func newCreateQuestionAnswerEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Answer this question for Claude."
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func (m *Model) ensureCreateQuestionAnswerEditor() {
	if m.createAIQuestionEditorReady {
		return
	}
	value := ""
	if len(m.createAIQuestions) > 0 {
		selected := clamp(m.selectedCreateAIQuestion, 0, len(m.createAIQuestions)-1)
		value = m.createAIQuestions[selected].Answer
	}
	m.createAIQuestionEditor = newCreateQuestionAnswerEditor(value)
	m.createAIQuestionEditorReady = true
}

func (m Model) configuredCreateQuestionAnswerEditor(width int, rows int) textarea.Model {
	editor := m.createAIQuestionEditor
	if !m.createAIQuestionEditorReady {
		value := ""
		if len(m.createAIQuestions) > 0 {
			selected := clamp(m.selectedCreateAIQuestion, 0, len(m.createAIQuestions)-1)
			value = m.createAIQuestions[selected].Answer
		}
		editor = newCreateQuestionAnswerEditor(value)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	editor.Focus()
	return editor
}

func (m *Model) ensureCreateAIPromptEditor() {
	if m.createAIPromptEditorReady {
		return
	}
	m.createAIPromptEditor = newCreateAIPromptEditor(m.createAIPrompt)
	m.createAIPromptEditorReady = true
}

func (m *Model) configureCreateAIPromptEditor(width int, rows int) {
	m.ensureCreateAIPromptEditor()
	m.createAIPromptEditor.MaxHeight = max(rows, 1)
	m.createAIPromptEditor.MaxWidth = width
	m.createAIPromptEditor.SetWidth(width)
	m.createAIPromptEditor.SetHeight(rows)
	m.createAIPromptEditor.Focus()
}

func (m Model) configuredCreateAIPromptEditor(width int, rows int) textarea.Model {
	editor := m.createAIPromptEditor
	if !m.createAIPromptEditorReady {
		editor = newCreateAIPromptEditor(m.createAIPrompt)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	editor.Focus()
	return editor
}

func (m Model) handleCreateAIPromptProgress(msg createAIPromptProgressMsg) Model {
	if msg.id != m.activeCreateAIPromptReqID {
		return m
	}
	if strings.TrimSpace(msg.event.Text) == "" {
		return m
	}
	m.createAIPromptProgress = append(m.createAIPromptProgress, msg.event)
	if len(m.createAIPromptProgress) > 6 {
		m.createAIPromptProgress = append([]claude.Event(nil), m.createAIPromptProgress[len(m.createAIPromptProgress)-6:]...)
	}
	m.recordDiagnosticEvent(diagnosticKindClaude, "create_ticket_draft", "progress", truncate(msg.event.Kind+" "+msg.event.Text, 100))
	return m
}

func (m Model) handleCreateAIPromptResult(msg createAIPromptResultMsg) (Model, tea.Cmd) {
	status := "ok"
	if msg.err != nil {
		status = "error"
		if errors.Is(msg.err, context.Canceled) {
			status = "cancel"
		} else if errors.Is(msg.err, context.DeadlineExceeded) {
			status = "timeout"
		}
	}
	m.recordDiagnosticEvent(diagnosticKindClaude, "create_ticket_draft", status, workerDiagnosticDetail(msg.id, m.createProjectKey, msg.err))
	if msg.id != m.activeCreateAIPromptReqID {
		return m, nil
	}
	m.createAIPromptLoading = false
	m.createAIPromptCancel = nil
	m.createAIPromptEvents = nil
	m.createAIPromptErr = msg.err
	m.createAIPromptOpen = true
	if msg.err != nil {
		return m, nil
	}
	summary, description := parseCreateIssueDraft(msg.text)
	if strings.TrimSpace(summary) == "" {
		m.createAIPromptErr = errors.New("Claude draft is missing a summary")
		m.createAIPromptOpen = true
		return m, nil
	}
	m.createAIFieldDrafts = parseCreateIssueDraftFields(msg.text)
	m.createAIQuestions = mergeCreateAIQuestionAnswers(parseCreateIssueOpenQuestions(msg.text), m.createAIQuestions)
	m.selectedCreateAIQuestion = clamp(m.selectedCreateAIQuestion, 0, max(0, len(m.createAIQuestions)-1))
	m.createAIQuestionAnswering = false
	m.createAIQuestionEditorReady = false
	m.createSummaryDraft = summary
	m.createDescriptionDraft = description
	m.createSummaryEditor = newSummaryEditor(summary)
	m.createSummaryEditorReady = true
	m.createDescriptionEditor = newCommentEditor(description)
	m.createDescriptionEditorReady = true
	m.applyCreateAIFieldDrafts()
	m.createFieldFocus = createSummaryFieldIndex
	m.createAIGeneratedMode = false
	m.createAIPromptOpen = false
	m.createAIPrompt = ""
	m.createAIPromptErr = nil
	m.createAIPromptProgress = nil
	if strings.TrimSpace(m.createIssueType.ID) == "" {
		if issueType, ok := m.createAIRecommendedIssueType(); ok {
			m.selectedCreateIssueType = issueType
			selectedName := displayValue(m.createIssueTypes[issueType].Name, m.createIssueTypes[issueType].ID)
			var cmd tea.Cmd
			m, cmd = m.selectCreateIssueType()
			m.detailNotice = "Applied Claude ticket draft and selected " + selectedName + "."
			return m, cmd
		}
		m.detailNotice = "Applied Claude ticket draft. Select an issue type to continue."
	} else {
		m.detailNotice = "Applied Claude ticket draft."
	}
	return m, nil
}

func (m Model) createAIRecommendedIssueType() (int, bool) {
	value := strings.TrimSpace(m.createAIFieldDrafts["issuetype"])
	if value == "" || strings.EqualFold(value, "unknown") || strings.EqualFold(value, "not selected yet") {
		return 0, false
	}
	for index, issueType := range m.createIssueTypes {
		if createIssueTypeMatches(issueType, value) {
			return index, true
		}
	}
	return 0, false
}

func mergeCreateAIQuestionAnswers(next []createAIQuestion, existing []createAIQuestion) []createAIQuestion {
	if len(next) == 0 {
		return nil
	}
	answers := map[string]string{}
	for _, question := range existing {
		key := normalizeCreateDraftFieldName(question.Question)
		if key != "" && strings.TrimSpace(question.Answer) != "" {
			answers[key] = question.Answer
		}
	}
	for index := range next {
		if answer := strings.TrimSpace(answers[normalizeCreateDraftFieldName(next[index].Question)]); answer != "" {
			next[index].Answer = answer
		}
	}
	return next
}

func (m *Model) resetCreateIssueState() {
	m.createOpen = false
	m.createProjectKey = ""
	m.createIssueTypes = nil
	m.selectedCreateIssueType = 0
	m.createIssueTypesLoading = false
	m.createIssueTypesErr = nil
	m.createFields = nil
	m.createFieldsLoading = false
	m.createFieldsErr = nil
	m.createIssueType = jira.CreateIssueType{}
	m.createChangingType = false
	m.createAIGeneratedMode = false
	m.createFieldFocus = createSummaryFieldIndex
	m.createSummaryDraft = ""
	m.createDescriptionDraft = ""
	m.createSummaryEditor = newSummaryEditor("")
	m.createSummaryEditorReady = true
	m.createDescriptionEditor = newCommentEditor("")
	m.createDescriptionEditorReady = true
	m.createSubmitting = false
	m.createSubmitSummary = ""
	m.createSubmitDescription = ""
	m.createDynamicValues = nil
	m.createDynamicSelections = nil
	m.createDynamicFilters = nil
	m.createSubmitFields = nil
	m.createAIPromptOpen = false
	m.createAIPrompt = ""
	m.createAIPromptEditor = textarea.Model{}
	m.createAIPromptEditorReady = false
	m.createAIPromptErr = nil
	m.createAIPromptLoading = false
	m.createAIPromptProgress = nil
	m.createAIPromptStartedAt = time.Time{}
	m.createAIPromptCancel = nil
	m.createAIPromptEvents = nil
	m.createAIPrompt = ""
	m.createAIFieldDrafts = nil
	m.createAIQuestions = nil
	m.selectedCreateAIQuestion = 0
	m.createAIQuestionAnswering = false
	m.createAIQuestionEditor = textarea.Model{}
	m.createAIQuestionEditorReady = false
}

func (m Model) startSummaryEditor() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.summaryFocus = true
	if metadata, ok := m.editMetadata[selected.Key]; ok {
		if !metadata.Summary.Editable {
			m.detailNotice = "Summary is not editable for " + selected.Key + "."
			return m, nil
		}
		m.beginSummaryEditing()
		return m, nil
	}
	if m.summaryMetadataLoading && m.summaryMetadataRequestKey == selected.Key {
		return m, nil
	}
	m.nextRequestID++
	m.activeSummaryMetadataReqID = m.nextRequestID
	m.summaryMetadataRequestKey = selected.Key
	m.summaryMetadataLoading = true
	m.summaryMetadataErr = nil
	m.detailNotice = ""
	return m, m.submitEditMetadata(m.activeSummaryMetadataReqID, selected.Key)
}

func (m *Model) beginSummaryEditing() {
	selected, ok := m.selectedIssue()
	if !ok {
		return
	}
	m.summaryFocus = true
	m.summaryEditing = true
	m.summaryDraft = selected.Summary
	m.summaryDirty = false
	if detail, ok := m.details[selected.Key]; ok && strings.TrimSpace(detail.Summary) != "" {
		m.summaryDraft = detail.Summary
	}
	m.summaryEditor = newSummaryEditor(m.summaryDraft)
	m.summaryEditorReady = true
	m.detailNotice = ""
}

func (m Model) updateSummaryEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.summaryEditing = false
		m.summaryDraft = ""
		m.summaryDirty = false
		m.summaryEditor = newSummaryEditor("")
		m.summaryEditorReady = true
		m.detailNotice = ""
		return m, nil
	case "enter":
		m.summaryDraft = m.summaryEditorValue()
		return m.submitSummaryDraft()
	}
	m.ensureSummaryEditor()
	m.configureSummaryEditor()
	before := m.summaryEditor.Value()
	editor, cmd := m.summaryEditor.Update(msg)
	m.summaryEditor = editor
	m.summaryDraft = m.summaryEditor.Value()
	if m.summaryDraft != before {
		m.summaryDirty = true
		m.detailNotice = ""
	}
	return m, cmd
}

func (m Model) submitSummaryDraft() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	summary := strings.TrimSpace(m.summaryDraft)
	if summary == "" {
		m.detailNotice = "Summary cannot be empty."
		return m, nil
	}
	if !m.summaryDirty {
		m.detailNotice = "Edit summary before saving."
		return m, nil
	}
	current := strings.TrimSpace(selected.Summary)
	if detail, ok := m.details[selected.Key]; ok && strings.TrimSpace(detail.Summary) != "" {
		current = strings.TrimSpace(detail.Summary)
	}
	if summary == current {
		m.detailNotice = "Summary unchanged."
		return m, nil
	}
	m.nextRequestID++
	m.activeSummaryReqID = m.nextRequestID
	m.summarySubmitting = true
	m.summarySubmitKey = selected.Key
	m.summarySubmitValue = summary
	m.detailNotice = "Updating summary for " + selected.Key + "."
	return m, m.submitUpdateSummary(m.activeSummaryReqID, selected.Key, summary)
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

func (m Model) canUseLinkSelection() bool {
	if m.mode != modeDetail {
		return false
	}
	section, ok := m.focusedDetailSection()
	if !ok || section.ID != "links" {
		return false
	}
	return len(m.currentDetailLinks()) > 0
}

func (m Model) canUseActionSelection() bool {
	if m.mode != modeDetail {
		return false
	}
	section, ok := m.focusedDetailSection()
	return ok && section.ID == "actions"
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
	contentWidth := max(1, blockWidth-2)
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = truncate(line, contentWidth)
		padded := line + strings.Repeat(" ", contentWidth-len(line))
		rendered = append(rendered, m.theme.CodeBlock.Width(contentWidth).Render(padded))
	}
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
	target, ok := m.focusedDetailTarget()
	if !ok || target.Kind != detailTargetSection {
		return detailSection{}, false
	}
	return target.Section, true
}

func (m Model) focusedDetailTarget() (detailTarget, bool) {
	targets := m.detailTargets()
	if len(targets) == 0 {
		return detailTarget{}, false
	}
	return targets[clamp(m.detailFocus, 0, len(targets)-1)], true
}

func (m Model) focusedDetailTargetID() string {
	target, ok := m.focusedDetailTarget()
	if !ok {
		return ""
	}
	return target.ID
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

func markdownTablesToRichMarkers(value string) string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	result := make([]string, 0, len(lines))
	for index := 0; index < len(lines); {
		if index+1 < len(lines) && isMarkdownTableRow(lines[index]) && isTableSeparator(lines[index+1]) {
			var tableLines []string
			for index < len(lines) && isMarkdownTableRow(lines[index]) {
				tableLines = append(tableLines, strings.TrimSpace(lines[index]))
				index++
			}
			result = append(result, "[table]")
			result = append(result, tableLines...)
			result = append(result, "[/table]")
			continue
		}
		result = append(result, lines[index])
		index++
	}
	return strings.Join(result, "\n")
}

func isMarkdownTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") && strings.Count(trimmed, "|") >= 2
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

func newSummaryEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Edit summary..."
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func newClaudeAssistEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Edit Claude ticket draft..."
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func newClaudeAssistRefineEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Tell Claude how to refine this draft..."
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func (m *Model) ensureSummaryEditor() {
	if m.summaryEditorReady {
		return
	}
	m.summaryEditor = newSummaryEditor(m.summaryDraft)
	m.summaryEditorReady = true
}

func (m *Model) configureSummaryEditor() {
	m.ensureSummaryEditor()
	width := max(32, m.browserLayout(m.width).contentWidth-16)
	m.summaryEditor.MaxHeight = 3
	m.summaryEditor.MaxWidth = width
	m.summaryEditor.SetWidth(width)
	m.summaryEditor.SetHeight(3)
}

func (m *Model) ensureCreateSummaryEditor() {
	if m.createSummaryEditorReady {
		return
	}
	m.createSummaryEditor = newSummaryEditor(m.createSummaryDraft)
	m.createSummaryEditorReady = true
}

func (m *Model) configureCreateSummaryEditor() {
	m.ensureCreateSummaryEditor()
	width := max(32, m.browserLayout(m.width).contentWidth-16)
	m.createSummaryEditor.MaxHeight = 3
	m.createSummaryEditor.MaxWidth = width
	m.createSummaryEditor.SetWidth(width)
	m.createSummaryEditor.SetHeight(3)
}

func (m Model) configuredCreateSummaryEditor(width int, rows int) textarea.Model {
	editor := m.createSummaryEditor
	if !m.createSummaryEditorReady {
		editor = newSummaryEditor(m.createSummaryDraft)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	if !m.createSubmitting && m.createFieldFocus == createSummaryFieldIndex {
		editor.Focus()
	} else {
		editor.Blur()
	}
	return editor
}

func (m *Model) ensureCreateDescriptionEditor() {
	if m.createDescriptionEditorReady {
		return
	}
	m.createDescriptionEditor = newCommentEditor(m.createDescriptionDraft)
	m.createDescriptionEditorReady = true
}

func (m *Model) configureCreateDescriptionEditor() {
	m.ensureCreateDescriptionEditor()
	width := max(32, m.browserLayout(m.width).contentWidth-16)
	m.createDescriptionEditor.MaxHeight = 6
	m.createDescriptionEditor.MaxWidth = width
	m.createDescriptionEditor.SetWidth(width)
	m.createDescriptionEditor.SetHeight(6)
}

func (m Model) configuredCreateDescriptionEditor(width int, rows int) textarea.Model {
	editor := m.createDescriptionEditor
	if !m.createDescriptionEditorReady {
		editor = newCommentEditor(m.createDescriptionDraft)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	if !m.createSubmitting && m.createFieldFocus == createDescriptionFieldIndex {
		editor.Focus()
	} else {
		editor.Blur()
	}
	return editor
}

func (m Model) configuredSummaryEditor(width int, rows int) textarea.Model {
	editor := m.summaryEditor
	if !m.summaryEditorReady {
		editor = newSummaryEditor(m.summaryDraft)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	if !m.summarySubmitting {
		editor.Focus()
	} else {
		editor.Blur()
	}
	return editor
}

func (m *Model) ensureClaudeAssistEditor() {
	if m.claudeAssistEditorReady {
		return
	}
	m.claudeAssistEditor = newClaudeAssistEditor(m.claudeAssistDraft)
	m.claudeAssistEditorReady = true
}

func (m *Model) configureClaudeAssistEditor(width int, rows int) {
	m.ensureClaudeAssistEditor()
	m.claudeAssistEditor.MaxHeight = max(rows, 1)
	m.claudeAssistEditor.MaxWidth = width
	m.claudeAssistEditor.SetWidth(width)
	m.claudeAssistEditor.SetHeight(rows)
	m.claudeAssistEditor.Focus()
}

func (m Model) configuredClaudeAssistEditor(width int, rows int) textarea.Model {
	editor := m.claudeAssistEditor
	if !m.claudeAssistEditorReady {
		editor = newClaudeAssistEditor(m.claudeAssistDraft)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	editor.Focus()
	return editor
}

func (m *Model) ensureClaudeAssistRefineEditor() {
	if m.claudeAssistRefineEditorReady {
		return
	}
	m.claudeAssistRefineEditor = newClaudeAssistRefineEditor(m.claudeAssistRefineInstruction)
	m.claudeAssistRefineEditorReady = true
}

func (m *Model) configureClaudeAssistRefineEditor(width int, rows int) {
	m.ensureClaudeAssistRefineEditor()
	m.claudeAssistRefineEditor.MaxHeight = max(rows, 1)
	m.claudeAssistRefineEditor.MaxWidth = width
	m.claudeAssistRefineEditor.SetWidth(width)
	m.claudeAssistRefineEditor.SetHeight(rows)
	m.claudeAssistRefineEditor.Focus()
}

func (m Model) configuredClaudeAssistRefineEditor(width int, rows int) textarea.Model {
	editor := m.claudeAssistRefineEditor
	if !m.claudeAssistRefineEditorReady {
		editor = newClaudeAssistRefineEditor(m.claudeAssistRefineInstruction)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	editor.Focus()
	return editor
}

func (m *Model) ensureInlineAIInstructionEditor() {
	if m.inlineAIInstructionReady {
		return
	}
	m.inlineAIInstructionEditor = newClaudeAssistRefineEditor(m.inlineAIInstruction)
	m.inlineAIInstructionReady = true
}

func (m *Model) configureInlineAIInstructionEditor(width int, rows int) {
	m.ensureInlineAIInstructionEditor()
	m.inlineAIInstructionEditor.MaxHeight = max(rows, 1)
	m.inlineAIInstructionEditor.MaxWidth = width
	m.inlineAIInstructionEditor.SetWidth(width)
	m.inlineAIInstructionEditor.SetHeight(rows)
	m.inlineAIInstructionEditor.Focus()
}

func (m Model) configuredInlineAIInstructionEditor(width int, rows int) textarea.Model {
	editor := m.inlineAIInstructionEditor
	if !m.inlineAIInstructionReady {
		editor = newClaudeAssistRefineEditor(m.inlineAIInstruction)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	editor.Focus()
	return editor
}

func (m Model) claudeAssistDraftValue() string {
	if !m.claudeAssistEditorReady {
		return m.claudeAssistDraft
	}
	return m.claudeAssistEditor.Value()
}

func (m Model) claudeAssistRefineInstructionValue() string {
	if !m.claudeAssistRefineEditorReady {
		return m.claudeAssistRefineInstruction
	}
	return m.claudeAssistRefineEditor.Value()
}

func (m Model) inlineAIInstructionValue() string {
	if !m.inlineAIInstructionReady {
		return m.inlineAIInstruction
	}
	return m.inlineAIInstructionEditor.Value()
}

func (m Model) updateClaudeAssistEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.claudeAssistPostingComment {
		if msg.String() == "esc" {
			m.claudeAssistOpen = false
		}
		return m, nil
	}
	if m.claudeAssistApplying {
		if msg.String() == "esc" {
			m.claudeAssistOpen = false
		}
		return m, nil
	}
	if m.claudeAssistConfirmComment {
		switch msg.String() {
		case "esc":
			m.claudeAssistConfirmComment = false
			return m, nil
		case "ctrl+s":
			return m.submitClaudeAssistComment()
		}
		return m, nil
	}
	if m.claudeAssistRefining {
		switch msg.String() {
		case "esc":
			m.claudeAssistRefineInstruction = m.claudeAssistRefineInstructionValue()
			m.claudeAssistRefining = false
			return m, nil
		case "ctrl+s":
			return m.submitClaudeAssistRefinement()
		}
		m.configureClaudeAssistRefineEditor(max(32, m.browserLayout(m.width).contentWidth-16), 4)
		editor, cmd := m.claudeAssistRefineEditor.Update(msg)
		m.claudeAssistRefineEditor = editor
		m.claudeAssistRefineInstruction = m.claudeAssistRefineEditor.Value()
		return m, cmd
	}
	if m.claudeAssistConfirmApply {
		switch msg.String() {
		case "esc":
			m.claudeAssistConfirmApply = false
			return m, nil
		case "ctrl+s":
			return m.submitClaudeAssistApply()
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.claudeAssistDraft = m.claudeAssistDraftValue()
		m.claudeAssistOpen = false
		return m, nil
	case "r":
		m.claudeAssistDraft = m.claudeAssistDraftValue()
		m.claudeAssistRefineInstruction = ""
		m.claudeAssistRefineEditor = newClaudeAssistRefineEditor("")
		m.claudeAssistRefineEditorReady = true
		m.claudeAssistRefining = true
		m.detailNotice = ""
		return m, nil
	case "c":
		m.claudeAssistDraft = m.claudeAssistDraftValue()
		if strings.TrimSpace(m.claudeAssistDraft) == "" {
			m.detailNotice = "No Ticket Assist draft to post as a comment."
			return m, nil
		}
		if selected, ok := m.selectedIssue(); ok {
			m.claudeAssistKey = selected.Key
		}
		m.claudeAssistConfirmComment = true
		m.detailNotice = ""
		return m, nil
	case "ctrl+s":
		return m.beginClaudeAssistApply()
	case "ctrl+y":
		return m.copyClaudeAssistDraft()
	}
	m.configureClaudeAssistEditor(max(32, m.browserLayout(m.width).contentWidth-16), m.claudeAssistEditorRows())
	editor, cmd := m.claudeAssistEditor.Update(msg)
	m.claudeAssistEditor = editor
	m.claudeAssistDraft = m.claudeAssistEditor.Value()
	return m, cmd
}

func (m Model) updateInlineAIPicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.inlineAIInstructionOpen {
		switch msg.String() {
		case "esc":
			m.inlineAIInstruction = m.inlineAIInstructionValue()
			m.inlineAIInstructionOpen = false
			return m, nil
		case "ctrl+s":
			actions := inlineDescriptionAIActions()
			action := actions[clamp(m.selectedInlineAIAction, 0, len(actions)-1)]
			return m.submitInlineDescriptionAI(action, m.inlineAIInstructionValue())
		}
		m.configureInlineAIInstructionEditor(max(32, m.browserLayout(m.width).contentWidth-16), 4)
		editor, cmd := m.inlineAIInstructionEditor.Update(msg)
		m.inlineAIInstructionEditor = editor
		m.inlineAIInstruction = m.inlineAIInstructionEditor.Value()
		return m, cmd
	}
	actions := inlineDescriptionAIActions()
	switch msg.String() {
	case "esc":
		m.inlineAIOpen = false
		return m, nil
	case "j", "down":
		m.selectedInlineAIAction = clamp(m.selectedInlineAIAction+1, 0, len(actions)-1)
		return m, nil
	case "k", "up":
		m.selectedInlineAIAction = clamp(m.selectedInlineAIAction-1, 0, len(actions)-1)
		return m, nil
	case "enter":
		return m.runSelectedInlineAIAction()
	default:
		return m, nil
	}
}

func (m Model) runSelectedInlineAIAction() (Model, tea.Cmd) {
	actions := inlineDescriptionAIActions()
	action := actions[clamp(m.selectedInlineAIAction, 0, len(actions)-1)]
	if action.ID == "ask_question" {
		m.inlineAIInstructionOpen = true
		m.inlineAIInstruction = ""
		m.inlineAIInstructionEditor = newClaudeAssistRefineEditor("")
		m.inlineAIInstructionReady = true
		return m, nil
	}
	return m.submitInlineDescriptionAI(action, "")
}

func (m Model) submitInlineDescriptionAI(action inlineAIAction, instruction string) (Model, tea.Cmd) {
	ctx, ok := m.detailRenderContext()
	if !ok {
		m.detailNotice = "No selected ticket for inline AI."
		return m, nil
	}
	if strings.TrimSpace(ctx.description) == "" && strings.TrimSpace(ctx.detail.Description) == "" {
		m.detailNotice = "Description is not loaded yet."
		return m, nil
	}
	key := ctx.display.Key
	if key == "" {
		key = ctx.selected.Key
	}
	m.nextRequestID++
	reqID := m.nextRequestID
	m.activeClaudeAssistReqID = reqID
	m.claudeAssistKey = key
	m.claudeAssistText = ""
	m.claudeAssistErr = nil
	m.claudeAssistLoading = true
	m.claudeAssistOpen = true
	m.claudeAssistStartedAt = m.claudeNow()
	m.claudeAssistProgress = nil
	m.claudeAssistDraft = ""
	m.claudeAssistEditor = newClaudeAssistEditor("")
	m.claudeAssistEditorReady = true
	m.claudeAssistTarget = claudeAssistTargetDescription
	m.inlineAIOpen = false
	m.inlineAIInstructionOpen = false
	m.claudeAssistEvents = make(chan claude.Event, 16)
	runCtx, cancel := context.WithCancel(context.Background())
	m.claudeAssistCancel = cancel
	m.detailNotice = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "inline_description_ai", "submit", workerDiagnosticDetail(reqID, key, nil))
	return m, tea.Batch(
		m.submitClaudeTicketAssist(runCtx, reqID, key, m.buildInlineDescriptionAIPrompt(ctx, action, instruction), m.claudeAssistEvents),
		m.waitForClaudeAssistProgress(reqID, key),
		m.scheduleClaudeAssistTick(reqID),
	)
}

func (m Model) beginClaudeAssistApply() (Model, tea.Cmd) {
	m.claudeAssistDraft = m.claudeAssistDraftValue()
	if !m.claudeConfig.AllowJiraWrites {
		m.detailNotice = "Jira writes are disabled for Claude Ticket Assist. Use ctrl+y to copy the draft."
		return m, nil
	}
	selected, ok := m.selectedIssue()
	if !ok {
		m.detailNotice = "No selected ticket to update."
		return m, nil
	}
	if m.claudeAssistTarget == claudeAssistTargetDescription {
		description := strings.TrimSpace(m.claudeAssistDraft)
		if description == "" {
			m.detailNotice = "Claude description draft has no text to apply."
			return m, nil
		}
		m.claudeAssistKey = selected.Key
		m.claudeAssistApplySummary = ""
		m.claudeAssistApplyDescription = description
		m.claudeAssistConfirmApply = m.claudeConfig.RequireConfirmation
		if m.claudeAssistConfirmApply {
			return m, nil
		}
		return m.submitClaudeAssistApply()
	}
	draft := parseClaudeAssistApplyDraft(m.claudeAssistDraft, selected.Summary)
	if strings.TrimSpace(draft.Description) == "" {
		m.detailNotice = "Claude ticket draft has no description to apply."
		return m, nil
	}
	m.claudeAssistKey = selected.Key
	m.claudeAssistApplySummary = draft.Summary
	m.claudeAssistApplyDescription = draft.Description
	m.claudeAssistConfirmApply = m.claudeConfig.RequireConfirmation
	if m.claudeAssistConfirmApply {
		return m, nil
	}
	return m.submitClaudeAssistApply()
}

func (m Model) submitClaudeAssistApply() (Model, tea.Cmd) {
	if !m.claudeConfig.AllowJiraWrites {
		m.detailNotice = "Jira writes are disabled for Claude Ticket Assist. Use ctrl+y to copy the draft."
		return m, nil
	}
	key := strings.TrimSpace(m.claudeAssistKey)
	if key == "" {
		if selected, ok := m.selectedIssue(); ok {
			key = selected.Key
		}
	}
	if key == "" {
		m.detailNotice = "No selected ticket to update."
		return m, nil
	}
	summary := strings.TrimSpace(m.claudeAssistApplySummary)
	description := strings.TrimSpace(m.claudeAssistApplyDescription)
	if m.claudeAssistTarget == claudeAssistTargetDescription {
		if description == "" {
			m.detailNotice = "Claude description draft needs text before applying."
			return m, nil
		}
		m.nextRequestID++
		m.activeClaudeAssistDescriptionReqID = m.nextRequestID
		m.activeClaudeAssistSummaryReqID = 0
		m.claudeAssistApplying = true
		m.claudeAssistConfirmApply = false
		m.claudeAssistSummaryApplied = true
		m.claudeAssistDescriptionApplied = false
		m.detailNotice = ""
		return m, m.submitUpdateDescription(m.activeClaudeAssistDescriptionReqID, key, description)
	}
	if summary == "" || description == "" {
		m.detailNotice = "Claude ticket draft needs both summary and description before applying."
		return m, nil
	}
	m.nextRequestID++
	m.activeClaudeAssistSummaryReqID = m.nextRequestID
	m.nextRequestID++
	m.activeClaudeAssistDescriptionReqID = m.nextRequestID
	m.claudeAssistApplying = true
	m.claudeAssistConfirmApply = false
	m.claudeAssistSummaryApplied = false
	m.claudeAssistDescriptionApplied = false
	m.detailNotice = ""
	return m, tea.Batch(
		m.submitUpdateSummary(m.activeClaudeAssistSummaryReqID, key, summary),
		m.submitUpdateDescription(m.activeClaudeAssistDescriptionReqID, key, description),
	)
}

func (m Model) submitClaudeAssistRefinement() (Model, tea.Cmd) {
	ctx, ok := m.detailRenderContext()
	if !ok {
		m.detailNotice = "No selected ticket to refine."
		return m, nil
	}
	instruction := strings.TrimSpace(m.claudeAssistRefineInstructionValue())
	if instruction == "" {
		m.detailNotice = "Write a refinement instruction before sending."
		return m, nil
	}
	currentDraft := strings.TrimSpace(m.claudeAssistDraftValue())
	if currentDraft == "" {
		m.detailNotice = "No Claude ticket draft to refine."
		return m, nil
	}
	key := ctx.display.Key
	if key == "" {
		key = ctx.selected.Key
	}
	m.nextRequestID++
	reqID := m.nextRequestID
	m.activeClaudeAssistReqID = reqID
	m.claudeAssistKey = key
	m.claudeAssistErr = nil
	m.claudeAssistLoading = true
	m.claudeAssistRefining = false
	m.claudeAssistOpen = true
	m.claudeAssistStartedAt = m.claudeNow()
	m.claudeAssistProgress = nil
	m.claudeAssistDraft = currentDraft
	m.claudeAssistEvents = make(chan claude.Event, 16)
	runCtx, cancel := context.WithCancel(context.Background())
	m.claudeAssistCancel = cancel
	m.detailNotice = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "ticket_assist_refine", "submit", workerDiagnosticDetail(reqID, key, nil))
	return m, tea.Batch(
		m.submitClaudeTicketAssist(runCtx, reqID, key, m.buildClaudeTicketAssistRefinementPrompt(ctx, currentDraft, instruction), m.claudeAssistEvents),
		m.waitForClaudeAssistProgress(reqID, key),
		m.scheduleClaudeAssistTick(reqID),
	)
}

func (m Model) submitClaudeAssistComment() (Model, tea.Cmd) {
	key := strings.TrimSpace(m.claudeAssistKey)
	if key == "" {
		if selected, ok := m.selectedIssue(); ok {
			key = selected.Key
		}
	}
	body := strings.TrimSpace(m.claudeAssistDraftValue())
	if key == "" {
		m.detailNotice = "No selected ticket for comment."
		return m, nil
	}
	if body == "" {
		m.detailNotice = "No Ticket Assist draft to post as a comment."
		return m, nil
	}
	m.nextRequestID++
	m.activeClaudeAssistCommentReqID = m.nextRequestID
	m.claudeAssistConfirmComment = false
	m.claudeAssistPostingComment = true
	m.detailNotice = ""
	return m, m.submitAddComment(m.activeClaudeAssistCommentReqID, key, body, nil)
}

type claudeAssistApplyDraft struct {
	Summary     string
	Description string
}

func parseClaudeAssistApplyDraft(draft string, fallbackSummary string) claudeAssistApplyDraft {
	var descriptionLines []string
	summary := strings.TrimSpace(fallbackSummary)
	for _, line := range strings.Split(draft, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "summary:") {
			if value := strings.TrimSpace(strings.TrimPrefix(trimmed, trimmed[:len("summary:")])); value != "" {
				summary = value
			}
			continue
		}
		descriptionLines = append(descriptionLines, line)
	}
	return claudeAssistApplyDraft{
		Summary:     summary,
		Description: strings.TrimSpace(strings.Join(descriptionLines, "\n")),
	}
}

func (m Model) copyClaudeAssistDraft() (Model, tea.Cmd) {
	draft := strings.TrimSpace(m.claudeAssistDraftValue())
	if draft == "" {
		m.detailNotice = "No Claude ticket draft to copy."
		return m, nil
	}
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "copy-draft",
			target: "Claude ticket draft",
			err:    copyToClipboard(draft),
		}
	}
}

func (m Model) summaryEditorValue() string {
	if !m.summaryEditorReady {
		return m.summaryDraft
	}
	return m.summaryEditor.Value()
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
	for index, target := range m.detailTargets() {
		if target.Kind != detailTargetSection {
			continue
		}
		if strings.EqualFold(target.Label, title) || strings.EqualFold(target.ID, title) {
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
	case "copy-draft":
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
		return workSubmittedMsg{kind: worker.KindSearchIssues, id: requestID}
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
		return workSubmittedMsg{kind: worker.KindExpandIssues, id: requestID, key: parentKey}
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
	if _, ok := m.details[selected.Key]; ok && m.isIssueDetailFresh(selected.Key) {
		m.recordDiagnosticEvent(diagnosticKindCache, "issue_detail", "hit", selected.Key)
		m.detailLoading = false
		m.detailErr = nil
		m.detailRequestKey = ""
	} else if !(m.detailLoading && m.detailRequestKey == selected.Key) {
		status := "miss"
		if _, ok := m.details[selected.Key]; ok {
			status = "stale"
		}
		m.recordDiagnosticEvent(diagnosticKindCache, "issue_detail", status, selected.Key)
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
		return workSubmittedMsg{kind: worker.KindGetIssue, id: requestID, key: key}
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
		return workSubmittedMsg{kind: worker.KindGetComments, id: requestID, key: key}
	}
}

func (m Model) submitIssueTransitions(requestID int, key string) tea.Cmd {
	return func() tea.Msg {
		if key == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindGetTransitions,
			Timeout: m.requestTimeout,
			GetTransitions: &worker.GetTransitionsRequest{
				Key: key,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindGetTransitions,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindGetTransitions, id: requestID, key: key}
	}
}

func (m Model) submitIssueTransition(requestID int, key string, transition jira.Transition) tea.Cmd {
	return func() tea.Msg {
		if key == "" || transition.ID == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindTransitionIssue,
			Timeout: m.requestTimeout,
			TransitionIssue: &worker.TransitionIssueRequest{
				Key:          key,
				TransitionID: transition.ID,
				ToStatus:     transition.ToStatus,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindTransitionIssue,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindTransitionIssue, id: requestID, key: key}
	}
}

func (m Model) submitEditMetadata(requestID int, key string) tea.Cmd {
	return func() tea.Msg {
		if key == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindGetEditMetadata,
			Timeout: m.requestTimeout,
			GetEditMetadata: &worker.GetEditMetadataRequest{
				Key: key,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindGetEditMetadata,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindGetEditMetadata, id: requestID, key: key}
	}
}

func (m Model) submitCreateIssueTypes(requestID int, projectKey string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(projectKey) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindGetCreateIssueTypes,
			Timeout: m.requestTimeout,
			GetCreateIssueTypes: &worker.GetCreateIssueTypesRequest{
				ProjectKey: projectKey,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindGetCreateIssueTypes,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindGetCreateIssueTypes, id: requestID, key: projectKey}
	}
}

func (m Model) submitCreateFields(requestID int, projectKey string, issueTypeID string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(issueTypeID) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindGetCreateFields,
			Timeout: m.requestTimeout,
			GetCreateFields: &worker.GetCreateFieldsRequest{
				ProjectKey:  projectKey,
				IssueTypeID: issueTypeID,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindGetCreateFields,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindGetCreateFields, id: requestID, key: strings.TrimSpace(projectKey + " " + issueTypeID)}
	}
}

func (m Model) submitCreateIssue(requestID int, request worker.CreateIssueRequest) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(request.ProjectKey) == "" || strings.TrimSpace(request.IssueTypeID) == "" || strings.TrimSpace(request.Summary) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:          requestID,
			Kind:        worker.KindCreateIssue,
			Timeout:     m.requestTimeout,
			CreateIssue: &request,
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindCreateIssue,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindCreateIssue, id: requestID, key: request.ProjectKey}
	}
}

func (m Model) submitUpdateSummary(requestID int, key string, summary string) tea.Cmd {
	return func() tea.Msg {
		if key == "" || strings.TrimSpace(summary) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindUpdateSummary,
			Timeout: m.requestTimeout,
			UpdateSummary: &worker.UpdateSummaryRequest{
				Key:     key,
				Summary: summary,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindUpdateSummary,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindUpdateSummary, id: requestID, key: key}
	}
}

func (m Model) submitUpdateDescription(requestID int, key string, description string) tea.Cmd {
	return func() tea.Msg {
		if key == "" || strings.TrimSpace(description) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindUpdateDescription,
			Timeout: m.requestTimeout,
			UpdateDescription: &worker.UpdateDescriptionRequest{
				Key:         key,
				Description: description,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindUpdateDescription,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindUpdateDescription, id: requestID, key: key}
	}
}

func (m Model) submitUpdatePriority(requestID int, key string, priority jira.FieldOption) tea.Cmd {
	return func() tea.Msg {
		if key == "" || (strings.TrimSpace(priority.ID) == "" && strings.TrimSpace(priority.Name) == "") {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindUpdatePriority,
			Timeout: m.requestTimeout,
			UpdatePriority: &worker.UpdatePriorityRequest{
				Key:      key,
				Priority: priority,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindUpdatePriority,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindUpdatePriority, id: requestID, key: key}
	}
}

func (m Model) submitUpdateAssignee(requestID int, key string, assignee jira.User) tea.Cmd {
	return func() tea.Msg {
		if key == "" || strings.TrimSpace(assignee.AccountID) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindUpdateAssignee,
			Timeout: m.requestTimeout,
			UpdateAssignee: &worker.UpdateAssigneeRequest{
				Key:      key,
				Assignee: assignee,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindUpdateAssignee,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindUpdateAssignee, id: requestID, key: key}
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
		return workSubmittedMsg{kind: worker.KindAddComment, id: requestID, key: key}
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
		return workSubmittedMsg{kind: worker.KindSearchUsers, id: requestID, key: query}
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

func (m *Model) updateIssueStatus(key string, status string) {
	if key == "" || status == "" {
		return
	}
	for index := range m.issues {
		if m.issues[index].Key == key {
			m.issues[index].Status = status
			break
		}
	}
	if detail, ok := m.details[key]; ok {
		detail.Status = status
		detail.Issue.Status = status
		m.details[key] = detail
	}
}

func (m *Model) updateIssuePriority(key string, priority string) {
	if key == "" || priority == "" {
		return
	}
	for index := range m.issues {
		if m.issues[index].Key == key {
			m.issues[index].Priority = priority
			break
		}
	}
	if detail, ok := m.details[key]; ok {
		detail.Priority = priority
		detail.Issue.Priority = priority
		m.details[key] = detail
	}
}

func (m *Model) updateIssueAssignee(key string, assignee string) {
	if key == "" || assignee == "" {
		return
	}
	for index := range m.issues {
		if m.issues[index].Key == key {
			m.issues[index].Assignee = assignee
			break
		}
	}
	if detail, ok := m.details[key]; ok {
		detail.Assignee = assignee
		detail.Issue.Assignee = assignee
		m.details[key] = detail
	}
}

func (m *Model) updateIssueSummary(key string, summary string) {
	if key == "" || summary == "" {
		return
	}
	for index := range m.issues {
		if m.issues[index].Key == key {
			m.issues[index].Summary = summary
			break
		}
	}
	if detail, ok := m.details[key]; ok {
		detail.Summary = summary
		detail.Issue.Summary = summary
		m.details[key] = detail
	}
}

func (m *Model) updateIssueDescription(key string, description string) {
	if key == "" {
		return
	}
	if detail, ok := m.details[key]; ok {
		detail.Description = description
		m.details[key] = detail
	}
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

func projectKeyFromJQL(jql string) string {
	fields := strings.Fields(strings.ReplaceAll(jql, "\"", " "))
	for index := 0; index+2 < len(fields); index++ {
		if strings.EqualFold(fields[index], "project") && fields[index+1] == "=" {
			return strings.Trim(fields[index+2], "'\"()")
		}
	}
	return ""
}

func unsupportedRequiredCreateFields(fields []jira.CreateField) []string {
	var names []string
	for _, field := range fields {
		if !field.Required {
			continue
		}
		if isBuiltInCreateTextField(field) || supportedCreateField(field) {
			continue
		}
		names = append(names, displayValue(field.Name, displayValue(field.Key, field.ID)))
	}
	return names
}

func supportedCreateFields(fields []jira.CreateField) []jira.CreateField {
	var supported []jira.CreateField
	for _, field := range fields {
		if isBuiltInCreateTextField(field) || !supportedCreateField(field) {
			continue
		}
		supported = append(supported, field)
	}
	return supported
}

func supportedCreateField(field jira.CreateField) bool {
	id := strings.ToLower(strings.TrimSpace(displayValue(field.ID, field.Key)))
	system := strings.ToLower(strings.TrimSpace(field.SchemaSystem))
	schemaType := strings.ToLower(strings.TrimSpace(field.SchemaType))
	if id == "priority" || system == "priority" || id == "labels" || system == "labels" || id == "components" || system == "components" {
		return true
	}
	if len(field.AllowedValues) > 0 && (schemaType == "option" || schemaType == "priority" || schemaType == "") {
		return true
	}
	switch schemaType {
	case "string", "textarea", "text", "number":
		return true
	default:
		return false
	}
}

func isBuiltInCreateTextField(field jira.CreateField) bool {
	id := strings.ToLower(strings.TrimSpace(displayValue(field.ID, field.Key)))
	system := strings.ToLower(strings.TrimSpace(field.SchemaSystem))
	return id == "summary" || system == "summary" ||
		id == "description" || system == "description" ||
		id == "project" || system == "project" ||
		id == "issuetype" || system == "issuetype"
}

func createFieldUsesPicker(field jira.CreateField) bool {
	id := strings.ToLower(strings.TrimSpace(displayValue(field.ID, field.Key)))
	system := strings.ToLower(strings.TrimSpace(field.SchemaSystem))
	return len(field.AllowedValues) > 0 || id == "priority" || system == "priority"
}

func createFieldValueKey(field jira.CreateField) string {
	return displayValue(field.ID, displayValue(field.Key, field.Name))
}

func defaultCreateFieldSelection(field jira.CreateField) int {
	if !createFieldUsesPicker(field) || len(field.AllowedValues) == 0 {
		return -1
	}
	id := strings.ToLower(strings.TrimSpace(displayValue(field.ID, field.Key)))
	system := strings.ToLower(strings.TrimSpace(field.SchemaSystem))
	if field.Required || id == "priority" || system == "priority" {
		return 0
	}
	return -1
}

func filteredCreateFieldOptionIndexes(options []jira.FieldOption, filter string) []int {
	filter = normalizeCreateDraftFieldName(filter)
	indexes := make([]int, 0, len(options))
	for index, option := range options {
		if filter == "" {
			indexes = append(indexes, index)
			continue
		}
		name := normalizeCreateDraftFieldName(option.Name)
		id := normalizeCreateDraftFieldName(option.ID)
		if strings.Contains(name, filter) || strings.Contains(id, filter) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func createOptionMatchPosition(matches []int, selected int) int {
	for index, match := range matches {
		if match == selected {
			return index
		}
	}
	return -1
}

func createComponentsField(fields []jira.CreateField) jira.CreateField {
	for _, field := range supportedCreateFields(fields) {
		id := strings.ToLower(strings.TrimSpace(displayValue(field.ID, field.Key)))
		system := strings.ToLower(strings.TrimSpace(field.SchemaSystem))
		if id == "components" || system == "components" {
			return field
		}
	}
	return jira.CreateField{}
}

func boundedSelectionWindow(total int, selected int, limit int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if limit <= 0 || total <= limit {
		return 0, total
	}
	selected = clamp(selected, 0, total-1)
	start := selected - limit/2
	start = clamp(start, 0, total-limit)
	return start, start + limit
}

func prependIssue(issues []jira.Issue, issue jira.Issue) []jira.Issue {
	if strings.TrimSpace(issue.Key) == "" {
		return issues
	}
	result := []jira.Issue{issue}
	for _, existing := range issues {
		if existing.Key == issue.Key {
			continue
		}
		result = append(result, existing)
	}
	return result
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
