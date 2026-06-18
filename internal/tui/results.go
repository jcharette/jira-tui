package tui

import (
	"fmt"
	"strings"
	"time"

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
	case worker.KindGetBoards:
		return m.handlePlanningBoardsResult(result)
	case worker.KindGetBoardSprints:
		return m.handlePlanningSprintsResult(result), nil
	case worker.KindCreateIssue:
		return m.handleCreateIssueResult(result), nil
	default:
		return m, nil
	}
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
	boardID := m.planningBoards[0].ID
	m.planningBoardID = boardID
	m.nextRequestID++
	m.activePlanningSprintsReqID = m.nextRequestID
	m.planningSprintsLoading = true
	m.planningSprintsErr = nil
	return m, m.submitPlanningSprints(m.activePlanningSprintsReqID, boardID)
}

func (m Model) handlePlanningSprintsResult(result worker.Result) Model {
	if result.ID != m.activePlanningSprintsReqID {
		return m
	}
	m.planningSprintsLoading = false
	if result.Err != nil {
		m.planningSprintsErr = result.Err
		return m
	}
	if result.GetBoardSprints == nil {
		m.planningSprintsErr = worker.ErrInvalidRequest
		return m
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
	return m
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
		requestKey := m.summaryMetadataRequestKey
		if isPriorityRequest {
			requestKey = m.priorityMetadataRequestKey
		}
		m.markIssueEditMetadataCacheError(requestKey, result.Err)
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
	m.cacheIssueEditMetadata(result.GetEditMetadata.Key, result.GetEditMetadata.Metadata, result.GetEditMetadata.SyncedAt)
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
