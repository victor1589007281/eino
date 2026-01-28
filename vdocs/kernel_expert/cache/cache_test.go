package cache

import (
	"context"
	"testing"
	"time"
)

func TestLRUCache_Basic(t *testing.T) {
	cache := NewLRUCache(100, time.Hour)
	ctx := context.Background()

	// 测试Set和Get
	entry := &CacheEntry{
		Key:       "test-key",
		Value:     "test-value",
		Type:      CacheTypeSearch,
		CreatedAt: time.Now(),
		Size:      10,
	}

	err := cache.Set(ctx, entry)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := cache.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got == nil {
		t.Fatal("Expected entry, got nil")
	}

	if got.Value != "test-value" {
		t.Errorf("Expected value 'test-value', got '%v'", got.Value)
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	cache := NewLRUCache(3, time.Hour)
	ctx := context.Background()

	// 添加4个条目，第一个应该被淘汰
	for i := 0; i < 4; i++ {
		entry := &CacheEntry{
			Key:   string(rune('a' + i)),
			Value: i,
			Size:  1,
		}
		cache.Set(ctx, entry)
	}

	// 第一个条目应该被淘汰
	got, _ := cache.Get(ctx, "a")
	if got != nil {
		t.Error("Expected 'a' to be evicted")
	}

	// 最后三个应该存在
	for _, key := range []string{"b", "c", "d"} {
		got, _ := cache.Get(ctx, key)
		if got == nil {
			t.Errorf("Expected '%s' to exist", key)
		}
	}
}

func TestLRUCache_Expiration(t *testing.T) {
	cache := NewLRUCache(100, 100*time.Millisecond)
	ctx := context.Background()

	entry := &CacheEntry{
		Key:   "expiring",
		Value: "value",
		Size:  1,
	}

	cache.Set(ctx, entry)

	// 应该能获取到
	got, _ := cache.Get(ctx, "expiring")
	if got == nil {
		t.Fatal("Expected entry to exist")
	}

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 应该已过期
	got, _ = cache.Get(ctx, "expiring")
	if got != nil {
		t.Error("Expected entry to be expired")
	}
}

func TestLRUCache_Tags(t *testing.T) {
	cache := NewLRUCache(100, time.Hour)
	ctx := context.Background()

	// 添加带标签的条目
	entry1 := &CacheEntry{
		Key:   "entry1",
		Value: "value1",
		Tags:  []string{"tag1", "tag2"},
		Size:  1,
	}
	entry2 := &CacheEntry{
		Key:   "entry2",
		Value: "value2",
		Tags:  []string{"tag1"},
		Size:  1,
	}
	entry3 := &CacheEntry{
		Key:   "entry3",
		Value: "value3",
		Tags:  []string{"tag2"},
		Size:  1,
	}

	cache.Set(ctx, entry1)
	cache.Set(ctx, entry2)
	cache.Set(ctx, entry3)

	// 按tag1删除
	cache.DeleteByTag(ctx, "tag1")

	// entry1和entry2应该被删除
	if got, _ := cache.Get(ctx, "entry1"); got != nil {
		t.Error("entry1 should be deleted")
	}
	if got, _ := cache.Get(ctx, "entry2"); got != nil {
		t.Error("entry2 should be deleted")
	}

	// entry3应该存在
	if got, _ := cache.Get(ctx, "entry3"); got == nil {
		t.Error("entry3 should exist")
	}
}

func TestLRUCache_Stats(t *testing.T) {
	cache := NewLRUCache(100, time.Hour)
	ctx := context.Background()

	// 添加条目
	entry := &CacheEntry{Key: "key", Value: "value", Size: 10}
	cache.Set(ctx, entry)

	// 命中
	cache.Get(ctx, "key")
	cache.Get(ctx, "key")

	// 未命中
	cache.Get(ctx, "nonexistent")

	stats := cache.Stats()
	if stats.Hits != 2 {
		t.Errorf("Expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
	if stats.Sets != 1 {
		t.Errorf("Expected 1 set, got %d", stats.Sets)
	}
}

func TestMultiLevelCache(t *testing.T) {
	l1 := NewLRUCache(10, time.Hour)
	l2 := NewLRUCache(100, time.Hour)

	mlc := NewMultiLevelCache(l1, l2, nil)
	ctx := context.Background()

	// 设置到L2
	entry := &CacheEntry{Key: "key", Value: "value", Size: 1}
	l2.Set(ctx, entry)

	// 从多级缓存获取（应该从L2获取并回填L1）
	got, err := mlc.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("Expected entry")
	}

	// 现在L1应该也有了
	l1Got, _ := l1.Get(ctx, "key")
	if l1Got == nil {
		t.Error("Expected L1 to be backfilled")
	}
}

func TestCacheKeyGenerator(t *testing.T) {
	gen := &CacheKeyGenerator{}

	// 测试搜索键
	searchKey := gen.SearchKey("pattern", map[string]interface{}{"limit": 10})
	if searchKey == "" {
		t.Error("Search key should not be empty")
	}

	// 相同输入应该生成相同的键
	searchKey2 := gen.SearchKey("pattern", map[string]interface{}{"limit": 10})
	if searchKey != searchKey2 {
		t.Error("Same input should generate same key")
	}

	// 测试函数键
	funcKey := gen.FunctionKey("do_fork", "abc123def456")
	if funcKey != "func:do_fork:abc123de" {
		t.Errorf("Unexpected function key: %s", funcKey)
	}

	// 测试调用链键
	ccKey := gen.CallChainKey("schedule", "callers", 3)
	if ccKey != "callchain:schedule:callers:3" {
		t.Errorf("Unexpected call chain key: %s", ccKey)
	}

	// 测试LLM响应键
	llmKey := gen.LLMResponseKey("What is fork?", "gpt-4")
	if llmKey == "" {
		t.Error("LLM key should not be empty")
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	cache := NewLRUCache(10000, time.Hour)
	ctx := context.Background()

	// 预填充
	for i := 0; i < 1000; i++ {
		entry := &CacheEntry{
			Key:   string(rune(i)),
			Value: i,
			Size:  1,
		}
		cache.Set(ctx, entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(ctx, string(rune(i%1000)))
	}
}

func BenchmarkLRUCache_Set(b *testing.B) {
	cache := NewLRUCache(10000, time.Hour)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry := &CacheEntry{
			Key:   string(rune(i % 10000)),
			Value: i,
			Size:  1,
		}
		cache.Set(ctx, entry)
	}
}
