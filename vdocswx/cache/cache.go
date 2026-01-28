/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	bolt "go.etcd.io/bbolt"

	"github.com/cloudwego/eino/vdocswx/config"
	"github.com/cloudwego/eino/vdocswx/stats"
)

// Cache 缓存接口
type Cache interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) bool
	Clear() error
	Close() error
}

// L1Cache L1内存缓存
type L1Cache struct {
	lru    *lru.Cache
	ttlMap map[string]time.Time
	stats  *stats.Collector
	mu     sync.RWMutex
}

// NewL1Cache 创建L1缓存
func NewL1Cache(maxSize int, statsCollector *stats.Collector) (*L1Cache, error) {
	cache, err := lru.New(maxSize)
	if err != nil {
		return nil, err
	}

	return &L1Cache{
		lru:    cache,
		ttlMap: make(map[string]time.Time),
		stats:  statsCollector,
	}, nil
}

// Get 获取缓存
func (c *L1Cache) Get(ctx context.Context, key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 检查TTL
	if expiry, ok := c.ttlMap[key]; ok && time.Now().After(expiry) {
		c.mu.RUnlock()
		c.mu.Lock()
		c.lru.Remove(key)
		delete(c.ttlMap, key)
		c.mu.Unlock()
		c.mu.RLock()
		if c.stats != nil {
			c.stats.RecordCacheHit("l1", false)
		}
		return nil, false
	}

	if value, ok := c.lru.Get(key); ok {
		if c.stats != nil {
			c.stats.RecordCacheHit("l1", true)
		}
		return value, true
	}

	if c.stats != nil {
		c.stats.RecordCacheHit("l1", false)
	}
	return nil, false
}

// Set 设置缓存
func (c *L1Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lru.Add(key, value)
	if ttl > 0 {
		c.ttlMap[key] = time.Now().Add(ttl)
	}

	return nil
}

// Delete 删除缓存
func (c *L1Cache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lru.Remove(key)
	delete(c.ttlMap, key)

	return nil
}

// Exists 检查是否存在
func (c *L1Cache) Exists(ctx context.Context, key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lru.Contains(key)
}

// Clear 清空缓存
func (c *L1Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lru.Purge()
	c.ttlMap = make(map[string]time.Time)

	return nil
}

// Close 关闭缓存
func (c *L1Cache) Close() error {
	return c.Clear()
}

// L2Cache L2本地存储缓存
type L2Cache struct {
	db     *bolt.DB
	bucket string
	stats  *stats.Collector
}

// cacheEntry 缓存条目
type cacheEntry struct {
	Value     []byte    `json:"value"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewL2Cache 创建L2缓存
func NewL2Cache(path string, statsCollector *stats.Collector) (*L2Cache, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{
		Timeout:      1 * time.Second,
		NoSync:       true,
		FreelistType: bolt.FreelistMapType,
	})
	if err != nil {
		return nil, err
	}

	bucket := "cache"
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucket))
		return err
	})
	if err != nil {
		return nil, err
	}

	return &L2Cache{
		db:     db,
		bucket: bucket,
		stats:  statsCollector,
	}, nil
}

// Get 获取缓存
func (c *L2Cache) Get(ctx context.Context, key string) (interface{}, bool) {
	var entry cacheEntry

	err := c.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(c.bucket))
		if bucket == nil {
			return bolt.ErrBucketNotFound
		}

		data := bucket.Get([]byte(key))
		if data == nil {
			return bolt.ErrKeyRequired
		}

		return json.Unmarshal(data, &entry)
	})

	if err != nil {
		if c.stats != nil {
			c.stats.RecordCacheHit("l2", false)
		}
		return nil, false
	}

	// 检查过期
	if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
		c.Delete(ctx, key)
		if c.stats != nil {
			c.stats.RecordCacheHit("l2", false)
		}
		return nil, false
	}

	if c.stats != nil {
		c.stats.RecordCacheHit("l2", true)
	}
	return entry.Value, true
}

// Set 设置缓存
func (c *L2Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	entry := cacheEntry{
		Value: data,
	}
	if ttl > 0 {
		entry.ExpiresAt = time.Now().Add(ttl)
	}

	entryData, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(c.bucket))
		return bucket.Put([]byte(key), entryData)
	})
}

// Delete 删除缓存
func (c *L2Cache) Delete(ctx context.Context, key string) error {
	return c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(c.bucket))
		return bucket.Delete([]byte(key))
	})
}

// Exists 检查是否存在
func (c *L2Cache) Exists(ctx context.Context, key string) bool {
	var exists bool
	c.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(c.bucket))
		if bucket == nil {
			return nil
		}
		exists = bucket.Get([]byte(key)) != nil
		return nil
	})
	return exists
}

// Clear 清空缓存
func (c *L2Cache) Clear() error {
	return c.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(c.bucket)); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		_, err := tx.CreateBucket([]byte(c.bucket))
		return err
	})
}

// Close 关闭缓存
func (c *L2Cache) Close() error {
	return c.db.Close()
}

// MultiLevelCache 多级缓存
type MultiLevelCache struct {
	l1    *L1Cache
	l2    *L2Cache
	stats *stats.Collector
}

// NewMultiLevelCache 创建多级缓存
func NewMultiLevelCache(cfg *config.CacheConfig, statsCollector *stats.Collector) (*MultiLevelCache, error) {
	l1, err := NewL1Cache(cfg.L1.MaxSize, statsCollector)
	if err != nil {
		return nil, err
	}

	l2, err := NewL2Cache(cfg.L2.Path, statsCollector)
	if err != nil {
		return nil, err
	}

	return &MultiLevelCache{
		l1:    l1,
		l2:    l2,
		stats: statsCollector,
	}, nil
}

// Get 获取缓存
func (c *MultiLevelCache) Get(ctx context.Context, key string) (interface{}, bool) {
	// L1查找
	if value, ok := c.l1.Get(ctx, key); ok {
		return value, true
	}

	// L2查找
	if value, ok := c.l2.Get(ctx, key); ok {
		// 回填L1
		c.l1.Set(ctx, key, value, 0)
		return value, true
	}

	return nil, false
}

// Set 设置缓存
func (c *MultiLevelCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if err := c.l1.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	return c.l2.Set(ctx, key, value, ttl)
}

// Delete 删除缓存
func (c *MultiLevelCache) Delete(ctx context.Context, key string) error {
	c.l1.Delete(ctx, key)
	return c.l2.Delete(ctx, key)
}

// Exists 检查是否存在
func (c *MultiLevelCache) Exists(ctx context.Context, key string) bool {
	return c.l1.Exists(ctx, key) || c.l2.Exists(ctx, key)
}

// Clear 清空缓存
func (c *MultiLevelCache) Clear() error {
	if err := c.l1.Clear(); err != nil {
		return err
	}
	return c.l2.Clear()
}

// Close 关闭缓存
func (c *MultiLevelCache) Close() error {
	if err := c.l1.Close(); err != nil {
		return err
	}
	return c.l2.Close()
}
