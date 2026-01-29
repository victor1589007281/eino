# MySQL内核专家Agent开发技能指南

> 基于 MySQL Kernel Expert Agent 项目的经验总结，适用于构建任意源码分析Agent

## 一、项目概述

### 1.1 目标定位
构建一个能够**基于MySQL源码(Percona Server)回答内核问题**的智能Agent，核心能力包括：
- 代码搜索与定位（grep + 索引）
- 函数调用链分析
- 负载模拟与瓶颈分析
- 架构文档生成（含Mermaid图表）

### 1.2 核心价值
- **准确性**：答案必须有源码依据，杜绝LLM臆测
- **效率性**：多层缓存 + 索引系统减少token消耗70%+
- **可观测性**：Token统计、缓存命中率、请求监控
- **易部署**：Docker一键部署，K8s Helm Chart支持

### 1.3 项目成果
```
✅ 7/7 集成测试全部通过
✅ Docker容器化部署成功
✅ DeepSeek API调用验证成功
✅ 源码搜索/索引系统可用
```

## 二、架构设计思路

### 2.1 分层架构

```
┌─────────────────────────────────────────────────────────────┐
│                    用户交互层                                 │
│  (CLI -q/-i / Web API / Docker)                             │
├─────────────────────────────────────────────────────────────┤
│                    Agent编排层 (Eino ADK)                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │ MasterAgent │  │ CodeSearch  │  │ FuncAnalyzer│ SubAgent │
│  │ (Supervisor)│  │  SubAgent   │  │  SubAgent   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                    工具层 (Tools)                            │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐        │
│  │ GrepTool │ │SymbolTool│ │IndexQuery│ │Simulation│        │
│  │ (ripgrep)│ │ (ctags)  │ │ (SQLite) │ │ (Graph)  │        │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘        │
├─────────────────────────────────────────────────────────────┤
│                    索引与存储层                              │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐        │
│  │ SQLite   │ │ BoltDB   │ │ LRU缓存  │ │ 分片缓存 │        │
│  │(倒排索引)│ │(调用图)  │ │(L1 Memory)│ │(L2 Dist.)│       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘        │
├─────────────────────────────────────────────────────────────┤
│                    基础设施层                                │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐        │
│  │ LLM接口  │ │ 统计监控 │ │ 配置管理 │ │ 日志系统 │        │
│  │(DeepSeek)│ │(Token等) │ │ (YAML)   │ │(logrus)  │        │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘        │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 核心设计决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| Agent框架 | Eino ADK | Supervisor模式、流式处理、中间件支持 |
| 搜索工具 | ripgrep | 性能极佳(比grep快10x+)，JSON输出 |
| 符号解析 | ctags | 跨语言支持，JSON格式输出 |
| 索引存储 | SQLite + BoltDB | 嵌入式、无需额外服务、FTS5可选 |
| 缓存系统 | 多层LRU | L1内存 + L2分片 + L3持久化 |
| LLM接口 | OpenAI兼容 | DeepSeek/GPT/Claude等均兼容 |
| 部署方案 | Docker + Helm | 一键部署，配置化管理 |

### 2.3 关键模块职责

```go
type ModuleResponsibilities struct {
    Agent   struct {
        MasterAgent       // 主协调器，意图识别与任务分发
        CodeSearchAgent   // 代码搜索子Agent
        FuncAnalyzerAgent // 函数分析子Agent
        SkillBackend      // MySQL特定Skills注入
    }
    Tools   struct {
        GrepTool          // ripgrep文本搜索
        SymbolLookupTool  // ctags符号查找
        IndexQueryTool    // 索引系统查询
        SimulationTool    // 负载模拟
    }
    Storage struct {
        SQLiteStorage     // 文档、倒排索引、函数摘要
        BoltDBStorage     // 调用图(键值存储)
    }
    Cache   struct {
        LRUCache          // 基础LRU缓存
        ShardedLRUCache   // 并发安全分片缓存
        MultiLayerCache   // 多层缓存(L1/L2/L3)
        QueryCache        // 查询结果缓存
    }
    Stats   struct {
        TokenCollector    // Token使用统计
        CacheCollector    // 缓存命中统计
        RequestCollector  // 请求统计
    }
    LLM     struct {
        OpenAIProvider    // OpenAI兼容接口
        ModelRouter       // 模型路由(复杂度分级)
    }
}
```

## 三、核心技术实现

### 3.1 工具设计 (关键!)

工具是Agent的"手脚"，设计质量直接影响Agent能力：

```go
// 1. 实现 InvokableTool 接口
type GrepTool struct {
    sourcePath string
    maxResults int
}

