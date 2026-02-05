// Package algorithm 算法层指标
package algorithm

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics 算法层指标收集器
type Metrics struct {
	// 意图识别指标
	IntentRequests   atomic.Int64
	IntentByEngine   sync.Map // map[string]*atomic.Int64
	IntentLatencies  *LatencyHistogram
	IntentConfidence *FloatHistogram
	IntentFallbacks  atomic.Int64

	// 向量化指标
	EmbeddingRequests  atomic.Int64
	EmbeddingLatencies *LatencyHistogram
	EmbeddingCacheHits atomic.Int64
	EmbeddingCacheMiss atomic.Int64

	// 索引指标
	IndexRequests  atomic.Int64
	SearchRequests atomic.Int64
	SearchLatencies *LatencyHistogram

	// NLP指标
	NLPRequests  atomic.Int64
	NLPLatencies *LatencyHistogram
}

// NewMetrics 创建指标收集器
func NewMetrics() *Metrics {
	return &Metrics{
		IntentLatencies:    NewLatencyHistogram(),
		IntentConfidence:   NewFloatHistogram(),
		EmbeddingLatencies: NewLatencyHistogram(),
		SearchLatencies:    NewLatencyHistogram(),
		NLPLatencies:       NewLatencyHistogram(),
	}
}

// RecordIntentRecognition 记录意图识别
func (m *Metrics) RecordIntentRecognition(engine string, latency time.Duration, confidence float64) {
	m.IntentRequests.Add(1)
	m.IntentLatencies.Record(latency)
	m.IntentConfidence.Record(confidence)

	// 按引擎统计
	counter, _ := m.IntentByEngine.LoadOrStore(engine, &atomic.Int64{})
	counter.(*atomic.Int64).Add(1)
}

// RecordIntentFallback 记录意图识别降级
func (m *Metrics) RecordIntentFallback() {
	m.IntentFallbacks.Add(1)
}

// RecordEmbedding 记录向量化
func (m *Metrics) RecordEmbedding(latency time.Duration, cacheHit bool) {
	m.EmbeddingRequests.Add(1)
	m.EmbeddingLatencies.Record(latency)
	if cacheHit {
		m.EmbeddingCacheHits.Add(1)
	} else {
		m.EmbeddingCacheMiss.Add(1)
	}
}

// RecordSearch 记录搜索
func (m *Metrics) RecordSearch(latency time.Duration) {
	m.SearchRequests.Add(1)
	m.SearchLatencies.Record(latency)
}

// RecordNLP 记录NLP处理
func (m *Metrics) RecordNLP(latency time.Duration) {
	m.NLPRequests.Add(1)
	m.NLPLatencies.Record(latency)
}

// Snapshot 获取指标快照
func (m *Metrics) Snapshot() *MetricsSnapshot {
	snap := &MetricsSnapshot{
		IntentRequests:     m.IntentRequests.Load(),
		IntentFallbacks:    m.IntentFallbacks.Load(),
		IntentAvgLatency:   m.IntentLatencies.Avg(),
		IntentP99Latency:   m.IntentLatencies.P99(),
		IntentByEngine:     make(map[string]int64),
		EmbeddingRequests:  m.EmbeddingRequests.Load(),
		EmbeddingCacheHits: m.EmbeddingCacheHits.Load(),
		EmbeddingCacheMiss: m.EmbeddingCacheMiss.Load(),
		EmbeddingAvgLatency: m.EmbeddingLatencies.Avg(),
		SearchRequests:     m.SearchRequests.Load(),
		SearchAvgLatency:   m.SearchLatencies.Avg(),
		NLPRequests:        m.NLPRequests.Load(),
		NLPAvgLatency:      m.NLPLatencies.Avg(),
	}

	m.IntentByEngine.Range(func(key, value interface{}) bool {
		snap.IntentByEngine[key.(string)] = value.(*atomic.Int64).Load()
		return true
	})

	return snap
}

// MetricsSnapshot 指标快照
type MetricsSnapshot struct {
	IntentRequests      int64            `json:"intent_requests"`
	IntentFallbacks     int64            `json:"intent_fallbacks"`
	IntentAvgLatency    time.Duration    `json:"intent_avg_latency"`
	IntentP99Latency    time.Duration    `json:"intent_p99_latency"`
	IntentByEngine      map[string]int64 `json:"intent_by_engine"`
	EmbeddingRequests   int64            `json:"embedding_requests"`
	EmbeddingCacheHits  int64            `json:"embedding_cache_hits"`
	EmbeddingCacheMiss  int64            `json:"embedding_cache_miss"`
	EmbeddingAvgLatency time.Duration    `json:"embedding_avg_latency"`
	SearchRequests      int64            `json:"search_requests"`
	SearchAvgLatency    time.Duration    `json:"search_avg_latency"`
	NLPRequests         int64            `json:"nlp_requests"`
	NLPAvgLatency       time.Duration    `json:"nlp_avg_latency"`
}

// LatencyHistogram 延迟直方图
type LatencyHistogram struct {
	mu      sync.RWMutex
	samples []time.Duration
	maxSize int
}

// NewLatencyHistogram 创建延迟直方图
func NewLatencyHistogram() *LatencyHistogram {
	return &LatencyHistogram{
		samples: make([]time.Duration, 0, 1000),
		maxSize: 1000,
	}
}

// Record 记录样本
func (h *LatencyHistogram) Record(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.samples) >= h.maxSize {
		// 简单丢弃旧数据
		h.samples = h.samples[1:]
	}
	h.samples = append(h.samples, d)
}

// Avg 平均值
func (h *LatencyHistogram) Avg() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.samples) == 0 {
		return 0
	}
	var total time.Duration
	for _, s := range h.samples {
		total += s
	}
	return total / time.Duration(len(h.samples))
}

// P99 P99延迟
func (h *LatencyHistogram) P99() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.samples) == 0 {
		return 0
	}
	// 简单实现：排序后取99%位置
	sorted := make([]time.Duration, len(h.samples))
	copy(sorted, h.samples)
	// 简单冒泡排序（生产环境应使用更高效算法）
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	idx := int(float64(len(sorted)) * 0.99)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// FloatHistogram 浮点数直方图
type FloatHistogram struct {
	mu      sync.RWMutex
	samples []float64
	maxSize int
}

// NewFloatHistogram 创建浮点数直方图
func NewFloatHistogram() *FloatHistogram {
	return &FloatHistogram{
		samples: make([]float64, 0, 1000),
		maxSize: 1000,
	}
}

// Record 记录样本
func (h *FloatHistogram) Record(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.samples) >= h.maxSize {
		h.samples = h.samples[1:]
	}
	h.samples = append(h.samples, v)
}

// Avg 平均值
func (h *FloatHistogram) Avg() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.samples) == 0 {
		return 0
	}
	var total float64
	for _, s := range h.samples {
		total += s
	}
	return total / float64(len(h.samples))
}
