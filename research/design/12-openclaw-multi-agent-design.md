# 基于 OpenClaw 的多 Agent 协作系统设计方案

## 1. 设计目标

基于对 AutoGen / CrewAI / MetaGPT / LangGraph / Swarm / CAMEL / AgentScope 7 大框架和 4 个学术专题的调研，设计一套在 OpenClaw 平台上运行、通过飞书交互的多 Agent 协作系统。

### 核心目标

- **Agent 间高效通信**：借鉴 MetaGPT 的 Pub-Sub + AutoGen 的 Actor 模型
- **并行任务处理**：借鉴 Swarm 的 Handoff + Kimi Agent Swarm 的并发 subagent
- **统一记忆系统**：借鉴 CrewAI 的统一 Memory + CAMEL 的 LongtermMemory
- **SOP 驱动质量**：借鉴 MetaGPT 的结构化产出 + 可执行反馈
- **飞书原生集成**：利用 OpenClaw 飞书 binding 机制实现路由和隔离
- **Go + TS 技术栈**：后台服务用 Golang，OpenClaw 插件用 TypeScript

### 设计原则

| 原则 | 说明 | 借鉴来源 |
|------|------|---------|
| 松耦合通信 | Agent 间通过消息总线通信，非直接引用 | MetaGPT Pub-Sub |
| 结构化产出 | 所有交付物有明确 schema，非自然语言闲聊 | MetaGPT SOP |
| 并行优先 | 无依赖任务默认并行执行 | Swarm / AgentScope |
| 渐进增强 | 核心能力用 OpenClaw 原生，高级能力用插件扩展 | AgentScope 分层 |
| 故障降级 | 模型/API 不可用时自动降级，不影响整体 | AgentScope 容错 |

---

## 2. 整体架构

```mermaid
graph TB
    subgraph 飞书["飞书 (Feishu)"]
        FUser[用户 Victor]
        FGroup[项目群]
        FDM[私聊]
    end

    subgraph OpenClaw["OpenClaw Gateway"]
        Router[路由引擎<br/>bindings + mentionPatterns]
        SessionMgr[Session Manager<br/>dmScope 用户隔离]
    end

    subgraph AgentLayer["Agent 层"]
        RDM[🎯 RD_MANAGER<br/>Coordinator]
        ARCH[🏗️ ARCHITECT<br/>Designer]
        DEV[💻 DEV_MANAGER<br/>Developer]
        TEST[✅ TEST_MANAGER<br/>QA]
        AUDIT[🔒 CODE_AUDITOR<br/>Auditor]
    end

    subgraph MessageBus["消息总线 (TS Plugin)"]
        PubSub[Pub-Sub Engine]
        SharedState[Shared State Store]
        TaskQueue[Task Queue]
    end

    subgraph MemoryLayer["记忆层"]
        ShortTerm[短期记忆<br/>Session Context]
        LongTerm[长期记忆<br/>Vector Store]
        SharedMem[共享记忆<br/>项目知识库]
    end

    subgraph Backend["Go 后台服务"]
        TaskSvc[Task Service<br/>任务管理]
        MemSvc[Memory Service<br/>记忆管理]
        MonitorSvc[Monitor Service<br/>监控告警]
    end

    FUser --> Router
    FGroup --> Router
    FDM --> Router
    Router --> SessionMgr
    SessionMgr --> AgentLayer

    RDM <--> PubSub
    ARCH <--> PubSub
    DEV <--> PubSub
    TEST <--> PubSub
    AUDIT <--> PubSub

    PubSub <--> SharedState
    PubSub <--> TaskQueue

    AgentLayer <--> ShortTerm
    AgentLayer <--> LongTerm
    AgentLayer <--> SharedMem

    TaskQueue <--> TaskSvc
    LongTerm <--> MemSvc
    MonitorSvc --> AgentLayer
```

### 架构分层说明

