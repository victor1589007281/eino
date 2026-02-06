// Package io 并行检索流水线
package io

import (
	"context"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// ParallelPipeline 并行检索流水线
type ParallelPipeline struct {
	prefetcher *SessionPrefetcher
	batchReader *BatchReader

	// 各层存储
	l1Storage L1Storage
	l2Storage L2Storage
	l3Storage L3Storage

	// 配置
	config PipelineConfig
}

// PipelineConfig 流水线配置
type PipelineConfig struct {
	ParallelRetrieve bool          `json:"parallel_retrieve"`
	AsyncProcessing  bool          `json:"async_processing"`
	TokenBudget      int           `json:"token_budget"`
	Timeout          time.Duration `json:"timeout"`
}

// DefaultPipelineConfig 默认配置
var DefaultPipelineConfig = PipelineConfig{
	ParallelRetrieve: true,
	AsyncProcessing:  true,
	TokenBudget:      4000,
	Timeout:          5 * time.Second,
}

// L1Storage L1 存储接口
type L1Storage interface {
	Search(ctx context.Context, sessionID, query string) ([]*SearchResult, error)
}

// L2Storage L2 存储接口
type L2Storage interface {
	VectorSearch(ctx context.Context, query string, topK int) ([]*SearchResult, error)
	EntitySearch(ctx context.Context, entities []string) ([]*SearchResult, error)
}

// L3Storage L3 存储接口
type L3Storage interface {
	ArchiveSearch(ctx context.Context, sessionID, query string) ([]*SearchResult, error)
}

// SearchResult 搜索结果
type SearchResult struct {
	ID         string
	Content    string
	Score      float64
	Tier       string
	TokenCount int
	Timestamp  time.Time
}

// RetrieveRequest 检索请求
type RetrieveRequest struct {
	SessionID   string
	Query       string
	TokenBudget int
	TopicID     string
}

// RetrieveResponse 检索响应
type RetrieveResponse struct {
	Results     []*SearchResult
	TotalTokens int
	SearchStats *SearchStats
}

// SearchStats 搜索统计
type SearchStats struct {
	L1Hits   int
	L2Hits   int
	L3Hits   int
	LatencyMs int64
}

// NewParallelPipeline 创建并行流水线
func NewParallelPipeline(prefetcher *SessionPrefetcher, config PipelineConfig) *ParallelPipeline {
	return &ParallelPipeline{
		prefetcher:  prefetcher,
		batchReader: NewBatchReader(100, config.Timeout),
		config:      config,
	}
}

// SetStorages 设置存储
func (p *ParallelPipeline) SetStorages(l1 L1Storage, l2 L2Storage, l3 L3Storage) {
	p.l1Storage = l1
	p.l2Storage = l2
	p.l3Storage = l3
}

// Retrieve 执行检索
func (p *ParallelPipeline) Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error) {
	startTime := time.Now()

	tokenBudget := req.TokenBudget
	if tokenBudget <= 0 {
		tokenBudget = p.config.TokenBudget
	}

	// 创建结果聚合器
	aggregator := NewResultAggregator(tokenBudget)

	// 触发预读取
	if p.prefetcher != nil {
		p.prefetcher.PrefetchOnAccess(ctx, req.SessionID, nil)
	}

	if p.config.ParallelRetrieve {
		// 并行检索
		p.parallelSearch(ctx, req, aggregator)
	} else {
		// 顺序检索
		p.sequentialSearch(ctx, req, aggregator)
	}

	// 检查预读取缓存
	if p.prefetcher != nil {
		if prefetched, ok := p.prefetcher.GetPrefetched(req.SessionID); ok {
			aggregator.AddPrefetchedResults(prefetched)
		}
	}

	// 最终处理
	response, err := aggregator.Finalize(ctx)
	if err != nil {
		return nil, err
	}

	response.SearchStats = &SearchStats{
		L1Hits:    aggregator.l1Hits,
		L2Hits:    aggregator.l2Hits,
		L3Hits:    aggregator.l3Hits,
		LatencyMs: time.Since(startTime).Milliseconds(),
	}

	return response, nil
}

// parallelSearch 并行检索
func (p *ParallelPipeline) parallelSearch(ctx context.Context, req *RetrieveRequest, aggregator *ResultAggregator) {
	g, ctx := errgroup.WithContext(ctx)

	// L1 检索
	if p.l1Storage != nil {
		g.Go(func() error {
			results, err := p.l1Storage.Search(ctx, req.SessionID, req.Query)
			if err != nil {
				return nil // L1 失败不阻塞
			}
			aggregator.AddResults("L1", results, 1.0)
			return nil
		})
	}

	// L2 向量检索
	if p.l2Storage != nil {
		g.Go(func() error {
			results, err := p.l2Storage.VectorSearch(ctx, req.Query, 10)
			if err != nil {
				return nil
			}
			aggregator.AddResults("L2", results, 0.8)
			return nil
		})

		// L2 实体检索
		g.Go(func() error {
			entities := extractQueryEntities(req.Query)
			if len(entities) == 0 {
				return nil
			}
			results, err := p.l2Storage.EntitySearch(ctx, entities)
			if err != nil {
				return nil
			}
			aggregator.AddResults("L2-Entity", results, 0.6)
			return nil
		})
	}

	// L3 归档检索 (仅在其他层结果不足时)
	if p.l3Storage != nil {
		g.Go(func() error {
			results, err := p.l3Storage.ArchiveSearch(ctx, req.SessionID, req.Query)
			if err != nil {
				return nil
			}
			aggregator.AddResults("L3", results, 0.5)
			return nil
		})
	}

	g.Wait()
}

