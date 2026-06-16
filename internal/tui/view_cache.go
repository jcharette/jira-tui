package tui

import (
	"context"
	"strings"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/jon/jira-tui/internal/cache"
	"github.com/jon/jira-tui/internal/jira"
)

type activeViewStore interface {
	GetActiveView(context.Context, string, string) (cache.ActiveViewRecord, bool, error)
	PutActiveView(context.Context, cache.ActiveViewRecord) error
	GetIssueDetail(context.Context, string, string) (cache.IssueDetailRecord, bool, error)
	PutIssueDetail(context.Context, cache.IssueDetailRecord) error
	GetIssueComments(context.Context, string, string, int) (cache.IssueCommentsRecord, bool, error)
	PutIssueComments(context.Context, cache.IssueCommentsRecord) error
	DeleteIssueComments(context.Context, string, string) error
	GetIssueTransitions(context.Context, string, string) (cache.IssueTransitionsRecord, bool, error)
	PutIssueTransitions(context.Context, cache.IssueTransitionsRecord) error
	GetIssueEditMetadata(context.Context, string, string) (cache.IssueEditMetadataRecord, bool, error)
	PutIssueEditMetadata(context.Context, cache.IssueEditMetadataRecord) error
}

type issueViewCacheRecord struct {
	Issues    []jira.Issue
	SyncedAt  time.Time
	FreshTill time.Time
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

func (m *Model) applyActiveIssueView(record issueViewCacheRecord, stale bool) {
	selectedKey := ""
	if selected, ok := m.selectedIssue(); ok {
		selectedKey = selected.Key
	}
	m.replaceIssues(append([]jira.Issue(nil), record.Issues...))
	if selectedKey != "" {
		for index, issue := range m.issues {
			if issue.Key == selectedKey {
				m.selected = index
				break
			}
		}
	}
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
	m.applyActiveIssueView(record, !m.activeIssueViewCacheFresh(record))
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
	if m.activeViewCache != nil {
		m.activeViewCache.Set(activeViewCacheKey(jql), cached, ttlcache.DefaultTTL)
	}
	return cached, true
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
