# 源码专家Agent开发技能指南

> 基于 Linux Kernel Expert Agent 项目的经验总结，适用于构建任意源码分析Agent

## 一、项目概述

### 1.1 目标定位
构建一个能够**基于源码回答技术问题**的智能Agent，核心能力包括：
- 代码搜索与定位
- 函数/结构体分析
- 调用链追踪
- 架构理解与文档生成

### 1.2 核心价值
- **准确性**：答案必须有源码依据，而非LLM臆测
- **效率性**：通过索引和缓存减少token消耗
- **可扩展性**：支持不同语言、不同代码库

## 二、架构设计思路

### 2.1 分层架构

```
┌─────────────────────────────────────────────────────────┐
│                    用户交互层                            │
│  (CLI / Web API / Chat Interface)                       │
├─────────────────────────────────────────────────────────┤
│                    Agent编排层                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │ 意图识别器   │  │  任务规划器  │  │  输出格式化  │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
├─────────────────────────────────────────────────────────┤
│                    工具层 (Tools)                        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ Grep搜索 │ │ 索引查询 │ │ 函数摘要 │ │ 调用图   │   │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │
├─────────────────────────────────────────────────────────┤
│                    索引层                               │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ 倒排索引 │ │ 符号表   │ │ 函数库   │ │ 调用图   │   │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │
├─────────────────────────────────────────────────────────┤
│                    基础设施层                            │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ LLM接口  │ │ 缓存系统 │ │ 统计监控 │ │ 配置管理 │   │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │
└─────────────────────────────────────────────────────────┘
```

### 2.2 核心设计决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| Agent框架 | Eino ADK | 提供ReAct循环、工具绑定、流式处理 |
| 搜索工具 | ripgrep | 性能极佳，支持正则，跨平台 |
| 索引格式 | gob + gzip | Go原生序列化，压缩存储 |
| LLM接口 | OpenAI兼容 | DeepSeek/OpenAI等都兼容此格式 |
| 并发模型 | Worker Pool | 控制并发数，避免资源耗尽 |

### 2.3 关键模块职责

```go
// 模块职责划分
type ModuleResponsibilities struct {
    Indexer     // 源码索引构建与查询
    Tools       // Agent可调用的工具集
    Agent       // 意图识别、规划、执行
    Output      // 结果格式化(Summary/Document)
    LLM         // 大模型接口封装
    Cache       // 查询结果缓存
    Statistics  // Token统计、性能监控
    Memory      // 上下文记忆管理
}
```

## 三、核心技术实现

### 3.1 索引系统设计

**关键洞察**：大型代码库无法全量发送给LLM，必须建立索引实现精准定位

```go
// 索引数据结构
type Index struct {
    InvertedIndex  // term -> [文件:行号] 倒排索引
    FunctionDB     // 函数签名、位置、参数信息
    SymbolTable    // 符号表：结构体、宏、类型定义
    CallGraph      // 函数调用关系图
    FileHashes     // 文件哈希用于增量更新
}

// 构建流程
1. 扫描源文件 (.c, .h, .S等)
2. 分词建立倒排索引
3. 解析函数定义建立函数库
4. 使用ctags/正则提取符号
5. 分析函数调用建立调用图
6. 持久化到磁盘
```

**索引字段必须导出**（gob序列化要求）：
```go
// ❌ 错误：私有字段无法序列化
type InvertedIndex struct {
    terms     map[string]*PostingList
    documents map[string]*Document
}

// ✅ 正确：导出字段
type InvertedIndex struct {
    Terms     map[string]*PostingList  // 首字母大写
    Documents map[string]*Document
}
```

### 3.2 工具设计原则

**工具是Agent的"手脚"**，设计要点：

```go
// 1. 实现 InvokableTool 接口
type Tool interface {
    Info(ctx context.Context) (*schema.ToolInfo, error)
    InvokableRun(ctx context.Context, args string, opts ...Option) (string, error)
}

// 2. 工具描述要详细（影响LLM决策）
func (t *GrepTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: "grep_code",
        Desc: `Search source code using pattern matching.
Use this tool to find code by exact text patterns.
Supports regex. Returns file path, line number, and content.

Example patterns:
- Function definition: "^static.*int\\s+function_name"
- Macro: "#define\\s+MACRO_NAME"
- Struct: "struct\\s+name\\s*\\{"`,
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "pattern": {
                Type:     schema.String,
                Desc:     "Search pattern (regex supported)",
                Required: true,
            },
            // ...
        }),
    }, nil
}

// 3. 参数Schema必须正确（API要求）
// 空参数也要提供对象schema：
if params == nil {
    params = json.RawMessage(`{"type": "object", "properties": {}}`)
}
```

