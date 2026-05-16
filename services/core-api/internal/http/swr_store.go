package http

import (
	"sync"
	"time"
)

// swrLookup is the result of loading a cache entry.
type swrLookup int

const (
	swrLookupMiss swrLookup = iota
	swrLookupFresh
	swrLookupStale
)

type swrStore interface {
	load(key string, now time.Time) (it swrItem, kind swrLookup)
	save(key string, it swrItem)
	remove(key string)
	tryBeginRefresh(key string) bool
	endRefresh(key string, it swrItem, ok bool)
}

type memorySWRStore struct {
	mu    sync.Mutex
	items map[string]*swrItem
}

func newMemorySWRStore() *memorySWRStore {
	return &memorySWRStore{items: map[string]*swrItem{}}
}

func (m *memorySWRStore) load(key string, now time.Time) (swrItem, swrLookup) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, exists := m.items[key]
	if exists && now.After(it.staleUntil) {
		delete(m.items, key)
		exists = false
	}
	if !exists {
		return swrItem{}, swrLookupMiss
	}
	if now.Before(it.freshUntil) {
		return *cloneSWRItem(it), swrLookupFresh
	}
	if now.Before(it.staleUntil) {
		return *cloneSWRItem(it), swrLookupStale
	}
	delete(m.items, key)
	return swrItem{}, swrLookupMiss
}

func (m *memorySWRStore) save(key string, it swrItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := cloneSWRItem(&it)
	m.items[key] = c
}

func (m *memorySWRStore) remove(key string) {
	m.mu.Lock()
	delete(m.items, key)
	m.mu.Unlock()
}

func (m *memorySWRStore) tryBeginRefresh(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[key]
	if !ok || it.refreshing {
		return false
	}
	it.refreshing = true
	return true
}

func (m *memorySWRStore) endRefresh(key string, it swrItem, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, exists := m.items[key]
	if !exists {
		return
	}
	cur.refreshing = false
	if ok {
		c := cloneSWRItem(&it)
		cur.status = c.status
		cur.contentType = c.contentType
		cur.body = c.body
		cur.freshUntil = c.freshUntil
		cur.staleUntil = c.staleUntil
	}
}

func cloneSWRItem(it *swrItem) *swrItem {
	return &swrItem{
		status:      it.status,
		contentType: it.contentType,
		body:        append([]byte(nil), it.body...),
		freshUntil:  it.freshUntil,
		staleUntil:  it.staleUntil,
		refreshing:  it.refreshing,
	}
}
