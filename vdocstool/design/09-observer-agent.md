# Memory 旁观者 Agent 设计文档

## 1. 概述

### 1.1 背景
Memory 系统的检索质量直接影响 AI Agent 的响应质量。当前缺乏有效的手段来：
- 评估检索结果与用户查询的匹配度
- 分析召回内容是否被 LLM 实际使用
- 识别检索性能瓶颈
- 为参数调优提供数据支撑

### 1.2 目标
设计一个**非侵入式**的旁观者 Agent，通过旁路采集和分析，为 Memory 系统的优化提供量化指标和改进建议。

## 2. 系统架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Memory Observatory System                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Client Request                                                             │
│        │                                                                     │
│        ▼                                                                     │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                          Tap Middleware                                │  │
│  │                                                                        │  │
│  │   ┌─────────────┐              ┌─────────────┐                        │  │
│  │   │ Request Tap │──── copy ───▶│ Collector   │                        │  │
│  │   └──────┬──────┘              └──────┬──────┘                        │  │
│  │          │                            │                                │  │
│  │          ▼                            ▼                                │  │
│  │   ┌─────────────┐              ┌─────────────┐                        │  │
│  │   │Memory Core  │              │Event Buffer │                        │  │
│  │   │  (原逻辑)   │              │ (环形缓冲)  │                        │  │
│  │   └──────┬──────┘              └──────┬──────┘                        │  │
│  │          │                            │                                │  │
│  │          ▼                            │                                │  │
│  │   ┌─────────────┐                     │                                │  │
│  │   │Response Tap │──── copy ───────────┤                                │  │
│  │   └──────┬──────┘                     │                                │  │
│  │          │                            │                                │  │
│  └──────────┼────────────────────────────┼────────────────────────────────┘  │
│             │                            │                                   │
│             ▼                            ▼                                   │
│        Response                 ┌─────────────────────────────────────┐     │
│                                 │        Observer Agent                │     │
│                                 │                                      │     │
│                                 │  ┌───────────┐  ┌───────────┐       │     │
│                                 │  │ Analyzer  │  │ Evaluator │       │     │
│                                 │  │ (指标计算) │  │ (质量评估) │       │     │
│                                 │  └─────┬─────┘  └─────┬─────┘       │     │
│                                 │        └──────┬───────┘             │     │
│                                 │               ▼                      │     │
│                                 │        ┌───────────┐                │     │
│                                 │        │ Reporter  │                │     │
│                                 │        │ (报告生成) │                │     │
│                                 │        └─────┬─────┘                │     │
│                                 │               │                      │     │
│                                 └───────────────┼──────────────────────┘     │
│                                                 ▼                            │
│                                 ┌─────────────────────────────────────┐     │
│                                 │        Dashboard / API               │     │
│                                 │    (实时仪表盘 / 报告接口)            │     │
│                                 └─────────────────────────────────────┘     │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                       Feedback Loop (可选)                             │  │
│  │                                                                        │  │
│  │   LLM Response ──▶ Usage Analyzer ──▶ 标注召回内容使用情况              │  │
│  │                                                                        │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

## 3. 核心组件设计

### 3.1 事件采集器 (Collector)

```go
// Collector 事件采集器
type Collector struct {
    buffer     *RingBuffer
    bufferSize int
    
    // 采样控制
    sampleRate float64  // 采样率 (0-1)，1表示全量采集
    
    // 统计
    totalEvents    int64
    sampledEvents  int64
}

// Event 观测事件
type Event struct {
    // 标识
    EventID     string    `json:"event_id"`
    RequestID   string    `json:"request_id"`
    SessionID   string    `json:"session_id"`
    Timestamp   time.Time `json:"timestamp"`
    
    // 请求信息
    EventType   EventType `json:"event_type"`  // retrieve, store, switch_topic...
    Query       string    `json:"query,omitempty"`
    QueryVector []float32 `json:"query_vector,omitempty"`
    
    // 响应信息
    RetrievedChunks []ChunkInfo `json:"retrieved_chunks,omitempty"`
    TotalTokens     int         `json:"total_tokens"`
    
    // 性能信息
    Latency        LatencyBreakdown `json:"latency"`
    
    // 上下文
    TopicID     string            `json:"topic_id,omitempty"`
    Metadata    map[string]string `json:"metadata,omitempty"`
}

// ChunkInfo 召回内容信息
type ChunkInfo struct {
    ChunkID        string  `json:"chunk_id"`
    Tier           string  `json:"tier"`           // L1, L2, L3
    SimilarityScore float64 `json:"similarity_score"`
    TokenCount     int     `json:"token_count"`
    ContentHash    string  `json:"content_hash"`   // 用于去重和追踪
    Position       int     `json:"position"`       // 在结果中的位置
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

// EventType 事件类型
type EventType string

const (
    EventTypeRetrieve    EventType = "retrieve"
    EventTypeStore       EventType = "store"
    EventTypeSwitchTopic EventType = "switch_topic"
    EventTypeRecallTopic EventType = "recall_topic"
)
```

