package tui

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jcharette/jira-tui/internal/cache"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
	"github.com/jellydator/ttlcache/v3"
)

type activeViewStore interface {
	GetActiveView(context.Context, string, string) (cache.ActiveViewRecord, bool, error)
	PutActiveView(context.Context, cache.ActiveViewRecord) error
	PutQueryHistory(context.Context, cache.QueryHistoryRecord) error
	ListQueryHistory(context.Context, string, int) ([]cache.QueryHistoryRecord, error)
	GetIssueDetail(context.Context, string, string) (cache.IssueDetailRecord, bool, error)
	PutIssueDetail(context.Context, cache.IssueDetailRecord) error
	GetIssueComments(context.Context, string, string, int) (cache.IssueCommentsRecord, bool, error)
	PutIssueComments(context.Context, cache.IssueCommentsRecord) error
	DeleteIssueComments(context.Context, string, string) error
	GetIssueTransitions(context.Context, string, string) (cache.IssueTransitionsRecord, bool, error)
	PutIssueTransitions(context.Context, cache.IssueTransitionsRecord) error
	DeleteIssueTransitions(context.Context, string, string) error
	GetIssueEditMetadata(context.Context, string, string) (cache.IssueEditMetadataRecord, bool, error)
	PutIssueEditMetadata(context.Context, cache.IssueEditMetadataRecord) error
	GetCreateIssueTypes(context.Context, string, string) (cache.CreateIssueTypesRecord, bool, error)
	PutCreateIssueTypes(context.Context, cache.CreateIssueTypesRecord) error
	GetCreateFields(context.Context, string, string, string) (cache.CreateFieldsRecord, bool, error)
	PutCreateFields(context.Context, cache.CreateFieldsRecord) error
	GetExpandedChildren(context.Context, string, string, string) (cache.ExpandedChildrenRecord, bool, error)
	PutExpandedChildren(context.Context, cache.ExpandedChildrenRecord) error
}

type issueViewCacheRecord struct {
	Issues    []jira.Issue
	SyncedAt  time.Time
	FreshTill time.Time
	Err       error
}

func newIssueViewCache() *ttlcache.Cache[string, issueViewCacheRecord] {
	return ttlcache.New[string, issueViewCacheRecord](ttlcache.WithTTL[string, issueViewCacheRecord](activeViewCacheRetentionTTL))
}

func activeViewCacheKey(jql string) string {
	return strings.Join(strings.Fields(jql), " ")
}

func (m Model) cachedActiveIssueView(jql string) (issueViewCacheRecord, bool) {
	if m.activeViewCache == nil {
		return m.cachedPersistentActiveIssueView(jql)
	}
	item := m.activeViewCache.Get(activeViewCacheKey(jql))
	if item == nil {
		return m.cachedPersistentActiveIssueView(jql)
	}
	return item.Value(), true
}

func (m Model) activeIssueViewCacheFresh(record issueViewCacheRecord) bool {
	return !record.FreshTill.IsZero() && m.currentTime().Before(record.FreshTill)
}

func (m Model) activeIssueViewCacheDisplayable(record issueViewCacheRecord) bool {
	return !record.SyncedAt.IsZero() && m.currentTime().Before(record.SyncedAt.Add(activeViewCacheDisplayTTL))
}

func (m *Model) cacheActiveIssueView(jql string, issues []jira.Issue, syncedAt time.Time) {
	if m.activeViewCache == nil {
		return
	}
	if syncedAt.IsZero() {
		if m.now != nil {
			syncedAt = m.now()
		} else {
			syncedAt = time.Now()
		}
	}
	record := issueViewCacheRecord{
		Issues:    append([]jira.Issue(nil), issues...),
		SyncedAt:  syncedAt,
		FreshTill: syncedAt.Add(activeViewCacheTTL),
	}
	m.activeViewCache.Set(activeViewCacheKey(jql), record, ttlcache.DefaultTTL)
	m.persistActiveIssueView(jql, record)
}