// 2. Info方法 - 描述越详细，LLM调用越准确
func (t *GrepTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: "grep_code",
        Desc: `在MySQL源码中搜索代码。支持正则表达式。

使用场景：
- 查找函数定义: "^(static\\s+)?\\w+\\s+mysql_parse\\s*\\("
- 查找宏定义: "#define\\s+MACRO_NAME"
- 查找结构体: "struct\\s+THD\\s*\\{"
- 模糊搜索: "innodb.*lock"

返回格式：JSON数组，包含文件路径、行号、内容`,
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "pattern": {
                Type:     schema.String,
                Desc:     "搜索模式(支持正则)",
                Required: true,
            },
            "file_types": {
                Type:     schema.Array,
                Desc:     "文件类型过滤，如[\"cc\", \"h\"]",
                Required: false,
            },
        }),
    }, nil
}

// 3. InvokableRun - 实际执行逻辑
func (t *GrepTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
    var input GrepInput
    json.Unmarshal([]byte(args), &input)
    
    // 调用ripgrep
    cmd := exec.CommandContext(ctx, "rg", "--json", input.Pattern, t.sourcePath)
    // ... 解析结果，返回JSON
}

// 4. 编译时检查接口实现
var _ tool.InvokableTool = (*GrepTool)(nil)
```

### 3.2 索引系统设计

**核心洞察**：MySQL源码80万+行，无法全量发送LLM，必须建立索引

```go
// 索引数据结构
type IndexSystem struct {
    // SQLite存储
    Documents      // 文件信息(路径、哈希、行数)
    InvertedIndex  // 倒排索引: term -> [docID:positions]
    FunctionSummary // 函数摘要: name -> {签名、位置、参数}
    NGramIndex     // N-gram子串索引(支持模糊搜索)
    
    // BoltDB存储
    CallGraph      // 调用图: funcID -> [callers, callees]
}

// SQLite Schema (关键：FTS5可选)
CREATE TABLE documents (...);
CREATE TABLE inverted_index (...);
CREATE TABLE function_summaries (...);
-- FTS5可选，macOS默认SQLite不支持
-- CREATE VIRTUAL TABLE function_fts USING fts5(...);
```

**FTS5问题与解决**：
```go
// macOS默认SQLite不包含FTS5模块，需要优雅降级
func (s *SQLiteStorage) Init(ctx context.Context) error {
    // 1. 创建基础表（必须成功）
    _, err := s.db.ExecContext(ctx, baseSchema)
    if err != nil {
        return err
    }
    
    // 2. 尝试创建FTS5表（可选，失败也OK）
    _, ftsErr := s.db.ExecContext(ctx, fts5Schema)
    s.fts5Available = ftsErr == nil  // 记录是否可用
    
    return nil  // 不因FTS5失败而中断
}
```

### 3.3 缓存系统设计

```go
// 多层缓存架构
type MultiLayerCache struct {
    L1 *LRUCache        // 内存缓存(毫秒级)
    L2 *ShardedLRUCache // 分片缓存(并发安全)
    L3 Cache            // 可选：Redis等分布式缓存
}