### 3.2 指标分析器 (Analyzer)

```go
// Analyzer 指标分析器
type Analyzer struct {
    // 滑动窗口统计
    windows map[string]*MetricsWindow  // 按时间窗口
    
    // 聚合统计
    globalStats *GlobalStats
    
    // 分析器
    latencyAnalyzer    *LatencyAnalyzer
    relevanceAnalyzer  *RelevanceAnalyzer
    efficiencyAnalyzer *EfficiencyAnalyzer
}

// MetricsWindow 指标窗口
type MetricsWindow struct {
    WindowStart time.Time
    WindowSize  time.Duration
    Events      []*Event
    
    // 预计算的统计值
    Stats *WindowStats
}

// WindowStats 窗口统计
type WindowStats struct {
    // 请求统计
    TotalRequests   int64
    SuccessRequests int64
    FailedRequests  int64
    
    // 延迟统计
    LatencyP50  time.Duration
    LatencyP95  time.Duration
    LatencyP99  time.Duration
    LatencyAvg  time.Duration
    
    // 各层命中统计
    L1Hits      int64
    L2Hits      int64
    L3Hits      int64
    CacheHitRate float64
    
    // 质量统计
    AvgSimilarityScore float64
    AvgRetrievedCount  float64
    AvgTokenUsage      float64
    
    // 效率统计
    AvgUtilizationRate float64  // 召回内容使用率
    WasteRate          float64  // 浪费率 (未使用的召回内容)
}

// Metrics 实时指标
type Metrics struct {
    // === 效率指标 ===
    Latency struct {
        P50    time.Duration `json:"p50_ms"`
        P95    time.Duration `json:"p95_ms"`
        P99    time.Duration `json:"p99_ms"`
        Breakdown struct {
            L1Avg     time.Duration `json:"l1_avg_ms"`
            L2Avg     time.Duration `json:"l2_avg_ms"`
            L3Avg     time.Duration `json:"l3_avg_ms"`
            RerankAvg time.Duration `json:"rerank_avg_ms"`
        } `json:"breakdown"`
    } `json:"latency"`
    
    Throughput struct {
        QPS         float64 `json:"qps"`
        PeakQPS     float64 `json:"peak_qps"`
        AvgQPS1h    float64 `json:"avg_qps_1h"`
    } `json:"throughput"`
    
    CacheMetrics struct {
        L1HitRate   float64 `json:"l1_hit_rate"`
        OverallHitRate float64 `json:"overall_hit_rate"`
    } `json:"cache"`
    
    // === 匹配度指标 ===
    Relevance struct {
        AvgSimilarityScore float64 `json:"avg_similarity_score"`
        TopKPrecision      float64 `json:"topk_precision"`      // Top-K 中相关内容占比
        MRR                float64 `json:"mrr"`                 // Mean Reciprocal Rank
        NDCG               float64 `json:"ndcg"`                // Normalized DCG
    } `json:"relevance"`
    
    Diversity struct {
        UniqueSourcesRatio float64 `json:"unique_sources_ratio"` // 来源多样性
        TopicCoverage      float64 `json:"topic_coverage"`       // 主题覆盖度
    } `json:"diversity"`
    
    // === 使用率指标 ===
    Utilization struct {
        ContentUsageRate  float64 `json:"content_usage_rate"`   // 召回内容被引用比例
        TokenEfficiency   float64 `json:"token_efficiency"`     // 有效token/总召回token
        TruncationRate    float64 `json:"truncation_rate"`      // 因超长被截断的比例
    } `json:"utilization"`
    
    // === 质量指标 ===
    Quality struct {
        SuccessRate       float64 `json:"success_rate"`         // 成功率
        EmptyResultRate   float64 `json:"empty_result_rate"`    // 空结果率
        ErrorRate         float64 `json:"error_rate"`           // 错误率
    } `json:"quality"`
}
```