func (m *Model) markActiveIssueViewCacheError(jql string, err error) {
	if m.activeViewCache == nil || err == nil {
		return
	}
	key := activeViewCacheKey(jql)
	item := m.activeViewCache.Get(key)
	if item == nil {
		return
	}
	record := item.Value()
	record.Err = err
	m.activeViewCache.Set(key, record, ttlcache.DefaultTTL)
}

func (m *Model) applyActiveIssueView(record issueViewCacheRecord, stale bool) {
	m.replaceIssues(append([]jira.Issue(nil), record.Issues...))
	m.loading = false
	m.refreshing = false
	m.err = nil
	m.lastSynced = record.SyncedAt
	m.viewStale = stale
	m.ensureSelectionVisible(m.currentLayoutRows())
}

func (m *Model) hydrateActiveIssueView() {
	record, ok := m.cachedPersistentActiveIssueView(m.jql)
	if !ok {
		return
	}
	stale := !m.activeIssueViewCacheFresh(record)
	m.applyActiveIssueView(record, stale)
	status := "hydrate_hit"
	refresh := "none"
	if stale {
		status = "hydrate_stale"
		refresh = "background"
		m.refreshing = true
	}
	m.recordDiagnosticEvent(diagnosticKindCache, "active_view", status, m.activeViewCacheDiagnosticDetail(record, refresh))
}

func (m Model) cachedPersistentActiveIssueView(jql string) (issueViewCacheRecord, bool) {
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return issueViewCacheRecord{}, false
	}
	record, ok, err := m.activeViewStore.GetActiveView(context.Background(), m.activeViewNamespace, activeViewCacheKey(jql))
	if err != nil || !ok {
		return issueViewCacheRecord{}, false
	}
	cached := issueViewCacheRecord{
		Issues:    append([]jira.Issue(nil), record.Issues...),
		SyncedAt:  record.SyncedAt,
		FreshTill: record.FreshTill,
	}
	if !m.activeIssueViewCacheDisplayable(cached) {
		return issueViewCacheRecord{}, false
	}
	if m.activeViewCache != nil {
		m.activeViewCache.Set(activeViewCacheKey(jql), cached, ttlcache.DefaultTTL)
	}
	return cached, true
}

func (m Model) activeViewCacheDiagnosticDetail(record issueViewCacheRecord, refresh string) string {
	age := m.currentTime().Sub(record.SyncedAt)
	if age < 0 {
		age = 0
	}
	age = age.Truncate(time.Second)
	if refresh == "" {
		refresh = "none"
	}
	return strings.Join([]string{
		m.activeViewName(),
		"issues=" + strconv.Itoa(len(record.Issues)),
		"age=" + age.String(),
		"refresh=" + refresh,
	}, " ")
}

func (m Model) persistActiveIssueView(jql string, record issueViewCacheRecord) {
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	_ = m.activeViewStore.PutActiveView(context.Background(), cache.ActiveViewRecord{
		Namespace: m.activeViewNamespace,
		CacheKey:  activeViewCacheKey(jql),
		Issues:    append([]jira.Issue(nil), record.Issues...),
		SyncedAt:  record.SyncedAt,
		FreshTill: record.FreshTill,
	})
}

func (m Model) loadQueryHistory() []cache.QueryHistoryRecord {
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return nil
	}
	records, err := m.activeViewStore.ListQueryHistory(context.Background(), m.activeViewNamespace, 10)
	if err != nil {
		return nil
	}
	return records
}

func (m Model) persistQueryHistory(jql string, source cache.QueryHistorySource, prompt string) {
	jql = strings.TrimSpace(jql)
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" || jql == "" {
		return
	}
	if source == "" {
		source = cache.QueryHistorySourceDirect
	}
	_ = m.activeViewStore.PutQueryHistory(context.Background(), cache.QueryHistoryRecord{
		Namespace:  m.activeViewNamespace,
		CacheKey:   activeViewCacheKey(jql),
		JQL:        jql,
		Prompt:     strings.TrimSpace(prompt),
		Source:     source,
		LastUsedAt: m.currentTime(),
	})
}

