// Package io 预读取与批量IO优化
package io

import (
	"context"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// SessionPrefetcher 会话预读取器
type SessionPrefetcher struct {
	// 预读取缓存
	cache *lru.Cache[string, *PrefetchResult]

	// 配置
	config PrefetchConfig

	// 统计
	stats PrefetchStats

	mu sync.RWMutex
}

// PrefetchConfig 预读取配置
type PrefetchConfig struct {
	// 会话预读取
	SessionLookAhead  int           `json:"session_look_ahead"`   // 预读取最近 N 个会话
	TopicLookAhead    int           `json:"topic_look_ahead"`     // 每个会话预读取 N 个主题
	TimeWindowMinutes int           `json:"time_window_minutes"`  // 预读取最近 N 分钟的数据

	// 触发条件
	TriggerOnAccess bool `json:"trigger_on_access"` // 访问时触发预读取
	TriggerOnIdle   bool `json:"trigger_on_idle"`   // 空闲时触发后台预读取
	IdleThresholdMs int  `json:"idle_threshold_ms"` // 空闲阈值

	// 资源限制
	MaxConcurrent int           `json:"max_concurrent"` // 最大并发预读取数
	MaxCacheSize  int           `json:"max_cache_size"` // 最大缓存大小
	CacheTTL      time.Duration `json:"cache_ttl"`      // 缓存过期时间
}

// DefaultPrefetchConfig 默认配置
var DefaultPrefetchConfig = PrefetchConfig{
	SessionLookAhead:  5,
	TopicLookAhead:    3,
	TimeWindowMinutes: 30,
	TriggerOnAccess:   true,
	TriggerOnIdle:     true,
	IdleThresholdMs:   100,
	MaxConcurrent:     10,
	MaxCacheSize:      1000,
	CacheTTL:          5 * time.Minute,
}

// PrefetchResult 预读取结果
type PrefetchResult struct {
	SessionID  string
	Topics     []*TopicData
	Entities   map[string][]*Relation
	LoadTime   time.Duration
	CachedAt   time.Time
	AccessedAt time.Time
}

// TopicData 主题数据
type TopicData struct {
	TopicID      string
	Title        string
	Summary      string
	KeyFragments []string
	TokenCount   int
}

// Relation 关系数据
type Relation struct {
	Source string
	Target string
	Type   string
	Weight float64
}

// PrefetchStats 预读取统计
type PrefetchStats struct {
	TotalPrefetches int64
	CacheHits       int64
	CacheMisses     int64
	AvgLoadTime     time.Duration
}

// NewSessionPrefetcher 创建预读取器
func NewSessionPrefetcher(config PrefetchConfig) (*SessionPrefetcher, error) {
	cache, err := lru.New[string, *PrefetchResult](config.MaxCacheSize)
	if err != nil {
		return nil, err
	}

	return &SessionPrefetcher{
		cache:  cache,
		config: config,
	}, nil
}

// PrefetchOnAccess 访问时触发预读取
func (p *SessionPrefetcher) PrefetchOnAccess(ctx context.Context, sessionID string, loader DataLoader) {
	if !p.config.TriggerOnAccess {
		return
	}

	// 检查缓存
	if p.cache.Contains(sessionID) {
		return
	}

	// 异步预读取
	go p.prefetch(ctx, sessionID, loader)
}

// prefetch 执行预读取
func (p *SessionPrefetcher) prefetch(ctx context.Context, sessionID string, loader DataLoader) {
	startTime := time.Now()

	result := &PrefetchResult{
		SessionID:  sessionID,
		Topics:     make([]*TopicData, 0),
		Entities:   make(map[string][]*Relation),
		CachedAt:   time.Now(),
		AccessedAt: time.Now(),
	}

	// 1. 预读取最近主题
	topics, err := loader.LoadRecentTopics(ctx, sessionID, p.config.TopicLookAhead)
	if err == nil {
		result.Topics = topics
	}

	// 2. 提取并预读取关联实体
	entities := extractEntities(topics)
	for _, entity := range entities {
		relations, err := loader.LoadEntityRelations(ctx, entity, 1)
		if err == nil {
			result.Entities[entity] = relations
		}
	}

	result.LoadTime = time.Since(startTime)

	// 更新缓存
	p.cache.Add(sessionID, result)

	// 更新统计
	p.mu.Lock()
	p.stats.TotalPrefetches++
	p.mu.Unlock()
}

// GetPrefetched 获取预读取的数据
func (p *SessionPrefetcher) GetPrefetched(sessionID string) (*PrefetchResult, bool) {
	result, ok := p.cache.Get(sessionID)
	if !ok {
		p.mu.Lock()
		p.stats.CacheMisses++
		p.mu.Unlock()
		return nil, false
	}

	// 检查是否过期
	if time.Since(result.CachedAt) > p.config.CacheTTL {
		p.cache.Remove(sessionID)
		p.mu.Lock()
		p.stats.CacheMisses++
		p.mu.Unlock()
		return nil, false
	}

	// 更新访问时间
	result.AccessedAt = time.Now()

	p.mu.Lock()
	p.stats.CacheHits++
	p.mu.Unlock()

	return result, true
}

// Invalidate 使缓存失效
func (p *SessionPrefetcher) Invalidate(sessionID string) {
	p.cache.Remove(sessionID)
}

// GetStats 获取统计
func (p *SessionPrefetcher) GetStats() PrefetchStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// DataLoader 数据加载器接口
type DataLoader interface {
	LoadRecentTopics(ctx context.Context, sessionID string, limit int) ([]*TopicData, error)
	LoadEntityRelations(ctx context.Context, entity string, depth int) ([]*Relation, error)
}

// extractEntities 从主题中提取实体
func extractEntities(topics []*TopicData) []string {
	entitySet := make(map[string]bool)
	for _, topic := range topics {
		// 简化实现：从摘要中提取关键词作为实体
		// 实际实现中应该使用 NER 或其他实体提取方法
		if topic.Summary != "" {
			entitySet[topic.Title] = true
		}
	}

	entities := make([]string, 0, len(entitySet))
	for entity := range entitySet {
		entities = append(entities, entity)
	}
	return entities
}

// SmartPrefetcher 智能预读取器
type SmartPrefetcher struct {
	basePrefetcher *SessionPrefetcher

	// 访问模式分析
	accessPattern *AccessPatternAnalyzer

	// 预测模型
	predictor *AccessPredictor
}

// AccessPatternAnalyzer 访问模式分析器
type AccessPatternAnalyzer struct {
	// 会话访问历史
	sessionHistory map[string]*SessionAccessHistory

	// 主题共现矩阵
	topicCooccurrence map[string]map[string]int

	mu sync.RWMutex
}

// SessionAccessHistory 会话访问历史
type SessionAccessHistory struct {
	SessionID      string
	RecentAccesses []AccessRecord
	TopTopics      []string
	AccessPattern  string        // sequential, random, topic_based
	AvgInterval    time.Duration // 平均访问间隔
}

// AccessRecord 访问记录
type AccessRecord struct {
	Timestamp time.Time
	TopicID   string
	QueryType string
}

// NewAccessPatternAnalyzer 创建访问模式分析器
func NewAccessPatternAnalyzer() *AccessPatternAnalyzer {
	return &AccessPatternAnalyzer{
		sessionHistory:    make(map[string]*SessionAccessHistory),
		topicCooccurrence: make(map[string]map[string]int),
	}
}

// RecordAccess 记录访问
func (a *AccessPatternAnalyzer) RecordAccess(sessionID, topicID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	history, ok := a.sessionHistory[sessionID]
	if !ok {
		history = &SessionAccessHistory{
			SessionID:      sessionID,
			RecentAccesses: make([]AccessRecord, 0),
		}
		a.sessionHistory[sessionID] = history
	}

	// 添加访问记录
	record := AccessRecord{
		Timestamp: time.Now(),
		TopicID:   topicID,
	}
	history.RecentAccesses = append(history.RecentAccesses, record)

	// 保持最近 100 条
	if len(history.RecentAccesses) > 100 {
		history.RecentAccesses = history.RecentAccesses[len(history.RecentAccesses)-100:]
	}

	// 更新共现矩阵
	if len(history.RecentAccesses) >= 2 {
		prevTopic := history.RecentAccesses[len(history.RecentAccesses)-2].TopicID
		if prevTopic != topicID {
			if a.topicCooccurrence[prevTopic] == nil {
				a.topicCooccurrence[prevTopic] = make(map[string]int)
			}
			a.topicCooccurrence[prevTopic][topicID]++
		}
	}
}

// GetCooccurredTopics 获取共现主题
func (a *AccessPatternAnalyzer) GetCooccurredTopics(topicID string) map[string]float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]float64)
	if cooccur, ok := a.topicCooccurrence[topicID]; ok {
		total := 0
		for _, count := range cooccur {
			total += count
		}
		for topic, count := range cooccur {
			result[topic] = float64(count) / float64(total)
		}
	}
	return result
}

// AccessPredictor 访问预测器
type AccessPredictor struct {
	analyzer *AccessPatternAnalyzer
}

// NewAccessPredictor 创建访问预测器
func NewAccessPredictor(analyzer *AccessPatternAnalyzer) *AccessPredictor {
	return &AccessPredictor{
		analyzer: analyzer,
	}
}

// PrefetchHint 预读取提示
type PrefetchHint struct {
	Type       string  // "topic", "entity"
	Target     string
	Confidence float64
}

// PredictNextAccess 预测下次访问
func (p *AccessPredictor) PredictNextAccess(sessionID, currentTopic string) []PrefetchHint {
	hints := make([]PrefetchHint, 0)

	// 基于共现预测
	cooccurred := p.analyzer.GetCooccurredTopics(currentTopic)
	for topic, score := range cooccurred {
		if score > 0.3 {
			hints = append(hints, PrefetchHint{
				Type:       "topic",
				Target:     topic,
				Confidence: score,
			})
		}
	}

	return hints
}
