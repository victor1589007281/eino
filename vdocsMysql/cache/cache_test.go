package cache

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestLRUCache(t *testing.T) {
	cache := NewLRUCache(&LRUCacheConfig{
		MaxSize: 3,
	})

	ctx := context.Background()

	// Test Set and Get
	t.Run("SetAndGet", func(t *testing.T) {
		cache.Set(ctx, "key1", "value1", time.Minute)
		cache.Set(ctx, "key2", "value2", time.Minute)

		val, ok := cache.Get(ctx, "key1")
		if !ok {
			t.Error("Expected key1 to exist")
		}
		if val.(string) != "value1" {
			t.Errorf("Expected value1, got %v", val)
		}

		val, ok = cache.Get(ctx, "key2")
		if !ok {
			t.Error("Expected key2 to exist")
		}
		if val.(string) != "value2" {
			t.Errorf("Expected value2, got %v", val)
		}
	})

	// Test cache miss
	t.Run("CacheMiss", func(t *testing.T) {
		_, ok := cache.Get(ctx, "nonexistent")
		if ok {
			t.Error("Expected cache miss for nonexistent key")
		}
	})

	// Test eviction
	t.Run("Eviction", func(t *testing.T) {
		cache.Clear(ctx)
		
		cache.Set(ctx, "a", 1, time.Minute)
		cache.Set(ctx, "b", 2, time.Minute)
		cache.Set(ctx, "c", 3, time.Minute)
		cache.Set(ctx, "d", 4, time.Minute) // Should evict 'a'

		_, ok := cache.Get(ctx, "a")
		if ok {
			t.Error("Key 'a' should have been evicted")
		}

		_, ok = cache.Get(ctx, "d")
		if !ok {
			t.Error("Key 'd' should exist")
		}
	})

	// Test LRU ordering
	t.Run("LRUOrdering", func(t *testing.T) {
		cache.Clear(ctx)
		
		cache.Set(ctx, "a", 1, time.Minute)
		cache.Set(ctx, "b", 2, time.Minute)
		cache.Set(ctx, "c", 3, time.Minute)
		
		// Access 'a' to make it most recently used
		cache.Get(ctx, "a")
		
		// Add new item, should evict 'b' (least recently used)
		cache.Set(ctx, "d", 4, time.Minute)

		_, ok := cache.Get(ctx, "b")
		if ok {
			t.Error("Key 'b' should have been evicted")
		}

		_, ok = cache.Get(ctx, "a")
		if !ok {
			t.Error("Key 'a' should still exist")
		}
	})

	// Test TTL expiration
	t.Run("TTLExpiration", func(t *testing.T) {
		cache.Clear(ctx)
		
		cache.Set(ctx, "shortlived", "value", 50*time.Millisecond)
		
		// Should exist immediately
		_, ok := cache.Get(ctx, "shortlived")
		if !ok {
			t.Error("Key should exist before TTL")
		}

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		_, ok = cache.Get(ctx, "shortlived")
		if ok {
			t.Error("Key should have expired")
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		cache.Clear(ctx)
		
		cache.Set(ctx, "todelete", "value", time.Minute)
		cache.Delete(ctx, "todelete")

		_, ok := cache.Get(ctx, "todelete")
		if ok {
			t.Error("Key should be deleted")
		}
	})

	// Test Clear
	t.Run("Clear", func(t *testing.T) {
		cache.Set(ctx, "x", 1, time.Minute)
		cache.Set(ctx, "y", 2, time.Minute)
		
		cache.Clear(ctx)

		_, ok := cache.Get(ctx, "x")
		if ok {
			t.Error("Cache should be cleared")
		}
	})

	// Test Stats
	t.Run("Stats", func(t *testing.T) {
		cache.Clear(ctx)
		
		cache.Set(ctx, "k1", "v1", time.Minute)
		cache.Get(ctx, "k1") // Hit
		cache.Get(ctx, "k1") // Hit
		cache.Get(ctx, "missing") // Miss

		stats := cache.Stats()
		if stats.Hits < 2 {
			t.Errorf("Expected at least 2 hits, got %d", stats.Hits)
		}
		if stats.Misses < 1 {
			t.Errorf("Expected at least 1 miss, got %d", stats.Misses)
		}
		if stats.Size != 1 {
			t.Errorf("Expected size 1, got %d", stats.Size)
		}
	})

	// Test Update existing key
	t.Run("UpdateExisting", func(t *testing.T) {
		cache.Clear(ctx)
		
		cache.Set(ctx, "update", "old", time.Minute)
		cache.Set(ctx, "update", "new", time.Minute)

		val, _ := cache.Get(ctx, "update")
		if val.(string) != "new" {
			t.Errorf("Expected 'new', got %v", val)
		}
	})
}

func TestLRUCacheOnEvict(t *testing.T) {
	evicted := make(map[string]interface{})
	var mu sync.Mutex

	cache := NewLRUCache(&LRUCacheConfig{
		MaxSize: 2,
		OnEvict: func(key string, value interface{}) {
			mu.Lock()
			evicted[key] = value
			mu.Unlock()
		},
	})

	ctx := context.Background()

	cache.Set(ctx, "a", 1, time.Minute)
	cache.Set(ctx, "b", 2, time.Minute)
	cache.Set(ctx, "c", 3, time.Minute) // Evicts 'a'

	mu.Lock()
	defer mu.Unlock()
	if evicted["a"] != 1 {
		t.Error("OnEvict callback not called correctly")
	}
}

func TestShardedLRUCache(t *testing.T) {
	cache := NewShardedLRUCache(&ShardedLRUCacheConfig{
		MaxSize:    100,
		ShardCount: 4,
	})

	ctx := context.Background()

	// Test basic operations
	t.Run("BasicOperations", func(t *testing.T) {
		cache.Set(ctx, "key1", "value1", time.Minute)
		cache.Set(ctx, "key2", "value2", time.Minute)

		val, ok := cache.Get(ctx, "key1")
		if !ok || val.(string) != "value1" {
			t.Error("Failed to get key1")
		}

		val, ok = cache.Get(ctx, "key2")
		if !ok || val.(string) != "value2" {
			t.Error("Failed to get key2")
		}
	})

	// Test concurrent access
	t.Run("ConcurrentAccess", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 100

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := string(rune('A' + (id % 26)))
				cache.Set(ctx, key, id, time.Minute)
				cache.Get(ctx, key)
			}(i)
		}

		wg.Wait()
	})

	// Test stats aggregation
	t.Run("StatsAggregation", func(t *testing.T) {
		cache.Clear(ctx)
		
		for i := 0; i < 10; i++ {
			key := string(rune('a' + i))
			cache.Set(ctx, key, i, time.Minute)
			cache.Get(ctx, key)
		}

		stats := cache.Stats()
		if stats.Size != 10 {
			t.Errorf("Expected size 10, got %d", stats.Size)
		}
		if stats.Hits < 10 {
			t.Errorf("Expected at least 10 hits, got %d", stats.Hits)
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		cache.Set(ctx, "todel", "val", time.Minute)
		cache.Delete(ctx, "todel")

		_, ok := cache.Get(ctx, "todel")
		if ok {
			t.Error("Key should be deleted")
		}
	})
}