| 层 | 技术 | 职责 |
|----|------|------|
| **飞书接入层** | OpenClaw Feishu Plugin | 接收消息、路由到 Agent、回复结果 |
| **Agent 层** | OpenClaw Agent Workspaces | 5 个独立 Agent，各有 SYSTEM_PROMPT + SOUL + AGENTS |
| **消息总线** | TS Plugin (`@team/message-bus`) | Agent 间 Pub-Sub 通信 + 共享状态 + 任务队列 |
| **记忆层** | TS Plugin (`@team/memory-store`) + Go Service | 短期/长期/共享三层记忆 |
| **Go 后台** | Golang 微服务 | 任务持久化、向量存储、监控告警 |

---

## 3. 通信机制设计

### 3.1 通信模式选择

借鉴调研结论，采用**混合通信模式**：

```mermaid
graph LR
    subgraph 通信模式
        A[sessions_send<br/>直接消息] -->|点对点任务分配| M((混合))
        B[Pub-Sub<br/>消息总线] -->|广播通知/状态更新| M
        C[Shared State<br/>共享状态] -->|项目上下文/交付物| M
    end
```

| 模式 | 适用场景 | 实现方式 | 借鉴来源 |
|------|---------|---------|---------|
| **直接消息** | 任务分配、进度汇报、Bug 反馈 | `sessions_send` (OpenClaw 原生) | AutoGen Direct |
| **Pub-Sub** | 状态变更广播、全员通知、事件驱动 | TS Plugin `message-bus` | MetaGPT / AutoGen |
| **共享状态** | 项目文档、设计稿、代码产出 | TS Plugin `shared-state` | LangGraph State |

### 3.2 消息协议

```typescript
// message-bus TS Plugin 消息格式
interface AgentMessage {
  id: string;                    // UUID
  timestamp: number;             // Unix ms
  from: AgentId;                 // 发送者 {name, emoji, prefix}
  to: AgentId | 'broadcast';    // 接收者或广播
  type: MessageType;             // 'task' | 'report' | 'alert' | 'handoff' | 'query'
  topic: string;                 // Pub-Sub topic (e.g., 'project.alpha.design')
  payload: {
    content: string;             // 消息正文
    structured?: object;         // 结构化数据 (如设计文档JSON)
    attachments?: string[];      // 附件路径
    priority: 'P0' | 'P1' | 'P2' | 'P3';
    replyTo?: string;            // 回复的消息 ID
  };
  metadata: {
    sessionId: string;           // 会话隔离
    projectId: string;           // 项目标识
    taskId?: string;             // 关联任务
  };
}
```

### 3.3 通信流程

```mermaid
sequenceDiagram
    participant User as Victor (飞书)
    participant GW as OpenClaw Gateway
    participant RDM as 🎯 RD_MANAGER
    participant BUS as Message Bus
    participant ARCH as 🏗️ ARCHITECT
    participant DEV as 💻 DEV_MANAGER

    User->>GW: @研发经理 开发用户登录模块
    GW->>RDM: 路由到 RD_MANAGER

    Note over RDM: [思考] 评估→决策→行动

    RDM->>BUS: publish(topic: "project.login", type: "task_created")
    RDM->>ARCH: sessions_send(设计任务)
    BUS->>DEV: subscribe("project.login") 通知有新项目

    ARCH->>ARCH: [思考] 分析→对比→选择
    ARCH->>BUS: publish(topic: "project.login.design", payload: 设计文档)
    ARCH->>RDM: sessions_send(设计完成报告)

    RDM->>DEV: sessions_send(开发任务 + 设计文档引用)
    DEV->>DEV: [思考] 理解→规划→实现→验证
    DEV->>BUS: publish(topic: "project.login.code", payload: 代码产出)
    DEV->>RDM: sessions_send(开发完成报告)
```

---

## 4. 任务编排设计

### 4.1 编排模式

借鉴调研，采用 **Coordinator-Worker + Subagent Swarm** 混合模式：

