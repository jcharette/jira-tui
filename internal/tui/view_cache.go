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
