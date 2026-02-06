# Memory REST API 设计文档

## 1. 概述

### 1.1 背景
当前 Memory 工具仅支持 MCP 协议，面向 AI Agent 设计。但在实际场景中，非 AI 服务（如 Web 应用、微服务、移动端）也需要存储和检索上下文信息。

### 1.2 目标
在保持 MCP 协议不变的基础上，增加 REST API 和 SDK 支持，让各类服务都能方便地使用 Memory 系统。

## 2. 系统架构

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Memory Service Gateway                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐             │
│  │   AI Agent     │  │   Web App      │  │  Microservice  │             │
│  │   (MCP)        │  │   (REST)       │  │   (gRPC)       │             │
│  └───────┬────────┘  └───────┬────────┘  └───────┬────────┘             │
│          │                   │                   │                       │
│          ▼                   ▼                   ▼                       │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                      Protocol Adapters                           │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │   │
│  │  │ MCP Handler  │  │ REST Handler │  │ gRPC Handler │           │   │
│  │  │ :8080/mcp    │  │ :8080/api/v1 │  │ :9090        │           │   │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘           │   │
│  │         └─────────────────┼─────────────────┘                    │   │
│  │                           ▼                                      │   │
│  │  ┌─────────────────────────────────────────────────────────────┐│   │
│  │  │              Unified Memory Interface                        ││   │
│  │  │                                                              ││   │
│  │  │  StoreMemory()  RetrieveContext()  SwitchTopic()  ...       ││   │
│  │  └──────────────────────────┬──────────────────────────────────┘│   │
│  └─────────────────────────────┼───────────────────────────────────┘   │
│                                │                                        │
│                                ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                       Memory Core                                │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐    │   │
│  │  │   L1    │  │   L2    │  │   L3    │  │ Retrieval       │    │   │
│  │  │ Working │  │ Short-  │  │ Long-   │  │ Pipeline        │    │   │
│  │  │ Memory  │  │ Term    │  │ Term    │  │                 │    │   │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘    │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

## 3. REST API 设计

### 3.1 API 概览

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/memory/store` | 存储记忆 |
| POST | `/api/v1/memory/retrieve` | 检索上下文 |
| POST | `/api/v1/memory/batch/store` | 批量存储 |
| POST | `/api/v1/memory/batch/retrieve` | 批量检索 |
| GET | `/api/v1/memory/session/{session_id}` | 获取会话信息 |
| DELETE | `/api/v1/memory/session/{session_id}` | 删除会话 |
| POST | `/api/v1/memory/topic/switch` | 切换主题 |
| POST | `/api/v1/memory/topic/recall` | 召回主题 |
| GET | `/api/v1/memory/topics/{session_id}` | 列出会话主题 |
| GET | `/api/v1/memory/entities/{entity_name}` | 查询实体关系 |
| POST | `/api/v1/memory/summarize` | 生成摘要 |
| POST | `/api/v1/memory/archive` | 手动归档 |
| GET | `/api/v1/memory/stats` | 获取统计信息 |
| GET | `/api/v1/health` | 健康检查 |

### 3.2 API 详细定义

#### 3.2.1 存储记忆

```http
POST /api/v1/memory/store
Content-Type: application/json
Authorization: Bearer {api_key}

{
    "session_id": "web-user-12345",
    "source": "web_app",
    "message": {
        "role": "user",
        "content": "用户浏览了商品页面",
        "topic_id": "shopping-session-001"
    },
    "metadata": {
        "user_id": "u_123",
        "client_ip": "192.168.1.1",
        "user_agent": "Mozilla/5.0...",
        "page_url": "/products/item-123",
        "custom": {
            "item_id": "SKU-001",
            "category": "electronics"
        }
    },
    "options": {
        "extract_entities": true,
        "generate_embedding": true,
        "importance": 0.7
    }
}
```

**响应：**

```json
{
    "success": true,
    "data": {
        "message_id": "msg_abc123",
        "tier": "L1",
        "token_count": 15,
        "entities_extracted": ["商品页面", "浏览"],
        "archive_triggered": false
    },
    "meta": {
        "request_id": "req_xyz789",
        "latency_ms": 45
    }
}
```

#### 3.2.2 检索上下文

```http
POST /api/v1/memory/retrieve
Content-Type: application/json
Authorization: Bearer {api_key}

{
    "session_id": "web-user-12345",
    "query": "用户之前看过什么商品",
    "options": {
        "token_budget": 2000,
        "topic_id": "shopping-session-001",
        "time_range": {
            "start": "2026-02-01T00:00:00Z",
            "end": "2026-02-05T23:59:59Z"
        },
        "filters": {
            "roles": ["user", "assistant"],
            "sources": ["web_app", "mobile_app"],
            "metadata.category": "electronics"
        },
        "include_summary": true,
        "include_entities": true
    }
}
```

**响应：**

```json
{
    "success": true,
    "data": {
        "context": [
            {
                "message_id": "msg_001",
                "role": "user",
                "content": "用户浏览了商品页面",
                "timestamp": "2026-02-05T10:30:00Z",
                "relevance_score": 0.92,
                "tier": "L1"
            }
        ],
        "summary": "用户在最近一小时内浏览了3个电子产品...",
        "entities": [
            {"name": "iPhone 15", "type": "product", "mentions": 3}
        ],
        "total_tokens": 450,
        "search_stats": {
            "l1_hits": 5,
            "l2_hits": 2,
            "l3_hits": 0,
            "search_latency_ms": 120
        }
    }
}
```

#### 3.2.3 批量存储

```http
POST /api/v1/memory/batch/store
Content-Type: application/json