```mermaid
graph TB
    subgraph Coordinator["协调者模式 (RD_MANAGER)"]
        RDM[🎯 RD_MANAGER<br/>任务拆解+分配+验收]
        RDM -->|设计任务| ARCH[🏗️ ARCHITECT]
        RDM -->|开发任务| DEV[💻 DEV_MANAGER]
        RDM -->|测试任务| TEST[✅ TEST_MANAGER]
        RDM -->|审计任务| AUDIT[🔒 CODE_AUDITOR]
    end

    subgraph Swarm["Subagent Swarm (并行执行)"]
        DEV -->|sessions_spawn| S1[Sub-Dev-1<br/>模块A]
        DEV -->|sessions_spawn| S2[Sub-Dev-2<br/>模块B]
        DEV -->|sessions_spawn| S3[Sub-Dev-3<br/>模块C]
    end

    subgraph Pipeline["SOP 流水线"]
        D1[需求分析] --> D2[架构设计]
        D2 --> D3[开发实现]
        D3 --> D4[代码审计]
        D3 --> D5[功能测试]
        D4 --> D6[验收交付]
        D5 --> D6
    end
```

### 4.2 任务状态机

```mermaid
stateDiagram-v2
    [*] --> Created: 任务创建
    Created --> Assigned: RD_MANAGER 分配
    Assigned --> InProgress: Agent 开始执行
    InProgress --> Review: 提交评审
    InProgress --> Blocked: 遇到阻塞
    Blocked --> InProgress: 阻塞解除
    Review --> Rejected: 质量不达标
    Rejected --> InProgress: 修改重做
    Review --> Completed: 验收通过
    Completed --> [*]
```

### 4.3 Go 后台 — Task Service

```go
// task_service.go

package task

import (
    "context"
    "time"
)

type TaskStatus string

const (
    StatusCreated    TaskStatus = "created"
    StatusAssigned   TaskStatus = "assigned"
    StatusInProgress TaskStatus = "in_progress"
    StatusBlocked    TaskStatus = "blocked"
    StatusReview     TaskStatus = "review"
    StatusRejected   TaskStatus = "rejected"
    StatusCompleted  TaskStatus = "completed"
)

type Task struct {
    ID          string            `json:"id"`
    ProjectID   string            `json:"project_id"`
    Title       string            `json:"title"`
    Description string            `json:"description"`
    Assignee    string            `json:"assignee"`     // Agent ID
    AssignedBy  string            `json:"assigned_by"`  // Coordinator Agent ID
    Status      TaskStatus        `json:"status"`
    Priority    string            `json:"priority"`     // P0-P3
    Deliverable string            `json:"deliverable"`  // 交付物要求
    Acceptance  string            `json:"acceptance"`   // 验收标准
    ParentID    *string           `json:"parent_id"`    // 父任务 (Swarm 场景)
    SubTasks    []string          `json:"sub_tasks"`    // 子任务 IDs
    Checkpoint  time.Time         `json:"checkpoint"`   // 检查点时间
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
    Metadata    map[string]string `json:"metadata"`
}

type TaskService interface {
    Create(ctx context.Context, task *Task) error
    UpdateStatus(ctx context.Context, id string, status TaskStatus, comment string) error
    GetByProject(ctx context.Context, projectID string) ([]*Task, error)
    GetByAssignee(ctx context.Context, agentID string) ([]*Task, error)
    GetOverdue(ctx context.Context) ([]*Task, error) // 超期未完成
    Decompose(ctx context.Context, parentID string, subTasks []*Task) error // 拆解子任务
    Aggregate(ctx context.Context, parentID string) (*TaskResult, error)    // 汇总子任务结果
}

type TaskResult struct {
    TaskID      string        `json:"task_id"`
    Status      TaskStatus    `json:"status"`
    Deliverables []string     `json:"deliverables"`
    SubResults  []*TaskResult `json:"sub_results"` // Swarm 子任务结果
    Duration    time.Duration `json:"duration"`
    QualityScore float64      `json:"quality_score"`
}
```

### 4.4 TS Plugin — Task Queue

