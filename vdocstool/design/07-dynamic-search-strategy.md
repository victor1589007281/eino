# 动态搜索策略设计文档

## 1. 概述

### 1.1 背景
当前搜索工具支持多个搜索引擎（Serper、Tavily、Exa.ai、DuckDuckGo、SearXNG 等），但路由策略相对简单，未能充分考虑：
- API 配额消耗与剩余额度
- 搜索结果质量的动态变化
- 不同引擎对不同查询类型的适配性
- 成本与质量的平衡

### 1.2 目标
设计一个智能动态搜索策略系统，以**搜索精准度**和**成功率**为核心优化目标，实现：
1. 实时监控各引擎的配额状态、响应质量、成功率
2. 基于历史数据动态调整引擎优先级和权重
3. 智能识别查询类型，匹配最优引擎
4. 配额耗尽或质量下降时自动平滑切换

## 2. 系统架构

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Dynamic Search Strategy System                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    Query Classifier (查询分类器)                     │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │ │
│  │  │技术/代码  │  │ 新闻时事 │  │ 学术研究 │  │ 中文本地 │           │ │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                              │                                           │
│                              ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    Engine Scorer (引擎评分器)                        │ │
│  │                                                                      │ │
│  │   Score = w1*QualityScore + w2*SuccessRate + w3*QuotaHealth        │ │
│  │           + w4*QueryTypeMatch + w5*LatencyScore - w6*CostFactor    │ │
│  │                                                                      │ │
│  │  ┌─────────────────────────────────────────────────────────────┐   │ │
│  │  │ Engine        Quality  Success  Quota   Match  Latency  Cost │   │ │
│  │  │ ─────────────────────────────────────────────────────────── │   │ │
│  │  │ serper        0.92     0.98     0.45    0.90   0.85    0.30 │   │ │
│  │  │ tavily        0.88     0.95     0.60    0.85   0.80    0.25 │   │ │
│  │  │ exa           0.85     0.92     0.70    0.95*  0.75    0.20 │   │ │
│  │  │ duckduckgo    0.65     0.80     1.00    0.60   0.70    0.00 │   │ │
│  │  │ searxng       0.60     0.75     1.00    0.55   0.60    0.00 │   │ │
│  │  └─────────────────────────────────────────────────────────────┘   │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                              │                                           │
│                              ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                  Adaptive Router (自适应路由器)                       │ │
│  │                                                                      │ │
│  │  策略模式:                                                           │ │
│  │  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐           │ │
│  │  │ Quality First │  │ Cost Saving   │  │   Balanced    │           │ │
│  │  │ 质量优先      │  │ 成本节约      │  │    均衡       │           │ │
│  │  └───────────────┘  └───────────────┘  └───────────────┘           │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                              │                                           │
│                              ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                   Quota Manager (配额管理器)                         │ │
│  │                                                                      │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                 │ │
│  │  │ 配额追踪    │  │ 阈值告警    │  │ 预算分配    │                 │ │
│  │  │ 实时统计    │  │ 自动降级    │  │ 周期重置    │                 │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘                 │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                              │                                           │
│                              ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                  Quality Tracker (质量追踪器)                        │ │
│  │                                                                      │ │
│  │  • 结果相关性评估 (基于点击率、停留时间、用户反馈)                   │ │
│  │  • 结果多样性评估                                                   │ │
│  │  • 时效性评估 (新闻类查询)                                          │ │
│  │  • 滑动窗口统计 (最近 N 次查询)                                     │ │
│  └────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

## 3. 核心组件设计

### 3.1 查询分类器 (Query Classifier)

```go
// QueryType 查询类型
type QueryType string

const (
    QueryTypeTechnical  QueryType = "technical"   // 技术/代码/API文档
    QueryTypeNews       QueryType = "news"        // 新闻/时事
    QueryTypeAcademic   QueryType = "academic"    // 学术/研究
    QueryTypeLocal      QueryType = "local"       // 本地化/中文
    QueryTypeGeneral    QueryType = "general"     // 通用查询
    QueryTypeShopping   QueryType = "shopping"    // 购物/产品
    QueryTypeJob        QueryType = "job"         // 招聘/职位
)

// QueryClassifier 查询分类器
type QueryClassifier struct {
    // 关键词规则
    technicalKeywords []string  // golang, API, SDK, error, debug...
    newsKeywords      []string  // 最新, 今日, 新闻, breaking...
    academicKeywords  []string  // 论文, 研究, paper, research...
    
    // 语言检测
    chineseRatio      float64   // 中文字符比例阈值
}

// Classify 分类查询
func (c *QueryClassifier) Classify(query string) QueryClassification {
    return QueryClassification{
        PrimaryType:   c.detectPrimaryType(query),
        SecondaryType: c.detectSecondaryType(query),
        Language:      c.detectLanguage(query),
        Confidence:    c.calculateConfidence(query),
    }
}
```