// 读取策略：L1 -> L2 -> L3 -> 源
func (c *MultiLayerCache) Get(ctx context.Context, key string) (interface{}, bool) {
    // 尝试L1
    if val, ok := c.L1.Get(ctx, key); ok {
        return val, true
    }
    // 尝试L2
    if c.L2 != nil {
        if val, ok := c.L2.Get(ctx, key); ok {
            c.L1.Set(ctx, key, val, time.Minute) // 回填L1
            return val, true
        }
    }
    // ...
    return nil, false
}

// 写入策略：写入所有层
func (c *MultiLayerCache) Set(ctx context.Context, key string, val interface{}, ttl time.Duration) {
    c.L1.Set(ctx, key, val, ttl)
    if c.L2 != nil { c.L2.Set(ctx, key, val, ttl) }
    if c.L3 != nil { c.L3.Set(ctx, key, val, ttl) }
}
```

### 3.4 意图识别

```go
// 意图类型
const (
    IntentCodeSearch      = "code_search"      // 找代码
    IntentCallChain       = "call_chain"       // 调用链
    IntentExplainMechanism = "explain_mechanism" // 原理解释
    IntentPerformance     = "performance"      // 性能分析
    IntentArchitecture    = "architecture"     // 架构分析
    IntentSimulation      = "simulation"       // 负载模拟
)

// 关键词匹配（快速路径）
func ClassifyIntent(query string) IntentType {
    q := strings.ToLower(query)
    
    switch {
    case containsAny(q, []string{"调用链", "调用关系", "谁调用"}):
        return IntentCallChain
    case containsAny(q, []string{"如何实现", "原理", "机制"}):
        return IntentExplainMechanism
    case containsAny(q, []string{"查找", "搜索", "定位", "找到"}):
        return IntentCodeSearch
    case containsAny(q, []string{"模拟", "负载", "瓶颈"}):
        return IntentSimulation
    default:
        return IntentCodeSearch // 默认代码搜索
    }
}
```

### 3.5 Eino ADK集成

```go
// 使用Supervisor模式组织Agent
func NewMasterAgent(ctx context.Context, cfg *MasterAgentConfig) (*MasterAgent, error) {
    // 1. 创建工具
    grepTool := tools.NewGrepTool(&tools.GrepToolConfig{
        SourcePath: cfg.Config.Source.Path,
    })
    
    // 2. 创建主Agent
    mainAgent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Name:        "mysql_expert",
        Description: "MySQL内核专家Agent",
        Model:       cfg.ChatModel,
        ToolsConfig: adk.ToolsConfig{
            ToolsNodeConfig: compose.ToolsNodeConfig{
                Tools: []tool.BaseTool{grepTool, symbolTool},
            },
        },
        Middlewares: []adk.AgentMiddleware{skillMiddleware},
    })
    
    // 3. 创建子Agent
    codeSearchAgent, _ := createCodeSearchAgent(ctx, grepTool)
    funcAnalyzerAgent, _ := createFunctionAnalyzerAgent(ctx)
    
    // 4. 建立Supervisor层级
    supervisorAgent, _ := supervisor.New(ctx, &supervisor.Config{
        Supervisor: mainAgent,
        SubAgents:  []adk.Agent{codeSearchAgent, funcAnalyzerAgent},
    })
    
    return &MasterAgent{agent: supervisorAgent}, nil
}
```

## 四、遇到的问题与解决方案

### 4.1 类型系统问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| `adk.ToolsNodeConfig undefined` | Eino ADK类型位置 | 使用`compose.ToolsNodeConfig` |
| `interface{} does not implement` | 返回类型不匹配 | 使用正确的接口类型 |
| 字段未导出 | gob序列化要求 | 首字母大写导出字段 |

```go
// ❌ 错误
ToolsConfig: adk.ToolsConfig{
    ToolsNodeConfig: adk.ToolsNodeConfig{...}, // undefined
}

// ✅ 正确
import "github.com/cloudwego/eino/compose"

