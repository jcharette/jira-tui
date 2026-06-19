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
	"github.com/jcharette/jira-tui/internal/cache"
	"github.com/jcharette/jira-tui/internal/claude"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/events"
	"github.com/jcharette/jira-tui/internal/gitworkflow"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/startworkflow"
	"github.com/jcharette/jira-tui/internal/ui"
	"github.com/jcharette/jira-tui/internal/worker"
	"github.com/jellydator/ttlcache/v3"
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
	planningMetadataPageSize           = 25
	planningSprintFetchConcurrency     = 2
	issueTreeRootGutter                = 2
	issueTreeMaxGutter                 = 12
	issueTypeColumnWidth               = 2
	createPickerMaxRows                = 6
	createFieldOptionMaxResults        = 25
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

	queryOpen                    bool
	queryMode                    queryMode
	queryJQLDraft                string
	queryJQLEditor               textarea.Model
	queryJQLEditorReady          bool
	queryAIPrompt                string
	queryAIEditor                textarea.Model
	queryAIEditorReady           bool
	queryAILoading               bool
	queryAIErr                   error
	queryGeneratedJQL            string
	queryGeneratedPrompt         string
	queryHistory                 []cache.QueryHistoryRecord
	queryHistorySelected         int
	queryTemplateSelected        int
	queryViewSelected            int
	querySaveViewOpen            bool
	querySaveViewName            string
	querySaveViewEditor          textinput.Model
	querySaveViewReady           bool
	querySaveViewJQL             string
	querySaveViewIncludeChildren bool
	querySaveViewAction          querySaveViewAction
	querySaveViewIndex           int
	queryAICancel                context.CancelFunc
	activeQueryAIReqID           int

	issues                             []jira.Issue
	collapsedIssueKeys                 map[string]bool
	issueLayout                        issueLayoutMode
	selected                           int
	offset                             int
	detailOffset                       int
	detailSectionOffset                map[string]int
	detailFocus                        int
	detailBackStack                    []int
	hierarchyFocus                     bool
	selectedHierarchy                  int
	commentFocus                       bool
	selectedComment                    int
	actionFocus                        bool
	selectedAction                     int
	actionPaletteOpen                  bool
	actionPaletteFilter                string
	actionPaletteEditor                textinput.Model
	actionPaletteEditorReady           bool
	selectedActionPalette              int
	startWorkflowOpen                  bool
	startWorkflowPreparing             bool
	startWorkflowApplying              bool
	startWorkflow                      startworkflow.Model
	startWorkflowIssue                 jira.Issue
	startWorkflowResult                startworkflow.Result
	startWorkflowOutcomes              []startworkflow.Outcome
	startWorkflowBranchSucceeded       bool
	startWorkflowErr                   error
	activeStartRepoReqID               int
	activeStartBranchReqID             int
	activeStartIssueReqID              int
	gitConfig                          config.Git
	gitClient                          gitworkflow.Client
	transitionFocus                    bool
	selectedTransition                 int
	transitionFieldEditing             bool
	selectedTransitionField            int
	transitionFieldSelections          map[string]int
	transitionFieldMultiSelections     map[string]map[int]bool
	transitionFieldDrafts              map[string]string
	transitionFieldFilters             map[string]string
	transitionFieldOptionsLoading      map[string]bool
	transitionFieldOptionsErr          map[string]error
	transitionFieldOptionsQuery        map[string]string
	transitionFieldCommentEditor       textarea.Model
	transitionFieldCommentEditorReady  bool
	transitionFieldEditorFieldID       string
	transitionSubmitFields             []jira.TransitionFieldValue
	priorityFocus                      bool
	selectedPriority                   int
	labelsFocus                        bool
	labelsEditing                      bool
	labelsDraft                        string
	labelsDirty                        bool
	labelsEditor                       textarea.Model
	labelsEditorReady                  bool
	componentsFocus                    bool
	selectedComponent                  int
	selectedComponents                 map[string]bool
	componentsFilter                   string
	componentsFilterEditor             textinput.Model
	componentsFilterEditorReady        bool
	componentsDirty                    bool
	genericFieldFocus                  bool
	genericFieldMetadataLoading        bool
	genericFieldMetadataRequestKey     string
	genericFieldMetadataErr            error
	genericFieldEditingID              string
	genericField                       jira.EditField
	genericFieldDraft                  string
	genericFieldEditor                 textarea.Model
	genericFieldEditorReady            bool
	genericFieldOptionsLoading         bool
	genericFieldOptionsErr             error
	genericFieldOptionsQuery           string
	selectedGenericFieldOption         int
	selectedGenericFieldOptions        map[string]bool
	genericFieldDirty                  bool
	genericFieldSubmitting             bool
	genericFieldSubmitKey              string
	genericFieldSubmitField            jira.EditField
	genericFieldSubmitValue            jira.EditFieldValue
	assigneeFocus                      bool
	selectedAssignee                   int
	assigneeUsers                      []jira.User
	assigneeQuery                      string
	assigneeQueryEditor                textinput.Model
	assigneeQueryEditorReady           bool
	assigneeSearchLoading              bool
	assigneeSearchErr                  error
	assigneeSearchReqID                int
	assigneeSearchIssueKey             string
	assigneeSubmitting                 bool
	assigneeSubmitKey                  string
	assigneeSubmitValue                jira.User
	issueLinkFocus                     bool
	issueLinkTypesLoading              bool
	issueLinkTypesErr                  error
	issueLinkTypes                     []jira.IssueLinkType
	issueLinkTargetDraft               string
	issueLinkTargetEditor              textinput.Model
	issueLinkTargetEditorReady         bool
	selectedIssueLinkRelation          int
	issueLinkSubmitting                bool
	issueLinkSubmitRequest             jira.CreateIssueLinkRequest
	issueLinkDeleteConfirm             bool
	issueLinkDeleteSubmitting          bool
	issueLinkDeleteID                  string
	issueLinkDeleteTarget              string
	userSearchCache                    *ttlcache.Cache[string, []jira.User]
	summaryFocus                       bool
	summaryEditing                     bool
	summaryDraft                       string
	summaryDirty                       bool
	summaryEditor                      textarea.Model
	summaryEditorReady                 bool
	createOpen                         bool
	createProjectKey                   string
	createParentKey                    string
	createParentSummary                string
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
	createSubmitParentKey              string
	createSubmitSummary                string
	createSubmitDescription            string
	createDynamicValues                map[string]string
	createDynamicSelections            map[string]int
	createDynamicFilters               map[string]string
	createDynamicFilterEditors         map[string]textinput.Model
	createFieldOptionsLoading          map[string]bool
	createFieldOptionsErr              map[string]error
	createFieldOptionsQuery            map[string]string
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
	bugReportOpen                      bool
	bugReportFieldFocus                int
	bugReportTitleDraft                string
	bugReportTitleEditor               textinput.Model
	bugReportTitleEditorReady          bool
	bugReportBodyDraft                 string
	bugReportBodyEditor                textarea.Model
	bugReportBodyEditorReady           bool
	bugReportIncludeDiagnostics        bool
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

	planningProjectKey         string
	planningBoards             []jira.Board
	planningBoardPage          jira.BoardPage
	planningBoardID            int
	planningSprints            map[int][]jira.Sprint
	planningSprintPages        map[int]jira.SprintPage
	planningBoardsLoading      bool
	planningSprintsLoading     bool
	planningBoardsErr          error
	planningSprintsErr         error
	activePlanningBoardsReqID  int
	activePlanningSprintsReqID int
	activePlanningSprintReqIDs map[int]int
	planningSprintQueue        []int

	details                           map[string]jira.IssueDetail
	detailCache                       *ttlcache.Cache[string, jiraCacheRecord[jira.IssueDetail]]
	detailLoading                     bool
	detailErr                         error
	detailRequestKey                  string
	comments                          map[string][]jira.Comment
	commentsCache                     *ttlcache.Cache[string, jiraCacheRecord[[]jira.Comment]]
	commentsLoading                   bool
	commentsErr                       error
	commentsRequestKey                string
	worklogs                          map[string][]jira.Worklog
	worklogsLoading                   bool
	worklogsErr                       error
	worklogsRequestKey                string
	worklogListFocus                  bool
	selectedWorklog                   int
	worklogEditing                    bool
	worklogDeleteConfirm              bool
	worklogDeleteSubmitting           bool
	worklogDeleteID                   string
	worklogFocus                      bool
	worklogFieldFocus                 int
	worklogTimeDraft                  string
	worklogCommentDraft               string
	worklogTimeEditor                 textinput.Model
	worklogTimeEditorReady            bool
	worklogCommentEditor              textarea.Model
	worklogCommentEditorReady         bool
	worklogSubmitting                 bool
	worklogSubmitKey                  string
	worklogSubmitRequest              jira.AddWorklogRequest
	worklogUpdateRequest              jira.UpdateWorklogRequest
	commentDraft                      string
	commentEditor                     textarea.Model
	commentEditorReady                bool
	commentConfirm                    bool
	commentSubmitting                 bool
	commentRequestKey                 string
	commentEditing                    bool
	commentEditIssueKey               string
	commentEditID                     string
	commentEditOriginal               string
	commentMentions                   []jira.Mention
	mentionPickerOpen                 bool
	mentionUsers                      []jira.User
	mentionCursor                     int
	mentionQuery                      string
	mentionQueryEditor                textinput.Model
	mentionQueryEditorReady           bool
	mentionSearchLoading              bool
	mentionSearchErr                  error
	mentionSearchReqID                int
	detailNotice                      string
	activeDetailRequestID             int
	activeCommentsReqID               int
	activeCommentReqID                int
	activeWorklogsReqID               int
	activeAddWorklogReqID             int
	activeExpandReqID                 int
	activeTransitionsReqID            int
	activeTransitionReqID             int
	activeTransitionFieldOptionsReqID int
	activeSummaryMetadataReqID        int
	activeSummaryReqID                int
	activePriorityMetadataReqID       int
	activePriorityReqID               int
	activeLabelsMetadataReqID         int
	activeLabelsReqID                 int
	activeComponentsMetadataReqID     int
	activeComponentsReqID             int
	activeGenericFieldMetadataReqID   int
	activeGenericFieldOptionsReqID    int
	activeGenericFieldReqID           int
	activeAssigneeReqID               int
	activeIssueLinkTypesReqID         int
	activeCreateIssueLinkReqID        int
	activeDeleteIssueLinkReqID        int
	activeUpdateWorklogReqID          int
	activeDeleteWorklogReqID          int
	activeCreateIssueTypesReqID       int
	activeCreateFieldsReqID           int
	activeCreateFieldOptionsReqID     int
	activeCreateIssueReqID            int
	expandLoading                     bool
	expandRequestKey                  string
	expandMode                        worker.ExpandMode
	expandedChildrenCache             *ttlcache.Cache[string, jiraCacheRecord[[]jira.Issue]]
	transitions                       map[string][]jira.Transition
	transitionsCache                  *ttlcache.Cache[string, jiraCacheRecord[[]jira.Transition]]
	transitionLoading                 bool
	transitionSubmitting              bool
	transitionRequestKey              string
	transitionSubmitKey               string
	transitionSubmitToStatus          string
	transitionErr                     error
	editMetadata                      map[string]jira.EditMetadata
	editMetadataCache                 *ttlcache.Cache[string, jiraCacheRecord[jira.EditMetadata]]
	summaryMetadataLoading            bool
	summaryMetadataRequestKey         string
	summaryMetadataErr                error
	summarySubmitting                 bool
	summarySubmitKey                  string
	summarySubmitValue                string
	priorityMetadataLoading           bool
	priorityMetadataRequestKey        string
	priorityMetadataErr               error
	prioritySubmitting                bool
	prioritySubmitKey                 string
	prioritySubmitValue               jira.FieldOption
	labelsMetadataLoading             bool
	labelsMetadataRequestKey          string
	labelsMetadataErr                 error
	labelsSubmitting                  bool
	labelsSubmitKey                   string
	labelsSubmitValue                 []string
	componentsMetadataLoading         bool
	componentsMetadataRequestKey      string
	componentsMetadataErr             error
	componentsSubmitting              bool
	componentsSubmitKey               string
	componentsSubmitValue             []jira.FieldOption

	refreshInterval   time.Duration
	requestTimeout    time.Duration
	workerCount       int
	queueSize         int
	nextRequestID     int
	activeRequestID   int
	theme             ui.Theme
	symbolMode        issueSymbolMode
	savedViewWriter   SavedViewWriter
	savedViewsWriter  SavedViewsWriter
	diagnosticSink    diagnosticSink
	diagnosticLogPath string
}

