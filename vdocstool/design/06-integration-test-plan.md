# 集成测试方案设计

## 1. 测试目标

从各模块的**顶层函数入口**进行集成测试，验证功能可用性：

| 模块 | 顶层入口 | 测试重点 |
|------|---------|---------|
| **Web Search** | `WebSearchTool.handleWebSearch()` | 覆盖所有搜索引擎，验证路由和降级 |
| **Email** | `EmailTool.handleSearchByIntent()` 等 | Mock IMAP/SMTP 服务 |
| **Memory** | `MemoryTool.handleStoreMemory()` 等 | Mock Redis/ES/Milvus/S3/Neo4j |

---

## 2. 测试架构

```
┌─────────────────────────────────────────────────────────────┐
│                     集成测试层                               │
├─────────────────────────────────────────────────────────────┤
│  TestWebSearchTool_Integration                              │
│  TestEmailTool_Integration                                  │
│  TestMemoryTool_Integration                                 │
├─────────────────────────────────────────────────────────────┤
│                     Mock 服务层                              │
├──────────────┬───────────────┬───────────────┬──────────────┤
│  MockHTTP    │   MockIMAP    │   MockRedis   │  MockMilvus  │
│  (搜索引擎)   │   MockSMTP    │   MockES      │  MockNeo4j   │
│              │               │   MockS3      │              │
└──────────────┴───────────────┴───────────────┴──────────────┘
```

---

## 3. Web Search 集成测试

### 3.1 测试场景

| 场景 | 描述 | 预期结果 |
|-----|------|---------|
| **DuckDuckGo 正常搜索** | 默认引擎正常响应 | 返回搜索结果 |
| **Bing 搜索** | 指定 Bing 引擎 | 使用 Bing 返回结果 |
| **Baidu 搜索** | 指定百度引擎 | 使用百度返回结果 |
| **Serper (Google) 搜索** | 指定 Serper 引擎 | 使用 Serper API 返回结果 |
| **自动降级** | 首选引擎失败 | 自动切换备用引擎 |
| **全部引擎失败** | 所有引擎返回错误 | 返回聚合错误 |
| **健康检查** | 检查引擎状态 | 返回各引擎健康状态 |
| **路由策略切换** | 修改路由策略 | 策略生效 |

### 3.2 Mock 设计

```go
// MockSearchEngine 搜索引擎 Mock
type MockSearchEngine struct {
    name       string
    results    []*engines.SearchResult
    shouldFail bool
    latency    time.Duration
}

// MockHTTPServer 模拟各搜索引擎的 HTTP 响应
type MockHTTPServer struct {
    server *httptest.Server
    
    // 各引擎的响应配置
    duckduckgoResponse []byte
    bingResponse       []byte
    baiduResponse      []byte
    serperResponse     []byte
    
    // 错误注入
    failEngine map[string]bool
}
```

### 3.3 测试用例

```go
func TestWebSearchTool_AllEngines(t *testing.T) {
    tests := []struct {
        name           string
        query          string
        preferredEngine string
        expectEngine   string
        expectResults  int
    }{
        {"DuckDuckGo_Default", "golang tutorial", "", "duckduckgo", 10},
        {"Bing_Preferred", "golang tutorial", "bing", "bing", 10},
        {"Baidu_Preferred", "golang 教程", "baidu", "baidu", 10},
        {"Serper_Preferred", "golang tutorial", "serper", "serper", 10},
    }
    // ...
}

func TestWebSearchTool_Fallback(t *testing.T) {
    // 测试自动降级场景
}

func TestWebSearchTool_HealthCheck(t *testing.T) {
    // 测试健康检查功能
}
```

---

## 4. Email 集成测试

### 4.1 测试场景

| 场景 | 描述 | 预期结果 |
|-----|------|---------|
| **意图搜索-发票** | 搜索"最近的发票邮件" | 返回发票相关邮件 |
| **意图搜索-会议** | 搜索"下周的会议邀请" | 返回会议邀请邮件 |
| **邮件分析** | 分析单封邮件的意图 | 返回意图分析结果 |
| **附件查找** | 查找PDF附件 | 返回附件列表 |
| **邮件读取** | 读取邮件详情 | 返回邮件内容 |
| **邮件发送** | 发送测试邮件 | 发送成功 |
| **附件下载** | 下载邮件附件 | 保存附件成功 |

### 4.2 Mock 设计

```go
// MockIMAPServer 模拟 IMAP 服务器
type MockIMAPServer struct {
    listener net.Listener
    
    // 预设邮件数据
    mailboxes map[string][]*MockEmail
    
    // 认证信息
    validUsers map[string]string
}

// MockEmail 模拟邮件
type MockEmail struct {
    UID         uint32
    Subject     string
    From        string
    To          []string
    Date        time.Time
    Body        string
    Attachments []*MockAttachment
    Flags       []string
}

// MockSMTPServer 模拟 SMTP 服务器
type MockSMTPServer struct {
    listener  net.Listener
    sentMails []*SentMail
}
```

