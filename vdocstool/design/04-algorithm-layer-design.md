# 算法层抽离架构设计

## 1. 设计目标

将意图识别、向量化、NLP 处理等智能能力从业务工具中抽离，形成独立的算法层：

1. **解耦**：工具层专注业务逻辑，算法层专注智能能力
2. **复用**：三个工具（Search/Email/Memory）共享算法能力
3. **可插拔**：支持不同算法实现（规则/ML/LLM）的无缝切换
4. **可测试**：算法能力独立测试，不依赖业务上下文

## 2. 架构总览

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          Tool Layer (工具层)                             │
│   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐                  │
│   │ Search Tool │   │ Email Tool  │   │ Memory Tool │                  │
│   └──────┬──────┘   └──────┬──────┘   └──────┬──────┘                  │
│          │                 │                 │                          │
│          └─────────────────┼─────────────────┘                          │
│                            │                                            │
│                            ▼                                            │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                   Algorithm Facade (算法门面)                    │  │
│   │   • 统一入口                                                     │  │
│   │   • 错误处理                                                     │  │
│   │   • 监控埋点                                                     │  │
│   └──────────────────────────┬──────────────────────────────────────┘  │
└──────────────────────────────┼──────────────────────────────────────────┘
                               │
┌──────────────────────────────┼──────────────────────────────────────────┐
│                              ▼                                          │
│                    Algorithm Layer (算法层)                              │
│                                                                         │
│  ┌────────────────────────────────────────────────────────────────┐    │
│  │                    Intent Engine (意图引擎)                     │    │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐                │    │
│  │  │ RuleEngine │  │ MLEngine   │  │ LLMEngine  │                │    │
│  │  │ (规则引擎)  │  │ (ML引擎)   │  │ (LLM引擎)  │                │    │
│  │  └────────────┘  └────────────┘  └────────────┘                │    │
│  └────────────────────────────────────────────────────────────────┘    │
│                                                                         │
│  ┌────────────────────────────────────────────────────────────────┐    │
│  │                  Embedding Engine (向量化引擎)                  │    │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐                │    │
│  │  │ OpenAI API │  │ BGE/M3E    │  │ Sentence-T │                │    │
│  │  │ (云端)      │  │ (本地)     │  │ (本地)     │                │    │
│  │  └────────────┘  └────────────┘  └────────────┘                │    │
│  └────────────────────────────────────────────────────────────────┘    │
│                                                                         │
│  ┌────────────────────────────────────────────────────────────────┐    │
│  │                     NLP Pipeline (NLP管道)                      │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │    │
│  │  │Tokenizer │→│   NER    │→│ Relation │→│Summarizer│           │    │
│  │  │ (分词)    │ │(实体识别)│ │(关系抽取) │ │(摘要生成) │           │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │    │
│  └────────────────────────────────────────────────────────────────┘    │
│                                                                         │
│  ┌────────────────────────────────────────────────────────────────┐    │
│  │                   Index Engine (索引引擎)                       │    │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐                │    │
│  │  │ Inverted   │  │  Vector    │  │   Graph    │                │    │
│  │  │ (倒排索引)  │  │ (向量索引)  │  │ (图索引)   │                │    │
│  │  └────────────┘  └────────────┘  └────────────┘                │    │
│  └────────────────────────────────────────────────────────────────┘    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## 3. 目录结构

