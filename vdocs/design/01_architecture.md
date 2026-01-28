# Linux内核专家Agent - 整体架构设计

## 1. 项目概述

### 1.1 目标
构建一个基于eino框架的Linux内核专家Agent，能够：
- 以Linux内核源码为依据，从源码角度解答用户的Linux内核问题
- 支持高效的代码搜索（grep、倒排索引、函数摘要）
- 使用SubAgent机制并发加快问题分析
- 输出高质量的技术文档（包含mermaid图表、函数调用链、架构图等）

### 1.2 核心能力
| 能力 | 描述 |
|------|------|
| **源码分析** | 深入分析Linux内核源码，提供代码级别的解答 |
| **智能搜索** | 结合grep精确搜索和语义搜索，快速定位代码 |
| **并发分析** | 多SubAgent协作，并发执行不同分析任务 |
| **文档生成** | 生成带有mermaid图表的专业技术文档 |

## 2. 整体架构

```mermaid
graph TB
    subgraph "用户层"
        USER[**用户查询**]
    end
    
    subgraph "Agent层"
        MAIN[**主Agent<br/>LinuxKernelExpert**]
        
        subgraph "SubAgent集群"
            SA1[**代码搜索Agent<br/>CodeSearchAgent**]
            SA2[**函数分析Agent<br/>FunctionAnalyzer**]
            SA3[**调用链分析Agent<br/>CallChainAnalyzer**]
            SA4[**架构分析Agent<br/>ArchitectureAnalyzer**]
        end
    end
    
    subgraph "工具层"
        T1[**GrepTool<br/>精确文本搜索**]
        T2[**IndexSearchTool<br/>倒排索引搜索**]
        T3[**FunctionSummaryTool<br/>函数摘要查询**]
        T4[**CallGraphTool<br/>调用图分析**]
        T5[**FileReaderTool<br/>源码读取**]
    end
    
    subgraph "索引层"
        IDX1[**倒排索引<br/>InvertedIndex**]
        IDX2[**函数摘要库<br/>FunctionSummary**]
        IDX3[**符号表<br/>SymbolTable**]
        IDX4[**调用关系图<br/>CallGraph**]
    end
    
    subgraph "数据层"
        SRC[**Linux内核源码<br/>/linux**]
    end
    
    USER --> MAIN
    MAIN --> SA1
    MAIN --> SA2
    MAIN --> SA3
    MAIN --> SA4
    
    SA1 --> T1
    SA1 --> T2
    SA2 --> T3
    SA2 --> T5
    SA3 --> T4
    SA3 --> T5
    SA4 --> T2
    SA4 --> T5
    
    T1 --> SRC
    T2 --> IDX1
    T3 --> IDX2
    T4 --> IDX4
    T5 --> SRC
    
    IDX1 --> SRC
    IDX2 --> SRC
    IDX3 --> SRC
    IDX4 --> SRC
    
    style USER fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style MAIN fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style SA1 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style SA2 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style SA3 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style SA4 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style T1 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style T2 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style T3 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style T4 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style T5 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style IDX1 fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style IDX2 fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style IDX3 fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style IDX4 fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style SRC fill:#fffacd,stroke:#333,stroke-width:2px,color:#000
```

## 3. 数据流架构

```mermaid
sequenceDiagram
    participant U as "用户"
    participant M as "主Agent"
    participant P as "意图识别器"
    participant S as "SubAgent协调器"
    participant SA as "SubAgents"
    participant T as "工具集"
    participant I as "索引系统"
    participant O as "输出格式化器"
    
    U->>M: **1. 提交Linux内核问题**
    M->>P: **2. 分析用户意图**
    P->>P: **3. 识别问题类型<br/>-概念解释/函数分析/调用链/架构**
    P-->>M: **4. 返回意图分析结果**
    
    M->>S: **5. 制定执行计划**
    S->>S: **6. 任务分解与分配**
    
    par **并行执行SubAgent任务**
        S->>SA: **7a. 代码搜索任务**
        SA->>T: **7b. 调用搜索工具**
        T->>I: **7c. 查询索引**
        I-->>T: **7d. 返回匹配结果**
        T-->>SA: **7e. 返回代码片段**
        SA-->>S: **7f. 返回搜索结果**
    and
        S->>SA: **8a. 函数分析任务**
        SA->>T: **8b. 读取源码**
        T-->>SA: **8c. 返回函数定义**
        SA-->>S: **8d. 返回分析结果**
    and
        S->>SA: **9a. 调用链分析任务**
        SA->>T: **9b. 查询调用图**
        T->>I: **9c. 遍历调用关系**
        I-->>T: **9d. 返回调用链**
        T-->>SA: **9e. 返回调用链数据**
        SA-->>S: **9f. 返回分析结果**
    end
    
    S-->>M: **10. 汇总所有SubAgent结果**
    M->>O: **11. 格式化输出**
    O->>O: **12. 生成mermaid图表**
    O-->>M: **13. 返回格式化文档**
    M-->>U: **14. 返回最终答案**
    
    rect rgb(255, 250, 205)
    Note over U,O: **关键：并行SubAgent机制显著提升响应速度**
    end
```

