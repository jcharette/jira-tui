package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/jellydator/ttlcache/v3"
	"github.com/jon/jira-tui/internal/claude"
	"github.com/jon/jira-tui/internal/config"
	"github.com/jon/jira-tui/internal/events"
	"github.com/jon/jira-tui/internal/jira"
	"github.com/jon/jira-tui/internal/ui"
	"github.com/jon/jira-tui/internal/worker"
)

const (
	maxIssues                          = 50
	maxComments                        = 10
	userSearchCacheTTL                 = 2 * time.Minute
	issueDetailCacheTTL                = 45 * time.Second
	issueDetailCacheRetentionTTL       = 15 * time.Minute
	issueCommentsCacheTTL              = 90 * time.Second
	issueCommentsCacheRetentionTTL     = 15 * time.Minute
	issueTransitionsCacheTTL           = 90 * time.Second
	issueTransitionsCacheRetentionTTL  = 15 * time.Minute
	issueEditMetadataCacheTTL          = 5 * time.Minute
	issueEditMetadataCacheRetentionTTL = 30 * time.Minute
	createIssueTypesCacheTTL           = 5 * time.Minute
	createIssueTypesCacheRetentionTTL  = 30 * time.Minute
	createFieldsCacheTTL               = 5 * time.Minute
	createFieldsCacheRetentionTTL      = 30 * time.Minute
	expandedChildrenCacheTTL           = 90 * time.Second
	expandedChildrenCacheRetentionTTL  = 15 * time.Minute
	activeViewCacheTTL                 = 90 * time.Second
	activeViewCacheDisplayTTL          = 24 * time.Hour
	activeViewCacheRetentionTTL        = 30 * time.Minute
	selectedIssueDetailPrefetchLimit   = 12
	initialRequestID                   = 1
	defaultRequestTimeout              = 20 * time.Second
	defaultWorkerCount                 = 2
	defaultQueueSize                   = 16
	minUsefulIssueRows                 = 8
	appChromeRows                      = 6
	panelFrameRows                     = 4
	detailHeaderRows                   = 6
	issueTreeRootGutter                = 2
	issueTreeMaxGutter                 = 12
	issueTypeColumnWidth               = 2
	createPickerMaxRows                = 6
)

const (
	createTypeFieldIndex = iota
	createSummaryFieldIndex
	createDescriptionFieldIndex
)