// sequentialSearch 顺序检索
func (p *ParallelPipeline) sequentialSearch(ctx context.Context, req *RetrieveRequest, aggregator *ResultAggregator) {
	// L1
	if p.l1Storage != nil {
		results, err := p.l1Storage.Search(ctx, req.SessionID, req.Query)
		if err == nil {
			aggregator.AddResults("L1", results, 1.0)
		}
	}

	// 如果 L1 结果足够，跳过后续层
	if aggregator.HasEnoughResults() {
		return
	}

	// L2
	if p.l2Storage != nil {
		results, err := p.l2Storage.VectorSearch(ctx, req.Query, 10)
		if err == nil {
			aggregator.AddResults("L2", results, 0.8)
		}
	}

	// L3
	if p.l3Storage != nil && !aggregator.HasEnoughResults() {
		results, err := p.l3Storage.ArchiveSearch(ctx, req.SessionID, req.Query)
		if err == nil {
			aggregator.AddResults("L3", results, 0.5)
		}
	}
}

// ResultAggregator 结果聚合器
type ResultAggregator struct {
	mu          sync.Mutex
	results     []*ScoredResult
	tokenBudget int
	tokenUsed   int
	l1Hits      int
	l2Hits      int
	l3Hits      int
}

// ScoredResult 带得分的结果
type ScoredResult struct {
	Source string
	Result *SearchResult
	Score  float64
	Tokens int
}

// NewResultAggregator 创建结果聚合器
func NewResultAggregator(tokenBudget int) *ResultAggregator {
	return &ResultAggregator{
		results:     make([]*ScoredResult, 0),
		tokenBudget: tokenBudget,
	}
}

// AddResults 添加检索结果
func (a *ResultAggregator) AddResults(source string, results []*SearchResult, weight float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, r := range results {
		a.results = append(a.results, &ScoredResult{
			Source: source,
			Result: r,
			Score:  r.Score * weight,
			Tokens: r.TokenCount,
		})

		// 统计命中
		switch source {
		case "L1":
			a.l1Hits++
		case "L2", "L2-Entity":
			a.l2Hits++
		case "L3":
			a.l3Hits++
		}
	}
}

// AddPrefetchedResults 添加预读取的结果
func (a *ResultAggregator) AddPrefetchedResults(prefetched *PrefetchResult) {
	// 简化实现：不添加重复内容
}

// HasEnoughResults 检查是否有足够结果
func (a *ResultAggregator) HasEnoughResults() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	var totalTokens int
	for _, r := range a.results {
		totalTokens += r.Tokens
	}

	return totalTokens >= a.tokenBudget/2
}

// Finalize 最终处理
func (a *ResultAggregator) Finalize(ctx context.Context) (*RetrieveResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 去重
	a.deduplicate()

	// 按得分排序
	sort.Slice(a.results, func(i, j int) bool {
		return a.results[i].Score > a.results[j].Score
	})

	// Token 预算裁剪
	finalResults := make([]*SearchResult, 0)
	for _, r := range a.results {
		if a.tokenUsed+r.Tokens > a.tokenBudget {
			break
		}
		finalResults = append(finalResults, r.Result)
		a.tokenUsed += r.Tokens
	}

	return &RetrieveResponse{
		Results:     finalResults,
		TotalTokens: a.tokenUsed,
	}, nil
}

// deduplicate 去重
func (a *ResultAggregator) deduplicate() {
	seen := make(map[string]bool)
	unique := make([]*ScoredResult, 0)

	for _, r := range a.results {
		if !seen[r.Result.ID] {
			seen[r.Result.ID] = true
			unique = append(unique, r)
		}
	}

	a.results = unique
}

// extractQueryEntities 从查询中提取实体
func extractQueryEntities(query string) []string {
	// 简化实现
	return nil
}

// MultiLevelCache 多级缓存
type MultiLevelCache struct {
	// L1: 本地内存缓存
	localCache *sync.Map

	// L2: 远程缓存 (如 Redis)
	remoteCache RemoteCache

	// 配置
	localTTL  time.Duration
	remoteTTL time.Duration
}

// RemoteCache 远程缓存接口
type RemoteCache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
}

// NewMultiLevelCache 创建多级缓存
func NewMultiLevelCache(remoteCache RemoteCache, localTTL, remoteTTL time.Duration) *MultiLevelCache {
	return &MultiLevelCache{
		localCache:  &sync.Map{},
		remoteCache: remoteCache,
		localTTL:    localTTL,
		remoteTTL:   remoteTTL,
	}
}

// Get 获取缓存
func (c *MultiLevelCache) Get(ctx context.Context, key string) (interface{}, bool) {
	// 检查本地缓存
	if entry, ok := c.localCache.Load(key); ok {
		e := entry.(*CacheEntry)
		if time.Now().Before(e.ExpiresAt) {
			return e.Value, true
		}
		c.localCache.Delete(key)
	}

	// 检查远程缓存
	if c.remoteCache != nil {
		data, err := c.remoteCache.Get(ctx, key)
		if err == nil && data != nil {
			// 回填本地缓存
			c.localCache.Store(key, &CacheEntry{
				Value:     data,
				ExpiresAt: time.Now().Add(c.localTTL),
			})
			return data, true
		}
	}

	return nil, false
}

// Set 设置缓存
func (c *MultiLevelCache) Set(ctx context.Context, key string, value interface{}) error {
	// 本地缓存
	c.localCache.Store(key, &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(c.localTTL),
	})

	// 远程缓存 (异步)
	if c.remoteCache != nil {
		go func() {
			if data, ok := value.([]byte); ok {
				c.remoteCache.Set(ctx, key, data, c.remoteTTL)
			}
		}()
	}

	return nil
}