func TestMultiLayerCache(t *testing.T) {
	l1 := NewLRUCache(&LRUCacheConfig{MaxSize: 10})
	l2 := NewLRUCache(&LRUCacheConfig{MaxSize: 100})
	l3 := NewLRUCache(&LRUCacheConfig{MaxSize: 1000})

	cache := NewMultiLayerCache(&MultiLayerCacheConfig{
		L1: l1,
		L2: l2,
		L3: l3,
	})

	ctx := context.Background()

	// Test Set populates all layers
	t.Run("SetPopulatesAllLayers", func(t *testing.T) {
		cache.Set(ctx, "multi", "value", time.Minute)

		// All layers should have the value
		val, ok := l1.Get(ctx, "multi")
		if !ok || val.(string) != "value" {
			t.Error("L1 should have value")
		}

		val, ok = l2.Get(ctx, "multi")
		if !ok || val.(string) != "value" {
			t.Error("L2 should have value")
		}

		val, ok = l3.Get(ctx, "multi")
		if !ok || val.(string) != "value" {
			t.Error("L3 should have value")
		}
	})

	// Test Get from L1
	t.Run("GetFromL1", func(t *testing.T) {
		l1.Set(ctx, "l1only", "l1value", time.Minute)

		val, ok := cache.Get(ctx, "l1only")
		if !ok || val.(string) != "l1value" {
			t.Error("Should get from L1")
		}
	})

	// Test Get from L2 (populates L1)
	t.Run("GetFromL2", func(t *testing.T) {
		l1.Clear(ctx)
		l2.Set(ctx, "l2only", "l2value", time.Minute)

		val, ok := cache.Get(ctx, "l2only")
		if !ok || val.(string) != "l2value" {
			t.Error("Should get from L2")
		}

		// L1 should now have it
		val, ok = l1.Get(ctx, "l2only")
		if !ok || val.(string) != "l2value" {
			t.Error("L1 should be populated from L2")
		}
	})

	// Test Get from L3 (populates L1 and L2)
	t.Run("GetFromL3", func(t *testing.T) {
		l1.Clear(ctx)
		l2.Clear(ctx)
		l3.Set(ctx, "l3only", "l3value", time.Hour)

		val, ok := cache.Get(ctx, "l3only")
		if !ok || val.(string) != "l3value" {
			t.Error("Should get from L3")
		}

		// L1 and L2 should now have it
		val, ok = l1.Get(ctx, "l3only")
		if !ok {
			t.Error("L1 should be populated from L3")
		}
		val, ok = l2.Get(ctx, "l3only")
		if !ok {
			t.Error("L2 should be populated from L3")
		}
	})

	// Test Delete removes from all layers
	t.Run("DeleteFromAllLayers", func(t *testing.T) {
		cache.Set(ctx, "todel", "val", time.Minute)
		cache.Delete(ctx, "todel")

		if _, ok := l1.Get(ctx, "todel"); ok {
			t.Error("L1 should not have deleted key")
		}
		if _, ok := l2.Get(ctx, "todel"); ok {
			t.Error("L2 should not have deleted key")
		}
		if _, ok := l3.Get(ctx, "todel"); ok {
			t.Error("L3 should not have deleted key")
		}
	})

	// Test Clear
	t.Run("ClearAllLayers", func(t *testing.T) {
		cache.Set(ctx, "x", 1, time.Minute)
		cache.Clear(ctx)

		if _, ok := l1.Get(ctx, "x"); ok {
			t.Error("L1 should be cleared")
		}
		if _, ok := l2.Get(ctx, "x"); ok {
			t.Error("L2 should be cleared")
		}
		if _, ok := l3.Get(ctx, "x"); ok {
			t.Error("L3 should be cleared")
		}
	})
}

