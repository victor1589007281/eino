# 微信公众号文章润色专家Agent - 整体架构设计

## 1. 系统概述

微信公众号文章润色专家Agent是一个基于eino框架开发的智能文章处理系统，专门用于从专业角度润色微信公众号文章。系统采用多Agent协作架构，结合内容索引、并发处理、多轮交互等技术，提供专业的文章润色能力，包括语法检查、逻辑优化、结构调整、语言风格统一等功能。

### 1.1 核心特性

| 特性 | 描述 | 技术实现 |
|------|------|----------|
| **多类型文章支持** | 支持技术类和非技术类文章润色 | 意图识别 + 专业技能库 |
| **Markdown处理** | 支持指定路径的Markdown文件输入输出 | 文件系统中间件 |
| **多轮交互** | 支持用户上传内容后的多轮修改 | 会话状态管理 |
| **索引系统** | 倒排索引、摘要索引加速搜索 | SQLite + BoltDB |
| **并发处理** | SubAgent并发机制加速处理 | Supervisor模式 |
| **模型路由** | 支持国内外多种大模型 | 智能路由器 |
| **缓存系统** | 记忆系统 + 索引缓存减少Token | 多级缓存 |
| **统计模块** | Token统计、缓存命中统计 | 实时监控 |
| **Agent交互** | 支持与外部Agent交互 | A2A协议 |

## 2. 整体架构图

```mermaid
graph TB
    subgraph "用户交互层"
        UI[**用户输入**]
        MD_IN[**Markdown文件输入**]
        OUT_S[**总结输出<br/>Chat Summary**]
        OUT_D[**Markdown文档输出**]
    end
    
    subgraph "主控层 - Supervisor Agent"
        MA[**MasterAgent<br/>文章润色专家**]
        IR[**IntentRecognizer<br/>意图识别器**]
        PL[**Planner<br/>规划器**]
        MR[**ModelRouter<br/>模型路由器**]
    end
    
    subgraph "执行层 - Sub Agents"
        SA1[**GrammarAgent<br/>语法检查Agent**]
        SA2[**LogicAgent<br/>逻辑优化Agent**]
        SA3[**StructureAgent<br/>结构调整Agent**]
        SA4[**StyleAgent<br/>风格统一Agent**]
        SA5[**TechAgent<br/>技术文章Agent**]
        SA6[**ContentAgent<br/>内容增强Agent**]
    end
    
    subgraph "工具层 - Tools"
        T1[**GrammarCheckTool<br/>语法检查**]
        T2[**StyleAnalyzeTool<br/>风格分析**]
        T3[**StructureTool<br/>结构分析**]
        T4[**WeChatNormTool<br/>公众号规范**]
        T5[**MarkdownTool<br/>Markdown处理**]
    end
    
    subgraph "索引层 - Index System"
        IDX1[**InvertedIndex<br/>倒排索引**]
        IDX2[**ContentSummary<br/>内容摘要**]
        IDX3[**StyleIndex<br/>风格索引**]
        IDX4[**SkillsIndex<br/>技能索引**]
    end
    
    subgraph "缓存层 - Cache System"
        C1[**MemoryCache<br/>记忆缓存**]
        C2[**IndexCache<br/>索引缓存**]
        C3[**ResponseCache<br/>响应缓存**]
    end
    
    subgraph "存储层 - Storage"
        DB1[**SQLite<br/>索引存储**]
        DB2[**BoltDB<br/>内容存储**]
        DB3[**External DB<br/>外部数据库**]
    end
    
    subgraph "统计层"
        STATS[**StatsCollector<br/>统计收集器**]
    end
    
    UI --> MA
    MD_IN --> MA
    MA --> IR
    IR --> PL
    PL --> MR
    MR --> SA1
    MR --> SA2
    MR --> SA3
    MR --> SA4
    MR --> SA5
    MR --> SA6
    
    SA1 --> T1
    SA2 --> T2
    SA3 --> T3
    SA4 --> T4
    SA5 --> T1
    SA5 --> T5
    SA6 --> T4
    
    T1 --> IDX1
    T2 --> IDX2
    T3 --> IDX3
    T4 --> IDX4
    
    IDX1 --> C2
    IDX2 --> C2
    C2 --> DB1
    C1 --> DB2
    DB1 -.-> DB3
    DB2 -.-> DB3
    
    MA --> STATS
    SA1 --> STATS
    SA2 --> STATS
    
    SA6 --> OUT_S
    SA6 --> OUT_D
    
    style MA fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style IR fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style PL fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style MR fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style SA1 fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style SA2 fill:#fffacd,stroke:#333,stroke-width:2px,color:#000
    style SA3 fill:#ffe1f5,stroke:#333,stroke-width:2px,color:#000
    style SA4 fill:#e8e1ff,stroke:#333,stroke-width:2px,color:#000
    style SA5 fill:#d4edda,stroke:#333,stroke-width:2px,color:#000
    style SA6 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style IDX1 fill:#d4edda,stroke:#333,stroke-width:2px,color:#000
    style IDX2 fill:#d4edda,stroke:#333,stroke-width:2px,color:#000
    style IDX3 fill:#d4edda,stroke:#333,stroke-width:2px,color:#000
    style IDX4 fill:#d4edda,stroke:#333,stroke-width:2px,color:#000
    style STATS fill:#fff0f0,stroke:#333,stroke-width:2px,color:#000
```