### 3.3 LLM接口封装

**关键点**：正确实现`ToolCallingChatModel`接口

```go
type ToolCallingChatModel interface {
    Generate(ctx context.Context, input []*schema.Message, opts ...Option) (*schema.Message, error)
    Stream(ctx context.Context, input []*schema.Message, opts ...Option) (*schema.StreamReader[*schema.Message], error)
    WithTools(tools []*schema.ToolInfo) (ToolCallingChatModel, error)
}

// 工具参数序列化要使用ToJSONSchema()
for _, tool := range m.tools {
    if tool.ParamsOneOf != nil {
        jsonSchema, _ := tool.ParamsOneOf.ToJSONSchema()  // ✅ 正确转换
        params, _ = json.Marshal(jsonSchema)
    }
}
```

### 3.4 意图识别与任务规划

```go
// 意图类型定义
const (
    IntentConcept      = "concept"      // 概念解释
    IntentFunction     = "function"     // 函数分析  
    IntentCallChain    = "callchain"    // 调用链
    IntentArchitecture = "architecture" // 架构分析
    IntentComparison   = "comparison"   // 对比分析
    IntentDebug        = "debug"        // 问题调试
)

// 意图识别器（基于关键词+LLM）
func (r *IntentRecognizer) Recognize(query string) IntentResult {
    // 1. 关键词快速匹配
    if containsAny(query, []string{"什么是", "概念", "定义"}) {
        return IntentResult{Intent: IntentConcept, Confidence: 0.8}
    }
    
    // 2. LLM辅助识别（复杂场景）
    // ...
}

// 任务规划器
func (p *TaskPlanner) Plan(intent IntentResult, query string) *ExecutionPlan {
    switch intent.Intent {
    case IntentFunction:
        return &ExecutionPlan{
            Tasks: []Task{
                {Type: "search_function", Tool: "function_summary"},
                {Type: "read_source", Tool: "read_source"},
                {Type: "analyze_calls", Tool: "call_graph"},
            },
        }
    // ...
    }
}
```

## 四、遇到的问题与解决方案

### 4.1 序列化问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| `gob: type has no exported fields` | 私有字段无法序列化 | 将字段首字母大写 |
| `io.EOF not returned` | 自定义ReadCloser | 读完数据时返回`io.EOF` |

```go
// 修复前
func (r *readCloser) Read(p []byte) (n int, err error) {
    if r.pos >= len(r.data) {
        return 0, nil  // ❌ 无法判断是否读完
    }
}

// 修复后
func (r *readCloser) Read(p []byte) (n int, err error) {
    if r.pos >= len(r.data) {
        return 0, io.EOF  // ✅ 正确返回EOF
    }
}
```

### 4.2 类型系统问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| 循环导入 | agent ↔ output互相引用 | 提取共享类型到独立types包 |
| 类型重复声明 | test_helpers.go重复定义 | 删除重复，使用统一定义 |
| 接口不匹配 | 字段类型或方法签名不符 | 严格按接口定义实现 |

```go
// 解决循环导入：提取共享类型
// types/types.go
package types

type IntentType string
type OutputType string
type CallChainNode struct { ... }

// agent/main_agent.go
import "kernel_expert/types"

// output/formatter.go  
import "kernel_expert/types"
```

### 4.3 工具调用问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| `Invalid schema for function` | 参数Schema格式错误 | 使用`ToJSONSchema()`转换 |
| `executable not found` | ripgrep未安装 | Dockerfile安装或使用fallback |
| 工具调用超时 | 大文件搜索耗时 | 设置超时+限制结果数量 |

### 4.4 Docker问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| 卷挂载为空 | macOS文件共享未配置 | Docker Desktop设置允许路径 |
| 构建找不到模块 | replace指令路径问题 | 构建上下文包含父目录 |
| OOM Killed | 索引大型代码库内存不足 | 增加Docker内存限制 |

## 五、可复用的Skills模板

### 5.1 源码分析Agent模板

