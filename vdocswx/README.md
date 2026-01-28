# 微信公众号文章润色专家 Agent

基于 [Eino](https://github.com/cloudwego/eino) 框架开发的微信公众号文章润色专家 Agent，专注于提升文章质量。

## 功能特性

### 核心功能
- **语法检查**：错别字、标点符号、语法结构检查与修正
- **逻辑优化**：论点论据分析、逻辑连贯性检查
- **结构调整**：文章大纲优化、段落重组建议
- **风格统一**：语言风格统一、表达方式优化
- **多轮交互**：支持上传文章后多轮对话修改

### 技术特性
- **多Agent协作**：主Agent协调多个子Agent并行处理
- **意图识别**：自动识别用户意图，智能选择处理策略
- **索引系统**：倒排索引支持快速内容搜索
- **缓存系统**：LRU缓存减少重复计算，提升响应速度
- **模型路由**：支持多种LLM，智能路由选择最优模型
- **统计监控**：Token使用统计、缓存命中率监控

## 快速开始

### 环境要求
- Go 1.21+
- SQLite 3.x

### 安装

```bash
# 克隆仓库
git clone https://github.com/cloudwego/eino.git
cd eino/vdocswx

# 下载依赖
go mod download

# 构建
make build
```

### 配置

复制配置文件模板并编辑：

```bash
cp config.yaml.example config.yaml
```

配置 LLM API Key：

```bash
export DEEPSEEK_API_KEY=your_api_key
# 或者在 config.yaml 中配置
```

### 运行

```bash
# 启动HTTP服务
./bin/wechat-polish serve --config=config.yaml

# 或者直接润色文件
./bin/wechat-polish polish --input=article.md --output=polished.md
```

## API 使用

### 健康检查

```bash
curl http://localhost:8080/health
```

### 润色文章

```bash
curl -X POST http://localhost:8080/api/v1/polish \
  -H "Content-Type: application/json" \
  -d '{
    "content": "这是一篇需要润色的文章内容...",
    "type": "tech"
  }'
```

### 多轮对话

```bash
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session_123",
    "message": "请帮我优化第二段的表达"
  }'
```

### 上传文件

```bash
curl -X POST http://localhost:8080/api/v1/upload \
  -F "file=@article.md" \
  -F "auto_polish=true"
```

## 项目结构

```
vdocswx/
├── agent/              # Agent模块
│   ├── master/         # 主Agent
│   ├── grammar/        # 语法检查Agent
│   ├── style/          # 风格优化Agent
│   ├── logic/          # 逻辑优化Agent
│   └── structure/      # 结构调整Agent
├── api/                # HTTP API服务
├── cache/              # 缓存模块
├── cmd/                # 入口程序
├── config/             # 配置模块
├── deploy/             # K8S部署配置
├── design/             # 设计文档
├── index/              # 索引模块
│   └── inverted/       # 倒排索引
├── llm/                # LLM管理模块
│   ├── provider/       # LLM提供商
│   └── router/         # 模型路由
├── skills/             # 技能模块
│   ├── wechat/         # 微信公众号技能
│   └── tech/           # 技术文章技能
├── stats/              # 统计模块
├── storage/            # 存储模块
└── tools/              # 工具模块
    ├── file/           # 文件操作
    ├── grammar/        # 语法检查工具
    └── markdown/       # Markdown处理
```

## 部署

### Docker

```bash
# 构建镜像
make docker

# 运行容器
docker run -d \
  -p 8080:8080 \
  -e DEEPSEEK_API_KEY=your_key \
  -v $(pwd)/data:/app/data \
  wechat-polish:latest
```

### Kubernetes

```bash
# 部署到K8S
make deploy

# 查看状态
kubectl get pods -n wechat-polish
```

## 开发

### 运行测试

```bash
# 单元测试
make test

# 集成测试
go test -tags=integration -v ./...

# 测试覆盖率
make coverage
```

### 代码检查

```bash
make lint
```

## 配置说明

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `server.port` | HTTP服务端口 | 8080 |
| `llm.provider` | LLM提供商 | deepseek |
| `llm.max_tokens` | 最大Token数 | 4096 |
| `agent.max_iterations` | 最大迭代次数 | 20 |
| `cache.enable` | 启用缓存 | true |
| `cache.ttl` | 缓存过期时间 | 30m |

更多配置请参考 `config.yaml`。

## 技能扩展

技能文件位于 `skills/` 目录，使用 Markdown 格式定义：

```markdown
---
name: 自定义技能
category: custom
description: 技能描述
keywords: 关键词1, 关键词2
---

# 技能内容

...
```

## License

Apache License 2.0
