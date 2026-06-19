package events

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/jcharette/jira-tui/internal/jira"
)

const topicAll = "jira-tui.events"

type Type string

const (
	TypeJiraTicketNew             Type = "jira.ticket.new"
	TypeJiraTicketUpdated         Type = "jira.ticket.updated"
	TypeJiraViewHydrated          Type = "jira.view.hydrated"
	TypeJiraViewRefreshStarted    Type = "jira.view.refresh.started"
	TypeJiraViewRefreshCompleted  Type = "jira.view.refresh.completed"
	TypeJiraViewRefreshFailed     Type = "jira.view.refresh.failed"
	TypeJiraCacheRefreshRequested Type = "jira.cache.refresh.requested"
	TypeJiraCacheRefreshCompleted Type = "jira.cache.refresh.completed"
	TypeJiraCacheRefreshFailed    Type = "jira.cache.refresh.failed"
	TypeAITaskRequested           Type = "ai.task.requested"
	TypeAITaskProgress            Type = "ai.task.progress"
	TypeAITaskCompleted           Type = "ai.task.completed"
	TypeAITaskFailed              Type = "ai.task.failed"
)

type Event struct {
	ID        string          `json:"id"`
	At        time.Time       `json:"at"`
	Type      Type            `json:"type"`
	Source    string          `json:"source,omitempty"`
	DedupeKey string          `json:"dedupe_key,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type TicketPayload struct {
	IssueKey      string      `json:"issue_key"`
	Previous      *jira.Issue `json:"previous,omitempty"`
	Current       jira.Issue  `json:"current"`
	ChangedFields []string    `json:"changed_fields,omitempty"`
	ViewName      string      `json:"view_name,omitempty"`
	JQL           string      `json:"jql,omitempty"`
	SyncedAt      time.Time   `json:"synced_at,omitempty"`
}

type AIProvider string

const (
	AIProviderAuto   AIProvider = "auto"
	AIProviderClaude AIProvider = "claude"
	AIProviderCodex  AIProvider = "codex"
)

type AIOperation string

const (
	AIOperationTicketPlan         AIOperation = "ticket_plan"
	AIOperationTicketAssist       AIOperation = "ticket_assist"
	AIOperationInlineAssist       AIOperation = "inline_assist"
	AIOperationCreateDraft        AIOperation = "create_draft"
	AIOperationRefineDraft        AIOperation = "refine_draft"
	AIOperationGenerateJQL        AIOperation = "generate_jql"
	AIOperationCodeReview         AIOperation = "code_review"
	AIOperationImplementationPlan AIOperation = "implementation_plan"
)

type AITaskPayload struct {
	RequestID         int         `json:"request_id,omitempty"`
	Operation         AIOperation `json:"operation"`
	PreferredProvider AIProvider  `json:"preferred_provider,omitempty"`
	Provider          AIProvider  `json:"provider,omitempty"`
	IssueKey          string      `json:"issue_key,omitempty"`
	ProjectKey        string      `json:"project_key,omitempty"`
	CancellationKey   string      `json:"cancellation_key,omitempty"`
	PromptBytes       int         `json:"prompt_bytes,omitempty"`
	ResultBytes       int         `json:"result_bytes,omitempty"`
	ProgressKind      string      `json:"progress_kind,omitempty"`
	ProgressBytes     int         `json:"progress_bytes,omitempty"`
	Error             string      `json:"error,omitempty"`
}

type Stream struct {
	pubsub *gochannel.GoChannel
	now    func() time.Time
}

type Option func(*Stream)

func WithNow(now func() time.Time) Option {
	return func(s *Stream) {
		if now != nil {
			s.now = now
		}
	}
}

func NewStream(options ...Option) *Stream {
	stream := &Stream{
		pubsub: gochannel.NewGoChannel(gochannel.Config{OutputChannelBuffer: 64}, watermill.NopLogger{}),
		now:    time.Now,
	}
	for _, option := range options {
		option(stream)
	}
	return stream
}

func (s *Stream) Publish(ctx context.Context, event Event) error {
	if s == nil || s.pubsub == nil {
		return errors.New("event stream is closed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.ID == "" {
		event.ID = watermill.NewUUID()
	}
	if event.At.IsZero() {
		event.At = s.now()
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return s.pubsub.Publish(topicAll, message.NewMessage(event.ID, payload))
}

func (s *Stream) Subscribe(ctx context.Context) (<-chan Event, error) {
	if s == nil || s.pubsub == nil {
		return nil, errors.New("event stream is closed")
	}
	messages, err := s.pubsub.Subscribe(ctx, topicAll)
	if err != nil {
		return nil, err
	}
	events := make(chan Event)
	go func() {
		defer close(events)
		for msg := range messages {
			var event Event
			if err := json.Unmarshal(msg.Payload, &event); err != nil {
				msg.Nack()
				continue
			}
			select {
			case events <- event:
				msg.Ack()
			case <-ctx.Done():
				msg.Nack()
				return
			}
		}
	}()
	return events, nil
}

func (s *Stream) Close() error {
	if s == nil || s.pubsub == nil {
		return nil
	}
	err := s.pubsub.Close()
	s.pubsub = nil
	return err
}
