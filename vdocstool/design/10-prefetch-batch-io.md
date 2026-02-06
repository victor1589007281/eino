# 预读取与批量 IO 优化设计文档

## 1. 概述

### 1.1 背景
当前 Memory 系统在处理检索请求时存在以下 IO 效率问题：
- 多层存储的顺序访问导致延迟累加
- 相关数据未预加载，热点数据重复读取
- 单条记录逐一处理，未利用批量 IO 优势
- 向量搜索与数据获取分离，产生额外往返

### 1.2 目标
通过引入预读取和批量 IO 策略，提升整体 IO 效率：
- 减少总体延迟 30-50%
- 提高吞吐量 2-3 倍
- 降低存储后端压力

## 2. 优化策略总览

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        IO Optimization Strategies                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    1. Prefetch (预读取)                               │   │
│  │                                                                       │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                  │   │
│  │  │ Session     │  │ Topic       │  │ Related     │                  │   │
│  │  │ Prefetch    │  │ Prefetch    │  │ Entity      │                  │   │
│  │  │ (会话预加载)│  │ (主题预加载)│  │ Prefetch    │                  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘                  │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    2. Batch IO (批量IO)                               │   │
│  │                                                                       │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                  │   │
│  │  │ Batch Read  │  │ Batch Write │  │ Batch       │                  │   │
│  │  │ (批量读取)  │  │ (批量写入)  │  │ Vector Ops  │                  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘                  │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    3. Pipeline (流水线)                               │   │
│  │                                                                       │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                  │   │
│  │  │ Parallel    │  │ Async       │  │ Streaming   │                  │   │
│  │  │ Fetch       │  │ Processing  │  │ Results     │                  │   │
│  │  │ (并行获取)  │  │ (异步处理)  │  │ (流式返回)  │                  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘                  │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    4. Caching (缓存层)                                │   │
│  │                                                                       │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                  │   │
│  │  │ Hot Data    │  │ Query       │  │ Embedding   │                  │   │
│  │  │ Cache       │  │ Cache       │  │ Cache       │                  │   │
│  │  │ (热数据缓存)│  │ (查询缓存)  │  │ (向量缓存)  │                  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘                  │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

## 3. 预读取策略

### 3.1 会话预读取 (Session Prefetch)

```go
// SessionPrefetcher 会话预读取器
type SessionPrefetcher struct {
    l1Cache    *L1WorkingMemory
    l2Storage  *L2ShortTermMemory
    
    // 预读取配置
    config     PrefetchConfig
    
    // 预读取缓存
    cache      *lru.Cache
    
    // 统计
    stats      PrefetchStats
}

// PrefetchConfig 预读取配置
type PrefetchConfig struct {
    // 会话预读取
    SessionLookAhead    int           // 预读取最近 N 个会话
    TopicLookAhead      int           // 每个会话预读取 N 个主题
    
    // 时间窗口
    TimeWindowMinutes   int           // 预读取最近 N 分钟的数据
    
    // 触发条件
    TriggerOnAccess     bool          // 访问时触发预读取相关数据
    TriggerOnIdle       bool          // 空闲时触发后台预读取
    IdleThresholdMs     int           // 空闲阈值
    
    // 资源限制
    MaxConcurrent       int           // 最大并发预读取数
    MaxCacheSize        int           // 最大缓存大小
    CacheTTL            time.Duration // 缓存过期时间
}

// PrefetchOnAccess 访问时触发预读取
func (p *SessionPrefetcher) PrefetchOnAccess(ctx context.Context, sessionID string) {
    // 检查是否已缓存
    if p.cache.Contains(sessionID) {
        return
    }
    
    // 异步预读取
    go func() {
        // 1. 预读取当前会话的最近主题
        topics, _ := p.l2Storage.GetRecentTopics(ctx, sessionID, p.config.TopicLookAhead)
        
        // 2. 预读取相关主题的摘要和关键片段
        for _, topic := range topics {
            capsule, _ := p.l2Storage.GetCapsule(ctx, topic.CapsuleID)
            p.cache.Add(topic.CapsuleID, capsule)
        }
        
        // 3. 预读取关联实体
        entities := p.extractEntities(topics)
        for _, entity := range entities {
            relations, _ := p.l3Storage.QueryRelations(ctx, entity, 1)
            p.cache.Add("entity:"+entity, relations)
        }
        
        p.stats.PrefetchCount.Add(1)
    }()
}

// PrefetchResult 预读取结果
type PrefetchResult struct {
    Topics    []*TopicCapsule
    Entities  map[string][]*Relation
    CacheHit  bool
    LoadTime  time.Duration
}

// GetPrefetched 获取预读取的数据
func (p *SessionPrefetcher) GetPrefetched(sessionID string) (*PrefetchResult, bool) {
    if val, ok := p.cache.Get(sessionID); ok {
        p.stats.CacheHits.Add(1)
        return val.(*PrefetchResult), true
    }
    p.stats.CacheMisses.Add(1)
    return nil, false
}
```