### 3.3 质量评估器 (Evaluator)

```go
// Evaluator 质量评估器
type Evaluator struct {
    // LLM 评估器 (可选，用于深度评估)
    llmClient LLMClient
    
    // 规则评估器
    ruleEvaluator *RuleEvaluator
}

// EvaluationResult 评估结果
type EvaluationResult struct {
    RequestID string `json:"request_id"`
    
    // 相关性评分
    RelevanceScore float64 `json:"relevance_score"`  // 0-1
    
    // 各项子评分
    Scores struct {
        SemanticMatch    float64 `json:"semantic_match"`     // 语义匹配度
        KeywordCoverage  float64 `json:"keyword_coverage"`   // 关键词覆盖
        TopicAlignment   float64 `json:"topic_alignment"`    // 主题对齐
        FreshnessScore   float64 `json:"freshness_score"`    // 时效性
        DiversityScore   float64 `json:"diversity_score"`    // 多样性
    } `json:"scores"`
    
    // 问题检测
    Issues []Issue `json:"issues,omitempty"`
}

// Issue 检测到的问题
type Issue struct {
    Type     IssueType `json:"type"`
    Severity string    `json:"severity"`  // low, medium, high
    Message  string    `json:"message"`
    Details  string    `json:"details,omitempty"`
}

type IssueType string

const (
    IssueTypeLowRelevance    IssueType = "low_relevance"
    IssueTypeHighLatency     IssueType = "high_latency"
    IssueTypeLowUtilization  IssueType = "low_utilization"
    IssueTypeEmptyResult     IssueType = "empty_result"
    IssueTypeTokenWaste      IssueType = "token_waste"
    IssueTypeCacheMiss       IssueType = "cache_miss"
)

// Evaluate 评估单次检索
func (e *Evaluator) Evaluate(event *Event) *EvaluationResult {
    result := &EvaluationResult{
        RequestID: event.RequestID,
    }
    
    // 1. 计算语义匹配度
    result.Scores.SemanticMatch = e.calculateSemanticMatch(event)
    
    // 2. 计算关键词覆盖
    result.Scores.KeywordCoverage = e.calculateKeywordCoverage(event)
    
    // 3. 计算主题对齐度
    result.Scores.TopicAlignment = e.calculateTopicAlignment(event)
    
    // 4. 计算时效性评分
    result.Scores.FreshnessScore = e.calculateFreshness(event)
    
    // 5. 计算多样性评分
    result.Scores.DiversityScore = e.calculateDiversity(event)
    
    // 6. 综合评分
    result.RelevanceScore = e.weightedScore(result.Scores)
    
    // 7. 检测问题
    result.Issues = e.detectIssues(event, result)
    
    return result
}
```

### 3.4 LLM 响应反馈分析器

```go
// FeedbackAnalyzer LLM响应反馈分析器
type FeedbackAnalyzer struct {
    // 用于分析的小模型
    analyzerLLM LLMClient
}

// FeedbackRequest 反馈请求
type FeedbackRequest struct {
    RequestID       string   `json:"request_id"`
    RetrievedChunks []string `json:"retrieved_chunks"`  // 召回的内容
    LLMResponse     string   `json:"llm_response"`      // LLM 最终响应
}

// FeedbackResult 反馈分析结果
type FeedbackResult struct {
    RequestID string `json:"request_id"`
    
    // 使用分析
    UsedChunks    []string `json:"used_chunks"`     // 被引用的 chunk IDs
    UnusedChunks  []string `json:"unused_chunks"`   // 未被使用的 chunk IDs
    
    // 使用率
    UsageRate     float64  `json:"usage_rate"`      // 使用率
    TokenWaste    int      `json:"token_waste"`     // 浪费的 token 数
    
    // 质量判断
    AnswerQuality string   `json:"answer_quality"`  // good/partial/poor/no_answer
    MissedInfo    []string `json:"missed_info"`     // Memory 中有但 LLM 未使用的关键信息
}

// Analyze 分析 LLM 响应中对召回内容的使用情况
func (a *FeedbackAnalyzer) Analyze(req *FeedbackRequest) (*FeedbackResult, error) {
    // 使用小模型分析响应中引用了哪些召回内容
    prompt := fmt.Sprintf(`分析以下LLM响应中使用了哪些召回内容。

召回内容:
%s

LLM响应:
%s

