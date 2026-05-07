package service

import (
	"sync"
	"time"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
)

// versionCache — in-memory cache версий mart'ов.
// Key: mart_name. Value: (version + expiresAt).
// Не пытаемся быть LRU — у нас всего 5 ключей + 1 для "current_version" (см. ADR-004).
type versionCache struct {
	mu      sync.RWMutex
	entries map[string]versionEntry
	ttl     time.Duration
	now     func() time.Time // тестируемый clock
}

type versionEntry struct {
	version   models.MartVersion
	expiresAt time.Time
}

// newVersionCache создаёт cache с заданным TTL.
func newVersionCache(ttl time.Duration) *versionCache {
	return &versionCache{
		entries: make(map[string]versionEntry),
		ttl:     ttl,
		now:     time.Now,
	}
}

// get возвращает version + ok=true, если запись жива.
// Stale записи воспринимаются как miss и удаляются.
func (c *versionCache) get(key string) (models.MartVersion, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return models.MartVersion{}, false
	}
	if c.now().After(e.expiresAt) {
		c.mu.Lock()
		// Double-check на гонке.
		if e2, ok2 := c.entries[key]; ok2 && !c.now().Before(e2.expiresAt) {
			delete(c.entries, key)
		}
		c.mu.Unlock()
		return models.MartVersion{}, false
	}
	return e.version, true
}

// put записывает version с TTL.
func (c *versionCache) put(key string, v models.MartVersion) {
	c.mu.Lock()
	c.entries[key] = versionEntry{
		version:   v,
		expiresAt: c.now().Add(c.ttl),
	}
	c.mu.Unlock()
}

// invalidate сбрасывает запись по ключу (используется на ошибках).
func (c *versionCache) invalidate(key string) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}
