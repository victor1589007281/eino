// Package embedding 向量缓存实现
package embedding

import (
	"context"
	"sync"
	"time"
)

// MemoryCache 内存缓存
type MemoryCache struct {
	data    map[string]*cacheEntry
	maxSize int
	ttl     time.Duration
	mu      sync.RWMutex
}

type cacheEntry struct {
	vector    []float64
	createdAt time.Time
}

// NewMemoryCache 创建内存缓存
func NewMemoryCache(maxSize int, ttl time.Duration) *MemoryCache {
	if maxSize <= 0 {
		maxSize = 10000
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	cache := &MemoryCache{
		data:    make(map[string]*cacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}

	// 启动清理协程
	go cache.cleanup()

	return cache
}

// Get 获取缓存
func (c *MemoryCache) Get(ctx context.Context, key string) ([]float64, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[key]
	if !ok {
		return nil, false, nil
	}

	// 检查是否过期
	if time.Since(entry.createdAt) > c.ttl {
		return nil, false, nil
	}

	return entry.vector, true, nil
}

// Set 设置缓存
func (c *MemoryCache) Set(ctx context.Context, key string, vector []float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查容量
	if len(c.data) >= c.maxSize {
		c.evict()
	}

	c.data[key] = &cacheEntry{
		vector:    vector,
		createdAt: time.Now(),
	}

	return nil
}

// Delete 删除缓存
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key)
	return nil
}

// Clear 清空缓存
func (c *MemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]*cacheEntry)
	return nil
}

// evict 驱逐旧数据
func (c *MemoryCache) evict() {
	// 简单策略：删除最旧的 10%
	toDelete := c.maxSize / 10
	if toDelete < 1 {
		toDelete = 1
	}

	var oldest []string
	for key := range c.data {
		oldest = append(oldest, key)
		if len(oldest) >= toDelete {
			break
		}
	}

	for _, key := range oldest {
		delete(c.data, key)
	}
}

// cleanup 定期清理过期数据
func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.data {
			if now.Sub(entry.createdAt) > c.ttl {
				delete(c.data, key)
			}
		}
		c.mu.Unlock()
	}
}

// Stats 获取缓存统计
func (c *MemoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Size:    len(c.data),
		MaxSize: c.maxSize,
	}
}

// CacheStats 缓存统计
type CacheStats struct {
	Size    int `json:"size"`
	MaxSize int `json:"max_size"`
}
