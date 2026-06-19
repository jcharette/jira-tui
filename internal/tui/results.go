package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
	"github.com/jellydator/ttlcache/v3"
)

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
	case worker.KindUpdateComment:
		return m.handleUpdateCommentResult(result)
	case worker.KindGetWorklogs:
		return m.handleGetWorklogsResult(result), nil
	case worker.KindAddWorklog:
		return m.handleAddWorklogResult(result)
	case worker.KindUpdateWorklog:
		return m.handleUpdateWorklogResult(result)
	case worker.KindDeleteWorklog:
		return m.handleDeleteWorklogResult(result)
	case worker.KindSearchUsers:
		return m.handleUserSearchResult(result), nil
	case worker.KindExpandIssues:
		return m.handleExpandIssuesResult(result), nil
	case worker.KindGetTransitions:
		return m.handleGetTransitionsResult(result), nil
	case worker.KindTransitionIssue:
		return m.handleTransitionIssueResult(result), nil
	case worker.KindStartIssue:
		return m.handleStartIssueResult(result)
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
	case worker.KindUpdateLabels:
		return m.handleUpdateLabelsResult(result), nil
	case worker.KindUpdateComponents:
		return m.handleUpdateComponentsResult(result), nil
	case worker.KindUpdateEditField:
		return m.handleUpdateEditFieldResult(result), nil
	case worker.KindUpdateAssignee:
		return m.handleUpdateAssigneeResult(result), nil
	case worker.KindGetCreateIssueTypes:
		return m.handleGetCreateIssueTypesResult(result), nil
	case worker.KindGetCreateFields:
		return m.handleGetCreateFieldsResult(result), nil
	case worker.KindSearchFieldOptions:
		if result.ID == m.activeTransitionFieldOptionsReqID {
			return m.handleTransitionFieldOptionsResult(result), nil
		}
		if result.ID == m.activeGenericFieldOptionsReqID {
			return m.handleGenericFieldOptionsResult(result), nil
		}
		return m.handleSearchFieldOptionsResult(result), nil
	case worker.KindGetBoards:
		return m.handlePlanningBoardsResult(result)
	case worker.KindGetBoardSprints:
		return m.handlePlanningSprintsResult(result)
	case worker.KindGetIssueLinkTypes:
		return m.handleGetIssueLinkTypesResult(result), nil
	case worker.KindCreateIssueLink:
		return m.handleCreateIssueLinkResult(result)
	case worker.KindDeleteIssueLink:
		return m.handleDeleteIssueLinkResult(result)
	case worker.KindCreateIssue:
		return m.handleCreateIssueResult(result), nil
	default:
		return m, nil
	}
}

func (m Model) handleGetIssueLinkTypesResult(result worker.Result) Model {
	if result.ID != m.activeIssueLinkTypesReqID {
		return m
	}
	m.issueLinkTypesLoading = false
	if result.Err != nil {
		m.issueLinkTypesErr = result.Err
		m.detailNotice = "Issue link types failed: " + result.Err.Error()
		return m
	}
	if result.GetIssueLinkTypes == nil {
		m.issueLinkTypesErr = worker.ErrInvalidRequest
		m.detailNotice = "Issue link types failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	m.issueLinkTypes = result.GetIssueLinkTypes.Types
	m.issueLinkTypesErr = nil
	m.selectedIssueLinkRelation = clamp(m.selectedIssueLinkRelation, 0, max(0, len(m.issueLinkRelations())-1))
	return m
}

func (m Model) handleCreateIssueLinkResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeCreateIssueLinkReqID {
		return m, nil
	}
	m.issueLinkSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Issue link failed: " + result.Err.Error()
		return m, nil
	}
	if result.CreateIssueLink == nil {
		m.detailNotice = "Issue link failed: " + worker.ErrInvalidRequest.Error()
		return m, nil
	}
	sourceKey := strings.TrimSpace(result.CreateIssueLink.Request.SourceKey)
	targetKey := strings.TrimSpace(result.CreateIssueLink.Request.TargetKey)
	m.closeIssueLinkEditor()
	m.detailNotice = "Linked " + displayValue(sourceKey, "selected") + " to " + displayValue(targetKey, "target") + "."
	if sourceKey != "" {
		delete(m.details, sourceKey)
		if m.detailCache != nil {
			m.detailCache.Delete(sourceKey)
		}
	}
	return m.startDetailRequestForSelected()
}

func (m Model) handleDeleteIssueLinkResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeDeleteIssueLinkReqID {
		return m, nil
	}
	m.issueLinkDeleteSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Issue link removal failed: " + result.Err.Error()
		return m, nil
	}
	if result.DeleteIssueLink == nil {
		m.detailNotice = "Issue link removal failed: " + worker.ErrInvalidRequest.Error()
		return m, nil
	}
	issueKey := strings.TrimSpace(result.DeleteIssueLink.IssueKey)
	target := strings.TrimSpace(result.DeleteIssueLink.Target)
	m.issueLinkDeleteConfirm = false
	m.issueLinkDeleteID = ""
	m.issueLinkDeleteTarget = ""
	m.linkFocus = false
	m.detailNotice = "Removed issue link " + displayValue(target, result.DeleteIssueLink.LinkID) + "."
	if issueKey != "" {
		delete(m.details, issueKey)
		if m.detailCache != nil {
			m.detailCache.Delete(issueKey)
		}
	}
	return m.startDetailRequestForSelected()
}

func (m Model) handlePlanningBoardsResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activePlanningBoardsReqID {
		return m, nil
	}
	m.planningBoardsLoading = false
	if result.Err != nil {
		m.planningBoardsErr = result.Err
		return m, nil
	}
	if result.GetBoards == nil {
		m.planningBoardsErr = worker.ErrInvalidRequest
		return m, nil
	}
	m.planningBoardsErr = nil
	m.planningBoardPage = result.GetBoards.Page
	m.planningBoards = result.GetBoards.Page.Boards
	if len(m.planningBoards) == 0 {
		m.planningBoardID = 0
		return m, nil
	}
	m.planningBoardID = m.planningBoards[0].ID
	m.planningSprintsErr = nil
	m.planningSprintQueue = m.boardIDsForPlanningSprints(m.planningBoards)
	return m.startQueuedPlanningSprintLoads()
}

func (m Model) handlePlanningSprintsResult(result worker.Result) (Model, tea.Cmd) {
	if len(m.activePlanningSprintReqIDs) > 0 {
		if _, ok := m.activePlanningSprintReqIDs[result.ID]; !ok {
			return m, nil
		}
		delete(m.activePlanningSprintReqIDs, result.ID)
	} else if result.ID != m.activePlanningSprintsReqID {
		return m, nil
	}
	if result.Err != nil {
		m.planningSprintsErr = result.Err
		m.planningSprintsLoading = len(m.activePlanningSprintReqIDs) > 0 || len(m.planningSprintQueue) > 0
		if m.planningSprintsLoading {
			return m.startQueuedPlanningSprintLoads()
		}
		return m, nil
	}
	if result.GetBoardSprints == nil {
		m.planningSprintsErr = worker.ErrInvalidRequest
		m.planningSprintsLoading = len(m.activePlanningSprintReqIDs) > 0 || len(m.planningSprintQueue) > 0
		if m.planningSprintsLoading {
			return m.startQueuedPlanningSprintLoads()
		}
		return m, nil
	}
	m.planningSprintsErr = nil
	if m.planningSprints == nil {
		m.planningSprints = make(map[int][]jira.Sprint)
	}
	if m.planningSprintPages == nil {
		m.planningSprintPages = make(map[int]jira.SprintPage)
	}
	boardID := result.GetBoardSprints.BoardID
	m.planningBoardID = boardID
	m.planningSprintPages[boardID] = result.GetBoardSprints.Page
	m.planningSprints[boardID] = result.GetBoardSprints.Page.Sprints
	return m.startQueuedPlanningSprintLoads()
}

