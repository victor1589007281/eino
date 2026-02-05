# 意图识别引擎设计

## 1. 概述

当前实现使用正则表达式和关键词匹配，存在以下问题：
- 无法处理语义相似但表达不同的输入
- 规则维护成本高，扩展性差
- 缺乏上下文理解能力
- 无法处理模糊意图

本文档设计基于 **ML + LLM 的混合意图识别方案**。

## 2. 架构设计

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Intent Recognition Engine                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                        Router (路由层)                        │   │
│  │  • 根据配置选择识别策略                                        │   │
│  │  • 支持 A/B 测试                                              │   │
│  │  • 自动降级处理                                               │   │
│  └─────────────────────────┬────────────────────────────────────┘   │
│                            │                                        │
│          ┌─────────────────┼─────────────────┐                      │
│          ▼                 ▼                 ▼                      │
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐             │
│  │  Rule Engine  │ │  ML Engine    │ │  LLM Engine   │             │
│  │  (规则引擎)    │ │  (机器学习)    │ │  (大模型)      │             │
│  ├───────────────┤ ├───────────────┤ ├───────────────┤             │
│  │ • 正则匹配    │ │ • FastText    │ │ • OpenAI API  │             │
│  │ • 关键词表    │ │ • BERT分类    │ │ • Claude API  │             │
│  │ • 模式库      │ │ • 意图聚类    │ │ • 本地LLM     │             │
│  ├───────────────┤ ├───────────────┤ ├───────────────┤             │
│  │ 延迟: <1ms    │ │ 延迟: 5-20ms  │ │ 延迟: 100-500ms│            │
│  │ 准确率: 70%   │ │ 准确率: 85%   │ │ 准确率: 95%   │             │
│  │ 成本: 免费    │ │ 成本: 计算    │ │ 成本: API     │             │
│  └───────┬───────┘ └───────┬───────┘ └───────┬───────┘             │
│          │                 │                 │                      │
│          └─────────────────┼─────────────────┘                      │
│                            ▼                                        │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                    Fusion Layer (融合层)                      │   │
│  │  • 多模型结果投票                                             │   │
│  │  • 置信度加权                                                 │   │
│  │  • 冲突解决                                                   │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## 3. 核心接口设计

```go
// IntentRecognizer 意图识别器接口
type IntentRecognizer interface {
    // Recognize 识别意图
    Recognize(ctx context.Context, input *RecognitionInput) (*RecognitionResult, error)
    
    // Name 引擎名称
    Name() string
    
    // Capabilities 能力描述
    Capabilities() RecognizerCapabilities
    
    // HealthCheck 健康检查
    HealthCheck(ctx context.Context) error
}

// RecognitionInput 识别输入
type RecognitionInput struct {
    Text        string                 `json:"text"`
    Context     []*ContextMessage      `json:"context,omitempty"`
    Domain      string                 `json:"domain"` // email, memory, search
    Language    string                 `json:"language"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// RecognitionResult 识别结果
type RecognitionResult struct {
    PrimaryIntent    Intent             `json:"primary_intent"`
    SecondaryIntents []Intent           `json:"secondary_intents,omitempty"`
    Entities         []*ExtractedEntity `json:"entities,omitempty"`
    Confidence       float64            `json:"confidence"`
    Source           string             `json:"source"` // rule/ml/llm
    Reasoning        string             `json:"reasoning,omitempty"`
    ProcessingTime   time.Duration      `json:"processing_time"`
}

// Intent 意图
type Intent struct {
    Name       string            `json:"name"`
    Confidence float64           `json:"confidence"`
    Slots      map[string]string `json:"slots,omitempty"`
}

// ExtractedEntity 提取的实体
type ExtractedEntity struct {
    Text   string  `json:"text"`
    Type   string  `json:"type"`
    Start  int     `json:"start"`
    End    int     `json:"end"`
    Score  float64 `json:"score"`
}
```

## 4. ML 引擎设计

### 4.1 模型选择

| 模型 | 用途 | 特点 |
|------|------|------|
| **FastText** | 快速分类 | 轻量、训练快、适合关键词特征 |
| **BERT-tiny** | 意图分类 | 语义理解好、推理较快 |
| **BGE-M3** | 向量化 | 多语言、效果好 |

### 4.2 训练数据格式

```json
{
  "samples": [
    {
      "text": "帮我找一下最近一周的发票邮件",
      "domain": "email",
      "intent": "search_email",
      "slots": {
        "time_range": "最近一周",
        "email_type": "发票"
      },
      "entities": [
        {"text": "最近一周", "type": "TIME_RANGE"},
        {"text": "发票", "type": "EMAIL_TYPE"}
      ]
    }
  ]
}
```

### 4.3 ML 引擎实现

```go
// MLRecognizer ML 意图识别器
type MLRecognizer struct {
    classifier    *TextClassifier
    nerModel      *NERModel
    slotExtractor *SlotExtractor
    modelPath     string
}

