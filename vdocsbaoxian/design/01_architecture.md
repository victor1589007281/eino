# 保险专家Agent - 整体架构设计

## 1. 系统概述

保险专家Agent是一个基于eino框架开发的智能保险顾问系统，以保险相关的法律法规、保险产品条款、理赔案例等为依据，从专业角度解答用户的保险问题。系统采用多Agent协作架构，结合网页搜索、知识索引、负载并发等技术，提供深度的保险专业分析能力。

### 1.1 核心能力

| 能力 | 描述 | 应用场景 |
|------|------|----------|
| **法规知识库** | 构建保险法律法规知识体系 | 合规性检查、政策解读 |
| **产品分析** | 保险产品条款解析与对比 | 产品设计、条款分析 |
| **理赔案例** | 理赔案例检索与分析 | 理赔指导、风险评估 |
| **用户匹配** | 用户需求与产品匹配分析 | 个性化推荐 |
| **健康数据分析** | 疾病与健康行业数据分析 | 风险评估、产品定价 |
| **网页搜索** | 实时获取最新保险资讯 | 市场动态、政策更新 |

## 2. 整体架构图

```mermaid
graph TB
    subgraph "用户交互层"
        UI[**用户输入**]
        OUT_S[**总结输出<br/>Chat Summary**]
        OUT_D[**文档输出<br/>Markdown Doc**]
    end
    
    subgraph "主控层 - Supervisor Agent"
        MA[**MasterAgent<br/>保险专家主控**]
        IR[**IntentRecognizer<br/>意图识别器**]
        PL[**Planner<br/>规划器**]
    end
    
    subgraph "执行层 - Sub Agents"
        SA1[**LegalAgent<br/>法规分析Agent**]
        SA2[**ProductAgent<br/>产品分析Agent**]
        SA3[**ClaimAgent<br/>理赔分析Agent**]
        SA4[**MatchAgent<br/>用户匹配Agent**]
        SA5[**HealthAgent<br/>健康数据Agent**]
        SA6[**WebSearchAgent<br/>网页搜索Agent**]
        SA7[**DocumentAgent<br/>文档生成Agent**]
    end
    
    subgraph "工具层 - Tools"
        T1[**WebSearchTool<br/>网页搜索**]
        T2[**CrawlerTool<br/>爬虫工具**]
        T3[**IndexSearchTool<br/>索引搜索**]
        T4[**KnowledgeRetriever<br/>知识检索**]
        T5[**StatsTool<br/>统计工具**]
        T6[**VerifyTool<br/>求证工具**]
    end
    
    subgraph "索引层 - Index System"
        IDX1[**InvertedIndex<br/>倒排索引**]
        IDX2[**DocumentSummary<br/>文档摘要**]
        IDX3[**KnowledgeGraph<br/>知识图谱**]
        IDX4[**VectorIndex<br/>向量索引**]
    end
    
    subgraph "缓存层 - Cache System"
        C1[**L1 Memory<br/>内存缓存**]
        C2[**L2 Local<br/>本地缓存**]
        C3[**L3 Redis<br/>Redis缓存**]
    end
    
    subgraph "存储层 - Storage"
        DB1[**SQLite<br/>内嵌存储**]
        DB2[**MySQL<br/>外部存储**]
        DB3[**BoltDB<br/>图存储**]
        DB4[**File Cache<br/>文件缓存**]
    end
    
    subgraph "数据源层"
        DS1[**法律法规<br/>知识库**]
        DS2[**产品条款<br/>数据库**]
        DS3[**理赔案例<br/>数据库**]
        DS4[**健康数据<br/>报告**]
    end
    
    UI --> MA
    MA --> IR
    IR --> PL
    PL --> SA1
    PL --> SA2
    PL --> SA3
    PL --> SA4
    PL --> SA5
    PL --> SA6
    PL --> SA7
    
    SA1 --> T3
    SA1 --> T4
    SA1 --> T6
    SA2 --> T3
    SA2 --> T4
    SA3 --> T3
    SA3 --> T4
    SA4 --> T5
    SA5 --> T5
    SA6 --> T1
    SA6 --> T2
    SA7 --> OUT_S
    SA7 --> OUT_D
    
    T3 --> IDX1
    T3 --> IDX2
    T4 --> IDX3
    T4 --> IDX4
    
    IDX1 --> C1
    IDX2 --> C1
    IDX3 --> C2
    IDX4 --> C2
    
    C1 --> C2
    C2 --> C3
    
    C1 --> DB1
    C2 --> DB1
    C3 --> DB2
    IDX3 --> DB3
    
    DB1 --> DS1
    DB1 --> DS2
    DB1 --> DS3
    DB1 --> DS4
    
    style MA fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style IR fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style PL fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style SA1 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style SA2 fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style SA3 fill:#fffacd,stroke:#333,stroke-width:2px,color:#000
    style SA4 fill:#ffe1f5,stroke:#333,stroke-width:2px,color:#000
    style SA5 fill:#e8e1ff,stroke:#333,stroke-width:2px,color:#000
    style SA6 fill:#d4edda,stroke:#333,stroke-width:2px,color:#000
    style SA7 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style IDX1 fill:#d4edda,stroke:#333,stroke-width:2px,color:#000
    style IDX2 fill:#d4edda,stroke:#333,stroke-width:2px,color:#000
    style IDX3 fill:#d4edda,stroke:#333,stroke-width:2px,color:#000
    style IDX4 fill:#d4edda,stroke:#333,stroke-width:2px,color:#000
```