func (m Model) boardIDsForPlanningSprints(boards []jira.Board) []int {
	ids := make([]int, 0, len(boards))
	seen := make(map[int]struct{}, len(boards))
	for _, board := range boards {
		if board.ID <= 0 {
			continue
		}
		if _, ok := seen[board.ID]; ok {
			continue
		}
		seen[board.ID] = struct{}{}
		ids = append(ids, board.ID)
	}
	return ids
}

func (m Model) startQueuedPlanningSprintLoads() (Model, tea.Cmd) {
	if m.activePlanningSprintReqIDs == nil {
		m.activePlanningSprintReqIDs = make(map[int]int)
	}
	var cmds []tea.Cmd
	for len(m.planningSprintQueue) > 0 && len(m.activePlanningSprintReqIDs) < planningSprintFetchConcurrency {
		boardID := m.planningSprintQueue[0]
		m.planningSprintQueue = m.planningSprintQueue[1:]
		m.nextRequestID++
		requestID := m.nextRequestID
		m.activePlanningSprintsReqID = requestID
		m.activePlanningSprintReqIDs[requestID] = boardID
		cmds = append(cmds, m.submitPlanningSprints(requestID, boardID))
	}
	m.planningSprintsLoading = len(m.activePlanningSprintReqIDs) > 0 || len(m.planningSprintQueue) > 0
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
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
	if result.SearchUsers.Query != m.assigneeQuery || result.SearchUsers.IssueKey != m.assigneeSearchIssueKey {
		return m
	}
	m.cacheAssignableUserSearch(result.SearchUsers.IssueKey, result.SearchUsers.Query, result.SearchUsers.Users)
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

func (m Model) cachedAssignableUserSearch(issueKey string, query string) ([]jira.User, bool) {
	if m.userSearchCache == nil {
		return nil, false
	}
	item := m.userSearchCache.Get(assignableUserSearchCacheKey(issueKey, query))
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

func (m Model) cacheAssignableUserSearch(issueKey string, query string, users []jira.User) {
	if m.userSearchCache == nil || strings.TrimSpace(issueKey) == "" || strings.TrimSpace(query) == "" {
		return
	}
	m.userSearchCache.Set(assignableUserSearchCacheKey(issueKey, query), users, ttlcache.DefaultTTL)
}

func userSearchCacheKey(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

func assignableUserSearchCacheKey(issueKey string, query string) string {
	return "assignable:" + strings.ToUpper(strings.TrimSpace(issueKey)) + ":" + userSearchCacheKey(query)
}

func (m Model) handleEditMetadataResult(result worker.Result) Model {
	if result.ID != m.activeSummaryMetadataReqID && result.ID != m.activePriorityMetadataReqID && result.ID != m.activeLabelsMetadataReqID && result.ID != m.activeComponentsMetadataReqID && result.ID != m.activeGenericFieldMetadataReqID {
		return m
	}
	isPriorityRequest := result.ID == m.activePriorityMetadataReqID
	isLabelsRequest := result.ID == m.activeLabelsMetadataReqID
	isComponentsRequest := result.ID == m.activeComponentsMetadataReqID
	isGenericFieldRequest := result.ID == m.activeGenericFieldMetadataReqID
	if isPriorityRequest {
		m.priorityMetadataLoading = false
	} else if isLabelsRequest {
		m.labelsMetadataLoading = false
	} else if isComponentsRequest {
		m.componentsMetadataLoading = false
	} else if isGenericFieldRequest {
		m.genericFieldMetadataLoading = false
	} else {
		m.summaryMetadataLoading = false
	}
	if result.Err != nil {
		requestKey := m.summaryMetadataRequestKey
		if isPriorityRequest {
			requestKey = m.priorityMetadataRequestKey
		} else if isLabelsRequest {
			requestKey = m.labelsMetadataRequestKey
		} else if isComponentsRequest {
			requestKey = m.componentsMetadataRequestKey
		} else if isGenericFieldRequest {
			requestKey = m.genericFieldMetadataRequestKey
		}
		m.markIssueEditMetadataCacheError(requestKey, result.Err)
		if isPriorityRequest {
			m.priorityMetadataErr = result.Err
			m.detailNotice = "Priority metadata failed: " + result.Err.Error()
		} else if isLabelsRequest {
			m.labelsMetadataErr = result.Err
			m.detailNotice = "Labels metadata failed: " + result.Err.Error()
		} else if isComponentsRequest {
			m.componentsMetadataErr = result.Err
			m.detailNotice = "Components metadata failed: " + result.Err.Error()
		} else if isGenericFieldRequest {
			m.genericFieldMetadataErr = result.Err
			m.detailNotice = "Field metadata failed: " + result.Err.Error()
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
		} else if isLabelsRequest {
			m.labelsMetadataErr = worker.ErrInvalidRequest
			m.detailNotice = "Labels metadata failed: " + worker.ErrInvalidRequest.Error()
		} else if isComponentsRequest {
			m.componentsMetadataErr = worker.ErrInvalidRequest
			m.detailNotice = "Components metadata failed: " + worker.ErrInvalidRequest.Error()
		} else if isGenericFieldRequest {
			m.genericFieldMetadataErr = worker.ErrInvalidRequest
			m.detailNotice = "Field metadata failed: " + worker.ErrInvalidRequest.Error()
		} else {
			m.summaryMetadataErr = worker.ErrInvalidRequest
			m.detailNotice = "Summary metadata failed: " + worker.ErrInvalidRequest.Error()
		}
		return m
	}
	requestKey := m.summaryMetadataRequestKey
	if isPriorityRequest {
		requestKey = m.priorityMetadataRequestKey
	} else if isLabelsRequest {
		requestKey = m.labelsMetadataRequestKey
	} else if isComponentsRequest {
		requestKey = m.componentsMetadataRequestKey
	} else if isGenericFieldRequest {
		requestKey = m.genericFieldMetadataRequestKey
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
	m.cacheIssueEditMetadata(result.GetEditMetadata.Key, result.GetEditMetadata.Metadata, result.GetEditMetadata.SyncedAt)
	if isPriorityRequest {
		m.priorityMetadataErr = nil
		return m.beginPriorityEditing(result.GetEditMetadata.Metadata)
	}
	if isLabelsRequest {
		m.labelsMetadataErr = nil
		return m.beginLabelsEditing(result.GetEditMetadata.Metadata)
	}
	if isComponentsRequest {
		m.componentsMetadataErr = nil
		return m.beginComponentsEditing(result.GetEditMetadata.Metadata)
	}
	if isGenericFieldRequest {
		m.genericFieldMetadataErr = nil
		return m.beginGenericFieldEditing(result.GetEditMetadata.Metadata)
	}
	m.summaryMetadataErr = nil
	if !result.GetEditMetadata.Metadata.Summary.Editable {
		m.detailNotice = "Summary is not editable for " + result.GetEditMetadata.Key + "."
		return m
	}
	m.beginSummaryEditing()
	return m
}

func (m Model) handleUpdateEditFieldResult(result worker.Result) Model {
	if result.ID != m.activeGenericFieldReqID {
		return m
	}
	m.genericFieldSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Field update failed: " + result.Err.Error()
		return m
	}
	if result.UpdateEditField == nil {
		m.detailNotice = "Field update failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	if result.UpdateEditField.Key != m.genericFieldSubmitKey {
		return m
	}
	fieldName := displayValue(result.UpdateEditField.Field.Name, result.UpdateEditField.Value.FieldID)
	m.closeGenericFieldEditor()
	m.detailNotice = fieldName + " updated."
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

func (m Model) handleUpdateLabelsResult(result worker.Result) Model {
	if result.ID != m.activeLabelsReqID {
		return m
	}
	m.labelsSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Labels update failed: " + result.Err.Error()
		return m
	}
	if result.UpdateLabels == nil {
		m.detailNotice = "Labels update failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	if result.UpdateLabels.Key != m.labelsSubmitKey {
		return m
	}
	m.updateIssueLabels(result.UpdateLabels.Key, result.UpdateLabels.Labels)
	m.labelsFocus = false
	m.labelsEditing = false
	m.labelsDirty = false
	m.labelsEditor = textarea.Model{}
	m.labelsEditorReady = false
	m.labelsSubmitKey = ""
	m.labelsSubmitValue = nil
	m.detailNotice = "Labels updated."
	return m
}

func (m Model) handleUpdateComponentsResult(result worker.Result) Model {
	if result.ID != m.activeComponentsReqID {
		return m
	}
	m.componentsSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Components update failed: " + result.Err.Error()
		return m
	}
	if result.UpdateComponents == nil {
		m.detailNotice = "Components update failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	if result.UpdateComponents.Key != m.componentsSubmitKey {
		return m
	}
	m.updateIssueComponents(result.UpdateComponents.Key, componentNamesFromOptions(result.UpdateComponents.Components))
	m.componentsFocus = false
	m.componentsDirty = false
	m.componentsFilter = ""
	m.componentsFilterEditor = textinput.Model{}
	m.componentsFilterEditorReady = false
	m.componentsSubmitKey = ""
	m.componentsSubmitValue = nil
	m.detailNotice = "Components updated."
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

func (m Model) handleGetTransitionsResult(result worker.Result) Model {
	if result.ID != m.activeTransitionsReqID {
		return m
	}
	m.transitionLoading = false
	if result.Err != nil {
		m.transitionErr = result.Err
		m.markIssueTransitionsCacheError(m.transitionRequestKey, result.Err)
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
	m.cacheIssueTransitions(result.GetTransitions.Key, result.GetTransitions.Transitions, result.GetTransitions.SyncedAt)
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
	m.transitionFieldEditing = false
	m.transitionSubmitKey = ""
	m.transitionSubmitToStatus = ""
	m.transitionSubmitFields = nil
	m.detailNotice = "Status updated to " + displayValue(result.TransitionIssue.ToStatus, "Unknown") + "."
	return m
}

func (m Model) handleExpandIssuesResult(result worker.Result) Model {
	if result.ID != m.activeExpandReqID {
		return m
	}
	m.expandLoading = false
	if result.Err != nil {
		m.markExpandedChildrenCacheError(m.expandRequestKey, m.expandMode, result.Err)
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
	m.cacheExpandedChildren(result.ExpandIssues.ParentKey, result.ExpandIssues.Mode, result.ExpandIssues.Issues, result.ExpandIssues.SyncedAt)
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
		m.markActiveIssueViewCacheError(m.jql, result.Err)
		if len(m.issues) > 0 {
			m.viewStale = true
		}
		return m, nil
	}
	if result.SearchIssues == nil {
		m.err = worker.ErrInvalidRequest
		if len(m.issues) > 0 {
			m.viewStale = true
		}
		return m, nil
	}

	m.err = nil
	m.publishTicketEvents(result.SearchIssues.Issues, result.SearchIssues.SyncedAt)
	m.replaceIssues(result.SearchIssues.Issues)
	m.hydrateVisibleExpandedChildren()
	m.lastSynced = result.SearchIssues.SyncedAt
	m.viewStale = false
	m.cacheActiveIssueView(m.jql, m.issues, result.SearchIssues.SyncedAt)
	if m.mode == modeDetail {
		return m.startDetailRequestForSelected()
	}
	return m.startSelectedIssuePrefetch()
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
		m.markIssueDetailCacheError(m.detailRequestKey, result.Err)
		return m
	}
	if result.GetIssue == nil {
		m.detailErr = worker.ErrInvalidRequest
		return m
	}
	m.cacheIssueDetail(result.GetIssue.Key, result.GetIssue.Detail, result.GetIssue.SyncedAt)
	m.detailErr = nil
	return m
}

func (m Model) isIssueDetailFresh(key string) bool {
	record, ok := m.cachedIssueDetail(key)
	return ok && record.Fresh(m.currentTime())
}

func (m Model) cachedIssueDetail(key string) (jiraCacheRecord[jira.IssueDetail], bool) {
	if m.detailCache == nil || strings.TrimSpace(key) == "" {
		return jiraCacheRecord[jira.IssueDetail]{}, false
	}
	item := m.detailCache.Get(strings.TrimSpace(key))
	if item == nil {
		return jiraCacheRecord[jira.IssueDetail]{}, false
	}
	return item.Value(), true
}

func (m *Model) markIssueDetailCacheError(key string, err error) {
	markJiraCacheRecordError(m.detailCache, strings.TrimSpace(key), err)
}

func (m *Model) cacheIssueDetail(key string, detail jira.IssueDetail, syncedAt time.Time) {
	key = strings.TrimSpace(key)
	if m.detailCache == nil || key == "" {
		return
	}
	if m.details == nil {
		m.details = make(map[string]jira.IssueDetail)
	}
	m.details[key] = detail
	m.detailCache.Set(key, newJiraCacheRecord(detail, syncedAt, issueDetailCacheTTL), ttlcache.DefaultTTL)
	m.persistIssueDetail(key, detail, syncedAt)
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
	m.invalidateIssueComments(key)
	m.nextRequestID++
	m.activeCommentsReqID = m.nextRequestID
	m.commentsRequestKey = key
	m.commentsLoading = true
	m.commentsErr = nil
	return m, m.submitIssueComments(m.activeCommentsReqID, key)
}

func (m Model) handleUpdateCommentResult(result worker.Result) (Model, tea.Cmd) {
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
		m.detailNotice = "Comment update failed: " + result.Err.Error()
		m.commentConfirm = false
		return m, nil
	}
	if result.UpdateComment == nil {
		m.detailNotice = "Comment update failed: " + worker.ErrInvalidRequest.Error()
		m.commentConfirm = false
		return m, nil
	}

	key := result.UpdateComment.Key
	m.mode = modeDetail
	m.commentDraft = ""
	m.commentMentions = nil
	m.commentConfirm = false
	m.commentEditing = false
	m.commentEditIssueKey = ""
	m.commentEditID = ""
	m.commentEditOriginal = ""
	m.commentRequestKey = ""
	m.detailNotice = "Comment updated."
	m.invalidateIssueComments(key)
	m.nextRequestID++
	m.activeCommentsReqID = m.nextRequestID
	m.commentsRequestKey = key
	m.commentsLoading = true
	m.commentsErr = nil
	return m, m.submitIssueComments(m.activeCommentsReqID, key)
}

func (m Model) handleGetWorklogsResult(result worker.Result) Model {
	if result.ID != m.activeWorklogsReqID {
		return m
	}
	if m.worklogsRequestKey != "" {
		selected, ok := m.selectedIssue()
		if ok && selected.Key != m.worklogsRequestKey {
			return m
		}
	}
	m.worklogsLoading = false
	if result.Err != nil {
		m.worklogsErr = result.Err
		return m
	}
	if result.GetWorklogs == nil {
		m.worklogsErr = worker.ErrInvalidRequest
		return m
	}
	if m.worklogs == nil {
		m.worklogs = make(map[string][]jira.Worklog)
	}
	m.worklogs[result.GetWorklogs.Key] = append([]jira.Worklog(nil), result.GetWorklogs.Worklogs...)
	m.worklogsErr = nil
	return m
}

func (m Model) handleAddWorklogResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeAddWorklogReqID {
		return m, nil
	}
	m.worklogSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Worklog failed: " + result.Err.Error()
		return m, nil
	}
	if result.AddWorklog == nil {
		m.detailNotice = "Worklog failed: " + worker.ErrInvalidRequest.Error()
		return m, nil
	}
	key := result.AddWorklog.Key
	m.closeWorklogEditor()
	m.detailNotice = "Work logged."
	if m.worklogs != nil {
		delete(m.worklogs, key)
	}
	m.nextRequestID++
	m.activeWorklogsReqID = m.nextRequestID
	m.worklogsRequestKey = key
	m.worklogsLoading = true
	m.worklogsErr = nil
	return m, m.submitIssueWorklogs(m.activeWorklogsReqID, key)
}

