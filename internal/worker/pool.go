package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/startworkflow"
	"github.com/panjf2000/ants/v2"
)

const (
	defaultWorkerCount = 2
	defaultQueueSize   = 16
)

var (
	ErrQueueFull      = errors.New("worker queue is full")
	ErrPoolClosed     = errors.New("worker pool is closed")
	ErrInvalidRequest = errors.New("invalid worker request")
	ErrWorkerPanic    = errors.New("worker panicked")
)

type Kind string

const (
	KindSearchIssues        Kind = "search_issues"
	KindGetCurrentUser      Kind = "get_current_user"
	KindStartIssue          Kind = "start_issue"
	KindGetIssue            Kind = "get_issue"
	KindGetComments         Kind = "get_comments"
	KindAddComment          Kind = "add_comment"
	KindUpdateComment       Kind = "update_comment"
	KindSearchUsers         Kind = "search_users"
	KindExpandIssues        Kind = "expand_issues"
	KindGetTransitions      Kind = "get_transitions"
	KindTransitionIssue     Kind = "transition_issue"
	KindGetEditMetadata     Kind = "get_edit_metadata"
	KindGetCreateIssueTypes Kind = "get_create_issue_types"
	KindGetCreateFields     Kind = "get_create_fields"
	KindSearchFieldOptions  Kind = "search_field_options"
	KindGetBoards           Kind = "get_boards"
	KindGetBoardSprints     Kind = "get_board_sprints"
	KindMoveIssuesToSprint  Kind = "move_issues_to_sprint"
	KindGetIssueLinkTypes   Kind = "get_issue_link_types"
	KindCreateIssueLink     Kind = "create_issue_link"
	KindDeleteIssueLink     Kind = "delete_issue_link"
	KindGetWorklogs         Kind = "get_worklogs"
	KindAddWorklog          Kind = "add_worklog"
	KindUpdateWorklog       Kind = "update_worklog"
	KindDeleteWorklog       Kind = "delete_worklog"
	KindUpdateSummary       Kind = "update_summary"
	KindUpdateDescription   Kind = "update_description"
	KindUpdatePriority      Kind = "update_priority"
	KindUpdateLabels        Kind = "update_labels"
	KindUpdateComponents    Kind = "update_components"
	KindUpdateEditField     Kind = "update_edit_field"
	KindUpdateParent        Kind = "update_parent"
	KindUpdateTimeTracking  Kind = "update_time_tracking"
	KindUpdateAssignee      Kind = "update_assignee"
	KindCreateIssue         Kind = "create_issue"
)

type Priority int

const (
	PriorityDefault Priority = iota
	PriorityWrite
	PriorityForeground
	PriorityRefresh
	PriorityPrefetch
	PriorityBackground
)

type JiraClient interface {
	SearchIssues(ctx context.Context, jql string, maxResults int) ([]jira.Issue, error)
	CurrentUser(ctx context.Context) (jira.User, error)
	GetIssue(ctx context.Context, key string) (jira.IssueDetail, error)
	GetComments(ctx context.Context, key string, maxResults int) ([]jira.Comment, error)
	AddComment(ctx context.Context, key string, body string, mentions []jira.Mention) (jira.Comment, error)
	UpdateComment(ctx context.Context, key string, commentID string, body string, mentions []jira.Mention) (jira.Comment, error)
	SearchUsers(ctx context.Context, query string, maxResults int) ([]jira.User, error)
	SearchAssignableUsers(ctx context.Context, issueKey string, query string, maxResults int) ([]jira.User, error)
	GetTransitions(ctx context.Context, key string) ([]jira.Transition, error)
	TransitionIssue(ctx context.Context, key string, request jira.TransitionIssueRequest) error
	GetEditMetadata(ctx context.Context, key string) (jira.EditMetadata, error)
	GetCreateIssueTypes(ctx context.Context, projectKey string) ([]jira.CreateIssueType, error)
	GetCreateFields(ctx context.Context, projectKey string, issueTypeID string) ([]jira.CreateField, error)
	SearchFieldOptions(ctx context.Context, autocompleteURL string, query string, maxResults int) ([]jira.FieldOption, error)
	GetBoards(ctx context.Context, projectKey string, startAt, maxResults int) (jira.BoardPage, error)
	GetBoardSprints(ctx context.Context, boardID int, states []string, startAt, maxResults int) (jira.SprintPage, error)
	MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error
	GetIssueLinkTypes(ctx context.Context) ([]jira.IssueLinkType, error)
	CreateIssueLink(ctx context.Context, request jira.CreateIssueLinkRequest) error
	DeleteIssueLink(ctx context.Context, linkID string) error
	GetWorklogs(ctx context.Context, key string, maxResults int) ([]jira.Worklog, error)
	AddWorklog(ctx context.Context, key string, request jira.AddWorklogRequest) (jira.Worklog, error)
	UpdateWorklog(ctx context.Context, key string, request jira.UpdateWorklogRequest) (jira.Worklog, error)
	DeleteWorklog(ctx context.Context, key string, worklogID string) error
	CreateIssue(ctx context.Context, request jira.CreateIssueRequest) (jira.Issue, error)
	UpdateSummary(ctx context.Context, key string, summary string) error
	UpdateDescription(ctx context.Context, key string, description string) error
	UpdatePriority(ctx context.Context, key string, priority jira.FieldOption) error
	UpdateLabels(ctx context.Context, key string, labels []string) error
	UpdateComponents(ctx context.Context, key string, components []jira.FieldOption) error
	UpdateEditField(ctx context.Context, key string, value jira.EditFieldValue) error
	UpdateParent(ctx context.Context, key string, request jira.UpdateParentRequest) error
	UpdateTimeTracking(ctx context.Context, key string, request jira.UpdateTimeTrackingRequest) error
	UpdateAssignee(ctx context.Context, key string, assignee jira.User) error
}

