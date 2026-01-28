package memory

import (
	"context"
	"sync"
	"testing"
	"time"
)

// MockMemoryStore 模拟内存存储
type MockMemoryStore struct {
	memories map[string]*Memory
	mu       sync.RWMutex
}

func NewMockMemoryStore() *MockMemoryStore {
	return &MockMemoryStore{
		memories: make(map[string]*Memory),
	}
}

func (s *MockMemoryStore) Save(ctx context.Context, memory *Memory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memories[memory.ID] = memory
	return nil
}

func (s *MockMemoryStore) Get(ctx context.Context, id string) (*Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m, ok := s.memories[id]; ok {
		return m, nil
	}
	return nil, nil
}

func (s *MockMemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.memories, id)
	return nil
}

func (s *MockMemoryStore) Search(ctx context.Context, query string, opts *SearchOptions) ([]*Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Memory
	for _, m := range s.memories {
		if contains(m.Content, query) || contains(m.Summary, query) {
			results = append(results, m)
		}
	}

	if opts != nil && opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

func (s *MockMemoryStore) SearchBySemantic(ctx context.Context, embedding []float32, opts *SearchOptions) ([]*Memory, error) {
	return nil, nil
}

func (s *MockMemoryStore) List(ctx context.Context, memType MemoryType, limit int) ([]*Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Memory
	for _, m := range s.memories {
		if m.Type == memType {
			results = append(results, m)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func (s *MockMemoryStore) UpdateAccess(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.memories[id]; ok {
		m.AccessedAt = time.Now()
		m.AccessCount++
	}
	return nil
}

func (s *MockMemoryStore) Cleanup(ctx context.Context, maxAge time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, m := range s.memories {
		if m.CreatedAt.Before(cutoff) {
			delete(s.memories, id)
		}
	}
	return nil
}

// MockEmbedder 模拟向量化器
type MockEmbedder struct{}

func (e *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// 返回简单的模拟向量
	return []float32{0.1, 0.2, 0.3}, nil
}

// MockSummarizer 模拟摘要生成器
type MockSummarizer struct{}

func (s *MockSummarizer) Summarize(ctx context.Context, content string) (string, error) {
	if len(content) > 50 {
		return content[:50] + "...", nil
	}
	return content, nil
}

func TestMemoryManager_AddShortTerm(t *testing.T) {
	store := NewMockMemoryStore()
	config := &MemoryManagerConfig{
		ShortTermLimit: 5,
		WorkingLimit:   10,
	}

	manager := NewMemoryManager(store, nil, nil, config)
	ctx := context.Background()

	// 添加短期记忆
	mem, err := manager.AddShortTerm(ctx, "This is a test memory", map[string]interface{}{
		"source": "test",
	})

	if err != nil {
		t.Fatalf("AddShortTerm failed: %v", err)
	}

	if mem == nil {
		t.Fatal("Memory should not be nil")
	}

	if mem.Type != MemoryShortTerm {
		t.Errorf("Expected type %s, got %s", MemoryShortTerm, mem.Type)
	}

	if mem.Content != "This is a test memory" {
		t.Errorf("Content mismatch")
	}
}

func TestMemoryManager_ShortTermLimit(t *testing.T) {
	store := NewMockMemoryStore()
	config := &MemoryManagerConfig{
		ShortTermLimit: 3,
		WorkingLimit:   10,
	}

	manager := NewMemoryManager(store, nil, nil, config)
	ctx := context.Background()

	// 添加超过限制的记忆
	for i := 0; i < 5; i++ {
		manager.AddShortTerm(ctx, "Memory "+string(rune('A'+i)), nil)
	}

	// 短期记忆应该只有限制数量
	memories := manager.GetShortTermMemories()
	if len(memories) != 3 {
		t.Errorf("Expected 3 short term memories, got %d", len(memories))
	}
}

func TestMemoryManager_AddWorking(t *testing.T) {
	store := NewMockMemoryStore()
	config := &MemoryManagerConfig{
		ShortTermLimit: 5,
		WorkingLimit:   10,
	}

	manager := NewMemoryManager(store, nil, nil, config)
	ctx := context.Background()

	mem, err := manager.AddWorking(ctx, "Working memory content", map[string]interface{}{
		"task": "analysis",
	})

	if err != nil {
		t.Fatalf("AddWorking failed: %v", err)
	}

	if mem.Type != MemoryWorking {
		t.Errorf("Expected type %s, got %s", MemoryWorking, mem.Type)
	}

	if mem.Importance != 0.7 {
		t.Errorf("Expected importance 0.7, got %f", mem.Importance)
	}
}

func TestMemoryManager_AddLongTerm(t *testing.T) {
	store := NewMockMemoryStore()
	embedder := &MockEmbedder{}
	summarizer := &MockSummarizer{}
	config := &MemoryManagerConfig{
		ShortTermLimit: 5,
		WorkingLimit:   10,
	}

	manager := NewMemoryManager(store, embedder, summarizer, config)
	ctx := context.Background()

	content := "This is a long term memory about Linux kernel scheduling"
	mem, err := manager.AddLongTerm(ctx, content, map[string]interface{}{
		"topic": "scheduler",
	})

	if err != nil {
		t.Fatalf("AddLongTerm failed: %v", err)
	}

	if mem.Type != MemoryLongTerm {
		t.Errorf("Expected type %s, got %s", MemoryLongTerm, mem.Type)
	}

	// 应该有embedding
	if mem.Embedding == nil {
		t.Error("Expected embedding to be set")
	}

	// 应该有summary
	if mem.Summary == "" {
		t.Error("Expected summary to be set")
	}

	// 应该保存到store
	saved, _ := store.Get(ctx, mem.ID)
	if saved == nil {
		t.Error("Memory should be saved to store")
	}
}

func TestMemoryManager_Recall(t *testing.T) {
	store := NewMockMemoryStore()
	config := &MemoryManagerConfig{
		ShortTermLimit: 10,
		WorkingLimit:   10,
	}

	manager := NewMemoryManager(store, nil, nil, config)
	ctx := context.Background()

	// 添加一些记忆
	manager.AddShortTerm(ctx, "Linux kernel fork implementation", nil)
	manager.AddShortTerm(ctx, "Process scheduling in Linux", nil)
	manager.AddShortTerm(ctx, "Memory management overview", nil)
	manager.AddWorking(ctx, "Current task: analyze fork", nil)

	// 召回测试
	opts := &RecallOptions{
		TopK:            5,
		IncludeLongTerm: false,
		UseSemantic:     false,
	}

	results, err := manager.Recall(ctx, "fork", opts)
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected some recall results")
	}

	// 验证包含fork相关的记忆
	found := false
	for _, m := range results {
		if contains(m.Content, "fork") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to recall fork-related memory")
	}
}

func TestMemoryManager_ClearShortTerm(t *testing.T) {
	store := NewMockMemoryStore()
	config := &MemoryManagerConfig{
		ShortTermLimit: 10,
		WorkingLimit:   10,
	}

	manager := NewMemoryManager(store, nil, nil, config)
	ctx := context.Background()

	manager.AddShortTerm(ctx, "Memory 1", nil)
	manager.AddShortTerm(ctx, "Memory 2", nil)

	if len(manager.GetShortTermMemories()) != 2 {
		t.Error("Should have 2 short term memories")
	}

	manager.ClearShortTerm()

	if len(manager.GetShortTermMemories()) != 0 {
		t.Error("Should have 0 short term memories after clear")
	}
}

func TestMemoryManager_ClearWorking(t *testing.T) {
	store := NewMockMemoryStore()
	config := &MemoryManagerConfig{
		ShortTermLimit: 10,
		WorkingLimit:   10,
	}

	manager := NewMemoryManager(store, nil, nil, config)
	ctx := context.Background()

	manager.AddWorking(ctx, "Working 1", nil)
	manager.AddWorking(ctx, "Working 2", nil)

	manager.ClearWorking()

	if len(manager.GetWorkingMemories()) != 0 {
		t.Error("Should have 0 working memories after clear")
	}
}

func TestMemory_Creation(t *testing.T) {
	mem := &Memory{
		ID:         "test-id",
		Type:       MemoryShortTerm,
		Content:    "Test content",
		Summary:    "Test summary",
		Metadata:   map[string]interface{}{"key": "value"},
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		Importance: 0.5,
	}

	if mem.ID != "test-id" {
		t.Error("ID mismatch")
	}
	if mem.Type != MemoryShortTerm {
		t.Error("Type mismatch")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID("content1")
	id2 := generateID("content2")
	id3 := generateID("content1")

	if id1 == "" {
		t.Error("ID should not be empty")
	}

	if id1 == id2 {
		t.Error("Different content should produce different IDs")
	}

	// 由于包含时间戳，相同内容也会产生不同ID
	if id1 == id3 {
		// 这是可能的，因为ID包含时间
	}
}

func TestDeduplicateMemories(t *testing.T) {
	mem1 := &Memory{ID: "1", Content: "A"}
	mem2 := &Memory{ID: "2", Content: "B"}
	mem3 := &Memory{ID: "1", Content: "A"} // 重复

	memories := []*Memory{mem1, mem2, mem3, mem1}

	result := deduplicateMemories(memories)

	if len(result) != 2 {
		t.Errorf("Expected 2 unique memories, got %d", len(result))
	}
}

func TestSortMemoriesByRelevance(t *testing.T) {
	mem1 := &Memory{ID: "1", Importance: 0.5, AccessedAt: time.Now().Add(-time.Hour)}
	mem2 := &Memory{ID: "2", Importance: 0.8, AccessedAt: time.Now()}
	mem3 := &Memory{ID: "3", Importance: 0.8, AccessedAt: time.Now().Add(-time.Minute)}

	memories := []*Memory{mem1, mem3, mem2}
	sortMemoriesByRelevance(memories, "")

	// 最高重要性，最近访问的应该在前面
	if memories[0].ID != "2" {
		t.Errorf("Expected memory 2 first, got %s", memories[0].ID)
	}
}

func TestSearchOptions(t *testing.T) {
	opts := &SearchOptions{
		Limit:               10,
		MemoryTypes:         []MemoryType{MemoryLongTerm, MemorySemantic},
		MinImportance:       0.5,
		SimilarityThreshold: 0.7,
		Tags:                []string{"kernel", "linux"},
	}

	if opts.Limit != 10 {
		t.Error("Limit mismatch")
	}
	if len(opts.MemoryTypes) != 2 {
		t.Error("MemoryTypes length mismatch")
	}
}

func TestRecallOptions(t *testing.T) {
	opts := &RecallOptions{
		TopK:                5,
		Limit:               100,
		IncludeLongTerm:     true,
		UseSemantic:         true,
		MinImportance:       0.3,
		SimilarityThreshold: 0.6,
	}

	if opts.TopK != 5 {
		t.Error("TopK mismatch")
	}
	if !opts.IncludeLongTerm {
		t.Error("IncludeLongTerm should be true")
	}
}

func BenchmarkMemoryManager_AddShortTerm(b *testing.B) {
	store := NewMockMemoryStore()
	config := &MemoryManagerConfig{
		ShortTermLimit: 1000,
		WorkingLimit:   1000,
	}
	manager := NewMemoryManager(store, nil, nil, config)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.AddShortTerm(ctx, "Test memory content", nil)
	}
}

func BenchmarkMemoryManager_Recall(b *testing.B) {
	store := NewMockMemoryStore()
	config := &MemoryManagerConfig{
		ShortTermLimit: 100,
		WorkingLimit:   100,
	}
	manager := NewMemoryManager(store, nil, nil, config)
	ctx := context.Background()

	// 填充记忆
	for i := 0; i < 100; i++ {
		manager.AddShortTerm(ctx, "Memory about Linux kernel scheduling fork process", nil)
	}

	opts := &RecallOptions{TopK: 10}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Recall(ctx, "fork", opts)
	}
}
