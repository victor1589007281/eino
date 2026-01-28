# MySQL内核专家Agent - 整体架构设计

## 1. 系统概述

MySQL内核专家Agent是一个基于eino框架开发的智能代码分析系统，专门用于从Percona Server源码角度解答MySQL内核相关问题。系统采用多Agent协作架构，结合代码索引、并发搜索、负载模拟等技术，提供深度的源码级分析能力。

## 2. 整体架构图

```mermaid
graph TB
    subgraph "用户交互层"
        UI[**用户输入**]
        OUT_S[**总结输出<br/>Chat Summary**]
        OUT_D[**文档输出<br/>Markdown Doc**]
    end
    
    subgraph "主控层 - Supervisor Agent"
        MA[**MasterAgent<br/>MySQL内核专家**]
        IR[**IntentRecognizer<br/>意图识别器**]
        PL[**Planner<br/>规划器**]
    end
    
    subgraph "执行层 - Sub Agents"
        SA1[**CodeSearchAgent<br/>代码搜索Agent**]
        SA2[**FunctionAnalyzer<br/>函数分析Agent**]
        SA3[**ArchitectureAgent<br/>架构分析Agent**]
        SA4[**SimulationAgent<br/>负载模拟Agent**]
        SA5[**DocumentAgent<br/>文档生成Agent**]
    end
    
    subgraph "工具层 - Tools"
        T1[**GrepTool<br/>文本搜索**]
        T2[**IndexSearchTool<br/>索引搜索**]
        T3[**SymbolLookupTool<br/>符号查找**]
        T4[**CallGraphTool<br/>调用图分析**]
        T5[**StatsTool<br/>统计信息工具**]
    end
    
    subgraph "索引层 - Index System"
        IDX1[**InvertedIndex<br/>倒排索引**]
        IDX2[**FunctionSummary<br/>函数摘要**]
        IDX3[**CallGraph<br/>调用图**]
        IDX4[**SymbolTable<br/>符号表**]
    end
    
    subgraph "存储层 - Storage"
        DB1[**SQLite<br/>索引存储**]
        DB2[**BoltDB<br/>图存储**]
        DB3[**File Cache<br/>文件缓存**]
    end
    
    subgraph "源码层"
        SRC[**Percona Server<br/>MySQL源码**]
    end
    
    UI --> MA
    MA --> IR
    IR --> PL
    PL --> SA1
    PL --> SA2
    PL --> SA3
    PL --> SA4
    PL --> SA5
    
    SA1 --> T1
    SA1 --> T2
    SA2 --> T3
    SA2 --> T4
    SA3 --> T4
    SA4 --> T5
    SA4 --> T4
    SA5 --> OUT_S
    SA5 --> OUT_D
    
    T1 --> SRC
    T2 --> IDX1
    T2 --> IDX2
    T3 --> IDX4
    T4 --> IDX3
    
    IDX1 --> DB1
    IDX2 --> DB1
    IDX3 --> DB2
    IDX4 --> DB1
    
    style MA fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style IR fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style PL fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style SA1 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style SA2 fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style SA3 fill:#fffacd,stroke:#333,stroke-width:2px,color:#000
    style SA4 fill:#ffe1f5,stroke:#333,stroke-width:2px,color:#000
    style SA5 fill:#e8e1ff,stroke:#333,stroke-width:2px,color:#000
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

| Agent | 职责 | 并发支持 |
|-------|------|----------|
| **CodeSearchAgent** | 代码搜索、关键词匹配 | ✅ 并发grep |
| **FunctionAnalyzer** | 函数分析、调用链追踪 | ✅ 并发分析 |
| **ArchitectureAgent** | 架构理解、模块关系 | ✅ 并发查询 |
| **SimulationAgent** | 负载模拟、瓶颈分析 | ✅ 并发计算 |
| **DocumentAgent** | 文档生成、图表渲染 | - |

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
| **GrepTool** | 正则搜索、文本匹配 | ripgrep加速 |
| **IndexSearchTool** | 索引查询、语义搜索 | 倒排索引 |
| **SymbolLookupTool** | 符号定位、定义跳转 | 符号表缓存 |
| **CallGraphTool** | 调用图查询、路径分析 | 图数据库 |
| **StatsTool** | 统计信息收集、性能指标 | 增量更新 |

## 4. 数据流架构

```mermaid
sequenceDiagram
    participant U as "用户"
    participant M as "MasterAgent"
    participant I as "IntentRecognizer"
    participant P as "Planner"
    participant S as "SubAgents"
    participant T as "Tools"
    participant D as "Database"
    
    U->>M: **1. 提交问题**
    M->>I: **2. 意图识别**
    I->>I: **3. 分类问题类型<br/>- 代码搜索类<br/>- 原理解释类<br/>- 性能分析类**
    I-->>M: **4. 返回意图结果**
    
    M->>P: **5. 制定执行计划**
    P->>P: **6. 任务分解<br/>- 确定需要的SubAgent<br/>- 确定执行顺序**
    P-->>M: **7. 返回执行计划**
    
    par **并行执行任务**
        M->>S: **8a. 调度CodeSearchAgent**
        S->>T: **9a. 调用GrepTool**
        T->>D: **10a. 查询索引**
        D-->>T: **11a. 返回结果**
        T-->>S: **12a. 返回搜索结果**
        S-->>M: **13a. 返回分析结果**
    and
        M->>S: **8b. 调度FunctionAnalyzer**
        S->>T: **9b. 调用CallGraphTool**
        T->>D: **10b. 查询调用图**
        D-->>T: **11b. 返回调用链**
        T-->>S: **12b. 返回调用链**
        S-->>M: **13b. 返回分析结果**
    end
    
    M->>M: **14. 汇总分析结果**
    M->>S: **15. 调度DocumentAgent**
    S-->>M: **16. 生成输出文档**
    M-->>U: **17. 返回最终结果**
    
    rect rgb(255, 250, 205)
    Note over U,D: **关键：并行执行SubAgent任务，减少总体响应时间**
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
        W1[**Worker-1**]
        W2[**Worker-2**]
        W3[**Worker-3**]
        W4[**Worker-N**]
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
| 多文件grep | Fan-out/Fan-in | `errgroup.Group` |
| 多函数分析 | Pipeline | Channel + Goroutine |
| 调用图遍历 | BFS/DFS并行 | WorkerPool |
| 索引构建 | MapReduce | 分片处理 |