请分析:
1. 哪些召回内容被明确引用或参考了
2. 哪些召回内容完全没有被使用
3. 是否有遗漏的重要信息

输出JSON格式: {"used": [...], "unused": [...], "missed": [...]}`,
        formatChunks(req.RetrievedChunks),
        req.LLMResponse)
    
    // 调用分析模型
    // ...
}
```

### 3.5 报告生成器 (Reporter)

```go
// Reporter 报告生成器
type Reporter struct {
    analyzer  *Analyzer
    evaluator *Evaluator
}

// Report 报告
type Report struct {
    GeneratedAt time.Time `json:"generated_at"`
    Period      string    `json:"period"`  // "1h", "24h", "7d"
    
    // 概览
    Summary struct {
        TotalRequests    int64   `json:"total_requests"`
        SuccessRate      float64 `json:"success_rate"`
        AvgLatency       string  `json:"avg_latency"`
        AvgRelevance     float64 `json:"avg_relevance"`
        OverallHealth    string  `json:"overall_health"`  // healthy/degraded/critical
    } `json:"summary"`
    
    // 详细指标
    Metrics Metrics `json:"metrics"`
    
    // 趋势数据
    Trends struct {
        LatencyTrend    []TrendPoint `json:"latency_trend"`
        QualityTrend    []TrendPoint `json:"quality_trend"`
        ThroughputTrend []TrendPoint `json:"throughput_trend"`
    } `json:"trends"`
    
    // 问题列表
    TopIssues []AggregatedIssue `json:"top_issues"`
    
    // 优化建议
    Recommendations []Recommendation `json:"recommendations"`
}

// Recommendation 优化建议
type Recommendation struct {
    ID          string `json:"id"`
    Priority    string `json:"priority"`  // high/medium/low
    Category    string `json:"category"`  // performance/quality/efficiency
    Title       string `json:"title"`
    Description string `json:"description"`
    Action      string `json:"action"`
    Impact      string `json:"impact"`
    
    // 具体参数建议
    SuggestedChanges map[string]interface{} `json:"suggested_changes,omitempty"`
}

// GenerateReport 生成报告
func (r *Reporter) GenerateReport(period string) (*Report, error) {
    report := &Report{
        GeneratedAt: time.Now(),
        Period:      period,
    }
    
    // 获取指标数据
    metrics := r.analyzer.GetMetrics(period)
    report.Metrics = *metrics
    
    // 生成概览
    report.Summary = r.generateSummary(metrics)
    
    // 生成趋势数据
    report.Trends = r.generateTrends(period)
    
    // 聚合问题
    report.TopIssues = r.aggregateIssues(period)
    
    // 生成优化建议
    report.Recommendations = r.generateRecommendations(metrics, report.TopIssues)
    
    return report, nil
}

// generateRecommendations 生成优化建议
func (r *Reporter) generateRecommendations(metrics *Metrics, issues []AggregatedIssue) []Recommendation {
    var recommendations []Recommendation
    
    // 基于延迟分析
    if metrics.Latency.P95 > 500*time.Millisecond {
        if metrics.Latency.Breakdown.L2Avg > 200*time.Millisecond {
            recommendations = append(recommendations, Recommendation{
                ID:          "perf-l2-slow",
                Priority:    "high",
                Category:    "performance",
                Title:       "L2 向量搜索延迟过高",
                Description: fmt.Sprintf("L2 平均延迟 %v，超过建议阈值 200ms", metrics.Latency.Breakdown.L2Avg),
                Action:      "考虑增加向量索引分片或启用 HNSW 索引",
                SuggestedChanges: map[string]interface{}{
                    "l2.index_type": "hnsw",
                    "l2.ef_search":  64,
                },
            })
        }
    }
    
    // 基于相关性分析
    if metrics.Relevance.AvgSimilarityScore < 0.7 {
        recommendations = append(recommendations, Recommendation{
            ID:          "quality-low-relevance",
            Priority:    "high",
            Category:    "quality",
            Title:       "检索相关性评分偏低",
            Description: fmt.Sprintf("平均相似度 %.2f，低于 0.7 的建议阈值", metrics.Relevance.AvgSimilarityScore),
            Action:      "建议调低相似度阈值或优化 embedding 模型",
            SuggestedChanges: map[string]interface{}{
                "retrieval.similarity_threshold": 0.65,
            },
        })
    }
    
    // 基于使用率分析
    if metrics.Utilization.ContentUsageRate < 0.5 {
        recommendations = append(recommendations, Recommendation{
            ID:          "efficiency-low-usage",
            Priority:    "medium",
            Category:    "efficiency",
            Title:       "召回内容使用率过低",
            Description: fmt.Sprintf("仅 %.0f%% 的召回内容被 LLM 使用", metrics.Utilization.ContentUsageRate*100),
            Action:      "建议减少 top_k 数量或优化重排序策略",
            SuggestedChanges: map[string]interface{}{
                "retrieval.top_k": 5,
                "retrieval.rerank_top_n": 3,
            },
        })
    }
    
    return recommendations
}
```