### 3.2 智能预读取策略

```go
// SmartPrefetcher 智能预读取器
type SmartPrefetcher struct {
    // 访问模式分析
    accessPattern *AccessPatternAnalyzer
    
    // 预测模型
    predictor     *AccessPredictor
}

// AccessPatternAnalyzer 访问模式分析器
type AccessPatternAnalyzer struct {
    // 会话访问历史
    sessionHistory map[string]*SessionAccessHistory
    
    // 主题关联矩阵
    topicCooccurrence map[string]map[string]int
}

// SessionAccessHistory 会话访问历史
type SessionAccessHistory struct {
    SessionID       string
    RecentAccesses  []AccessRecord
    TopTopics       []string          // 最常访问的主题
    AccessPattern   string            // sequential, random, topic_based
    AvgInterval     time.Duration     // 平均访问间隔
}

// PredictNextAccess 预测下次可能访问的数据
func (p *SmartPrefetcher) PredictNextAccess(sessionID string, currentTopic string) []PrefetchHint {
    history := p.accessPattern.GetHistory(sessionID)
    
    hints := []PrefetchHint{}
    
    // 1. 基于共现关系预测
    cooccurred := p.accessPattern.GetCooccurredTopics(currentTopic)
    for topic, score := range cooccurred {
        if score > 0.3 {  // 共现概率 > 30%
            hints = append(hints, PrefetchHint{
                Type:       "topic",
                Target:     topic,
                Confidence: score,
            })
        }
    }
    
    // 2. 基于时间模式预测
    if history.AccessPattern == "sequential" {
        // 顺序访问模式，预读取后续主题
        nextTopics := p.predictor.PredictSequential(history)
        hints = append(hints, nextTopics...)
    }
    
    // 3. 基于实体关联预测
    entities := p.extractEntities(currentTopic)
    for _, entity := range entities {
        relatedTopics := p.accessPattern.GetEntityRelatedTopics(entity)
        hints = append(hints, relatedTopics...)
    }
    
    // 按置信度排序，取 Top-K
    sort.Slice(hints, func(i, j int) bool {
        return hints[i].Confidence > hints[j].Confidence
    })
    
    if len(hints) > 10 {
        hints = hints[:10]
    }
    
    return hints
}
```

## 4. 批量 IO 操作

### 4.1 批量读取