## 3. 核心组件说明

### 3.1 主控层 (Supervisor Agent)

| 组件 | 功能 | 技术实现 |
|------|------|----------|
| **MasterAgent** | 协调所有子Agent，管理对话上下文 | `adk.ChatModelAgent` + `supervisor.New` |
| **IntentRecognizer** | 识别用户意图，分类文章类型 | LLM + 规则引擎 |
| **Planner** | 制定润色计划，分配任务 | `adk.prebuilt.deep` 任务规划 |
| **ModelRouter** | 根据任务类型路由到合适的模型 | 智能路由策略 |

### 3.2 执行层 (Sub Agents)

| Agent | 职责 | 并发支持 |
|-------|------|----------|
| **GrammarAgent** | 语法检查、错别字纠正 | ✅ 段落并发 |
| **LogicAgent** | 逻辑分析、论点论据优化 | ✅ 章节并发 |
| **StructureAgent** | 结构调整、段落重组 | ✅ 并发分析 |
| **StyleAgent** | 风格统一、用词规范 | ✅ 并发检查 |
| **TechAgent** | 技术术语、代码块处理 | ✅ 代码并发 |
| **ContentAgent** | 内容增强、可读性优化 | - |

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
| **GrammarCheckTool** | 语法、拼写检查 | 批量处理 |
| **StyleAnalyzeTool** | 风格分析、一致性检查 | 索引加速 |
| **StructureTool** | 结构分析、大纲生成 | 缓存优化 |
| **WeChatNormTool** | 微信公众号规范检查 | 规则引擎 |
| **MarkdownTool** | Markdown解析和格式化 | 流式处理 |

## 4. 数据流架构