type Request struct {
	ID          int
	Kind        Kind
	Timeout     time.Duration
	Priority    Priority
	CoalesceKey string

	SearchIssues        *SearchIssuesRequest
	GetCurrentUser      *GetCurrentUserRequest
	StartIssue          *StartIssueRequest
	GetIssue            *GetIssueRequest
	GetComments         *GetCommentsRequest
	AddComment          *AddCommentRequest
	UpdateComment       *UpdateCommentRequest
	SearchUsers         *SearchUsersRequest
	ExpandIssues        *ExpandIssuesRequest
	GetTransitions      *GetTransitionsRequest
	TransitionIssue     *TransitionIssueRequest
	GetEditMetadata     *GetEditMetadataRequest
	GetCreateIssueTypes *GetCreateIssueTypesRequest
	GetCreateFields     *GetCreateFieldsRequest
	SearchFieldOptions  *SearchFieldOptionsRequest
	GetBoards           *GetBoardsRequest
	GetBoardSprints     *GetBoardSprintsRequest
	MoveIssuesToSprint  *MoveIssuesToSprintRequest
	GetIssueLinkTypes   *GetIssueLinkTypesRequest
	CreateIssueLink     *CreateIssueLinkRequest
	DeleteIssueLink     *DeleteIssueLinkRequest
	GetWorklogs         *GetWorklogsRequest
	AddWorklog          *AddWorklogRequest
	UpdateWorklog       *UpdateWorklogRequest
	DeleteWorklog       *DeleteWorklogRequest
	UpdateSummary       *UpdateSummaryRequest
	UpdateDescription   *UpdateDescriptionRequest
	UpdatePriority      *UpdatePriorityRequest
	UpdateLabels        *UpdateLabelsRequest
	UpdateComponents    *UpdateComponentsRequest
	UpdateEditField     *UpdateEditFieldRequest
	UpdateParent        *UpdateParentRequest
	UpdateTimeTracking  *UpdateTimeTrackingRequest
	UpdateAssignee      *UpdateAssigneeRequest
	CreateIssue         *CreateIssueRequest
}

type SearchIssuesRequest struct {
	JQL             string
	MaxResults      int
	IncludeChildren bool
}

type GetCurrentUserRequest struct{}

type StartIssueRequest struct {
	Result          startworkflow.Result
	BranchSucceeded bool
}

type GetIssueRequest struct {
	Key string
}

type GetCommentsRequest struct {
	Key        string
	MaxResults int
}

type AddCommentRequest struct {
	Key      string
	Body     string
	Mentions []jira.Mention
}

type UpdateCommentRequest struct {
	Key       string
	CommentID string
	Body      string
	Mentions  []jira.Mention
}

type SearchUsersRequest struct {
	Query      string
	IssueKey   string
	MaxResults int
}

type ExpandMode string

const (
	ExpandModeOpen ExpandMode = "open"
	ExpandModeAll  ExpandMode = "all"
)

type ExpandIssuesRequest struct {
	ParentKey  string
	Mode       ExpandMode
	MaxResults int
}

type GetTransitionsRequest struct {
	Key string
}

type TransitionIssueRequest struct {
	Key          string
	TransitionID string
	ToStatus     string
	Fields       []jira.TransitionFieldValue
}

type GetEditMetadataRequest struct {
	Key string
}

type GetCreateIssueTypesRequest struct {
	ProjectKey string
}

type GetCreateFieldsRequest struct {
	ProjectKey  string
	IssueTypeID string
}

type SearchFieldOptionsRequest struct {
	FieldID         string
	AutoCompleteURL string
	Query           string
	MaxResults      int
}

type GetBoardsRequest struct {
	ProjectKey string
	StartAt    int
	MaxResults int
}

type GetBoardSprintsRequest struct {
	BoardID    int
	States     []string
	StartAt    int
	MaxResults int
}

type MoveIssuesToSprintRequest struct {
	Sprint    jira.Sprint
	IssueKeys []string
}

type GetIssueLinkTypesRequest struct{}

type CreateIssueLinkRequest struct {
	Request jira.CreateIssueLinkRequest
}

type DeleteIssueLinkRequest struct {
	IssueKey string
	LinkID   string
	Target   string
}

type GetWorklogsRequest struct {
	Key        string
	MaxResults int
}

type AddWorklogRequest struct {
	Key     string
	Request jira.AddWorklogRequest
}

type UpdateWorklogRequest struct {
	Key     string
	Request jira.UpdateWorklogRequest
}

type DeleteWorklogRequest struct {
	Key       string
	WorklogID string
}

type UpdateSummaryRequest struct {
	Key     string
	Summary string
}

type UpdateDescriptionRequest struct {
	Key         string
	Description string
}

type UpdatePriorityRequest struct {
	Key      string
	Priority jira.FieldOption
}

type UpdateLabelsRequest struct {
	Key    string
	Labels []string
}

type UpdateComponentsRequest struct {
	Key        string
	Components []jira.FieldOption
}

type UpdateEditFieldRequest struct {
	Key   string
	Field jira.EditField
	Value jira.EditFieldValue
}

type UpdateParentRequest struct {
	Key     string
	Request jira.UpdateParentRequest
}

type UpdateTimeTrackingRequest struct {
	Key     string
	Request jira.UpdateTimeTrackingRequest
}

type UpdateAssigneeRequest struct {
	Key      string
	Assignee jira.User
}

type CreateIssueRequest struct {
	ProjectKey  string
	IssueTypeID string
	ParentKey   string
	Summary     string
	Description string
	Fields      []jira.CreateIssueFieldValue
}

