// Package cache 提供多级缓存系统
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CacheLevel 缓存级别
type CacheLevel int

const (
	L1Cache CacheLevel = iota // 内存缓存
	L2Cache                   // 磁盘缓存
	L3Cache                   // 分布式缓存
)

// CacheType 缓存类型
type CacheType string

const (
	CacheTypeSearch    CacheType = "search"
	CacheTypeFunction  CacheType = "function"
	CacheTypeCallChain CacheType = "callchain"
	CacheTypeLLM       CacheType = "llm"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	Key       string      `json:"key"`
	Value     interface{} `json:"value"`
	Type      CacheType   `json:"type"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at"`
	Tags      []string    `json:"tags,omitempty"`
	Size      int         `json:"size"`
}

// Cache 缓存接口
type Cache interface {
	Get(ctx context.Context, key string) (*CacheEntry, error)
	Set(ctx context.Context, entry *CacheEntry) error
	Delete(ctx context.Context, key string) error
	DeleteByPattern(ctx context.Context, pattern string) error
	DeleteByTag(ctx context.Context, tag string) error
	Clear(ctx context.Context) error
	Stats() *CacheStats
}

// CacheStats 缓存统计
type CacheStats struct {
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	Sets      int64   `json:"sets"`
	Deletes   int64   `json:"deletes"`
	Evictions int64   `json:"evictions"`
	Size      int64   `json:"size"`
	MaxSize   int64   `json:"max_size"`
	ItemCount int64   `json:"item_count"`
	HitRate   float64 `json:"hit_rate"`
}

// MultiLevelCache 多级缓存
type MultiLevelCache struct {
	l1 Cache
	l2 Cache
	l3 Cache

	stats *MultiLevelStats
	mu    sync.RWMutex
}

// MultiLevelStats 多级缓存统计
type MultiLevelStats struct {
	L1Stats     *CacheStats `json:"l1_stats"`
	L2Stats     *CacheStats `json:"l2_stats"`
	L3Stats     *CacheStats `json:"l3_stats"`
	TotalHits   int64       `json:"total_hits"`
	TotalMisses int64       `json:"total_misses"`
}

// NewMultiLevelCache 创建多级缓存
func NewMultiLevelCache(l1, l2, l3 Cache) *MultiLevelCache {
	return &MultiLevelCache{
		l1:    l1,
		l2:    l2,
		l3:    l3,
		stats: &MultiLevelStats{},
	}
}

// Get 获取缓存
func (c *MultiLevelCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	// L1查询
	if c.l1 != nil {
		if entry, err := c.l1.Get(ctx, key); err == nil && entry != nil {
			c.recordHit(L1Cache)
			return entry, nil
		}
	}

	// L2查询
	if c.l2 != nil {
		if entry, err := c.l2.Get(ctx, key); err == nil && entry != nil {
			c.recordHit(L2Cache)
			// 回填L1
			if c.l1 != nil {
				_ = c.l1.Set(ctx, entry)
			}
			return entry, nil
		}
	}

	// L3查询
	if c.l3 != nil {
		if entry, err := c.l3.Get(ctx, key); err == nil && entry != nil {
			c.recordHit(L3Cache)
			// 回填L1和L2
			if c.l1 != nil {
				_ = c.l1.Set(ctx, entry)
			}
			if c.l2 != nil {
				_ = c.l2.Set(ctx, entry)
			}
			return entry, nil
		}
	}

	c.recordMiss()
	return nil, fmt.Errorf("cache miss: %s", key)
}

