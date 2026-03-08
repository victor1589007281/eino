# 研发 Agent 团队构建方案

> 基于 OpenClaw + 飞书（Lark）+ 阿里百炼 Coding Plan 的研发 AI 员工群方案

---

## 1. 整体架构

### 1.1 ASCII 架构图

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              飞书（Lark）                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐ │
│  │ 项目A 飞书群  │  │ 项目B 飞书群  │  │ 项目C 飞书群  │  │ 用户私聊（1v1）      │ │
│  │ oc_project_a │  │ oc_project_b │  │ oc_project_c │  │ @PM / @架构师 / ...  │ │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘ │
└─────────┼────────────────┼────────────────┼─────────────────────┼──────────────┘
          │                │                │                     │
          │  WebSocket 长连接（无需公网 URL）  │                     │
          ▼                ▼                ▼                     ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         OpenClaw Gateway（mini 主机）                             │
│                                                                                   │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │                          Agent 路由（bindings）                               │ │
│  │   match.peer.kind="group" + match.peer.id=群ID  → 项目群 Agent 团队           │ │
│  │   match.peer.kind="direct" + match.peer.id=用户ID → 角色 Agent 1v1            │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────────┘
          │                │                │                     │
          ▼                ▼                ▼                     ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              各角色 Agent                                        │
│                                                                                   │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐   │
│  │ PM      │ │Architect │ │ Backend  │ │ Frontend │ │   QA     │ │ DevOps   │   │
│  │ Agent   │ │ Agent    │ │ Dev      │ │ Dev      │ │ Agent    │ │ Agent    │   │
│  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘   │   │
│       │           │           │           │           │           │         │   │
│  ┌────┴───────────┴───────────┴───────────┴───────────┴───────────┴────┐   │   │
│  │                        Doc Agent（文档）                             │   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │   │
│                                                                                   │
│  每个 Agent：独立 workspace / SOUL.md / AGENTS.md / MEMORY.md / agentDir           │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 一个飞书群 = 一个项目

| 飞书群 | 项目 | 绑定 Agent 团队 | 说明 |
|--------|------|-----------------|------|
| 群 ID: `oc_project_a` | 项目 A | PM + Architect + Backend + Frontend + QA + DevOps + Doc | 该群内 @机器人 触发，消息路由到项目 A 的 Coordinator |
| 群 ID: `oc_project_b` | 项目 B | 同上（角色复用） | 同一套角色 Agent 服务多个项目 |
| 群 ID: `oc_project_c` | 项目 C | 同上 | 项目隔离通过 MEMORY.md 实现 |

**映射关系**：每个飞书群通过 `bindings` 的 `match.peer.kind="group"` + `match.peer.id="群ID"` 绑定到一个 **Coordinator Agent**，该 Coordinator 负责拆解任务并 `sessions_spawn` 调用各角色 Sub-Agent。

### 1.3 同一角色 Agent 如何复用到多个项目

- **一个 Architect Agent** 可被多个项目群的 Coordinator 通过 `sessions_spawn` 调用
- **实现方式**：每个项目群绑定一个 **PM/Coordinator Agent**，该 Agent 的 workspace 中有项目专属的 `MEMORY.md`（项目背景、技术栈、历史决策）
- Coordinator 调用 `sessions_spawn({ agentId: "architect", ... })` 时，Architect 的 session 可接收 `instruction` 中注入的「当前项目上下文」
- **项目隔离**：通过 `instruction` 传入 `projectId` 或项目名，Architect 的 MEMORY.md 可按项目分块存储，或使用 `memory_search` 按项目过滤

---

## 2. Agent 角色设计

| 角色 | 名称 | 职责 | 模型选择 | 核心 Skills |
|------|------|------|----------|-------------|
| **PM Agent** | 小项（或 ProjectPM） | 需求分析、任务拆解、优先级排序、进度跟踪 | qwen3.5-plus | github, linear, notion, memory-tools |
| **Architect Agent** | 小架（或 TechArch） | 架构设计、技术方案、Code Review、技术选型 | qwen3-coder-plus / kimi-k2.5 | github, codebase-search, memory-tools |
| **Backend Dev Agent** | 小后（或 BackendDev） | 后端开发、API 设计、数据库设计、接口实现 | qwen3-coder-plus | github, exec, read, write, apply_patch |
| **Frontend Dev Agent** | 小前（或 FrontendDev） | 前端开发、UI/UX 实现、组件开发 | qwen3-coder-plus | github, exec, read, write, apply_patch |
| **QA Agent** | 小测（或 QAEngineer） | 测试用例设计、Bug 分析、回归测试建议 | qwen3.5-plus | github, exec, memory-tools |
| **DevOps Agent** | 小运（或 DevOpsEng） | CI/CD、部署、监控、日志分析 | qwen3-coder-plus | github, exec, read, write |
| **Doc Agent** | 小文（或 DocWriter） | 文档生成、API 文档、变更日志、README | qwen3.5-plus | github, read, memory-tools |