```
vdocstool/
├── algorithm/                        # 算法层根目录
│   ├── algorithm.go                  # 算法门面（统一入口）
│   ├── config.go                     # 算法层配置
│   │
│   ├── intent/                       # 意图识别引擎
│   │   ├── engine.go                 # 意图引擎主入口
│   │   ├── types.go                  # 类型定义
│   │   ├── router.go                 # 策略路由
│   │   ├── fusion.go                 # 结果融合
│   │   │
│   │   ├── rule/                     # 规则引擎
│   │   │   ├── engine.go             # 规则引擎实现
│   │   │   ├── patterns.go           # 模式定义
│   │   │   └── email_rules.go        # 邮件领域规则
│   │   │   └── memory_rules.go       # 记忆领域规则
│   │   │
│   │   ├── ml/                       # ML引擎
│   │   │   ├── engine.go             # ML引擎实现
│   │   │   ├── fasttext.go           # FastText 分类器
│   │   │   ├── bert.go               # BERT 分类器
│   │   │   └── models/               # 模型文件目录
│   │   │
│   │   └── llm/                      # LLM引擎
│   │       ├── engine.go             # LLM引擎实现
│   │       ├── prompts.go            # Prompt模板
│   │       ├── openai.go             # OpenAI 适配
│   │       └── claude.go             # Claude 适配
│   │
│   ├── embedding/                    # 向量化引擎
│   │   ├── engine.go                 # 向量引擎主入口
│   │   ├── types.go                  # 类型定义
│   │   ├── openai.go                 # OpenAI text-embedding
│   │   ├── bge.go                    # BGE 本地模型
│   │   ├── sentence_transformer.go   # SentenceTransformer
│   │   └── cache.go                  # 向量缓存
│   │
│   ├── nlp/                          # NLP管道
│   │   ├── pipeline.go               # NLP管道主入口
│   │   ├── tokenizer.go              # 分词器
│   │   ├── ner.go                    # 命名实体识别
│   │   ├── relation.go               # 关系抽取
│   │   ├── summarizer.go             # 摘要生成
│   │   └── sentiment.go              # 情感分析（可选）
│   │
│   └── index/                        # 索引引擎
│       ├── manager.go                # 索引管理器
│       ├── inverted.go               # 倒排索引
│       ├── vector.go                 # 向量索引
│       ├── graph.go                  # 图索引
│       └── hybrid.go                 # 混合检索
│
└── tools/                            # 工具层（使用算法层）
    ├── search/
    ├── email/
    └── memory/
```

## 4. 核心接口设计

### 4.1 算法门面 (Algorithm Facade)

```go
// algorithm/algorithm.go

package algorithm

// Algorithm 算法层统一入口
type Algorithm struct {
    Intent    *intent.Engine
    Embedding *embedding.Engine
    NLP       *nlp.Pipeline
    Index     *index.Manager
    
    config    *Config
    metrics   *Metrics
}

// New 创建算法层
func New(cfg *Config) (*Algorithm, error) {
    alg := &Algorithm{
        config:  cfg,
        metrics: NewMetrics(),
    }
    
    // 初始化各引擎
    var err error
    
    if alg.Intent, err = intent.NewEngine(cfg.Intent); err != nil {
        return nil, fmt.Errorf("init intent engine: %w", err)
    }
    
    if alg.Embedding, err = embedding.NewEngine(cfg.Embedding); err != nil {
        return nil, fmt.Errorf("init embedding engine: %w", err)
    }
    
    if alg.NLP, err = nlp.NewPipeline(cfg.NLP); err != nil {
        return nil, fmt.Errorf("init nlp pipeline: %w", err)
    }
    
    if alg.Index, err = index.NewManager(cfg.Index); err != nil {
        return nil, fmt.Errorf("init index manager: %w", err)
    }
    
    return alg, nil
}

// Close 关闭算法层
func (a *Algorithm) Close() error {
    var errs []error
    
    if err := a.Intent.Close(); err != nil {
        errs = append(errs, err)
    }
    if err := a.Embedding.Close(); err != nil {
        errs = append(errs, err)
    }
    if err := a.NLP.Close(); err != nil {
        errs = append(errs, err)
    }
    if err := a.Index.Close(); err != nil {
        errs = append(errs, err)
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("close errors: %v", errs)
    }
    return nil
}

// HealthCheck 健康检查
func (a *Algorithm) HealthCheck(ctx context.Context) *HealthStatus {
    return &HealthStatus{
        Intent:    a.Intent.HealthCheck(ctx),
        Embedding: a.Embedding.HealthCheck(ctx),
        NLP:       a.NLP.HealthCheck(ctx),
        Index:     a.Index.HealthCheck(ctx),
    }
}
```

### 4.2 意图引擎接口