## 6. 输出格式设计

### 6.1 总结输出 (Summary Mode)

```
【问题】: 用户问题简述
【结论】: 1-2句核心结论
【关键代码】: 关键函数/文件位置
【参考】: 相关源码链接
```

### 6.2 文档输出 (Document Mode)

```markdown
# 主题标题

## 1. 概述
简要说明...

## 2. 架构图
[mermaid架构图]

## 3. 核心函数分析
### 3.1 函数调用链
[函数调用链树状图]

### 3.2 关键代码
[代码片段及分析]

## 4. 时序图
[mermaid时序图]

## 5. 总结
结论及建议...
```

## 7. 扩展性设计

### 7.1 插件机制

- **新Agent注册**: 通过 `adk.SetSubAgents` 动态添加
- **新Tool注册**: 通过 `ToolsConfig.Tools` 扩展
- **新Skill注册**: 通过 `skill.Backend` 接口扩展

### 7.2 配置化

```yaml
mysql_expert:
  source_path: "/path/to/percona-server"
  index_path: "/path/to/index"
  max_concurrent: 10
  output_mode: "document"  # summary | document
  agents:
    - code_search
    - function_analyzer
    - architecture_agent
    - simulation_agent
```

## 8. 部署架构

```mermaid
graph TB
    subgraph "客户端"
        CLI[**CLI Interface**]
        API[**REST API**]
    end
    
    subgraph "服务层"
        SVC[**Agent Service**]
        CACHE[**Redis Cache**]
    end
    
    subgraph "存储层"
        IDX_DB[**SQLite-索引**]
        GRAPH_DB[**BoltDB-图**]
        FILE_SYS[**文件系统**]
    end
    
    subgraph "源码"
        SRC[**Percona Server**]
    end
    
    CLI --> SVC
    API --> SVC
    SVC --> CACHE
    SVC --> IDX_DB
    SVC --> GRAPH_DB
    SVC --> FILE_SYS
    FILE_SYS --> SRC
    
    style SVC fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style CACHE fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style IDX_DB fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style GRAPH_DB fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
```