### 2.1 各角色详细配置

```yaml
# PM Agent
id: pm
name: 小项
theme: 项目管理与需求分析专家
model: dashscope/qwen3.5-plus
skills: ["github", "linear", "notion", "memory-tools", "sessions_spawn", "sessions_history"]

# Architect Agent
id: architect
name: 小架
theme: 架构设计与技术方案专家
model: dashscope/qwen3-coder-plus  # 复杂方案用 kimi-k2.5
skills: ["github", "codebase-search", "memory-tools", "read", "sessions_history"]

# Backend Dev Agent
id: backend
name: 小后
theme: 后端开发与 API 设计专家
model: dashscope/qwen3-coder-plus
skills: ["github", "exec", "read", "write", "apply_patch", "sessions_history"]

# Frontend Dev Agent
id: frontend
name: 小前
theme: 前端开发与 UI 实现专家
model: dashscope/qwen3-coder-plus
skills: ["github", "exec", "read", "write", "apply_patch", "sessions_history"]

# QA Agent
id: qa
name: 小测
theme: 测试用例与 Bug 分析专家
model: dashscope/qwen3.5-plus
skills: ["github", "exec", "memory-tools", "read", "sessions_history"]

# DevOps Agent
id: devops
name: 小运
theme: CI/CD 与部署运维专家
model: dashscope/qwen3-coder-plus
skills: ["github", "exec", "read", "write", "sessions_history"]

# Doc Agent
id: doc
name: 小文
theme: 技术文档与 API 文档专家
model: dashscope/qwen3.5-plus
skills: ["github", "read", "memory-tools", "sessions_history"]
```

---

## 3. 飞书群与 Agent 绑定配置

### 3.1 openclaw.json 完整示例