{
    "session_id": "web-user-12345",
    "messages": [
        {
            "role": "user",
            "content": "消息1",
            "timestamp": "2026-02-05T10:30:00Z"
        },
        {
            "role": "assistant", 
            "content": "消息2",
            "timestamp": "2026-02-05T10:30:05Z"
        }
    ],
    "options": {
        "preserve_order": true,
        "parallel_processing": true
    }
}
```

#### 3.2.4 批量检索

```http
POST /api/v1/memory/batch/retrieve
Content-Type: application/json

{
    "requests": [
        {
            "session_id": "session-1",
            "query": "查询1"
        },
        {
            "session_id": "session-2",
            "query": "查询2"
        }
    ],
    "options": {
        "parallel": true,
        "timeout_ms": 5000
    }
}
```

### 3.3 认证与授权

```go
// API Key 认证
type APIKeyAuth struct {
    KeyPrefix string  // "mem_"
    Scopes    []string // ["read", "write", "admin"]
}

// 示例 Key 格式
// mem_live_abc123...  (生产环境)
// mem_test_xyz789...  (测试环境)

// 权限级别
const (
    ScopeRead  = "read"   // 只能检索
    ScopeWrite = "write"  // 可以存储和检索
    ScopeAdmin = "admin"  // 可以删除、归档等管理操作
)
```

### 3.4 错误处理

```json
{
    "success": false,
    "error": {
        "code": "QUOTA_EXCEEDED",
        "message": "Monthly storage quota exceeded",
        "details": {
            "current_usage": 10000,
            "limit": 10000,
            "reset_at": "2026-03-01T00:00:00Z"
        }
    },
    "meta": {
        "request_id": "req_xyz789"
    }
}
```

**错误码：**

| 错误码 | HTTP 状态码 | 描述 |
|--------|-------------|------|
| INVALID_REQUEST | 400 | 请求参数无效 |
| UNAUTHORIZED | 401 | 认证失败 |
| FORBIDDEN | 403 | 权限不足 |
| NOT_FOUND | 404 | 资源不存在 |
| QUOTA_EXCEEDED | 429 | 配额超限 |
| INTERNAL_ERROR | 500 | 内部错误 |
| SERVICE_UNAVAILABLE | 503 | 服务不可用 |

## 4. 实现设计

### 4.1 REST Handler

```go
// RESTHandler REST API 处理器
type RESTHandler struct {
    memoryService *MemoryService
    authService   *AuthService
    rateLimiter   *RateLimiter
}

// RegisterRoutes 注册路由
func (h *RESTHandler) RegisterRoutes(router *gin.Engine) {
    api := router.Group("/api/v1")
    
    // 中间件
    api.Use(h.authMiddleware())
    api.Use(h.rateLimitMiddleware())
    api.Use(h.requestIDMiddleware())
    api.Use(h.loggingMiddleware())
    
    // Memory 路由
    memory := api.Group("/memory")
    {
        memory.POST("/store", h.handleStore)
        memory.POST("/retrieve", h.handleRetrieve)
        memory.POST("/batch/store", h.handleBatchStore)
        memory.POST("/batch/retrieve", h.handleBatchRetrieve)
        
        memory.GET("/session/:session_id", h.handleGetSession)
        memory.DELETE("/session/:session_id", h.handleDeleteSession)
        
        memory.POST("/topic/switch", h.handleSwitchTopic)
        memory.POST("/topic/recall", h.handleRecallTopic)
        memory.GET("/topics/:session_id", h.handleListTopics)
        
        memory.GET("/entities/:entity_name", h.handleGetEntities)
        memory.POST("/summarize", h.handleSummarize)
        memory.POST("/archive", h.handleArchive)
        memory.GET("/stats", h.handleGetStats)
    }
    
    // 健康检查
    api.GET("/health", h.handleHealth)
}
```

### 4.2 统一 Memory 接口

```go
// MemoryService 统一内存服务接口
type MemoryService interface {
    // 存储
    Store(ctx context.Context, req *StoreRequest) (*StoreResponse, error)
    BatchStore(ctx context.Context, req *BatchStoreRequest) (*BatchStoreResponse, error)
    
    // 检索
    Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error)
    BatchRetrieve(ctx context.Context, req *BatchRetrieveRequest) (*BatchRetrieveResponse, error)
    
    // 会话管理
    GetSession(ctx context.Context, sessionID string) (*SessionInfo, error)
    DeleteSession(ctx context.Context, sessionID string) error
    
    // 主题管理
    SwitchTopic(ctx context.Context, req *SwitchTopicRequest) (*SwitchTopicResponse, error)
    RecallTopic(ctx context.Context, req *RecallTopicRequest) (*RecallTopicResponse, error)
    ListTopics(ctx context.Context, sessionID string) ([]*TopicInfo, error)
    
    // 实体与摘要
    GetEntityRelations(ctx context.Context, entityName string, depth int) ([]*Relation, error)
    Summarize(ctx context.Context, sessionID string) (*Summary, error)
    
    // 归档
    Archive(ctx context.Context, req *ArchiveRequest) (*ArchiveResponse, error)
    
    // 统计
    GetStats(ctx context.Context) (*Stats, error)
}