// Set 设置缓存
func (c *MultiLevelCache) Set(ctx context.Context, entry *CacheEntry) error {
	var lastErr error

	// 写入所有级别
	if c.l1 != nil {
		if err := c.l1.Set(ctx, entry); err != nil {
			lastErr = err
		}
	}

	if c.l2 != nil {
		if err := c.l2.Set(ctx, entry); err != nil {
			lastErr = err
		}
	}

	if c.l3 != nil {
		if err := c.l3.Set(ctx, entry); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Delete 删除缓存
func (c *MultiLevelCache) Delete(ctx context.Context, key string) error {
	var lastErr error

	if c.l1 != nil {
		if err := c.l1.Delete(ctx, key); err != nil {
			lastErr = err
		}
	}

	if c.l2 != nil {
		if err := c.l2.Delete(ctx, key); err != nil {
			lastErr = err
		}
	}

	if c.l3 != nil {
		if err := c.l3.Delete(ctx, key); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// DeleteByTag 按标签删除
func (c *MultiLevelCache) DeleteByTag(ctx context.Context, tag string) error {
	var lastErr error

	if c.l1 != nil {
		if err := c.l1.DeleteByTag(ctx, tag); err != nil {
			lastErr = err
		}
	}

	if c.l2 != nil {
		if err := c.l2.DeleteByTag(ctx, tag); err != nil {
			lastErr = err
		}
	}

	if c.l3 != nil {
		if err := c.l3.DeleteByTag(ctx, tag); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Clear 清空缓存
func (c *MultiLevelCache) Clear(ctx context.Context) error {
	var lastErr error

	if c.l1 != nil {
		if err := c.l1.Clear(ctx); err != nil {
			lastErr = err
		}
	}

	if c.l2 != nil {
		if err := c.l2.Clear(ctx); err != nil {
			lastErr = err
		}
	}

	if c.l3 != nil {
		if err := c.l3.Clear(ctx); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Stats 获取统计
func (c *MultiLevelCache) Stats() *MultiLevelStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &MultiLevelStats{
		TotalHits:   c.stats.TotalHits,
		TotalMisses: c.stats.TotalMisses,
	}

	if c.l1 != nil {
		stats.L1Stats = c.l1.Stats()
	}
	if c.l2 != nil {
		stats.L2Stats = c.l2.Stats()
	}
	if c.l3 != nil {
		stats.L3Stats = c.l3.Stats()
	}

	return stats
}

func (c *MultiLevelCache) recordHit(level CacheLevel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.TotalHits++
}

func (c *MultiLevelCache) recordMiss() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.TotalMisses++
}

// CacheKeyGenerator 缓存键生成器
type CacheKeyGenerator struct{}

// SearchKey 生成搜索缓存键
func (g *CacheKeyGenerator) SearchKey(pattern string, opts map[string]interface{}) string {
	data, _ := json.Marshal(map[string]interface{}{
		"pattern": pattern,
		"opts":    opts,
	})
	hash := sha256.Sum256(data)
	return fmt.Sprintf("search:%s", hex.EncodeToString(hash[:8]))
}

// FunctionKey 生成函数分析缓存键
func (g *CacheKeyGenerator) FunctionKey(name, fileHash string) string {
	return fmt.Sprintf("func:%s:%s", name, fileHash[:8])
}

// CallChainKey 生成调用链缓存键
func (g *CacheKeyGenerator) CallChainKey(function, direction string, depth int) string {
	return fmt.Sprintf("callchain:%s:%s:%d", function, direction, depth)
}

// LLMResponseKey 生成LLM响应缓存键
func (g *CacheKeyGenerator) LLMResponseKey(prompt, model string) string {
	hash := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("llm:%s:%s", model, hex.EncodeToString(hash[:8]))
}

// CacheInvalidator 缓存失效器
type CacheInvalidator struct {
	cache *MultiLevelCache
}

// NewCacheInvalidator 创建缓存失效器
func NewCacheInvalidator(cache *MultiLevelCache) *CacheInvalidator {
	return &CacheInvalidator{cache: cache}
}

// InvalidateOnFileChange 文件变更时失效缓存
func (i *CacheInvalidator) InvalidateOnFileChange(ctx context.Context, files []string) error {
	// 失效搜索缓存
	if err := i.cache.DeleteByTag(ctx, "search"); err != nil {
		return err
	}

	// 失效函数缓存（涉及这些文件的）
	for _, file := range files {
		if err := i.cache.DeleteByTag(ctx, fmt.Sprintf("file:%s", file)); err != nil {
			return err
		}
	}

	return nil
}
