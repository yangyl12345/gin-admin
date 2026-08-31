package retrieval

import (
	"sync"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
)

type cacheEntry struct {
	records   []VectorRecord
	expiresAt time.Time
}

type Cache struct {
	mu      sync.RWMutex
	indices map[string]cacheEntry
	ttl     time.Duration
	now     func() time.Time
}

func NewCache() *Cache {
	ttl := time.Duration(config.C.Agent.CacheTTLSeconds) * time.Second
	return &Cache{indices: make(map[string]cacheEntry), ttl: ttl, now: time.Now}
}

func (a *Cache) Get(knowledgeBaseID string) ([]VectorRecord, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.indices[knowledgeBaseID]
	if !ok {
		return nil, false
	}
	if !entry.expiresAt.IsZero() && !a.now().Before(entry.expiresAt) {
		delete(a.indices, knowledgeBaseID)
		return nil, false
	}
	return append([]VectorRecord(nil), entry.records...), true
}

func (a *Cache) Put(knowledgeBaseID string, items []VectorRecord) {
	entry := cacheEntry{records: append([]VectorRecord(nil), items...)}
	if a.ttl > 0 {
		entry.expiresAt = a.now().Add(a.ttl)
	}
	a.mu.Lock()
	a.indices[knowledgeBaseID] = entry
	a.mu.Unlock()
}

func (a *Cache) Invalidate(knowledgeBaseID string) {
	a.mu.Lock()
	delete(a.indices, knowledgeBaseID)
	a.mu.Unlock()
}