func (m *Model) hydrateIssueDetail(key string) {
	if _, ok := m.cachedIssueDetail(key); ok {
		return
	}
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	record, ok, err := m.activeViewStore.GetIssueDetail(context.Background(), m.activeViewNamespace, strings.TrimSpace(key))
	if err != nil || !ok {
		return
	}
	m.cacheIssueDetailRecord(record)
}

func (m *Model) hydrateIssueComments(key string) {
	if _, ok := m.cachedIssueComments(key); ok {
		return
	}
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	record, ok, err := m.activeViewStore.GetIssueComments(context.Background(), m.activeViewNamespace, strings.TrimSpace(key), maxComments)
	if err != nil || !ok {
		return
	}
	m.cacheIssueCommentsRecord(record)
}

func (m *Model) cacheIssueDetailRecord(record cache.IssueDetailRecord) {
	key := strings.TrimSpace(record.IssueKey)
	if m.detailCache == nil || key == "" {
		return
	}
	if m.details == nil {
		m.details = make(map[string]jira.IssueDetail)
	}
	m.details[key] = record.Detail
	m.detailCache.Set(key, jiraCacheRecord[jira.IssueDetail]{
		Value:     record.Detail,
		SyncedAt:  record.SyncedAt,
		FreshTill: record.FreshTill,
	}, ttlcache.DefaultTTL)
}

func (m *Model) cacheIssueCommentsRecord(record cache.IssueCommentsRecord) {
	key := strings.TrimSpace(record.IssueKey)
	if m.commentsCache == nil || key == "" {
		return
	}
	if m.comments == nil {
		m.comments = make(map[string][]jira.Comment)
	}
	copied := append([]jira.Comment(nil), record.Comments...)
	m.comments[key] = copied
	m.commentsCache.Set(key, jiraCacheRecord[[]jira.Comment]{
		Value:     copied,
		SyncedAt:  record.SyncedAt,
		FreshTill: record.FreshTill,
	}, ttlcache.DefaultTTL)
}

func (m Model) persistIssueDetail(key string, detail jira.IssueDetail, syncedAt time.Time) {
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	if syncedAt.IsZero() {
		syncedAt = m.currentTime()
	}
	_ = m.activeViewStore.PutIssueDetail(context.Background(), cache.IssueDetailRecord{
		Namespace: m.activeViewNamespace,
		IssueKey:  strings.TrimSpace(key),
		Detail:    detail,
		SyncedAt:  syncedAt,
		FreshTill: syncedAt.Add(issueDetailCacheTTL),
	})
}

func (m Model) persistIssueComments(key string, comments []jira.Comment, syncedAt time.Time) {
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	if syncedAt.IsZero() {
		syncedAt = m.currentTime()
	}
	_ = m.activeViewStore.PutIssueComments(context.Background(), cache.IssueCommentsRecord{
		Namespace:  m.activeViewNamespace,
		IssueKey:   strings.TrimSpace(key),
		MaxResults: maxComments,
		Comments:   append([]jira.Comment(nil), comments...),
		SyncedAt:   syncedAt,
		FreshTill:  syncedAt.Add(issueCommentsCacheTTL),
	})
}

func (m Model) deletePersistentIssueComments(key string) {
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	_ = m.activeViewStore.DeleteIssueComments(context.Background(), m.activeViewNamespace, strings.TrimSpace(key))
}

func (m *Model) hydrateIssueTransitions(key string) {
	if _, ok := m.cachedIssueTransitions(key); ok {
		return
	}
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	record, ok, err := m.activeViewStore.GetIssueTransitions(context.Background(), m.activeViewNamespace, strings.TrimSpace(key))
	if err != nil || !ok {
		return
	}
	m.cacheIssueTransitionsRecord(record)
}

func (m *Model) hydrateIssueEditMetadata(key string) {
	if _, ok := m.cachedIssueEditMetadata(key); ok {
		return
	}
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	record, ok, err := m.activeViewStore.GetIssueEditMetadata(context.Background(), m.activeViewNamespace, strings.TrimSpace(key))
	if err != nil || !ok {
		return
	}
	m.cacheIssueEditMetadataRecord(record)
}

func (m Model) isIssueTransitionsFresh(key string) bool {
	record, ok := m.cachedIssueTransitions(key)
	return ok && record.Fresh(m.currentTime())
}

