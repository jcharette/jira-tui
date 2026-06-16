package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jon/jira-tui/internal/jira"
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
	KindGetIssue            Kind = "get_issue"
	KindGetComments         Kind = "get_comments"
	KindAddComment          Kind = "add_comment"
	KindSearchUsers         Kind = "search_users"
	KindExpandIssues        Kind = "expand_issues"
	KindGetTransitions      Kind = "get_transitions"
	KindTransitionIssue     Kind = "transition_issue"
	KindGetEditMetadata     Kind = "get_edit_metadata"
	KindGetCreateIssueTypes Kind = "get_create_issue_types"
	KindGetCreateFields     Kind = "get_create_fields"
	KindUpdateSummary       Kind = "update_summary"
	KindUpdateDescription   Kind = "update_description"
	KindUpdatePriority      Kind = "update_priority"
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
	GetIssue(ctx context.Context, key string) (jira.IssueDetail, error)
	GetComments(ctx context.Context, key string, maxResults int) ([]jira.Comment, error)
	AddComment(ctx context.Context, key string, body string, mentions []jira.Mention) (jira.Comment, error)
	SearchUsers(ctx context.Context, query string, maxResults int) ([]jira.User, error)
	GetTransitions(ctx context.Context, key string) ([]jira.Transition, error)
	TransitionIssue(ctx context.Context, key string, transitionID string) error
	GetEditMetadata(ctx context.Context, key string) (jira.EditMetadata, error)
	GetCreateIssueTypes(ctx context.Context, projectKey string) ([]jira.CreateIssueType, error)
	GetCreateFields(ctx context.Context, projectKey string, issueTypeID string) ([]jira.CreateField, error)
	CreateIssue(ctx context.Context, request jira.CreateIssueRequest) (jira.Issue, error)
	UpdateSummary(ctx context.Context, key string, summary string) error
	UpdateDescription(ctx context.Context, key string, description string) error
	UpdatePriority(ctx context.Context, key string, priority jira.FieldOption) error
	UpdateAssignee(ctx context.Context, key string, assignee jira.User) error
}

type Request struct {
	ID          int
	Kind        Kind
	Timeout     time.Duration
	Priority    Priority
	CoalesceKey string

	SearchIssues        *SearchIssuesRequest
	GetIssue            *GetIssueRequest
	GetComments         *GetCommentsRequest
	AddComment          *AddCommentRequest
	SearchUsers         *SearchUsersRequest
	ExpandIssues        *ExpandIssuesRequest
	GetTransitions      *GetTransitionsRequest
	TransitionIssue     *TransitionIssueRequest
	GetEditMetadata     *GetEditMetadataRequest
	GetCreateIssueTypes *GetCreateIssueTypesRequest
	GetCreateFields     *GetCreateFieldsRequest
	UpdateSummary       *UpdateSummaryRequest
	UpdateDescription   *UpdateDescriptionRequest
	UpdatePriority      *UpdatePriorityRequest
	UpdateAssignee      *UpdateAssigneeRequest
	CreateIssue         *CreateIssueRequest
}

type SearchIssuesRequest struct {
	JQL        string
	MaxResults int
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

type SearchUsersRequest struct {
	Query      string
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

type UpdateAssigneeRequest struct {
	Key      string
	Assignee jira.User
}

type CreateIssueRequest struct {
	ProjectKey  string
	IssueTypeID string
	Summary     string
	Description string
	Fields      []jira.CreateIssueFieldValue
}

type Result struct {
	ID   int
	Kind Kind
	Err  error

	SearchIssues        *SearchIssuesResult
	GetIssue            *GetIssueResult
	GetComments         *GetCommentsResult
	AddComment          *AddCommentResult
	SearchUsers         *SearchUsersResult
	ExpandIssues        *ExpandIssuesResult
	GetTransitions      *GetTransitionsResult
	TransitionIssue     *TransitionIssueResult
	GetEditMetadata     *GetEditMetadataResult
	GetCreateIssueTypes *GetCreateIssueTypesResult
	GetCreateFields     *GetCreateFieldsResult
	UpdateSummary       *UpdateSummaryResult
	UpdateDescription   *UpdateDescriptionResult
	UpdatePriority      *UpdatePriorityResult
	UpdateAssignee      *UpdateAssigneeResult
	CreateIssue         *CreateIssueResult
}

type SearchIssuesResult struct {
	Issues   []jira.Issue
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

type SearchUsersResult struct {
	Query    string
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
	case KindGetIssue:
		return p.handleGetIssue(request)
	case KindGetComments:
		return p.handleGetComments(request)
	case KindAddComment:
		return p.handleAddComment(request)
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
	case KindUpdateSummary:
		return p.handleUpdateSummary(request)
	case KindUpdateDescription:
		return p.handleUpdateDescription(request)
	case KindUpdatePriority:
		return p.handleUpdatePriority(request)
	case KindUpdateAssignee:
		return p.handleUpdateAssignee(request)
	case KindCreateIssue:
		return p.handleCreateIssue(request)
	default:
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
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

	err := p.client.TransitionIssue(ctx, request.TransitionIssue.Key, request.TransitionIssue.TransitionID)
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

	users, err := p.client.SearchUsers(ctx, request.SearchUsers.Query, request.SearchUsers.MaxResults)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		SearchUsers: &SearchUsersResult{
			Query:    request.SearchUsers.Query,
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
	issues = p.withChildIssues(ctx, issues, request.SearchIssues.MaxResults)

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
