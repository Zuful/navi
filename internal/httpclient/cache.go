package httpclient

import (
	"sync"
	"time"
)

// cacheEntry holds a cached value with its expiration time.
type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// Cache is a thread-safe in-memory TTL cache.
type Cache struct {
	mu         sync.RWMutex
	entries    map[string]cacheEntry
	defaultTTL time.Duration
	maxEntries int
}

// NewCache creates a new Cache with the given TTL and max entry count.
func NewCache(defaultTTL time.Duration, maxEntries int) *Cache {
	return &Cache{
		entries:    make(map[string]cacheEntry),
		defaultTTL: defaultTTL,
		maxEntries: maxEntries,
	}
}

// Get retrieves a cached value. Returns nil and false if not found or expired.
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			// Lazy delete expired entry.
			c.mu.Lock()
			delete(c.entries, key)
			c.mu.Unlock()
		}
		return nil, false
	}

	return entry.value, true
}

// Set stores a value in the cache with the default TTL.
func (c *Cache) Set(key string, value []byte) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

// SetWithTTL stores a value in the cache with a custom TTL.
func (c *Cache) SetWithTTL(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict expired entries if at capacity.
	if len(c.entries) >= c.maxEntries {
		c.evictExpired()
	}

	// If still at capacity after eviction, remove oldest entry.
	if len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}

	c.entries[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// evictExpired removes all expired entries. Must be called with lock held.
func (c *Cache) evictExpired() {
	now := time.Now()
	for k, v := range c.entries {
		if now.After(v.expiresAt) {
			delete(c.entries, k)
		}
	}
}

// evictOldest removes the entry with the earliest expiration. Must be called with lock held.
func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true

	for k, v := range c.entries {
		if first || v.expiresAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.expiresAt
			first = false
		}
	}

	if !first {
		delete(c.entries, oldestKey)
	}
}
