# 专家Agent开发技能库

## 一、项目概述

### 1.1 项目定位
基于Eino框架开发的**微信公众号文章润色专家Agent**，支持：
- 多轮对话交互
- 文章语法检查、逻辑优化、结构调整、风格统一
- 子Agent并发协作
- 多模型路由
- 索引与缓存系统

### 1.2 核心架构

```
┌─────────────────────────────────────────────────────────────┐
│                      API Layer (HTTP/REST)                   │
├─────────────────────────────────────────────────────────────┤
│                      Master Agent                            │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │  Intent     │   Planner   │  Executor   │  Aggregator │  │
│  │  Recognizer │             │             │             │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                      Sub-Agents Layer                        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │ Grammar  │ │  Style   │ │  Logic   │ │Structure │       │
│  │  Agent   │ │  Agent   │ │  Agent   │ │  Agent   │       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘       │
├─────────────────────────────────────────────────────────────┤
│                      Tools Layer                             │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │ Markdown │ │   File   │ │  Index   │ │  Grammar │       │
│  │   Tool   │ │   Tool   │ │   Tool   │ │   Tool   │       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘       │
├─────────────────────────────────────────────────────────────┤
│                    Infrastructure Layer                      │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │   LLM    │ │  Cache   │ │  Index   │ │  Stats   │       │
│  │ Manager  │ │  System  │ │  System  │ │ Collector│       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘       │
└─────────────────────────────────────────────────────────────┘
```

---

## 二、关键设计模式

### 2.1 Master-SubAgent 模式

```go
// 核心接口定义
type Agent interface {
    Name() string
    Type() AgentType
    Execute(ctx context.Context, input *AgentInput) (*AgentOutput, error)
    Capabilities() []string
}

// Master Agent 负责：
// 1. 意图识别 - 判断用户需求类型
// 2. 任务规划 - 分解任务到子Agent
// 3. 并发执行 - 调度子Agent执行
// 4. 结果聚合 - 合并各Agent输出
```

**优点**：
- 职责分离，便于扩展
- 支持并发提升效率
- 子Agent可独立测试和复用

### 2.2 意图识别模式

```go
type IntentType string

const (
    IntentTypeGrammarCheck   IntentType = "grammar_check"
    IntentTypeStyleOptimize  IntentType = "style_optimize"
    IntentTypeLogicOptimize  IntentType = "logic_optimize"
    IntentTypeStructureAdjust IntentType = "structure_adjust"
    IntentTypeFullPolish     IntentType = "full_polish"
)

// 使用LLM进行意图识别
func (ir *IntentRecognizer) RecognizeIntent(ctx context.Context, prompt string) (IntentType, error) {
    // 1. 规则匹配（快速路径）
    // 2. LLM分类（复杂场景）
}
```

### 2.3 工具抽象模式

```go
// 工具定义 - 兼容Eino框架
type Tool interface {
    Info() *schema.ToolInfo
    InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error)
}

// 工具注册与管理
type ToolRegistry struct {
    tools map[string]Tool
}
```

---

## 三、依赖管理经验

### 3.1 Eino框架版本选择

**问题**：Eino框架版本迭代快，不同版本API差异大

**解决方案**：
```go
// go.mod - 推荐使用稳定版本
module github.com/cloudwego/eino/vdocswx

go 1.23

require (
    github.com/cloudwego/eino v0.7.28  // 使用最新稳定版
)
```

**注意事项**：
1. 避免使用 `replace` 指令指向本地路径（除非开发框架本身）
2. 优先使用已发布的tag版本，避免使用 `latest` 或 `rc` 版本
3. 仔细阅读框架的CHANGELOG了解API变更

### 3.2 依赖包不存在问题

**问题**：框架子包可能不存在或路径变更

```
go: github.com/cloudwego/eino/components/model/ollama: 
    module does not contain package
```

**解决方案**：
1. 检查框架源码确认包路径
2. 使用 `go list -m all` 查看实际依赖
3. 必要时自行封装适配层