```json5
{
  // ═══════════════════════════════════════════════════════════
  // 研发 Agent 团队 - OpenClaw 主配置
  // 飞书群 = 项目，一个群一个 Coordinator，角色 Agent 复用
  // ═══════════════════════════════════════════════════════════

  "agent": {
    "model": "dashscope/qwen3.5-plus",
    "fallbackModels": [
      "dashscope/qwen3-coder-plus",
      "dashscope/kimi-k2.5",
      "deepseek/deepseek-chat"
    ],
    "name": "研发助手",
    "theme": "研发 Agent 团队默认入口",
    "emoji": "🔧"
  },

  "security": {
    "sensitiveData": {
      "patterns": ["sk-*", "key-*", "token-*", "secret-*"]
    }
  },

  // ═══════════════════════════════════════════════════════════
  // 多 Agent 定义
  // ═══════════════════════════════════════════════════════════
  "agents": {
    "defaults": {
      "bootstrapMaxChars": 20000,
      "maxContextTokens": 128000,
      "maxResponseTokens": 8192,
      "temperature": 0.7,
      "compaction": { "memoryFlush": true },
      "subagents": {
        "maxSpawnDepth": 2,
        "maxChildrenPerAgent": 6,
        "maxConcurrent": 10
      }
    },

    "list": [
      {
        "id": "coordinator_project_a",
        "name": "项目A协调者",
        "workspace": "~/.openclaw/workspace-project-a",
        "agentDir": "~/.openclaw/agents/coordinator_project_a/agent",
        "model": "dashscope/qwen3.5-plus",
        "groupChat": {
          "mentionPatterns": ["@研发助手", "@机器人", "openclaw"]
        }
      },
      {
        "id": "coordinator_project_b",
        "name": "项目B协调者",
        "workspace": "~/.openclaw/workspace-project-b",
        "agentDir": "~/.openclaw/agents/coordinator_project_b/agent",
        "model": "dashscope/qwen3.5-plus",
        "groupChat": {
          "mentionPatterns": ["@研发助手", "@机器人", "openclaw"]
        }
      },
      {
        "id": "pm",
        "name": "小项",
        "workspace": "~/.openclaw/workspace-pm",
        "agentDir": "~/.openclaw/agents/pm/agent",
        "model": "dashscope/qwen3.5-plus"
      },
      {
        "id": "architect",
        "name": "小架",
        "workspace": "~/.openclaw/workspace-architect",
        "agentDir": "~/.openclaw/agents/architect/agent",
        "model": "dashscope/qwen3-coder-plus"
      },
      {
        "id": "backend",
        "name": "小后",
        "workspace": "~/.openclaw/workspace-backend",
        "agentDir": "~/.openclaw/agents/backend/agent",
        "model": "dashscope/qwen3-coder-plus"
      },
      {
        "id": "frontend",
        "name": "小前",
        "workspace": "~/.openclaw/workspace-frontend",
        "agentDir": "~/.openclaw/agents/frontend/agent",
        "model": "dashscope/qwen3-coder-plus"
      },
      {
        "id": "qa",
        "name": "小测",
        "workspace": "~/.openclaw/workspace-qa",
        "agentDir": "~/.openclaw/agents/qa/agent",
        "model": "dashscope/qwen3.5-plus"
      },
      {
        "id": "devops",
        "name": "小运",
        "workspace": "~/.openclaw/workspace-devops",
        "agentDir": "~/.openclaw/agents/devops/agent",
        "model": "dashscope/qwen3-coder-plus"
      },
      {
        "id": "doc",
        "name": "小文",
        "workspace": "~/.openclaw/workspace-doc",
        "agentDir": "~/.openclaw/agents/doc/agent",
        "model": "dashscope/qwen3.5-plus"
      }
    ]
  },

  // ═══════════════════════════════════════════════════════════
  // Bindings：飞书群绑定 + 私聊绑定
  // ═══════════════════════════════════════════════════════════
  "bindings": [
    // ─── 项目群绑定（群聊）───
    {
      "agentId": "coordinator_project_a",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "group", "id": "oc_project_a" }
      }
    },
    {
      "agentId": "coordinator_project_b",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "group", "id": "oc_project_b" }
      }
    },

    // ─── 私聊绑定（1v1 直接找某个角色）───
    {
      "agentId": "pm",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "direct", "id": "user_pm_chat" }
      }
    },
    {
      "agentId": "architect",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "direct", "id": "user_architect_chat" }
      }
    },
    {
      "agentId": "backend",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "direct", "id": "user_backend_chat" }
      }
    },
    {
      "agentId": "frontend",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "direct", "id": "user_frontend_chat" }
      }
    },
    {
      "agentId": "qa",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "direct", "id": "user_qa_chat" }
      }
    },
    {
      "agentId": "devops",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "direct", "id": "user_devops_chat" }
      }
    },
    {
      "agentId": "doc",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "direct", "id": "user_doc_chat" }
      }
    },

    // ─── 飞书默认 fallback（未匹配时）───
    {
      "agentId": "coordinator_project_a",
      "match": { "channel": "feishu" }
    }
  ],

  // ═══════════════════════════════════════════════════════════
  // 飞书 Channel 配置
  // ═══════════════════════════════════════════════════════════
  "channels": {
    "feishu": {
      "enabled": true,
      "appId": "cli_xxxxxxxxxx",
      "appSecret": "${FEISHU_APP_SECRET}"
    }
  },

  // ═══════════════════════════════════════════════════════════
  // Skills
  // ═══════════════════════════════════════════════════════════
  "skills": {
    "allowBundled": [],
    "load": {
      "extraDirs": ["~/.openclaw/skills"],
      "watch": true,
      "watchDebounceMs": 250
    },
    "entries": {
      "secure-install": { "enabled": true },
      "memory-tools": { "enabled": true }
    }
  }
}
```

### 3.2 群聊 @mention 配置

- 每个项目群绑定的 Coordinator 已配置 `groupChat.mentionPatterns`
- 用户在群内需 **@机器人** 或输入 `@研发助手` 才会触发回复
- 飞书端需申请 `im.message.group_at_msg` 权限

### 3.3 已知 Bug 与 Workaround

| 问题 | Issue | 现象 | Workaround |
|------|-------|------|------------|
| 飞书多 Agent 路由失败 | #16354, #32678 | 所有消息路由到 main/default agent，bindings 不生效 | **方案 1**：使用 `systemPromptAppend` 在默认 agent 的 system prompt 中注入「当前会话应模拟的角色」，由单 agent 根据上下文切换人格。<br>**方案 2**：为每个项目/角色创建**独立的飞书应用**（不同 appId），每个应用对应一个 accountId，通过 `match.accountId` 路由到对应 agent。 |
| peer.id 格式 | - | 飞书群 ID / 用户 ID 需从实际事件中获取 | 使用 `openclaw gateway` 日志或 `openclaw doctor` 查看实际 session key 格式，将 `oc_project_a` 等替换为真实 ID。 |

