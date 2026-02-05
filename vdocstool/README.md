# Agent Tools for Eino

基于 Eino 框架的 Agent 工具集，支持 MCP (Model Context Protocol) 协议。

## 功能特性

### 1. 网页搜索工具 (Web Search)

- **多引擎支持**: DuckDuckGo、Bing、百度、Serper API
- **自适应路由**: 自动检测引擎健康状态，智能切换
- **熔断降级**: 连续失败自动熔断，定期探测恢复
- **内容提取**: 支持网页内容深度提取

### 2. 邮件工具 (Email)

- **多邮箱支持**: QQ邮箱、163邮箱、126邮箱、Gmail
- **意图分析**: 基于主题、内容、附件的智能分类
- **附件管理**: 支持附件下载、类型识别
- **IMAP/SMTP**: 完整的邮件收发功能

### 3. 记忆工具 (Memory)

- **三级存储**: L1(Redis) / L2(Milvus+PostgreSQL) / L3(S3+Neo4j)
- **主题胶囊**: 智能归纳对话主题，生成摘要向量
- **实体图谱**: 提取实体关系，构建知识网络
- **多层检索**: 意图解析 → L1匹配 → 多路召回 → 重排序 → 上下文组装

## 项目结构

```
vdocstool/
├── cmd/mcp-server/          # MCP Server 入口
├── tools/
│   ├── search/              # 网页搜索工具
│   │   ├── router/          # 自适应路由
│   │   └── engines/         # 搜索引擎适配器
│   ├── email/               # 邮件工具
│   │   ├── intent/          # 意图分析
│   │   └── providers/       # 邮箱提供商
│   └── memory/              # 记忆工具
│       ├── storage/         # 三级存储
│       ├── capsule/         # 主题胶囊
│       ├── retrieval/       # 检索管道
│       └── capacity/        # 容量管理
├── mcp/                     # MCP Server 封装
├── skills/                  # Skills 指导文档
└── config/                  # 配置管理
```

## 快速开始

### 安装依赖

```bash
cd vdocstool
go mod tidy
```

### 配置

复制配置模板并修改：

```bash
cp config.example.json config.json
# 编辑 config.json 配置各服务连接信息
```

或通过环境变量配置：

```bash
export REDIS_ADDR=localhost:6379
export MILVUS_ADDR=localhost:19530
export SERPER_API_KEY=your_api_key
```

### 启动 MCP Server

```bash
go run cmd/mcp-server/main.go
```

## MCP 工具列表

### 搜索工具

| 工具名 | 说明 |
|--------|------|
| `web_search` | 执行网页搜索（支持自动降级） |
| `check_engine_health` | 检查搜索引擎健康状态 |
| `set_routing_strategy` | 设置路由策略 |

### 邮件工具

| 工具名 | 说明 |
|--------|------|
| `search_email_by_intent` | 基于意图搜索邮件 |
| `analyze_email` | 分析单封邮件 |
| `find_attachments` | 按意图查找附件 |
| `read_email` | 读取邮件详情 |
| `send_email` | 发送邮件 |

### 记忆工具

| 工具名 | 说明 |
|--------|------|
| `store_memory` | 存储记忆（自动分层） |
| `retrieve_context` | 多层检索组装上下文 |
| `switch_topic` | 切换主题，归档当前L1 |
| `recall_topic` | 召回历史主题到L1 |
| `get_entity_relations` | 查询实体关联图谱 |
| `summarize_session` | 生成会话摘要 |

## Skills 使用指南

参考 `skills/` 目录下的文档：

- [web-search-skill.md](skills/web-search-skill.md) - 网页搜索工具使用指南
- [email-skill.md](skills/email-skill.md) - 邮件工具使用指南
- [memory-skill.md](skills/memory-skill.md) - 记忆工具使用指南

## 依赖服务

- **Redis**: L1 工作记忆存储
- **Milvus**: L2 向量语义检索
- **PostgreSQL**: L2 元数据存储
- **S3 (MinIO)**: L3 归档存储
- **Neo4j**: L3 知识图谱

## License

Apache License 2.0