## 3. 核心组件说明

### 3.1 主控层 (Supervisor Agent)

| 组件 | 功能 | 技术实现 |
|------|------|----------|
| **MasterAgent** | 协调所有子Agent，管理对话上下文 | `adk.ChatModelAgent` + `supervisor.New` |
| **IntentRecognizer** | 识别用户意图，分类问题类型 | LLM + 规则引擎 |
| **Planner** | 制定执行计划，分配任务 | `adk.prebuilt.deep` 任务规划 |

### 3.2 执行层 (Sub Agents)

| Agent | 职责 | 并发支持 | 依赖数据源 |
|-------|------|----------|-----------|
| **LegalAgent** | 法律法规分析、合规检查 | ✅ 并发检索 | 法律法规知识库 |
| **ProductAgent** | 产品条款分析、条款对比 | ✅ 并发分析 | 产品条款数据库 |
| **ClaimAgent** | 理赔案例分析、流程指导 | ✅ 并发查询 | 理赔案例数据库 |
| **MatchAgent** | 用户需求匹配、个性化推荐 | ✅ 并发计算 | 用户画像、产品库 |
| **HealthAgent** | 健康数据分析、风险评估 | ✅ 并发统计 | 健康数据报告 |
| **WebSearchAgent** | 网页搜索、信息爬取 | ✅ 并发爬取 | 互联网 |
| **DocumentAgent** | 文档生成、图表渲染 | - | - |

### 3.3 工具层 (Tools)

```go
// 工具接口定义
type Tool interface {
    Info(ctx context.Context) (*schema.ToolInfo, error)
    InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error)
}
```

| 工具 | 功能 | 性能优化 |
|------|------|----------|
| **WebSearchTool** | 网页搜索、实时资讯获取 | 并发搜索+缓存 |
| **CrawlerTool** | 网页爬取、内容提取 | 智能限流+去重 |
| **IndexSearchTool** | 索引查询、快速检索 | 倒排索引+缓存 |
| **KnowledgeRetriever** | 知识检索、语义搜索 | 向量索引+知识图谱 |
| **StatsTool** | 统计分析、数据汇总 | 增量计算 |
| **VerifyTool** | 信息求证、来源验证 | 多源校验 |

## 4. 数据流架构

```mermaid
sequenceDiagram
    participant U as "用户"
    participant M as "MasterAgent"
    participant I as "IntentRecognizer"
    participant P as "Planner"
    participant S as "SubAgents"
    participant T as "Tools"
    participant C as "Cache"
    participant D as "Database"
    
    U->>M: **1. 提交保险问题**
    M->>I: **2. 意图识别**
    I->>I: **3. 分类问题类型<br/>- 法规咨询类<br/>- 产品分析类<br/>- 理赔指导类<br/>- 匹配推荐类**
    I-->>M: **4. 返回意图结果**
    
    M->>P: **5. 制定执行计划**
    P->>P: **6. 任务分解<br/>- 确定需要的SubAgent<br/>- 确定执行顺序<br/>- 标注并发任务**
    P-->>M: **7. 返回执行计划**
    
    par **并行执行任务**
        M->>S: **8a. 调度LegalAgent**
        S->>C: **9a. 查询缓存**
        C-->>S: **10a. 缓存命中/未命中**
        S->>T: **11a. 调用IndexSearchTool**
        T->>D: **12a. 查询索引**
        D-->>T: **13a. 返回结果**
        T-->>S: **14a. 返回检索结果**
        S-->>M: **15a. 返回法规分析结果**
    and
        M->>S: **8b. 调度ProductAgent**
        S->>C: **9b. 查询缓存**
        S->>T: **11b. 调用KnowledgeRetriever**
        T->>D: **12b. 查询知识图谱**
        D-->>T: **13b. 返回产品信息**
        T-->>S: **14b. 返回产品信息**
        S-->>M: **15b. 返回产品分析结果**
    end
    
    M->>S: **16. 调度VerifyTool求证**
    S->>S: **17. 交叉验证信息**
    S-->>M: **18. 返回验证结果**
    
    M->>M: **19. 汇总分析结果**
    M->>S: **20. 调度DocumentAgent**
    S-->>M: **21. 生成输出文档**
    M-->>U: **22. 返回最终结果**
    
    rect rgb(255, 250, 205)
    Note over U,D: **关键：并行执行SubAgent任务，信息必须经过法规和案例求证**
    end
```

