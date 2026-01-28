// Package statistics 提供统计功能
package statistics

import (
	"encoding/json"
	"sort"
	"sync"
	"time"
)

// StatsCollector 统计收集器
type StatsCollector struct {
	token       *TokenCounter
	cache       *CacheStatsCollector
	performance *PerformanceCollector
	business    *BusinessCollector

	mu sync.RWMutex
}

// NewStatsCollector 创建统计收集器
func NewStatsCollector() *StatsCollector {
	return &StatsCollector{
		token:       NewTokenCounter(),
		cache:       NewCacheStatsCollector(),
		performance: NewPerformanceCollector(),
		business:    NewBusinessCollector(),
	}
}

// Token 获取Token计数器
func (s *StatsCollector) Token() *TokenCounter {
	return s.token
}

// Cache 获取缓存统计收集器
func (s *StatsCollector) Cache() *CacheStatsCollector {
	return s.cache
}

// Performance 获取性能统计收集器
func (s *StatsCollector) Performance() *PerformanceCollector {
	return s.performance
}

// Business 获取业务统计收集器
func (s *StatsCollector) Business() *BusinessCollector {
	return s.business
}

// CacheStatsCollector 缓存统计收集器
type CacheStatsCollector struct {
	mu sync.RWMutex

	totalHits      int64
	totalMisses    int64
	totalSets      int64
	totalDeletes   int64
	totalEvictions int64

	byLevel map[string]*LevelStats
	byType  map[string]*TypeStats
}

// LevelStats 缓存级别统计
type LevelStats struct {
	Level     string  `json:"level"`
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	HitRate   float64 `json:"hit_rate"`
	Size      int64   `json:"size"`
	MaxSize   int64   `json:"max_size"`
	ItemCount int64   `json:"item_count"`
}

// TypeStats 缓存类型统计
type TypeStats struct {
	Type    string  `json:"type"`
	Hits    int64   `json:"hits"`
	Misses  int64   `json:"misses"`
	HitRate float64 `json:"hit_rate"`
	AvgTTL  float64 `json:"avg_ttl"`
}

// NewCacheStatsCollector 创建缓存统计收集器
func NewCacheStatsCollector() *CacheStatsCollector {
	return &CacheStatsCollector{
		byLevel: make(map[string]*LevelStats),
		byType:  make(map[string]*TypeStats),
	}
}

// RecordHit 记录命中
func (c *CacheStatsCollector) RecordHit(level, cacheType string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalHits++

	if c.byLevel[level] == nil {
		c.byLevel[level] = &LevelStats{Level: level}
	}
	c.byLevel[level].Hits++

	if c.byType[cacheType] == nil {
		c.byType[cacheType] = &TypeStats{Type: cacheType}
	}
	c.byType[cacheType].Hits++
}

// RecordMiss 记录未命中
func (c *CacheStatsCollector) RecordMiss(level, cacheType string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalMisses++

	if c.byLevel[level] == nil {
		c.byLevel[level] = &LevelStats{Level: level}
	}
	c.byLevel[level].Misses++

	if c.byType[cacheType] == nil {
		c.byType[cacheType] = &TypeStats{Type: cacheType}
	}
	c.byType[cacheType].Misses++
}

// HitRate 计算总命中率
func (c *CacheStatsCollector) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.totalHits + c.totalMisses
	if total == 0 {
		return 0
	}
	return float64(c.totalHits) / float64(total)
}

// GetStats 获取统计
func (c *CacheStatsCollector) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 计算命中率
	for _, level := range c.byLevel {
		total := level.Hits + level.Misses
		if total > 0 {
			level.HitRate = float64(level.Hits) / float64(total)
		}
	}

	for _, t := range c.byType {
		total := t.Hits + t.Misses
		if total > 0 {
			t.HitRate = float64(t.Hits) / float64(total)
		}
	}

	return map[string]interface{}{
		"total_hits":   c.totalHits,
		"total_misses": c.totalMisses,
		"hit_rate":     c.HitRate(),
		"by_level":     c.byLevel,
		"by_type":      c.byType,
	}
}

// PerformanceCollector 性能统计收集器
type PerformanceCollector struct {
	mu sync.RWMutex

	responseTimes []float64
	toolCalls     map[string]*ToolCallStats
	agentStats    *AgentPerformanceStats
}

// ToolCallStats 工具调用统计
type ToolCallStats struct {
	Tool         string  `json:"tool"`
	CallCount    int64   `json:"call_count"`
	SuccessCount int64   `json:"success_count"`
	ErrorCount   int64   `json:"error_count"`
	TotalTime    float64 `json:"total_time"`
	AvgLatency   float64 `json:"avg_latency"`
}

// AgentPerformanceStats Agent性能统计
type AgentPerformanceStats struct {
	TotalIterations int64   `json:"total_iterations"`
	AvgIterations   float64 `json:"avg_iterations"`
	MaxIterations   int64   `json:"max_iterations"`
	TimeoutCount    int64   `json:"timeout_count"`
}

// NewPerformanceCollector 创建性能统计收集器
func NewPerformanceCollector() *PerformanceCollector {
	return &PerformanceCollector{
		responseTimes: make([]float64, 0),
		toolCalls:     make(map[string]*ToolCallStats),
		agentStats:    &AgentPerformanceStats{},
	}
}

