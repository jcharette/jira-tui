package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

func (m Model) submitIssueSearch(requestID int, priority worker.Priority) tea.Cmd {
	return func() tea.Msg {
		err := m.workers.Submit(worker.Request{
			ID:          requestID,
			Kind:        worker.KindSearchIssues,
			Timeout:     m.requestTimeout,
			Priority:    priority,
			CoalesceKey: "search:" + m.activeIssueViewCacheKey(m.jql),
			SearchIssues: &worker.SearchIssuesRequest{
				JQL:             m.jql,
				MaxResults:      maxIssues,
				IncludeChildren: m.activeViewIncludeChildren(),
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

func (m Model) submitPlanningBoards(requestID int) tea.Cmd {
	return func() tea.Msg {
		projectKey := strings.TrimSpace(m.planningProjectKey)
		if projectKey == "" {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindGetBoards,
					Err:  worker.ErrInvalidRequest,
				},
			}
		}
		err := m.workers.Submit(worker.Request{
			ID:          requestID,
			Kind:        worker.KindGetBoards,
			Timeout:     m.requestTimeout,
			Priority:    worker.PriorityBackground,
			CoalesceKey: "boards:" + projectKey,
			GetBoards: &worker.GetBoardsRequest{
				ProjectKey: projectKey,
				StartAt:    0,
				MaxResults: planningMetadataPageSize,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindGetBoards,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindGetBoards, id: requestID, key: projectKey}
	}
}

func (m Model) submitPlanningSprints(requestID int, boardID int) tea.Cmd {
	return func() tea.Msg {
		if boardID <= 0 {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindGetBoardSprints,
					Err:  worker.ErrInvalidRequest,
				},
			}
		}
		err := m.workers.Submit(worker.Request{
			ID:          requestID,
			Kind:        worker.KindGetBoardSprints,
			Timeout:     m.requestTimeout,
			Priority:    worker.PriorityBackground,
			CoalesceKey: fmt.Sprintf("board-sprints:%d", boardID),
			GetBoardSprints: &worker.GetBoardSprintsRequest{
				BoardID:    boardID,
				States:     []string{"active", "future"},
				StartAt:    0,
				MaxResults: planningMetadataPageSize,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindGetBoardSprints,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindGetBoardSprints, id: requestID, key: fmt.Sprintf("%d", boardID)}
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
	m.hydrateIssueDetail(selected.Key)
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

	m.hydrateIssueComments(selected.Key)
	if _, ok := m.comments[selected.Key]; ok && m.isIssueCommentsFresh(selected.Key) {
		m.commentsLoading = false
		m.commentsErr = nil
		m.commentsRequestKey = ""
	} else if !(m.commentsLoading && m.commentsRequestKey == selected.Key) {
		status := "miss"
		if _, ok := m.comments[selected.Key]; ok {
			status = "stale"
		}
		m.recordDiagnosticEvent(diagnosticKindCache, "issue_comments", status, selected.Key)
		m.nextRequestID++
		m.activeCommentsReqID = m.nextRequestID
		m.commentsRequestKey = selected.Key
		m.commentsLoading = true
		m.commentsErr = nil
		cmds = append(cmds, m.submitIssueComments(m.activeCommentsReqID, selected.Key))
	}

	if _, ok := m.worklogs[selected.Key]; ok {
		m.worklogsLoading = false
		m.worklogsErr = nil
		m.worklogsRequestKey = ""
	} else if !(m.worklogsLoading && m.worklogsRequestKey == selected.Key) {
		m.nextRequestID++
		m.activeWorklogsReqID = m.nextRequestID
		m.worklogsRequestKey = selected.Key
		m.worklogsLoading = true
		m.worklogsErr = nil
		cmds = append(cmds, m.submitIssueWorklogs(m.activeWorklogsReqID, selected.Key))
	}

	return m, tea.Batch(cmds...)
}

func (m Model) startWorklogRequestForSelected() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		m.worklogsLoading = false
		m.worklogsErr = nil
		m.worklogsRequestKey = ""
		return m, nil
	}
	if m.worklogsLoading && m.worklogsRequestKey == selected.Key {
		return m, nil
	}
	m.nextRequestID++
	m.activeWorklogsReqID = m.nextRequestID
	m.worklogsRequestKey = selected.Key
	m.worklogsLoading = true
	m.worklogsErr = nil
	return m, m.submitIssueWorklogs(m.activeWorklogsReqID, selected.Key)
}

func (m Model) startSelectedIssuePrefetch() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		m.detailLoading = false
		m.detailErr = nil
		m.detailRequestKey = ""
		return m, nil
	}

	var cmds []tea.Cmd
	m.hydrateIssueDetail(selected.Key)
	_, hasDetail := m.details[selected.Key]
	if hasDetail && m.isIssueDetailFresh(selected.Key) {
		m.recordDiagnosticEvent(diagnosticKindCache, "issue_detail", "hit", selected.Key)
		if m.detailRequestKey == selected.Key {
			m.detailLoading = false
			m.detailErr = nil
			m.detailRequestKey = ""
		}
	} else if m.detailLoading && m.detailRequestKey == selected.Key {
		// Existing request will hydrate the preview.
	} else if !hasDetail && len(m.issues) > selectedIssueDetailPrefetchLimit {
		m.recordDiagnosticEvent(diagnosticKindCache, "issue_detail", "prefetch_skip", selected.Key)
	} else {
		status := "miss"
		if hasDetail {
			status = "stale"
		}
		m.recordDiagnosticEvent(diagnosticKindCache, "issue_detail", status, selected.Key)
		m.nextRequestID++
		m.activeDetailRequestID = m.nextRequestID
		m.detailRequestKey = selected.Key
		m.detailLoading = true
		m.detailErr = nil
		cmds = append(cmds, m.submitIssueDetailWithPriority(m.activeDetailRequestID, selected.Key, worker.PriorityPrefetch))
	}

	if m.issueLayout == issueLayoutWorkbench {
		var commentCmd tea.Cmd
		m, commentCmd = m.startSelectedIssueCommentPrefetch(selected.Key)
		if commentCmd != nil {
			cmds = append(cmds, commentCmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) startSelectedIssueCommentPrefetch(key string) (Model, tea.Cmd) {
	if key == "" {
		return m, nil
	}
	m.hydrateIssueComments(key)
	_, hasComments := m.comments[key]
	if hasComments && m.isIssueCommentsFresh(key) {
		if m.commentsRequestKey == key {
			m.commentsLoading = false
			m.commentsErr = nil
			m.commentsRequestKey = ""
		}
		return m, nil
	}
	if m.commentsLoading && m.commentsRequestKey == key {
		return m, nil
	}
	status := "miss"
	if hasComments {
		status = "stale"
	}
	m.recordDiagnosticEvent(diagnosticKindCache, "issue_comments", status, key)
	m.nextRequestID++
	m.activeCommentsReqID = m.nextRequestID
	m.commentsRequestKey = key
	m.commentsLoading = true
	m.commentsErr = nil
	return m, m.submitIssueComments(m.activeCommentsReqID, key)
}

func (m Model) submitIssueDetail(requestID int, key string) tea.Cmd {
	return m.submitIssueDetailWithPriority(requestID, key, worker.PriorityForeground)
}

func (m Model) submitIssueDetailWithPriority(requestID int, key string, priority worker.Priority) tea.Cmd {
	return func() tea.Msg {
		if key == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:       requestID,
			Kind:     worker.KindGetIssue,
			Timeout:  m.requestTimeout,
			Priority: priority,
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

func (m Model) submitIssueTransition(requestID int, key string, transition jira.Transition, fields []jira.TransitionFieldValue) tea.Cmd {
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
				Fields:       append([]jira.TransitionFieldValue(nil), fields...),
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

func (m Model) submitCreateFieldOptions(requestID int, field jira.CreateField, query string) tea.Cmd {
	return func() tea.Msg {
		fieldID := createFieldValueKey(field)
		autocompleteURL := strings.TrimSpace(field.AutoCompleteURL)
		if strings.TrimSpace(fieldID) == "" || autocompleteURL == "" {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindSearchFieldOptions,
					Err:  worker.ErrInvalidRequest,
				},
			}
		}
		err := m.workers.Submit(worker.Request{
			ID:          requestID,
			Kind:        worker.KindSearchFieldOptions,
			Timeout:     m.requestTimeout,
			Priority:    worker.PriorityForeground,
			CoalesceKey: "field-options:" + fieldID + ":" + strings.TrimSpace(query),
			SearchFieldOptions: &worker.SearchFieldOptionsRequest{
				FieldID:         fieldID,
				AutoCompleteURL: autocompleteURL,
				Query:           strings.TrimSpace(query),
				MaxResults:      createFieldOptionMaxResults,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindSearchFieldOptions,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindSearchFieldOptions, id: requestID, key: fieldID}
	}
}

func (m Model) submitTransitionFieldOptions(requestID int, field jira.TransitionField, query string) tea.Cmd {
	return func() tea.Msg {
		fieldID := strings.TrimSpace(field.ID)
		autocompleteURL := strings.TrimSpace(field.AutoCompleteURL)
		if fieldID == "" || autocompleteURL == "" {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindSearchFieldOptions,
					Err:  worker.ErrInvalidRequest,
				},
			}
		}
		err := m.workers.Submit(worker.Request{
			ID:          requestID,
			Kind:        worker.KindSearchFieldOptions,
			Timeout:     m.requestTimeout,
			Priority:    worker.PriorityForeground,
			CoalesceKey: "transition-field-options:" + fieldID + ":" + strings.TrimSpace(query),
			SearchFieldOptions: &worker.SearchFieldOptionsRequest{
				FieldID:         fieldID,
				AutoCompleteURL: autocompleteURL,
				Query:           strings.TrimSpace(query),
				MaxResults:      createFieldOptionMaxResults,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindSearchFieldOptions,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindSearchFieldOptions, id: requestID, key: fieldID}
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

func (m Model) submitUpdateLabels(requestID int, key string, labels []string) tea.Cmd {
	return func() tea.Msg {
		if key == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindUpdateLabels,
			Timeout: m.requestTimeout,
			UpdateLabels: &worker.UpdateLabelsRequest{
				Key:    key,
				Labels: append([]string{}, labels...),
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindUpdateLabels,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindUpdateLabels, id: requestID, key: key}
	}
}

func (m Model) submitUpdateComponents(requestID int, key string, components []jira.FieldOption) tea.Cmd {
	return func() tea.Msg {
		if key == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindUpdateComponents,
			Timeout: m.requestTimeout,
			UpdateComponents: &worker.UpdateComponentsRequest{
				Key:        key,
				Components: append([]jira.FieldOption{}, components...),
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindUpdateComponents,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindUpdateComponents, id: requestID, key: key}
	}
}

func (m Model) submitUpdateEditField(requestID int, key string, field jira.EditField, value jira.EditFieldValue) tea.Cmd {
	return func() tea.Msg {
		if key == "" || strings.TrimSpace(value.FieldID) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:       requestID,
			Kind:     worker.KindUpdateEditField,
			Timeout:  m.requestTimeout,
			Priority: worker.PriorityWrite,
			UpdateEditField: &worker.UpdateEditFieldRequest{
				Key:   key,
				Field: field,
				Value: value,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindUpdateEditField,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindUpdateEditField, id: requestID, key: key}
	}
}

func (m Model) submitIssueLinkTypes(requestID int) tea.Cmd {
	return func() tea.Msg {
		err := m.workers.Submit(worker.Request{
			ID:                requestID,
			Kind:              worker.KindGetIssueLinkTypes,
			Timeout:           m.requestTimeout,
			Priority:          worker.PriorityForeground,
			GetIssueLinkTypes: &worker.GetIssueLinkTypesRequest{},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindGetIssueLinkTypes,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindGetIssueLinkTypes, id: requestID}
	}
}

func (m Model) submitCreateIssueLink(requestID int, request jira.CreateIssueLinkRequest) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(request.SourceKey) == "" || strings.TrimSpace(request.TargetKey) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:       requestID,
			Kind:     worker.KindCreateIssueLink,
			Timeout:  m.requestTimeout,
			Priority: worker.PriorityWrite,
			CreateIssueLink: &worker.CreateIssueLinkRequest{
				Request: request,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindCreateIssueLink,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindCreateIssueLink, id: requestID, key: request.SourceKey}
	}
}

func (m Model) submitIssueWorklogs(requestID int, key string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(key) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:       requestID,
			Kind:     worker.KindGetWorklogs,
			Timeout:  m.requestTimeout,
			Priority: worker.PriorityForeground,
			GetWorklogs: &worker.GetWorklogsRequest{
				Key:        key,
				MaxResults: 20,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindGetWorklogs,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindGetWorklogs, id: requestID, key: key}
	}
}

func (m Model) submitAddWorklog(requestID int, key string, request jira.AddWorklogRequest) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(request.TimeSpent) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:       requestID,
			Kind:     worker.KindAddWorklog,
			Timeout:  m.requestTimeout,
			Priority: worker.PriorityWrite,
			AddWorklog: &worker.AddWorklogRequest{
				Key:     key,
				Request: request,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindAddWorklog,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindAddWorklog, id: requestID, key: key}
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

func (m Model) submitUpdateComment(requestID int, key string, commentID string, body string, mentions []jira.Mention) tea.Cmd {
	return func() tea.Msg {
		if key == "" || commentID == "" || strings.TrimSpace(body) == "" {
			return noDetailRequestMsg{}
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindUpdateComment,
			Timeout: m.requestTimeout,
			UpdateComment: &worker.UpdateCommentRequest{
				Key:       key,
				CommentID: commentID,
				Body:      body,
				Mentions:  mentions,
			},
		})
		if err != nil {
			return workerResultMsg{
				result: worker.Result{
					ID:   requestID,
					Kind: worker.KindUpdateComment,
					Err:  err,
				},
			}
		}
		return workSubmittedMsg{kind: worker.KindUpdateComment, id: requestID, key: key}
	}
}

func (m Model) submitUserSearch(requestID int, query string, issueKeys ...string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(query) == "" {
			return noDetailRequestMsg{}
		}
		issueKey := ""
		if len(issueKeys) > 0 {
			issueKey = strings.TrimSpace(issueKeys[0])
		}
		err := m.workers.Submit(worker.Request{
			ID:      requestID,
			Kind:    worker.KindSearchUsers,
			Timeout: m.requestTimeout,
			SearchUsers: &worker.SearchUsersRequest{
				Query:      query,
				IssueKey:   issueKey,
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
