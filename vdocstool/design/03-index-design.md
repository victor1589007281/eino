# 索引设计与存储方案评估

## 1. 当前存储架构评估

### 1.1 现有索引情况

| 层级 | 存储介质 | 现有索引 | 问题 |
|------|---------|---------|------|
| **L1** | Redis | Hash/List | ✅ 满足需求（小数据量，全量扫描可接受） |
| **L2** | PostgreSQL + Milvus(TODO) | B-Tree索引 | ⚠️ 向量索引未实现，缺少全文检索 |
| **L3** | S3 + Neo4j | 图索引 | ⚠️ 缺少归档数据的快速检索能力 |

### 1.2 检索场景分析

```
┌─────────────────────────────────────────────────────────────────┐
│                        检索需求分析                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  精确匹配检索                     语义相似检索                   │
│  ├─ 会话ID查询                   ├─ "之前讨论的那个方案"         │
│  ├─ 时间范围查询                 ├─ "类似上次的代码问题"         │
│  └─ 邮件发件人查询               └─ "关于Redis的内容"            │
│                                                                 │
│  关键词检索                       关系检索                       │
│  ├─ "发票"、"会议"等             ├─ "张三相关的所有项目"         │
│  ├─ 邮件主题搜索                 ├─ "Python涉及的技术栈"         │
│  └─ 内容关键词匹配               └─ "最近讨论的所有技术"         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## 2. 索引必要性分析

### 2.1 **倒排索引** - 必要 ✅

**理由：**
1. **全文检索需求**：用户经常需要按关键词搜索邮件内容、历史对话
2. **PostgreSQL ILIKE 不够用**：
   - 大数据量时 ILIKE 性能极差（全表扫描）
   - 不支持分词、同义词、相关性排序
3. **多字段联合搜索**：主题+内容+附件名 联合搜索

**方案对比：**

| 方案 | 优点 | 缺点 | 推荐度 |
|------|------|------|--------|
| **Elasticsearch** | 功能强大、生态好、分布式 | 资源占用大 | ⭐⭐⭐⭐⭐ |
| **Bleve (Go原生)** | 纯Go、嵌入式、轻量 | 功能较少 | ⭐⭐⭐ |
| **MeiliSearch** | 简单易用、速度快 | 需额外服务 | ⭐⭐⭐ |
| **PostgreSQL FTS** | 无需额外组件 | 中文支持差、功能有限 | ⭐⭐ |

**选定方案：Elasticsearch（支持中文分词、聚合、高亮、分布式）**

### 2.2 **向量索引** - 必要 ✅

**理由：**
1. **语义检索是核心需求**：用户的自然语言查询需要语义匹配
2. **L2已规划Milvus**：设计中已包含，需要落地实现
3. **跨层检索**：L2、L3都需要向量检索能力

**向量索引类型对比：**

| 索引类型 | 特点 | 适用场景 |
|---------|------|---------|
| **IVF_FLAT** | 精确度高、速度中等 | 数据量<100万 |
| **IVF_PQ** | 内存占用小、速度快 | 数据量>100万 |
| **HNSW** | 速度极快、精确度高 | 实时检索、内存充足 |

**推荐：HNSW (L2) + IVF_PQ (L3)**

### 2.3 **图索引** - 已有 ✅

**L3已使用Neo4j**，图索引已覆盖：
- 实体关系网络
- 知识图谱查询
- 多跳关系检索

## 3. 索引架构设计

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Index Layer (索引层)                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                   Unified Index Manager                           │   │
│  │                   (统一索引管理器)                                 │   │
│  └─────────────────────────────┬────────────────────────────────────┘   │
│                                │                                        │
│          ┌─────────────────────┼─────────────────────┐                  │
│          ▼                     ▼                     ▼                  │
│  ┌───────────────┐     ┌───────────────┐     ┌───────────────┐         │
│  │ Inverted Index│     │ Vector Index  │     │ Graph Index   │         │
│  │ (倒排索引)     │     │ (向量索引)     │     │ (图索引)       │         │
│  ├───────────────┤     ├───────────────┤     ├───────────────┤         │
│  │               │     │               │     │               │         │
│  │  ┌─────────┐  │     │  ┌─────────┐  │     │  ┌─────────┐  │         │
│  │  │  Bleve  │  │     │  │ Milvus  │  │     │  │  Neo4j  │  │         │
│  │  │ (嵌入式) │  │     │  │(向量DB) │  │     │  │ (图DB)  │  │         │
│  │  └─────────┘  │     │  └─────────┘  │     │  └─────────┘  │         │
│  │               │     │               │     │               │         │
│  │  功能:        │     │  功能:        │     │  功能:        │         │
│  │  • 关键词搜索 │     │  • 语义搜索   │     │  • 关系查询   │         │
│  │  • 分词匹配   │     │  • 相似度召回 │     │  • 实体遍历   │         │
│  │  • 模糊搜索   │     │  • TopK检索   │     │  • 路径发现   │         │
│  │  • 高亮显示   │     │  • 范围过滤   │     │  • 社区发现   │         │
│  │               │     │               │     │               │         │
│  └───────────────┘     └───────────────┘     └───────────────┘         │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## 4. 各层索引配置

### 4.1 L1 层（工作记忆）- 无需额外索引

```go
// L1 数据量小（10-20条），Redis 原生结构足够
// 使用 SCAN + 内存过滤 即可满足需求
type L1Index struct {
    // 不需要额外索引
    // Redis List: 按时间顺序
    // Redis Hash: 按ID快速访问
}
```

### 4.2 L2 层（短期记忆）- 需要倒排+向量索引

```
┌─────────────────────────────────────────────────────────────────┐
│                    L2 存储 + 索引架构                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   PostgreSQL (主存储)         Milvus (向量索引)                 │
│   ┌───────────────────┐      ┌───────────────────┐             │
│   │ topic_capsules    │      │ capsule_vectors   │             │
│   ├───────────────────┤      ├───────────────────┤             │
│   │ id                │◄────►│ capsule_id        │             │
│   │ summary           │      │ summary_vector    │             │
│   │ key_fragments     │      │ fragment_vectors  │             │
│   │ entities (JSONB)  │      └───────────────────┘             │
│   │ created_at        │                                        │
│   └────────┬──────────┘      Bleve (全文索引)                  │
│            │                 ┌───────────────────┐             │
│            │                 │ capsule_index     │             │
│            └────────────────►├───────────────────┤             │
│                              │ id                │             │
│                              │ title (text)      │             │
│                              │ summary (text)    │             │
│                              │ fragments (text)  │             │
│                              │ entities (text)   │             │
│                              └───────────────────┘             │
│                                                                 │
│   索引同步策略:                                                 │
│   • 写入时: PG + Milvus + Bleve 同步写入（事务）               │
│   • 查询时: Bleve关键词 + Milvus语义 并行召回                  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 4.3 L3 层（长期记忆）- 需要向量+图索引

