# OpenClaw 集成指南

## 一、集成架构

```
┌──────────────────────────────────────────────┐
│                  OpenClaw                     │
│                                              │
│  ┌──────────┐  ┌───────────┐  ┌──────────┐  │
│  │ Agent A  │  │ Agent B   │  │ Agent C  │  │
│  │(研发经理) │  │ (架构师)  │  │ (开发)   │  │
│  └────┬─────┘  └─────┬─────┘  └─────┬────┘  │
│       │              │              │         │
│  ┌────▼──────────────▼──────────────▼────┐   │
│  │         @team/collab 插件 (TS)         │   │
│  │  Hooks → Tools → Commands → Bridge    │   │
│  └───────────────────┬───────────────────┘   │
│                      │                        │
└──────────────────────┼────────────────────────┘
                       │ HTTP
              ┌────────▼────────┐
              │  Go 协作服务     │
              │  :8090          │
              └────────┬────────┘
                       │
              ┌────────▼────────┐
              │   PostgreSQL     │
              └─────────────────┘
```

## 二、步骤总览

| 步骤 | 操作 | 耗时 |
|------|------|------|
| 1 | 部署 Go 服务 + PostgreSQL | 5 分钟 |
| 2 | 编译安装 TS 插件 | 2 分钟 |
| 3 | 配置 openclaw.json | 5 分钟 |
| 4 | 配置 Agent 协作协议 | 每 Agent 2 分钟 |
| 5 | 配置飞书 Binding | 5 分钟 |
| 6 | 验证 | 5 分钟 |

## 三、详细步骤

### 步骤 1：部署 Go 服务

参考 [build-deploy.md](./build-deploy.md)，选择一种部署方案：

```bash
cd code

# 方案 A：Docker 全家桶
make docker-up

# 方案 B：已有 PG + 本地 Go
make run-go
```

验证：

```bash
curl http://localhost:8090/health
# {"status":"ok"}
```

### 步骤 2：安装 TS 插件

```bash
# 一键安装（编译 + 拷贝到 OpenClaw 插件目录）
make install-plugin

# 默认安装到: ~/.openclaw/plugins/@team/collab/
# 自定义路径: OPENCLAW_DIR=/your/path make install-plugin
```

安装后确认目录结构：

```
~/.openclaw/plugins/@team/collab/
├── dist/
│   ├── index.js
│   ├── go-bridge.js
│   ├── hooks/
│   ├── tools/
│   ├── commands/
│   └── protocol/
├── package.json
└── openclaw.plugin.json
```

### 步骤 3：配置 openclaw.json

在 OpenClaw 的主配置文件中添加以下内容：

```json5
// openclaw.json
{
  // --- 插件配置 ---
  "plugins": {
    "@team/collab": {
      "goServiceUrl": "http://localhost:8090",
      "feishuEnabled": true,
      "contextMaxTokens": 3000
    }
  },

  // --- Agent 默认配置 ---
  "agents": {
    "defaults": {
      "model": {
        "primary": "alibaba/qwen3.5-plus",
        "fallbacks": ["alibaba/kimi-k2.5", "glm-5", "minimax-m-2.5"]
      },
      "subagents": {
        "maxConcurrent": 3,
        "model": "alibaba/qwen3.5-plus",
        "archiveAfterMinutes": 120
      },
      "compaction": {
        "memoryFlush": { "enabled": true, "softThresholdTokens": 4000 }
      },
      "memorySearch": {
        "enabled": true,
        "provider": "openai",
        "query": { "hybrid": { "enabled": true, "vectorWeight": 0.7, "textWeight": 0.3 } }
      },
      "heartbeat": { "every": "30m" },
      "maxConcurrent": 2
    },

    // --- Agent 列表 ---
    "list": [
      {
        "id": "RD_MANAGER",
        "name": "研发经理",
        "agentDir": "/path/to/agents/rd-manager/agent",
        "subagents": { "allowAgents": ["*"] }
      },
      {
        "id": "ARCHITECT",
        "name": "架构师",
        "agentDir": "/path/to/agents/architect/agent",
        "subagents": { "allowAgents": ["RD_MANAGER", "DEV_MANAGER", "CODE_AUDITOR"] }
      },
      {
        "id": "DEV_MANAGER",
        "name": "开发主管",
        "agentDir": "/path/to/agents/dev-manager/agent",
        "subagents": { "allowAgents": ["ARCHITECT", "TEST_MANAGER"] }
      },
      {
        "id": "CODE_AUDITOR",
        "name": "代码审计",
        "agentDir": "/path/to/agents/code-auditor/agent",
        "subagents": { "allowAgents": ["DEV_MANAGER"] }
      },
      {
        "id": "TEST_MANAGER",
        "name": "测试主管",
        "agentDir": "/path/to/agents/test-manager/agent",
        "subagents": { "allowAgents": ["DEV_MANAGER"] }
      }
    ]
  },

  // --- Agent 间通信 ---
  "tools": {
    "agentToAgent": { "enabled": true, "allow": ["*"] }
  },

  // --- 飞书通道 ---
  "channels": {
    "feishu": {
      "enabled": true,
      "dmPolicy": "pairing",
      "streaming": true,
      "blockStreaming": true
    }
  },

  // --- 群 → Agent 路由 ---
  "bindings": [
    {
      "agentId": "RD_MANAGER",
      "match": { "channel": "feishu", "peer": { "kind": "group", "id": "oc_your_project_group" } }
    }
  ]
}
```