func (m Model) cachedIssueTransitions(key string) (jiraCacheRecord[[]jira.Transition], bool) {
	if m.transitionsCache == nil || strings.TrimSpace(key) == "" {
		return jiraCacheRecord[[]jira.Transition]{}, false
	}
	item := m.transitionsCache.Get(strings.TrimSpace(key))
	if item == nil {
		return jiraCacheRecord[[]jira.Transition]{}, false
	}
	return item.Value(), true
}

func (m *Model) markIssueTransitionsCacheError(key string, err error) {
	markJiraCacheRecordError(m.transitionsCache, strings.TrimSpace(key), err)
}

func (m *Model) cacheIssueTransitions(key string, transitions []jira.Transition, syncedAt time.Time) {
	key = strings.TrimSpace(key)
	if m.transitionsCache == nil || key == "" {
		return
	}
	if m.transitions == nil {
		m.transitions = make(map[string][]jira.Transition)
	}
	copied := append([]jira.Transition(nil), transitions...)
	m.transitions[key] = copied
	m.transitionsCache.Set(key, newJiraCacheRecord(copied, syncedAt, issueTransitionsCacheTTL), ttlcache.DefaultTTL)
	m.persistIssueTransitions(key, copied, syncedAt)
}

func (m *Model) cacheIssueTransitionsRecord(record cache.IssueTransitionsRecord) {
	key := strings.TrimSpace(record.IssueKey)
	if m.transitionsCache == nil || key == "" {
		return
	}
	if m.transitions == nil {
		m.transitions = make(map[string][]jira.Transition)
	}
	copied := append([]jira.Transition(nil), record.Transitions...)
	m.transitions[key] = copied
	m.transitionsCache.Set(key, jiraCacheRecord[[]jira.Transition]{
		Value:     copied,
		SyncedAt:  record.SyncedAt,
		FreshTill: record.FreshTill,
	}, ttlcache.DefaultTTL)
}

func (m Model) isIssueEditMetadataFresh(key string) bool {
	record, ok := m.cachedIssueEditMetadata(key)
	return ok && record.Fresh(m.currentTime())
}

func (m Model) cachedIssueEditMetadata(key string) (jiraCacheRecord[jira.EditMetadata], bool) {
	if m.editMetadataCache == nil || strings.TrimSpace(key) == "" {
		return jiraCacheRecord[jira.EditMetadata]{}, false
	}
	item := m.editMetadataCache.Get(strings.TrimSpace(key))
	if item == nil {
		return jiraCacheRecord[jira.EditMetadata]{}, false
	}
	return item.Value(), true
}

func (m *Model) markIssueEditMetadataCacheError(key string, err error) {
	markJiraCacheRecordError(m.editMetadataCache, strings.TrimSpace(key), err)
}

func (m *Model) cacheIssueEditMetadata(key string, metadata jira.EditMetadata, syncedAt time.Time) {
	key = strings.TrimSpace(key)
	if m.editMetadataCache == nil || key == "" {
		return
	}
	if m.editMetadata == nil {
		m.editMetadata = make(map[string]jira.EditMetadata)
	}
	m.editMetadata[key] = metadata
	m.editMetadataCache.Set(key, newJiraCacheRecord(metadata, syncedAt, issueEditMetadataCacheTTL), ttlcache.DefaultTTL)
	m.persistIssueEditMetadata(key, metadata, syncedAt)
}

func (m *Model) cacheIssueEditMetadataRecord(record cache.IssueEditMetadataRecord) {
	key := strings.TrimSpace(record.IssueKey)
	if m.editMetadataCache == nil || key == "" {
		return
	}
	if m.editMetadata == nil {
		m.editMetadata = make(map[string]jira.EditMetadata)
	}
	m.editMetadata[key] = record.Metadata
	m.editMetadataCache.Set(key, jiraCacheRecord[jira.EditMetadata]{
		Value:     record.Metadata,
		SyncedAt:  record.SyncedAt,
		FreshTill: record.FreshTill,
	}, ttlcache.DefaultTTL)
}

