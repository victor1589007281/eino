package cache

import (
	"context"
	"testing"
	"time"
)

func TestLRUCache_BasicOperations(t *testing.T) {
	cache := NewLRUCache(100, time.Minute)
	ctx := context.Background()

	// Test Set and Get
	err := cache.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	value, ok := cache.Get(ctx, "key1")
	if !ok {
		t.Fatal("Get failed: key not found")
	}
	if value != "value1" {
		t.Errorf("Get returned wrong value: got %v, want %v", value, "value1")
	}

	// Test Get non-existent key
	_, ok = cache.Get(ctx, "nonexistent")
	if ok {
		t.Error("Get should return false for non-existent key")
	}

	// Test Delete
	err = cache.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, ok = cache.Get(ctx, "key1")
	if ok {
		t.Error("Get should return false after Delete")
	}
}

func TestLRUCache_Expiration(t *testing.T) {
	cache := NewLRUCache(100, 50*time.Millisecond)
	ctx := context.Background()

	err := cache.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should exist immediately
	_, ok := cache.Get(ctx, "key1")
	if !ok {
		t.Fatal("Key should exist immediately after Set")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, ok = cache.Get(ctx, "key1")
	if ok {
		t.Error("Key should be expired")
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	cache := NewLRUCache(3, time.Minute)
	ctx := context.Background()

	// Fill cache
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	cache.Set(ctx, "key3", "value3", 0)

	// Access key1 to make it recently used
	cache.Get(ctx, "key1")

	// Add key4, should evict key2 (least recently used)
	cache.Set(ctx, "key4", "value4", 0)

	// key2 should be evicted
	_, ok := cache.Get(ctx, "key2")
	if ok {
		t.Error("key2 should be evicted")
	}

	// key1 should still exist
	_, ok = cache.Get(ctx, "key1")
	if !ok {
		t.Error("key1 should still exist")
	}
}

func TestLRUCache_Stats(t *testing.T) {
	cache := NewLRUCache(100, time.Minute)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Get(ctx, "key1")         // hit
	cache.Get(ctx, "nonexistent")  // miss

	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
	if stats.Sets != 1 {
		t.Errorf("Expected 1 set, got %d", stats.Sets)
	}
}

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(100, time.Minute)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	err := cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	_, ok := cache.Get(ctx, "key1")
	if ok {
		t.Error("Cache should be empty after Clear")
	}
}

func TestMultiLevelCache(t *testing.T) {
	l1 := NewLRUCache(10, time.Minute)
	l2 := NewLRUCache(100, time.Minute)
	
	hitStats := make(map[int]int)
	missStats := make(map[int]int)
	
	statsFunc := func(level int, hit bool) {
		if hit {
			hitStats[level]++
		} else {
			missStats[level]++
		}
	}
	
	multi := NewMultiLevelCache([]Cache{l1, l2}, statsFunc)
	ctx := context.Background()

	// Set in L2 only
	l2.Set(ctx, "key1", "value1", 0)

	// Get should populate L1
	value, ok := multi.Get(ctx, "key1")
	if !ok {
		t.Fatal("Get failed")
	}
	if value != "value1" {
		t.Errorf("Wrong value: got %v, want value1", value)
	}

	// L1 should now have the value
	_, ok = l1.Get(ctx, "key1")
	if !ok {
		t.Error("L1 should have the value after multi-level get")
	}
}

func TestQueryCache(t *testing.T) {
	qc := NewQueryCache(100, time.Minute)
	ctx := context.Background()

	query := "SELECT * FROM insurance WHERE type = ?"
	params := map[string]interface{}{"type": "health"}
	result := []string{"result1", "result2"}

	// Set
	err := qc.Set(ctx, query, params, result, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get
	cached, ok := qc.Get(ctx, query, params)
	if !ok {
		t.Fatal("Get failed")
	}

	cachedResult, ok := cached.([]string)
	if !ok {
		t.Fatal("Wrong type returned")
	}
	if len(cachedResult) != 2 {
		t.Errorf("Wrong result length: got %d, want 2", len(cachedResult))
	}

	// Get with different params should miss
	_, ok = qc.Get(ctx, query, map[string]interface{}{"type": "life"})
	if ok {
		t.Error("Should miss with different params")
	}
}

func TestVerifyCache(t *testing.T) {
	vc := NewVerifyCache(100, time.Minute)
	ctx := context.Background()

	query := "保险法第16条规定是什么"
	result := &VerificationResult{
		Query:     query,
		Result:    true,
		Sources:   []string{"保险法", "银保监会解释"},
		Timestamp: time.Now(),
	}

	// Set
	err := vc.Set(ctx, query, result, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get
	cached, ok := vc.Get(ctx, query)
	if !ok {
		t.Fatal("Get failed")
	}
	if cached.Query != query {
		t.Errorf("Wrong query: got %s, want %s", cached.Query, query)
	}
	if !cached.Result {
		t.Error("Wrong result")
	}
}

func TestIndexCache(t *testing.T) {
	ic := NewIndexCache(100, time.Minute)
	ctx := context.Background()

	term := "保险责任"
	result := &IndexResult{
		Term:      term,
		DocIDs:    []string{"doc1", "doc2", "doc3"},
		Positions: []int{10, 25, 42},
		Timestamp: time.Now(),
	}

	// Set
	err := ic.Set(ctx, term, result, 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get
	cached, ok := ic.Get(ctx, term)
	if !ok {
		t.Fatal("Get failed")
	}
	if cached.Term != term {
		t.Errorf("Wrong term: got %s, want %s", cached.Term, term)
	}
	if len(cached.DocIDs) != 3 {
		t.Errorf("Wrong DocIDs count: got %d, want 3", len(cached.DocIDs))
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	cache := NewLRUCache(10000, time.Minute)
	ctx := context.Background()

	// Prepopulate
	for i := 0; i < 1000; i++ {
		cache.Set(ctx, string(rune('A'+i)), i, 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(ctx, string(rune('A'+(i%1000))))
	}
}

func BenchmarkLRUCache_Set(b *testing.B) {
	cache := NewLRUCache(10000, time.Minute)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(ctx, string(rune('A'+(i%1000))), i, 0)
	}
}
