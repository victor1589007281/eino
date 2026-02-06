// Package router 引擎评分器
package router

import (
	"math"
	"sync"
	"time"
)

// EngineScorer 引擎评分器
type EngineScorer struct {
	qualityTracker *QualityTracker
	quotaManager   *QuotaManager
	classifier     *QueryClassifier
	
	// 权重配置
	weights ScorerWeights
	
	// 得分缓存
	cache    map[string]*ScoreCache
	cacheTTL time.Duration
	mu       sync.RWMutex
}

// ScorerWeights 评分权重
type ScorerWeights struct {
	Quality        float64 `json:"quality"`         // 质量权重 (默认 0.30)
	SuccessRate    float64 `json:"success_rate"`    // 成功率权重 (默认 0.25)
	QuotaHealth    float64 `json:"quota_health"`    // 配额健康权重 (默认 0.15)
	QueryTypeMatch float64 `json:"query_type_match"` // 查询类型匹配权重 (默认 0.15)
	Latency        float64 `json:"latency"`         // 延迟权重 (默认 0.10)
	Cost           float64 `json:"cost"`            // 成本权重 (默认 0.05，负向)
}

// DefaultScorerWeights 默认权重
var DefaultScorerWeights = ScorerWeights{
	Quality:        0.30,
	SuccessRate:    0.25,
	QuotaHealth:    0.15,
	QueryTypeMatch: 0.15,
	Latency:        0.10,
	Cost:           0.05,
}

// ScoreCache 得分缓存
type ScoreCache struct {
	Score     float64
	Details   *ScoreDetails
	Timestamp time.Time
}

// ScoreDetails 得分详情
type ScoreDetails struct {
	QualityScore    float64 `json:"quality_score"`
	SuccessScore    float64 `json:"success_score"`
	QuotaScore      float64 `json:"quota_score"`
	MatchScore      float64 `json:"match_score"`
	LatencyScore    float64 `json:"latency_score"`
	CostScore       float64 `json:"cost_score"`
	FinalScore      float64 `json:"final_score"`
	Penalties       float64 `json:"penalties"`
	QueryType       QueryType `json:"query_type"`
}

// NewEngineScorer 创建引擎评分器
func NewEngineScorer(qt *QualityTracker, qm *QuotaManager, classifier *QueryClassifier) *EngineScorer {
	return &EngineScorer{
		qualityTracker: qt,
		quotaManager:   qm,
		classifier:     classifier,
		weights:        DefaultScorerWeights,
		cache:          make(map[string]*ScoreCache),
		cacheTTL:       10 * time.Second,
	}
}

// Score 计算引擎得分
func (s *EngineScorer) Score(engineName string, query string) float64 {
	details := s.ScoreWithDetails(engineName, query)
	return details.FinalScore
}

// ScoreWithDetails 计算引擎得分（带详情）
func (s *EngineScorer) ScoreWithDetails(engineName string, query string) *ScoreDetails {
	// 检查缓存
	cacheKey := engineName + ":" + query
	s.mu.RLock()
	if cached, ok := s.cache[cacheKey]; ok {
		if time.Since(cached.Timestamp) < s.cacheTTL {
			s.mu.RUnlock()
			return cached.Details
		}
	}
	s.mu.RUnlock()
	
	// 分类查询
	classification := s.classifier.Classify(query)
	
	// 获取质量指标
	metrics := s.qualityTracker.GetMetrics(engineName)
	
	// 计算各维度得分
	details := &ScoreDetails{
		QueryType: classification.PrimaryType,
	}
	
	// 1. 质量得分
	details.QualityScore = metrics.RelevanceScore
	
	// 2. 成功率得分
	details.SuccessScore = metrics.SuccessRate
	
	// 3. 配额健康得分
	details.QuotaScore = s.quotaManager.GetQuotaHealth(engineName)
	
	// 4. 查询类型匹配得分
	details.MatchScore = GetQueryTypeMatchScore(engineName, classification.PrimaryType)
	
	// 5. 延迟得分
	details.LatencyScore = s.normalizeLatency(metrics.LatencyP95)
	
	// 6. 成本得分 (越低越好，但免费不一定最好)
	details.CostScore = s.normalizeCost(engineName)
	
	// 加权求和
	score := s.weights.Quality*details.QualityScore +
		s.weights.SuccessRate*details.SuccessScore +
		s.weights.QuotaHealth*details.QuotaScore +
		s.weights.QueryTypeMatch*details.MatchScore +
		s.weights.Latency*details.LatencyScore -
		s.weights.Cost*details.CostScore
	
	// 惩罚项：连续失败
	consecutiveFails := s.qualityTracker.GetConsecutiveFails(engineName)
	if consecutiveFails > 0 {
		penalty := 0.1 * float64(consecutiveFails)
		details.Penalties = math.Min(penalty, 0.5)
		score = score * (1 - details.Penalties)
	}
	
	// 配额耗尽惩罚
	if !s.quotaManager.IsAvailable(engineName) {
		score = 0 // 配额耗尽，得分为0
	}
	
	details.FinalScore = math.Max(0, math.Min(1, score))
	
	// 更新缓存
	s.mu.Lock()
	s.cache[cacheKey] = &ScoreCache{
		Score:     details.FinalScore,
		Details:   details,
		Timestamp: time.Now(),
	}
	s.mu.Unlock()
	
	return details
}

// normalizeLatency 归一化延迟得分 (延迟越低得分越高)
func (s *EngineScorer) normalizeLatency(latency time.Duration) float64 {
	// 假设理想延迟 100ms，最大容忍 2000ms
	ms := float64(latency.Milliseconds())
	
	if ms <= 0 {
		return 0.8 // 无数据，给默认值
	}
	
	if ms <= 100 {
		return 1.0
	} else if ms <= 500 {
		return 1.0 - (ms-100)/400*0.3 // 0.7 - 1.0
	} else if ms <= 1000 {
		return 0.7 - (ms-500)/500*0.3 // 0.4 - 0.7
	} else if ms <= 2000 {
		return 0.4 - (ms-1000)/1000*0.3 // 0.1 - 0.4
	}
	return 0.1
}

// normalizeCost 归一化成本得分 (成本越高得分越高，用于减分)
func (s *EngineScorer) normalizeCost(engineName string) float64 {
	if s.quotaManager.IsFree(engineName) {
		return 0 // 免费引擎，无成本惩罚
	}
	
	cost := s.quotaManager.GetCost(engineName)
	
	// 假设最高成本 $0.01/query
	normalized := cost / 0.01
	return math.Min(normalized, 1.0)
}

// ScoreAllEngines 对所有引擎评分
func (s *EngineScorer) ScoreAllEngines(engines []string, query string) map[string]*ScoreDetails {
	result := make(map[string]*ScoreDetails)
	for _, engine := range engines {
		result[engine] = s.ScoreWithDetails(engine, query)
	}
	return result
}

// SetWeights 设置权重
func (s *EngineScorer) SetWeights(weights ScorerWeights) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.weights = weights
	// 清除缓存
	s.cache = make(map[string]*ScoreCache)
}

// GetWeights 获取权重
func (s *EngineScorer) GetWeights() ScorerWeights {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.weights
}

// ClearCache 清除缓存
func (s *EngineScorer) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]*ScoreCache)
}

// InvalidateEngine 使特定引擎的缓存失效
func (s *EngineScorer) InvalidateEngine(engineName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 删除所有以该引擎开头的缓存
	for key := range s.cache {
		if len(key) > len(engineName) && key[:len(engineName)+1] == engineName+":" {
			delete(s.cache, key)
		}
	}
}