## 5. 并发执行架构

### 5.1 并发模型

```mermaid
graph LR
    subgraph "任务调度器"
        SCH[**Scheduler**]
    end
    
    subgraph "Worker Pool"
        W1[**Worker-1<br/>法规分析**]
        W2[**Worker-2<br/>产品分析**]
        W3[**Worker-3<br/>理赔分析**]
        W4[**Worker-4<br/>网页搜索**]
        W5[**Worker-N<br/>其他任务**]
    end
    
    subgraph "任务队列"
        Q[**TaskQueue**]
    end
    
    subgraph "结果聚合"
        AGG[**Aggregator<br/>结果验证**]
    end
    
    SCH --> Q
    Q --> W1
    Q --> W2
    Q --> W3
    Q --> W4
    Q --> W5
    W1 --> AGG
    W2 --> AGG
    W3 --> AGG
    W4 --> AGG
    W5 --> AGG
    
    style SCH fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style AGG fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style W1 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style W2 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style W3 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style W4 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style W5 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
```

### 5.2 并发策略

| 场景 | 并发策略 | 实现方式 |
|------|----------|----------|
| 多知识库检索 | Fan-out/Fan-in | `errgroup.Group` |
| 多产品对比分析 | Pipeline | Channel + Goroutine |
| 知识图谱遍历 | BFS/DFS并行 | WorkerPool |
| 网页批量爬取 | MapReduce | 分片处理 |
| 信息交叉验证 | 并行验证 | `sync.WaitGroup` |

## 6. 缓存架构

### 6.1 多级缓存设计

```mermaid
graph TB
    subgraph "L1 - 内存缓存"
        L1[**Memory LRU<br/>热点数据<br/>TTL: 5min**]
    end
    
    subgraph "L2 - 本地缓存"
        L2[**Local Cache<br/>会话数据<br/>TTL: 30min**]
    end
    
    subgraph "L3 - 分布式缓存"
        L3[**Redis/SQLite<br/>持久化数据<br/>TTL: 24h**]
    end
    
    subgraph "存储层"
        DB[**Database<br/>永久存储**]
    end
    
    L1 --> L2
    L2 --> L3
    L3 --> DB
    
    style L1 fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style L2 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style L3 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style DB fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
```

### 6.2 缓存策略

| 缓存层 | 数据类型 | TTL | 淘汰策略 |
|--------|----------|-----|----------|
| L1 Memory | 热点查询、常用法规 | 5min | LRU |
| L2 Local | 会话上下文、用户画像 | 30min | LFU |
| L3 Redis | 索引结果、产品信息 | 24h | TTL |
| Database | 原始数据、历史记录 | 永久 | - |

### 6.3 记忆系统

```go
// 记忆系统设计
type MemorySystem struct {
    ShortTerm   *ShortTermMemory   // 短期记忆：当前会话上下文
    LongTerm    *LongTermMemory    // 长期记忆：用户偏好、历史查询
    Episodic    *EpisodicMemory    // 情景记忆：历史问答对
    Semantic    *SemanticMemory    // 语义记忆：知识图谱缓存
}
```

## 7. 输出格式设计

### 7.1 总结输出 (Summary Mode)

```
【问题】: 用户问题简述
【结论】: 1-2句核心结论
【法律依据】: 相关法规条款
【产品建议】: 推荐产品及理由
【风险提示】: 潜在风险说明
【参考来源】: 信息来源链接
```

### 7.2 文档输出 (Document Mode)

```markdown
# 主题标题

## 1. 问题概述
简要说明用户问题...

## 2. 法律法规分析
### 2.1 相关法规
[法规条款及解读]

### 2.2 合规性分析
[合规检查结果]

## 3. 产品分析
### 3.1 产品对比表
[产品对比mermaid表格]

### 3.2 推荐产品
[推荐理由及分析]

## 4. 理赔案例参考
[相关案例及分析]

## 5. 风险评估
[风险分析图表]

## 6. 结论与建议
结论及专业建议...

## 7. 参考来源
- 法规来源
- 产品来源
- 案例来源
```

## 8. 信息求证机制

### 8.1 求证流程