### 4.3 测试用例

```go
func TestEmailTool_SearchByIntent(t *testing.T) {
    tests := []struct {
        name          string
        intentQuery   string
        expectIntents []string
        expectCount   int
    }{
        {"发票搜索", "查找最近的发票", []string{"invoice"}, 3},
        {"会议搜索", "下周的会议邀请", []string{"meeting"}, 2},
        {"报销搜索", "需要报销的单据", []string{"invoice", "expense"}, 5},
    }
    // ...
}

func TestEmailTool_AnalyzeEmail(t *testing.T) {
    // 测试邮件分析功能
}

func TestEmailTool_FindAttachments(t *testing.T) {
    // 测试附件查找功能
}
```

---

## 5. Memory 集成测试

### 5.1 测试场景

| 场景 | 描述 | 预期结果 |
|-----|------|---------|
| **存储消息** | 存储用户消息到 L1 | 消息写入 Redis |
| **L1 溢出归档** | L1 超过阈值 | 自动归档到 L2 |
| **上下文检索** | 多层检索上下文 | 聚合 L1/L2/L3 结果 |
| **主题切换** | 切换话题 | L1 归档，创建新胶囊 |
| **主题召回** | 召回历史主题 | 从 L2/L3 加载到 L1 |
| **实体关系查询** | 查询实体图谱 | 返回关系网络 |
| **会话摘要** | 生成会话摘要 | 返回摘要和实体 |
| **手动归档** | 归档到 L3 | 压缩存储到 S3 |

### 5.2 Mock 设计

```go
// MockRedis Mock Redis 客户端
type MockRedis struct {
    data map[string]interface{}
    mu   sync.RWMutex
}

// MockElasticsearch Mock ES 客户端
type MockElasticsearch struct {
    documents map[string]map[string]interface{}
    mu        sync.RWMutex
}

// MockMilvus Mock Milvus 客户端
type MockMilvus struct {
    collections map[string][]*MockVector
    mu          sync.RWMutex
}

// MockS3 Mock S3 客户端
type MockS3 struct {
    buckets map[string]map[string][]byte
    mu      sync.RWMutex
}

// MockNeo4j Mock Neo4j 客户端
type MockNeo4j struct {
    nodes     map[string]*MockNode
    relations []*MockRelation
    mu        sync.RWMutex
}
```

### 5.3 测试用例

```go
func TestMemoryTool_StoreAndRetrieve(t *testing.T) {
    // 测试存储和检索流程
}

func TestMemoryTool_L1Overflow(t *testing.T) {
    // 测试 L1 溢出自动归档
}

func TestMemoryTool_TopicSwitch(t *testing.T) {
    // 测试主题切换
}

func TestMemoryTool_EntityRelations(t *testing.T) {
    // 测试实体关系查询
}
```

---

## 6. 测试目录结构

```
tools/
├── search/
│   ├── search.go
│   └── search_integration_test.go    # 搜索集成测试
├── email/
│   ├── email.go
│   └── email_integration_test.go     # 邮件集成测试
├── memory/
│   ├── memory.go
│   └── memory_integration_test.go    # 记忆集成测试
└── testutil/                         # 测试工具包
    ├── mock_http.go                  # HTTP Mock
    ├── mock_imap.go                  # IMAP Mock
    ├── mock_smtp.go                  # SMTP Mock
    ├── mock_redis.go                 # Redis Mock
    ├── mock_elasticsearch.go         # ES Mock
    ├── mock_milvus.go                # Milvus Mock
    ├── mock_s3.go                    # S3 Mock
    ├── mock_neo4j.go                 # Neo4j Mock
    └── fixtures/                     # 测试数据
        ├── emails.json
        ├── search_results.json
        └── messages.json
```

---

## 7. 运行方式

```bash
# 运行所有集成测试
go test -v ./tools/... -tags=integration

# 运行单个模块的集成测试
go test -v ./tools/search/... -tags=integration
go test -v ./tools/email/... -tags=integration
go test -v ./tools/memory/... -tags=integration

# 生成覆盖率报告
go test -v -coverprofile=coverage.out ./tools/... -tags=integration
go tool cover -html=coverage.out -o coverage.html
```

---

## 8. CI/CD 集成

```yaml
# .github/workflows/integration-test.yml
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run Integration Tests
        run: |
          go test -v -race -coverprofile=coverage.out \
            ./tools/... -tags=integration
      
      - name: Upload Coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out
```
