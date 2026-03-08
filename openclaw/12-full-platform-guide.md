# OpenClaw 全平台 Agent 部署指南

> **适用场景**：中国大陆 mini PC 单机部署，飞书多群/多用户接入，6 个 Agent 团队协同服务  
> **版本**：1.0.0 | 最后更新：2026-03-08

---

## 1. 平台概览

### 1.1 整体架构

本方案在一台 mini 主机上运行 **OpenClaw Gateway**，通过飞书 WebSocket 长连接接收消息，将不同飞书群/私聊路由到 6 个 Agent 团队，实现多用户、多场景的智能服务。

**核心特性**：
- **6 个 Agent 团队**：mediacraft、dev-team、finance-team、xiaohongshu、wechat-mp、email-mgr
- **约 25 个 Skills**：覆盖 OCR、文生图/视频、金融、研发、内容运营、邮件等
- **多用户隔离**：`dmScope: per-channel-peer` 按渠道+发送者隔离会话
- **国内 API 优先**：全部使用中国大陆可直连的 API，无 Google/fal.ai/krea 依赖

### 1.2 ASCII 架构图

```
飞书用户群
├── 研发项目Alpha群 → DevTeam Agent (7角色)
├── 研发项目Beta群  → DevTeam Agent (复用)
├── 金融分析群      → FinanceTeam Agent (6角色)
├── 小红书运营群    → RedNote Agent
├── 公众号运营群    → WeChatEditor Agent
├── 邮件管理(私聊)  → EmailAssistant Agent
└── 多媒体(私聊)    → MediaCraft Agent
         │
    OpenClaw Gateway (mini 主机)
    ├── smart-router (意图路由)
    ├── dmScope: per-channel-peer (用户隔离)
    └── 阿里百炼 Coding Plan (LLM)
```

### 1.3 数据流说明

| 层级 | 组件 | 说明 |
|------|------|------|
| **入口** | 飞书群 / 飞书私聊 | 用户 @机器人 或 私聊发送消息 |
| **路由** | bindings + smart-router | 群聊按群 ID 静态路由，私聊可智能路由 |
| **隔离** | dmScope: per-channel-peer | 每个飞书用户/群有独立 session，互不干扰 |
| **LLM** | 阿里百炼 DashScope | qwen3.5-plus / qwen3-coder-plus / kimi-k2.5 |
| **Skills** | ~25 个 | 按 Agent 需求加载，沙箱内执行 |

---

## 2. 中国大陆 API 可用性评估（重要）

### 2.1 可用性总表

**说明**：此前推荐的 Gemini、fal.ai、krea 等因中国大陆网络限制已移除，所有生成能力均通过国内可直连 API 实现。

| API/服务 | 中国大陆可用 | 免费额度 | 实际状态(2026.03) | 替代方案 |
|---------|------------|---------|-------------------|---------|
| **阿里百炼 DashScope** | ✅ 直连 | Coding Plan 已购 | 稳定 | - |
| **硅基流动 SiliconFlow** | ✅ 直连 | Flux Schnell 免费 | 稳定 | - |
| **即梦 Jimeng** | ✅ 直连 | 每日 66 积分 | 稳定 | - |
| **可灵 Kling** | ✅ 直连 | 每月 366 积分 | 稳定 | - |
| **DeepSeek** | ✅ 直连 | 注册赠送 | 稳定 | - |
| **Tushare Pro** | ✅ 直连 | 基础免费 | 稳定 | AKShare |
| **Yahoo Finance** | ✅ 直连 | 完全免费 | 稳定 | - |
| **Google Gemini** | ❌ 被墙 | - | 不可用 | 百炼视觉 / 硅基流动 |
| **fal.ai** | ❌/⚠️ | - | 不稳定 | SiliconFlow |
| **krea-api** | ❌/⚠️ | - | 不稳定 | SiliconFlow |
| **Pollinations** | ⚠️ 不稳定 | 完全免费 | 时好时坏 | 作为最后备选 |

### 2.2 迁移说明