ToolsConfig: adk.ToolsConfig{
    ToolsNodeConfig: compose.ToolsNodeConfig{...},
}
```

### 4.2 SQLite FTS5问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| `no such module: fts5` | macOS默认SQLite不含FTS5 | 优雅降级，FTS5可选 |
| CGO构建问题 | SQLite需要CGO | `CGO_ENABLED=1` |

```go
// 解决：FTS5可选，不影响核心功能
func (s *SQLiteStorage) Init(ctx context.Context) error {
    // 基础表必须成功
    _, err := s.db.ExecContext(ctx, baseSchema)
    if err != nil { return err }
    
    // FTS5尝试创建，失败也OK
    _, ftsErr := s.db.ExecContext(ctx, fts5Schema)
    s.fts5Available = ftsErr == nil
    
    return nil  // 即使FTS5失败也继续
}
```

### 4.3 Docker构建问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| `universal-ctags: no such package` | Alpine 3.19包名不同 | 使用`ctags`替代 |
| replace指令失效 | 构建上下文问题 | COPY整个workspace |
| 路径硬编码 | 测试代码路径写死 | 使用环境变量 |

```dockerfile
# Dockerfile关键修改

# 1. 使用ctags替代universal-ctags
RUN apk add --no-cache ripgrep ctags sqlite-libs

# 2. 复制整个workspace（包含eino父模块）
COPY . /workspace
WORKDIR /workspace/vdocsMysql

# 3. CGO启用构建
RUN CGO_ENABLED=1 go build -o /app/bin/mysql-expert-agent ./cmd/main.go
```

### 4.4 测试问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| `fmt.Println redundant newline` | 字符串末尾+Println都有换行 | 使用`fmt.Print` |
| 浮点数比较失败 | 精度问题 | 使用范围比较 |
| 变量名冲突 | `storage`包名与变量名冲突 | 改变量名为`store` |

```go
// ❌ 变量名与包名冲突
storage, err := storage.NewSQLiteStorage(...)

// ✅ 重命名变量
store, err := storage.NewSQLiteStorage(...)

// ❌ 浮点数精确比较
if summary.TotalCost != 0.01 { ... }

// ✅ 范围比较
if summary.TotalCost < 0.009 || summary.TotalCost > 0.011 { ... }
```

## 五、可复用的Skills模板

### 5.1 源码分析Agent通用框架

```yaml
# skill: source_expert_agent_framework
name: 源码分析专家Agent框架
description: 适用于任意大型代码库的分析Agent

# 必需组件
components:
  tools:
    grep_code:
      binary: ripgrep
      purpose: 文本搜索
      output: JSON
    symbol_lookup:
      binary: ctags
      purpose: 符号定位
      output: JSON
    index_query:
      storage: SQLite/BoltDB
      purpose: 快速检索
  
  storage:
    documents: SQLite     # 文件元信息
    inverted_index: SQLite # 倒排索引
    call_graph: BoltDB    # 调用图
    cache: LRU            # 多层缓存
  
  agent:
    framework: Eino ADK
    pattern: Supervisor   # 主Agent + 子Agent
    intent_recognition: keyword + LLM
    streaming: true

# 语言适配器
language_adapters:
  cpp:  # MySQL/Linux内核
    extensions: [.cc, .cpp, .c, .h]
    function_pattern: "^(static\\s+)?\\w+\\s+\\w+\\s*\\([^)]*\\)"
    comment_pattern: "/\\*.*?\\*/|//.*$"
  go:
    extensions: [.go]
    function_pattern: "^func\\s+(\\([^)]+\\)\\s+)?\\w+\\s*\\("
  java:
    extensions: [.java]
    function_pattern: "(public|private|protected)?\\s+\\w+\\s+\\w+\\s*\\("
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

// 1. 定义配置
type MyToolConfig struct {
    SourcePath string
    MaxResults int
    Timeout    time.Duration
}

// 2. 定义输入输出
type MyToolInput struct {
    Query    string   `json:"query"`
    Filters  []string `json:"filters,omitempty"`
}