```typescript
// @team/task-queue plugin

import { OpenClawPlugin, PluginContext } from '@openclaw/sdk';

interface TaskEvent {
  type: 'created' | 'assigned' | 'completed' | 'blocked' | 'overdue';
  taskId: string;
  agentId: string;
  payload: any;
}

export default class TaskQueuePlugin implements OpenClawPlugin {
  private queue: TaskEvent[] = [];
  private subscribers: Map<string, ((event: TaskEvent) => void)[]> = new Map();

  async onMessage(ctx: PluginContext, message: any) {
    // 监听任务相关消息，维护队列
    if (message.type === 'task') {
      const event: TaskEvent = {
        type: 'created',
        taskId: message.payload.taskId,
        agentId: message.from.name,
        payload: message.payload,
      };
      this.queue.push(event);
      this.notify(event);
    }
  }

  subscribe(agentId: string, callback: (event: TaskEvent) => void) {
    if (!this.subscribers.has(agentId)) {
      this.subscribers.set(agentId, []);
    }
    this.subscribers.get(agentId)!.push(callback);
  }

  private notify(event: TaskEvent) {
    for (const [agentId, callbacks] of this.subscribers) {
      callbacks.forEach(cb => cb(event));
    }
  }
}
```

---

## 5. 记忆系统设计

### 5.1 三层记忆架构

借鉴 CoALA + CrewAI + CAMEL 的记忆设计：

```mermaid
graph TB
    subgraph Agent["Agent 运行时"]
        WM[工作记忆<br/>当前对话上下文]
    end

    subgraph ShortTerm["短期记忆"]
        Session[Session Context<br/>本次会话历史]
        TaskCtx[Task Context<br/>当前任务上下文]
    end

    subgraph LongTerm["长期记忆 (向量存储)"]
        Episodic[情景记忆<br/>历史对话/决策]
        Semantic[语义记忆<br/>知识/规则/偏好]
        Procedural[程序记忆<br/>技能/模式/SOP]
    end

    subgraph Shared["共享记忆 (项目级)"]
        ProjectKB[项目知识库<br/>设计文档/代码/测试报告]
        TeamExp[团队经验<br/>最佳实践/踩坑记录]
    end

    WM --> ShortTerm
    ShortTerm --> LongTerm
    LongTerm --> Shared

    style WM fill:#e3f2fd
    style ShortTerm fill:#fff3e0
    style LongTerm fill:#e8f5e9
    style Shared fill:#fce4ec
```

### 5.2 Go 后台 — Memory Service

```go
// memory_service.go

package memory

import (
    "context"
)

type MemoryType string

const (
    Episodic   MemoryType = "episodic"   // 情景记忆
    Semantic   MemoryType = "semantic"   // 语义记忆
    Procedural MemoryType = "procedural" // 程序记忆
    Shared     MemoryType = "shared"     // 共享记忆
)

type MemoryEntry struct {
    ID        string            `json:"id"`
    AgentID   string            `json:"agent_id"`
    Type      MemoryType        `json:"type"`
    Content   string            `json:"content"`
    Embedding []float64         `json:"embedding"`   // 向量
    Score     float64           `json:"score"`        // 重要性评分
    Recency   float64           `json:"recency"`      // 近期性 (衰减)
    Tags      []string          `json:"tags"`
    ProjectID string            `json:"project_id"`   // 共享记忆关联
    Metadata  map[string]string `json:"metadata"`
}

type MemoryService interface {
    // 存储
    Remember(ctx context.Context, entry *MemoryEntry) error

    // 检索 (语义相似度 + 近期性 + 重要性 综合评分)
    Recall(ctx context.Context, agentID string, query string, topK int) ([]*MemoryEntry, error)

    // 共享记忆 (项目级, 所有 Agent 可访问)
    ShareToProject(ctx context.Context, entry *MemoryEntry, projectID string) error
    GetProjectMemory(ctx context.Context, projectID string, query string, topK int) ([]*MemoryEntry, error)

    // 衰减 (艾宾浩斯遗忘, 定期执行)
    Decay(ctx context.Context, agentID string) error

    // 整合 (去重+合并相似记忆)
    Consolidate(ctx context.Context, agentID string) error
}
```

### 5.3 TS Plugin — Memory Bridge

