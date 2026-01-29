# Eino框架问题排查与解决方案

## 问题分类索引

| 问题类型 | 症状关键词 | 快速跳转 |
|---------|-----------|---------|
| 依赖问题 | `module does not contain package` | [#1 包不存在](#1-框架包路径不存在) |
| 版本问题 | `toolchain upgrade needed` | [#2 Go版本升级](#2-go版本被强制升级) |
| 编译问题 | `undefined` | [#3 字段未定义](#3-配置字段未定义) |
| 运行时问题 | `nil pointer` | [#4 空指针](#4-运行时空指针) |

---

## 1. 框架包路径不存在

### 症状
```
go: github.com/cloudwego/eino/components/model/ollama: 
    module does not contain package
```

### 原因分析
1. Eino框架版本迭代，包路径变更
2. 某些provider包被移到独立仓库
3. 使用了框架未发布的包

### 解决方案

**方案A：检查框架源码确认路径**
```bash
# 查看框架实际包结构
go list -m -json github.com/cloudwego/eino@v0.7.28 | jq .Dir
cd $DIR && find . -name "*.go" -type f | head -20
```

**方案B：自行封装HTTP客户端**
```go
// 不依赖框架provider，直接调用API
type DeepSeekClient struct {
    apiKey  string
    baseURL string
    client  *http.Client
}

func (c *DeepSeekClient) GenerateText(ctx context.Context, prompt string) (string, error) {
    reqBody := map[string]interface{}{
        "model":    "deepseek-chat",
        "messages": []map[string]string{{"role": "user", "content": prompt}},
    }
    // 直接HTTP调用...
}
```

**方案C：使用框架基础接口**
```go
// 只使用框架的基础schema和tool接口
import (
    "github.com/cloudwego/eino/schema"
    "github.com/cloudwego/eino/components/tool"
)
// 避免使用 components/model/* 等可能变更的包
```

---

## 2. Go版本被强制升级

### 症状
```
go: toolchain upgrade needed to resolve go.etcd.io/bbolt
go.etcd.io/bbolt@v1.4.3 requires go >= 1.23; switching to go1.24.12
```

### 原因分析
依赖包（如bbolt v1.4.3）要求更高Go版本

### 解决方案

**方案A：接受升级（推荐）**
```go
// go.mod
go 1.23

toolchain go1.24.12
```

**方案B：锁定低版本依赖**
```go
// go.mod
require (
    go.etcd.io/bbolt v1.3.9  // 使用兼容Go 1.21的旧版本
)
```

**方案C：替换依赖**
```go
// 如果bbolt不是核心依赖，可以用其他库替代
// 例如用 sqlite 替代 bbolt 做KV存储
```

---

## 3. 配置字段未定义

### 症状
```
cfg.DefaultModel undefined (type *config.LLMConfig has no field DefaultModel)
cfg.APIKeys undefined (type *config.LLMConfig has no field APIKeys)
```

### 原因分析
配置结构变更后，引用代码未同步更新

### 解决方案

**步骤1：检查配置结构定义**
```go
// config/config.go
type LLMConfig struct {
    DeepSeek DeepSeekConfig `yaml:"deepseek"`
    // 没有 DefaultModel 和 APIKeys 字段！
}

type DeepSeekConfig struct {
    Enabled   bool   `yaml:"enabled"`
    APIKeyEnv string `yaml:"api_key_env"`  // 通过环境变量
    Model     string `yaml:"model"`
}
```

**步骤2：修正引用代码**
```go
// llm/manager.go - 修正后
func NewLLMManager(cfg *config.LLMConfig) (*LLMManager, error) {
    // 错误：cfg.DefaultModel
    // 正确：cfg.DeepSeek.Model
    model := cfg.DeepSeek.Model
    
    // 错误：cfg.APIKeys["deepseek"]
    // 正确：os.Getenv(cfg.DeepSeek.APIKeyEnv)
    apiKey := os.Getenv(cfg.DeepSeek.APIKeyEnv)
}
```

**预防措施**
```go
// 使用常量避免硬编码
const (
    DefaultModel = "deepseek-chat"
    DefaultMaxTokens = 4096
)

// 配置验证
func (c *LLMConfig) Validate() error {
    if c.DeepSeek.Enabled && c.DeepSeek.Model == "" {
        return fmt.Errorf("deepseek model is required when enabled")
    }
    return nil
}
```

---

## 4. 运行时空指针

### 症状
```
panic: runtime error: invalid memory address or nil pointer dereference
```

### 常见原因

**4.1 未初始化的map**
```go
// 错误
type Agent struct {
    tools map[string]Tool  // nil map
}
func (a *Agent) RegisterTool(name string, t Tool) {
    a.tools[name] = t  // panic!
}

// 正确
func NewAgent() *Agent {
    return &Agent{
        tools: make(map[string]Tool),
    }
}
```

**4.2 未检查返回值**
```go
// 错误
client, _ := NewDeepSeekClient(apiKey, model)
client.GenerateText(ctx, prompt)  // client可能为nil

// 正确
client, err := NewDeepSeekClient(apiKey, model)
if err != nil {
    return nil, fmt.Errorf("create client failed: %w", err)
}
```

**4.3 配置未加载**
```go
// 错误：配置可能加载失败
cfg, _ := config.Load("config.yaml")
llmManager, _ := llm.NewLLMManager(&cfg.LLM)

// 正确
cfg, err := config.Load("config.yaml")
if err != nil {
    log.Fatalf("load config failed: %v", err)
}
```

---

## 5. 接口实现不完整

### 症状
```
cannot use &GrammarAgent{} (type *GrammarAgent) as type Agent in return statement:
    *GrammarAgent does not implement Agent (missing Capabilities method)
```

### 解决方案

**使用IDE辅助**
```go
// 在结构体定义处添加编译期检查
var _ Agent = (*GrammarAgent)(nil)

// IDE会提示缺少的方法
```

**实现缺失方法**
```go
func (a *GrammarAgent) Capabilities() []string {
    return []string{
        "grammar_check",
        "punctuation_fix",
        "typo_correction",
    }
}
```

---

## 6. 循环依赖

### 症状
```
import cycle not allowed
package github.com/cloudwego/eino/vdocswx/agent
    imports github.com/cloudwego/eino/vdocswx/llm
    imports github.com/cloudwego/eino/vdocswx/agent
```

### 解决方案

**方案A：接口抽象**
```go
// 在独立包定义接口
// types/interfaces.go
package types

type LLMClient interface {
    GenerateText(ctx context.Context, prompt string) (string, error)
}

// agent包依赖接口
// llm包实现接口
// 两者都依赖types包
```

**方案B：依赖注入**
```go
// agent不直接import llm包
type Agent struct {
    llmClient LLMClient  // 接口类型
}

// 在main中注入
agent := NewAgent(llmManager)
```

---

## 7. CGO编译问题

### 症状
```
# github.com/mattn/go-sqlite3
cgo: C compiler "gcc" not found
```

### 解决方案

**macOS**
```bash
xcode-select --install
```

**Linux**
```bash
apt-get install build-essential
```

**禁用CGO（如果可能）**
```bash
CGO_ENABLED=0 go build -o app ./cmd/main.go
# 注意：sqlite3需要CGO，此方案不适用
```

**Docker中解决**
```dockerfile
FROM golang:1.23-alpine
RUN apk add --no-cache gcc musl-dev sqlite-dev
```

---

## 8. 调试技巧

### 8.1 依赖分析
```bash
# 查看为什么引入某个依赖
go mod why github.com/some/package

# 查看依赖图
go mod graph | grep some-package
```

### 8.2 编译详情
```bash
# 显示编译过程
go build -v ./...

# 显示链接详情
go build -ldflags="-v" ./cmd/main.go
```

### 8.3 运行时调试
```go
// 添加调试日志
import "log"

func (m *LLMManager) GetChatModel(ctx context.Context, intentType string) {
    log.Printf("[DEBUG] GetChatModel: intentType=%s, deepseekClient=%v", 
        intentType, m.deepseekClient != nil)
}
```

---

## 9. 常用命令速查

```bash
# 清理并重新下载依赖
go clean -modcache
go mod download

# 整理依赖
go mod tidy

# 检查依赖更新
go list -u -m all

# 编译所有包（检查语法）
go build ./...

# 运行所有测试
go test -v ./...

# 查看包文档
go doc github.com/cloudwego/eino/schema.Message
```

---

## 10. 推荐的开发流程

```
1. 定义接口 → 2. 实现核心逻辑 → 3. 编译验证 → 4. 单元测试 → 5. 集成测试
      ↑                                    |
      └────────────── 发现问题后修复 ←──────┘
```

**关键原则**：
- 每完成一个模块就编译
- 使用 `var _ Interface = (*Impl)(nil)` 编译期检查接口实现
- 错误必须处理，不用 `_` 忽略
- 配置加载失败应该 fatal，不应该继续运行