```go
// 自行封装的客户端适配层
type DeepSeekClient struct {
    apiKey  string
    baseURL string
    model   string
    client  *http.Client
}

func (c *DeepSeekClient) GenerateText(ctx context.Context, prompt string) (string, error) {
    // 直接调用HTTP API，不依赖框架的provider包
}
```

---

## 四、配置设计模式

### 4.1 分层配置结构

```go
type Config struct {
    Paths   PathsConfig   `yaml:"paths"`   // 路径配置
    Index   IndexConfig   `yaml:"index"`   // 索引配置
    LLM     LLMConfig     `yaml:"llm"`     // LLM配置
    Router  RouterConfig  `yaml:"router"`  // 路由配置
    Agent   AgentConfig   `yaml:"agent"`   // Agent配置
    Cache   CacheConfig   `yaml:"cache"`   // 缓存配置
    Stats   StatsConfig   `yaml:"stats"`   // 统计配置
    API     APIConfig     `yaml:"api"`     // API配置
}
```

### 4.2 环境变量与敏感信息

```yaml
# 配置文件中使用环境变量占位符
llm:
  deepseek:
    api_key_env: "DEEPSEEK_API_KEY"  # 通过环境变量注入
```

```go
// 加载时展开环境变量
func Load(path string) (*Config, error) {
    data, _ := os.ReadFile(path)
    data = []byte(os.ExpandEnv(string(data)))  // 展开 ${VAR} 语法
    // ...
}
```

### 4.3 默认值与验证

```go
func setDefaults(config *Config) {
    if config.LLM.DeepSeek.MaxTokens == 0 {
        config.LLM.DeepSeek.MaxTokens = 4096
    }
    // 其他默认值...
}

func validate(config *Config) error {
    // 至少启用一个LLM
    hasLLM := config.LLM.DeepSeek.Enabled || config.LLM.Qwen.Enabled
    if !hasLLM {
        return fmt.Errorf("at least one LLM provider must be enabled")
    }
    return nil
}
```

---

## 五、LLM集成模式

### 5.1 多模型统一管理

```go
type LLMManager struct {
    cfg          *config.LLMConfig
    clients      map[string]LLMClient  // 多模型客户端
    router       *ModelRouter          // 模型路由
}

// 统一接口
type LLMClient interface {
    GenerateText(ctx context.Context, prompt string) (string, error)
    GenerateChat(ctx context.Context, messages []*Message) (*Message, error)
}
```

### 5.2 模型路由策略

```go
type ModelRouter struct {
    strategies map[string]RoutingStrategy
}

// 根据意图类型路由到不同模型
func (r *ModelRouter) RouteModel(intentType string) string {
    switch intentType {
    case "simple_query":
        return "deepseek-chat"      // 简单任务用轻量模型
    case "complex_reasoning":
        return "deepseek-reasoner"  // 复杂推理用强模型
    default:
        return r.defaultModel
    }
}
```

### 5.3 重试与容错

```go
// 带重试的LLM调用
func (m *LLMManager) GenerateWithRetry(ctx context.Context, prompt string) (string, error) {
    var lastErr error
    for i := 0; i < m.cfg.MaxRetries; i++ {
        result, err := m.client.GenerateText(ctx, prompt)
        if err == nil {
            return result, nil
        }
        lastErr = err
        time.Sleep(m.cfg.RetryInterval)
    }
    return "", fmt.Errorf("all retries failed: %w", lastErr)
}
```

---

## 六、缓存设计模式

### 6.1 三级缓存架构

```
┌─────────────────────────────────────────┐
│           L1: Memory (LRU)              │  ← 热数据，毫秒级
├─────────────────────────────────────────┤
│           L2: SQLite (Local)            │  ← 持久化，磁盘级
├─────────────────────────────────────────┤
│           L3: Redis (Optional)          │  ← 分布式，可选
└─────────────────────────────────────────┘
```

### 6.2 缓存键设计

```go
// 索引缓存键
type CacheKey struct {
    Type      string  // "term", "doc", "summary"
    Namespace string  // 命名空间隔离
    Key       string  // 实际键值
}

func (k *CacheKey) String() string {
    return fmt.Sprintf("%s:%s:%s", k.Type, k.Namespace, k.Key)
}
```