```typescript
// @team/memory-bridge plugin
// 桥接 OpenClaw 的 memory-tools 与 Go 后台 Memory Service

import { OpenClawPlugin, PluginContext } from '@openclaw/sdk';

interface MemoryConfig {
  backendUrl: string;  // Go Memory Service 地址
  embedModel: string;  // 向量模型
}

export default class MemoryBridgePlugin implements OpenClawPlugin {
  private config: MemoryConfig;

  async remember(ctx: PluginContext, content: string, tags: string[]) {
    // 1. 调用 OpenClaw 原生 memory-tools 写入短期记忆
    await ctx.tools.invoke('memory-tools', 'memory_set', { content });

    // 2. 生成向量, 发送到 Go 后台写入长期记忆
    const embedding = await this.embed(content);
    await fetch(`${this.config.backendUrl}/api/memory`, {
      method: 'POST',
      body: JSON.stringify({
        agent_id: ctx.agentId,
        content,
        embedding,
        tags,
        type: 'episodic',
      }),
    });
  }

  async recall(ctx: PluginContext, query: string, topK: number = 5) {
    // 从 Go 后台检索长期记忆 (语义+近期+重要性综合)
    const res = await fetch(
      `${this.config.backendUrl}/api/memory/recall?agent=${ctx.agentId}&q=${query}&k=${topK}`
    );
    return res.json();
  }

  async shareToProject(ctx: PluginContext, content: string, projectId: string) {
    // 写入项目级共享记忆
    const embedding = await this.embed(content);
    await fetch(`${this.config.backendUrl}/api/memory/shared`, {
      method: 'POST',
      body: JSON.stringify({
        agent_id: ctx.agentId,
        project_id: projectId,
        content,
        embedding,
        type: 'shared',
      }),
    });
  }

  private async embed(text: string): Promise<number[]> {
    // 调用向量模型 (SiliconFlow / Bailian)
  }
}
```

---

## 6. 飞书集成设计

### 6.1 路由架构

```mermaid
graph TB
    subgraph Feishu["飞书"]
        DM[私聊消息]
        Group[群聊消息]
        Mention[@提及消息]
    end

    subgraph OpenClaw["OpenClaw 路由"]
        DMRouter[DM Router<br/>dmScope=per-channel-peer]
        BindRouter[Binding Router<br/>peer.kind=group]
        MentionRouter[Mention Router<br/>mentionPatterns]
    end

    subgraph Agents["Agent 层"]
        RDM[🎯 RD_MANAGER]
        ARCH[🏗️ ARCHITECT]
        DEV[💻 DEV_MANAGER]
        TEST[✅ TEST_MANAGER]
        AUDIT[🔒 CODE_AUDITOR]
    end

    DM --> DMRouter
    Group --> BindRouter
    Mention --> MentionRouter

    DMRouter -->|默认| RDM
    BindRouter -->|项目群绑定| RDM
    MentionRouter -->|@研发经理/@RD| RDM
    MentionRouter -->|@架构师/@ARCH| ARCH
    MentionRouter -->|@开发经理/@DEV| DEV
    MentionRouter -->|@测试经理/@QA| TEST
    MentionRouter -->|@代码审计/@AUDITOR| AUDIT
```

### 6.2 openclaw.json 配置

