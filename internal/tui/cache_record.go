package tui

import (
	"time"

	"github.com/jellydator/ttlcache/v3"
)

type jiraCacheRecord[T any] struct {
	Value     T
	SyncedAt  time.Time
	FreshTill time.Time
	Err       error
}

func newJiraCacheRecord[T any](value T, syncedAt time.Time, freshFor time.Duration) jiraCacheRecord[T] {
	if syncedAt.IsZero() {
		syncedAt = time.Now()
	}
	return jiraCacheRecord[T]{
		Value:     value,
		SyncedAt:  syncedAt,
		FreshTill: syncedAt.Add(freshFor),
	}
}

func newJiraCache[T any](retention time.Duration) *ttlcache.Cache[string, jiraCacheRecord[T]] {
	return ttlcache.New[string, jiraCacheRecord[T]](ttlcache.WithTTL[string, jiraCacheRecord[T]](retention))
}

func (r jiraCacheRecord[T]) Fresh(now time.Time) bool {
	return !r.FreshTill.IsZero() && now.Before(r.FreshTill)
}

func (m Model) currentTime() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}