```mermaid
sequenceDiagram
    participant U as "用户"
    participant M as "MasterAgent"
    participant I as "IntentRecognizer"
    participant P as "Planner"
    participant R as "ModelRouter"
    participant S as "SubAgents"
    participant T as "Tools"
    participant C as "Cache"
    participant ST as "Stats"
    
    U->>M: **1. 提交文章内容**
    M->>C: **2. 检查缓存**
    C-->>M: **3. 缓存未命中**
    M->>I: **4. 意图识别**
    I->>I: **5. 分类文章类型<br/>- 技术类文章<br/>- 非技术类文章**
    I-->>M: **6. 返回意图结果**
    
    M->>P: **7. 制定润色计划**
    P->>P: **8. 任务分解<br/>- 确定需要的SubAgent<br/>- 确定执行顺序**
    P-->>M: **9. 返回执行计划**
    
    M->>R: **10. 模型路由**
    R->>R: **11. 选择合适模型<br/>- 简单任务用轻量模型<br/>- 复杂任务用推理模型**
    
    par **并行执行任务**
        M->>S: **12a. 调度GrammarAgent**
        S->>T: **13a. 调用GrammarCheckTool**
        T->>ST: **14a. 记录Token消耗**
        T-->>S: **15a. 返回检查结果**
        S-->>M: **16a. 返回语法优化**
    and
        M->>S: **12b. 调度StyleAgent**
        S->>T: **13b. 调用StyleAnalyzeTool**
        T->>ST: **14b. 记录Token消耗**
        T-->>S: **15b. 返回风格分析**
        S-->>M: **16b. 返回风格建议**
    end
    
    M->>M: **17. 汇总润色结果**
    M->>C: **18. 更新缓存**
    M->>ST: **19. 汇总统计信息**
    M-->>U: **20. 返回润色后文章**
    
    rect rgb(255, 250, 205)
    Note over U,ST: **关键：并行执行SubAgent任务，减少总体响应时间**
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
        W1[**GrammarWorker**]
        W2[**LogicWorker**]
        W3[**StyleWorker**]
        W4[**StructureWorker**]
    end
    
    subgraph "任务队列"
        Q[**TaskQueue**]
    end
    
    subgraph "结果聚合"
        AGG[**Aggregator**]
    end
    
    SCH --> Q
    Q --> W1
    Q --> W2
    Q --> W3
    Q --> W4
    W1 --> AGG
    W2 --> AGG
    W3 --> AGG
    W4 --> AGG
    
    style SCH fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style AGG fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style W1 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style W2 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style W3 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style W4 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
```

### 5.2 并发策略

| 场景 | 并发策略 | 实现方式 |
|------|----------|----------|
| 多段落检查 | Fan-out/Fan-in | `errgroup.Group` |
| 多维度分析 | Pipeline | Channel + Goroutine |
| 风格遍历 | 并行分析 | WorkerPool |
| 索引构建 | MapReduce | 分片处理 |

## 6. 模型路由架构

```mermaid
graph TB
    subgraph "路由入口"
        REQ[**润色请求**]
    end
    
    subgraph "任务分类"
        TC[**TaskClassifier<br/>任务分类器**]
    end
    
    subgraph "路由策略"
        RS1[**简单任务<br/>-语法检查<br/>-格式修正**]
        RS2[**中等任务<br/>-逻辑优化<br/>-风格统一**]
        RS3[**复杂任务<br/>-内容重构<br/>-深度润色**]
    end
    
    subgraph "模型池 - 国内模型"
        M1[**通义千问<br/>Qwen**]
        M2[**文心一言<br/>ERNIE**]
        M3[**DeepSeek<br/>深度求索**]
        M4[**智谱AI<br/>GLM**]
        M5[**Moonshot<br/>月之暗面**]
    end
    
    subgraph "模型池 - 国外模型"
        M6[**Claude<br/>Anthropic**]
        M7[**GPT-4<br/>OpenAI**]
        M8[**Gemini<br/>Google**]
    end
    
    subgraph "模型池 - 本地模型"
        M9[**Ollama<br/>本地部署**]
        M10[**vLLM<br/>高性能推理**]
    end
    
    REQ --> TC
    TC --> RS1
    TC --> RS2
    TC --> RS3
    
    RS1 --> M1
    RS1 --> M3
    RS1 --> M9
    
    RS2 --> M2
    RS2 --> M4
    RS2 --> M5
    
    RS3 --> M6
    RS3 --> M7
    RS3 --> M8
    
    style TC fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style RS1 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style RS2 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style RS3 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style M6 fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style M7 fill:#fffacd,stroke:#333,stroke-width:2px,color:#000
```

## 7. Agent交互架构 (A2A)