```json
{
  "agents": {
    "list": [
      {
        "id": "RD_MANAGER",
        "workspace": "./agents/RD_MANAGER",
        "default": true,
        "groupChat": {
          "mentionPatterns": ["@?RD_MANAGER", "@?研发经理", "@?项目经理", "@?RD"]
        }
      },
      {
        "id": "ARCHITECT",
        "workspace": "./agents/ARCHITECT",
        "groupChat": {
          "mentionPatterns": ["@?ARCHITECT", "@?架构师", "@?ARCH"]
        }
      },
      {
        "id": "DEV_MANAGER",
        "workspace": "./agents/DEV_MANAGER",
        "groupChat": {
          "mentionPatterns": ["@?DEV_MANAGER", "@?开发经理", "@?DEV"]
        }
      },
      {
        "id": "TEST_MANAGER",
        "workspace": "./agents/TEST_MANAGER",
        "groupChat": {
          "mentionPatterns": ["@?TEST_MANAGER", "@?测试经理", "@?QA"]
        }
      },
      {
        "id": "CODE_AUDITOR",
        "workspace": "./agents/CODE_AUDITOR",
        "groupChat": {
          "mentionPatterns": ["@?CODE_AUDITOR", "@?代码审计", "@?AUDITOR"]
        }
      }
    ]
  },
  "channels": {
    "feishu": {
      "enabled": true,
      "dmPolicy": "pairing",
      "accounts": {
        "main": {
          "appId": "${FEISHU_APP_ID}",
          "appSecret": "${FEISHU_APP_SECRET}"
        }
      }
    }
  },
  "bindings": [
    {
      "agentId": "RD_MANAGER",
      "match": { "channel": "feishu", "peer": { "kind": "group", "id": "${PROJECT_GROUP_ID}" } }
    }
  ],
  "skills": [
    "@team/message-bus",
    "@team/memory-bridge",
    "@team/task-queue",
    "memory-tools",
    "opencode"
  ]
}
```

---

## 7. 自定义 TS 插件清单

### 需要开发的插件

| 插件名 | 功能 | 核心接口 |
|--------|------|---------|
| `@team/message-bus` | Agent 间 Pub-Sub 通信 | `publish(topic, msg)` / `subscribe(topic, handler)` |
| `@team/memory-bridge` | 桥接 OpenClaw memory 与 Go 向量存储 | `remember()` / `recall()` / `shareToProject()` |
| `@team/task-queue` | 任务生命周期管理 | `createTask()` / `updateStatus()` / `getOverdue()` |
| `@team/quality-guard` | 代码质量自动检测 | `scanTodo()` / `scanEmptyFunctions()` / `report()` |
| `@team/feishu-notify` | 飞书消息格式化输出 | `sendCard()` / `sendReport()` / `sendAlert()` |

### 插件架构

```mermaid
graph LR
    subgraph TS_Plugins["TypeScript 插件层"]
        MB[message-bus]
        MBR[memory-bridge]
        TQ[task-queue]
        QG[quality-guard]
        FN[feishu-notify]
    end

    subgraph Go_Backend["Go 后台服务"]
        TaskAPI[Task API<br/>:8081]
        MemAPI[Memory API<br/>:8082]
        MonAPI[Monitor API<br/>:8083]
    end

    subgraph Storage["存储"]
        PG[(PostgreSQL<br/>任务/项目)]
        VEC[(向量数据库<br/>记忆)]
        Redis[(Redis<br/>缓存/队列)]
    end

    MB <-->|HTTP/WS| TaskAPI
    MBR <-->|HTTP| MemAPI
    TQ <-->|HTTP| TaskAPI
    QG -->|HTTP| MonAPI

    TaskAPI --> PG
    TaskAPI --> Redis
    MemAPI --> VEC
    MemAPI --> PG
    MonAPI --> Redis
```

---

## 8. Go 后台服务设计

### 8.1 服务结构

```
go-backend/
├── cmd/
│   └── server/
│       └── main.go          # 入口
├── internal/
│   ├── task/
│   │   ├── service.go       # TaskService 实现
│   │   ├── handler.go       # HTTP Handler
│   │   └── model.go         # Task Model
│   ├── memory/
│   │   ├── service.go       # MemoryService 实现
│   │   ├── handler.go       # HTTP Handler
│   │   ├── embedding.go     # 向量生成
│   │   └── decay.go         # 艾宾浩斯衰减
│   ├── monitor/
│   │   ├── service.go       # 监控服务
│   │   └── handler.go       # 告警 Handler
│   └── shared/
│       ├── config.go         # 配置
│       └── middleware.go     # 中间件
├── pkg/
│   └── vectordb/
│       ├── client.go         # 向量数据库客户端
│       └── milvus.go         # Milvus 实现 (或 FAISS)
├── go.mod
├── go.sum
└── Makefile
```

### 8.2 核心 API