**引擎-查询类型匹配矩阵：**

| 查询类型 | Serper | Tavily | Exa | DuckDuckGo | SearXNG | Baidu |
|---------|--------|--------|-----|------------|---------|-------|
| Technical | 0.95 | 0.90 | **0.98** | 0.70 | 0.65 | 0.50 |
| News | **0.95** | 0.85 | 0.60 | 0.75 | 0.70 | 0.80 |
| Academic | 0.85 | 0.80 | **0.95** | 0.70 | 0.75 | 0.60 |
| Local(CN) | 0.70 | 0.60 | 0.50 | 0.40 | 0.50 | **0.90** |
| General | 0.90 | 0.85 | 0.75 | 0.75 | 0.70 | 0.70 |

### 3.2 配额管理器 (Quota Manager)

```go
// QuotaManager 配额管理器
type QuotaManager struct {
    quotas map[string]*EngineQuota
    mu     sync.RWMutex
}

// EngineQuota 引擎配额
type EngineQuota struct {
    EngineName     string
    
    // 配额设置
    MonthlyLimit   int64          // 月度限额
    DailyLimit     int64          // 日限额（可选）
    
    // 使用统计
    MonthlyUsed    int64          // 本月已用
    DailyUsed      int64          // 今日已用
    LastResetTime  time.Time      // 上次重置时间
    
    // 阈值设置
    WarningThreshold  float64     // 告警阈值 (0.7 = 70%)
    CriticalThreshold float64     // 临界阈值 (0.9 = 90%)
    
    // 状态
    Status         QuotaStatus    // healthy/warning/critical/exhausted
}

// QuotaStatus 配额状态
type QuotaStatus string

const (
    QuotaHealthy   QuotaStatus = "healthy"   // < 70%
    QuotaWarning   QuotaStatus = "warning"   // 70% - 90%
    QuotaCritical  QuotaStatus = "critical"  // 90% - 100%
    QuotaExhausted QuotaStatus = "exhausted" // >= 100%
)

// GetQuotaHealth 获取配额健康度 (0-1)
func (q *EngineQuota) GetQuotaHealth() float64 {
    if q.MonthlyLimit <= 0 {
        return 1.0 // 无限制
    }
    
    usageRatio := float64(q.MonthlyUsed) / float64(q.MonthlyLimit)
    
    // 非线性衰减：使用率越高，健康度下降越快
    if usageRatio < 0.5 {
        return 1.0
    } else if usageRatio < 0.7 {
        return 1.0 - (usageRatio-0.5)*0.5  // 0.9 - 1.0
    } else if usageRatio < 0.9 {
        return 0.9 - (usageRatio-0.7)*1.5  // 0.6 - 0.9
    } else if usageRatio < 1.0 {
        return 0.6 - (usageRatio-0.9)*5.0  // 0.1 - 0.6
    }
    return 0.0 // 已耗尽
}
```

**配额预警与降级策略：**

```
配额使用率    状态        行为
─────────────────────────────────────────
< 50%        Healthy    正常使用，无限制
50% - 70%    Healthy    正常使用，记录日志
70% - 90%    Warning    降低优先级，减少非必要调用
90% - 100%   Critical   仅用于高优先级查询，预备降级
>= 100%      Exhausted  完全禁用，自动切换备选引擎
```

### 3.3 质量追踪器 (Quality Tracker)

