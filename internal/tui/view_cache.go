package tui

import (
	"strings"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/jon/jira-tui/internal/jira"
)

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
		return issueViewCacheRecord{}, false
	}
	item := m.activeViewCache.Get(activeViewCacheKey(jql))
	if item == nil {
		return issueViewCacheRecord{}, false
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
