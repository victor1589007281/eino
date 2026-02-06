// Package observatory 类型定义
package observatory

import "time"

// EventType 事件类型
type EventType string

const (
	EventTypeRetrieve    EventType = "retrieve"
	EventTypeStore       EventType = "store"
	EventTypeSwitchTopic EventType = "switch_topic"
	EventTypeRecallTopic EventType = "recall_topic"
)

// Event 观测事件
type Event struct {
	// 标识
	EventID   string    `json:"event_id"`
	RequestID string    `json:"request_id"`
	SessionID string    `json:"session_id"`
	Timestamp time.Time `json:"timestamp"`

	// 请求信息
	EventType EventType `json:"event_type"`
	Query     string    `json:"query,omitempty"`

	// 响应信息
	RetrievedChunks []ChunkInfo `json:"retrieved_chunks,omitempty"`
	TotalTokens     int         `json:"total_tokens"`

	// 性能信息
	Latency LatencyBreakdown `json:"latency"`

	// 错误信息
	Error string `json:"error,omitempty"`

	// 上下文
	TopicID  string            `json:"topic_id,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ChunkInfo 召回内容信息
type ChunkInfo struct {
	ChunkID         string  `json:"chunk_id"`
	Tier            string  `json:"tier"`
	SimilarityScore float64 `json:"similarity_score"`
	TokenCount      int     `json:"token_count"`
	ContentHash     string  `json:"content_hash,omitempty"`
	Position        int     `json:"position"`
}

// LatencyBreakdown 延迟分解
type LatencyBreakdown struct {
	Total        time.Duration `json:"total_ms"`
	L1Lookup     time.Duration `json:"l1_lookup_ms"`
	L2Search     time.Duration `json:"l2_search_ms"`
	L3Fetch      time.Duration `json:"l3_fetch_ms"`
	VectorSearch time.Duration `json:"vector_search_ms"`
	Rerank       time.Duration `json:"rerank_ms"`
	Assembly     time.Duration `json:"assembly_ms"`
}

// Metrics 实时指标
type Metrics struct {
	// 效率指标
	Latency LatencyMetrics `json:"latency"`

	// 吞吐量指标
	Throughput ThroughputMetrics `json:"throughput"`

	// 缓存指标
	Cache CacheMetrics `json:"cache"`

	// 相关性指标
	Relevance RelevanceMetrics `json:"relevance"`

	// 多样性指标
	Diversity DiversityMetrics `json:"diversity"`

	// 使用率指标
	Utilization UtilizationMetrics `json:"utilization"`

	// 质量指标
	Quality QualityMetrics `json:"quality"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`
}

// LatencyMetrics 延迟指标
type LatencyMetrics struct {
	P50       time.Duration     `json:"p50_ms"`
	P95       time.Duration     `json:"p95_ms"`
	P99       time.Duration     `json:"p99_ms"`
	Avg       time.Duration     `json:"avg_ms"`
	Breakdown LatencyBreakdownAvg `json:"breakdown"`
}

// LatencyBreakdownAvg 平均延迟分解
type LatencyBreakdownAvg struct {
	L1Avg     time.Duration `json:"l1_avg_ms"`
	L2Avg     time.Duration `json:"l2_avg_ms"`
	L3Avg     time.Duration `json:"l3_avg_ms"`
	RerankAvg time.Duration `json:"rerank_avg_ms"`
}

// ThroughputMetrics 吞吐量指标
type ThroughputMetrics struct {
	QPS      float64 `json:"qps"`
	PeakQPS  float64 `json:"peak_qps"`
	AvgQPS1h float64 `json:"avg_qps_1h"`
}

// CacheMetrics 缓存指标
type CacheMetrics struct {
	L1HitRate      float64 `json:"l1_hit_rate"`
	OverallHitRate float64 `json:"overall_hit_rate"`
}

// RelevanceMetrics 相关性指标
type RelevanceMetrics struct {
	AvgSimilarityScore float64 `json:"avg_similarity_score"`
	TopKPrecision      float64 `json:"topk_precision"`
	MRR                float64 `json:"mrr"`
	NDCG               float64 `json:"ndcg"`
}

// DiversityMetrics 多样性指标
type DiversityMetrics struct {
	UniqueSourcesRatio float64 `json:"unique_sources_ratio"`
	TopicCoverage      float64 `json:"topic_coverage"`
}

// UtilizationMetrics 使用率指标
type UtilizationMetrics struct {
	ContentUsageRate float64 `json:"content_usage_rate"`
	TokenEfficiency  float64 `json:"token_efficiency"`
	TruncationRate   float64 `json:"truncation_rate"`
}

// QualityMetrics 质量指标
type QualityMetrics struct {
	SuccessRate     float64 `json:"success_rate"`
	EmptyResultRate float64 `json:"empty_result_rate"`
	ErrorRate       float64 `json:"error_rate"`
}

// Report 报告
type Report struct {
	GeneratedAt time.Time `json:"generated_at"`
	Period      string    `json:"period"`

	// 概览
	Summary ReportSummary `json:"summary"`

	// 详细指标
	Metrics Metrics `json:"metrics"`

	// 趋势数据
	Trends ReportTrends `json:"trends"`

	// 问题列表
	TopIssues []AggregatedIssue `json:"top_issues"`

	// 优化建议
	Recommendations []Recommendation `json:"recommendations"`
}

// ReportSummary 报告概览
type ReportSummary struct {
	TotalRequests int64   `json:"total_requests"`
	SuccessRate   float64 `json:"success_rate"`
	AvgLatency    string  `json:"avg_latency"`
	AvgRelevance  float64 `json:"avg_relevance"`
	OverallHealth string  `json:"overall_health"`
}

// ReportTrends 趋势数据
type ReportTrends struct {
	LatencyTrend    []TrendPoint `json:"latency_trend"`
	QualityTrend    []TrendPoint `json:"quality_trend"`
	ThroughputTrend []TrendPoint `json:"throughput_trend"`
}

// TrendPoint 趋势点
type TrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// AggregatedIssue 聚合问题
type AggregatedIssue struct {
	Type        IssueType `json:"type"`
	Count       int       `json:"count"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
}