```
┌─────────────────────────────────────────────────────────────────┐
│                    L3 存储 + 索引架构                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   S3 (归档存储)              Neo4j (图索引)                    │
│   ┌───────────────────┐      ┌───────────────────┐             │
│   │ archive_packages/ │      │ Entities          │             │
│   │ ├─ capsule_001.gz │      │ ├─ Person         │             │
│   │ ├─ capsule_002.gz │      │ ├─ Project        │             │
│   │ └─ ...            │      │ ├─ Technology     │             │
│   └───────────────────┘      │ └─ Topic          │             │
│                              │                   │             │
│   Redis (元数据缓存)         │ Relations         │             │
│   ┌───────────────────┐      │ ├─ MENTIONS       │             │
│   │ archive_meta:*    │      │ ├─ RELATES_TO     │             │
│   │ • capsule_id      │      │ ├─ WORKED_ON      │             │
│   │ • s3_key          │      │ └─ DISCUSSED      │             │
│   │ • summary_vector  │      └───────────────────┘             │
│   └───────────────────┘                                        │
│                              Milvus (向量索引-稀疏)            │
│   适用场景:                  ┌───────────────────┐             │
│   • 归档数据按需加载         │ archive_vectors   │             │
│   • 图谱查询找关联           │ ├─ summary_vec    │             │
│   • 向量索引快速定位         │ └─ entity_vec     │             │
│                              └───────────────────┘             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## 5. 实现方案

### 5.1 倒排索引实现 (Elasticsearch)

```go
// ESIndex Elasticsearch 倒排索引
type ESIndex struct {
    client    *elasticsearch.Client
    indexName string
}