```go
// BatchReader 批量读取器
type BatchReader struct {
    l1Storage *L1WorkingMemory
    l2Storage *L2ShortTermMemory
    l3Storage *L3LongTermMemory
    
    // 批量配置
    maxBatchSize int
    timeout      time.Duration
}

// BatchReadRequest 批量读取请求
type BatchReadRequest struct {
    // L1 读取
    L1Keys []L1ReadKey
    
    // L2 读取
    L2Keys []L2ReadKey
    
    // L3 读取
    L3Keys []L3ReadKey
}

type L1ReadKey struct {
    SessionID string
    MessageID string
}

type L2ReadKey struct {
    CapsuleID string
    Fields    []string  // 选择性读取字段
}

type L3ReadKey struct {
    ArchiveID string
    ChunkIDs  []string
}

// BatchRead 批量读取
func (r *BatchReader) BatchRead(ctx context.Context, req *BatchReadRequest) (*BatchReadResult, error) {
    result := &BatchReadResult{
        L1Results: make(map[string]*Message),
        L2Results: make(map[string]*Capsule),
        L3Results: make(map[string]*ArchiveChunk),
    }
    
    // 使用 errgroup 并行读取各层
    g, ctx := errgroup.WithContext(ctx)
    
    // L1 批量读取
    if len(req.L1Keys) > 0 {
        g.Go(func() error {
            messages, err := r.batchReadL1(ctx, req.L1Keys)
            if err != nil {
                return err
            }
            for k, v := range messages {
                result.L1Results[k] = v
            }
            return nil
        })
    }
    
    // L2 批量读取
    if len(req.L2Keys) > 0 {
        g.Go(func() error {
            capsules, err := r.batchReadL2(ctx, req.L2Keys)
            if err != nil {
                return err
            }
            for k, v := range capsules {
                result.L2Results[k] = v
            }
            return nil
        })
    }
    
    // L3 批量读取
    if len(req.L3Keys) > 0 {
        g.Go(func() error {
            chunks, err := r.batchReadL3(ctx, req.L3Keys)
            if err != nil {
                return err
            }
            for k, v := range chunks {
                result.L3Results[k] = v
            }
            return nil
        })
    }
    
    if err := g.Wait(); err != nil {
        return nil, err
    }
    
    return result, nil
}

// batchReadL1 批量读取 L1 (Redis MGET)
func (r *BatchReader) batchReadL1(ctx context.Context, keys []L1ReadKey) (map[string]*Message, error) {
    // 构建 Redis MGET 命令
    redisKeys := make([]string, len(keys))
    for i, k := range keys {
        redisKeys[i] = fmt.Sprintf("memory:l1:%s:%s", k.SessionID, k.MessageID)
    }
    
    // 批量获取
    values, err := r.l1Storage.redis.MGet(ctx, redisKeys...).Result()
    if err != nil {
        return nil, err
    }
    
    // 解析结果
    result := make(map[string]*Message)
    for i, val := range values {
        if val != nil {
            var msg Message
            if err := json.Unmarshal([]byte(val.(string)), &msg); err == nil {
                result[keys[i].MessageID] = &msg
            }
        }
    }
    
    return result, nil
}

// batchReadL2 批量读取 L2 (Milvus + PostgreSQL)
func (r *BatchReader) batchReadL2(ctx context.Context, keys []L2ReadKey) (map[string]*Capsule, error) {
    // 从 PostgreSQL 批量获取元数据
    capsuleIDs := make([]string, len(keys))
    for i, k := range keys {
        capsuleIDs[i] = k.CapsuleID
    }
    
    // 使用 IN 查询
    query := `SELECT * FROM capsules WHERE id = ANY($1)`
    rows, err := r.l2Storage.db.QueryContext(ctx, query, pq.Array(capsuleIDs))
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    // 解析结果
    result := make(map[string]*Capsule)
    for rows.Next() {
        var capsule Capsule
        if err := rows.Scan(&capsule); err == nil {
            result[capsule.ID] = &capsule
        }
    }
    
    return result, nil
}
```

### 4.2 批量写入

