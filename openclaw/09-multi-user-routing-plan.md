# 多用户路由隔离方案

## 1. 问题分析

### 1.1 为什么需要隔离

当一台 mini 主机上的 OpenClaw 对接飞书机器人，且该机器人被分享给多人使用时，**不同用户会同时向同一个 Agent 发送消息**。若不进行隔离，会出现以下严重问题：

1. **上下文污染**：用户 A 的对话历史会混入用户 B 的会话，模型可能基于「用户 A 之前说的」来回答用户 B 的问题
2. **隐私泄露**：用户 A 的敏感信息（如医疗、财务、日程）可能被模型误用或泄露给用户 B
3. **回复错乱**：模型可能把「我们之前讨论的」指向错误的用户
4. **记忆混淆**：MEMORY.md 等持久化记忆会混入多用户数据，导致偏好、错误历史被错误应用

### 1.2 不隔离会出什么问题

| 问题类型 | 示例 | 后果 |
|---------|------|------|
| **上下文污染** | 用户 A 问「帮我生成一张儿童成长手册」→ 用户 B 问「我们刚才聊的什么？」→ 模型回答「儿童成长手册」 | 用户 B 完全困惑，体验极差 |
| **隐私泄露** | 用户 A 提到「我的孩子小明 5 岁」→ 用户 B 问「帮我写个文案」→ 模型可能生成「小明 5 岁」相关内容 | 严重隐私违规 |
| **回复错乱** | 多用户共享同一 session，模型无法区分「你」指谁 | 回复牛头不对马嘴 |
| **资源滥用** | 某用户恶意请求耗尽 API 配额 | 影响其他用户 |

### 1.3 隔离的几个层级

| 层级 | 说明 | 隔离粒度 | 适用场景 |
|---------|------|---------|---------|
| **会话隔离** | 每个用户有独立的 session 上下文和记忆 | 同一 Agent，不同 session | 多用户共享同一 Agent，轻量级 |
| **Agent 隔离** | 不同用户路由到不同 Agent（独立 workspace、agentDir、session store） | 完全隔离的 Agent 实例 | 不同角色/权限/人格 |
| **数据隔离** | 每用户独立 workspace、FRIENDS/、RELATIONS/ | 文件级隔离 | 高安全、多租户 |

---

## 2. 隔离架构

### 2.1 架构图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  飞书用户                                                                     │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐           │
│  │ 用户 A      │ │ 用户 B      │ │ 用户 C      │ │ 用户 D      │           │
│  │ 管理员      │ │ 普通用户    │ │ 普通用户    │ │ VIP 用户    │           │
│  │ ou_admin    │ │ ou_user1    │ │ ou_user2    │ │ ou_vip      │           │
│  └──────┬──────┘ └──────┬──────┘ └──────┬──────┘ └──────┬──────┘           │
└─────────┼──────────────┼──────────────┼──────────────┼────────────────────┘
          │              │              │              │
          ▼              ▼              ▼              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  飞书 Gateway（WebSocket 长连接）                                             │
│  - 接收 im.message.receive_v1                                                │
│  - 解析 peer.id (open_id) = ou_xxx                                           │
└─────────────────────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  路由判断（bindings）                                                         │
│  - 精确匹配 peer.id                                                          │
│  - 最具体规则优先 → fallback 默认 Agent                                       │
└─────────────────────────────────────────────────────────────────────────────┘
          │
          ├──────────────────┬──────────────────┬──────────────────────────────┐
          ▼                  ▼                  ▼                              ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│  管理员 Agent    │ │  普通用户 Agent   │ │  普通用户 Agent   │ │  VIP Agent       │
