// Package observatory Memory 旁观者 Agent
package observatory

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Observer 旁观者 Agent
type Observer struct {
	// 事件收集器
	collector *Collector

	// 指标分析器
	analyzer *Analyzer

	// 质量评估器
	evaluator *Evaluator

	// 报告生成器
	reporter *Reporter

	// 配置
	config ObserverConfig

	// 控制
	enabled bool
	mu      sync.RWMutex
}

// ObserverConfig 观察者配置
type ObserverConfig struct {
	Enabled      bool    `json:"enabled"`
	SampleRate   float64 `json:"sample_rate"`   // 采样率 (0-1)
	BufferSize   int     `json:"buffer_size"`   // 缓冲区大小
	WindowSizes  []string `json:"window_sizes"` // 统计窗口大小
	RetentionHrs int     `json:"retention_hours"` // 保留时间
}

// DefaultObserverConfig 默认配置
var DefaultObserverConfig = ObserverConfig{
	Enabled:      true,
	SampleRate:   1.0,
	BufferSize:   10000,
	WindowSizes:  []string{"1m", "5m", "1h", "24h"},
	RetentionHrs: 168, // 7 days
}

// NewObserver 创建观察者
func NewObserver(config ObserverConfig) *Observer {
	o := &Observer{
		config:  config,
		enabled: config.Enabled,
	}

	o.collector = NewCollector(config.BufferSize, config.SampleRate)
	o.analyzer = NewAnalyzer(config.WindowSizes)
	o.evaluator = NewEvaluator()
	o.reporter = NewReporter(o.analyzer, o.evaluator)

	return o
}

// Start 启动观察者
func (o *Observer) Start() {
	o.mu.Lock()
	o.enabled = true
	o.mu.Unlock()

	// 启动分析器后台任务
	go o.analyzer.StartBackgroundTasks()
}

// Stop 停止观察者
func (o *Observer) Stop() {
	o.mu.Lock()
	o.enabled = false
	o.mu.Unlock()

	o.analyzer.Stop()
}

// IsEnabled 检查是否启用
func (o *Observer) IsEnabled() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.enabled
}

// RecordEvent 记录事件
func (o *Observer) RecordEvent(event *Event) {
	if !o.IsEnabled() {
		return
	}

	// 异步处理
	go func() {
		o.collector.Collect(event)
		o.analyzer.ProcessEvent(event)
	}()
}

// GetRealtimeMetrics 获取实时指标
func (o *Observer) GetRealtimeMetrics() *Metrics {
	return o.analyzer.GetRealtimeMetrics()
}

// GetReport 获取报告
func (o *Observer) GetReport(period string) (*Report, error) {
	return o.reporter.GenerateReport(period)
}

// GetRecommendations 获取优化建议
func (o *Observer) GetRecommendations() []Recommendation {
	return o.reporter.GetRecommendations()
}

// TapMiddleware 旁路采集中间件
type TapMiddleware struct {
	observer *Observer
}

// NewTapMiddleware 创建中间件
func NewTapMiddleware(observer *Observer) *TapMiddleware {
	return &TapMiddleware{
		observer: observer,
	}
}

// WrapRetrieve 包装检索函数
func (m *TapMiddleware) WrapRetrieve(handler RetrieveHandler) RetrieveHandler {
	return func(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error) {
		if !m.observer.IsEnabled() {
			return handler(ctx, req)
		}

		// 生成事件 ID
		eventID := uuid.New().String()
		startTime := time.Now()

		// 创建延迟追踪器
		tracker := NewLatencyTracker()
		ctx = WithLatencyTracker(ctx, tracker)

		// 执行实际处理
		resp, err := handler(ctx, req)
		_ = time.Since(startTime) // 用于记录总延迟

		// 构建事件
		event := &Event{
			EventID:   eventID,
			RequestID: GetRequestID(ctx),
			SessionID: req.SessionID,
			Timestamp: startTime,
			EventType: EventTypeRetrieve,
			Query:     req.Query,
			Latency:   tracker.GetBreakdown(),
		}

		if resp != nil {
			event.RetrievedChunks = extractChunkInfo(resp)
			event.TotalTokens = resp.TotalTokens
		}

		if err != nil {
			event.Error = err.Error()
		}

		// 记录事件
		m.observer.RecordEvent(event)

		return resp, err
	}
}

// RetrieveHandler 检索处理函数类型
type RetrieveHandler func(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error)

// RetrieveRequest 检索请求（简化版）
type RetrieveRequest struct {
	SessionID   string
	Query       string
	TokenBudget int
}

// RetrieveResponse 检索响应（简化版）
type RetrieveResponse struct {
	Context     []ContextItem
	TotalTokens int
}

// ContextItem 上下文项
type ContextItem struct {
	ChunkID   string
	Content   string
	Score     float64
	Tier      string
	TokenCount int
}

// extractChunkInfo 提取 chunk 信息
func extractChunkInfo(resp *RetrieveResponse) []ChunkInfo {
	var chunks []ChunkInfo
	for i, item := range resp.Context {
		chunks = append(chunks, ChunkInfo{
			ChunkID:        item.ChunkID,
			Tier:           item.Tier,
			SimilarityScore: item.Score,
			TokenCount:     item.TokenCount,
			Position:       i,
		})
	}
	return chunks
}

// LatencyTracker 延迟追踪器
type LatencyTracker struct {
	breakdown LatencyBreakdown
	mu        sync.Mutex
}

// NewLatencyTracker 创建延迟追踪器
func NewLatencyTracker() *LatencyTracker {
	return &LatencyTracker{}
}

// RecordL1 记录 L1 延迟
func (t *LatencyTracker) RecordL1(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.breakdown.L1Lookup = d
}

// RecordL2 记录 L2 延迟
func (t *LatencyTracker) RecordL2(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.breakdown.L2Search = d
}

// RecordL3 记录 L3 延迟
func (t *LatencyTracker) RecordL3(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.breakdown.L3Fetch = d
}

// RecordRerank 记录重排序延迟
func (t *LatencyTracker) RecordRerank(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.breakdown.Rerank = d
}

// GetBreakdown 获取延迟分解
func (t *LatencyTracker) GetBreakdown() LatencyBreakdown {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.breakdown
}

// Context key 类型
type ctxKey string

const latencyTrackerKey ctxKey = "latency_tracker"
const requestIDKey ctxKey = "request_id"

// WithLatencyTracker 将追踪器放入 context
func WithLatencyTracker(ctx context.Context, tracker *LatencyTracker) context.Context {
	return context.WithValue(ctx, latencyTrackerKey, tracker)
}

// GetLatencyTracker 从 context 获取追踪器
func GetLatencyTracker(ctx context.Context) *LatencyTracker {
	if tracker, ok := ctx.Value(latencyTrackerKey).(*LatencyTracker); ok {
		return tracker
	}
	return nil
}

// GetRequestID 获取请求 ID
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// MarshalJSON 实现 JSON 序列化
func (o *Observer) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"enabled": o.enabled,
		"config":  o.config,
		"metrics": o.GetRealtimeMetrics(),
	})
}
