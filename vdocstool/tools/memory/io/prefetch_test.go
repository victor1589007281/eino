package io

import (
	"context"
	"testing"
	"time"
)

func TestNewSessionPrefetcher(t *testing.T) {
	config := DefaultPrefetchConfig
	prefetcher, err := NewSessionPrefetcher(config)
	if err != nil {
		t.Fatalf("NewSessionPrefetcher failed: %v", err)
	}
	if prefetcher == nil {
		t.Fatal("NewSessionPrefetcher returned nil")
	}
}

func TestSessionPrefetcher_GetPrefetched_Miss(t *testing.T) {
	config := DefaultPrefetchConfig
	prefetcher, err := NewSessionPrefetcher(config)
	if err != nil {
		t.Fatalf("NewSessionPrefetcher failed: %v", err)
	}

	// 获取不存在的会话
	result, ok := prefetcher.GetPrefetched("nonexistent")
	if ok {
		t.Error("GetPrefetched should return false for nonexistent session")
	}
	if result != nil {
		t.Error("Result should be nil for nonexistent session")
	}
}

func TestSessionPrefetcher_GetStats(t *testing.T) {
	config := DefaultPrefetchConfig
	prefetcher, err := NewSessionPrefetcher(config)
	if err != nil {
		t.Fatalf("NewSessionPrefetcher failed: %v", err)
	}

	stats := prefetcher.GetStats()

	// 初始状态应该都是0
	if stats.TotalPrefetches != 0 {
		t.Errorf("TotalPrefetches = %d, want 0", stats.TotalPrefetches)
	}
	if stats.CacheHits != 0 {
		t.Errorf("CacheHits = %d, want 0", stats.CacheHits)
	}
}

func TestSessionPrefetcher_Invalidate(t *testing.T) {
	config := DefaultPrefetchConfig
	prefetcher, err := NewSessionPrefetcher(config)
	if err != nil {
		t.Fatalf("NewSessionPrefetcher failed: %v", err)
	}

	// 测试 Invalidate 不会 panic
	prefetcher.Invalidate("session1")
}

func TestPrefetchConfig_Defaults(t *testing.T) {
	config := DefaultPrefetchConfig

	if config.SessionLookAhead != 5 {
		t.Errorf("SessionLookAhead = %d, want 5", config.SessionLookAhead)
	}
	if config.MaxConcurrent != 10 {
		t.Errorf("MaxConcurrent = %d, want 10", config.MaxConcurrent)
	}
	if config.MaxCacheSize != 1000 {
		t.Errorf("MaxCacheSize = %d, want 1000", config.MaxCacheSize)
	}
	if !config.TriggerOnAccess {
		t.Error("TriggerOnAccess should be true by default")
	}
	if !config.TriggerOnIdle {
		t.Error("TriggerOnIdle should be true by default")
	}
}

func TestTopicData(t *testing.T) {
	topic := &TopicData{
		TopicID:      "topic1",
		Title:        "Test Topic",
		Summary:      "A test topic summary",
		KeyFragments: []string{"key1", "key2"},
		TokenCount:   100,
	}

	if topic.TopicID != "topic1" {
		t.Errorf("TopicID = %s, want topic1", topic.TopicID)
	}
	if topic.TokenCount != 100 {
		t.Errorf("TokenCount = %d, want 100", topic.TokenCount)
	}
}

func TestPrefetchResult(t *testing.T) {
	result := &PrefetchResult{
		SessionID: "session1",
		Topics: []*TopicData{
			{TopicID: "t1"},
			{TopicID: "t2"},
		},
		Entities: map[string][]*Relation{
			"entity1": {
				{Source: "entity1", Target: "entity2", Type: "relates", Weight: 0.8},
			},
		},
		LoadTime:   time.Millisecond * 100,
		CachedAt:   time.Now(),
		AccessedAt: time.Now(),
	}

	if len(result.Topics) != 2 {
		t.Errorf("Topics count = %d, want 2", len(result.Topics))
	}
	if len(result.Entities["entity1"]) != 1 {
		t.Errorf("Entities count = %d, want 1", len(result.Entities["entity1"]))
	}
}

func TestRelation(t *testing.T) {
	rel := &Relation{
		Source: "entity1",
		Target: "entity2",
		Type:   "related_to",
		Weight: 0.9,
	}

	if rel.Source != "entity1" {
		t.Errorf("Source = %s, want entity1", rel.Source)
	}
	if rel.Weight != 0.9 {
		t.Errorf("Weight = %v, want 0.9", rel.Weight)
	}
}

// MockDataLoader 模拟数据加载器 (实现 DataLoader 接口)
type MockDataLoader struct {
	topics    map[string][]*TopicData
	relations map[string][]*Relation
}

func (m *MockDataLoader) LoadRecentTopics(ctx context.Context, sessionID string, limit int) ([]*TopicData, error) {
	if topics, ok := m.topics[sessionID]; ok {
		if len(topics) > limit {
			return topics[:limit], nil
		}
		return topics, nil
	}
	return nil, nil
}

func (m *MockDataLoader) LoadEntityRelations(ctx context.Context, entity string, depth int) ([]*Relation, error) {
	if rels, ok := m.relations[entity]; ok {
		return rels, nil
	}
	return nil, nil
}

func TestSessionPrefetcher_PrefetchOnAccess(t *testing.T) {
	config := DefaultPrefetchConfig
	prefetcher, err := NewSessionPrefetcher(config)
	if err != nil {
		t.Fatalf("NewSessionPrefetcher failed: %v", err)
	}

	loader := &MockDataLoader{
		topics: map[string][]*TopicData{
			"session1": {
				{TopicID: "topic1", Title: "Test"},
			},
		},
		relations: map[string][]*Relation{},
	}

	// 触发预读取
	ctx := context.Background()
	prefetcher.PrefetchOnAccess(ctx, "session1", loader)

	// 等待一小段时间让异步预读取完成
	time.Sleep(time.Millisecond * 100)
}