```mermaid
graph TB
    subgraph "本地Agent系统"
        LA[**WeChatPolishAgent<br/>微信润色Agent**]
        A2AC[**A2AClient<br/>Agent通信客户端**]
    end
    
    subgraph "A2A协议层"
        PROTO[**A2A Protocol<br/>- AgentCard<br/>- Task/Message<br/>- Artifact**]
    end
    
    subgraph "外部Agent系统"
        EA1[**SEOAgent<br/>SEO优化Agent**]
        EA2[**TranslateAgent<br/>翻译Agent**]
        EA3[**ImageGenAgent<br/>配图生成Agent**]
    end
    
    LA --> A2AC
    A2AC --> PROTO
    PROTO --> EA1
    PROTO --> EA2
    PROTO --> EA3
    
    style LA fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style A2AC fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style PROTO fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style EA1 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style EA2 fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style EA3 fill:#fffacd,stroke:#333,stroke-width:2px,color:#000
```

## 8. 输出格式设计

### 8.1 总结输出 (Summary Mode)

```
【原文】: 文章原标题
【优化标题】: 优化后的标题
【主要修改】: 
  1. 语法修正: XX处
  2. 逻辑优化: XX处
  3. 风格统一: XX处
【Token消耗】: 输入XX / 输出XX
【缓存命中率】: XX%
```

### 8.2 文档输出 (Document Mode)

```markdown
# 润色后文章

## 原文对比
[差异对比视图]

## 修改说明
### 1. 语法修正
[具体修改列表]

### 2. 逻辑优化
[逻辑调整说明]

### 3. 结构调整
[结构变化图]

### 4. 风格统一
[风格建议]

## 统计信息
[Token消耗、处理时间等]
```

## 9. 部署架构

```mermaid
graph TB
    subgraph "客户端"
        CLI[**CLI Interface**]
        API[**REST API**]
        WEB[**Web UI**]
    end
    
    subgraph "Kubernetes集群"
        subgraph "服务层"
            SVC[**Agent Service**]
            GW[**API Gateway**]
        end
        
        subgraph "存储层"
            REDIS[**Redis Cache**]
            SQLITE[**SQLite-索引**]
            BOLT[**BoltDB-内容**]
        end
        
        subgraph "配置管理"
            CM[**ConfigMap**]
            SEC[**Secrets**]
        end
    end
    
    subgraph "外部服务"
        LLM[**LLM Providers**]
        EXT_DB[**External Database**]
    end
    
    CLI --> GW
    API --> GW
    WEB --> GW
    GW --> SVC
    SVC --> REDIS
    SVC --> SQLITE
    SVC --> BOLT
    SVC --> CM
    SVC --> SEC
    SVC --> LLM
    SQLITE -.-> EXT_DB
    BOLT -.-> EXT_DB
    
    style SVC fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style GW fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style REDIS fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style SQLITE fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style BOLT fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
```

## 10. 扩展性设计

### 10.1 插件机制

- **新Agent注册**: 通过 `adk.SetSubAgents` 动态添加
- **新Tool注册**: 通过 `ToolsConfig.Tools` 扩展
- **新Skill注册**: 通过 `skill.Backend` 接口扩展
- **新模型注册**: 通过 `ModelRouter.Register` 添加

### 10.2 配置化

```yaml
wechat_polish:
  work_path: "/path/to/workspace"
  input_path: "/path/to/input"
  output_path: "/path/to/output"
  
  index:
    type: "sqlite"          # sqlite | external
    path: "/path/to/index"
    external_dsn: ""        # 外部数据库连接
  
  cache:
    enabled: true
    type: "memory"          # memory | redis
    ttl: 30m
    redis_url: ""
  
  models:
    primary: "deepseek"
    fallback: "qwen"
    local: "ollama"
    
  agents:
    - grammar
    - logic
    - structure
    - style
    - tech
    - content
    
  stats:
    enabled: true
    export_interval: 60s
```