// NewESIndex 创建 ES 索引
func NewESIndex(addresses []string, indexName string) (*ESIndex, error) {
    cfg := elasticsearch.Config{
        Addresses: addresses,
    }
    
    client, err := elasticsearch.NewClient(cfg)
    if err != nil {
        return nil, fmt.Errorf("create es client: %w", err)
    }
    
    idx := &ESIndex{
        client:    client,
        indexName: indexName,
    }
    
    // 创建索引（如果不存在）
    if err := idx.ensureIndex(); err != nil {
        return nil, err
    }
    
    return idx, nil
}

// ensureIndex 确保索引存在
func (e *ESIndex) ensureIndex() error {
    // 检查索引是否存在
    res, err := e.client.Indices.Exists([]string{e.indexName})
    if err != nil {
        return err
    }
    defer res.Body.Close()
    
    if res.StatusCode == 200 {
        return nil // 索引已存在
    }
    
    // 创建索引，配置 IK 中文分词
    mapping := `{
        "settings": {
            "analysis": {
                "analyzer": {
                    "ik_smart_analyzer": {
                        "type": "custom",
                        "tokenizer": "ik_smart"
                    },
                    "ik_max_analyzer": {
                        "type": "custom",
                        "tokenizer": "ik_max_word"
                    }
                }
            }
        },
        "mappings": {
            "properties": {
                "id": {"type": "keyword"},
                "type": {"type": "keyword"},
                "title": {
                    "type": "text",
                    "analyzer": "ik_max_word",
                    "search_analyzer": "ik_smart"
                },
                "summary": {
                    "type": "text",
                    "analyzer": "ik_max_word",
                    "search_analyzer": "ik_smart"
                },
                "content": {
                    "type": "text",
                    "analyzer": "ik_max_word",
                    "search_analyzer": "ik_smart"
                },
                "keywords": {"type": "keyword"},
                "session_id": {"type": "keyword"},
                "created_at": {"type": "date"}
            }
        }
    }`
    
    res, err = e.client.Indices.Create(
        e.indexName,
        e.client.Indices.Create.WithBody(strings.NewReader(mapping)),
    )
    if err != nil {
        return err
    }
    defer res.Body.Close()
    
    return nil
}

// IndexDocument 索引文档
type IndexDocument struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    Title     string    `json:"title"`
    Summary   string    `json:"summary"`
    Content   string    `json:"content"`
    Keywords  []string  `json:"keywords"`
    SessionID string    `json:"session_id"`
    CreatedAt time.Time `json:"created_at"`
}

// Index 添加文档到索引
func (e *ESIndex) Index(ctx context.Context, doc *IndexDocument) error {
    data, err := json.Marshal(doc)
    if err != nil {
        return err
    }
    
    res, err := e.client.Index(
        e.indexName,
        bytes.NewReader(data),
        e.client.Index.WithDocumentID(doc.ID),
        e.client.Index.WithContext(ctx),
        e.client.Index.WithRefresh("true"),
    )
    if err != nil {
        return err
    }
    defer res.Body.Close()
    
    if res.IsError() {
        return fmt.Errorf("index error: %s", res.String())
    }
    
    return nil
}

// Search 搜索
func (e *ESIndex) Search(ctx context.Context, query string, limit int) ([]*SearchHit, error) {
    // 构建多字段查询
    searchQuery := map[string]interface{}{
        "query": map[string]interface{}{
            "multi_match": map[string]interface{}{
                "query":  query,
                "fields": []string{"title^3", "summary^2", "content", "keywords^2"},
                "type":   "best_fields",
            },
        },
        "highlight": map[string]interface{}{
            "fields": map[string]interface{}{
                "title":   map[string]interface{}{},
                "summary": map[string]interface{}{},
                "content": map[string]interface{}{},
            },
        },
        "size": limit,
    }
    
    data, _ := json.Marshal(searchQuery)
    
    res, err := e.client.Search(
        e.client.Search.WithContext(ctx),
        e.client.Search.WithIndex(e.indexName),
        e.client.Search.WithBody(bytes.NewReader(data)),
    )
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()
    
    // 解析响应
    var result struct {
        Hits struct {
            Hits []struct {
                ID        string                       `json:"_id"`
                Score     float64                      `json:"_score"`
                Source    IndexDocument                `json:"_source"`
                Highlight map[string][]string          `json:"highlight"`
            } `json:"hits"`
        } `json:"hits"`
    }
    
    if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    var hits []*SearchHit
    for _, h := range result.Hits.Hits {
        hits = append(hits, &SearchHit{
            ID:         h.ID,
            Score:      h.Score,
            Highlights: h.Highlight,
        })
    }
    
    return hits, nil
}
```

### 5.2 向量索引实现 (Milvus)

```go
// VectorIndex 向量索引
type VectorIndex struct {
    client     *milvus.Client
    collection string
    dimension  int
}