func (m Model) persistIssueTransitions(key string, transitions []jira.Transition, syncedAt time.Time) {
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	if syncedAt.IsZero() {
		syncedAt = m.currentTime()
	}
	_ = m.activeViewStore.PutIssueTransitions(context.Background(), cache.IssueTransitionsRecord{
		Namespace:   m.activeViewNamespace,
		IssueKey:    strings.TrimSpace(key),
		Transitions: append([]jira.Transition(nil), transitions...),
		SyncedAt:    syncedAt,
		FreshTill:   syncedAt.Add(issueTransitionsCacheTTL),
	})
}

func (m Model) deletePersistentIssueTransitions(key string) {
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	_ = m.activeViewStore.DeleteIssueTransitions(context.Background(), m.activeViewNamespace, strings.TrimSpace(key))
}

func (m Model) persistIssueEditMetadata(key string, metadata jira.EditMetadata, syncedAt time.Time) {
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	if syncedAt.IsZero() {
		syncedAt = m.currentTime()
	}
	_ = m.activeViewStore.PutIssueEditMetadata(context.Background(), cache.IssueEditMetadataRecord{
		Namespace: m.activeViewNamespace,
		IssueKey:  strings.TrimSpace(key),
		Metadata:  metadata,
		SyncedAt:  syncedAt,
		FreshTill: syncedAt.Add(issueEditMetadataCacheTTL),
	})
}

func (m *Model) hydrateCreateIssueTypes(projectKey string) {
	if _, ok := m.cachedCreateIssueTypes(projectKey); ok {
		return
	}
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	record, ok, err := m.activeViewStore.GetCreateIssueTypes(context.Background(), m.activeViewNamespace, normalizeCreateProjectKey(projectKey))
	if err != nil || !ok {
		return
	}
	m.cacheCreateIssueTypesRecord(record)
}

func (m *Model) hydrateCreateFields(projectKey string, issueTypeID string) {
	if _, ok := m.cachedCreateFields(projectKey, issueTypeID); ok {
		return
	}
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	record, ok, err := m.activeViewStore.GetCreateFields(context.Background(), m.activeViewNamespace, normalizeCreateProjectKey(projectKey), strings.TrimSpace(issueTypeID))
	if err != nil || !ok {
		return
	}
	m.cacheCreateFieldsRecord(record)
}

func (m Model) isCreateIssueTypesFresh(projectKey string) bool {
	record, ok := m.cachedCreateIssueTypes(projectKey)
	return ok && record.Fresh(m.currentTime())
}

func (m Model) cachedCreateIssueTypes(projectKey string) (jiraCacheRecord[[]jira.CreateIssueType], bool) {
	if m.createIssueTypesCache == nil || normalizeCreateProjectKey(projectKey) == "" {
		return jiraCacheRecord[[]jira.CreateIssueType]{}, false
	}
	item := m.createIssueTypesCache.Get(normalizeCreateProjectKey(projectKey))
	if item == nil {
		return jiraCacheRecord[[]jira.CreateIssueType]{}, false
	}
	return item.Value(), true
}

func (m *Model) markCreateIssueTypesCacheError(projectKey string, err error) {
	markJiraCacheRecordError(m.createIssueTypesCache, normalizeCreateProjectKey(projectKey), err)
}

func (m *Model) cacheCreateIssueTypes(projectKey string, issueTypes []jira.CreateIssueType, syncedAt time.Time) {
	projectKey = normalizeCreateProjectKey(projectKey)
	if projectKey == "" {
		return
	}
	copied := append([]jira.CreateIssueType(nil), issueTypes...)
	m.createIssueTypes = copied
	if m.createIssueTypesCache != nil {
		m.createIssueTypesCache.Set(projectKey, newJiraCacheRecord(copied, syncedAt, createIssueTypesCacheTTL), ttlcache.DefaultTTL)
	}
	m.persistCreateIssueTypes(projectKey, copied, syncedAt)
}