**推荐 Workaround（单应用场景）**：若 bindings 不生效，可暂时采用「单 Coordinator + systemPromptAppend」：

```json5
{
  "agents": {
    "list": [
      {
        "id": "main",
        "default": true,
        "systemPromptAppend": "当前会话上下文：若在项目A群则扮演项目A协调者，若在项目B群则扮演项目B协调者；根据群ID动态加载对应 MEMORY.md。"
      }
    ]
  }
}
```

通过 `channels.feishu.groups` 的 `systemPrompt` 按群覆盖也可实现类似效果。

---

## 4. 项目协作工作流

### 4.1 新需求 → 全流程

```
用户（项目群）: @研发助手 我们要做一个用户反馈功能，支持提交和查看

    │
    ▼
┌─ Coordinator（PM 模式）─────────────────────────────────────────┐
│  1. 需求澄清 → PM Agent 拆解任务                                 │
│  2. Architect 评审技术方案                                       │
│  3. 并行 spawn: Backend + Frontend                               │
│  4. QA 设计测试用例                                              │
│  5. DevOps 配置部署流程                                          │
│  6. Doc 生成 API 文档                                            │
└──────────────────────────────────────────────────────────────────┘

流程：
  Phase 1: PM 拆解 → [需求文档、任务列表、优先级]
  Phase 2: Architect 评审 → [技术方案、API 设计、数据库设计]
  Phase 3: Backend + Frontend 并行开发（sessions_spawn）
  Phase 4: QA 生成测试用例
  Phase 5: DevOps 提供部署命令/脚本
  Phase 6: Doc 生成变更日志和 API 文档
```

### 4.2 Code Review 流程

```
用户: @研发助手 帮我 Review 一下 PR #42

    │
    ▼
Coordinator → spawn Architect Agent
  instruction: "对 PR #42 进行 Code Review，关注：架构一致性、安全、性能、可维护性"
  skills: ["github", "read"]
    │
    ▼
Architect 返回 Review 意见 → Coordinator 汇总 → 回复用户
```

### 4.3 Bug 修复流程

```
用户: @研发助手 生产环境报错 [堆栈信息]，帮忙分析

    │
    ▼
Coordinator → 并行 spawn:
  - QA Agent: 分析堆栈，定位可能原因
  - Backend Agent: 检查相关代码逻辑
  - DevOps Agent: 检查日志、监控
    │
    ▼
汇总三方结论 → 给出修复建议和优先级
```

### 4.4 紧急上线流程

```
用户: @研发助手 紧急修复，需要立刻上线

    │
    ▼
Coordinator → spawn DevOps Agent
  instruction: "执行紧急上线流程：1) 确认变更范围 2) 备份 3) 部署 4) 验证 5) 回滚预案"
  skills: ["exec", "read", "write"]
    │
    ▼
DevOps 按步骤执行并反馈 → Coordinator 同步到群
```

---

## 5. 角色复用方案

### 5.1 同一 Architect 服务多个项目

- **配置**：只有一个 `architect` agent，被多个 Coordinator 通过 `sessions_spawn` 调用
- **项目上下文注入**：在 `instruction` 中传入项目信息

```javascript
sessions_spawn({
  agentId: "architect",
  instruction: `
    项目: 项目A（电商后台）
    代码库: /path/to/project-a
    技术栈: Go + PostgreSQL + Redis
    
    任务: 评审 API 设计文档，给出架构建议。
    文档内容: ...
  `,
  skills: ["github", "read", "memory-tools"]
})
```

### 5.2 通过 bindings 多条规则实现

若飞书支持多 account（多应用），可配置：

```json5
{
  "bindings": [
    { "agentId": "architect", "match": { "channel": "feishu", "accountId": "project_a" } },
    { "agentId": "architect", "match": { "channel": "feishu", "accountId": "project_b" } }
  ]
}
```

同一 architect 响应不同 account 的请求，通过 session 的 channel/account 信息区分项目。

### 5.3 项目隔离的 MEMORY.md 设计

```
~/.openclaw/workspace-architect/
├── MEMORY.md          # 全局：架构原则、技术偏好
├── projects/
│   ├── project_a.md   # 项目A：技术栈、历史决策、已知坑
│   └── project_b.md   # 项目B：同上
└── SOUL.md
```