// RecordResponseTime 记录响应时间
func (p *PerformanceCollector) RecordResponseTime(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.responseTimes = append(p.responseTimes, duration.Seconds())
}

// RecordToolCall 记录工具调用
func (p *PerformanceCollector) RecordToolCall(tool string, duration time.Duration, success bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.toolCalls[tool] == nil {
		p.toolCalls[tool] = &ToolCallStats{Tool: tool}
	}

	stats := p.toolCalls[tool]
	stats.CallCount++
	stats.TotalTime += duration.Seconds()
	stats.AvgLatency = stats.TotalTime / float64(stats.CallCount)

	if success {
		stats.SuccessCount++
	} else {
		stats.ErrorCount++
	}
}

// RecordAgentIteration 记录Agent迭代
func (p *PerformanceCollector) RecordAgentIteration(iterations int, timeout bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.agentStats.TotalIterations += int64(iterations)
	if int64(iterations) > p.agentStats.MaxIterations {
		p.agentStats.MaxIterations = int64(iterations)
	}
	if timeout {
		p.agentStats.TimeoutCount++
	}
}

// GetLatencyStats 获取延迟统计
func (p *PerformanceCollector) GetLatencyStats() map[string]float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.responseTimes) == 0 {
		return map[string]float64{}
	}

	// 排序计算百分位
	sorted := make([]float64, len(p.responseTimes))
	copy(sorted, p.responseTimes)
	sort.Float64s(sorted)

	var sum float64
	for _, t := range sorted {
		sum += t
	}

	return map[string]float64{
		"count": float64(len(sorted)),
		"avg":   sum / float64(len(sorted)),
		"min":   sorted[0],
		"max":   sorted[len(sorted)-1],
		"p50":   percentile(sorted, 50),
		"p90":   percentile(sorted, 90),
		"p99":   percentile(sorted, 99),
	}
}

func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * float64(p) / 100.0)
	return sorted[idx]
}

// BusinessCollector 业务统计收集器
type BusinessCollector struct {
	mu sync.RWMutex

	queryCount   int64
	successCount int64
	failureCount int64

	intentDist map[string]int64
	hourlyDist [24]int64
	dailyDist  map[string]int64

	uniqueUsers map[string]struct{}
}

// NewBusinessCollector 创建业务统计收集器
func NewBusinessCollector() *BusinessCollector {
	return &BusinessCollector{
		intentDist:  make(map[string]int64),
		dailyDist:   make(map[string]int64),
		uniqueUsers: make(map[string]struct{}),
	}
}

// RecordQuery 记录查询
func (b *BusinessCollector) RecordQuery(intent string, success bool, userID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.queryCount++
	if success {
		b.successCount++
	} else {
		b.failureCount++
	}

	// 意图分布
	b.intentDist[intent]++

	// 时间分布
	now := time.Now()
	b.hourlyDist[now.Hour()]++
	b.dailyDist[now.Format("2006-01-02")]++

	// 用户统计
	if userID != "" {
		b.uniqueUsers[userID] = struct{}{}
	}
}

// SuccessRate 计算成功率
func (b *BusinessCollector) SuccessRate() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	total := b.successCount + b.failureCount
	if total == 0 {
		return 0
	}
	return float64(b.successCount) / float64(total)
}

// GetStats 获取统计
func (b *BusinessCollector) GetStats() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return map[string]interface{}{
		"query_count":   b.queryCount,
		"success_count": b.successCount,
		"failure_count": b.failureCount,
		"success_rate":  b.SuccessRate(),
		"intent_dist":   b.intentDist,
		"hourly_dist":   b.hourlyDist,
		"unique_users":  len(b.uniqueUsers),
	}
}

// Report 综合报告
type Report struct {
	Period      string                 `json:"period"`
	Generated   time.Time              `json:"generated"`
	Token       map[string]interface{} `json:"token"`
	Cache       map[string]interface{} `json:"cache"`
	Performance map[string]interface{} `json:"performance"`
	Business    map[string]interface{} `json:"business"`
	Summary     *ReportSummary         `json:"summary"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalQueries    int64   `json:"total_queries"`
	SuccessRate     float64 `json:"success_rate"`
	AvgResponseTime float64 `json:"avg_response_time"`
	TotalTokenCost  float64 `json:"total_token_cost"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
	TokenSavings    int64   `json:"token_savings"`
	CostSavings     float64 `json:"cost_savings"`
}

// GenerateReport 生成综合报告
func (s *StatsCollector) GenerateReport() *Report {
	latencyStats := s.performance.GetLatencyStats()

	// Convert performance stats to interface map
	perfStats := make(map[string]interface{})
	for k, v := range latencyStats {
		perfStats[k] = v
	}

	return &Report{
		Period:      "current",
		Generated:   time.Now(),
		Token:       toMap(s.token.GetAllStats()),
		Cache:       s.cache.GetStats(),
		Performance: perfStats,
		Business:    s.business.GetStats(),
		Summary: &ReportSummary{
			TotalQueries:    s.business.queryCount,
			SuccessRate:     s.business.SuccessRate(),
			AvgResponseTime: latencyStats["avg"],
			TotalTokenCost:  s.token.GetTotalCost(),
			CacheHitRate:    s.cache.HitRate(),
		},
	}
}

func toMap(stats map[string]*ModelTokenStats) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range stats {
		result[k] = v
	}
	return result
}

// ExportJSON 导出为JSON
func (r *Report) ExportJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