│  MediaCraft      │ │  MediaCraft      │ │  MediaCraft      │ │  MediaCraft-VIP  │
│  (全功能)        │ │  (sandbox)       │ │  (sandbox)       │ │  (独立 workspace)│
│  session:       │ │  session:        │ │  session:        │ │  session:        │
│  feishu:dm:     │ │  feishu:dm:      │ │  feishu:dm:      │ │  feishu:dm:      │
│  ou_admin       │ │  ou_user1        │ │  ou_user2        │ │  ou_vip          │
└─────────────────┘ └─────────────────┘ └─────────────────┘ └─────────────────┘
```

### 2.2 三种隔离策略对比表

| 策略 | Session 级 | Agent 级 | 完全独立实例 |
|------|-----------|----------|--------------|
| **配置方式** | `dmScope: "per-channel-peer"` | `bindings` 按 peer.id 路由 | 每个用户一个 Agent + 独立 workspace |
| **Session 隔离** | ✅ 每用户独立 session | ✅ 每 Agent 独立 session store | ✅ 完全隔离 |
| **Workspace 隔离** | ❌ 共享 | ❌ 共享（同一 Agent） | ✅ 完全独立 |
| **资源占用** | 低 | 低 | 高（多 Agent 实例） |
| **适用场景** | 多用户共享同一 Agent，轻量 | 按用户分配不同 Agent 能力 | 高安全、多租户 |

---

## 3. 推荐方案：Session 级隔离 + Agent 级按需

### 3.1 Session 级隔离（轻量，推荐大多数场景）

**适用**：所有用户共享同一个 Agent（如 MediaCraft），但需要各自独立的会话上下文和记忆。

**配置**：将 `dmScope` 设置为 `per-channel-peer`：

```json5
// ~/.openclaw/openclaw.json
{
  session: {
    // 按渠道 + 发送者隔离：每个飞书用户有独立 session
    dmScope: "per-channel-peer",
  },
}
```

**效果**：
- 同一个 Agent：`main` 或 MediaCraft
- 每个飞书用户（`ou_xxx`）有独立的 session key：`agent:main:feishu:dm:ou_xxx`
- 会话上下文、记忆、transcript 完全隔离
- 零额外配置，仅改一行

**openclaw.json 配置示例**：

```json5
{
  "session": {
    "dmScope": "per-channel-peer"  // 多用户必配
  },
  "channels": {
    "feishu": {
      "enabled": true,
      "dmPolicy": "pairing",
      "accounts": {
        "main": {
          "appId": "cli_xxx",
          "appSecret": "xxx",
          "botName": "我的AI助手"
        }
      }
    }
  }
}
```

### 3.2 Agent 级隔离（高安全场景）

**适用**：为特定用户分配独立 Agent（独立 workspace、agentDir、session store），完全隔离数据和能力。

**配置**：为每个用户创建独立 Agent，并通过 `bindings` 按飞书 `open_id` 精确路由：

```json5
{
  "agents": {
    "list": [
      {
        "id": "main",
        "default": true,
        "workspace": "~/.openclaw/workspace",
        "agentDir": "~/.openclaw/agents/main/agent"
      },
      {
        "id": "vip-user",
        "workspace": "~/.openclaw/workspace-vip",
        "agentDir": "~/.openclaw/agents/vip-user/agent"
      }
    ]
  },
  "bindings": [
    {
      "agentId": "vip-user",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "dm", "id": "ou_vip_user_open_id" }
      }
    },
    { "agentId": "main", "match": { "channel": "feishu" } }
  ]
}
```

**注意**：`bindings` 规则按**最具体优先**顺序匹配，`peer` 精确匹配优先于 channel 级匹配，因此 `peer` 规则应放在前面。

### 3.3 混合方案（推荐）

**管理员**（用户自己）→ 全功能 Agent，无 sandbox  
**普通用户** → 受限 Agent（sandbox 模式，工具限制）  
**VIP 用户** → 独立 Agent（可选）

```json5
{
  "agents": {
    "list": [
      {
        "id": "admin",
        "default": true,
        "workspace": "~/.openclaw/workspace",
        "sandbox": { "mode": "off" }
      },
      {
        "id": "guest",
        "workspace": "~/.openclaw/workspace",
        "sandbox": {
          "mode": "all",
          "scope": "session",
          "workspaceAccess": "none",
          "docker": {
            "binds": ["~/.openclaw/workspace/guests/bob:/workspace:rw"]
          }
        },
        "tools": {
          "allow": ["read", "write", "edit", "exec", "process"],
          "deny": ["browser", "canvas", "nodes", "cron", "gateway"]
        }
      }
    ]
  },
  "bindings": [
    {
      "agentId": "admin",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "dm", "id": "ou_admin_open_id" }
      }
    },
    {
      "agentId": "guest",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "dm", "id": "ou_user1" }
      }
    },
    {
      "agentId": "guest",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "dm", "id": "ou_user2" }
      }
    },
    { "agentId": "admin", "match": { "channel": "feishu" } }
  ]
}
```

---

## 4. multi-user-workspace Skill 配置

### 4.1 安装和启用

```bash
npx playbooks add skill openclaw/skills --skill multi-user-workspace
```

在 `openclaw.json` 的 `skills.entries` 中启用：

```json5
{
  "skills": {
    "entries": {
      "multi-user-workspace": { "enabled": true }
    }
  }
}
```

### 4.2 USER.md 中的用户注册

在 workspace 的 `USER.md` 中注册所有用户，包含 `userId` 和角色分配：

```markdown
# User Registry

