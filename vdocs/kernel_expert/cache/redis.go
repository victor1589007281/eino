// Package cache 提供多级缓存系统
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache Redis缓存 (L3)
type RedisCache struct {
	client *redis.Client
	prefix string
	ttl    time.Duration

	stats *redisCacheStats
}

type redisCacheStats struct {
	hits    int64
	misses  int64
	sets    int64
	deletes int64
}

// RedisCacheConfig Redis缓存配置
type RedisCacheConfig struct {
	URL      string
	Prefix   string
	TTL      time.Duration
	Password string
	DB       int
}

// NewRedisCache 创建Redis缓存
func NewRedisCache(config *RedisCacheConfig) (*RedisCache, error) {
	opts, err := redis.ParseURL(config.URL)
	if err != nil {
		// 尝试作为地址解析
		opts = &redis.Options{
			Addr:     config.URL,
			Password: config.Password,
			DB:       config.DB,
		}
	}

	client := redis.NewClient(opts)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	prefix := config.Prefix
	if prefix == "" {
		prefix = "kernel_expert:"
	}

	return &RedisCache{
		client: client,
		prefix: prefix,
		ttl:    config.TTL,
		stats:  &redisCacheStats{},
	}, nil
}

// Get 获取缓存
func (c *RedisCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	fullKey := c.prefix + key

	data, err := c.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		atomic.AddInt64(&c.stats.misses, 1)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	atomic.AddInt64(&c.stats.hits, 1)
	return &entry, nil
}

// Set 设置缓存
func (c *RedisCache) Set(ctx context.Context, entry *CacheEntry) error {
	fullKey := c.prefix + entry.Key

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	ttl := c.ttl
	if !entry.ExpiresAt.IsZero() {
		ttl = time.Until(entry.ExpiresAt)
	}

	if err := c.client.Set(ctx, fullKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	// 添加标签索引
	for _, tag := range entry.Tags {
		tagKey := c.prefix + "tag:" + tag
		c.client.SAdd(ctx, tagKey, entry.Key)
		c.client.Expire(ctx, tagKey, ttl)
	}

	atomic.AddInt64(&c.stats.sets, 1)
	return nil
}

// Delete 删除缓存
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	fullKey := c.prefix + key

	if err := c.client.Del(ctx, fullKey).Err(); err != nil {
		return fmt.Errorf("redis del: %w", err)
	}

	atomic.AddInt64(&c.stats.deletes, 1)
	return nil
}

// DeleteByPattern 按模式删除
func (c *RedisCache) DeleteByPattern(ctx context.Context, pattern string) error {
	fullPattern := c.prefix + pattern

	iter := c.client.Scan(ctx, 0, fullPattern, 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("del: %w", err)
		}
		atomic.AddInt64(&c.stats.deletes, int64(len(keys)))
	}

	return nil
}

// DeleteByTag 按标签删除
func (c *RedisCache) DeleteByTag(ctx context.Context, tag string) error {
	tagKey := c.prefix + "tag:" + tag

	// 获取标签下的所有键
	keys, err := c.client.SMembers(ctx, tagKey).Result()
	if err != nil {
		return fmt.Errorf("smembers: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	// 构建完整键名
	fullKeys := make([]string, len(keys)+1)
	for i, key := range keys {
		fullKeys[i] = c.prefix + key
	}
	fullKeys[len(keys)] = tagKey

	// 删除所有键
	if err := c.client.Del(ctx, fullKeys...).Err(); err != nil {
		return fmt.Errorf("del: %w", err)
	}

	atomic.AddInt64(&c.stats.deletes, int64(len(keys)))
	return nil
}

// Clear 清空缓存
func (c *RedisCache) Clear(ctx context.Context) error {
	pattern := c.prefix + "*"

	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("del: %w", err)
		}
	}

	return nil
}

// Stats 获取统计
func (c *RedisCache) Stats() *CacheStats {
	hits := atomic.LoadInt64(&c.stats.hits)
	misses := atomic.LoadInt64(&c.stats.misses)

	stats := &CacheStats{
		Hits:    hits,
		Misses:  misses,
		Sets:    atomic.LoadInt64(&c.stats.sets),
		Deletes: atomic.LoadInt64(&c.stats.deletes),
	}

	total := hits + misses
	if total > 0 {
		stats.HitRate = float64(hits) / float64(total)
	}

	// 获取键数量
	ctx := context.Background()
	pattern := c.prefix + "*"
	count := int64(0)
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		count++
	}
	stats.ItemCount = count

	return stats
}

// Close 关闭连接
func (c *RedisCache) Close() error {
	return c.client.Close()
}
