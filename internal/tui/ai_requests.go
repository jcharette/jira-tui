package tui

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/claude"
	"github.com/jcharette/jira-tui/internal/events"
)

type aiTaskRequest struct {
	RequestID         int
	Operation         events.AIOperation
	PreferredProvider events.AIProvider
	IssueKey          string
	ProjectKey        string
	Prompt            string
	Progress          chan<- claude.Event
	ResultMsg         func(int, string, string, error) tea.Msg
}

func (m Model) submitAIRequest(ctx context.Context, request aiTaskRequest) tea.Cmd {
	runner := m.claudeRunner
	if runner == nil {
		runner = claude.LocalRunner{}
	}
	provider := events.AIProviderClaude
	preferredProvider := request.PreferredProvider
	if preferredProvider == "" {
		preferredProvider = events.AIProviderAuto
	}
	config := claude.Config{
		Enabled: m.claudeConfig.Enabled,
		Command: m.claudeConfig.Command,
		Timeout: m.claudeConfig.Timeout,
	}
	return func() tea.Msg {
		defer closeClaudeEvents(request.Progress)
		m.publishAITaskEvent(events.TypeAITaskRequested, request, aiTaskEventOptions{
			preferredProvider: preferredProvider,
			provider:          provider,
			promptBytes:       len([]byte(request.Prompt)),
		})
		result, err := runner.Run(ctx, claude.Request{
			Config: config,
			Prompt: request.Prompt,
			Progress: func(event claude.Event) {
				if strings.TrimSpace(event.Text) == "" {
					return
				}
				m.publishAITaskEvent(events.TypeAITaskProgress, request, aiTaskEventOptions{
					preferredProvider: preferredProvider,
					provider:          provider,
					progress:          event,
				})
				if request.Progress != nil {
					select {
					case request.Progress <- event:
					case <-ctx.Done():
					}
				}
			},
		})
		if err != nil {
			m.publishAITaskEvent(events.TypeAITaskFailed, request, aiTaskEventOptions{
				preferredProvider: preferredProvider,
				provider:          provider,
				err:               err,
			})
			return request.ResultMsg(request.RequestID, request.IssueKey, "", err)
		}
		m.publishAITaskEvent(events.TypeAITaskCompleted, request, aiTaskEventOptions{
			preferredProvider: preferredProvider,
			provider:          provider,
			resultBytes:       len([]byte(result.Text)),
		})
		return request.ResultMsg(request.RequestID, request.IssueKey, result.Text, nil)
	}
}

type aiTaskEventOptions struct {
	preferredProvider events.AIProvider
	provider          events.AIProvider
	promptBytes       int
	resultBytes       int
	progress          claude.Event
	err               error
}

func (m Model) publishAITaskEvent(eventType events.Type, request aiTaskRequest, options aiTaskEventOptions) {
	if m.eventStream == nil {
		return
	}
	payload := events.AITaskPayload{
		RequestID:         request.RequestID,
		Operation:         request.Operation,
		PreferredProvider: options.preferredProvider,
		Provider:          options.provider,
		IssueKey:          request.IssueKey,
		ProjectKey:        request.ProjectKey,
		CancellationKey:   aiTaskCancellationKey(request),
		PromptBytes:       options.promptBytes,
		ResultBytes:       options.resultBytes,
		ProgressKind:      options.progress.Kind,
		ProgressBytes:     len([]byte(options.progress.Text)),
	}
	if options.err != nil {
		payload.Error = options.err.Error()
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = m.eventStream.Publish(context.Background(), events.Event{
		Type:      eventType,
		Source:    "ai",
		DedupeKey: payload.CancellationKey,
		Payload:   data,
	})
}

func aiTaskCancellationKey(request aiTaskRequest) string {
	if strings.TrimSpace(request.IssueKey) != "" {
		return strings.TrimSpace(request.IssueKey) + ":" + string(request.Operation)
	}
	if strings.TrimSpace(request.ProjectKey) != "" {
		return strings.TrimSpace(request.ProjectKey) + ":" + string(request.Operation)
	}
	return strconv.Itoa(request.RequestID) + ":" + string(request.Operation)
}