## 4. Tap 中间件实现

```go
// TapMiddleware 旁路采集中间件
type TapMiddleware struct {
    collector *Collector
    enabled   bool
}

// NewTapMiddleware 创建中间件
func NewTapMiddleware(collector *Collector) *TapMiddleware {
    return &TapMiddleware{
        collector: collector,
        enabled:   true,
    }
}

// Wrap 包装 Memory 处理函数
func (m *TapMiddleware) Wrap(handler MemoryHandler) MemoryHandler {
    return func(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error) {
        if !m.enabled {
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
        
        // 采集事件
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
            event.RetrievedChunks = m.extractChunkInfo(resp)
            event.TotalTokens = resp.TotalTokens
        }
        
        // 异步发送到收集器 (不阻塞主流程)
        go m.collector.Collect(event)
        
        return resp, err
    }
}
```

## 5. Dashboard API

```go
// DashboardHandler 仪表盘 API
type DashboardHandler struct {
    observer *Observer
}

// RegisterRoutes 注册路由
func (h *DashboardHandler) RegisterRoutes(router *gin.Engine) {
    dashboard := router.Group("/api/v1/observatory")
    {
        // 实时指标
        dashboard.GET("/metrics", h.handleGetMetrics)
        dashboard.GET("/metrics/stream", h.handleMetricsStream)  // SSE
        
        // 报告
        dashboard.GET("/report", h.handleGetReport)
        dashboard.GET("/report/export", h.handleExportReport)
        
        // 事件查询
        dashboard.GET("/events", h.handleListEvents)
        dashboard.GET("/events/:event_id", h.handleGetEvent)
        
        // 问题
        dashboard.GET("/issues", h.handleListIssues)
        
        // 建议
        dashboard.GET("/recommendations", h.handleGetRecommendations)
        
        // 配置
        dashboard.GET("/config", h.handleGetConfig)
        dashboard.PUT("/config", h.handleUpdateConfig)
    }
}

// 实时指标 SSE 推送
func (h *DashboardHandler) handleMetricsStream(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            metrics := h.observer.GetRealtimeMetrics()
            data, _ := json.Marshal(metrics)
            c.SSEvent("metrics", string(data))
            c.Writer.Flush()
        case <-c.Request.Context().Done():
            return
        }
    }
}
```

## 6. 配置

```json
{
    "observatory": {
        "enabled": true,
        "sample_rate": 1.0,
        "buffer_size": 10000,
        
        "collector": {
            "async": true,
            "batch_size": 100,
            "flush_interval_ms": 1000
        },
        
        "analyzer": {
            "window_sizes": ["1m", "5m", "1h", "24h"],
            "retention_hours": 168
        },
        
        "evaluator": {
            "enable_llm_analysis": false,
            "llm_model": "deepseek-chat",
            "llm_sample_rate": 0.1
        },
        
        "reporter": {
            "auto_report": true,
            "report_interval": "1h",
            "export_format": ["json", "html"]
        },
        
        "dashboard": {
            "enabled": true,
            "listen_addr": ":8081"
        },
        
        "alerts": {
            "enabled": true,
            "rules": [
                {
                    "name": "high_latency",
                    "condition": "latency_p95 > 500ms",
                    "severity": "warning"
                },
                {
                    "name": "low_success_rate",
                    "condition": "success_rate < 0.95",
                    "severity": "critical"
                }
            ]
        }
    }
}
```

## 7. 实现计划

1. **Phase 1**: Tap 中间件 + 事件采集器 + 环形缓冲
2. **Phase 2**: 指标分析器 + 基础统计计算
3. **Phase 3**: 质量评估器 + 问题检测
4. **Phase 4**: 报告生成器 + 优化建议
5. **Phase 5**: Dashboard API + 实时推送
6. **Phase 6**: LLM 反馈分析 (可选)