// IssueType 问题类型
type IssueType string

const (
	IssueTypeLowRelevance   IssueType = "low_relevance"
	IssueTypeHighLatency    IssueType = "high_latency"
	IssueTypeLowUtilization IssueType = "low_utilization"
	IssueTypeEmptyResult    IssueType = "empty_result"
	IssueTypeTokenWaste     IssueType = "token_waste"
	IssueTypeCacheMiss      IssueType = "cache_miss"
)

// Recommendation 优化建议
type Recommendation struct {
	ID               string                 `json:"id"`
	Priority         string                 `json:"priority"`
	Category         string                 `json:"category"`
	Title            string                 `json:"title"`
	Description      string                 `json:"description"`
	Action           string                 `json:"action"`
	Impact           string                 `json:"impact"`
	SuggestedChanges map[string]interface{} `json:"suggested_changes,omitempty"`
}

// EvaluationResult 评估结果
type EvaluationResult struct {
	RequestID      string       `json:"request_id"`
	RelevanceScore float64      `json:"relevance_score"`
	Scores         ScoreDetails `json:"scores"`
	Issues         []Issue      `json:"issues,omitempty"`
}

// ScoreDetails 评分详情
type ScoreDetails struct {
	SemanticMatch   float64 `json:"semantic_match"`
	KeywordCoverage float64 `json:"keyword_coverage"`
	TopicAlignment  float64 `json:"topic_alignment"`
	FreshnessScore  float64 `json:"freshness_score"`
	DiversityScore  float64 `json:"diversity_score"`
}

// Issue 问题
type Issue struct {
	Type     IssueType `json:"type"`
	Severity string    `json:"severity"`
	Message  string    `json:"message"`
	Details  string    `json:"details,omitempty"`
}