type MyToolOutput struct {
    Results []Result `json:"results"`
    Total   int      `json:"total"`
}

// 3. 实现Tool
type MyTool struct {
    config *MyToolConfig
}

func NewMyTool(cfg *MyToolConfig) *MyTool {
    return &MyTool{config: cfg}
}

// 4. Info - 描述详细！
func (t *MyTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: "my_tool",
        Desc: `工具详细描述...
        
使用场景：
- 场景1: xxx
- 场景2: yyy`,
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "query": {Type: schema.String, Desc: "查询", Required: true},
        }),
    }, nil
}

// 5. InvokableRun
func (t *MyTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
    var input MyToolInput
    if err := json.Unmarshal([]byte(args), &input); err != nil {
        return "", fmt.Errorf("invalid input: %w", err)
    }
    
    output := t.execute(ctx, &input)
    result, _ := json.Marshal(output)
    return string(result), nil
}

// 6. 编译检查
var _ tool.InvokableTool = (*MyTool)(nil)
```

### 5.3 LLM提供商适配模板

```go
// skill: llm_provider_template
package llm

// OpenAI兼容接口适配器
type OpenAIProvider struct {
    config     *OpenAIConfig
    httpClient *http.Client
}

type OpenAIConfig struct {
    BaseURL    string        // https://api.deepseek.com/v1
    APIKey     string
    Model      string        // deepseek-chat
    Timeout    time.Duration
    MaxRetries int
}

// 支持的Provider
var Providers = map[string]OpenAIConfig{
    "deepseek": {BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-chat"},
    "openai":   {BaseURL: "https://api.openai.com/v1", Model: "gpt-4-turbo"},
    "moonshot": {BaseURL: "https://api.moonshot.cn/v1", Model: "moonshot-v1-32k"},
    "qwen":     {BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Model: "qwen-turbo"},
}

// 自动选择可用Provider
func CreateProvider() (*OpenAIProvider, error) {
    for name, cfg := range Providers {
        envKey := strings.ToUpper(name) + "_API_KEY"
        if apiKey := os.Getenv(envKey); apiKey != "" {
            cfg.APIKey = apiKey
            return NewOpenAIProvider(&cfg)
        }
    }
    return nil, errors.New("no API key found")
}
```

### 5.4 缓存系统模板

```go
// skill: cache_system_template
package cache

// 通用缓存接口
type Cache interface {
    Get(ctx context.Context, key string) (interface{}, bool)
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration)
    Delete(ctx context.Context, key string)
    Clear(ctx context.Context)
    Stats() *CacheStats
}

// LRU缓存配置
type LRUCacheConfig struct {
    MaxSize   int           // 最大条目数
    OnEvict   func(k, v interface{}) // 淘汰回调
}

// 多层缓存
type MultiLayerCache struct {
    L1 Cache  // 内存(快)
    L2 Cache  // 分片(并发)
    L3 Cache  // 持久化(慢但大)
}

// 查询缓存（带参数哈希）
type QueryCache struct {
    cache Cache
}