## Users

### admin
- UserId: admin
- Name: 管理员
- Role: administrator

### user1
- UserId: user1
- Name: 张三
- Role: power-user

### user2
- UserId: user2
- Name: 李四
- Role: basic-user

### vip
- UserId: vip
- Name: 王五
- Role: power-user
```

**权限分级**：`administrator` / `power-user` / `basic-user`，用于映射到 sandbox 和工具权限配置。

### 4.3 FRIENDS/ 用户档案

每用户一个 `{userId}.md` 文件：

```
workspace/
├── USER.md
├── FRIENDS/
│   ├── admin.md
│   ├── user1.md
│   ├── user2.md
│   └── vip.md
```

示例 `FRIENDS/user1.md`：

```markdown
# 张三

## Info
- UserId: user1
- Name: 张三
- Role: power-user
- 飞书 open_id: ou_user1

## Assistant Relationship
- 偏好简洁回复
- 常用文生图

## Notes
- 项目：xxx
```

### 4.4 RELATIONS/ 信息共享控制

文件命名：`{userId1}-{userId2}.md`（字母序）

```markdown
# admin & user1

## Users
- **admin**: 管理员
- **user1**: 张三

## Information Sharing
- 可提及 admin 的公开项目
- 不共享 admin 的私人日程
```

### 4.5 权限分级与映射

| Role | 说明 | sandbox | 工具权限 |
|------|------|---------|---------|
| `administrator` | 管理员 | 关闭 | 全部 |
| `power-user` | 高级用户 | 可选 | 大部分（除 gateway、cron） |
| `basic-user` | 普通用户 | 启用 | read/write/edit/exec/process |

---

## 5. 安全加固

### 5.1 sandbox 模式配置

对外部用户（非管理员）启用 sandbox：

```json5
{
  "agents": {
    "list": [
      {
        "id": "guest",
        "sandbox": {
          "mode": "all",
          "scope": "session",
          "workspaceAccess": "none",
          "docker": {
            "binds": ["~/.openclaw/workspace/guests/{userId}:/workspace:rw"]
          }
        }
      }
    ]
  }
}
```

### 5.2 工具权限限制

对外部用户禁用以下 Skill/工具：

| 工具 | 风险 | 建议 |
|------|------|------|
| `exec` | 任意命令执行 | 对外部用户 deny |
| `browser` | 网页访问 | deny |
| `gateway` | 网关控制 | deny |
| `cron` | 定时任务 | deny |
| `write` / `edit` | 主 workspace 修改 | 仅限 sandbox 内目录 |

### 5.3 敏感数据保护

- **API Key**：通过 `sandbox.docker.env` 或环境变量注入，**不**暴露给外部用户 session 的 workspace
- **主 workspace**：`workspaceAccess: "none"` 避免外部用户读取 USER.md、FRIENDS/、RELATIONS/
- **敏感文件**：放在 `private/` 等目录，仅管理员 Agent 可访问

### 5.4 审计日志

```bash
# 查看会话列表
openclaw sessions --json

