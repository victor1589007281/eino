// Package cache provides multi-layer caching for the MySQL Expert Agent.
package cache

import (
	"container/list"
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// Cache provides a multi-layer caching interface.
type Cache interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration)
	Delete(ctx context.Context, key string)
	Clear(ctx context.Context)
	Stats() *CacheStats
}

// CacheStats contains cache statistics.
type CacheStats struct {
	Hits       int64   `json:"hits"`
	Misses     int64   `json:"misses"`
	Size       int64   `json:"size"`
	MaxSize    int64   `json:"max_size"`
	HitRate    float64 `json:"hit_rate"`
	Evictions  int64   `json:"evictions"`
}

// LRUCache implements an LRU cache with TTL support.
type LRUCache struct {
	maxSize    int
	items      map[string]*list.Element
	evictList  *list.List
	mu         sync.RWMutex
	stats      CacheStats
	onEvict    func(key string, value interface{})
}

type cacheEntry struct {
	key       string
	value     interface{}
	expiresAt time.Time
}

// LRUCacheConfig contains configuration for LRU cache.
type LRUCacheConfig struct {
	MaxSize int
	OnEvict func(key string, value interface{})
}

// NewLRUCache creates a new LRU cache.
func NewLRUCache(config *LRUCacheConfig) *LRUCache {
	return &LRUCache{
		maxSize:   config.MaxSize,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
		onEvict:   config.OnEvict,
		stats: CacheStats{
			MaxSize: int64(config.MaxSize),
		},
	}
}

// Get retrieves a value from the cache.
func (c *LRUCache) Get(ctx context.Context, key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*cacheEntry)
		
		// Check if expired
		if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
			c.removeElement(elem)
			atomic.AddInt64(&c.stats.Misses, 1)
			return nil, false
		}

		// Move to front (most recently used)
		c.evictList.MoveToFront(elem)
		atomic.AddInt64(&c.stats.Hits, 1)
		return entry.value, true
	}

	atomic.AddInt64(&c.stats.Misses, 1)
	return nil, false
}

// Set stores a value in the cache.
func (c *LRUCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	// Check if key exists
	if elem, ok := c.items[key]; ok {
		c.evictList.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.value = value
		entry.expiresAt = expiresAt
		return
	}

	// Add new item
	entry := &cacheEntry{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}
	elem := c.evictList.PushFront(entry)
	c.items[key] = elem

	// Evict if necessary
	for c.evictList.Len() > c.maxSize {
		c.evictOldest()
	}

	c.stats.Size = int64(len(c.items))
}

// Delete removes a value from the cache.
func (c *LRUCache) Delete(ctx context.Context, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
	}
}

// Clear removes all items from the cache.
func (c *LRUCache) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, elem := range c.items {
		if c.onEvict != nil {
			entry := elem.Value.(*cacheEntry)
			c.onEvict(key, entry.value)
		}
		delete(c.items, key)
	}
	c.evictList.Init()
	c.stats.Size = 0
}

// Stats returns cache statistics.
func (c *LRUCache) Stats() *CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}
	stats.Size = int64(len(c.items))
	return &stats
}

func (c *LRUCache) evictOldest() {
	elem := c.evictList.Back()
	if elem != nil {
		c.removeElement(elem)
		atomic.AddInt64(&c.stats.Evictions, 1)
	}
}

func (c *LRUCache) removeElement(elem *list.Element) {
	c.evictList.Remove(elem)
	entry := elem.Value.(*cacheEntry)
	delete(c.items, entry.key)
	if c.onEvict != nil {
		c.onEvict(entry.key, entry.value)
	}
}

// ShardedLRUCache provides a sharded LRU cache for better concurrency.
type ShardedLRUCache struct {
	shards    []*LRUCache
	shardMask uint32
}

// ShardedLRUCacheConfig contains configuration for sharded LRU cache.
type ShardedLRUCacheConfig struct {
	MaxSize    int
	ShardCount int
	OnEvict    func(key string, value interface{})
}

// NewShardedLRUCache creates a new sharded LRU cache.
func NewShardedLRUCache(config *ShardedLRUCacheConfig) *ShardedLRUCache {
	shardCount := config.ShardCount
	if shardCount <= 0 {
		shardCount = 16
	}
	// Round up to power of 2
	shardCount = nextPowerOf2(shardCount)

	shardSize := config.MaxSize / shardCount
	if shardSize < 1 {
		shardSize = 1
	}

	shards := make([]*LRUCache, shardCount)
	for i := 0; i < shardCount; i++ {
		shards[i] = NewLRUCache(&LRUCacheConfig{
			MaxSize: shardSize,
			OnEvict: config.OnEvict,
		})
	}

	return &ShardedLRUCache{
		shards:    shards,
		shardMask: uint32(shardCount - 1),
	}
}

func (c *ShardedLRUCache) getShard(key string) *LRUCache {
	hash := fnv32(key)
	return c.shards[hash&c.shardMask]
}

// Get retrieves a value from the cache.
func (c *ShardedLRUCache) Get(ctx context.Context, key string) (interface{}, bool) {
	return c.getShard(key).Get(ctx, key)
}

// Set stores a value in the cache.
func (c *ShardedLRUCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	c.getShard(key).Set(ctx, key, value, ttl)
}

// Delete removes a value from the cache.
func (c *ShardedLRUCache) Delete(ctx context.Context, key string) {
	c.getShard(key).Delete(ctx, key)
}

