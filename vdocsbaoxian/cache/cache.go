// Package cache provides multi-level caching support.
package cache

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// Cache defines the cache interface.
type Cache interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Stats() *CacheStats
}

// CacheStats represents cache statistics.
type CacheStats struct {
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	Sets      int64 `json:"sets"`
	Deletes   int64 `json:"deletes"`
	Evictions int64 `json:"evictions"`
	Size      int64 `json:"size"`
}

// LRUCache implements an LRU cache.
type LRUCache struct {
	capacity int
	ttl      time.Duration
	mu       sync.RWMutex
	cache    map[string]*list.Element
	list     *list.List
	stats    CacheStats
}

type lruEntry struct {
	key       string
	value     interface{}
	expiresAt time.Time
}

// NewLRUCache creates a new LRU cache.
func NewLRUCache(capacity int, ttl time.Duration) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		ttl:      ttl,
		cache:    make(map[string]*list.Element),
		list:     list.New(),
	}
}

// Get retrieves a value from the cache.
func (c *LRUCache) Get(ctx context.Context, key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[key]; ok {
		entry := elem.Value.(*lruEntry)
		if time.Now().Before(entry.expiresAt) {
			c.list.MoveToFront(elem)
			c.stats.Hits++
			return entry.value, true
		}
		// Expired, remove it
		c.removeElement(elem)
	}

	c.stats.Misses++
	return nil, false
}

// Set stores a value in the cache.
func (c *LRUCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl == 0 {
		ttl = c.ttl
	}

	if elem, ok := c.cache[key]; ok {
		c.list.MoveToFront(elem)
		entry := elem.Value.(*lruEntry)
		entry.value = value
		entry.expiresAt = time.Now().Add(ttl)
		c.stats.Sets++
		return nil
	}

	// Check capacity
	for c.list.Len() >= c.capacity {
		c.evict()
	}

	entry := &lruEntry{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	elem := c.list.PushFront(entry)
	c.cache[key] = elem
	c.stats.Sets++
	c.stats.Size = int64(c.list.Len())

	return nil
}

// Delete removes a value from the cache.
func (c *LRUCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[key]; ok {
		c.removeElement(elem)
		c.stats.Deletes++
	}
	return nil
}

// Clear removes all values from the cache.
func (c *LRUCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*list.Element)
	c.list.Init()
	c.stats.Size = 0
	return nil
}

// Stats returns cache statistics.
func (c *LRUCache) Stats() *CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &CacheStats{
		Hits:      c.stats.Hits,
		Misses:    c.stats.Misses,
		Sets:      c.stats.Sets,
		Deletes:   c.stats.Deletes,
		Evictions: c.stats.Evictions,
		Size:      c.stats.Size,
	}
}

func (c *LRUCache) evict() {
	elem := c.list.Back()
	if elem != nil {
		c.removeElement(elem)
		c.stats.Evictions++
	}
}

func (c *LRUCache) removeElement(elem *list.Element) {
	c.list.Remove(elem)
	entry := elem.Value.(*lruEntry)
	delete(c.cache, entry.key)
	c.stats.Size = int64(c.list.Len())
}

// MultiLevelCache implements a multi-level cache.
type MultiLevelCache struct {
	levels    []Cache
	statsFunc func(level int, hit bool)
}

// NewMultiLevelCache creates a new multi-level cache.
func NewMultiLevelCache(levels []Cache, statsFunc func(level int, hit bool)) *MultiLevelCache {
	return &MultiLevelCache{
		levels:    levels,
		statsFunc: statsFunc,
	}
}

// Get retrieves a value from the multi-level cache.
func (c *MultiLevelCache) Get(ctx context.Context, key string) (interface{}, bool) {
	for i, level := range c.levels {
		if value, ok := level.Get(ctx, key); ok {
			if c.statsFunc != nil {
				c.statsFunc(i+1, true)
			}
			// Populate higher levels
			for j := 0; j < i; j++ {
				c.levels[j].Set(ctx, key, value, 0)
			}
			return value, true
		}
		if c.statsFunc != nil {
			c.statsFunc(i+1, false)
		}
	}
	return nil, false
}