type Result struct {
	ID   int
	Kind Kind
	Err  error

	SearchIssues        *SearchIssuesResult
	GetCurrentUser      *GetCurrentUserResult
	StartIssue          *StartIssueResult
	GetIssue            *GetIssueResult
	GetComments         *GetCommentsResult
	AddComment          *AddCommentResult
	UpdateComment       *UpdateCommentResult
	SearchUsers         *SearchUsersResult
	ExpandIssues        *ExpandIssuesResult
	GetTransitions      *GetTransitionsResult
	TransitionIssue     *TransitionIssueResult
	GetEditMetadata     *GetEditMetadataResult
	GetCreateIssueTypes *GetCreateIssueTypesResult
	GetCreateFields     *GetCreateFieldsResult
	SearchFieldOptions  *SearchFieldOptionsResult
	GetBoards           *GetBoardsResult
	GetBoardSprints     *GetBoardSprintsResult
	MoveIssuesToSprint  *MoveIssuesToSprintResult
	GetIssueLinkTypes   *GetIssueLinkTypesResult
	CreateIssueLink     *CreateIssueLinkResult
	DeleteIssueLink     *DeleteIssueLinkResult
	GetWorklogs         *GetWorklogsResult
	AddWorklog          *AddWorklogResult
	UpdateWorklog       *UpdateWorklogResult
	DeleteWorklog       *DeleteWorklogResult
	UpdateSummary       *UpdateSummaryResult
	UpdateDescription   *UpdateDescriptionResult
	UpdatePriority      *UpdatePriorityResult
	UpdateLabels        *UpdateLabelsResult
	UpdateComponents    *UpdateComponentsResult
	UpdateEditField     *UpdateEditFieldResult
	UpdateParent        *UpdateParentResult
	UpdateTimeTracking  *UpdateTimeTrackingResult
	UpdateAssignee      *UpdateAssigneeResult
	CreateIssue         *CreateIssueResult
}

type SearchIssuesResult struct {
	Issues   []jira.Issue
	SyncedAt time.Time
}

type GetCurrentUserResult struct {
	User     jira.User
	SyncedAt time.Time
}

type StartIssueResult struct {
	Key      string
	Outcomes []startworkflow.Outcome
	SyncedAt time.Time
}

type GetIssueResult struct {
	Key      string
	Detail   jira.IssueDetail
	SyncedAt time.Time
}

type GetCommentsResult struct {
	Key      string
	Comments []jira.Comment
	SyncedAt time.Time
}

type AddCommentResult struct {
	Key      string
	Comment  jira.Comment
	SyncedAt time.Time
}

type UpdateCommentResult struct {
	Key      string
	Comment  jira.Comment
	SyncedAt time.Time
}

type SearchUsersResult struct {
	Query    string
	IssueKey string
	Users    []jira.User
	SyncedAt time.Time
}

type ExpandIssuesResult struct {
	ParentKey string
	Mode      ExpandMode
	Issues    []jira.Issue
	SyncedAt  time.Time
}

type GetTransitionsResult struct {
	Key         string
	Transitions []jira.Transition
	SyncedAt    time.Time
}

type TransitionIssueResult struct {
	Key      string
	ToStatus string
	SyncedAt time.Time
}

type GetEditMetadataResult struct {
	Key      string
	Metadata jira.EditMetadata
	SyncedAt time.Time
}

type GetCreateIssueTypesResult struct {
	ProjectKey string
	IssueTypes []jira.CreateIssueType
	SyncedAt   time.Time
}

type GetCreateFieldsResult struct {
	ProjectKey  string
	IssueTypeID string
	Fields      []jira.CreateField
	SyncedAt    time.Time
}

type SearchFieldOptionsResult struct {
	FieldID  string
	Query    string
	Options  []jira.FieldOption
	SyncedAt time.Time
}

type GetBoardsResult struct {
	ProjectKey string
	Page       jira.BoardPage
	SyncedAt   time.Time
}

type GetBoardSprintsResult struct {
	BoardID  int
	Page     jira.SprintPage
	SyncedAt time.Time
}

type MoveIssuesToSprintResult struct {
	Sprint    jira.Sprint
	IssueKeys []string
	SyncedAt  time.Time
}

type GetIssueLinkTypesResult struct {
	Types    []jira.IssueLinkType
	SyncedAt time.Time
}

type CreateIssueLinkResult struct {
	Request  jira.CreateIssueLinkRequest
	SyncedAt time.Time
}

type DeleteIssueLinkResult struct {
	IssueKey string
	LinkID   string
	Target   string
	SyncedAt time.Time
}

type GetWorklogsResult struct {
	Key      string
	Worklogs []jira.Worklog
	SyncedAt time.Time
}

type AddWorklogResult struct {
	Key      string
	Worklog  jira.Worklog
	Request  jira.AddWorklogRequest
	SyncedAt time.Time
}

type UpdateWorklogResult struct {
	Key      string
	Worklog  jira.Worklog
	Request  jira.UpdateWorklogRequest
	SyncedAt time.Time
}

type DeleteWorklogResult struct {
	Key       string
	WorklogID string
	SyncedAt  time.Time
}

type UpdateSummaryResult struct {
	Key      string
	Summary  string
	SyncedAt time.Time
}

type UpdateDescriptionResult struct {
	Key         string
	Description string
	SyncedAt    time.Time
}

type UpdatePriorityResult struct {
	Key      string
	Priority jira.FieldOption
	SyncedAt time.Time
}

type UpdateLabelsResult struct {
	Key      string
	Labels   []string
	SyncedAt time.Time
}

type UpdateComponentsResult struct {
	Key        string
	Components []jira.FieldOption
	SyncedAt   time.Time
}

type UpdateEditFieldResult struct {
	Key      string
	Field    jira.EditField
	Value    jira.EditFieldValue
	SyncedAt time.Time
}

type UpdateParentResult struct {
	Key      string
	Request  jira.UpdateParentRequest
	SyncedAt time.Time
}

type UpdateTimeTrackingResult struct {
	Key      string
	Request  jira.UpdateTimeTrackingRequest
	SyncedAt time.Time
}

type UpdateAssigneeResult struct {
	Key      string
	Assignee jira.User
	SyncedAt time.Time
}

type CreateIssueResult struct {
	Issue    jira.Issue
	SyncedAt time.Time
}

type Option func(*Pool)

type scheduledRequest struct {
	request  Request
	sequence int64
}

type Stats struct {
	Running   int
	Pending   int
	Coalesced int
	Capacity  int
}

type Pool struct {
	client JiraClient

	workerCount int
	queueSize   int

	engine  *ants.Pool
	results chan Result
	done    chan struct{}
	initErr error

	pending           []scheduledRequest
	coalesced         map[string][]Request
	activeCoalesceKey map[string]struct{}
	sequence          int64
	running           int
	cond              *sync.Cond

	wg           sync.WaitGroup
	dispatcherWG sync.WaitGroup
	stopOnce     sync.Once
	mu           sync.Mutex
	closed       bool
}