// TextClassifier 文本分类器 (FastText/BERT)
type TextClassifier struct {
    modelType string // "fasttext" or "bert"
    labels    []string
    // 模型具体实现
}

// Recognize 识别意图
func (r *MLRecognizer) Recognize(ctx context.Context, input *RecognitionInput) (*RecognitionResult, error) {
    start := time.Now()
    
    // 1. 文本预处理
    processedText := r.preprocess(input.Text)
    
    // 2. 分类预测
    classification, err := r.classifier.Predict(ctx, processedText)
    if err != nil {
        return nil, fmt.Errorf("classification failed: %w", err)
    }
    
    // 3. NER 实体提取
    entities, err := r.nerModel.Extract(ctx, processedText)
    if err != nil {
        // NER 失败不影响主流程
        entities = nil
    }
    
    // 4. 槽位填充
    slots := r.slotExtractor.Extract(ctx, processedText, entities)
    
    return &RecognitionResult{
        PrimaryIntent: Intent{
            Name:       classification.Label,
            Confidence: classification.Score,
            Slots:      slots,
        },
        Entities:       entities,
        Confidence:     classification.Score,
        Source:         "ml",
        ProcessingTime: time.Since(start),
    }, nil
}
```

## 5. LLM 引擎设计

### 5.1 Prompt 模板

```go
const EmailIntentPrompt = `你是一个邮件意图分析专家。请分析以下用户查询，识别其意图。

## 可用意图类型
- search_email: 搜索邮件
- read_email: 阅读邮件详情
- download_attachment: 下载附件
- categorize_email: 邮件分类
- summarize_email: 邮件摘要

## 可提取的槽位
- time_range: 时间范围
- sender: 发件人
- subject: 主题关键词
- email_type: 邮件类型(发票/报告/会议等)
- attachment_type: 附件类型

## 用户查询
{{.Query}}

## 历史上下文
{{range .Context}}
- {{.Role}}: {{.Content}}
{{end}}

请以JSON格式返回分析结果：
{
  "intent": "意图名称",
  "confidence": 0.0-1.0,
  "slots": {"槽位名": "值"},
  "entities": [{"text": "实体文本", "type": "实体类型"}],
  "reasoning": "分析理由"
}`

const MemoryIntentPrompt = `你是一个上下文理解专家。请分析用户查询是否需要历史上下文。

## 需要分析的维度
1. 是否引用之前的对话（代词指代、话题延续）
2. 是否是全新的独立问题
3. 如果需要历史，需要哪些相关主题
4. 时间相关性（最近的、某个时间的）

## 用户查询
{{.Query}}

## 当前上下文
{{range .RecentMessages}}
[{{.Role}}]: {{.Content}}
{{end}}

请以JSON格式返回：
{
  "needs_history": true/false,
  "history_type": "recent/topic_specific/entity_related",
  "related_topics": ["主题1", "主题2"],
  "reference_type": "pronoun/continuation/explicit",
  "time_relevance": "recent/specific/all",
  "confidence": 0.0-1.0,
  "reasoning": "分析理由"
}`
```

### 5.2 LLM 引擎实现

```go
// LLMRecognizer LLM 意图识别器
type LLMRecognizer struct {
    client       LLMClient
    prompts      map[string]*template.Template
    maxTokens    int
    temperature  float64
    retryConfig  RetryConfig
}

// LLMClient LLM 客户端接口
type LLMClient interface {
    Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}