```yaml
# skill: source_code_expert
name: 源码分析专家Agent
description: 通用源码分析Agent框架

# 必需组件
components:
  - indexer:        # 索引系统
      inverted_index: true
      symbol_table: true
      call_graph: optional
  - tools:          # 工具集
      - grep_code    # 文本搜索
      - read_source  # 读取源码
      - index_search # 索引搜索
      - function_summary  # 函数信息
  - agent:          # Agent核心
      intent_recognition: true
      task_planning: true
      result_synthesis: true
  - output:         # 输出格式化
      formats: [summary, document]
      mermaid_support: true

# 语言适配
language_adapters:
  c:
    extensions: [.c, .h, .S]
    function_pattern: "^\\s*(static\\s+)?\\w+\\s+\\w+\\s*\\([^)]*\\)\\s*\\{"
    tools: [ctags, cscope]
  go:
    extensions: [.go]
    function_pattern: "^func\\s+(\\([^)]+\\)\\s+)?\\w+\\s*\\("
    tools: [gopls]
  python:
    extensions: [.py]
    function_pattern: "^\\s*def\\s+\\w+\\s*\\("
    tools: [jedi, rope]
  java:
    extensions: [.java]
    function_pattern: "(public|private|protected)?\\s+\\w+\\s+\\w+\\s*\\("
    tools: [jdtls]

# 部署配置
deployment:
  docker:
    base_image: golang:1.21-alpine
    runtime_deps: [ripgrep, ctags]
  resources:
    memory: 2Gi  # 大型代码库需要更多
    cpu: 2
```

### 5.2 工具开发模板

```go
// skill: tool_development_template
package tools

import (
    "context"
    "encoding/json"
    
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/schema"
)

// 1. 定义输入输出结构
type MyToolInput struct {
    RequiredField string   `json:"required_field"`
    OptionalField []string `json:"optional_field,omitempty"`
}

type MyToolOutput struct {
    Results []Result `json:"results"`
    Count   int      `json:"count"`
}

// 2. 实现Tool结构
type MyTool struct {
    // 依赖注入
    indexManager *IndexManager
    config       *Config
}

// 3. 实现Info方法 - 描述要详细！
func (t *MyTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: "my_tool",
        Desc: `详细描述工具用途、使用场景、输入输出格式。
        
使用示例：
- 场景A: "xxx"
- 场景B: "yyy"

返回格式：JSON对象包含results数组和count计数`,
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "required_field": {
                Type:     schema.String,
                Desc:     "必填字段说明",
                Required: true,
            },
            "optional_field": {
                Type:     schema.Array,
                Desc:     "可选字段说明，默认值xxx",
                Required: false,
            },
        }),
    }, nil
}

// 4. 实现InvokableRun方法
func (t *MyTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
    // 解析输入
    var input MyToolInput
    if err := json.Unmarshal([]byte(args), &input); err != nil {
        return "", fmt.Errorf("invalid input: %w", err)
    }
    
    // 参数验证
    if input.RequiredField == "" {
        return "", fmt.Errorf("required_field is required")
    }
    
    // 执行逻辑
    output := t.execute(ctx, &input)
    
    // 返回JSON
    result, _ := json.Marshal(output)
    return string(result), nil
}

// 5. 编译时检查接口实现
var _ tool.InvokableTool = (*MyTool)(nil)
```

### 5.3 LLM适配模板

```go
// skill: llm_adapter_template
package llm

// 通用LLM配置
type LLMConfig struct {
    APIKey      string
    BaseURL     string        // OpenAI兼容API的base URL
    Model       string
    Temperature float64
    MaxTokens   int
    Timeout     time.Duration
}

// 适配不同Provider
var ProviderConfigs = map[string]*LLMConfig{
    "openai": {
        BaseURL: "https://api.openai.com/v1",
        Model:   "gpt-4-turbo",
    },
    "deepseek": {
        BaseURL: "https://api.deepseek.com/v1",
        Model:   "deepseek-chat",
    },
    "moonshot": {
        BaseURL: "https://api.moonshot.cn/v1",
        Model:   "moonshot-v1-32k",
    },
    "zhipu": {
        BaseURL: "https://open.bigmodel.cn/api/paas/v4",
        Model:   "glm-4",
    },
}

// 创建模型（根据环境变量自动选择）
func CreateModel() (model.ToolCallingChatModel, error) {
    for provider, config := range ProviderConfigs {
        envKey := strings.ToUpper(provider) + "_API_KEY"
        if apiKey := os.Getenv(envKey); apiKey != "" {
            return NewOpenAICompatibleModel(&LLMConfig{
                APIKey:  apiKey,
                BaseURL: config.BaseURL,
                Model:   config.Model,
            })
        }
    }
    return nil, fmt.Errorf("no API key found")
}
```

