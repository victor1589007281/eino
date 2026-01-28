// Package memory 提供记忆系统
package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

// MemoryType 记忆类型
type MemoryType string

const (
	MemoryShortTerm MemoryType = "short_term" // 短期记忆
	MemoryWorking   MemoryType = "working"    // 工作记忆
	MemoryLongTerm  MemoryType = "long_term"  // 长期记忆
	MemorySemantic  MemoryType = "semantic"   // 语义记忆
)

// Memory 记忆条目
type Memory struct {
	ID          string                 `json:"id"`
	Type        MemoryType             `json:"type"`
	Content     string                 `json:"content"`
	Summary     string                 `json:"summary"`
	Embedding   []float32              `json:"embedding,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	AccessedAt  time.Time              `json:"accessed_at"`
	AccessCount int                    `json:"access_count"`
	Importance  float64                `json:"importance"`
	Tags        []string               `json:"tags,omitempty"`
}

// MemoryStore 记忆存储接口
type MemoryStore interface {
	Save(ctx context.Context, memory *Memory) error
	Get(ctx context.Context, id string) (*Memory, error)
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, query string, opts *SearchOptions) ([]*Memory, error)
	SearchBySemantic(ctx context.Context, embedding []float32, opts *SearchOptions) ([]*Memory, error)
	List(ctx context.Context, memType MemoryType, limit int) ([]*Memory, error)
	UpdateAccess(ctx context.Context, id string) error
	Cleanup(ctx context.Context, maxAge time.Duration) error
}

// SearchOptions 搜索选项
type SearchOptions struct {
	Limit               int
	MemoryTypes         []MemoryType
	MinImportance       float64
	SimilarityThreshold float64
	Tags                []string
}

// MemoryManager 记忆管理器
type MemoryManager struct {
	store      MemoryStore
	embedder   Embedder
	summarizer Summarizer

	shortTermLimit int
	workingLimit   int

	mu             sync.RWMutex
	shortTermCache []*Memory
	workingCache   []*Memory
}

// Embedder 向量化接口
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Summarizer 摘要生成接口
type Summarizer interface {
	Summarize(ctx context.Context, content string) (string, error)
}

// MemoryManagerConfig 记忆管理器配置
type MemoryManagerConfig struct {
	ShortTermLimit int
	WorkingLimit   int
}

// NewMemoryManager 创建记忆管理器
func NewMemoryManager(store MemoryStore, embedder Embedder, summarizer Summarizer, config *MemoryManagerConfig) *MemoryManager {
	return &MemoryManager{
		store:          store,
		embedder:       embedder,
		summarizer:     summarizer,
		shortTermLimit: config.ShortTermLimit,
		workingLimit:   config.WorkingLimit,
		shortTermCache: make([]*Memory, 0),
		workingCache:   make([]*Memory, 0),
	}
}

// AddShortTerm 添加短期记忆
func (m *MemoryManager) AddShortTerm(ctx context.Context, content string, metadata map[string]interface{}) (*Memory, error) {
	memory := &Memory{
		ID:         generateID(content),
		Type:       MemoryShortTerm,
		Content:    content,
		Metadata:   metadata,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		Importance: 0.5,
	}

	// 生成摘要
	if m.summarizer != nil {
		summary, err := m.summarizer.Summarize(ctx, content)
		if err == nil {
			memory.Summary = summary
		}
	}

	// 保存到缓存
	m.mu.Lock()
	m.shortTermCache = append(m.shortTermCache, memory)

	// 检查限制
	if len(m.shortTermCache) > m.shortTermLimit {
		// 移除最旧的并转为长期记忆
		oldest := m.shortTermCache[0]
		m.shortTermCache = m.shortTermCache[1:]
		m.mu.Unlock()

		// 异步转为长期记忆
		go m.promoteToLongTerm(context.Background(), oldest)
	} else {
		m.mu.Unlock()
	}

	return memory, nil
}

// AddWorking 添加工作记忆
func (m *MemoryManager) AddWorking(ctx context.Context, content string, metadata map[string]interface{}) (*Memory, error) {
	memory := &Memory{
		ID:         generateID(content),
		Type:       MemoryWorking,
		Content:    content,
		Metadata:   metadata,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		Importance: 0.7,
	}

	m.mu.Lock()
	m.workingCache = append(m.workingCache, memory)

	// 检查限制
	if len(m.workingCache) > m.workingLimit {
		oldest := m.workingCache[0]
		m.workingCache = m.workingCache[1:]
		m.mu.Unlock()

		go m.promoteToLongTerm(context.Background(), oldest)
	} else {
		m.mu.Unlock()
	}

	return memory, nil
}

// AddLongTerm 添加长期记忆
func (m *MemoryManager) AddLongTerm(ctx context.Context, content string, metadata map[string]interface{}) (*Memory, error) {
	memory := &Memory{
		ID:         generateID(content),
		Type:       MemoryLongTerm,
		Content:    content,
		Metadata:   metadata,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		Importance: 0.8,
	}

	// 生成摘要
	if m.summarizer != nil {
		summary, err := m.summarizer.Summarize(ctx, content)
		if err == nil {
			memory.Summary = summary
		}
	}

	// 生成向量
	if m.embedder != nil {
		embedding, err := m.embedder.Embed(ctx, content)
		if err == nil {
			memory.Embedding = embedding
		}
	}

	// 保存到存储
	if err := m.store.Save(ctx, memory); err != nil {
		return nil, fmt.Errorf("save memory: %w", err)
	}

	return memory, nil
}

// Recall 召回记忆
func (m *MemoryManager) Recall(ctx context.Context, query string, opts *RecallOptions) ([]*Memory, error) {
	results := make([]*Memory, 0)

	// 1. 从短期记忆召回
	m.mu.RLock()
	for _, mem := range m.shortTermCache {
		if matchQuery(mem, query) {
			results = append(results, mem)
		}
	}

	// 2. 从工作记忆召回
	for _, mem := range m.workingCache {
		if matchQuery(mem, query) {
			results = append(results, mem)
		}
	}
	m.mu.RUnlock()

	// 3. 关键词搜索长期记忆
	if opts.IncludeLongTerm {
		searchOpts := &SearchOptions{
			Limit:         opts.Limit,
			MemoryTypes:   []MemoryType{MemoryLongTerm, MemorySemantic},
			MinImportance: opts.MinImportance,
		}

		longTermResults, err := m.store.Search(ctx, query, searchOpts)
		if err == nil {
			results = append(results, longTermResults...)
		}
	}

	// 4. 语义搜索
	if opts.UseSemantic && m.embedder != nil {
		embedding, err := m.embedder.Embed(ctx, query)
		if err == nil {
			semanticOpts := &SearchOptions{
				Limit:               opts.Limit,
				SimilarityThreshold: opts.SimilarityThreshold,
			}
			semanticResults, err := m.store.SearchBySemantic(ctx, embedding, semanticOpts)
			if err == nil {
				results = append(results, semanticResults...)
			}
		}
	}

	// 5. 去重和排序
	results = deduplicateMemories(results)
	sortMemoriesByRelevance(results, query)

	// 6. 限制结果数量
	if opts.TopK > 0 && len(results) > opts.TopK {
		results = results[:opts.TopK]
	}

	// 7. 更新访问统计
	for _, mem := range results {
		if mem.Type == MemoryLongTerm || mem.Type == MemorySemantic {
			go m.store.UpdateAccess(context.Background(), mem.ID)
		}
	}

	return results, nil
}

// RecallOptions 召回选项
type RecallOptions struct {
	TopK                int
	Limit               int
	IncludeLongTerm     bool
	UseSemantic         bool
	MinImportance       float64
	SimilarityThreshold float64
}

// ClearShortTerm 清空短期记忆
func (m *MemoryManager) ClearShortTerm() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shortTermCache = make([]*Memory, 0)
}

// ClearWorking 清空工作记忆
func (m *MemoryManager) ClearWorking() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workingCache = make([]*Memory, 0)
}

// GetShortTermMemories 获取短期记忆
func (m *MemoryManager) GetShortTermMemories() []*Memory {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Memory, len(m.shortTermCache))
	copy(result, m.shortTermCache)
	return result
}

// GetWorkingMemories 获取工作记忆
func (m *MemoryManager) GetWorkingMemories() []*Memory {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Memory, len(m.workingCache))
	copy(result, m.workingCache)
	return result
}

// promoteToLongTerm 将记忆提升为长期记忆
func (m *MemoryManager) promoteToLongTerm(ctx context.Context, memory *Memory) {
	memory.Type = MemoryLongTerm
	memory.Importance = 0.6 // 自动提升的重要性较低

	// 生成向量
	if m.embedder != nil {
		embedding, err := m.embedder.Embed(ctx, memory.Content)
		if err == nil {
			memory.Embedding = embedding
		}
	}

	// 保存
	_ = m.store.Save(ctx, memory)
}

// generateID 生成记忆ID
func generateID(content string) string {
	hash := sha256.Sum256([]byte(content + time.Now().String()))
	return hex.EncodeToString(hash[:8])
}

// matchQuery 简单的查询匹配
func matchQuery(mem *Memory, query string) bool {
	// 简单的包含匹配
	return contains(mem.Content, query) || contains(mem.Summary, query)
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) > 0 &&
		(len(s) >= len(substr) && findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// deduplicateMemories 去重
func deduplicateMemories(memories []*Memory) []*Memory {
	seen := make(map[string]bool)
	result := make([]*Memory, 0)

	for _, mem := range memories {
		if !seen[mem.ID] {
			seen[mem.ID] = true
			result = append(result, mem)
		}
	}

	return result
}

// sortMemoriesByRelevance 按相关性排序
func sortMemoriesByRelevance(memories []*Memory, query string) {
	sort.Slice(memories, func(i, j int) bool {
		// 按重要性和访问时间排序
		if memories[i].Importance != memories[j].Importance {
			return memories[i].Importance > memories[j].Importance
		}
		return memories[i].AccessedAt.After(memories[j].AccessedAt)
	})
}
