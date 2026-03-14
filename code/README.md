# OpenClaw Multi-Agent Collaboration System (V5)

基于 V5 设计方案的多 Agent 协作系统实现，包含 Go 后端服务和 OpenClaw TypeScript 插件。

## 架构

```
┌──────────┐    WebSocket    ┌────────────────────────────┐
│  飞书群聊  │◄──────────────►│   OpenClaw v2026.3.13      │
│  飞书私聊  │               │  路由/隔离/LLM/记忆管理      │
└──────────┘               └───────────┬────────────────┘
                                       │
                            ┌──────────▼──────────────┐
                            │  @team/collab 插件 (TS)  │
                            │  6 Hooks + 7 Tools       │
                            │  3 Commands + Bridge     │
                            └──────────┬──────────────┘
                                       │ HTTP
                            ┌──────────▼──────────────┐
                            │   Go 协作服务 :8090       │
                            │  Registry / Task / Proj  │
                            │  Feishu / Dashboard      │
                            │  Scheduler               │
                            └──────────┬──────────────┘
                                       │
                            ┌──────────▼──────────────┐
                            │     PostgreSQL :5432      │
                            └─────────────────────────┘
```

## 快速开始

### 前置条件

- Go 1.23+
- Node.js 20+
- Docker (用于 PostgreSQL)
- OpenClaw v2026.3.13

### 启动

```bash
# 1. 启动基础设施（PostgreSQL）
make docker-up

# 2. 构建所有组件
make build

# 3. 运行 Go 服务
make run-go

# 4. 安装 TS 插件到 OpenClaw
cd ts-plugins/collab && npm install && npm run build
```

### 常用命令

```bash
make test        # 运行所有测试
make test-go     # Go 单元测试
make test-ts     # TS 单元测试
make lint        # 代码检查
make db-shell    # 进入数据库 CLI
make db-reset    # 重置数据库
make help        # 查看所有命令
```

## 模块总览

### Go 服务 (14 个 API 组)

| 模块 | 路径 | 功能 |
|------|------|------|
| Registry | `/api/v1/agents` | Agent 注册/发现/状态 |
| Task | `/api/v1/tasks` | DAG 任务树 + 交付物 + 活动记录 |
| Project | `/api/v1/projects` | 项目 + 迭代 + 结构化记忆 |
| Feishu | `/api/v1/feishu` | Bot 选择 + 消息转发 |
| Dashboard | `/api/v1/dashboard` | 全局概览 + 告警 |
| Scheduler | cron | 卡任务检测 + Agent 健康 |

### TS 插件 (6 Hooks + 7 Tools + 3 Commands)

| Hook | 触发时机 | 作用 |
|------|---------|------|
| `before_agent_start` | Agent 推理前 | 注入协作上下文 |
| `message_sending` | 消息出站 | 角色前缀格式化 |
| `before_tool_call` | 工具调用前 | 监控 Agent 间通信 |
| `after_tool_call` | 工具调用后 | 记录活动/错误 |
| `agent_end` | Agent 结束 | 异常处理/状态重置 |
| `before_compaction` | 记忆压缩前 | 同步任务状态到 Go |

| Tool | 名称 | 功能 |
|------|------|------|
| create_task | 创建任务 | 支持 DAG 依赖 |
| update_task | 更新任务 | 状态机 + 级联传播 |
| query_tasks | 查询任务 | 多维度过滤 |
| save_artifact | 保存交付物 | 文档/代码/报告 |
| create_project | 创建项目 | 绑定飞书群 |
| save_decision | 保存决策 | Pin 到上下文 |
| query_project_memory | 查询记忆 | 关键词 + 分类 |

| Command | 用法 | 功能 |
|---------|------|------|
| /status | `/status [project_id]` | 状态概览 |
| /pause | `/pause <task\|project>` | 暂停任务 |
| /tasks | `/tasks [filters]` | 任务列表 |

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | 8090 | Go 服务端口 |
| `DATABASE_DSN` | localhost:5432/collab | PostgreSQL 连接 |
| `OPENCLAW_DIR` | ~/.openclaw | OpenClaw 工作目录 |
| `DEFAULT_BOT_ID` | | 默认飞书机器人 App ID |
| `FEISHU_APP_ID` | | 飞书应用 ID |
| `FEISHU_APP_SECRET` | | 飞书应用 Secret |

## 文档

| 文档 | 说明 |
|------|------|
| **[完整部署与测试手册](docs/full-deploy-and-test.md)** | **编译→K8s部署→插件安装→测试Agent→5级测试方案（推荐先读）** |
| [编译部署指南](docs/build-deploy.md) | 前置条件、编译方式、三种部署方案、端口冲突、数据库管理、故障排查 |
| [OpenClaw 集成指南](docs/openclaw-integration.md) | 6 步集成流程、openclaw.json 配置、Agent 协作协议、飞书 Binding、验证清单 |
| [测试指南](docs/testing-guide.md) | 测试架构、运行方式、62 个测试用例详表、CI 配置、测试规范 |

## 测试覆盖

- **Go**: 25 个单元测试覆盖 Registry/Task/Project/Feishu/Dashboard
- **TS**: 37 个单元测试覆盖 Hooks/Tools/Commands/GoBridge
- **集成测试**: curl E2E 验证全链路 API
- **详情**: 见 [测试指南](docs/testing-guide.md)
