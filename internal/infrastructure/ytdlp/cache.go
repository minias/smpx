// internal/infrastructure/ytdlp/cache.go
package ytdlp

import (
	"sync"
	"time"
)

// cacheEntry 캐시 항목 (expires 가 zero 면 만료 없음)
type cacheEntry struct {
	value   string
	expires time.Time
}

// memoCache 스레드 안전 문자열 캐시 (ttl <= 0 이면 만료 없음)
type memoCache struct {
	mu    sync.Mutex
	ttl   time.Duration
	now   func() time.Time
	items map[string]cacheEntry
}

func newMemoCache(ttl time.Duration, now func() time.Time) *memoCache {
	if now == nil {
		now = time.Now
	}
	return &memoCache{ttl: ttl, now: now, items: make(map[string]cacheEntry)}
}

// get 유효한 값 조회 (만료 항목은 삭제)
func (c *memoCache) get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.items[key]
	if !ok {
		return "", false
	}
	if !e.expires.IsZero() && !c.now().Before(e.expires) {
		delete(c.items, key)
		return "", false
	}
	return e.value, true
}

// set 값 저장
func (c *memoCache) set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e := cacheEntry{value: value}
	if c.ttl > 0 {
		e.expires = c.now().Add(c.ttl)
	}
	c.items[key] = e
}