// CreateCollection 创建集合
func (v *VectorIndex) CreateCollection(ctx context.Context) error {
    schema := &entity.Schema{
        CollectionName: v.collection,
        Fields: []*entity.Field{
            {
                Name:       "id",
                DataType:   entity.FieldTypeVarChar,
                PrimaryKey: true,
                MaxLength:  64,
            },
            {
                Name:     "vector",
                DataType: entity.FieldTypeFloatVector,
                TypeParams: map[string]string{
                    "dim": fmt.Sprintf("%d", v.dimension),
                },
            },
            {
                Name:     "session_id",
                DataType: entity.FieldTypeVarChar,
                MaxLength: 64,
            },
            {
                Name:     "created_at",
                DataType: entity.FieldTypeInt64,
            },
        },
    }
    
    return v.client.CreateCollection(ctx, schema, 2) // 2 shards
}

// CreateIndex 创建HNSW索引
func (v *VectorIndex) CreateIndex(ctx context.Context) error {
    idx, err := entity.NewIndexHNSW(entity.L2, 16, 256) // M=16, efConstruction=256
    if err != nil {
        return err
    }
    return v.client.CreateIndex(ctx, v.collection, "vector", idx, false)
}

// Insert 插入向量
func (v *VectorIndex) Insert(ctx context.Context, id string, vector []float64, sessionID string) error {
    ids := []string{id}
    vectors := [][]float32{toFloat32(vector)}
    sessionIDs := []string{sessionID}
    timestamps := []int64{time.Now().Unix()}
    
    _, err := v.client.Insert(ctx, v.collection, "", ids, vectors, sessionIDs, timestamps)
    return err
}

// Search 向量搜索
func (v *VectorIndex) Search(ctx context.Context, vector []float64, topK int, filter string) ([]*VectorHit, error) {
    sp, _ := entity.NewIndexHNSWSearchParam(64) // ef=64
    
    results, err := v.client.Search(
        ctx,
        v.collection,
        nil,
        filter, // 例如: "session_id == 'xxx'"
        []string{"id", "session_id", "created_at"},
        []entity.Vector{entity.FloatVector(toFloat32(vector))},
        "vector",
        entity.L2,
        topK,
        sp,
    )
    if err != nil {
        return nil, err
    }
    
    var hits []*VectorHit
    for i := 0; i < results[0].ResultCount; i++ {
        hits = append(hits, &VectorHit{
            ID:        results[0].IDs.(*entity.ColumnVarChar).Data()[i],
            Distance:  results[0].Scores[i],
            SessionID: results[0].Fields[0].(*entity.ColumnVarChar).Data()[i],
        })
    }
    
    return hits, nil
}
```

### 5.3 混合检索实现

```go
// HybridSearcher 混合检索器
type HybridSearcher struct {
    inverted *InvertedIndex
    vector   *VectorIndex
    embedder EmbeddingEngine
}

// Search 混合检索
func (h *HybridSearcher) Search(ctx context.Context, query string, topK int) ([]*SearchResult, error) {
    var wg sync.WaitGroup
    var invertedHits []*SearchHit
    var vectorHits []*VectorHit
    var err1, err2 error
    
    // 并行执行两种检索
    wg.Add(2)
    
    // 1. 倒排检索
    go func() {
        defer wg.Done()
        invertedHits, err1 = h.inverted.Search(query, topK*2)
    }()
    
    // 2. 向量检索
    go func() {
        defer wg.Done()
        // 先向量化查询
        embedding, err := h.embedder.Embed(ctx, query)
        if err != nil {
            err2 = err
            return
        }
        vectorHits, err2 = h.vector.Search(ctx, embedding, topK*2, "")
    }()
    
    wg.Wait()
    
    // 处理错误
    if err1 != nil && err2 != nil {
        return nil, fmt.Errorf("both searches failed: %v, %v", err1, err2)
    }
    
    // 融合结果 (RRF - Reciprocal Rank Fusion)
    return h.fuseResults(invertedHits, vectorHits, topK), nil
}