```go
// BatchWriter 批量写入器
type BatchWriter struct {
    l1Storage *L1WorkingMemory
    l2Storage *L2ShortTermMemory
    
    // 写入缓冲
    buffer    *WriteBuffer
    
    // 配置
    maxBufferSize  int
    flushInterval  time.Duration
    
    // 控制
    stopCh    chan struct{}
}

// WriteBuffer 写入缓冲
type WriteBuffer struct {
    mu       sync.Mutex
    l1Buffer []*L1WriteItem
    l2Buffer []*L2WriteItem
}

// L1WriteItem L1 写入项
type L1WriteItem struct {
    SessionID string
    Message   *Message
    TTL       time.Duration
}

// BufferedWrite 缓冲写入 (非阻塞)
func (w *BatchWriter) BufferedWrite(item *L1WriteItem) {
    w.buffer.mu.Lock()
    w.buffer.l1Buffer = append(w.buffer.l1Buffer, item)
    bufferSize := len(w.buffer.l1Buffer)
    w.buffer.mu.Unlock()
    
    // 如果缓冲区满，触发刷新
    if bufferSize >= w.maxBufferSize {
        go w.Flush()
    }
}

// Flush 刷新缓冲区
func (w *BatchWriter) Flush() error {
    w.buffer.mu.Lock()
    l1Items := w.buffer.l1Buffer
    l2Items := w.buffer.l2Buffer
    w.buffer.l1Buffer = nil
    w.buffer.l2Buffer = nil
    w.buffer.mu.Unlock()
    
    if len(l1Items) == 0 && len(l2Items) == 0 {
        return nil
    }
    
    g := new(errgroup.Group)
    
    // 批量写入 L1 (Redis Pipeline)
    if len(l1Items) > 0 {
        g.Go(func() error {
            return w.batchWriteL1(l1Items)
        })
    }
    
    // 批量写入 L2
    if len(l2Items) > 0 {
        g.Go(func() error {
            return w.batchWriteL2(l2Items)
        })
    }
    
    return g.Wait()
}

// batchWriteL1 批量写入 L1 (Redis Pipeline)
func (w *BatchWriter) batchWriteL1(items []*L1WriteItem) error {
    pipe := w.l1Storage.redis.Pipeline()
    
    for _, item := range items {
        key := fmt.Sprintf("memory:l1:%s:%s", item.SessionID, item.Message.ID)
        data, _ := json.Marshal(item.Message)
        pipe.Set(context.Background(), key, data, item.TTL)
        
        // 添加到会话列表
        listKey := fmt.Sprintf("memory:l1:%s:messages", item.SessionID)
        pipe.RPush(context.Background(), listKey, item.Message.ID)
    }
    
    _, err := pipe.Exec(context.Background())
    return err
}

// StartFlushLoop 启动定时刷新
func (w *BatchWriter) StartFlushLoop() {
    ticker := time.NewTicker(w.flushInterval)
    go func() {
        for {
            select {
            case <-ticker.C:
                w.Flush()
            case <-w.stopCh:
                ticker.Stop()
                w.Flush()  // 最终刷新
                return
            }
        }
    }()
}
```

### 4.3 批量向量操作

```go
// BatchVectorOps 批量向量操作
type BatchVectorOps struct {
    milvus    *MilvusClient
    embedder  *EmbeddingService
    
    // 配置
    batchSize int
}

// BatchEmbed 批量生成向量
func (b *BatchVectorOps) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
    // 分批处理，避免单次请求过大
    results := make([][]float32, len(texts))
    
    for i := 0; i < len(texts); i += b.batchSize {
        end := i + b.batchSize
        if end > len(texts) {
            end = len(texts)
        }
        
        batch := texts[i:end]
        embeddings, err := b.embedder.BatchEmbed(ctx, batch)
        if err != nil {
            return nil, err
        }
        
        copy(results[i:end], embeddings)
    }
    
    return results, nil
}

// BatchSearch 批量向量搜索
func (b *BatchVectorOps) BatchSearch(ctx context.Context, queries [][]float32, topK int) ([][]SearchHit, error) {
    // Milvus 支持批量搜索
    searchParams := &milvus.SearchParams{
        CollectionName: "memory_vectors",
        Vectors:        queries,
        TopK:           topK,
        MetricType:     "IP",  // Inner Product
    }
    
    results, err := b.milvus.Search(ctx, searchParams)
    if err != nil {
        return nil, err
    }
    
    return results, nil
}

// BatchInsert 批量插入向量
func (b *BatchVectorOps) BatchInsert(ctx context.Context, items []*VectorItem) error {
    // 分批插入
    for i := 0; i < len(items); i += b.batchSize {
        end := i + b.batchSize
        if end > len(items) {
            end = len(items)
        }
        
        batch := items[i:end]
        
        // 准备 Milvus 插入数据
        ids := make([]string, len(batch))
        vectors := make([][]float32, len(batch))
        metadata := make([]string, len(batch))
        
        for j, item := range batch {
            ids[j] = item.ID
            vectors[j] = item.Vector
            metadata[j] = item.MetadataJSON
        }
        
        if err := b.milvus.Insert(ctx, "memory_vectors", ids, vectors, metadata); err != nil {
            return err
        }
    }
    
    return nil
}
```

## 5. 流水线处理