func (m Model) handleUpdateWorklogResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeUpdateWorklogReqID {
		return m, nil
	}
	m.worklogSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Worklog update failed: " + result.Err.Error()
		return m, nil
	}
	if result.UpdateWorklog == nil {
		m.detailNotice = "Worklog update failed: " + worker.ErrInvalidRequest.Error()
		return m, nil
	}
	key := result.UpdateWorklog.Key
	m.closeWorklogEditor()
	m.detailNotice = "Worklog updated."
	if m.worklogs != nil {
		delete(m.worklogs, key)
	}
	m.nextRequestID++
	m.activeWorklogsReqID = m.nextRequestID
	m.worklogsRequestKey = key
	m.worklogsLoading = true
	m.worklogsErr = nil
	return m, m.submitIssueWorklogs(m.activeWorklogsReqID, key)
}

func (m Model) handleDeleteWorklogResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeDeleteWorklogReqID {
		return m, nil
	}
	m.worklogDeleteSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Worklog delete failed: " + result.Err.Error()
		return m, nil
	}
	if result.DeleteWorklog == nil {
		m.detailNotice = "Worklog delete failed: " + worker.ErrInvalidRequest.Error()
		return m, nil
	}
	key := result.DeleteWorklog.Key
	m.worklogDeleteConfirm = false
	m.worklogDeleteID = ""
	m.worklogListFocus = false
	m.detailNotice = "Worklog deleted."
	if m.worklogs != nil {
		delete(m.worklogs, key)
	}
	m.nextRequestID++
	m.activeWorklogsReqID = m.nextRequestID
	m.worklogsRequestKey = key
	m.worklogsLoading = true
	m.worklogsErr = nil
	return m, m.submitIssueWorklogs(m.activeWorklogsReqID, key)
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
		m.markIssueCommentsCacheError(m.commentsRequestKey, result.Err)
		return m
	}
	if result.GetComments == nil {
		m.commentsErr = worker.ErrInvalidRequest
		return m
	}
	if m.comments == nil {
		m.comments = make(map[string][]jira.Comment)
	}
	m.cacheIssueComments(result.GetComments.Key, result.GetComments.Comments, result.GetComments.SyncedAt)
	m.commentsErr = nil
	return m
}