// fuseResults RRF 融合
func (h *HybridSearcher) fuseResults(inverted []*SearchHit, vector []*VectorHit, topK int) []*SearchResult {
    const k = 60 // RRF 常数
    
    scores := make(map[string]float64)
    metadata := make(map[string]*SearchResult)
    
    // 倒排结果
    for i, hit := range inverted {
        scores[hit.ID] += 1.0 / float64(k+i+1)
        metadata[hit.ID] = &SearchResult{
            ID:         hit.ID,
            Highlights: hit.Highlights,
            Sources:    []string{"inverted"},
        }
    }
    
    // 向量结果
    for i, hit := range vector {
        scores[hit.ID] += 1.0 / float64(k+i+1)
        if m, ok := metadata[hit.ID]; ok {
            m.Sources = append(m.Sources, "vector")
            m.VectorDistance = hit.Distance
        } else {
            metadata[hit.ID] = &SearchResult{
                ID:             hit.ID,
                VectorDistance: hit.Distance,
                Sources:        []string{"vector"},
            }
        }
    }
    
    // 按分数排序
    var results []*SearchResult
    for id, score := range scores {
        r := metadata[id]
        r.FusedScore = score
        results = append(results, r)
    }
    
    sort.Slice(results, func(i, j int) bool {
        return results[i].FusedScore > results[j].FusedScore
    })
    
    if len(results) > topK {
        results = results[:topK]
    }
    
    return results
}
```

## 6. 性能优化建议

### 6.1 索引预热

```go
// WarmupIndex 索引预热
func (h *HybridSearcher) WarmupIndex(ctx context.Context) error {
    // 预热向量索引
    if err := h.vector.Load(ctx); err != nil {
        return err
    }
    
    // 预热倒排索引（读取到内存）
    _, _ = h.inverted.Search("*", 1)
    
    return nil
}
```

### 6.2 增量索引

```go
// IndexQueue 索引队列
type IndexQueue struct {
    queue  chan *IndexTask
    buffer []*IndexTask
    ticker *time.Ticker
}

// 批量索引（减少IO）
func (q *IndexQueue) processBatch() {
    for {
        select {
        case task := <-q.queue:
            q.buffer = append(q.buffer, task)
            if len(q.buffer) >= 100 { // 批量大小
                q.flushBatch()
            }
        case <-q.ticker.C:
            if len(q.buffer) > 0 {
                q.flushBatch()
            }
        }
    }
}
```

### 6.3 索引分片

```go
// 按时间分片索引
// 最近7天: 热索引（常驻内存）
// 7-30天: 温索引（按需加载）
// 30天+: 冷索引（归档存储）

type ShardedIndex struct {
    hot  *InvertedIndex // 最近7天
    warm *InvertedIndex // 7-30天
    cold *InvertedIndex // 30天+，延迟加载
}
```

## 7. 总结与建议

### 7.1 索引必要性总结

| 索引类型 | 必要性 | 理由 |
|---------|--------|------|
| **倒排索引** | ✅ 必要 | 关键词检索核心能力，PostgreSQL ILIKE 无法满足 |
| **向量索引** | ✅ 必要 | 语义检索是核心需求，设计中已规划需落地 |
| **图索引** | ✅ 已有 | Neo4j 已满足知识图谱需求 |

### 7.2 技术选型

| 组件 | 选定方案 | 理由 |
|------|---------|------|
| 倒排索引 | **Elasticsearch** | 功能强大、中文分词好、分布式、生态完善 |
| 向量索引 | **Milvus** | 设计已包含、功能完善、性能好 |
| 图数据库 | **Neo4j** | 已选用、功能强大 |

### 7.3 实施优先级

1. **P0 - 立即实现**：Milvus 向量索引落地（L2 层）
2. **P1 - 短期实现**：Elasticsearch 倒排索引（L2 层）
3. **P2 - 中期实现**：混合检索融合（RRF）、L3 向量索引
4. **P3 - 长期优化**：索引分片、预热、增量更新