// MemoryServiceImpl 实现 (复用现有 MemoryTool 的核心逻辑)
type MemoryServiceImpl struct {
    memoryTool *MemoryTool
}
```

### 4.3 gRPC 支持 (可选)

```protobuf
// memory.proto
syntax = "proto3";
package memory.v1;

service MemoryService {
    rpc Store(StoreRequest) returns (StoreResponse);
    rpc Retrieve(RetrieveRequest) returns (RetrieveResponse);
    rpc BatchStore(BatchStoreRequest) returns (BatchStoreResponse);
    rpc BatchRetrieve(BatchRetrieveRequest) returns (BatchRetrieveResponse);
    // ... 其他方法
}
```

## 5. SDK 设计

### 5.1 Go SDK

```go
// client.go
package memoryclient

type Client struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
    return &Client{
        baseURL:    baseURL,
        apiKey:     apiKey,
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }
}

// Store 存储记忆
func (c *Client) Store(ctx context.Context, req *StoreRequest) (*StoreResponse, error)

// Retrieve 检索上下文
func (c *Client) Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error)

// BatchStore 批量存储
func (c *Client) BatchStore(ctx context.Context, req *BatchStoreRequest) (*BatchStoreResponse, error)

// 使用示例
func Example() {
    client := memoryclient.NewClient("http://localhost:8080", "mem_live_xxx")
    
    // 存储
    _, err := client.Store(ctx, &memoryclient.StoreRequest{
        SessionID: "web-user-123",
        Message: memoryclient.Message{
            Role:    "user",
            Content: "用户提交了订单",
        },
    })
    
    // 检索
    result, _ := client.Retrieve(ctx, &memoryclient.RetrieveRequest{
        SessionID: "web-user-123",
        Query:     "用户最近的订单操作",
    })
}
```

### 5.2 Python SDK

```python
# memory_client.py
from typing import Optional, List, Dict
import httpx

class MemoryClient:
    def __init__(self, base_url: str, api_key: str):
        self.base_url = base_url
        self.api_key = api_key
        self._client = httpx.Client(
            base_url=base_url,
            headers={"Authorization": f"Bearer {api_key}"},
            timeout=30.0
        )
    
    def store(
        self, 
        session_id: str, 
        role: str, 
        content: str,
        topic_id: Optional[str] = None,
        metadata: Optional[Dict] = None
    ) -> Dict:
        """存储记忆"""
        return self._client.post("/api/v1/memory/store", json={
            "session_id": session_id,
            "message": {"role": role, "content": content, "topic_id": topic_id},
            "metadata": metadata
        }).json()
    
    def retrieve(
        self,
        session_id: str,
        query: str,
        token_budget: int = 4000,
        topic_id: Optional[str] = None
    ) -> Dict:
        """检索上下文"""
        return self._client.post("/api/v1/memory/retrieve", json={
            "session_id": session_id,
            "query": query,
            "options": {"token_budget": token_budget, "topic_id": topic_id}
        }).json()

# 使用示例
client = MemoryClient("http://localhost:8080", "mem_live_xxx")
client.store("session-123", "user", "用户查看了购物车")
context = client.retrieve("session-123", "用户最近做了什么")
```

## 6. 配置

```json
{
    "rest_api": {
        "enabled": true,
        "listen_addr": ":8080",
        "base_path": "/api/v1",
        "cors": {
            "allowed_origins": ["*"],
            "allowed_methods": ["GET", "POST", "DELETE"],
            "allowed_headers": ["Authorization", "Content-Type"]
        },
        "rate_limit": {
            "enabled": true,
            "requests_per_minute": 100,
            "burst_size": 20
        },
        "auth": {
            "enabled": true,
            "api_keys_file": "/etc/memory/api_keys.json"
        }
    },
    "grpc": {
        "enabled": false,
        "listen_addr": ":9090"
    }
}
```

## 7. 实现计划

1. **Phase 1**: REST Handler + 基础 API (Store, Retrieve)
2. **Phase 2**: 批量操作 + 会话管理 API
3. **Phase 3**: 认证授权 + 限流
4. **Phase 4**: SDK (Go, Python)
5. **Phase 5**: gRPC 支持 (可选)