func (m *Model) cacheCreateIssueTypesRecord(record cache.CreateIssueTypesRecord) {
	projectKey := normalizeCreateProjectKey(record.ProjectKey)
	if projectKey == "" {
		return
	}
	copied := append([]jira.CreateIssueType(nil), record.IssueTypes...)
	m.createIssueTypes = copied
	if m.createIssueTypesCache != nil {
		m.createIssueTypesCache.Set(projectKey, jiraCacheRecord[[]jira.CreateIssueType]{
			Value:     copied,
			SyncedAt:  record.SyncedAt,
			FreshTill: record.FreshTill,
		}, ttlcache.DefaultTTL)
	}
}

func (m Model) isCreateFieldsFresh(projectKey string, issueTypeID string) bool {
	record, ok := m.cachedCreateFields(projectKey, issueTypeID)
	return ok && record.Fresh(m.currentTime())
}

func (m Model) cachedCreateFields(projectKey string, issueTypeID string) (jiraCacheRecord[[]jira.CreateField], bool) {
	key := createFieldsCacheKey(projectKey, issueTypeID)
	if m.createFieldsCache == nil || key == "" {
		return jiraCacheRecord[[]jira.CreateField]{}, false
	}
	item := m.createFieldsCache.Get(key)
	if item == nil {
		return jiraCacheRecord[[]jira.CreateField]{}, false
	}
	return item.Value(), true
}

func (m *Model) markCreateFieldsCacheError(projectKey string, issueTypeID string, err error) {
	markJiraCacheRecordError(m.createFieldsCache, createFieldsCacheKey(projectKey, issueTypeID), err)
}

func (m *Model) cacheCreateFields(projectKey string, issueTypeID string, fields []jira.CreateField, syncedAt time.Time) {
	key := createFieldsCacheKey(projectKey, issueTypeID)
	if key == "" {
		return
	}
	copied := append([]jira.CreateField(nil), fields...)
	m.createFields = copied
	if m.createFieldsCache != nil {
		m.createFieldsCache.Set(key, newJiraCacheRecord(copied, syncedAt, createFieldsCacheTTL), ttlcache.DefaultTTL)
	}
	m.persistCreateFields(projectKey, issueTypeID, copied, syncedAt)
}

func (m *Model) cacheCreateFieldsRecord(record cache.CreateFieldsRecord) {
	key := createFieldsCacheKey(record.ProjectKey, record.IssueTypeID)
	if key == "" {
		return
	}
	copied := append([]jira.CreateField(nil), record.Fields...)
	m.createFields = copied
	if m.createFieldsCache != nil {
		m.createFieldsCache.Set(key, jiraCacheRecord[[]jira.CreateField]{
			Value:     copied,
			SyncedAt:  record.SyncedAt,
			FreshTill: record.FreshTill,
		}, ttlcache.DefaultTTL)
	}
}

func (m Model) persistCreateIssueTypes(projectKey string, issueTypes []jira.CreateIssueType, syncedAt time.Time) {
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	if syncedAt.IsZero() {
		syncedAt = m.currentTime()
	}
	_ = m.activeViewStore.PutCreateIssueTypes(context.Background(), cache.CreateIssueTypesRecord{
		Namespace:  m.activeViewNamespace,
		ProjectKey: normalizeCreateProjectKey(projectKey),
		IssueTypes: append([]jira.CreateIssueType(nil), issueTypes...),
		SyncedAt:   syncedAt,
		FreshTill:  syncedAt.Add(createIssueTypesCacheTTL),
	})
}

func (m Model) persistCreateFields(projectKey string, issueTypeID string, fields []jira.CreateField, syncedAt time.Time) {
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	if syncedAt.IsZero() {
		syncedAt = m.currentTime()
	}
	_ = m.activeViewStore.PutCreateFields(context.Background(), cache.CreateFieldsRecord{
		Namespace:   m.activeViewNamespace,
		ProjectKey:  normalizeCreateProjectKey(projectKey),
		IssueTypeID: strings.TrimSpace(issueTypeID),
		Fields:      append([]jira.CreateField(nil), fields...),
		SyncedAt:    syncedAt,
		FreshTill:   syncedAt.Add(createFieldsCacheTTL),
	})
}

func normalizeCreateProjectKey(projectKey string) string {
	return strings.ToUpper(strings.TrimSpace(projectKey))
}