- **文生图**：原 Gemini/fal.ai/krea → 现 **siliconflow-image-gen**（Flux Schnell 免费）+ **pollinations**（兜底）
- **文生视频**：原 fal.ai → 现 **video-gen**（即梦/可灵）+ **siliconflow-video-gen**
- **LLM**：原 Gemini → 现 **阿里百炼**（qwen3.5-plus / qwen3-coder-plus）
- **金融数据**：Tushare + Yahoo Finance 均国内可访问，无需变更

### 2.3 推荐优先级

```
第一梯队（必配）：
  阿里百炼、硅基流动、即梦、可灵、Tushare

第二梯队（推荐）：
  DeepSeek（LLM 备用）、Yahoo Finance（美股/港股）

第三梯队（兜底）：
  Pollinations（文生图最后备选，网络不稳定时可能失败）
```

---

## 3. 飞书路由方案（核心）

### 3.1 三种路由方式对比

| 方案 | 优点 | 缺点 | 适用场景 |
|------|------|------|---------|
| **A. bindings 静态路由** | 简单、确定性、无额外延迟 | 飞书有已知 bug，需手动维护群 ID | 群聊绑定（群 ID 固定） |
| **B. smart-router Skill** | 自动识别意图，无需维护 bindings | 消耗 LLM token，有延迟 | 私聊场景（用户意图多变） |
| **C. 混合方案（推荐）** | 群聊稳定 + 私聊灵活 | 配置稍复杂 | 生产环境 |

### 3.2 方案 A：bindings 静态路由

**原理**：在 `openclaw.json` 的 `bindings` 中，按 `channel` + `peer.kind` + `peer.id` 精确匹配，将消息路由到指定 Agent。

**优点**：
- 配置清晰，行为可预测
- 无 LLM 调用，零延迟

