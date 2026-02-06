package io

import (
	"context"
	"testing"
	"time"
)

func TestNewParallelPipeline(t *testing.T) {
	prefetcher, err := NewSessionPrefetcher(DefaultPrefetchConfig)
	if err != nil {
		t.Fatalf("NewSessionPrefetcher failed: %v", err)
	}

	pipeline := NewParallelPipeline(prefetcher, DefaultPipelineConfig)
	if pipeline == nil {
		t.Fatal("NewParallelPipeline returned nil")
	}
}

func TestPipelineConfig_Defaults(t *testing.T) {
	config := DefaultPipelineConfig

	if !config.ParallelRetrieve {
		t.Error("ParallelRetrieve should be true by default")
	}
	if !config.AsyncProcessing {
		t.Error("AsyncProcessing should be true by default")
	}
	if config.TokenBudget != 4000 {
		t.Errorf("TokenBudget = %d, want 4000", config.TokenBudget)
	}
	if config.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", config.Timeout)
	}
}

func TestSearchResult(t *testing.T) {
	result := &SearchResult{
		ID:         "result1",
		Content:    "Test content",
		Score:      0.95,
		Tier:       "L1",
		TokenCount: 50,
		Timestamp:  time.Now(),
	}

	if result.Score != 0.95 {
		t.Errorf("Score = %v, want 0.95", result.Score)
	}
	if result.Tier != "L1" {
		t.Errorf("Tier = %s, want L1", result.Tier)
	}
}

func TestRetrieveRequest(t *testing.T) {
	req := &RetrieveRequest{
		SessionID:   "session1",
		Query:       "test query",
		TokenBudget: 1000,
		TopicID:     "topic1",
	}

	if req.SessionID != "session1" {
		t.Errorf("SessionID = %s, want session1", req.SessionID)
	}
	if req.TokenBudget != 1000 {
		t.Errorf("TokenBudget = %d, want 1000", req.TokenBudget)
	}
}

func TestRetrieveResponse(t *testing.T) {
	resp := &RetrieveResponse{
		Results: []*SearchResult{
			{ID: "r1", Score: 0.9},
			{ID: "r2", Score: 0.8},
		},
		TotalTokens: 100,
		SearchStats: &SearchStats{
			L1Hits:    1,
			L2Hits:    1,
			L3Hits:    0,
			LatencyMs: 50,
		},
	}

	if len(resp.Results) != 2 {
		t.Errorf("Results count = %d, want 2", len(resp.Results))
	}
	if resp.SearchStats.L1Hits != 1 {
		t.Errorf("L1Hits = %d, want 1", resp.SearchStats.L1Hits)
	}
}

// MockL1Storage 模拟L1存储
type MockL1Storage struct {
	results []*SearchResult
}

func (m *MockL1Storage) Search(ctx context.Context, sessionID, query string) ([]*SearchResult, error) {
	return m.results, nil
}

// MockL2Storage 模拟L2存储
type MockL2Storage struct {
	vectorResults []*SearchResult
	entityResults []*SearchResult
}

func (m *MockL2Storage) VectorSearch(ctx context.Context, query string, topK int) ([]*SearchResult, error) {
	return m.vectorResults, nil
}

func (m *MockL2Storage) EntitySearch(ctx context.Context, entities []string) ([]*SearchResult, error) {
	return m.entityResults, nil
}

// MockL3Storage 模拟L3存储
type MockL3Storage struct {
	results []*SearchResult
}

func (m *MockL3Storage) ArchiveSearch(ctx context.Context, sessionID, query string) ([]*SearchResult, error) {
	return m.results, nil
}

func TestParallelPipeline_SetStorages(t *testing.T) {
	prefetcher, err := NewSessionPrefetcher(DefaultPrefetchConfig)
	if err != nil {
		t.Fatalf("NewSessionPrefetcher failed: %v", err)
	}

	pipeline := NewParallelPipeline(prefetcher, DefaultPipelineConfig)

	l1 := &MockL1Storage{
		results: []*SearchResult{
			{ID: "l1_1", Content: "L1 content", Score: 0.9, Tier: "L1"},
		},
	}
	l2 := &MockL2Storage{
		vectorResults: []*SearchResult{
			{ID: "l2_1", Content: "L2 content", Score: 0.8, Tier: "L2"},
		},
	}
	l3 := &MockL3Storage{
		results: []*SearchResult{
			{ID: "l3_1", Content: "L3 content", Score: 0.7, Tier: "L3"},
		},
	}

	pipeline.SetStorages(l1, l2, l3)

	// 应该能设置成功
	if pipeline.l1Storage == nil {
		t.Error("L1Storage should not be nil after SetStorages")
	}
	if pipeline.l2Storage == nil {
		t.Error("L2Storage should not be nil after SetStorages")
	}
	if pipeline.l3Storage == nil {
		t.Error("L3Storage should not be nil after SetStorages")
	}
}