func NewPool(client JiraClient, options ...Option) *Pool {
	pool := &Pool{
		client:      client,
		workerCount: defaultWorkerCount,
		queueSize:   defaultQueueSize,
		done:        make(chan struct{}),
	}
	for _, option := range options {
		option(pool)
	}
	if pool.workerCount <= 0 {
		pool.workerCount = defaultWorkerCount
	}
	if pool.queueSize <= 0 {
		pool.queueSize = defaultQueueSize
	}

	capacity := pool.workerCount + pool.queueSize
	pool.results = make(chan Result, capacity)
	pool.engine, pool.initErr = ants.NewPool(pool.workerCount)
	pool.coalesced = make(map[string][]Request)
	pool.activeCoalesceKey = make(map[string]struct{})
	pool.cond = sync.NewCond(&pool.mu)
	pool.dispatcherWG.Add(1)
	go pool.dispatch()
	return pool
}

func WithWorkerCount(count int) Option {
	return func(pool *Pool) {
		pool.workerCount = count
	}
}

func WithQueueSize(size int) Option {
	return func(pool *Pool) {
		pool.queueSize = size
	}
}

func (p *Pool) Submit(request Request) error {
	p.mu.Lock()
	if p.initErr != nil {
		p.mu.Unlock()
		return p.initErr
	}
	if p.closed {
		p.mu.Unlock()
		return ErrPoolClosed
	}

	request = normalizeRequest(request)
	if request.CoalesceKey != "" {
		if _, ok := p.activeCoalesceKey[request.CoalesceKey]; ok {
			p.coalesced[request.CoalesceKey] = append(p.coalesced[request.CoalesceKey], request)
			p.mu.Unlock()
			return nil
		}
	}

	var dropped []Result
	if p.admittedLocked() >= p.capacity() {
		droppedRequest, droppedWaiters, ok := p.dropQueuedLowerPriorityLocked(request.Priority)
		if !ok {
			p.mu.Unlock()
			return ErrQueueFull
		}
		dropped = append(dropped, Result{ID: droppedRequest.ID, Kind: droppedRequest.Kind, Err: ErrQueueFull})
		dropped = append(dropped, droppedWaiters...)
	}

	p.enqueueLocked(request)
	p.cond.Signal()
	p.mu.Unlock()

	for _, result := range dropped {
		p.send(result)
	}
	return nil
}

func (p *Pool) Results() <-chan Result {
	return p.results
}

func (p *Pool) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := Stats{
		Running:  p.running,
		Pending:  len(p.pending),
		Capacity: p.capacity(),
	}
	for _, waiters := range p.coalesced {
		stats.Coalesced += len(waiters)
	}
	return stats
}

func (p *Pool) Stop() {
	p.stopOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		close(p.done)
		p.cond.Broadcast()
		p.mu.Unlock()

		p.dispatcherWG.Wait()
		if p.engine != nil {
			p.engine.Release()
		}
		p.wg.Wait()
		close(p.results)
	})
}

func (p *Pool) send(result Result) {
	select {
	case p.results <- result:
	case <-p.done:
	}
}

func (p *Pool) dispatch() {
	defer p.dispatcherWG.Done()
	for {
		p.mu.Lock()
		for !p.closed && (len(p.pending) == 0 || p.running >= p.workerCount) {
			p.cond.Wait()
		}
		if p.closed {
			p.mu.Unlock()
			return
		}
		request := p.popNextLocked()
		p.running++
		p.wg.Add(1)
		p.mu.Unlock()

		p.submitToEngine(request)
	}
}

func (p *Pool) submitToEngine(request Request) {
	err := p.engine.Submit(func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				p.completeRequest(request, Result{ID: request.ID, Kind: request.Kind, Err: ErrWorkerPanic})
			}
		}()

		p.completeRequest(request, p.handle(request))
	})
	if err != nil {
		p.completeRequest(request, Result{ID: request.ID, Kind: request.Kind, Err: mapAntsError(err)})
	}
}

func (p *Pool) completeRequest(request Request, result Result) {
	p.mu.Lock()
	p.running--
	var waiters []Request
	if request.CoalesceKey != "" {
		waiters = p.coalesced[request.CoalesceKey]
		delete(p.coalesced, request.CoalesceKey)
		delete(p.activeCoalesceKey, request.CoalesceKey)
	}
	p.cond.Signal()
	p.mu.Unlock()

	p.send(result)
	for _, waiter := range waiters {
		clone := result
		clone.ID = waiter.ID
		clone.Kind = waiter.Kind
		p.send(clone)
	}
	p.wg.Done()
}

func normalizeRequest(request Request) Request {
	if request.Priority == PriorityDefault || request.Priority < PriorityDefault || request.Priority > PriorityBackground {
		request.Priority = PriorityForeground
	}
	return request
}

func (p *Pool) capacity() int {
	return p.workerCount + p.queueSize
}

func (p *Pool) admittedLocked() int {
	return p.running + len(p.pending)
}

func (p *Pool) enqueueLocked(request Request) {
	if request.CoalesceKey != "" {
		p.activeCoalesceKey[request.CoalesceKey] = struct{}{}
	}
	item := scheduledRequest{request: request, sequence: p.sequence}
	p.sequence++
	index := len(p.pending)
	for i, existing := range p.pending {
		if request.Priority < existing.request.Priority ||
			(request.Priority == existing.request.Priority && item.sequence < existing.sequence) {
			index = i
			break
		}
	}
	p.pending = append(p.pending, scheduledRequest{})
	copy(p.pending[index+1:], p.pending[index:])
	p.pending[index] = item
}

func (p *Pool) popNextLocked() Request {
	item := p.pending[0]
	copy(p.pending, p.pending[1:])
	p.pending = p.pending[:len(p.pending)-1]
	return item.request
}