**缺点**：
- 飞书存在已知 bug（[#16354](https://github.com/openclaw/openclaw/issues/16354)、[#32678](https://github.com/openclaw/openclaw/issues/32678)）：部分版本 bindings 不生效，所有消息路由到默认 Agent
- 群 ID 变更需手动更新配置
- 私聊场景下，单应用无法按「用户想找哪个 Agent」区分

**适用**：群聊绑定（一群一 Agent），群 ID 固定且可获取。

**配置示例**：

```json5
{
  "bindings": [
    {
      "agentId": "dev-team",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "group", "id": "oc_rd_project_alpha" }
      }
    },
    {
      "agentId": "finance-team",
      "match": {
        "channel": "feishu",
        "peer": { "kind": "group", "id": "oc_finance_group" }
      }
    }
  ]
}
```

### 3.3 方案 B：smart-router Skill 智能路由

**原理**：默认 Agent 收到消息后，调用 `smart-router` Skill，由 LLM 分析用户意图，决定转发到哪个 Agent。

**优点**：
- 无需维护 bindings，用户说「帮我分析股票」自动路由到 finance-team
- 私聊场景下，单应用即可实现多 Agent 切换

**缺点**：
- 每次路由消耗 LLM token
- 增加 1–3 秒延迟
- 依赖 smart-router Skill 可用性

**适用**：私聊场景，用户意图多变，无法预先绑定。

### 3.4 方案 C：混合方案（推荐）

**策略**：
- **飞书群**：用 bindings 静态绑定（一群一 Agent）
- **飞书私聊**：用 smart-router 智能路由，或绑定默认 Agent（如 mediacraft）作为网关
- **群内 @角色**：由 Agent 内部 AGENTS.md 规则分发（如 dev-team 的 @小架、@小项）

**好处**：
- 群聊稳定、可预测
- 私聊灵活、用户体验好
- 降低 bindings bug 影响范围

**推荐配置示例**：

```json5
{
  "session": {
    "dmScope": "per-channel-peer"
  },
  "bindings": [
    // ─── 群聊：静态绑定 ───
    { "agentId": "mediacraft", "match": { "channel": "feishu", "peer": { "kind": "group", "id": "oc_mediacraft_group" } } },
    { "agentId": "dev-team", "match": { "channel": "feishu", "peer": { "kind": "group", "id": "oc_rd_project_alpha" } } },
    { "agentId": "dev-team", "match": { "channel": "feishu", "peer": { "kind": "group", "id": "oc_rd_project_beta" } } },
    { "agentId": "finance-team", "match": { "channel": "feishu", "peer": { "kind": "group", "id": "oc_finance_group" } } },
    { "agentId": "xiaohongshu", "match": { "channel": "feishu", "peer": { "kind": "group", "id": "oc_xiaohongshu_group" } } },
    { "agentId": "wechat-mp", "match": { "channel": "feishu", "peer": { "kind": "group", "id": "oc_wechat_group" } } },
    { "agentId": "email-mgr", "match": { "channel": "feishu", "peer": { "kind": "group", "id": "oc_email_group" } } },
    // ─── 私聊：默认网关（可配合 smart-router）───
    { "agentId": "mediacraft", "match": { "channel": "feishu" } }
  ]
}
```

**部署前**：将 `oc_xxx` 占位符替换为真实飞书群 ID。群 ID 获取方式：飞书开放平台 → 机器人所在群 → 群聊 ID，或 `openclaw logs --follow` 查看收到消息时的 peer.id。

---

## 4. Agent 团队一览

| Agent ID | 名称 | 角色数 | 核心 Skills | 飞书绑定方式 |
|----------|------|--------|------------|-------------|
| **mediacraft** | 多媒体助手 | Swarm | siliconflow-image-gen, video-gen, smart-ocr | 私聊/指定群 |
| **dev-team** | 研发团队 | 7 角色 | github, memory-tools | 项目飞书群 |
| **finance-team** | 金融团队 | 6 角色 | tushare-finance, yahoo-finance, stock-analysis, portfolio-watcher | 金融飞书群 |
| **xiaohongshu** | 小红书运营 | 1 | xiaohongshu skill | 运营飞书群 |
| **wechat-mp** | 公众号运营 | 1 | wechat-accounts-publisher | 运营飞书群 |
| **email-mgr** | 邮件管理 | 1 | imap-smtp-email | 私聊 |

### 4.1 各 Agent 详细说明

**mediacraft**：Coordinator-Worker Swarm 架构，支持 OCR、文生图、文生视频、儿童成长手册。私聊默认路由到此 Agent，可配合 smart-router 转发到其他 Agent。

**dev-team**：7 个角色（PM/架构/后端/前端/QA/运维/文档），通过 AGENTS.md 规则支持 @小项、@小架 等群内分发。同一套角色可服务多个项目群（Alpha、Beta）。

**finance-team**：6 个角色（首席分析师/选股师/盯盘员/交易助手/研报分析师/风控官），依赖 Tushare、Yahoo Finance 等数据源。所有输出需带免责声明。

**xiaohongshu / wechat-mp**：内容运营 Agent，需对应平台 API 或发布 Skill。

**email-mgr**：IMAP/SMTP 邮件收发，通常绑定私聊或专用群。

---

## 5. 一键部署步骤

### 5.1 准备环境

```bash
# 1. 系统要求
# - Node.js 18+
# - Python 3.9+
# - 网络可访问 ClawHub / npm

# 2. 安装 OpenClaw
npm install -g openclaw@latest

# 3. 安装飞书插件（若未内置）
openclaw plugins install @openclaw/feishu
```

### 5.2 克隆配置仓库

```bash
git clone <your-repo-url> eino
cd eino/openclaw
```

### 5.3 配置环境变量

```bash
# 复制模板
cp .env.example ~/.openclaw/.env
# 若没有 .env.example，直接创建：
touch ~/.openclaw/.env

# 编辑 ~/.openclaw/.env，填写以下 Key（见第 7 节清单）
# DASHSCOPE_API_KEY=
# SILICONFLOW_API_KEY=
# JIMENG_API_KEY=
# KLING_API_KEY=
# TUSHARE_TOKEN=
# FEISHU_APP_ID=
# FEISHU_APP_SECRET=
# DEEPSEEK_API_KEY=  # 可选

# 加载到当前 shell
echo 'source ~/.openclaw/.env' >> ~/.zshrc && source ~/.openclaw/.env
```

### 5.4 运行 Skills 安装脚本

```bash
cd openclaw/agent-config
chmod +x install-skills.sh
./install-skills.sh
```

或按第 6 节清单逐条执行安装命令。

### 5.5 复制 Agent 配置

```bash
CONFIG_DIR="$(pwd)/openclaw/agent-config"

# 主配置
cp "$CONFIG_DIR/openclaw.json" ~/.openclaw/openclaw.json

# 各 Agent workspace（若存在）
for agent in mediacraft dev-team finance-team xiaohongshu wechat-mp email-mgr; do
  mkdir -p ~/.openclaw/workspace/$agent
  [ -d "$CONFIG_DIR/agents/$agent/workspace" ] && cp -r "$CONFIG_DIR/agents/$agent/workspace/"* ~/.openclaw/workspace/$agent/
done

# 默认 workspace（若无 agent 专属）
[ -d "$CONFIG_DIR/workspace" ] && cp -r "$CONFIG_DIR/workspace/"* ~/.openclaw/workspace/

# 输出目录
mkdir -p ~/.openclaw/workspace/output/{images,videos,handbooks}
mkdir -p ~/.openclaw/workspace/memory
```

### 5.6 配置飞书应用

1. 登录 [飞书开放平台](https://open.feishu.cn/)
2. 创建企业自建应用，获取 **App ID**、**App Secret**
3. 开启「机器人」能力
4. 权限：`im:message`、`im:message.group_at_msg`、`im:message.p2p_msg` 等
5. 事件订阅：`im.message.receive_v1`，选择「长连接接收」
6. 将机器人加入目标飞书群

### 5.7 配置 Feishu Bindings

编辑 `~/.openclaw/openclaw.json`，将 bindings 中的占位符替换为真实群 ID：

```json5
// 将 oc_rd_project_alpha 替换为研发项目 Alpha 群的群 ID
// 将 oc_finance_group 替换为金融分析群的群 ID
// 以此类推
```

群 ID 获取：`openclaw logs --follow`，在对应群发一条消息，日志中会打印 `peer.id`。

### 5.8 启动并验证

```bash
# 启动
openclaw start
# 或 openclaw gateway restart

# 验证 Skills
openclaw skill list
# 期望看到 ~25 个 Skills 全部 enabled

# 安全检查
openclaw doctor --security

# 功能验证：在飞书群 @机器人 发送「你好」
```

### 5.9 设置备份 Crontab

```bash
# 每日凌晨 2 点备份
crontab -e
# 添加：
0 2 * * * /path/to/backup.sh >> /var/log/openclaw-backup.log 2>&1
```

---

## 6. Skills 安装清单

### 6.1 完整技能表（约 25 个）

| # | Skill | 类型 | 服务的 Agent | 中国可用 | 安装命令 |
|---|-------|------|-------------|---------|---------|
| 1 | secure-install | 安全 | 全部 | ✅ | `npx playbooks add skill openclaw/skills --skill secure-install` |
| 2 | clawsec-suite | 安全 | 全部 | ✅ | `npx playbooks add skill openclaw/skills --skill clawsec-suite` |
| 3 | smart-ocr | OCR | mediacraft | ✅ | `npx playbooks add skill openclaw/skills --skill smart-ocr` |
| 4 | pdf-text-extractor | OCR | mediacraft, finance-team | ✅ | `npx playbooks add skill openclaw/skills --skill pdf-text-extractor` |
| 5 | siliconflow-image-gen | 文生图 | mediacraft | ✅ | `npx playbooks add skill openclaw/skills --skill siliconflow-image-gen` |
| 6 | siliconflow-video-gen | 文生视频 | mediacraft | ✅ | `npx playbooks add skill openclaw/skills --skill siliconflow-video-gen` |
| 7 | video-gen | 文/图生视频 | mediacraft | ✅ | `npx playbooks add skill openclaw/skills --skill video-gen` |
| 8 | pollinations | 文生图 | mediacraft | ⚠️ | `npx playbooks add skill openclaw/skills --skill pollinations` |
| 9 | tushare-finance | 金融 | finance-team | ✅ | `npx playbooks add skill openclaw/skills --skill tushare-finance` 或 `tushare` |
| 10 | yahoo-finance | 金融 | finance-team | ✅ | `npx playbooks add skill openclaw/skills --skill yahoofinance` 或 `yahoo-finance` |
| 11 | stock-analysis | 金融 | finance-team | ✅ | `npx playbooks add skill openclaw/skills --skill stock-analysis` |
| 12 | portfolio-watcher | 金融 | finance-team | ✅ | `npx playbooks add skill openclaw/skills --skill portfolio-watcher` |
| 13 | xiaohongshu | 内容运营 | xiaohongshu | ✅ | `npx playbooks add skill openclaw/skills --skill xiaohongshu` |
| 14 | wechat-accounts-publisher | 内容运营 | wechat-mp | ✅ | `npx playbooks add skill openclaw/skills --skill wechat-accounts-publisher` |
| 15 | clawhub-publish | 内容运营 | 多 Agent | ✅ | `npx playbooks add skill openclaw/skills --skill clawhub-publish` |
| 16 | imap-smtp-email | 邮件 | email-mgr | ✅ | `npx playbooks add skill openclaw/skills --skill imap-smtp-email` |
| 17 | github | 研发 | dev-team | ✅ | `npx playbooks add skill openclaw/skills --skill github` |
| 18 | memory-tools | 工具 | dev-team, mediacraft, finance-team | ✅ | `npx playbooks add skill openclaw/skills --skill memory-tools` |
| 19 | prompt-enhancer | 工具 | mediacraft | ✅ | `npx playbooks add skill openclaw/skills --skill prompt-enhancer` |
| 20 | smart-router | 工具 | 默认网关 | ✅ | `npx playbooks add skill openclaw/skills --skill smart-router` |
| 21 | backup-and-restore | 工具 | 运维 | ✅ | `npx playbooks add skill openclaw/skills --skill backup-and-restore` |
| 22 | multi-user-workspace | 工具 | 多用户 | ✅ | `npx playbooks add skill openclaw/skills --skill multi-user-workspace` |
| 23 | children-growth-handbook | 自定义 | mediacraft | ✅ | `cp -r agent-config/skills/children-growth-handbook ~/.openclaw/skills/` + `pip install -r ~/.openclaw/skills/children-growth-handbook/scripts/requirements.txt` |
| 24 | api-fallback-guard | 自定义 | mediacraft | ✅ | `cp -r agent-config/skills/api-fallback-guard ~/.openclaw/skills/` |

> 注：部分社区 Skill（如 stock-analysis、portfolio-watcher、xiaohongshu、wechat-accounts-publisher）可能需从 ClawHub 搜索或自建，安装命令以实际可用为准。

### 6.2 一键安装脚本

使用 `agent-config/install-skills.sh` 可批量安装上述 Skills，脚本会自动处理可选 Skill 的缺失情况。

---

## 7. API Key 申请清单

| # | 平台 | 申请地址 | 环境变量 | 必要性 | 服务的 Agent |
|---|------|---------|---------|--------|-------------|
| 1 | 阿里百炼 | https://bailian.console.aliyun.com/ | DASHSCOPE_API_KEY | **必须** | 全部（LLM） |
| 2 | 硅基流动 | https://cloud.siliconflow.cn/ | SILICONFLOW_API_KEY | **必须** | mediacraft（文生图/视频） |
| 3 | 即梦 AI | https://jimeng.jianying.com/ → API 开放平台 | JIMENG_API_KEY | **推荐** | mediacraft（视频） |
| 4 | 可灵 AI | https://klingai.com/ → 开发者平台 | KLING_API_KEY | **推荐** | mediacraft（视频） |
| 5 | Tushare Pro | https://tushare.pro/ | TUSHARE_TOKEN | **必须**（金融） | finance-team |
| 6 | 飞书开放平台 | https://open.feishu.cn/ | FEISHU_APP_ID, FEISHU_APP_SECRET | **必须** | 全部（飞书接入） |
| 7 | DeepSeek | https://platform.deepseek.com/ | DEEPSEEK_API_KEY | 可选 | LLM fallback |
| 8 | Yahoo Finance | 无需 | 无 | 可选 | finance-team（美股/港股） |

### 7.1 申请顺序建议

```
Day 1（核心，约 15 分钟）：
  ✅ 阿里百炼 → DASHSCOPE_API_KEY
  ✅ 硅基流动 → SILICONFLOW_API_KEY
  ✅ 飞书开放平台 → FEISHU_APP_ID, FEISHU_APP_SECRET
  ✅ Tushare（若用金融 Agent）→ TUSHARE_TOKEN

Day 2（扩展）：
  ✅ 即梦 AI → JIMENG_API_KEY
  ✅ 可灵 AI → KLING_API_KEY
  ✅ DeepSeek（可选）→ DEEPSEEK_API_KEY
```

---

## 8. 备份与灾难恢复

### 8.1 备份策略

**3-2-1 原则**：
- **3** 份数据：生产 + 本地备份 + 异地备份
- **2** 种介质：磁盘 + 云存储/NAS
- **1** 份异地：防止机房级故障

**备份内容清单**：

| 目录/文件 | 说明 |
|----------|------|
| ~/.openclaw/openclaw.json | 主配置 |
| ~/.openclaw/.env | 环境变量（含 API Key，需加密或排除） |
| ~/.openclaw/workspace/ | 各 Agent workspace、MEMORY.md、输出 |
| ~/.openclaw/skills/ | 自定义 Skills |
| ~/.openclaw/agents/ | Agent 目录（若有） |

**自动备份脚本 `backup.sh`**：

```bash
#!/bin/bash
# OpenClaw 备份脚本
BACKUP_DIR="${BACKUP_DIR:-$HOME/openclaw-backups}"
DATE=$(date +%Y%m%d_%H%M%S)
mkdir -p "$BACKUP_DIR"

# 备份配置（排除 .env 中的敏感内容，或使用加密）
tar -czf "$BACKUP_DIR/openclaw_$DATE.tar.gz" \
  -C "$HOME" \
  .openclaw/openclaw.json \
  .openclaw/workspace \
  .openclaw/skills \
  .openclaw/agents 2>/dev/null || true

# 保留最近 7 天
find "$BACKUP_DIR" -name "openclaw_*.tar.gz" -mtime +7 -delete
echo "Backup done: $BACKUP_DIR/openclaw_$DATE.tar.gz"
```

**Crontab 配置**：

```bash
# 每日凌晨 2 点
0 2 * * * /path/to/backup.sh >> /var/log/openclaw-backup.log 2>&1
```

### 8.2 一键恢复

**恢复脚本 `restore.sh`**：

```bash
#!/bin/bash
# 用法: ./restore.sh <备份文件路径>
BACKUP_FILE="$1"
if [ -z "$BACKUP_FILE" ] || [ ! -f "$BACKUP_FILE" ]; then
  echo "Usage: $0 <path-to-backup.tar.gz>"
  exit 1
fi

# 解压到临时目录，再复制到 ~/.openclaw
TMP=$(mktemp -d)
tar -xzf "$BACKUP_FILE" -C "$TMP"
cp -r "$TMP"/.openclaw/* "$HOME/.openclaw/"
rm -rf "$TMP"
echo "Restore done. Please restart OpenClaw."
```

**恢复验证步骤**：

1. 执行 `restore.sh <备份文件>`
2. 检查 `~/.openclaw/openclaw.json` 是否正确
3. 检查 `~/.openclaw/workspace/` 下各 Agent 目录
4. 重启 OpenClaw：`openclaw start`
5. 在飞书发送测试消息验证

**恢复后检查清单**：

- [ ] openclaw.json 语法正确
- [ ] .env 已重新配置（若备份时排除）
- [ ] Skills 目录完整
- [ ] 飞书 bindings 中的群 ID 仍有效
- [ ] 各 Agent 可正常响应

### 8.3 异地备份

**rsync 到 NAS/另一台机器**：

```bash
# 将备份同步到 NAS
rsync -avz "$BACKUP_DIR/" user@nas:/volume1/openclaw-backups/
```

**阿里云 OSS 备份选项**：

```bash
# 需安装 ossutil
ossutil cp "$BACKUP_DIR/openclaw_$DATE.tar.gz" oss://your-bucket/openclaw/
```

---

## 9. 月度成本总结

| 项目 | 成本 | 说明 |
|------|------|------|
| 阿里百炼 Coding Plan | ¥7.9–200/月 | 用户已购，驱动全部 LLM |
| 硅基流动 | **¥0** | Flux Schnell 免费 |
| 即梦 AI | **¥0** | 每日 66 积分免费 |
| 可灵 AI | **¥0** | 每月 366 积分免费 |
| Tushare Pro | **¥0** | 基础免费（高频需积分） |
| Yahoo Finance | **¥0** | 完全免费 |
| DeepSeek | **¥0** | 注册赠送 |
| 飞书 | **¥0** | 基础版免费 |
| Pollinations | **¥0** | 完全免费 |
| **外部 API 月支出** | **¥0** | 仅用免费额度 |
| **总月度成本** | **¥7.9–200** | 主要为百炼 Coding Plan |

---

## 10. 已知限制与 Workaround

### 10.1 飞书 Bindings Bug

| 问题 | Issue | 现象 | Workaround |
|------|-------|------|------------|
| 多 Agent 路由失败 | #16354, #32678 | 所有消息路由到 default agent | **方案 1**：使用 `systemPromptAppend` 在默认 Agent 中注入「当前会话应模拟的角色」，根据群 ID 动态切换人格。<br>**方案 2**：为每个 Agent 创建独立飞书应用（不同 appId），通过 `match.accountId` 路由。 |

### 10.2 Pollinations 中国大陆访问

| 问题 | 说明 | Workaround |
|------|------|------------|
| 网络不稳定 | 时好时坏，可能超时 | 作为文生图最后兜底，主用 siliconflow-image-gen；失败时重试或提示用户稍后。 |

### 10.3 Mini 主机资源限制

| 问题 | 说明 | Workaround |
|------|------|------------|
| 内存不足 | 多 Agent 并发时内存占用高 | 降低 `subagents.maxConcurrent`，限制并行 Worker 数。 |
| CPU 占用 | smart-ocr 等本地计算密集 | 设置 `OCR_USE_GPU: false` 或使用轻量 OCR。 |
| 磁盘空间 | workspace 与输出目录增长 | 定期清理 output 目录，设置备份保留策略。 |

### 10.4 Session 记忆限制

| 问题 | 说明 | Workaround |
|------|------|------------|
| 上下文长度 | 长对话可能超出 maxContextTokens | 启用 `compaction.memoryFlush`，依赖 MEMORY.md 持久化关键信息。 |
| 多用户 session 残留 | dmScope 变更后旧 session 可能残留 | 手动清理 `~/.openclaw/agents/*/sessions/sessions.json`。 |

### 10.5 验证命令

```bash
# 列出 Agent 及 bindings
openclaw agents list --bindings

# 安全审计
openclaw doctor --security

# 查看 session 列表
openclaw sessions --json

# 查看飞书配对请求
openclaw pairing list feishu
```

---

## 附录：相关文档

| 文档 | 说明 |
|------|------|
| [06-mediacraft-agent-deploy-guide.md](./06-mediacraft-agent-deploy-guide.md) | MediaCraft 详细部署 |
| [07-deploy-checklist.md](./07-deploy-checklist.md) | API + Skills 部署清单 |
| [08-rd-agent-team-plan.md](./08-rd-agent-team-plan.md) | 研发 Agent 团队方案 |
| [09-multi-user-routing-plan.md](./09-multi-user-routing-plan.md) | 多用户路由隔离方案 |
| [10-finance-agent-team-plan.md](./10-finance-agent-team-plan.md) | 金融 Agent 群方案 |

---

*文档版本: 1.0.0 | 最后更新: 2026-03-08*