```go
// algorithm/intent/types.go

package intent

// Recognizer 意图识别器接口
type Recognizer interface {
    // Recognize 识别意图
    Recognize(ctx context.Context, input *Input) (*Result, error)
    
    // Name 引擎名称
    Name() string
    
    // Domain 支持的领域
    Domain() []string
    
    // HealthCheck 健康检查
    HealthCheck(ctx context.Context) error
}

// Input 识别输入
type Input struct {
    Text     string            `json:"text"`
    Domain   string            `json:"domain"`   // email, memory, search
    Context  []*Message        `json:"context"`  // 上下文消息
    Language string            `json:"language"` // zh, en
    Options  map[string]string `json:"options"`
}

// Result 识别结果
type Result struct {
    Intent       *Intent           `json:"intent"`
    Alternatives []*Intent         `json:"alternatives,omitempty"`
    Entities     []*Entity         `json:"entities,omitempty"`
    Confidence   float64           `json:"confidence"`
    Source       string            `json:"source"` // rule, ml, llm
    Reasoning    string            `json:"reasoning,omitempty"`
    Latency      time.Duration     `json:"latency"`
}

// Intent 意图
type Intent struct {
    Name       string            `json:"name"`
    Confidence float64           `json:"confidence"`
    Slots      map[string]string `json:"slots,omitempty"`
}

// Entity 实体
type Entity struct {
    Text   string  `json:"text"`
    Type   string  `json:"type"`
    Start  int     `json:"start"`
    End    int     `json:"end"`
    Score  float64 `json:"score"`
}
```

### 4.3 向量化引擎接口

```go
// algorithm/embedding/types.go

package embedding

// Embedder 向量化接口
type Embedder interface {
    // Embed 单文本向量化
    Embed(ctx context.Context, text string) ([]float64, error)
    
    // EmbedBatch 批量向量化
    EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
    
    // Dimension 向量维度
    Dimension() int
    
    // Name 引擎名称
    Name() string
    
    // HealthCheck 健康检查
    HealthCheck(ctx context.Context) error
}

// Similarity 相似度计算
type Similarity interface {
    // CosineSimilarity 余弦相似度
    CosineSimilarity(a, b []float64) float64
    
    // EuclideanDistance 欧氏距离
    EuclideanDistance(a, b []float64) float64
    
    // DotProduct 点积
    DotProduct(a, b []float64) float64
}
```

### 4.4 NLP 管道接口

```go
// algorithm/nlp/pipeline.go

package nlp

// Pipeline NLP处理管道
type Pipeline struct {
    tokenizer  Tokenizer
    ner        NERExtractor
    relation   RelationExtractor
    summarizer Summarizer
}

// Process 处理文本
func (p *Pipeline) Process(ctx context.Context, text string, opts *ProcessOptions) (*ProcessResult, error) {
    result := &ProcessResult{}
    
    // 1. 分词
    if opts.Tokenize {
        tokens, err := p.tokenizer.Tokenize(ctx, text)
        if err != nil {
            return nil, err
        }
        result.Tokens = tokens
    }
    
    // 2. NER
    if opts.ExtractEntities {
        entities, err := p.ner.Extract(ctx, text)
        if err != nil {
            return nil, err
        }
        result.Entities = entities
    }
    
    // 3. 关系抽取
    if opts.ExtractRelations && len(result.Entities) > 1 {
        relations, err := p.relation.Extract(ctx, text, result.Entities)
        if err != nil {
            return nil, err
        }
        result.Relations = relations
    }
    
    // 4. 摘要
    if opts.Summarize {
        summary, err := p.summarizer.Summarize(ctx, text, opts.SummaryMaxLength)
        if err != nil {
            return nil, err
        }
        result.Summary = summary
    }
    
    return result, nil
}

// Tokenizer 分词器接口
type Tokenizer interface {
    Tokenize(ctx context.Context, text string) ([]*Token, error)
}

// NERExtractor NER提取器接口
type NERExtractor interface {
    Extract(ctx context.Context, text string) ([]*Entity, error)
}

// RelationExtractor 关系提取器接口
type RelationExtractor interface {
    Extract(ctx context.Context, text string, entities []*Entity) ([]*Relation, error)
}

// Summarizer 摘要生成器接口
type Summarizer interface {
    Summarize(ctx context.Context, text string, maxLength int) (string, error)
}
```

### 4.5 索引管理器接口