// Clear removes all items from the cache.
func (c *ShardedLRUCache) Clear(ctx context.Context) {
	for _, shard := range c.shards {
		shard.Clear(ctx)
	}
}

// Stats returns aggregated cache statistics.
func (c *ShardedLRUCache) Stats() *CacheStats {
	stats := &CacheStats{}
	for _, shard := range c.shards {
		s := shard.Stats()
		stats.Hits += s.Hits
		stats.Misses += s.Misses
		stats.Size += s.Size
		stats.MaxSize += s.MaxSize
		stats.Evictions += s.Evictions
	}
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}
	return stats
}

// MultiLayerCache provides L1 (memory) + L2 (optional) + L3 (optional) caching.
type MultiLayerCache struct {
	l1 Cache // Memory cache
	l2 Cache // Optional distributed/local cache
	l3 Cache // Optional persistent cache
}

// MultiLayerCacheConfig contains configuration for multi-layer cache.
type MultiLayerCacheConfig struct {
	L1 Cache
	L2 Cache
	L3 Cache
}

// NewMultiLayerCache creates a new multi-layer cache.
func NewMultiLayerCache(config *MultiLayerCacheConfig) *MultiLayerCache {
	return &MultiLayerCache{
		l1: config.L1,
		l2: config.L2,
		l3: config.L3,
	}
}

// Get retrieves a value, checking each layer.
func (c *MultiLayerCache) Get(ctx context.Context, key string) (interface{}, bool) {
	// Check L1
	if c.l1 != nil {
		if val, ok := c.l1.Get(ctx, key); ok {
			return val, true
		}
	}

	// Check L2
	if c.l2 != nil {
		if val, ok := c.l2.Get(ctx, key); ok {
			// Populate L1
			if c.l1 != nil {
				c.l1.Set(ctx, key, val, 5*time.Minute)
			}
			return val, true
		}
	}

	// Check L3
	if c.l3 != nil {
		if val, ok := c.l3.Get(ctx, key); ok {
			// Populate L1 and L2
			if c.l1 != nil {
				c.l1.Set(ctx, key, val, 5*time.Minute)
			}
			if c.l2 != nil {
				c.l2.Set(ctx, key, val, 30*time.Minute)
			}
			return val, true
		}
	}

	return nil, false
}

// Set stores a value in all cache layers.
func (c *MultiLayerCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	if c.l1 != nil {
		c.l1.Set(ctx, key, value, ttl)
	}
	if c.l2 != nil {
		c.l2.Set(ctx, key, value, ttl*6) // L2 has longer TTL
	}
	if c.l3 != nil {
		c.l3.Set(ctx, key, value, ttl*24) // L3 has longest TTL
	}
}

// Delete removes a value from all cache layers.
func (c *MultiLayerCache) Delete(ctx context.Context, key string) {
	if c.l1 != nil {
		c.l1.Delete(ctx, key)
	}
	if c.l2 != nil {
		c.l2.Delete(ctx, key)
	}
	if c.l3 != nil {
		c.l3.Delete(ctx, key)
	}
}

// Clear removes all items from all cache layers.
func (c *MultiLayerCache) Clear(ctx context.Context) {
	if c.l1 != nil {
		c.l1.Clear(ctx)
	}
	if c.l2 != nil {
		c.l2.Clear(ctx)
	}
	if c.l3 != nil {
		c.l3.Clear(ctx)
	}
}

// Stats returns statistics for all layers.
func (c *MultiLayerCache) Stats() *CacheStats {
	stats := &CacheStats{}
	
	if c.l1 != nil {
		l1Stats := c.l1.Stats()
		stats.Hits += l1Stats.Hits
		stats.Misses += l1Stats.Misses
		stats.Size += l1Stats.Size
	}
	if c.l2 != nil {
		l2Stats := c.l2.Stats()
		stats.Hits += l2Stats.Hits
		stats.Size += l2Stats.Size
	}
	if c.l3 != nil {
		l3Stats := c.l3.Stats()
		stats.Hits += l3Stats.Hits
		stats.Size += l3Stats.Size
	}

	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}

	return stats
}

// QueryCache provides specialized caching for search queries.
type QueryCache struct {
	cache *ShardedLRUCache
	ttl   time.Duration
}

// NewQueryCache creates a new query cache.
func NewQueryCache(maxSize int, ttl time.Duration) *QueryCache {
	return &QueryCache{
		cache: NewShardedLRUCache(&ShardedLRUCacheConfig{
			MaxSize:    maxSize,
			ShardCount: 16,
		}),
		ttl: ttl,
	}
}

// GetQuery retrieves cached query results.
func (c *QueryCache) GetQuery(ctx context.Context, query string, params map[string]interface{}) ([]byte, bool) {
	key := c.buildKey(query, params)
	if val, ok := c.cache.Get(ctx, key); ok {
		return val.([]byte), true
	}
	return nil, false
}

// SetQuery caches query results.
func (c *QueryCache) SetQuery(ctx context.Context, query string, params map[string]interface{}, result interface{}) error {
	key := c.buildKey(query, params)
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	c.cache.Set(ctx, key, data, c.ttl)
	return nil
}

func (c *QueryCache) buildKey(query string, params map[string]interface{}) string {
	data, _ := json.Marshal(params)
	return query + ":" + string(data)
}

// Stats returns cache statistics.
func (c *QueryCache) Stats() *CacheStats {
	return c.cache.Stats()
}

// Helper functions

func fnv32(key string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		hash *= 16777619
		hash ^= uint32(key[i])
	}
	return hash
}

func nextPowerOf2(n int) int {
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	return n
}