// Recognize 识别意图
func (r *LLMRecognizer) Recognize(ctx context.Context, input *RecognitionInput) (*RecognitionResult, error) {
    start := time.Now()
    
    // 1. 选择 prompt 模板
    promptTpl, ok := r.prompts[input.Domain]
    if !ok {
        promptTpl = r.prompts["default"]
    }
    
    // 2. 渲染 prompt
    prompt, err := r.renderPrompt(promptTpl, input)
    if err != nil {
        return nil, fmt.Errorf("render prompt failed: %w", err)
    }
    
    // 3. 调用 LLM
    resp, err := r.client.Complete(ctx, &CompletionRequest{
        Prompt:      prompt,
        MaxTokens:   r.maxTokens,
        Temperature: r.temperature,
    })
    if err != nil {
        return nil, fmt.Errorf("llm call failed: %w", err)
    }
    
    // 4. 解析响应
    result, err := r.parseResponse(resp.Text)
    if err != nil {
        return nil, fmt.Errorf("parse response failed: %w", err)
    }
    
    result.Source = "llm"
    result.ProcessingTime = time.Since(start)
    
    return result, nil
}
```

## 6. 融合策略

### 6.1 级联模式 (Cascade)

```go
// CascadeStrategy 级联策略：快 → 准
func (e *IntentEngine) CascadeRecognize(ctx context.Context, input *RecognitionInput) (*RecognitionResult, error) {
    // 1. 先用规则引擎（最快）
    ruleResult, err := e.ruleEngine.Recognize(ctx, input)
    if err == nil && ruleResult.Confidence > 0.9 {
        return ruleResult, nil
    }
    
    // 2. 规则置信度不够，用 ML
    mlResult, err := e.mlEngine.Recognize(ctx, input)
    if err == nil && mlResult.Confidence > 0.85 {
        return mlResult, nil
    }
    
    // 3. ML 置信度不够，用 LLM
    llmResult, err := e.llmEngine.Recognize(ctx, input)
    if err == nil {
        return llmResult, nil
    }
    
    // 4. 返回可用的最好结果
    return e.selectBest(ruleResult, mlResult, llmResult), nil
}
```

### 6.2 投票模式 (Voting)

```go
// VotingStrategy 投票策略：多模型投票
func (e *IntentEngine) VotingRecognize(ctx context.Context, input *RecognitionInput) (*RecognitionResult, error) {
    var wg sync.WaitGroup
    results := make([]*RecognitionResult, 3)
    
    // 并行调用三个引擎
    wg.Add(3)
    go func() { defer wg.Done(); results[0], _ = e.ruleEngine.Recognize(ctx, input) }()
    go func() { defer wg.Done(); results[1], _ = e.mlEngine.Recognize(ctx, input) }()
    go func() { defer wg.Done(); results[2], _ = e.llmEngine.Recognize(ctx, input) }()
    wg.Wait()
    
    // 加权投票
    weights := []float64{0.2, 0.3, 0.5} // rule, ml, llm 权重
    return e.weightedVote(results, weights), nil
}
```

### 6.3 自适应模式 (Adaptive)

```go
// AdaptiveStrategy 自适应策略：根据输入特征选择引擎
func (e *IntentEngine) AdaptiveRecognize(ctx context.Context, input *RecognitionInput) (*RecognitionResult, error) {
    features := e.extractFeatures(input)
    
    // 简单明确的查询 → 规则引擎
    if features.Clarity > 0.9 && features.HasPattern {
        return e.ruleEngine.Recognize(ctx, input)
    }
    
    // 有上下文依赖 → LLM
    if features.ContextDependent || features.Ambiguous {
        return e.llmEngine.Recognize(ctx, input)
    }
    
    // 默认 → ML
    return e.mlEngine.Recognize(ctx, input)
}
```

## 7. 配置示例

```yaml
intent:
  # 默认策略
  strategy: cascade  # cascade | voting | adaptive
  
  # 规则引擎配置
  rule_engine:
    enabled: true
    patterns_file: "config/intent_patterns.yaml"
  
  # ML 引擎配置
  ml_engine:
    enabled: true
    model_type: fasttext  # fasttext | bert
    model_path: "models/intent_classifier"
    confidence_threshold: 0.85
  
  # LLM 引擎配置
  llm_engine:
    enabled: true
    provider: openai  # openai | claude | local
    model: gpt-4o-mini
    temperature: 0.1
    max_tokens: 500
    timeout: 10s
    
  # 融合配置
  fusion:
    strategy: weighted_vote
    weights:
      rule: 0.2
      ml: 0.3
      llm: 0.5
    confidence_threshold: 0.7
```

## 8. 领域特化

### 8.1 邮件意图

```yaml
email_intents:
  - name: search_email
    description: 搜索邮件
    examples:
      - "找一下上周的发票邮件"
      - "帮我搜索来自张三的邮件"
    slots:
      - time_range
      - sender
      - subject_keyword
      - email_type
  
  - name: read_email
    description: 阅读邮件
    examples:
      - "打开最新的邮件"
      - "看看这封邮件的内容"
    slots:
      - email_id
      - position  # latest, previous
  
  - name: download_attachment
    description: 下载附件
    examples:
      - "下载那个PDF附件"
      - "把附件保存下来"
    slots:
      - attachment_type
      - email_id
```

### 8.2 记忆意图

```yaml
memory_intents:
  - name: reference_history
    description: 引用历史上下文
    patterns:
      - "之前.*说的"
      - "刚才.*提到的"
      - "上次.*讨论的"
    
  - name: topic_switch
    description: 切换话题
    patterns:
      - "回到.*话题"
      - "继续.*之前"
    
  - name: new_topic
    description: 开始新话题
    patterns:
      - "新话题"
      - "换个话题"
      - "另外一个问题"
```

## 9. 监控与评估

```go
// IntentMetrics 意图识别指标
type IntentMetrics struct {
    TotalRequests    int64           `json:"total_requests"`
    ByEngine         map[string]int64 `json:"by_engine"`
    AvgLatency       time.Duration   `json:"avg_latency"`
    ConfidenceHist   []float64       `json:"confidence_histogram"`
    SuccessRate      float64         `json:"success_rate"`
    FallbackRate     float64         `json:"fallback_rate"`
}

// RecordRecognition 记录识别结果
func (m *MetricsCollector) RecordRecognition(result *RecognitionResult) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    m.totalRequests++
    m.byEngine[result.Source]++
    m.latencies = append(m.latencies, result.ProcessingTime)
    m.confidences = append(m.confidences, result.Confidence)
}
```