### 步骤 4：为每个 Agent 添加协作协议

在每个 Agent 的 `AGENTS.md` 文件末尾追加以下内容（也可以从代码中生成）：

```markdown
## 协作协议

### 上下文感知
每次收到消息时，上下文中已包含（由系统自动注入）：
- 当前项目信息、迭代目标
- 我的进行中任务列表
- 关键决策（pinned decisions）
- 可调动的团队 Agent 列表
- 任务统计

### 任务管理
- 收到任务 → 调 update_task(status=in_progress)
- 完成任务 → 调 update_task(status=review, result=交付物) + save_artifact
- 遇到阻塞 → 调 update_task(status=blocked, block_reason=原因)
- 需要拆分 → 调 create_task(parent_task_id=当前任务)
- 需要协作 → 调 create_task(assignee=目标Agent) 或 sessions_send

### 反失忆
以下场景必须调 save_decision(pin=true)：
- 技术选型、架构决策、用户指示、方向变更、重大约束

### 短期记忆
- 对话由 OpenClaw 自动管理（transcript + compaction）
- 压缩前系统自动让你保存到 memory/
- 使用 memory_search 搜索历史记忆

### 消息格式
不需要手动添加角色前缀（系统自动添加）
```

**自动生成方式（推荐）：**

```bash
# Node.js 脚本生成协作协议段落
node -e "
  const { generateCollabProtocol } = require('./ts-plugins/collab/dist/protocol/collab-agents-md.js');
  console.log(generateCollabProtocol());
" >> /path/to/agent/AGENTS.md
```

### 步骤 5：配置飞书 Binding

#### 5.1 群聊 → Agent 路由

在 `openclaw.json` 的 `bindings` 中配置群聊到 Agent 的映射：

```json5
"bindings": [
  // 项目 A 群 → 研发经理
  {
    "agentId": "RD_MANAGER",
    "match": { "channel": "feishu", "peer": { "kind": "group", "id": "oc_project_a" } }
  },
  // 项目 B 群 → 研发经理（同一 Agent 可绑定多个群）
  {
    "agentId": "RD_MANAGER",
    "match": { "channel": "feishu", "peer": { "kind": "group", "id": "oc_project_b" } }
  }
]
```

#### 5.2 @提及路由

在 `openclaw.json` 中配置 `mentionPatterns`，支持中英文@提及：

```json5
"agents": {
  "list": [
    {
      "id": "ARCHITECT",
      "name": "架构师",
      "groupChat": {
        "mentionPatterns": ["架构师", "ARCHITECT", "arch"]
      }
    }
  ]
}
```

#### 5.3 飞书 Bot 映射（多 Bot 场景）

如果有多个飞书机器人，通过 Go 服务 API 配置映射：

```bash
# 为特定 Agent + 群聊 指定专属 Bot
curl -X POST http://localhost:8090/api/v1/feishu/bot-mapping \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_id": "ARCHITECT",
    "bot_app_id": "cli_architect_bot",
    "group_id": "oc_project_a"
  }'
```

### 步骤 6：验证

#### 6.1 Go 服务验证

```bash
# 健康检查
curl http://localhost:8090/health

# 查看自动扫描到的 Agent
curl http://localhost:8090/api/v1/agents | python3 -m json.tool

# Dashboard 概览
curl http://localhost:8090/api/v1/dashboard/overview | python3 -m json.tool
```

#### 6.2 插件验证

重启 OpenClaw 后检查日志：

```bash
# 查看 OpenClaw 日志中是否有插件加载信息
# 应该能看到类似:
# [plugin] Loaded @team/collab v1.0.0
# [collab] Starting Go bridge health monitor
# [collab] Go service reachable at http://localhost:8090
```

#### 6.3 端到端验证

1. 在飞书群中 @机器人 发送消息
2. 确认收到带有角色前缀的回复（如 `【🏗️ 架构师】`）
3. 使用 `/status` 命令查看项目状态
4. 创建项目：在群聊中让管理 Agent 执行 `create_project`
5. 分配任务：让管理 Agent 执行 `create_task`
6. 查看 Dashboard：`curl http://localhost:8090/api/v1/dashboard/overview`

## 四、创建新项目

通过飞书群聊或 API 创建：

**群聊方式（推荐）：**

在项目群中对 @研发经理 说：「创建一个名为 XX 的新项目，技术栈是 Go + React，团队包含架构师、开发主管、测试主管」

**API 方式：**

```bash
curl -X POST http://localhost:8090/api/v1/projects \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "新项目名称",
    "group_id": "oc_feishu_group_id",
    "team_agents": ["RD_MANAGER", "ARCHITECT", "DEV_MANAGER", "CODE_AUDITOR", "TEST_MANAGER"],
    "tech_stack": ["Go", "React", "PostgreSQL"]
  }'
```

## 五、卸载

```bash
# 1. 停止 Go 服务
make docker-down
# 或
make stop-go

# 2. 移除插件
make uninstall-plugin

# 3. 从 openclaw.json 中删除 plugins 和相关配置

# 4. 重启 OpenClaw
openclaw restart

# 5. 清理数据（可选）
make db-reset
docker volume rm code_pgdata
```