Architect 的 AGENTS.md 中约定：收到任务时先 `memory_search("项目名")` 加载项目上下文，再执行任务。

---

## 6. 直接沟通（1v1 私聊）

### 6.1 配置说明

- 用户在飞书中**直接私聊机器人**，通过 bindings 的 `match.peer.kind="direct"` 路由
- **问题**：飞书单应用下，所有私聊的 peer.id 可能相同（都是和机器人的会话），无法按「用户想找哪个角色」区分

**可行方案**：

1. **多应用方案**：创建 7 个飞书应用（PM、架构师、后端…），每个应用对应一个 accountId，bindings 按 accountId 路由到对应 agent。用户根据需要添加不同机器人进行 1v1。
2. **单应用 + 文本指令**：所有私聊路由到默认 agent，在 systemPrompt 中约定：用户说「找架构师」「找小架」时，该 agent 切换为架构师人格，或内部 `sessions_spawn` 给 architect 并转发结果。

### 6.2 场景示例（多应用方案）

| 用户操作 | 路由 | 场景 |
|----------|------|------|
| 添加「小架-架构师」机器人并私聊 | architect | 直接讨论技术方案、Code Review |
| 添加「小项-PM」机器人并私聊 | pm | 需求澄清、任务拆解、进度同步 |
| 添加「小后-后端」机器人并私聊 | backend | 讨论 API 设计、数据库设计 |
| 在项目群 @研发助手 | coordinator_project_x | 全团队协作流程 |

### 6.3 单应用下的 1v1 简化方案

若只用一个飞书应用，私聊统一到一个「路由 Agent」：

```json5
{
  "agents": {
    "list": [
      {
        "id": "dm_router",
        "name": "研发助手",
        "systemPromptAppend": "用户私聊你时，根据其意图路由：说「找架构师/小架」则扮演架构师；说「找PM/小项」则扮演PM；否则默认扮演全栈助手。"
      }
    ]
  },
  "bindings": [
    {
      "agentId": "dm_router",
      "match": { "channel": "feishu", "peer": { "kind": "direct" } }
    }
  ]
}
```

---

## 7. 推荐 Skills 安装列表

| 类别 | Skill | 用途 | 安装命令 |
|------|-------|------|----------|
| 安全 | secure-install | 安装前恶意检测 | `npx playbooks add skill openclaw/skills --skill secure-install` |
| 记忆 | memory-tools | 持久化记忆、语义检索 | `npx playbooks add skill openclaw/skills --skill memory-tools` |
| 代码 | github | GitHub 集成、PR、Issue | `npx playbooks add skill openclaw/skills --skill github` |
| 项目管理 | linear | Linear 任务管理（可选） | 社区/官方 |
| 文档 | notion | Notion 集成（可选） | 社区/官方 |
| 代码分析 | codebase-search | 代码库语义搜索 | 官方/社区 |

**最小可用集合**：`secure-install` + `memory-tools` + `github`

---

## 8. 生产力落地建议

### 8.1 第一周怎么用

1. **Day 1-2**：部署 OpenClaw + 飞书，配置 1 个项目群 + 1 个 Coordinator
2. **Day 3**：只启用 PM + Architect 两个角色，在群内试「需求拆解」和「方案评审」
3. **Day 4-5**：加入 Backend/Frontend，试「任务分配」和「代码生成」
4. **Day 6-7**：记录 bindings 是否生效，若无效则启用 systemPromptAppend workaround

### 8.2 第一个月怎么用

1. **Week 1**：单项目跑通全流程（需求→方案→开发→测试→文档）
2. **Week 2**：加入第二个项目群，验证角色复用
3. **Week 3**：配置 1v1 私聊（多应用或单应用路由方案）
4. **Week 4**：沉淀各角色 MEMORY.md，固化项目背景和技术决策

### 8.3 持续优化

- 每周回顾 MEMORY.md，清理过时信息
- 根据实际使用调整各角色的 model（复杂任务用 kimi-k2.5）
- 关注 OpenClaw 飞书 bindings 相关 Issue 的修复进展，修复后切回标准 bindings 配置

---

## 附录：飞书群 ID 获取方式

1. 在群设置中查看（部分版本可见）
2. 通过飞书开放平台 API：`im/v1/chats` 获取群列表
3. 查看 OpenClaw 日志：收到群消息时，日志中会打印 `peer.id` 或 session key

---

*文档版本: 1.0.0 | 最后更新: 2026-03-08*