### 5.1 并行检索流水线

```go
// ParallelPipeline 并行检索流水线
type ParallelPipeline struct {
    l1Storage *L1WorkingMemory
    l2Storage *L2ShortTermMemory
    l3Storage *L3LongTermMemory
    
    prefetcher *SessionPrefetcher
    batchReader *BatchReader
}

// ParallelRetrieve 并行检索
func (p *ParallelPipeline) ParallelRetrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error) {
    // 创建结果聚合器
    aggregator := NewResultAggregator(req.TokenBudget)
    
    // 触发预读取
    p.prefetcher.PrefetchOnAccess(ctx, req.SessionID)
    
    g, ctx := errgroup.WithContext(ctx)
    
    // 1. L1 检索 (最快，优先)
    g.Go(func() error {
        results, err := p.l1Storage.Search(ctx, req.SessionID, req.Query)
        if err != nil {
            return nil  // L1 失败不阻塞其他层
        }
        aggregator.AddResults("L1", results, 1.0)  // L1 权重最高
        return nil
    })
    
    // 2. L2 向量检索 (并行)
    g.Go(func() error {
        results, err := p.l2Storage.VectorSearch(ctx, req.Query, 10)
        if err != nil {
            return nil
        }
        aggregator.AddResults("L2", results, 0.8)
        return nil
    })
    
    // 3. L2 实体图谱检索 (并行)
    g.Go(func() error {
        entities := extractEntities(req.Query)
        if len(entities) == 0 {
            return nil
        }
        results, err := p.l2Storage.EntitySearch(ctx, entities)
        if err != nil {
            return nil
        }
        aggregator.AddResults("L2-Entity", results, 0.6)
        return nil
    })
    
    // 4. 检查预读取缓存
    if prefetched, ok := p.prefetcher.GetPrefetched(req.SessionID); ok {
        aggregator.AddPrefetchedResults(prefetched)
    }
    
    // 等待所有检索完成
    g.Wait()
    
    // 5. 重排序和组装
    return aggregator.Finalize(ctx)
}

// ResultAggregator 结果聚合器
type ResultAggregator struct {
    mu          sync.Mutex
    results     []*ScoredResult
    tokenBudget int
    tokenUsed   int
}

// AddResults 添加检索结果
func (a *ResultAggregator) AddResults(source string, results []*SearchResult, weight float64) {
    a.mu.Lock()
    defer a.mu.Unlock()
    
    for _, r := range results {
        a.results = append(a.results, &ScoredResult{
            Source:   source,
            Result:   r,
            Score:    r.Score * weight,
            Tokens:   r.TokenCount,
        })
    }
}

// Finalize 最终处理
func (a *ResultAggregator) Finalize(ctx context.Context) (*RetrieveResponse, error) {
    a.mu.Lock()
    defer a.mu.Unlock()
    
    // 去重
    a.deduplicate()
    
    // 按得分排序
    sort.Slice(a.results, func(i, j int) bool {
        return a.results[i].Score > a.results[j].Score
    })
    
    // Token 预算裁剪
    finalResults := []*SearchResult{}
    for _, r := range a.results {
        if a.tokenUsed + r.Tokens > a.tokenBudget {
            break
        }
        finalResults = append(finalResults, r.Result)
        a.tokenUsed += r.Tokens
    }
    
    return &RetrieveResponse{
        Results:     finalResults,
        TotalTokens: a.tokenUsed,
    }, nil
}
```

### 5.2 异步处理模式

```go
// AsyncProcessor 异步处理器
type AsyncProcessor struct {
    // 任务队列
    taskQueue chan *Task
    
    // 工作池
    workerCount int
    workers     []*Worker
    
    // 结果回调
    callbacks   map[string]ResultCallback
}

// Task 任务
type Task struct {
    ID       string
    Type     TaskType
    Payload  interface{}
    Callback string
}

// StartWorkers 启动工作池
func (p *AsyncProcessor) StartWorkers() {
    for i := 0; i < p.workerCount; i++ {
        worker := NewWorker(i, p.taskQueue, p.callbacks)
        p.workers = append(p.workers, worker)
        go worker.Run()
    }
}

// SubmitAsync 异步提交任务
func (p *AsyncProcessor) SubmitAsync(task *Task) string {
    task.ID = uuid.New().String()
    p.taskQueue <- task
    return task.ID
}
```