### 6.3 LLM响应缓存（语义相似度）

```go
// 基于语义相似度的缓存命中
func (c *LLMCache) Get(prompt string) (string, bool) {
    hash := hashPrompt(prompt)
    if cached, ok := c.exactCache.Get(hash); ok {
        return cached, true
    }
    
    // 语义相似度匹配
    embedding := c.embedder.Embed(prompt)
    similar := c.findSimilar(embedding, c.similarityThreshold)
    if similar != nil {
        return similar.Response, true
    }
    return "", false
}
```

---

## 七、索引系统设计

### 7.1 倒排索引结构

```go
type InvertedIndex struct {
    // term -> document IDs
    termIndex map[string][]int64
    // document ID -> document
    documents map[int64]*Document
}

type Document struct {
    ID       int64
    Content  string
    Metadata map[string]string
    Tokens   []string
}
```

### 7.2 SQLite存储实现

```sql
-- 文档表
CREATE TABLE documents (
    id INTEGER PRIMARY KEY,
    content TEXT NOT NULL,
    metadata TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 倒排索引表
CREATE TABLE inverted_index (
    term TEXT NOT NULL,
    doc_id INTEGER NOT NULL,
    positions TEXT,
    tf REAL,
    PRIMARY KEY (term, doc_id)
);

-- 索引加速
CREATE INDEX idx_term ON inverted_index(term);
```

---

## 八、遇到的问题与解决方案

### 8.1 问题：框架版本不兼容

**症状**：
```
schema.Message undefined
model.ChatModel has no method Generate
```

**原因**：使用了错误的框架版本或本地replace导致版本混乱

**解决**：
1. 移除 `go.mod` 中的 `replace` 指令
2. 明确指定稳定版本号
3. 运行 `go mod tidy` 清理依赖

### 8.2 问题：包路径不存在

**症状**：
```
module does not contain package github.com/cloudwego/eino/components/model/ollama
```

**原因**：框架重构后包路径变更，或包被移除

**解决**：
1. 检查框架最新源码确认包位置
2. 自行封装HTTP客户端替代
3. 使用框架提供的基础接口

### 8.3 问题：配置字段类型不匹配

**症状**：
```
cfg.DefaultModel undefined (type *config.LLMConfig has no field DefaultModel)
```

**原因**：配置结构变更后，引用代码未同步更新

**解决**：
1. 保持配置结构与代码引用一致
2. 使用 IDE 的重构功能批量更新
3. 编写配置验证单元测试

### 8.4 问题：Go版本升级

**症状**：
```
go: toolchain upgrade needed to resolve go.etcd.io/bbolt
go.etcd.io/bbolt@v1.4.3 requires go >= 1.23
```

**原因**：依赖包要求更高Go版本

**解决**：
```go
// go.mod 中明确工具链版本
go 1.23

toolchain go1.24.12
```

---

## 九、最佳实践清单

### 9.1 项目结构

```
project/
├── cmd/
│   └── main.go           # 入口
├── config/
│   └── config.go         # 配置定义
├── agent/
│   ├── interface.go      # 接口定义
│   ├── master/           # 主Agent
│   └── grammar/          # 子Agent
├── llm/
│   ├── manager.go        # LLM管理
│   └── client.go         # 客户端封装
├── tools/
│   ├── file/             # 文件工具
│   └── markdown/         # Markdown工具
├── cache/
│   └── cache.go          # 缓存实现
├── index/
│   └── inverted.go       # 索引实现
├── storage/
│   └── sqlite.go         # 存储实现
├── stats/
│   └── collector.go      # 统计收集
├── api/
│   └── server.go         # HTTP服务
├── skills/               # 技能库
├── config.yaml           # 配置文件
├── Dockerfile
├── Makefile
└── go.mod
```

### 9.2 开发流程

1. **设计先行**：先输出设计文档，明确架构和接口
2. **接口驱动**：定义清晰的接口，便于测试和替换
3. **渐进开发**：先实现核心功能，再逐步完善
4. **持续验证**：每完成一个模块就编译验证
5. **配置外化**：所有可变参数配置化

### 9.3 依赖管理

