// Package cache 提供多级缓存系统
package cache

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// LRUCache LRU内存缓存 (L1)
type LRUCache struct {
	maxSize int
	ttl     time.Duration

	items     map[string]*list.Element
	evictList *list.List

	stats *CacheStats
	mu    sync.RWMutex

	// 标签索引
	tagIndex map[string]map[string]struct{}
}

type lruEntry struct {
	key       string
	entry     *CacheEntry
	expiresAt time.Time
}

// NewLRUCache 创建LRU缓存
func NewLRUCache(maxSize int, ttl time.Duration) *LRUCache {
	c := &LRUCache{
		maxSize:   maxSize,
		ttl:       ttl,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
		stats:     &CacheStats{MaxSize: int64(maxSize)},
		tagIndex:  make(map[string]map[string]struct{}),
	}

	// 启动过期清理
	go c.cleanupLoop()

	return c
}

// Get 获取缓存
func (c *LRUCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*lruEntry)

		// 检查是否过期
		if time.Now().After(entry.expiresAt) {
			c.removeElement(elem)
			c.stats.Misses++
			return nil, nil
		}

		// 移动到前面
		c.evictList.MoveToFront(elem)
		c.stats.Hits++
		return entry.entry, nil
	}

	c.stats.Misses++
	return nil, nil
}

// Set 设置缓存
func (c *LRUCache) Set(ctx context.Context, entry *CacheEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiresAt := time.Now().Add(c.ttl)
	if !entry.ExpiresAt.IsZero() {
		expiresAt = entry.ExpiresAt
	}

	// 更新已存在的条目
	if elem, ok := c.items[entry.Key]; ok {
		c.evictList.MoveToFront(elem)
		oldEntry := elem.Value.(*lruEntry)
		// 更新标签索引
		c.removeFromTagIndex(oldEntry.entry)
		elem.Value = &lruEntry{
			key:       entry.Key,
			entry:     entry,
			expiresAt: expiresAt,
		}
		c.addToTagIndex(entry)
		c.stats.Sets++
		return nil
	}

	// 检查容量
	for c.evictList.Len() >= c.maxSize {
		c.evict()
	}

	// 添加新条目
	elem := c.evictList.PushFront(&lruEntry{
		key:       entry.Key,
		entry:     entry,
		expiresAt: expiresAt,
	})
	c.items[entry.Key] = elem
	c.addToTagIndex(entry)

	c.stats.Sets++
	c.stats.ItemCount++
	c.stats.Size += int64(entry.Size)

	return nil
}

// Delete 删除缓存
func (c *LRUCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
		c.stats.Deletes++
	}

	return nil
}

// DeleteByPattern 按模式删除
func (c *LRUCache) DeleteByPattern(ctx context.Context, pattern string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 简单的前缀匹配
	toDelete := make([]string, 0)
	for key := range c.items {
		if matchPattern(key, pattern) {
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		if elem, ok := c.items[key]; ok {
			c.removeElement(elem)
			c.stats.Deletes++
		}
	}

	return nil
}

// DeleteByTag 按标签删除
func (c *LRUCache) DeleteByTag(ctx context.Context, tag string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if keys, ok := c.tagIndex[tag]; ok {
		for key := range keys {
			if elem, ok := c.items[key]; ok {
				c.removeElement(elem)
				c.stats.Deletes++
			}
		}
		delete(c.tagIndex, tag)
	}

	return nil
}

// Clear 清空缓存
func (c *LRUCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictList.Init()
	c.tagIndex = make(map[string]map[string]struct{})
	c.stats.ItemCount = 0
	c.stats.Size = 0

	return nil
}

// Stats 获取统计
func (c *LRUCache) Stats() *CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := *c.stats
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}
	return &stats
}

func (c *LRUCache) evict() {
	elem := c.evictList.Back()
	if elem != nil {
		c.removeElement(elem)
		c.stats.Evictions++
	}
}

func (c *LRUCache) removeElement(elem *list.Element) {
	c.evictList.Remove(elem)
	entry := elem.Value.(*lruEntry)
	delete(c.items, entry.key)
	c.removeFromTagIndex(entry.entry)
	c.stats.ItemCount--
	c.stats.Size -= int64(entry.entry.Size)
}

func (c *LRUCache) addToTagIndex(entry *CacheEntry) {
	for _, tag := range entry.Tags {
		if c.tagIndex[tag] == nil {
			c.tagIndex[tag] = make(map[string]struct{})
		}
		c.tagIndex[tag][entry.Key] = struct{}{}
	}
}

func (c *LRUCache) removeFromTagIndex(entry *CacheEntry) {
	for _, tag := range entry.Tags {
		if keys, ok := c.tagIndex[tag]; ok {
			delete(keys, entry.Key)
			if len(keys) == 0 {
				delete(c.tagIndex, tag)
			}
		}
	}
}

func (c *LRUCache) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

func (c *LRUCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	toDelete := make([]*list.Element, 0)

	for elem := c.evictList.Back(); elem != nil; elem = elem.Prev() {
		entry := elem.Value.(*lruEntry)
		if now.After(entry.expiresAt) {
			toDelete = append(toDelete, elem)
		}
	}

	for _, elem := range toDelete {
		c.removeElement(elem)
		c.stats.Evictions++
	}
}

func matchPattern(s, pattern string) bool {
	// 简单的前缀匹配
	if len(pattern) == 0 {
		return true
	}
	if pattern[len(pattern)-1] == '*' {
		return len(s) >= len(pattern)-1 && s[:len(pattern)-1] == pattern[:len(pattern)-1]
	}
	return s == pattern
}