```
POST   /api/tasks                     # 创建任务
PATCH  /api/tasks/:id/status          # 更新状态
GET    /api/tasks?project=xxx         # 按项目查询
GET    /api/tasks?assignee=xxx        # 按 Agent 查询
GET    /api/tasks/overdue             # 超期任务
POST   /api/tasks/:id/decompose       # 拆解子任务
POST   /api/tasks/:id/aggregate       # 汇总结果

POST   /api/memory                    # 存储记忆
GET    /api/memory/recall             # 检索记忆
POST   /api/memory/shared             # 共享记忆
POST   /api/memory/decay              # 触发衰减
POST   /api/memory/consolidate        # 触发整合

GET    /api/monitor/health            # 健康检查
GET    /api/monitor/agents            # Agent 状态
POST   /api/monitor/alert             # 告警
```

---

## 9. 与业界方案对比

```mermaid
graph TB
    subgraph 我们的方案["OpenClaw 多 Agent 方案"]
        direction LR
        F1[Coordinator-Worker<br/>借鉴 AutoGen/CrewAI]
        F2[Pub-Sub 消息总线<br/>借鉴 MetaGPT]
        F3[Subagent Swarm<br/>借鉴 Swarm/Kimi]
        F4[三层记忆<br/>借鉴 CoALA/CrewAI]
        F5[SOP 结构化产出<br/>借鉴 MetaGPT]
        F6[模型降级容错<br/>借鉴 AgentScope]
    end
```

| 维度 | AutoGen | CrewAI | MetaGPT | LangGraph | 我们的方案 |
|------|---------|--------|---------|-----------|-----------|
| 通信 | Direct + Pub-Sub | Task 输出 | 共享消息池 | 共享 State | **混合：Direct + Pub-Sub + Shared** |
| 编排 | GroupChat 轮转 | Flow-First | SOP 流水线 | 图状态机 | **Coordinator + Swarm + SOP** |
| 记忆 | 无内置 | 统一 Memory | 进程内列表 | Checkpoint | **三层记忆（向量+衰减+共享）** |
| 容错 | 无 | Guardrails | 可执行反馈 | 无 | **模型降级 + 质量巡检 + 重试** |
| 分布式 | 支持 | 无 | 无 | 无 | **Go 微服务 + TS 插件** |
| 飞书 | 无 | 无 | 无 | 无 | **原生支持（binding + mentionPatterns）** |
| 技术栈 | Python | Python | Python | Python | **Go 后台 + TS 插件** |

---

## 10. 实施路线图

### Phase 1: 基础通信（1-2 周）

- [ ] 开发 `@team/message-bus` TS 插件（Pub-Sub + 消息格式）
- [ ] 配置 openclaw.json（5 Agent + 飞书 bindings）
- [ ] 验证飞书 → Agent 路由 + Agent 间 sessions_send

### Phase 2: 任务管理（2-3 周）

- [ ] 开发 Go Task Service（CRUD + 状态机 + 拆解/汇总）
- [ ] 开发 `@team/task-queue` TS 插件（桥接 Agent ↔ Task Service）
- [ ] 验证 RD_MANAGER 拆解任务 → 分配 → 跟踪 → 验收流程

### Phase 3: 记忆系统（2-3 周）

- [ ] 部署向量数据库（Milvus / FAISS）
- [ ] 开发 Go Memory Service（存储 + 检索 + 衰减 + 整合）
- [ ] 开发 `@team/memory-bridge` TS 插件
- [ ] 验证跨 Agent 记忆共享 + 项目知识库

### Phase 4: 质量与监控（1-2 周）

- [ ] 开发 `@team/quality-guard` TS 插件（TODO/空函数/存根检测）
- [ ] 开发 Go Monitor Service（Agent 状态 + 超期告警）
- [ ] 开发 `@team/feishu-notify` TS 插件（卡片/报告/告警格式）

### Phase 5: 优化与演进（持续）

- [ ] 基于使用数据优化模型降级策略
- [ ] 基于记忆积累优化 Agent 行为
- [ ] 探索动态编排（根据任务类型自动选择协作模式）