1. 优先使用稳定版本（非rc、非latest）
2. 定期运行 `go mod tidy` 清理
3. 使用 `go mod why` 分析依赖来源
4. 对不稳定的外部包封装适配层

### 9.4 错误处理

```go
// 统一错误包装
func wrapError(op string, err error) error {
    return fmt.Errorf("%s failed: %w", op, err)
}

// 可恢复错误与致命错误分离
type RecoverableError struct {
    Err     error
    Retry   bool
    Backoff time.Duration
}
```

---

## 十、可复用组件

### 10.1 通用Agent接口

```go
// 可直接复用的Agent接口定义
type Agent interface {
    Name() string
    Type() AgentType
    Execute(ctx context.Context, input *AgentInput) (*AgentOutput, error)
    Capabilities() []string
}

type AgentInput struct {
    SessionID string
    Content   string
    Context   map[string]interface{}
    Options   *AgentOptions
}

type AgentOutput struct {
    Content     string
    Suggestions []string
    Metadata    map[string]interface{}
    TokenUsage  *TokenUsage
}
```

### 10.2 通用缓存接口

```go
type Cache interface {
    Get(ctx context.Context, key string) (interface{}, bool)
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
}
```

### 10.3 通用统计收集器

```go
type StatsCollector struct {
    tokenUsage   atomic.Int64
    cacheHits    atomic.Int64
    cacheMisses  atomic.Int64
    requestCount atomic.Int64
    errorCount   atomic.Int64
}

func (s *StatsCollector) RecordTokenUsage(count int) {
    s.tokenUsage.Add(int64(count))
}

func (s *StatsCollector) GetStats() *Stats {
    return &Stats{
        TokenUsage:   s.tokenUsage.Load(),
        CacheHitRate: float64(s.cacheHits.Load()) / float64(s.cacheHits.Load()+s.cacheMisses.Load()),
        // ...
    }
}
```

---

## 十一、部署检查清单

### 11.1 本地开发

- [ ] Go 1.23+ 已安装
- [ ] 配置文件已创建
- [ ] 环境变量已设置（API Key等）
- [ ] 依赖已下载 (`go mod download`)
- [ ] 编译通过 (`go build`)
- [ ] 单元测试通过 (`go test ./...`)

### 11.2 Docker部署

```dockerfile
# 多阶段构建
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /app/server ./cmd/main.go

FROM alpine:latest
RUN apk add --no-cache sqlite-libs
COPY --from=builder /app/server /app/server
COPY --from=builder /app/config.yaml /app/config.yaml
CMD ["/app/server", "-config", "/app/config.yaml"]
```

### 11.3 K8S部署

```yaml
# 必须配置的Secret
apiVersion: v1
kind: Secret
metadata:
  name: llm-secrets
type: Opaque
stringData:
  DEEPSEEK_API_KEY: "your-api-key"
```

---

## 十二、扩展指南

### 12.1 添加新的子Agent

1. 在 `agent/` 目录创建新包
2. 实现 `Agent` 接口
3. 在 Master Agent 中注册
4. 更新意图识别逻辑

### 12.2 添加新的LLM提供商

1. 在 `llm/` 目录创建新客户端
2. 实现 `LLMClient` 接口
3. 在 `LLMManager` 中注册
4. 更新配置结构和路由逻辑

### 12.3 添加新的Tool

1. 在 `tools/` 目录创建新包
2. 实现 `Tool` 接口
3. 注册到 Tool Registry
4. 在相关 Agent 中使用

---

## 十三、总结

开发专家Agent的核心要点：

1. **架构清晰**：Master-SubAgent 分层，职责明确
2. **接口统一**：所有组件实现标准接口，便于替换和测试
3. **配置驱动**：所有可变参数外部化，支持环境变量
4. **缓存优先**：多级缓存减少LLM调用，降低成本
5. **错误容忍**：重试机制、降级策略、优雅失败
6. **可观测性**：统计收集、日志记录、健康检查
7. **渐进开发**：先跑通核心流程，再完善细节

最重要的经验：**不要过度依赖框架的高级特性**，基础的HTTP调用和接口抽象往往更可靠。