func (m Model) isIssueCommentsFresh(key string) bool {
	record, ok := m.cachedIssueComments(key)
	return ok && record.Fresh(m.currentTime())
}

func (m Model) cachedIssueComments(key string) (jiraCacheRecord[[]jira.Comment], bool) {
	if m.commentsCache == nil || strings.TrimSpace(key) == "" {
		return jiraCacheRecord[[]jira.Comment]{}, false
	}
	item := m.commentsCache.Get(strings.TrimSpace(key))
	if item == nil {
		return jiraCacheRecord[[]jira.Comment]{}, false
	}
	return item.Value(), true
}

func (m *Model) markIssueCommentsCacheError(key string, err error) {
	markJiraCacheRecordError(m.commentsCache, strings.TrimSpace(key), err)
}

func (m *Model) cacheIssueComments(key string, comments []jira.Comment, syncedAt time.Time) {
	key = strings.TrimSpace(key)
	if m.commentsCache == nil || key == "" {
		return
	}
	if m.comments == nil {
		m.comments = make(map[string][]jira.Comment)
	}
	copied := append([]jira.Comment(nil), comments...)
	m.comments[key] = copied
	m.commentsCache.Set(key, newJiraCacheRecord(copied, syncedAt, issueCommentsCacheTTL), ttlcache.DefaultTTL)
	m.persistIssueComments(key, copied, syncedAt)
}

func (m *Model) invalidateIssueComments(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	if m.comments != nil {
		delete(m.comments, key)
	}
	if m.commentsCache != nil {
		m.commentsCache.Delete(key)
	}
	m.deletePersistentIssueComments(key)
}