func TestNewResultAggregator(t *testing.T) {
	aggregator := NewResultAggregator(1000)
	if aggregator == nil {
		t.Fatal("NewResultAggregator returned nil")
	}
}

func TestResultAggregator_AddResults(t *testing.T) {
	aggregator := NewResultAggregator(1000)

	results := []*SearchResult{
		{ID: "r1", Score: 0.9, TokenCount: 100, Tier: "L1"},
		{ID: "r2", Score: 0.8, TokenCount: 150, Tier: "L1"},
	}

	aggregator.AddResults("L1", results, 1.0)

	if aggregator.l1Hits != 2 {
		t.Errorf("l1Hits = %d, want 2", aggregator.l1Hits)
	}
}

func TestResultAggregator_Finalize(t *testing.T) {
	aggregator := NewResultAggregator(1000)

	results := []*SearchResult{
		{ID: "r1", Score: 0.9, TokenCount: 100},
		{ID: "r2", Score: 0.7, TokenCount: 150},
		{ID: "r3", Score: 0.8, TokenCount: 200},
	}

	aggregator.AddResults("L1", results, 1.0)

	ctx := context.Background()
	response, err := aggregator.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	if len(response.Results) != 3 {
		t.Errorf("Results count = %d, want 3", len(response.Results))
	}

	// 结果应该按分数排序
	if len(response.Results) >= 2 && response.Results[0].Score < response.Results[1].Score {
		t.Error("Results should be sorted by score descending")
	}
}

func TestResultAggregator_TokenBudget(t *testing.T) {
	// 小的token预算
	aggregator := NewResultAggregator(200)

	// 添加超出预算的结果
	results := []*SearchResult{
		{ID: "r1", Score: 0.9, TokenCount: 100},
		{ID: "r2", Score: 0.8, TokenCount: 100},
		{ID: "r3", Score: 0.7, TokenCount: 100}, // 会超出
	}

	aggregator.AddResults("L1", results, 1.0)

	ctx := context.Background()
	response, err := aggregator.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	// 总token应该不超过预算
	if response.TotalTokens > 200 {
		t.Errorf("TotalTokens = %d, should not exceed budget 200", response.TotalTokens)
	}

	// 应该只有2个结果（前两个刚好200 tokens）
	if len(response.Results) > 2 {
		t.Errorf("Results count = %d, expected <= 2 due to token budget", len(response.Results))
	}
}

func TestResultAggregator_HasEnoughResults(t *testing.T) {
	aggregator := NewResultAggregator(1000)

	// 初始应该没有足够结果
	if aggregator.HasEnoughResults() {
		t.Error("Should not have enough results initially")
	}

	// 添加足够多的结果
	results := []*SearchResult{
		{ID: "r1", Score: 0.9, TokenCount: 300},
		{ID: "r2", Score: 0.8, TokenCount: 300},
	}
	aggregator.AddResults("L1", results, 1.0)

	// 现在应该有足够结果（600 tokens >= 1000/2）
	if !aggregator.HasEnoughResults() {
		t.Error("Should have enough results after adding 600 tokens")
	}
}

func TestResultAggregator_Deduplicate(t *testing.T) {
	aggregator := NewResultAggregator(1000)

	// 添加重复结果
	results1 := []*SearchResult{
		{ID: "r1", Score: 0.9, TokenCount: 100},
		{ID: "r2", Score: 0.8, TokenCount: 100},
	}
	results2 := []*SearchResult{
		{ID: "r1", Score: 0.85, TokenCount: 100}, // 重复
		{ID: "r3", Score: 0.7, TokenCount: 100},
	}

	aggregator.AddResults("L1", results1, 1.0)
	aggregator.AddResults("L2", results2, 1.0)

	ctx := context.Background()
	response, err := aggregator.Finalize(ctx)
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	// 应该只有3个唯一结果
	if len(response.Results) != 3 {
		t.Errorf("Results count = %d, want 3 (after dedup)", len(response.Results))
	}
}