# 查看特定 Agent 的 session
openclaw sessions --json --active-key agent:main:feishu:dm:ou_xxx

# 安全审计
openclaw security audit
```

---

## 6. 飞书侧配置

### 6.1 如何获取用户的 open_id

**方法一（推荐）**：查看日志

```bash
openclaw logs --follow
```

用户发消息时，日志中会输出 `open_id`（格式如 `ou_28b31a88...`）。

**方法二**：配对请求列表

```bash
openclaw pairing list feishu
```

待审批列表中会显示每个用户的 open_id。

**方法三**：飞书 API 调试工具获取机器人所在群组/用户列表。

### 6.2 用户首次使用的准入流程（pairing approve）

默认 `dmPolicy: "pairing"`，陌生用户会收到配对码，管理员批准后才能对话：

```bash
# 查看待审批列表
openclaw pairing list feishu

# 批准指定用户
openclaw pairing approve feishu <配对码>
```

**白名单模式**：若无需配对，直接允许指定用户：

```json5
{
  "channels": {
    "feishu": {
      "dmPolicy": "allowlist",
      "allowFrom": ["ou_admin", "ou_user1", "ou_user2"]
    }
  }
}
```

### 6.3 群聊 vs 私聊的隔离差异

| 类型 | Session 隔离 | 说明 |
|------|-------------|------|
| **私聊** | 按 `dmScope` 决定 | `per-channel-peer` 时每个用户独立 session |
| **群聊** | 按群组 ID 隔离 | 每个群组独立 session：`agent:main:feishu:group:oc_xxx` |

群聊中如需按发送者隔离，需在群组内单独配置 `groups..allowFrom` 限制可发言用户，或通过 bindings 将特定群组路由到专用 Agent。

---

## 7. 完整配置示例

以下是一个可直接部署的 `openclaw.json` 示例，包含：

- 管理员路由（全功能）
- 3 个普通用户路由（sandbox）
- 默认 fallback 路由
- dmScope 和安全配置

```json5
{
  // ═══════════════════════════════════════════════════════════
  // 多用户路由隔离 - 完整配置
  // ═══════════════════════════════════════════════════════════

  "agent": {
    "model": "dashscope/qwen3.5-plus",
    "fallbackModels": [
      "dashscope/qwen3-coder-plus",
      "deepseek/deepseek-chat"
    ],
    "name": "MediaCraft",
    "theme": "专业多媒体 AI 创作助手",
    "emoji": "🎨"
  },

  "session": {
    "dmScope": "per-channel-peer"
  },

  "security": {
    "sensitiveData": {
      "patterns": ["sk-*", "key-*", "token-*", "secret-*"]
    }
  },

  "agents": {
    "defaults": {
      "bootstrapMaxChars": 20000,
      "maxContextTokens": 128000,
      "maxResponseTokens": 8192,
      "temperature": 0.7,
      "compaction": { "memoryFlush": true },
      "sandbox": {
        "mode": "non-main",
        "docker": {
          "env": {
            "DASHSCOPE_API_KEY": "${DASHSCOPE_API_KEY}",
            "FAL_KEY": "${FAL_KEY}",
            "GEMINI_API_KEY": "${GEMINI_API_KEY}",
            "DEEPSEEK_API_KEY": "${DEEPSEEK_API_KEY}"
          }
        }
      }
    },
    "list": [
      {
        "id": "admin",
        "default": true,
        "workspace": "~/.openclaw/workspace",
        "agentDir": "~/.openclaw/agents/admin/agent",
        "sandbox": { "mode": "off" }
      },
      {
        "id": "guest",
        "workspace": "~/.openclaw/workspace",
        "agentDir": "~/.openclaw/agents/guest/agent",
        "sandbox": {
          "mode": "all",
          "scope": "session",
          "workspaceAccess": "none",
          "docker": {
            "binds": ["~/.openclaw/workspace/guests:/workspace:rw"],
            "env": {
              "DASHSCOPE_API_KEY": "${DASHSCOPE_API_KEY}",
              "DEEPSEEK_API_KEY": "${DEEPSEEK_API_KEY}"
            }
          }
        },
        "tools": {
          "allow": ["read", "write", "edit", "exec", "process"],
          "deny": ["browser", "canvas", "nodes", "cron", "gateway"]
        }
      }
    ]
  },

  "bindings": [
    {
      "agentId": "admin",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "dm", "id": "ou_admin_open_id" }
      }
    },
    {
      "agentId": "guest",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "dm", "id": "ou_user1_open_id" }
      }
    },
    {
      "agentId": "guest",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "dm", "id": "ou_user2_open_id" }
      }
    },
    {
      "agentId": "guest",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "dm", "id": "ou_user3_open_id" }
      }
    },
    { "agentId": "admin", "match": { "channel": "feishu" } }
  ],

  "channels": {
    "feishu": {
      "enabled": true,
      "dmPolicy": "pairing",
      "accounts": {
        "main": {
          "appId": "cli_xxx",
          "appSecret": "xxx",
          "botName": "MediaCraft AI"
        }
      }
    }
  },

  "skills": {
    "load": {
      "extraDirs": ["~/.openclaw/skills"],
      "watch": true,
      "watchDebounceMs": 250
    },
    "entries": {
      "multi-user-workspace": { "enabled": true },
      "secure-install": { "enabled": true },
      "clawsec-suite": { "enabled": true },
      "smart-ocr": { "enabled": true },
      "pdf-text-extractor": { "enabled": true },
      "pollinations": { "enabled": true },
      "siliconflow-image-gen": { "enabled": true },
      "gemini-image-gen": { "enabled": true },
      "fal-ai": { "enabled": true },
      "memory-tools": { "enabled": true }
    }
  },

  "output": {
    "baseDir": "~/.openclaw/workspace/output",
    "images": "~/.openclaw/workspace/output/images",
    "videos": "~/.openclaw/workspace/output/videos"
  }
}
```

**部署前替换**：
- `ou_admin_open_id`、`ou_user1_open_id`、`ou_user2_open_id`、`ou_user3_open_id` → 实际飞书 open_id
- `cli_xxx`、`appSecret` → 实际飞书应用凭证

**创建 guest 目录**：

```bash
mkdir -p ~/.openclaw/workspace/guests
```

---

## 8. 已知限制和注意事项

### 8.1 飞书 bindings 路由

- **已知问题**：部分版本中 `bindings` 按 `peer.id` 路由可能失败，所有消息路由到默认 agent（见 [Issue #32678](https://github.com/openclaw/openclaw/issues/32678)）
- **临时方案**：若 bindings 不生效，可先依赖 `dmScope: "per-channel-peer"` 实现会话隔离，再关注官方修复

### 8.2 dmScope 变更后的会话残留

- 从 `main` 改为 `per-channel-peer` 后，旧 session `agent:main:main` 可能残留，导致重复投递
- **处理**：手动删除 `~/.openclaw/agents/main/sessions/sessions.json` 中对应条目

### 8.3 同一人多渠道的会话合并

- 若同一用户同时在飞书和 WebChat 使用，默认会得到两个独立 session
- **合并**：使用 `session.identityLinks` 将 `feishu:ou_xxx` 映射到同一 canonical identity

### 8.4 群聊与私聊的隔离

- 群聊：每个群组一个 session（`group:oc_xxx`），群内所有成员共享
- 私聊：`per-channel-peer` 下每个用户独立 session
- 群聊中无法按发送者隔离上下文，除非将群组路由到专用 Agent

### 8.5 资源与配额

- 多用户共享同一 Agent 时，API 配额（百炼、DeepSeek 等）为所有用户共享
- 建议：为外部用户启用 sandbox 并限制工具，避免滥用

### 8.6 验证配置

```bash
# 列出所有 agent 及 bindings
openclaw agents list --bindings

# 安全审计
openclaw security audit

# 查看 session 列表
openclaw sessions --json
```

---

*文档版本: 1.0.0 | 最后更新: 2026-03-08*