```go
// algorithm/index/manager.go

package index

// Manager 索引管理器
type Manager struct {
    inverted *InvertedIndex
    vector   *VectorIndex
    graph    *GraphIndex
    
    hybrid   *HybridSearcher
}

// Index 索引文档
func (m *Manager) Index(ctx context.Context, doc *Document) error {
    var errs []error
    
    // 并行索引
    var wg sync.WaitGroup
    wg.Add(3)
    
    go func() {
        defer wg.Done()
        if err := m.inverted.Index(ctx, doc.ToInvertedDoc()); err != nil {
            errs = append(errs, err)
        }
    }()
    
    go func() {
        defer wg.Done()
        if err := m.vector.Index(ctx, doc.ID, doc.Vector); err != nil {
            errs = append(errs, err)
        }
    }()
    
    go func() {
        defer wg.Done()
        if len(doc.Entities) > 0 {
            if err := m.graph.IndexEntities(ctx, doc.Entities, doc.Relations); err != nil {
                errs = append(errs, err)
            }
        }
    }()
    
    wg.Wait()
    
    if len(errs) > 0 {
        return fmt.Errorf("index errors: %v", errs)
    }
    return nil
}

// Search 混合搜索
func (m *Manager) Search(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
    return m.hybrid.Search(ctx, query)
}

// SearchQuery 搜索查询
type SearchQuery struct {
    Text       string    `json:"text"`
    Vector     []float64 `json:"vector,omitempty"`
    Filters    *Filters  `json:"filters,omitempty"`
    TopK       int       `json:"top_k"`
    UseVector  bool      `json:"use_vector"`
    UseKeyword bool      `json:"use_keyword"`
    UseGraph   bool      `json:"use_graph"`
}
```

## 5. 工具层使用示例

### 5.1 邮件工具使用算法层

```go
// tools/email/email.go

package email

import (
    "github.com/vdocstool/algorithm"
)

type EmailTool struct {
    alg *algorithm.Algorithm
    // ... 其他字段
}

// SearchEmails 搜索邮件
func (t *EmailTool) SearchEmails(ctx context.Context, query string) ([]*Email, error) {
    // 1. 使用算法层解析意图
    intentResult, err := t.alg.Intent.Recognize(ctx, &intent.Input{
        Text:   query,
        Domain: "email",
    })
    if err != nil {
        return nil, err
    }
    
    // 2. 根据意图构建搜索条件
    searchCriteria := t.buildSearchCriteria(intentResult)
    
    // 3. 使用算法层向量化查询
    queryVector, err := t.alg.Embedding.Embed(ctx, query)
    if err != nil {
        // 降级为关键词搜索
        queryVector = nil
    }
    
    // 4. 使用算法层混合搜索
    results, err := t.alg.Index.Search(ctx, &index.SearchQuery{
        Text:       query,
        Vector:     queryVector,
        Filters:    searchCriteria,
        TopK:       20,
        UseVector:  queryVector != nil,
        UseKeyword: true,
    })
    if err != nil {
        return nil, err
    }
    
    // 5. 获取邮件详情
    return t.fetchEmails(ctx, results.Hits)
}
```

### 5.2 记忆工具使用算法层

```go
// tools/memory/memory.go

package memory

import (
    "github.com/vdocstool/algorithm"
)

type MemoryTool struct {
    alg *algorithm.Algorithm
    // ... 其他字段
}

// RetrieveContext 检索上下文
func (t *MemoryTool) RetrieveContext(ctx context.Context, query string, recent []*Message) ([]*ContextFragment, error) {
    // 1. 使用算法层解析是否需要历史
    intentResult, err := t.alg.Intent.Recognize(ctx, &intent.Input{
        Text:    query,
        Domain:  "memory",
        Context: recent,
    })
    if err != nil {
        return nil, err
    }
    
    // 2. 如果不需要历史，返回空
    if !intentResult.Intent.Slots["needs_history"] == "false" {
        return nil, nil
    }
    
    // 3. 使用 NLP 提取关键实体
    nlpResult, err := t.alg.NLP.Process(ctx, query, &nlp.ProcessOptions{
        ExtractEntities: true,
    })
    if err != nil {
        return nil, err
    }
    
    // 4. 向量化查询
    queryVector, err := t.alg.Embedding.Embed(ctx, query)
    if err != nil {
        return nil, err
    }
    
    // 5. 混合检索
    results, err := t.alg.Index.Search(ctx, &index.SearchQuery{
        Text:       query,
        Vector:     queryVector,
        TopK:       10,
        UseVector:  true,
        UseKeyword: true,
        UseGraph:   len(nlpResult.Entities) > 0,
    })
    if err != nil {
        return nil, err
    }
    
    // 6. 组装上下文
    return t.assembleContext(ctx, results)
}
```