## 4. 核心组件说明

### 4.1 主Agent (LinuxKernelExpert)
- **职责**：接收用户问题，协调SubAgent，汇总结果
- **实现**：基于eino的`ChatModelAgent`
- **特性**：
  - 意图识别与任务规划
  - SubAgent任务分配与协调
  - 结果汇总与输出格式化

### 4.2 SubAgent集群
| SubAgent | 职责 | 核心工具 |
|----------|------|----------|
| CodeSearchAgent | 代码搜索定位 | GrepTool, IndexSearchTool |
| FunctionAnalyzer | 函数深度分析 | FunctionSummaryTool, FileReaderTool |
| CallChainAnalyzer | 调用链追踪 | CallGraphTool, FileReaderTool |
| ArchitectureAnalyzer | 架构分析 | IndexSearchTool, FileReaderTool |

### 4.3 工具层
- **GrepTool**：基于ripgrep的精确文本搜索
- **IndexSearchTool**：基于倒排索引的快速搜索
- **FunctionSummaryTool**：函数摘要快速查询
- **CallGraphTool**：函数调用关系图查询
- **FileReaderTool**：源码文件读取

### 4.4 索引层
- **InvertedIndex**：倒排索引，支持关键词快速定位
- **FunctionSummary**：函数摘要库，包含签名、参数、简介
- **SymbolTable**：符号表，记录宏定义、类型定义等
- **CallGraph**：调用关系图，记录函数间调用关系

## 5. 技术栈

| 层次 | 技术选型 |
|------|----------|
| Agent框架 | eino adk |
| 编程语言 | Go 1.18+ |
| 文本搜索 | ripgrep (rg) |
| 索引存储 | 内存 + 磁盘持久化 |
| 序列化 | gob / JSON |
| 并发控制 | Go goroutine + channel |

## 6. 目录结构

```
vdocs/
├── design/                    # 设计文档
│   ├── 01_architecture.md     # 整体架构设计
│   ├── 02_modules.md          # 模块设计
│   ├── 03_tech_selection.md   # 技术选型
│   └── 04_thinking_tools.md   # 思维工具使用方案
├── kernel_expert/             # 源代码
│   ├── agent/                 # Agent实现
│   │   ├── main_agent.go      # 主Agent
│   │   ├── code_search.go     # 代码搜索Agent
│   │   ├── function_analyzer.go # 函数分析Agent
│   │   ├── call_chain.go      # 调用链分析Agent
│   │   └── architecture.go    # 架构分析Agent
│   ├── tools/                 # 工具实现
│   │   ├── grep.go            # Grep工具
│   │   ├── index_search.go    # 索引搜索工具
│   │   ├── function_summary.go # 函数摘要工具
│   │   ├── call_graph.go      # 调用图工具
│   │   └── file_reader.go     # 文件读取工具
│   ├── indexer/               # 索引系统
│   │   ├── inverted_index.go  # 倒排索引
│   │   ├── function_summary.go # 函数摘要
│   │   ├── symbol_table.go    # 符号表
│   │   └── call_graph.go      # 调用关系图
│   ├── output/                # 输出格式化
│   │   ├── formatter.go       # 格式化器
│   │   ├── mermaid.go         # Mermaid生成
│   │   └── markdown.go        # Markdown生成
│   └── main.go                # 入口文件
└── output/                    # 生成的文档输出
```

## 7. 扩展性设计

### 7.1 工具插件化
- 工具实现`tool.BaseTool`接口
- 支持动态注册新工具
- 工具配置化管理

### 7.2 SubAgent可扩展
- SubAgent继承基础Agent能力
- 支持新增专业化SubAgent
- 任务分配策略可配置

### 7.3 索引可更新
- 支持增量索引更新
- 索引版本管理
- 自动检测源码变更