func createFieldsCacheKey(projectKey string, issueTypeID string) string {
	projectKey = normalizeCreateProjectKey(projectKey)
	issueTypeID = strings.TrimSpace(issueTypeID)
	if projectKey == "" || issueTypeID == "" {
		return ""
	}
	return projectKey + "\x00" + issueTypeID
}

func (m *Model) hydrateExpandedChildren(parentKey string, mode worker.ExpandMode) {
	if _, ok := m.cachedExpandedChildren(parentKey, mode); ok {
		return
	}
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	record, ok, err := m.activeViewStore.GetExpandedChildren(context.Background(), m.activeViewNamespace, strings.TrimSpace(parentKey), string(mode))
	if err != nil || !ok {
		return
	}
	m.cacheExpandedChildrenRecord(record)
}

func (m Model) isExpandedChildrenFresh(parentKey string, mode worker.ExpandMode) bool {
	record, ok := m.cachedExpandedChildren(parentKey, mode)
	return ok && record.Fresh(m.currentTime())
}

func (m Model) cachedExpandedChildren(parentKey string, mode worker.ExpandMode) (jiraCacheRecord[[]jira.Issue], bool) {
	key := expandedChildrenCacheKey(parentKey, mode)
	if m.expandedChildrenCache == nil || key == "" {
		return jiraCacheRecord[[]jira.Issue]{}, false
	}
	item := m.expandedChildrenCache.Get(key)
	if item == nil {
		return jiraCacheRecord[[]jira.Issue]{}, false
	}
	return item.Value(), true
}

func (m *Model) markExpandedChildrenCacheError(parentKey string, mode worker.ExpandMode, err error) {
	markJiraCacheRecordError(m.expandedChildrenCache, expandedChildrenCacheKey(parentKey, mode), err)
}

func (m *Model) cacheExpandedChildren(parentKey string, mode worker.ExpandMode, issues []jira.Issue, syncedAt time.Time) {
	key := expandedChildrenCacheKey(parentKey, mode)
	if key == "" {
		return
	}
	copied := append([]jira.Issue(nil), issues...)
	if m.expandedChildrenCache != nil {
		m.expandedChildrenCache.Set(key, newJiraCacheRecord(copied, syncedAt, expandedChildrenCacheTTL), ttlcache.DefaultTTL)
	}
	m.persistExpandedChildren(parentKey, mode, copied, syncedAt)
}

func (m *Model) cacheExpandedChildrenRecord(record cache.ExpandedChildrenRecord) {
	key := expandedChildrenCacheKey(record.ParentKey, worker.ExpandMode(record.Mode))
	if key == "" || m.expandedChildrenCache == nil {
		return
	}
	copied := append([]jira.Issue(nil), record.Issues...)
	m.expandedChildrenCache.Set(key, jiraCacheRecord[[]jira.Issue]{
		Value:     copied,
		SyncedAt:  record.SyncedAt,
		FreshTill: record.FreshTill,
	}, ttlcache.DefaultTTL)
}

func (m Model) persistExpandedChildren(parentKey string, mode worker.ExpandMode, issues []jira.Issue, syncedAt time.Time) {
	if m.activeViewStore == nil || strings.TrimSpace(m.activeViewNamespace) == "" {
		return
	}
	if syncedAt.IsZero() {
		syncedAt = m.currentTime()
	}
	_ = m.activeViewStore.PutExpandedChildren(context.Background(), cache.ExpandedChildrenRecord{
		Namespace: m.activeViewNamespace,
		ParentKey: strings.TrimSpace(parentKey),
		Mode:      string(mode),
		Issues:    append([]jira.Issue(nil), issues...),
		SyncedAt:  syncedAt,
		FreshTill: syncedAt.Add(expandedChildrenCacheTTL),
	})
}

func expandedChildrenCacheKey(parentKey string, mode worker.ExpandMode) string {
	parentKey = strings.TrimSpace(parentKey)
	mode = worker.ExpandMode(strings.TrimSpace(string(mode)))
	if parentKey == "" || mode == "" {
		return ""
	}
	return parentKey + "\x00" + string(mode)
}