## 6. 配置示例

```yaml
# config/algorithm.yaml

algorithm:
  # 意图引擎配置
  intent:
    strategy: cascade  # cascade | voting | adaptive
    default_domain: general
    
    # 规则引擎
    rule:
      enabled: true
      patterns_dir: config/intent_patterns
    
    # ML引擎  
    ml:
      enabled: true
      model_type: fasttext
      model_path: models/intent_classifier.bin
      confidence_threshold: 0.85
    
    # LLM引擎
    llm:
      enabled: true
      provider: openai
      model: gpt-4o-mini
      temperature: 0.1
      max_tokens: 500
      timeout: 10s
  
  # 向量化引擎配置
  embedding:
    provider: openai  # openai | bge | sentence-transformer
    model: text-embedding-3-small
    dimension: 1536
    batch_size: 100
    cache:
      enabled: true
      ttl: 24h
      max_size: 10000
  
  # NLP管道配置
  nlp:
    tokenizer: jieba  # jieba | whitespace | bert
    ner:
      provider: llm  # llm | hanlp | spacy
      model: gpt-4o-mini
    relation:
      enabled: true
      provider: llm
    summarizer:
      provider: llm
      max_length: 200
  
  # 索引配置
  index:
    inverted:
      engine: bleve
      path: data/index/inverted
      analyzer: jieba
    
    vector:
      engine: milvus
      address: localhost:19530
      collection: vdocs_vectors
      index_type: HNSW
      metric_type: L2
    
    graph:
      engine: neo4j
      uri: bolt://localhost:7687
      database: vdocs
```

## 7. 监控与可观测性

```go
// algorithm/metrics.go

// Metrics 算法层指标
type Metrics struct {
    // 意图识别指标
    IntentRequests    *prometheus.CounterVec
    IntentLatency     *prometheus.HistogramVec
    IntentConfidence  *prometheus.HistogramVec
    IntentFallbacks   *prometheus.CounterVec
    
    // 向量化指标
    EmbeddingRequests  *prometheus.CounterVec
    EmbeddingLatency   *prometheus.HistogramVec
    EmbeddingCacheHits *prometheus.CounterVec
    
    // 索引指标
    IndexRequests     *prometheus.CounterVec
    IndexLatency      *prometheus.HistogramVec
    SearchRecall      *prometheus.GaugeVec
}

// RecordIntentRecognition 记录意图识别
func (m *Metrics) RecordIntentRecognition(domain, source string, latency time.Duration, confidence float64) {
    m.IntentRequests.WithLabelValues(domain, source).Inc()
    m.IntentLatency.WithLabelValues(domain, source).Observe(latency.Seconds())
    m.IntentConfidence.WithLabelValues(domain, source).Observe(confidence)
}
```

## 8. 错误处理与降级

```go
// algorithm/intent/engine.go

// Recognize 带降级的意图识别
func (e *Engine) Recognize(ctx context.Context, input *Input) (*Result, error) {
    var lastErr error
    
    // 尝试主引擎
    for _, recognizer := range e.recognizers {
        result, err := recognizer.Recognize(ctx, input)
        if err == nil && result.Confidence >= e.config.ConfidenceThreshold {
            return result, nil
        }
        lastErr = err
    }
    
    // 降级到规则引擎
    if e.ruleEngine != nil {
        result, err := e.ruleEngine.Recognize(ctx, input)
        if err == nil {
            result.Source = "rule_fallback"
            e.metrics.IntentFallbacks.WithLabelValues(input.Domain).Inc()
            return result, nil
        }
    }
    
    // 兜底：返回未知意图
    return &Result{
        Intent: &Intent{
            Name:       "unknown",
            Confidence: 0,
        },
        Source: "fallback",
    }, lastErr
}
```