type mode int
type sortMode int
type issueStatusFilter int
type issueLayoutMode int
type queryMode int
type querySaveViewAction int
type SavedViewWriter func(config.IssueView) error
type SavedViewsWriter func([]config.IssueView, string) error

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
	issueLayoutTable issueLayoutMode = iota
	issueLayoutWorkbench
	issueLayoutLanes
	issueLayoutPlanning
)

const (
	queryModeJQL queryMode = iota
	queryModeAI
	queryModeTemplates
	queryModeRecent
	queryModeViews
)

const (
	querySaveViewActionAdd querySaveViewAction = iota
	querySaveViewActionRename
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

func WithGitConfig(git config.Git) Option {
	return func(m *Model) {
		m.gitConfig = git
	}
}

func WithGitWorkflowClient(client gitworkflow.Client) Option {
	return func(m *Model) {
		if client != nil {
			m.gitClient = client
		}
	}
}

func WithDiagnosticLog(sink diagnosticSink, path string) Option {
	return func(m *Model) {
		m.diagnosticSink = sink
		m.diagnosticLogPath = strings.TrimSpace(path)
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

func WithSavedViewWriter(writer SavedViewWriter) Option {
	return func(m *Model) {
		m.savedViewWriter = writer
	}
}

func WithSavedViewsWriter(writer SavedViewsWriter) Option {
	return func(m *Model) {
		m.savedViewsWriter = writer
	}
}

func WithActiveViewStore(store activeViewStore, namespace string) Option {
	return func(m *Model) {
		m.activeViewStore = store
		m.activeViewNamespace = strings.TrimSpace(namespace)
	}
}

func WithPlanningProject(projectKey string) Option {
	return func(m *Model) {
		m.planningProjectKey = strings.TrimSpace(projectKey)
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

type queryAIResultMsg struct {
	id   int
	text string
	err  error
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
		issueLayout:           issueLayoutLanes,
		theme:                 ui.NewTheme(config.DefaultTheme()),
		symbolMode:            symbolModeAuto,
		gitConfig:             config.Defaults().Git,
		gitClient:             gitworkflow.NewCLIClient(),
		details:               make(map[string]jira.IssueDetail),
		activeViewCache:       newIssueViewCache(),
		detailCache:           newJiraCache[jira.IssueDetail](issueDetailCacheRetentionTTL),
		comments:              make(map[string][]jira.Comment),
		commentsCache:         newJiraCache[[]jira.Comment](issueCommentsCacheRetentionTTL),
		worklogs:              make(map[string][]jira.Worklog),
		transitions:           make(map[string][]jira.Transition),
		transitionsCache:      newJiraCache[[]jira.Transition](issueTransitionsCacheRetentionTTL),
		editMetadata:          make(map[string]jira.EditMetadata),
		editMetadataCache:     newJiraCache[jira.EditMetadata](issueEditMetadataCacheRetentionTTL),
		createIssueTypesCache: newJiraCache[[]jira.CreateIssueType](createIssueTypesCacheRetentionTTL),
		createFieldsCache:     newJiraCache[[]jira.CreateField](createFieldsCacheRetentionTTL),
		expandedChildrenCache: newJiraCache[[]jira.Issue](expandedChildrenCacheRetentionTTL),
		planningSprints:       make(map[int][]jira.Sprint),
		planningSprintPages:   make(map[int]jira.SprintPage),
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
	if next, planningCmd := m.startPlanningMetadataLoad(); planningCmd != nil {
		m = next
		cmds = append(cmds, planningCmd)
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
	case startRepoDetectedMsg:
		return m.handleStartRepoDetected(msg)
	case startBranchResultMsg:
		return m.handleStartBranchResult(msg)
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
	case queryAIResultMsg:
		m = m.handleQueryAIResult(msg)
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
		if m.bugReportOpen {
			return m.updateBugReport(msg)
		}
		if m.queryOpen {
			return m.updateQueryModal(msg)
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
		if m.mode == modeDetail && m.startWorkflowOpen {
			return m.updateStartWorkflow(msg)
		}
		if m.mode == modeDetail && m.actionPaletteOpen {
			return m.updateActionPalette(msg)
		}
		if m.mode == modeComment {
			return m.updateCommentComposer(msg)
		}
		if m.mode == modeDetail && m.transitionFieldEditing {
			return m.updateTransitionFieldForm(msg)
		}
		if m.mode == modeDetail && m.labelsEditing {
			return m.updateLabelsEditor(msg)
		}
		if m.mode == modeDetail && m.componentsFocus {
			return m.updateComponentsEditor(msg)
		}
		if m.mode == modeDetail && m.genericFieldFocus {
			return m.updateGenericFieldEditor(msg)
		}
		if m.mode == modeDetail && m.issueLinkFocus {
			return m.updateIssueLinkEditor(msg)
		}
		if m.mode == modeDetail && m.issueLinkDeleteConfirm {
			switch msg.String() {
			case "esc":
				m.cancelIssueLinkDelete()
				return m, nil
			case "enter", "d":
				return m.submitIssueLinkDelete()
			}
			return m, nil
		}
		if m.mode == modeDetail && m.worklogFocus {
			return m.updateWorklogEditor(msg)
		}
		if m.mode == modeDetail && m.worklogDeleteConfirm {
			switch msg.String() {
			case "esc":
				m.cancelWorklogDelete()
				return m, nil
			case "enter", "d":
				return m.submitSelectedWorklogDelete()
			}
			return m, nil
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
		case "B":
			m = m.startBugReport()
			return m, nil
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
				if m.startWorkflowOpen {
					m.closeStartWorkflow()
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
				if m.labelsFocus {
					m.labelsFocus = false
					m.labelsEditing = false
					m.detailNotice = ""
					return m, nil
				}
				if m.componentsFocus {
					m.componentsFocus = false
					m.componentsDirty = false
					m.detailNotice = ""
					return m, nil
				}
				if m.genericFieldFocus {
					m.closeGenericFieldEditor()
					return m, nil
				}
				if m.assigneeFocus {
					m.assigneeFocus = false
					m.detailNotice = ""
					return m, nil
				}
				if m.issueLinkFocus {
					m.closeIssueLinkEditor()
					return m, nil
				}
				if m.issueLinkDeleteConfirm {
					m.cancelIssueLinkDelete()
					return m, nil
				}
				if m.worklogFocus {
					m.closeWorklogEditor()
					return m, nil
				}
				if m.worklogDeleteConfirm {
					m.cancelWorklogDelete()
					return m, nil
				}
				if m.worklogListFocus {
					m.worklogListFocus = false
					m.detailNotice = ""
					return m, nil
				}
				if m.hierarchyFocus {
					m.hierarchyFocus = false
					m.detailNotice = ""
					return m, nil
				}
				if m.commentFocus {
					m.commentFocus = false
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
				m.commentFocus = false
				m.actionFocus = false
				m.transitionFocus = false
				m.priorityFocus = false
				m.assigneeFocus = false
				m.issueLinkFocus = false
				m.worklogFocus = false
				m.summaryFocus = false
				m.detailNotice = ""
			}
		case "r":
			m.err = nil
			return m.startRefresh()
		case "/":
			if m.mode == modeTable {
				m.startQueryModal()
				return m, nil
			}
		case "v":
			if m.mode == modeTable {
				m.startQueryModal()
				m.openCurrentQuerySaveViewPrompt()
				return m, nil
			}
		case "n":
			return m.startCreateIssue()
		case ".":
			if m.mode == modeDetail {
				m.openActionPalette()
				return m, nil
			}
		case "e":
			if m.mode == modeDetail && m.commentFocus {
				return m.startSelectedCommentEditor()
			}
			if m.mode == modeDetail && m.worklogListFocus {
				return m.startSelectedWorklogEditor()
			}
		case "d":
			if m.mode == modeDetail && m.linkFocus {
				return m.startIssueLinkDelete()
			}
			if m.mode == modeDetail && m.worklogListFocus {
				return m.startSelectedWorklogDelete()
			}
		case "x":
			if m.mode == modeTable {
				return m.startExpandSelectedIssue(worker.ExpandModeOpen)
			}
		case "X":
			if m.mode == modeTable {
				return m.startExpandSelectedIssue(worker.ExpandModeAll)
			}
		case "z":
			if m.mode == modeTable {
				m.toggleSelectedIssueCollapse()
				return m, nil
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
		case "a":
			if m.mode == modeDetail {
				if m.inlineDescriptionAIAvailable() {
					return m.openInlineDescriptionAI()
				}
				if m.claudeAvailable() {
					m.jumpDetailSection("Claude")
					return m, nil
				}
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
			if m.mode == modeDetail && m.labelsFocus {
				return m.submitSelectedLabels()
			}
			if m.mode == modeDetail && m.componentsFocus {
				return m.submitSelectedComponents()
			}
			if m.mode == modeDetail && m.genericFieldFocus {
				return m.submitGenericField()
			}
			if m.mode == modeDetail && m.assigneeFocus {
				return m.submitSelectedAssignee()
			}
			if m.mode == modeDetail && m.issueLinkFocus {
				return m.submitSelectedIssueLink()
			}
			if m.mode == modeDetail && m.summaryFocus {
				return m.startSummaryEditor()
			}
			if m.mode == modeDetail && m.hierarchyFocus {
				return m.openSelectedHierarchyIssue()
			}
			if m.mode == modeDetail && m.commentFocus {
				m.startCommentComposer()
				return m, nil
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
				if m.issueLinkFocus {
					m.moveSelectedIssueLinkRelation(-1)
					return m, nil
				}
				if m.worklogListFocus {
					m.moveSelectedWorklog(-1)
					return m, nil
				}
				if m.genericFieldFocus {
					m.moveSelectedGenericFieldOption(-1)
					return m, nil
				}
				if m.hierarchyFocus {
					m.moveSelectedHierarchyIssue(-1)
					return m, nil
				}
				if m.commentFocus {
					m.moveSelectedComment(-1)
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
				if m.issueLinkFocus {
					m.moveSelectedIssueLinkRelation(1)
					return m, nil
				}
				if m.worklogListFocus {
					m.moveSelectedWorklog(1)
					return m, nil
				}
				if m.genericFieldFocus {
					m.moveSelectedGenericFieldOption(1)
					return m, nil
				}
				if m.hierarchyFocus {
					m.moveSelectedHierarchyIssue(1)
					return m, nil
				}
				if m.commentFocus {
					m.moveSelectedComment(1)
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
		case "L":
			if m.mode == modeTable {
				m.cycleIssueLayoutMode()
				if m.issueLayout == issueLayoutWorkbench {
					return m.startSelectedIssuePrefetch()
				}
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
			visible := m.currentVisibleIssueIndexes(m.browserLayout(m.width))
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
			visible := m.currentVisibleIssueIndexes(m.browserLayout(m.width))
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

	if m.queryOpen {
		b.WriteString(m.renderQueryModal(layout))
		b.WriteString("\n\n")
		b.WriteString(m.renderModelFooterHelp(layout))
		return b.String()
	}

	if m.bugReportOpen {
		b.WriteString(m.renderBugReport(layout))
		b.WriteString("\n\n")
		b.WriteString(m.renderModelFooterHelp(layout))
		return b.String()
	}

	if m.createOpen {
		b.WriteString(m.renderCreateIssue(layout))
		b.WriteString("\n\n")
		b.WriteString(m.renderModelFooterHelp(layout))
		return b.String()
	}

	if m.mode == modeDetail && m.actionPaletteOpen && len(m.issues) > 0 {
		b.WriteString(m.renderActionPalette(layout))
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