## 6. 多级缓存

```go
// MultiLevelCache 多级缓存
type MultiLevelCache struct {
    // L1: 本地内存缓存 (最快)
    localCache *lru.Cache
    
    // L2: Redis 缓存 (共享)
    redisCache *redis.Client
    
    // 配置
    localTTL  time.Duration
    redisTTL  time.Duration
}

// Get 获取缓存 (先本地，后 Redis)
func (c *MultiLevelCache) Get(ctx context.Context, key string) (interface{}, bool) {
    // 1. 检查本地缓存
    if val, ok := c.localCache.Get(key); ok {
        return val, true
    }
    
    // 2. 检查 Redis 缓存
    data, err := c.redisCache.Get(ctx, key).Bytes()
    if err == nil {
        var val interface{}
        json.Unmarshal(data, &val)
        
        // 回填本地缓存
        c.localCache.Add(key, val)
        return val, true
    }
    
    return nil, false
}

// Set 设置缓存 (同时设置本地和 Redis)
func (c *MultiLevelCache) Set(ctx context.Context, key string, val interface{}) error {
    // 本地缓存
    c.localCache.Add(key, val)
    
    // Redis 缓存 (异步)
    go func() {
        data, _ := json.Marshal(val)
        c.redisCache.Set(ctx, key, data, c.redisTTL)
    }()
    
    return nil
}

// QueryCache 查询结果缓存
type QueryCache struct {
    cache      *MultiLevelCache
    hasher     *QueryHasher
}

// CacheKey 生成缓存键
func (c *QueryCache) CacheKey(sessionID, query string, opts *RetrieveOptions) string {
    // 基于查询内容和选项生成哈希
    h := sha256.New()
    h.Write([]byte(sessionID))
    h.Write([]byte(query))
    h.Write([]byte(fmt.Sprintf("%v", opts)))
    return fmt.Sprintf("query:%x", h.Sum(nil)[:16])
}
```

## 7. 配置

```json
{
    "io_optimization": {
        "prefetch": {
            "enabled": true,
            "session_look_ahead": 5,
            "topic_look_ahead": 3,
            "time_window_minutes": 30,
            "trigger_on_access": true,
            "trigger_on_idle": true,
            "idle_threshold_ms": 100,
            "max_concurrent": 10,
            "max_cache_size": 1000,
            "cache_ttl_seconds": 300
        },
        
        "batch_io": {
            "enabled": true,
            "max_batch_size": 100,
            "flush_interval_ms": 100,
            "parallel_reads": true,
            "parallel_writes": true
        },
        
        "pipeline": {
            "parallel_retrieve": true,
            "async_processing": true,
            "worker_count": 4,
            "task_queue_size": 1000
        },
        
        "cache": {
            "local_cache_size": 10000,
            "local_ttl_seconds": 60,
            "redis_ttl_seconds": 300,
            "query_cache_enabled": true,
            "embedding_cache_enabled": true
        }
    }
}
```

## 8. 性能指标目标

| 指标 | 当前 | 目标 | 优化手段 |
|------|------|------|----------|
| L1 延迟 | 5ms | 2ms | 本地缓存 |
| L2 延迟 | 50ms | 20ms | 批量读取 + 并行 |
| L3 延迟 | 200ms | 100ms | 预读取 + 压缩 |
| 总延迟 P50 | 100ms | 40ms | 流水线并行 |
| 总延迟 P95 | 300ms | 100ms | 预读取 + 缓存 |
| 吞吐量 | 100 QPS | 300 QPS | 批量 IO |
| 缓存命中率 | 30% | 70% | 多级缓存 |

## 9. 实现计划

1. **Phase 1**: 批量读写基础设施 (Redis Pipeline, PostgreSQL Batch)
2. **Phase 2**: 会话预读取 + 基础缓存
3. **Phase 3**: 并行检索流水线
4. **Phase 4**: 智能预读取 + 访问模式分析
5. **Phase 5**: 多级缓存 + 查询缓存
6. **Phase 6**: 性能测试 + 调优