func TestQueryCache(t *testing.T) {
	cache := NewQueryCache(100, time.Minute)
	ctx := context.Background()

	// Test SetQuery and GetQuery
	t.Run("SetAndGetQuery", func(t *testing.T) {
		params := map[string]interface{}{"limit": 10, "offset": 0}
		result := map[string]string{"key": "value"}

		err := cache.SetQuery(ctx, "SELECT * FROM test", params, result)
		if err != nil {
			t.Fatalf("SetQuery failed: %v", err)
		}

		data, ok := cache.GetQuery(ctx, "SELECT * FROM test", params)
		if !ok {
			t.Error("GetQuery should find cached result")
		}
		if data == nil {
			t.Error("GetQuery should return data")
		}
	})

	// Test cache miss with different params
	t.Run("CacheMissWithDifferentParams", func(t *testing.T) {
		params1 := map[string]interface{}{"limit": 10}
		params2 := map[string]interface{}{"limit": 20}

		cache.SetQuery(ctx, "query", params1, "result1")

		_, ok := cache.GetQuery(ctx, "query", params2)
		if ok {
			t.Error("Should not find result with different params")
		}
	})

	// Test Stats
	t.Run("Stats", func(t *testing.T) {
		stats := cache.Stats()
		if stats == nil {
			t.Error("Stats should not be nil")
		}
	})
}

func TestCacheConcurrency(t *testing.T) {
	cache := NewShardedLRUCache(&ShardedLRUCacheConfig{
		MaxSize:    1000,
		ShardCount: 16,
	})

	ctx := context.Background()
	var wg sync.WaitGroup
	numOps := 1000

	// Concurrent writes
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune('a' + (id % 26)))
			cache.Set(ctx, key, id, time.Minute)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune('a' + (id % 26)))
			cache.Get(ctx, key)
		}(i)
	}

	// Concurrent deletes
	for i := 0; i < numOps/10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune('a' + (id % 26)))
			cache.Delete(ctx, key)
		}(i)
	}

	wg.Wait()

	// Should not panic or deadlock
	stats := cache.Stats()
	t.Logf("After concurrent ops: size=%d, hits=%d, misses=%d",
		stats.Size, stats.Hits, stats.Misses)
}

func BenchmarkLRUCache(b *testing.B) {
	cache := NewLRUCache(&LRUCacheConfig{MaxSize: 10000})
	ctx := context.Background()

	b.Run("Set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cache.Set(ctx, string(rune(i)), i, time.Minute)
		}
	})

	b.Run("Get", func(b *testing.B) {
		for i := 0; i < 1000; i++ {
			cache.Set(ctx, string(rune(i)), i, time.Minute)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.Get(ctx, string(rune(i%1000)))
		}
	})
}

func BenchmarkShardedLRUCache(b *testing.B) {
	cache := NewShardedLRUCache(&ShardedLRUCacheConfig{
		MaxSize:    10000,
		ShardCount: 16,
	})
	ctx := context.Background()

	b.Run("Set", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				cache.Set(ctx, string(rune(i)), i, time.Minute)
				i++
			}
		})
	})

	b.Run("Get", func(b *testing.B) {
		for i := 0; i < 1000; i++ {
			cache.Set(ctx, string(rune(i)), i, time.Minute)
		}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				cache.Get(ctx, string(rune(i%1000)))
				i++
			}
		})
	})
}