```mermaid
graph TD
    INPUT[**输入信息**] --> V1{**来源验证**}
    V1 -->|有来源| V2{**时效检查**}
    V1 -->|无来源| SEARCH[**搜索验证**]
    
    V2 -->|时效内| V3{**法规核对**}
    V2 -->|已过期| UPDATE[**更新信息**]
    
    V3 -->|符合法规| V4{**案例验证**}
    V3 -->|不符合| REJECT[**标记不可信**]
    
    V4 -->|有案例支持| ACCEPT[**标记可信**]
    V4 -->|无案例| PARTIAL[**标记待验证**]
    
    SEARCH --> V2
    UPDATE --> V3
    
    style INPUT fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style ACCEPT fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style REJECT fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style PARTIAL fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
```

### 8.2 时效标注

| 信息类型 | 时效周期 | 更新策略 |
|----------|----------|----------|
| 法律法规 | 发布日期起有效 | 政策发布时更新 |
| 产品条款 | 产品有效期内 | 产品更新时同步 |
| 理赔案例 | 永久有效 | 定期补充新案例 |
| 健康数据 | 年度/季度 | 定期更新 |
| 网页信息 | 7天 | 定期刷新 |

## 9. Agent交互架构

### 9.1 交互协议

```mermaid
graph LR
    subgraph "本地Agent"
        LOCAL[**保险专家Agent**]
    end
    
    subgraph "外部Agent - MCP"
        MCP1[**法规Agent<br/>MCP Server**]
        MCP2[**行业数据Agent<br/>MCP Server**]
    end
    
    subgraph "外部Agent - A2A"
        A2A1[**医疗知识Agent<br/>A2A Protocol**]
        A2A2[**金融分析Agent<br/>A2A Protocol**]
    end
    
    subgraph "API服务"
        REST[**REST API**]
        WS[**WebSocket**]
    end
    
    LOCAL <--> MCP1
    LOCAL <--> MCP2
    LOCAL <--> A2A1
    LOCAL <--> A2A2
    LOCAL --> REST
    LOCAL --> WS
    
    style LOCAL fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style MCP1 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style MCP2 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style A2A1 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style A2A2 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
```

## 10. 扩展性设计

### 10.1 插件机制

- **新Agent注册**: 通过 `adk.SetSubAgents` 动态添加
- **新Tool注册**: 通过 `ToolsConfig.Tools` 扩展
- **新Skill注册**: 通过 `skill.Backend` 接口扩展
- **新数据源**: 通过 `DataSource` 接口扩展

### 10.2 配置化

```yaml
insurance_expert:
  # 数据源配置
  data_sources:
    legal_db: "/path/to/legal"
    product_db: "/path/to/products"
    claim_db: "/path/to/claims"
    
  # 索引配置
  index:
    path: "/path/to/index"
    rebuild_on_start: false
    
  # Agent配置
  agent:
    max_concurrent: 10
    output_mode: "document"  # summary | document
    verify_mode: true        # 启用求证
    
  # 模型路由配置
  model_routing:
    simple_query: "glm-4-flash"
    complex_analysis: "gpt-4-turbo"
    reasoning: "claude-3-opus"
```

## 11. 部署架构

```mermaid
graph TB
    subgraph "客户端"
        CLI[**CLI Interface**]
        API[**REST API**]
        WS[**WebSocket**]
    end
    
    subgraph "负载均衡"
        LB[**Nginx/Ingress**]
    end
    
    subgraph "服务层 - K8S"
        SVC1[**Agent Service<br/>Pod-1**]
        SVC2[**Agent Service<br/>Pod-2**]
        SVC3[**Agent Service<br/>Pod-N**]
    end
    
    subgraph "缓存层"
        REDIS[**Redis Cluster**]
    end
    
    subgraph "存储层"
        MYSQL[**MySQL<br/>主从集群**]
        SQLITE[**SQLite<br/>本地索引**]
    end
    
    subgraph "配置中心"
        CONFIG[**ConfigMap<br/>Secrets**]
    end
    
    CLI --> LB
    API --> LB
    WS --> LB
    LB --> SVC1
    LB --> SVC2
    LB --> SVC3
    SVC1 --> REDIS
    SVC2 --> REDIS
    SVC3 --> REDIS
    SVC1 --> MYSQL
    SVC2 --> MYSQL
    SVC3 --> MYSQL
    SVC1 --> SQLITE
    SVC2 --> SQLITE
    SVC3 --> SQLITE
    CONFIG --> SVC1
    CONFIG --> SVC2
    CONFIG --> SVC3
    
    style LB fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style SVC1 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style SVC2 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style SVC3 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style REDIS fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style MYSQL fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
```