```go
// QualityTracker 质量追踪器
type QualityTracker struct {
    // 滑动窗口存储
    windows map[string]*SlidingWindow  // engine -> window
    
    // 质量评估器
    evaluator *QualityEvaluator
}

// QualityMetrics 质量指标
type QualityMetrics struct {
    // 基础指标
    SuccessRate      float64  // 成功率 (非错误响应比例)
    ResultCount      float64  // 平均结果数
    LatencyP50       float64  // P50 延迟
    LatencyP95       float64  // P95 延迟
    
    // 质量指标 (需要反馈数据)
    RelevanceScore   float64  // 相关性评分 (基于用户行为)
    DiversityScore   float64  // 多样性评分
    FreshnessScore   float64  // 时效性评分
    
    // 稳定性指标
    QualityVariance  float64  // 质量波动 (越低越稳定)
    ConsecutiveFails int      // 连续失败次数
}

// SlidingWindow 滑动窗口
type SlidingWindow struct {
    Size     int
    Records  []*SearchRecord
    Index    int
}

// SearchRecord 搜索记录
type SearchRecord struct {
    Timestamp    time.Time
    QueryType    QueryType
    ResultCount  int
    Latency      time.Duration
    Success      bool
    ErrorType    string
    
    // 质量反馈 (异步更新)
    UserClicked  bool     // 用户是否点击了结果
    ClickedIndex int      // 点击的结果序号
    DwellTime    float64  // 停留时间
}
```

### 3.4 引擎评分器 (Engine Scorer)

```go
// EngineScorer 引擎评分器
type EngineScorer struct {
    qualityTracker *QualityTracker
    quotaManager   *QuotaManager
    classifier     *QueryClassifier
    
    // 权重配置
    weights ScorerWeights
}

// ScorerWeights 评分权重
type ScorerWeights struct {
    Quality        float64  // 质量权重 (默认 0.30)
    SuccessRate    float64  // 成功率权重 (默认 0.25)
    QuotaHealth    float64  // 配额健康权重 (默认 0.15)
    QueryTypeMatch float64  // 查询类型匹配权重 (默认 0.15)
    Latency        float64  // 延迟权重 (默认 0.10)
    Cost           float64  // 成本权重 (默认 0.05，负向)
}

// Score 计算引擎得分
func (s *EngineScorer) Score(engine string, query string) float64 {
    classification := s.classifier.Classify(query)
    metrics := s.qualityTracker.GetMetrics(engine)
    quota := s.quotaManager.GetQuota(engine)
    
    // 计算各维度得分
    qualityScore := metrics.RelevanceScore
    successScore := metrics.SuccessRate
    quotaScore := quota.GetQuotaHealth()
    matchScore := s.getQueryTypeMatch(engine, classification.PrimaryType)
    latencyScore := s.normalizeLatency(metrics.LatencyP50)
    costScore := s.getEngineCost(engine)
    
    // 加权求和
    score := s.weights.Quality * qualityScore +
             s.weights.SuccessRate * successScore +
             s.weights.QuotaHealth * quotaScore +
             s.weights.QueryTypeMatch * matchScore +
             s.weights.Latency * latencyScore -
             s.weights.Cost * costScore
    
    // 惩罚项：连续失败
    if metrics.ConsecutiveFails > 0 {
        penalty := 0.1 * float64(metrics.ConsecutiveFails)
        score = score * (1 - math.Min(penalty, 0.5))
    }
    
    return math.Max(0, math.Min(1, score))
}
```

### 3.5 自适应路由器增强

```go
// AdaptiveRouter 自适应路由器
type AdaptiveRouter struct {
    engines        map[string]Engine
    scorer         *EngineScorer
    quotaManager   *QuotaManager
    qualityTracker *QualityTracker
    
    // 策略配置
    strategy       RoutingStrategy
    
    // 缓存
    scoreCache     *ScoreCache
    cacheTTL       time.Duration
}

// RoutingStrategy 路由策略
type RoutingStrategy string

const (
    StrategyQualityFirst RoutingStrategy = "quality_first"  // 质量优先
    StrategyCostSaving   RoutingStrategy = "cost_saving"    // 成本节约
    StrategyBalanced     RoutingStrategy = "balanced"       // 均衡
)

// Route 路由请求
func (r *AdaptiveRouter) Route(ctx context.Context, req *SearchRequest) (*RouteResult, error) {
    // 1. 获取所有可用引擎的得分
    scores := r.scoreAllEngines(req.Query)
    
    // 2. 根据策略选择引擎
    selectedEngine := r.selectByStrategy(scores, r.strategy)
    
    // 3. 执行搜索
    startTime := time.Now()
    result, err := r.executeSearch(ctx, selectedEngine, req)
    latency := time.Since(startTime)
    
    // 4. 记录结果 (用于质量追踪)
    r.recordSearchResult(selectedEngine, req, result, err, latency)
    
    // 5. 如果失败，尝试降级
    if err != nil && r.shouldFallback(err) {
        return r.fallbackSearch(ctx, req, scores, selectedEngine)
    }
    
    // 6. 更新配额
    r.quotaManager.IncrementUsage(selectedEngine)
    
    return result, err
}

// selectByStrategy 根据策略选择引擎
func (r *AdaptiveRouter) selectByStrategy(scores map[string]float64, strategy RoutingStrategy) string {
    switch strategy {
    case StrategyQualityFirst:
        // 选择得分最高的
        return r.selectTopScorer(scores)
        
    case StrategyCostSaving:
        // 优先选择免费引擎，除非质量差距太大
        freeEngines := r.filterFreeEngines(scores)
        if len(freeEngines) > 0 {
            topFree := r.selectTopScorer(freeEngines)
            topPaid := r.selectTopScorer(scores)
            
            // 如果免费引擎得分 > 付费引擎 * 0.7，使用免费
            if freeEngines[topFree] >= scores[topPaid]*0.7 {
                return topFree
            }
        }
        return r.selectTopScorer(scores)
        
    case StrategyBalanced:
        // 加权随机选择 (得分越高，被选中概率越大)
        return r.weightedRandomSelect(scores)
        
    default:
        return r.selectTopScorer(scores)
    }
}
```