type Model struct {
	workers      *worker.Pool
	jql          string
	views        []config.IssueView
	view         int
	mode         mode
	sort         sortMode
	statusFilter issueStatusFilter

	issues                             []jira.Issue
	collapsedIssueKeys                 map[string]bool
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
	assigneeQueryEditor                textinput.Model
	assigneeQueryEditorReady           bool
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
	createIssueTypesCache              *ttlcache.Cache[string, jiraCacheRecord[[]jira.CreateIssueType]]
	selectedCreateIssueType            int
	createIssueTypesLoading            bool
	createIssueTypesErr                error
	createFields                       []jira.CreateField
	createFieldsCache                  *ttlcache.Cache[string, jiraCacheRecord[[]jira.CreateField]]
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
	createDynamicFilterEditors         map[string]textinput.Model
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
	workerRequestStartedAt             map[int]time.Time
	eventStream                        eventStream
	eventInbox                         <-chan events.Event
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

	loading             bool
	refreshing          bool
	viewStale           bool
	err                 error
	lastSynced          time.Time
	activeViewCache     *ttlcache.Cache[string, issueViewCacheRecord]
	activeViewStore     activeViewStore
	activeViewNamespace string

	details                     map[string]jira.IssueDetail
	detailCache                 *ttlcache.Cache[string, jiraCacheRecord[jira.IssueDetail]]
	detailLoading               bool
	detailErr                   error
	detailRequestKey            string
	comments                    map[string][]jira.Comment
	commentsCache               *ttlcache.Cache[string, jiraCacheRecord[[]jira.Comment]]
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
	mentionQueryEditor          textinput.Model
	mentionQueryEditorReady     bool
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
	expandedChildrenCache       *ttlcache.Cache[string, jiraCacheRecord[[]jira.Issue]]
	transitions                 map[string][]jira.Transition
	transitionsCache            *ttlcache.Cache[string, jiraCacheRecord[[]jira.Transition]]
	transitionLoading           bool
	transitionSubmitting        bool
	transitionRequestKey        string
	transitionSubmitKey         string
	transitionSubmitToStatus    string
	transitionErr               error
	editMetadata                map[string]jira.EditMetadata
	editMetadataCache           *ttlcache.Cache[string, jiraCacheRecord[jira.EditMetadata]]
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
type issueStatusFilter int

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

const (
	issueStatusFilterAll issueStatusFilter = iota
	issueStatusFilterActive
)

const (
	detailTargetField   detailTargetKind = "field"
	detailTargetSection detailTargetKind = "section"
)

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

func WithActiveViewStore(store activeViewStore, namespace string) Option {
	return func(m *Model) {
		m.activeViewStore = store
		m.activeViewNamespace = strings.TrimSpace(namespace)
	}
}

func WithEventStream(stream eventStream) Option {
	return func(m *Model) {
		if stream == nil {
			return
		}
		inbox, err := stream.Subscribe(context.Background())
		if err != nil {
			return
		}
		m.eventStream = stream
		m.eventInbox = inbox
	}
}

func WithNow(now func() time.Time) Option {
	return func(m *Model) {
		if now != nil {
			m.now = now
		}
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

type appEventMsg struct {
	event events.Event
}

func NewModel(client worker.JiraClient, jql string, options ...Option) Model {
	model := Model{
		jql:                   jql,
		loading:               true,
		requestTimeout:        defaultRequestTimeout,
		workerCount:           defaultWorkerCount,
		queueSize:             defaultQueueSize,
		nextRequestID:         initialRequestID,
		activeRequestID:       initialRequestID,
		theme:                 ui.NewTheme(config.DefaultTheme()),
		symbolMode:            symbolModeAuto,
		details:               make(map[string]jira.IssueDetail),
		activeViewCache:       newIssueViewCache(),
		detailCache:           newJiraCache[jira.IssueDetail](issueDetailCacheRetentionTTL),
		comments:              make(map[string][]jira.Comment),
		commentsCache:         newJiraCache[[]jira.Comment](issueCommentsCacheRetentionTTL),
		transitions:           make(map[string][]jira.Transition),
		transitionsCache:      newJiraCache[[]jira.Transition](issueTransitionsCacheRetentionTTL),
		editMetadata:          make(map[string]jira.EditMetadata),
		editMetadataCache:     newJiraCache[jira.EditMetadata](issueEditMetadataCacheRetentionTTL),
		createIssueTypesCache: newJiraCache[[]jira.CreateIssueType](createIssueTypesCacheRetentionTTL),
		createFieldsCache:     newJiraCache[[]jira.CreateField](createFieldsCacheRetentionTTL),
		expandedChildrenCache: newJiraCache[[]jira.Issue](expandedChildrenCacheRetentionTTL),
		detailSectionOffset:   make(map[string]int),
		userSearchCache:       ttlcache.New[string, []jira.User](ttlcache.WithTTL[string, []jira.User](userSearchCacheTTL)),
		claudeRunner:          claude.LocalRunner{},
		now:                   time.Now,
	}
	for _, option := range options {
		option(&model)
	}
	model.hydrateActiveIssueView()
	model.workers = worker.NewPool(
		client,
		worker.WithWorkerCount(model.workerCount),
		worker.WithQueueSize(model.queueSize),
	)
	return model
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.waitForWorkerResult(),
		m.scheduleRefresh(),
	}
	if m.eventInbox != nil {
		cmds = append(cmds, m.waitForAppEvent())
	}
	if m.loading || m.viewStale || len(m.issues) == 0 {
		priority := worker.PriorityForeground
		if len(m.issues) > 0 && !m.loading {
			priority = worker.PriorityBackground
		}
		cmds = append(cmds, m.submitIssueSearch(m.activeRequestID, priority))
	}
	return tea.Batch(cmds...)
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
			m, cmd = m.startCachedRefresh(worker.PriorityBackground)
		}
		return m, tea.Batch(cmd, m.scheduleRefresh())
	case workerResultMsg:
		var cmd tea.Cmd
		m.recordWorkerResult(resultDiagnosticEvent(msg.result))
		m.recordAPIResult(msg.result)
		m, cmd = m.handleWorkerResult(msg.result)
		return m, tea.Batch(cmd, m.waitForWorkerResult())
	case appEventMsg:
		m.recordAppEvent(msg.event)
		return m, m.waitForAppEvent()
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
		m.recordWorkerSubmitted(msg.kind, msg.id, msg.key)
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
			return m.startSelectedIssuePrefetch()
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
			return m.startSelectedIssuePrefetch()
		case "pgup", "ctrl+b":
			if m.mode == modeDetail {
				m.pageDetail(-1)
				return m, nil
			}
			m.pageSelection(-1)
			return m.startSelectedIssuePrefetch()
		case "pgdown", "ctrl+f", " ":
			if m.mode == modeDetail {
				m.pageDetail(1)
				return m, nil
			}
			m.pageSelection(1)
			return m.startSelectedIssuePrefetch()
		case "f":
			if m.mode == modeTable {
				m.toggleStatusFilter()
				return m, nil
			}
		case "home", "g":
			if m.mode == modeDetail {
				m.setDetailOffset(0)
				m.linkFocus = false
				m.transitionFocus = false
				m.priorityFocus = false
				m.assigneeFocus = false
				return m, nil
			}
			displayTree := buildIssueDisplayTree(m.issues)
			visible := m.visibleIssueIndexes(displayTree)
			if len(visible) > 0 {
				m.selected = visible[0]
			} else {
				m.selected = 0
			}
			m.offset = 0
			return m.startSelectedIssuePrefetch()
		case "end", "G":
			if m.mode == modeDetail {
				m.scrollDetailToBottom()
				m.linkFocus = false
				m.transitionFocus = false
				m.priorityFocus = false
				m.assigneeFocus = false
				return m, nil
			}
			displayTree := buildIssueDisplayTree(m.issues)
			visible := m.visibleIssueIndexes(displayTree)
			if len(visible) > 0 {
				m.selected = visible[len(visible)-1]
			} else {
				m.selected = max(0, len(m.issues)-1)
			}
			m.ensureSelectionVisible(m.currentLayoutRows())
			return m.startSelectedIssuePrefetch()
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

const (
	claudeAssistTargetTicket claudeAssistTarget = iota
	claudeAssistTargetDescription
)

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
