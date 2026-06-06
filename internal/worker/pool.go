package worker

import (
	"context"
	"errors"
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

const KindSearchIssues Kind = "search_issues"

type IssueSearcher interface {
	SearchIssues(ctx context.Context, jql string, maxResults int) ([]jira.Issue, error)
}

type Request struct {
	ID      int
	Kind    Kind
	Timeout time.Duration

	SearchIssues *SearchIssuesRequest
}

type SearchIssuesRequest struct {
	JQL        string
	MaxResults int
}

type Result struct {
	ID   int
	Kind Kind
	Err  error

	SearchIssues *SearchIssuesResult
}

type SearchIssuesResult struct {
	Issues   []jira.Issue
	SyncedAt time.Time
}

type Option func(*Pool)

type Pool struct {
	client IssueSearcher

	workerCount int
	queueSize   int

	engine    *ants.Pool
	admission chan struct{}
	results   chan Result
	done      chan struct{}
	initErr   error
	wg        sync.WaitGroup
	stopOnce  sync.Once
	mu        sync.Mutex
	closed    bool
}

func NewPool(client IssueSearcher, options ...Option) *Pool {
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
	pool.admission = make(chan struct{}, capacity)
	pool.engine, pool.initErr = ants.NewPool(pool.workerCount)
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
	defer p.mu.Unlock()

	if p.initErr != nil {
		return p.initErr
	}
	if p.closed {
		return ErrPoolClosed
	}

	select {
	case p.admission <- struct{}{}:
	default:
		return ErrQueueFull
	}

	p.wg.Add(1)
	go p.submitToEngine(request)
	return nil
}

func (p *Pool) Results() <-chan Result {
	return p.results
}

func (p *Pool) Stop() {
	p.stopOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		close(p.done)
		p.mu.Unlock()

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

func (p *Pool) submitToEngine(request Request) {
	err := p.engine.Submit(func() {
		defer p.wg.Done()
		defer p.releaseAdmission()
		defer func() {
			if recovered := recover(); recovered != nil {
				p.send(Result{ID: request.ID, Kind: request.Kind, Err: ErrWorkerPanic})
			}
		}()

		p.send(p.handle(request))
	})
	if err != nil {
		p.wg.Done()
		p.releaseAdmission()
		p.send(Result{ID: request.ID, Kind: request.Kind, Err: mapAntsError(err)})
	}
}

func (p *Pool) releaseAdmission() {
	select {
	case <-p.admission:
	default:
	}
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
	default:
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
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

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		SearchIssues: &SearchIssuesResult{
			Issues:   issues,
			SyncedAt: time.Now(),
		},
	}
}