## 4. 配额预设配置

```go
// DefaultQuotaConfig 默认配额配置
var DefaultQuotaConfig = map[string]QuotaConfig{
    "serper": {
        MonthlyLimit:      2500,
        DailyLimit:        100,
        CostPerQuery:      0.001,  // $0.001/query
        IsFree:            false,
    },
    "tavily": {
        MonthlyLimit:      1000,
        DailyLimit:        50,
        CostPerQuery:      0.001,
        IsFree:            false,
    },
    "exa": {
        MonthlyLimit:      1000,
        DailyLimit:        50,
        CostPerQuery:      0.001,
        IsFree:            false,
    },
    "brave": {
        MonthlyLimit:      2000,
        DailyLimit:        100,
        CostPerQuery:      0.0005,
        IsFree:            false,
    },
    "duckduckgo": {
        MonthlyLimit:      0,  // 无限制
        DailyLimit:        0,
        CostPerQuery:      0,
        IsFree:            true,
    },
    "searxng": {
        MonthlyLimit:      0,
        DailyLimit:        0,
        CostPerQuery:      0,
        IsFree:            true,
    },
    "bing": {
        MonthlyLimit:      0,
        DailyLimit:        0,
        CostPerQuery:      0,
        IsFree:            true,
    },
    "baidu": {
        MonthlyLimit:      0,
        DailyLimit:        0,
        CostPerQuery:      0,
        IsFree:            true,
    },
}
```

## 5. 状态持久化

```go
// StatePersistence 状态持久化
type StatePersistence struct {
    filePath string
}

// State 持久化状态
type State struct {
    QuotaUsage    map[string]*QuotaUsageState   `json:"quota_usage"`
    QualityStats  map[string]*QualityStatsState `json:"quality_stats"`
    LastSaveTime  time.Time                     `json:"last_save_time"`
}

// 定期保存 (每分钟) 和程序退出时保存
// 启动时加载恢复状态
```

## 6. MCP 工具扩展

新增以下 MCP 工具：

```go
// 1. 获取引擎状态
get_engine_status:
    返回所有引擎的配额、质量、得分等完整状态

// 2. 设置路由策略
set_search_strategy:
    strategy: quality_first | cost_saving | balanced

// 3. 手动调整引擎权重
adjust_engine_priority:
    engine: string
    priority_boost: float  // -1.0 到 1.0

// 4. 获取配额报告
get_quota_report:
    返回本月/本日各引擎的使用统计和预测
```

## 7. 监控与告警

```go
// 告警规则
type AlertRule struct {
    QuotaWarningThreshold   float64  // 配额告警阈值 (默认 0.7)
    SuccessRateThreshold    float64  // 成功率告警阈值 (默认 0.8)
    ConsecutiveFailThreshold int     // 连续失败告警阈值 (默认 3)
    LatencyThreshold        time.Duration  // 延迟告警阈值 (默认 5s)
}

// 告警通知方式
// - 日志记录 (始终)
// - MCP 工具返回警告信息
// - Webhook 通知 (可配置)
```

## 8. 实现计划

1. **Phase 1**: 配额管理器 + 基础评分
2. **Phase 2**: 查询分类器 + 类型匹配
3. **Phase 3**: 质量追踪器 + 反馈机制
4. **Phase 4**: 状态持久化 + 监控告警