func (p *Pool) dropQueuedLowerPriorityLocked(priority Priority) (Request, []Result, bool) {
	index := -1
	for i := len(p.pending) - 1; i >= 0; i-- {
		if p.pending[i].request.Priority > priority {
			index = i
			break
		}
	}
	if index < 0 {
		return Request{}, nil, false
	}
	item := p.pending[index]
	p.pending = append(p.pending[:index], p.pending[index+1:]...)
	var droppedWaiters []Result
	if item.request.CoalesceKey != "" {
		waiters := p.coalesced[item.request.CoalesceKey]
		delete(p.coalesced, item.request.CoalesceKey)
		delete(p.activeCoalesceKey, item.request.CoalesceKey)
		for _, waiter := range waiters {
			droppedWaiters = append(droppedWaiters, Result{ID: waiter.ID, Kind: waiter.Kind, Err: ErrQueueFull})
		}
	}
	return item.request, droppedWaiters, true
}

func mapAntsError(err error) error {
	switch {
	case errors.Is(err, ants.ErrPoolClosed):
		return ErrPoolClosed
	case errors.Is(err, ants.ErrPoolOverload):
		return ErrQueueFull
	default:
		return err
	}
}

func (p *Pool) handle(request Request) Result {
	switch request.Kind {
	case KindSearchIssues:
		return p.handleSearchIssues(request)
	case KindGetCurrentUser:
		return p.handleGetCurrentUser(request)
	case KindStartIssue:
		return p.handleStartIssue(request)
	case KindGetIssue:
		return p.handleGetIssue(request)
	case KindGetComments:
		return p.handleGetComments(request)
	case KindAddComment:
		return p.handleAddComment(request)
	case KindUpdateComment:
		return p.handleUpdateComment(request)
	case KindSearchUsers:
		return p.handleSearchUsers(request)
	case KindExpandIssues:
		return p.handleExpandIssues(request)
	case KindGetTransitions:
		return p.handleGetTransitions(request)
	case KindTransitionIssue:
		return p.handleTransitionIssue(request)
	case KindGetEditMetadata:
		return p.handleGetEditMetadata(request)
	case KindGetCreateIssueTypes:
		return p.handleGetCreateIssueTypes(request)
	case KindGetCreateFields:
		return p.handleGetCreateFields(request)
	case KindSearchFieldOptions:
		return p.handleSearchFieldOptions(request)
	case KindGetBoards:
		return p.handleGetBoards(request)
	case KindGetBoardSprints:
		return p.handleGetBoardSprints(request)
	case KindMoveIssuesToSprint:
		return p.handleMoveIssuesToSprint(request)
	case KindGetIssueLinkTypes:
		return p.handleGetIssueLinkTypes(request)
	case KindCreateIssueLink:
		return p.handleCreateIssueLink(request)
	case KindDeleteIssueLink:
		return p.handleDeleteIssueLink(request)
	case KindGetWorklogs:
		return p.handleGetWorklogs(request)
	case KindAddWorklog:
		return p.handleAddWorklog(request)
	case KindUpdateWorklog:
		return p.handleUpdateWorklog(request)
	case KindDeleteWorklog:
		return p.handleDeleteWorklog(request)
	case KindUpdateSummary:
		return p.handleUpdateSummary(request)
	case KindUpdateDescription:
		return p.handleUpdateDescription(request)
	case KindUpdatePriority:
		return p.handleUpdatePriority(request)
	case KindUpdateLabels:
		return p.handleUpdateLabels(request)
	case KindUpdateComponents:
		return p.handleUpdateComponents(request)
	case KindUpdateEditField:
		return p.handleUpdateEditField(request)
	case KindUpdateParent:
		return p.handleUpdateParent(request)
	case KindUpdateTimeTracking:
		return p.handleUpdateTimeTracking(request)
	case KindUpdateAssignee:
		return p.handleUpdateAssignee(request)
	case KindCreateIssue:
		return p.handleCreateIssue(request)
	default:
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}
}