// Set stores a value in all cache levels.
func (c *MultiLevelCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	for _, level := range c.levels {
		if err := level.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes a value from all cache levels.
func (c *MultiLevelCache) Delete(ctx context.Context, key string) error {
	for _, level := range c.levels {
		if err := level.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// Clear removes all values from all cache levels.
func (c *MultiLevelCache) Clear(ctx context.Context) error {
	for _, level := range c.levels {
		if err := level.Clear(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Stats returns combined cache statistics.
func (c *MultiLevelCache) Stats() *CacheStats {
	combined := &CacheStats{}
	for _, level := range c.levels {
		stats := level.Stats()
		combined.Hits += stats.Hits
		combined.Misses += stats.Misses
		combined.Sets += stats.Sets
		combined.Deletes += stats.Deletes
		combined.Evictions += stats.Evictions
		combined.Size += stats.Size
	}
	return combined
}

// QueryCache is a specialized cache for query results.
type QueryCache struct {
	cache Cache
}

// NewQueryCache creates a new query cache.
func NewQueryCache(capacity int, ttl time.Duration) *QueryCache {
	return &QueryCache{
		cache: NewLRUCache(capacity, ttl),
	}
}

// GenerateKey generates a cache key from query parameters.
func (c *QueryCache) GenerateKey(query string, params map[string]interface{}) string {
	data, _ := json.Marshal(map[string]interface{}{
		"query":  query,
		"params": params,
	})
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Get retrieves a cached query result.
func (c *QueryCache) Get(ctx context.Context, query string, params map[string]interface{}) (interface{}, bool) {
	key := c.GenerateKey(query, params)
	return c.cache.Get(ctx, key)
}

// Set caches a query result.
func (c *QueryCache) Set(ctx context.Context, query string, params map[string]interface{}, result interface{}, ttl time.Duration) error {
	key := c.GenerateKey(query, params)
	return c.cache.Set(ctx, key, result, ttl)
}

// VerifyCache is a specialized cache for verification results.
type VerifyCache struct {
	cache Cache
}

// NewVerifyCache creates a new verification cache.
func NewVerifyCache(capacity int, ttl time.Duration) *VerifyCache {
	return &VerifyCache{
		cache: NewLRUCache(capacity, ttl),
	}
}

// VerificationResult represents a cached verification result.
type VerificationResult struct {
	Query     string    `json:"query"`
	Result    bool      `json:"result"`
	Sources   []string  `json:"sources"`
	Timestamp time.Time `json:"timestamp"`
}

// Get retrieves a cached verification result.
func (c *VerifyCache) Get(ctx context.Context, query string) (*VerificationResult, bool) {
	hash := sha256.Sum256([]byte(query))
	key := hex.EncodeToString(hash[:])
	if value, ok := c.cache.Get(ctx, key); ok {
		if result, ok := value.(*VerificationResult); ok {
			return result, true
		}
	}
	return nil, false
}

// Set caches a verification result.
func (c *VerifyCache) Set(ctx context.Context, query string, result *VerificationResult, ttl time.Duration) error {
	hash := sha256.Sum256([]byte(query))
	key := hex.EncodeToString(hash[:])
	return c.cache.Set(ctx, key, result, ttl)
}

// IndexCache is a specialized cache for index lookups.
type IndexCache struct {
	cache Cache
}

// NewIndexCache creates a new index cache.
func NewIndexCache(capacity int, ttl time.Duration) *IndexCache {
	return &IndexCache{
		cache: NewLRUCache(capacity, ttl),
	}
}

// IndexResult represents a cached index result.
type IndexResult struct {
	Term      string   `json:"term"`
	DocIDs    []string `json:"doc_ids"`
	Positions []int    `json:"positions"`
	Timestamp time.Time `json:"timestamp"`
}

// Get retrieves a cached index result.
func (c *IndexCache) Get(ctx context.Context, term string) (*IndexResult, bool) {
	if value, ok := c.cache.Get(ctx, term); ok {
		if result, ok := value.(*IndexResult); ok {
			return result, true
		}
	}
	return nil, false
}

// Set caches an index result.
func (c *IndexCache) Set(ctx context.Context, term string, result *IndexResult, ttl time.Duration) error {
	return c.cache.Set(ctx, term, result, ttl)
}