### 5.4 索引系统模板

```go
// skill: index_system_template
package indexer

// 索引接口定义
type Index interface {
    // 文本搜索
    Search(terms []string, operator string, limit int) []*SearchResult
    
    // 符号查找
    GetSymbol(name string) []*Symbol
    
    // 函数信息
    GetFunction(name string) []*FunctionInfo
    
    // 调用链
    GetCallChain(function string, direction string, depth int) *CallChainNode
    
    // 读取源码
    ReadFile(path string, startLine, endLine int) (string, error)
}

// 通用索引构建器
type IndexBuilder struct {
    sourcePath string
    indexPath  string
    workers    int
    
    // 可配置的解析器
    tokenizer     Tokenizer
    functionParser FunctionParser
    symbolExtractor SymbolExtractor
}

// 构建流程
func (b *IndexBuilder) Build(ctx context.Context) error {
    // 1. 收集文件
    files, _ := b.collectFiles()
    
    // 2. 并发处理
    jobs := make(chan string, len(files))
    var wg sync.WaitGroup
    
    for i := 0; i < b.workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for file := range jobs {
                b.processFile(file)
            }
        }()
    }
    
    for _, f := range files {
        jobs <- f
    }
    close(jobs)
    wg.Wait()
    
    // 3. 持久化
    return b.save()
}

// 持久化注意事项：
// - 使用gob时字段必须导出（首字母大写）
// - 大索引考虑分片存储
// - 建议压缩（gzip）节省空间
```

## 六、最佳实践清单

### 6.1 开发阶段
- [ ] 先定义清晰的模块边界，避免循环依赖
- [ ] 共享类型提取到独立的`types`包
- [ ] 工具描述详细，包含示例
- [ ] 参数Schema完整，必须为object类型
- [ ] gob序列化字段必须导出

### 6.2 测试阶段
- [ ] 单元测试覆盖所有模块
- [ ] 集成测试使用小规模测试数据
- [ ] 测试LLM mock，避免真实API调用
- [ ] 测试超时和错误处理

### 6.3 部署阶段
- [ ] Docker镜像包含所有运行时依赖（ripgrep等）
- [ ] 配置文件支持环境变量覆盖
- [ ] 大型代码库预留足够内存
- [ ] macOS Docker需配置文件共享

### 6.4 运维阶段
- [ ] Token使用统计和成本监控
- [ ] 缓存命中率监控
- [ ] 响应时间告警
- [ ] 索引定期增量更新

## 七、项目文件清单

```
source_expert_agent/
├── agent/                 # Agent核心逻辑
│   ├── intent.go         # 意图识别
│   ├── main_agent.go     # 主Agent
│   └── sub_agents.go     # 子Agent
├── cache/                 # 缓存系统
│   ├── cache.go          # 缓存接口
│   └── lru.go            # LRU实现
├── config/                # 配置管理
│   └── config.go
├── indexer/               # 索引系统
│   ├── builder.go        # 索引构建
│   ├── inverted_index.go # 倒排索引
│   ├── function_summary.go # 函数库
│   ├── call_graph.go     # 调用图
│   ├── manager.go        # 索引管理
│   └── persistence.go    # 持久化
├── llm/                   # LLM接口
│   └── deepseek.go       # OpenAI兼容实现
├── memory/                # 记忆系统
│   └── memory.go
├── models/                # 模型路由
│   ├── router.go         # 复杂度路由
│   └── pool.go           # 连接池
├── output/                # 输出格式化
│   ├── formatter.go      # 格式化器
│   └── mermaid.go        # Mermaid图表
├── statistics/            # 统计系统
│   ├── stats.go          # 统计收集
│   └── token.go          # Token计数
├── tools/                 # 工具实现
│   ├── grep.go           # 代码搜索
│   ├── file_reader.go    # 文件读取
│   ├── index_search.go   # 索引搜索
│   ├── function_summary.go # 函数查询
│   └── call_graph.go     # 调用链查询
├── types/                 # 共享类型
│   └── types.go
├── skills/                # Skills定义
│   └── *.md
├── main.go               # 入口
├── Dockerfile            # 容器化
├── docker-compose.yml
├── Makefile              # 构建脚本
├── go.mod
└── README.md
```

---

**总结**：源码专家Agent的核心在于**索引系统**和**工具设计**。索引解决了大规模代码库的检索问题，工具让Agent能够实际操作代码。通过合理的分层架构和模块化设计，可以快速适配不同语言和代码库。