func (p *Pool) handleStartIssue(request Request) Result {
	if request.StartIssue == nil || strings.TrimSpace(request.StartIssue.Result.Issue.Key) == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	outcomes := startworkflow.ApplyJiraActions(ctx, p.client, request.StartIssue.Result, request.StartIssue.BranchSucceeded)
	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		StartIssue: &StartIssueResult{
			Key:      request.StartIssue.Result.Issue.Key,
			Outcomes: outcomes,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleGetCurrentUser(request Request) Result {
	if request.GetCurrentUser == nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	user, err := p.client.CurrentUser(ctx)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetCurrentUser: &GetCurrentUserResult{
			User:     user,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleGetIssueLinkTypes(request Request) Result {
	if request.GetIssueLinkTypes == nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	types, err := p.client.GetIssueLinkTypes(ctx)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetIssueLinkTypes: &GetIssueLinkTypesResult{
			Types:    types,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleCreateIssueLink(request Request) Result {
	if request.CreateIssueLink == nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	createRequest := request.CreateIssueLink.Request
	if err := p.client.CreateIssueLink(ctx, createRequest); err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		CreateIssueLink: &CreateIssueLinkResult{
			Request:  createRequest,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleDeleteIssueLink(request Request) Result {
	if request.DeleteIssueLink == nil || request.DeleteIssueLink.LinkID == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	if err := p.client.DeleteIssueLink(ctx, request.DeleteIssueLink.LinkID); err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		DeleteIssueLink: &DeleteIssueLinkResult{
			IssueKey: request.DeleteIssueLink.IssueKey,
			LinkID:   request.DeleteIssueLink.LinkID,
			Target:   request.DeleteIssueLink.Target,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleGetWorklogs(request Request) Result {
	if request.GetWorklogs == nil || request.GetWorklogs.Key == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	worklogs, err := p.client.GetWorklogs(ctx, request.GetWorklogs.Key, request.GetWorklogs.MaxResults)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetWorklogs: &GetWorklogsResult{
			Key:      request.GetWorklogs.Key,
			Worklogs: worklogs,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleAddWorklog(request Request) Result {
	if request.AddWorklog == nil || request.AddWorklog.Key == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	worklog, err := p.client.AddWorklog(ctx, request.AddWorklog.Key, request.AddWorklog.Request)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		AddWorklog: &AddWorklogResult{
			Key:      request.AddWorklog.Key,
			Worklog:  worklog,
			Request:  request.AddWorklog.Request,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleUpdateWorklog(request Request) Result {
	if request.UpdateWorklog == nil || request.UpdateWorklog.Key == "" || request.UpdateWorklog.Request.ID == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	updateRequest := request.UpdateWorklog.Request
	worklog, err := p.client.UpdateWorklog(ctx, request.UpdateWorklog.Key, updateRequest)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		UpdateWorklog: &UpdateWorklogResult{
			Key:      request.UpdateWorklog.Key,
			Worklog:  worklog,
			Request:  updateRequest,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleDeleteWorklog(request Request) Result {
	if request.DeleteWorklog == nil || request.DeleteWorklog.Key == "" || request.DeleteWorklog.WorklogID == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	if err := p.client.DeleteWorklog(ctx, request.DeleteWorklog.Key, request.DeleteWorklog.WorklogID); err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		DeleteWorklog: &DeleteWorklogResult{
			Key:       request.DeleteWorklog.Key,
			WorklogID: request.DeleteWorklog.WorklogID,
			SyncedAt:  time.Now(),
		},
	}
}

func (p *Pool) handleGetBoards(request Request) Result {
	if request.GetBoards == nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	page, err := p.client.GetBoards(ctx, request.GetBoards.ProjectKey, request.GetBoards.StartAt, request.GetBoards.MaxResults)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetBoards: &GetBoardsResult{
			ProjectKey: request.GetBoards.ProjectKey,
			Page:       page,
			SyncedAt:   time.Now(),
		},
	}
}

func (p *Pool) handleGetBoardSprints(request Request) Result {
	if request.GetBoardSprints == nil || request.GetBoardSprints.BoardID <= 0 {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	page, err := p.client.GetBoardSprints(ctx, request.GetBoardSprints.BoardID, request.GetBoardSprints.States, request.GetBoardSprints.StartAt, request.GetBoardSprints.MaxResults)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetBoardSprints: &GetBoardSprintsResult{
			BoardID:  request.GetBoardSprints.BoardID,
			Page:     page,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleMoveIssuesToSprint(request Request) Result {
	if request.MoveIssuesToSprint == nil || request.MoveIssuesToSprint.Sprint.ID <= 0 || len(request.MoveIssuesToSprint.IssueKeys) == 0 {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	issueKeys := append([]string{}, request.MoveIssuesToSprint.IssueKeys...)
	err := p.client.MoveIssuesToSprint(ctx, request.MoveIssuesToSprint.Sprint.ID, issueKeys)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		MoveIssuesToSprint: &MoveIssuesToSprintResult{
			Sprint:    request.MoveIssuesToSprint.Sprint,
			IssueKeys: issueKeys,
			SyncedAt:  time.Now(),
		},
	}
}

func (p *Pool) handleGetCreateIssueTypes(request Request) Result {
	if request.GetCreateIssueTypes == nil || request.GetCreateIssueTypes.ProjectKey == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	issueTypes, err := p.client.GetCreateIssueTypes(ctx, request.GetCreateIssueTypes.ProjectKey)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetCreateIssueTypes: &GetCreateIssueTypesResult{
			ProjectKey: request.GetCreateIssueTypes.ProjectKey,
			IssueTypes: issueTypes,
			SyncedAt:   time.Now(),
		},
	}
}

func (p *Pool) handleGetCreateFields(request Request) Result {
	if request.GetCreateFields == nil || request.GetCreateFields.ProjectKey == "" || request.GetCreateFields.IssueTypeID == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	fields, err := p.client.GetCreateFields(ctx, request.GetCreateFields.ProjectKey, request.GetCreateFields.IssueTypeID)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetCreateFields: &GetCreateFieldsResult{
			ProjectKey:  request.GetCreateFields.ProjectKey,
			IssueTypeID: request.GetCreateFields.IssueTypeID,
			Fields:      fields,
			SyncedAt:    time.Now(),
		},
	}
}

func (p *Pool) handleSearchFieldOptions(request Request) Result {
	if request.SearchFieldOptions == nil || request.SearchFieldOptions.FieldID == "" || request.SearchFieldOptions.AutoCompleteURL == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	options, err := p.client.SearchFieldOptions(ctx, request.SearchFieldOptions.AutoCompleteURL, request.SearchFieldOptions.Query, request.SearchFieldOptions.MaxResults)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		SearchFieldOptions: &SearchFieldOptionsResult{
			FieldID:  request.SearchFieldOptions.FieldID,
			Query:    request.SearchFieldOptions.Query,
			Options:  options,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleCreateIssue(request Request) Result {
	if request.CreateIssue == nil || request.CreateIssue.ProjectKey == "" || request.CreateIssue.IssueTypeID == "" || request.CreateIssue.Summary == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	issue, err := p.client.CreateIssue(ctx, jira.CreateIssueRequest{
		ProjectKey:  request.CreateIssue.ProjectKey,
		IssueTypeID: request.CreateIssue.IssueTypeID,
		ParentKey:   request.CreateIssue.ParentKey,
		Summary:     request.CreateIssue.Summary,
		Description: request.CreateIssue.Description,
		Fields:      request.CreateIssue.Fields,
	})
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		CreateIssue: &CreateIssueResult{
			Issue:    issue,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleGetEditMetadata(request Request) Result {
	if request.GetEditMetadata == nil || request.GetEditMetadata.Key == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	metadata, err := p.client.GetEditMetadata(ctx, request.GetEditMetadata.Key)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetEditMetadata: &GetEditMetadataResult{
			Key:      request.GetEditMetadata.Key,
			Metadata: metadata,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleUpdateSummary(request Request) Result {
	if request.UpdateSummary == nil || request.UpdateSummary.Key == "" || request.UpdateSummary.Summary == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	err := p.client.UpdateSummary(ctx, request.UpdateSummary.Key, request.UpdateSummary.Summary)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		UpdateSummary: &UpdateSummaryResult{
			Key:      request.UpdateSummary.Key,
			Summary:  request.UpdateSummary.Summary,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleUpdateDescription(request Request) Result {
	if request.UpdateDescription == nil || request.UpdateDescription.Key == "" || request.UpdateDescription.Description == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	err := p.client.UpdateDescription(ctx, request.UpdateDescription.Key, request.UpdateDescription.Description)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		UpdateDescription: &UpdateDescriptionResult{
			Key:         request.UpdateDescription.Key,
			Description: request.UpdateDescription.Description,
			SyncedAt:    time.Now(),
		},
	}
}

func (p *Pool) handleGetTransitions(request Request) Result {
	if request.GetTransitions == nil || request.GetTransitions.Key == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	transitions, err := p.client.GetTransitions(ctx, request.GetTransitions.Key)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetTransitions: &GetTransitionsResult{
			Key:         request.GetTransitions.Key,
			Transitions: transitions,
			SyncedAt:    time.Now(),
		},
	}
}

func (p *Pool) handleTransitionIssue(request Request) Result {
	if request.TransitionIssue == nil || request.TransitionIssue.Key == "" || request.TransitionIssue.TransitionID == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	err := p.client.TransitionIssue(ctx, request.TransitionIssue.Key, jira.TransitionIssueRequest{
		TransitionID: request.TransitionIssue.TransitionID,
		Fields:       append([]jira.TransitionFieldValue(nil), request.TransitionIssue.Fields...),
	})
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		TransitionIssue: &TransitionIssueResult{
			Key:      request.TransitionIssue.Key,
			ToStatus: request.TransitionIssue.ToStatus,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleExpandIssues(request Request) Result {
	if request.ExpandIssues == nil || request.ExpandIssues.ParentKey == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	maxResults := request.ExpandIssues.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}
	mode := request.ExpandIssues.Mode
	if mode == "" {
		mode = ExpandModeOpen
	}
	issues, err := p.client.SearchIssues(ctx, expandIssuesJQL(request.ExpandIssues.ParentKey, mode), maxResults)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		ExpandIssues: &ExpandIssuesResult{
			ParentKey: request.ExpandIssues.ParentKey,
			Mode:      mode,
			Issues:    issues,
			SyncedAt:  time.Now(),
		},
	}
}

func (p *Pool) handleUpdatePriority(request Request) Result {
	if request.UpdatePriority == nil || request.UpdatePriority.Key == "" || (request.UpdatePriority.Priority.ID == "" && request.UpdatePriority.Priority.Name == "") {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	err := p.client.UpdatePriority(ctx, request.UpdatePriority.Key, request.UpdatePriority.Priority)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		UpdatePriority: &UpdatePriorityResult{
			Key:      request.UpdatePriority.Key,
			Priority: request.UpdatePriority.Priority,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleUpdateLabels(request Request) Result {
	if request.UpdateLabels == nil || request.UpdateLabels.Key == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	labels := append([]string{}, request.UpdateLabels.Labels...)
	err := p.client.UpdateLabels(ctx, request.UpdateLabels.Key, labels)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		UpdateLabels: &UpdateLabelsResult{
			Key:      request.UpdateLabels.Key,
			Labels:   labels,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleUpdateComponents(request Request) Result {
	if request.UpdateComponents == nil || request.UpdateComponents.Key == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	components := append([]jira.FieldOption{}, request.UpdateComponents.Components...)
	err := p.client.UpdateComponents(ctx, request.UpdateComponents.Key, components)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		UpdateComponents: &UpdateComponentsResult{
			Key:        request.UpdateComponents.Key,
			Components: components,
			SyncedAt:   time.Now(),
		},
	}
}

func (p *Pool) handleUpdateEditField(request Request) Result {
	if request.UpdateEditField == nil || request.UpdateEditField.Key == "" || request.UpdateEditField.Value.FieldID == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	value := request.UpdateEditField.Value
	err := p.client.UpdateEditField(ctx, request.UpdateEditField.Key, value)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		UpdateEditField: &UpdateEditFieldResult{
			Key:      request.UpdateEditField.Key,
			Field:    request.UpdateEditField.Field,
			Value:    value,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleUpdateParent(request Request) Result {
	if request.UpdateParent == nil || strings.TrimSpace(request.UpdateParent.Key) == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}
	parentRequest := request.UpdateParent.Request
	if !parentRequest.Clear && strings.TrimSpace(parentRequest.ParentKey) == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	err := p.client.UpdateParent(ctx, request.UpdateParent.Key, parentRequest)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		UpdateParent: &UpdateParentResult{
			Key:      request.UpdateParent.Key,
			Request:  parentRequest,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleUpdateTimeTracking(request Request) Result {
	if request.UpdateTimeTracking == nil || strings.TrimSpace(request.UpdateTimeTracking.Key) == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}
	timeRequest := request.UpdateTimeTracking.Request
	if strings.TrimSpace(timeRequest.OriginalEstimate) == "" && strings.TrimSpace(timeRequest.RemainingEstimate) == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	err := p.client.UpdateTimeTracking(ctx, request.UpdateTimeTracking.Key, timeRequest)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		UpdateTimeTracking: &UpdateTimeTrackingResult{
			Key:      request.UpdateTimeTracking.Key,
			Request:  timeRequest,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleUpdateAssignee(request Request) Result {
	if request.UpdateAssignee == nil || request.UpdateAssignee.Key == "" || request.UpdateAssignee.Assignee.AccountID == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	err := p.client.UpdateAssignee(ctx, request.UpdateAssignee.Key, request.UpdateAssignee.Assignee)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		UpdateAssignee: &UpdateAssigneeResult{
			Key:      request.UpdateAssignee.Key,
			Assignee: request.UpdateAssignee.Assignee,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleSearchUsers(request Request) Result {
	if request.SearchUsers == nil || request.SearchUsers.Query == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	issueKey := strings.TrimSpace(request.SearchUsers.IssueKey)
	var users []jira.User
	var err error
	if issueKey != "" {
		users, err = p.client.SearchAssignableUsers(ctx, issueKey, request.SearchUsers.Query, request.SearchUsers.MaxResults)
	} else {
		users, err = p.client.SearchUsers(ctx, request.SearchUsers.Query, request.SearchUsers.MaxResults)
	}
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		SearchUsers: &SearchUsersResult{
			Query:    request.SearchUsers.Query,
			IssueKey: issueKey,
			Users:    users,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) withMissingParents(ctx context.Context, issues []jira.Issue) []jira.Issue {
	if len(issues) == 0 {
		return issues
	}
	seen := make(map[string]bool, len(issues))
	for _, issue := range issues {
		seen[issue.Key] = true
	}

	parentKeys := make([]string, 0)
	parentSeen := make(map[string]bool)
	for _, issue := range issues {
		if issue.ParentKey == "" || seen[issue.ParentKey] || parentSeen[issue.ParentKey] {
			continue
		}
		parentKeys = append(parentKeys, issue.ParentKey)
		parentSeen[issue.ParentKey] = true
	}
	if len(parentKeys) == 0 {
		return issues
	}

	parents := make(map[string]jira.Issue, len(parentKeys))
	for _, key := range parentKeys {
		detail, err := p.client.GetIssue(ctx, key)
		if err != nil || detail.Key == "" {
			continue
		}
		parents[key] = detail.Issue
	}
	if len(parents) == 0 {
		return issues
	}

	enriched := make([]jira.Issue, 0, len(issues)+len(parents))
	inserted := make(map[string]bool, len(parents))
	for _, issue := range issues {
		if parent, ok := parents[issue.ParentKey]; ok && !inserted[parent.Key] {
			enriched = append(enriched, parent)
			inserted[parent.Key] = true
		}
		enriched = append(enriched, issue)
	}
	return enriched
}

func withKnownSubtasks(issues []jira.Issue) []jira.Issue {
	if len(issues) == 0 {
		return issues
	}
	seen := make(map[string]bool, len(issues))
	for _, issue := range issues {
		seen[issue.Key] = true
	}
	enriched := append([]jira.Issue(nil), issues...)
	for _, issue := range issues {
		for _, subtask := range issue.Subtasks {
			if subtask.Key == "" || seen[subtask.Key] {
				continue
			}
			enriched = append(enriched, subtask)
			seen[subtask.Key] = true
		}
	}
	return enriched
}

func (p *Pool) handleAddComment(request Request) Result {
	if request.AddComment == nil || request.AddComment.Key == "" || request.AddComment.Body == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	comment, err := p.client.AddComment(ctx, request.AddComment.Key, request.AddComment.Body, request.AddComment.Mentions)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		AddComment: &AddCommentResult{
			Key:      request.AddComment.Key,
			Comment:  comment,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleUpdateComment(request Request) Result {
	if request.UpdateComment == nil || request.UpdateComment.Key == "" || request.UpdateComment.CommentID == "" || request.UpdateComment.Body == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	comment, err := p.client.UpdateComment(ctx, request.UpdateComment.Key, request.UpdateComment.CommentID, request.UpdateComment.Body, request.UpdateComment.Mentions)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		UpdateComment: &UpdateCommentResult{
			Key:      request.UpdateComment.Key,
			Comment:  comment,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleGetComments(request Request) Result {
	if request.GetComments == nil || request.GetComments.Key == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	comments, err := p.client.GetComments(ctx, request.GetComments.Key, request.GetComments.MaxResults)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetComments: &GetCommentsResult{
			Key:      request.GetComments.Key,
			Comments: comments,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleSearchIssues(request Request) Result {
	if request.SearchIssues == nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	issues, err := p.client.SearchIssues(
		ctx,
		request.SearchIssues.JQL,
		request.SearchIssues.MaxResults,
	)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}
	issues = p.withMissingParents(ctx, issues)
	issues = withKnownSubtasks(issues)
	if request.SearchIssues.IncludeChildren {
		issues = p.withChildIssues(ctx, issues, request.SearchIssues.MaxResults)
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		SearchIssues: &SearchIssuesResult{
			Issues:   issues,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) withChildIssues(ctx context.Context, issues []jira.Issue, maxResults int) []jira.Issue {
	parentKeys := childLookupParentKeys(issues)
	if len(parentKeys) == 0 {
		return issues
	}
	if maxResults <= 0 {
		maxResults = 50
	}
	children, err := p.client.SearchIssues(ctx, childLookupJQL(parentKeys), maxResults)
	if err != nil {
		return issues
	}
	if len(children) == 0 {
		return issues
	}
	seen := make(map[string]bool, len(issues)+len(children))
	for _, issue := range issues {
		seen[issue.Key] = true
	}
	enriched := append([]jira.Issue(nil), issues...)
	for _, child := range children {
		if child.Key == "" || child.ParentKey == "" || seen[child.Key] {
			continue
		}
		enriched = append(enriched, child)
		seen[child.Key] = true
	}
	return enriched
}

func childLookupParentKeys(issues []jira.Issue) []string {
	keys := make([]string, 0, len(issues))
	seen := make(map[string]bool, len(issues))
	for _, issue := range issues {
		if issue.Key == "" || issue.IsSubtask || seen[issue.Key] {
			continue
		}
		keys = append(keys, issue.Key)
		seen[issue.Key] = true
	}
	return keys
}

func childLookupJQL(parentKeys []string) string {
	return fmt.Sprintf("parent in (%s) ORDER BY key ASC", joinJQLKeys(parentKeys))
}

func expandIssuesJQL(parentKey string, mode ExpandMode) string {
	base := fmt.Sprintf("parent = %s", parentKey)
	if mode == ExpandModeOpen {
		base += " AND statusCategory != Done"
	}
	return base + " ORDER BY key ASC"
}

func joinJQLKeys(keys []string) string {
	result := ""
	for index, key := range keys {
		if index > 0 {
			result += ", "
		}
		result += key
	}
	return result
}

func (p *Pool) handleGetIssue(request Request) Result {
	if request.GetIssue == nil || request.GetIssue.Key == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	detail, err := p.client.GetIssue(ctx, request.GetIssue.Key)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetIssue: &GetIssueResult{
			Key:      request.GetIssue.Key,
			Detail:   detail,
			SyncedAt: time.Now(),
		},
	}
}