func (c *QueryCache) GetQuery(ctx context.Context, query string, params map[string]interface{}) (interface{}, bool) {
    key := c.hashKey(query, params)
    return c.cache.Get(ctx, key)
}
```

## 六、最佳实践清单

### 6.1 开发阶段
- [x] 使用Eino ADK的Supervisor模式组织Agent层级
- [x] 工具描述详细，包含使用场景和示例
- [x] 索引存储使用SQLite(嵌入式)，避免外部依赖
- [x] FTS5等高级特性做可选降级处理
- [x] 共享类型提取到独立包，避免循环依赖

### 6.2 测试阶段
- [x] 集成测试覆盖：Storage、Cache、Tools、LLM
- [x] 使用环境变量配置测试路径，适配不同环境
- [x] LLM测试使用真实API验证（DeepSeek性价比高）
- [x] 浮点数比较使用范围而非精确值

### 6.3 部署阶段
- [x] Docker镜像包含运行时依赖(ripgrep, ctags)
- [x] CGO_ENABLED=1编译以支持SQLite
- [x] 配置文件支持环境变量覆盖
- [x] 提供docker-compose和Helm Chart

### 6.4 运维阶段
- [x] Token使用统计和成本监控
- [x] 缓存命中率监控（目标>60%）
- [x] 请求响应时间告警
- [x] 索引支持增量更新

## 七、项目文件清单

```
vdocsMysql/
├── agent/                    # Agent核心
│   ├── master.go            # MasterAgent (Supervisor)
│   └── skill_backend.go     # MySQL特定Skills
├── cache/                    # 缓存系统
│   ├── cache.go             # 多层缓存实现
│   └── cache_test.go        # 缓存单元测试
├── cmd/                      # 入口
│   ├── main.go              # CLI入口
│   └── test_integration/    # 集成测试
├── config/                   # 配置
│   ├── config.go            # 配置结构
│   └── config-dev.yaml      # 开发配置
├── design/                   # 设计文档
│   ├── architecture.md      # 架构设计
│   ├── modules.md           # 模块设计
│   └── ...
├── index/                    # 索引系统
│   └── indexer.go           # 索引构建器
├── llm/                      # LLM接口
│   ├── interface.go         # 通用接口
│   ├── openai.go           # OpenAI兼容实现
│   ├── chinese_providers.go # 国产大模型
│   └── router.go            # 模型路由
├── simulation/               # 负载模拟
│   ├── model.go             # 模型定义
│   └── simulator.go         # 模拟器
├── stats/                    # 统计模块
│   ├── stats.go             # 统计收集
│   └── stats_test.go        # 统计测试
├── storage/                  # 存储层
│   ├── interface.go         # 存储接口
│   ├── sqlite.go            # SQLite实现
│   ├── boltdb.go           # BoltDB实现
│   └── sqlite_test.go       # 存储测试
├── tools/                    # 工具实现
│   ├── grep.go              # ripgrep搜索
│   └── symbol.go            # ctags符号
├── skills/                   # Skills文档
│   └── mysql_expert_agent.md # 本文档
├── deploy/                   # 部署配置
│   ├── k8s/                 # K8s manifests
│   └── helm/                # Helm Chart
├── Dockerfile               # 容器镜像
├── docker-compose.yml       # 组合部署
├── Makefile                 # 构建脚本
├── go.mod
└── README.md
```

## 八、快速启动

### 本地开发
```bash
cd vdocsMysql

# 编译
go build -o bin/mysql-expert ./cmd/main.go
go build -o bin/test_integration ./cmd/test_integration/main.go

# 运行集成测试
DEEPSEEK_API_KEY="your-key" ./bin/test_integration

# 运行主程序
./bin/mysql-expert -q "mysql_parse函数的调用链是什么?"
```

### Docker部署
```bash
# 构建镜像
docker build -f vdocsMysql/Dockerfile -t mysql-expert-agent:latest .

# 运行集成测试
docker run --rm \
  -e DEEPSEEK_API_KEY="your-key" \
  -e DATA_DIR="/data" \
  -e SOURCE_PATH="/data/source" \
  -v /path/to/percona-server:/data/source:ro \
  --entrypoint /app/test_integration \
  mysql-expert-agent:latest

# 运行主程序
docker run --rm \
  -e DEEPSEEK_API_KEY="your-key" \
  -v /path/to/percona-server:/data/source:ro \
  mysql-expert-agent:latest -q "你的问题"
```

---

**总结**：MySQL内核专家Agent的核心在于：
1. **工具设计** - ripgrep/ctags提供精准代码定位能力
2. **索引系统** - SQLite+BoltDB实现大规模代码库的快速检索
3. **缓存架构** - 多层LRU缓存显著减少token消耗
4. **优雅降级** - FTS5等可选特性不影响核心功能
5. **容器化** - Docker一键部署，环境一致性保证
