# 编译部署指南

## 一、前置条件

| 组件 | 最低版本 | 说明 |
|------|---------|------|
| Go | 1.23+ | Go 服务编译运行 |
| Node.js | 20+ | TS 插件编译 |
| npm | 9+ | 包管理 |
| Docker | 24+ | 容器运行 PG / 生产部署 |
| PostgreSQL | 15+ | 数据持久化（可 Docker 运行） |
| OpenClaw | v2026.3.13 | 插件宿主 |

## 二、项目结构

```
code/
├── docker-compose.yml          # Docker 编排
├── Makefile                    # 一键命令
├── go-collab-service/          # Go 后端
│   ├── cmd/server/main.go
│   ├── internal/               # 6 个模块
│   ├── migrations/             # SQL 迁移脚本
│   ├── Dockerfile              # Go 多阶段构建
│   └── go.mod
├── ts-plugins/collab/          # OpenClaw TS 插件
│   ├── src/                    # 源码
│   ├── test/                   # 测试
│   ├── Dockerfile              # 插件构建镜像
│   ├── openclaw.plugin.json    # 插件声明
│   └── package.json
└── docs/                       # 文档
```

## 三、编译

### 3.1 一键编译

```bash
cd code
make build
```

### 3.2 分步编译

**Go 服务:**

```bash
cd go-collab-service
go mod tidy
go build -o bin/collab-service ./cmd/server
# 产出: bin/collab-service (约 15MB)
```

**TS 插件:**

```bash
cd ts-plugins/collab
npm install
npm run build
# 产出: dist/ 目录（编译后的 JS + 类型声明）
```

### 3.3 Docker 镜像构建

```bash
# 构建所有镜像
make docker-build-images

# 或单独构建
docker build -t collab-go-service:latest ./go-collab-service
docker build -t collab-ts-plugin:latest ./ts-plugins/collab
```

## 四、部署方案

### 方案 A：Docker Compose 一键部署（推荐）

适合迷你主机、单机部署场景。

```bash
# 1. 配置环境变量（可选）
export FEISHU_APP_ID="your-app-id"
export FEISHU_APP_SECRET="your-secret"
export DEFAULT_BOT_ID="your-bot-id"

# 2. 启动所有服务
make docker-up
# 等价于: docker compose up -d --build

# 3. 验证
curl http://localhost:8090/health
# 期望: {"status":"ok"}

# 4. 安装插件到 OpenClaw
make install-plugin
```

**端口说明：**

| 服务 | 端口 | 协议 |
|------|------|------|
| Go 服务 | 8090 | HTTP |
| PostgreSQL | 5432 | TCP |

**端口冲突处理：**

如果 5432 已被占用（例如已有 PG 实例），修改 `.env` 文件：

```bash
# .env
PG_PORT=5433
```

或直接使用已有 PG 实例：

```bash
# 在已有 PG 中创建数据库和用户
psql -U your_admin -c "CREATE DATABASE collab;"
psql -U your_admin -c "CREATE USER collab WITH PASSWORD 'collab' SUPERUSER;"
psql -U your_admin -c "GRANT ALL PRIVILEGES ON DATABASE collab TO collab;"

# 运行迁移
psql "host=localhost user=collab password=collab dbname=collab" < go-collab-service/migrations/000001_init.up.sql

# 只启动 Go 服务（不启动 Docker PG）
DATABASE_DSN="host=localhost user=collab password=collab dbname=collab port=5432 sslmode=disable" \
  make run-go
```

### 方案 B：本地开发部署

```bash
# 1. 确保 PG 可用（Docker 或已有实例）
make docker-up  # 仅启动 PG

# 2. 本地运行 Go 服务
make run-go

# 3. 构建并安装 TS 插件
make install-plugin

# 4. 重启 OpenClaw
openclaw restart
```

### 方案 C：纯 Docker 生产部署

```bash
# 1. 构建并推送镜像
docker build -t your-registry/collab-go-service:v1.0 ./go-collab-service
docker push your-registry/collab-go-service:v1.0

# 2. 在目标机器上拉取并运行
docker compose up -d
```

## 五、配置

### 5.1 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | 8090 | Go 服务监听端口 |
| `DATABASE_DSN` | `host=localhost user=collab password=collab dbname=collab port=5432 sslmode=disable` | PostgreSQL 连接串 |
| `OPENCLAW_DIR` | `~/.openclaw` | OpenClaw 工作目录，用于 Agent 扫描 |
| `DEFAULT_BOT_ID` | 空 | 默认飞书机器人 App ID |
| `FEISHU_APP_ID` | 空 | 飞书应用 ID |
| `FEISHU_APP_SECRET` | 空 | 飞书应用 Secret |
| `GIN_MODE` | debug | Gin 模式，生产环境设为 `release` |

### 5.2 TS 插件配置

在 OpenClaw 的 `openclaw.json` 中添加插件配置：

```json5
{
  "plugins": {
    "@team/collab": {
      "goServiceUrl": "http://localhost:8090",
      "feishuEnabled": true,
      "contextMaxTokens": 3000
    }
  }
}
```

## 六、数据库管理

### 初始化

首次启动时 Go 服务会通过 GORM `AutoMigrate` 自动创建表。也可手动执行 SQL：

```bash
make db-migrate
```

### 备份

```bash
# 备份
docker compose exec postgres pg_dump -U collab collab > backup_$(date +%Y%m%d).sql

# 恢复
docker compose exec -T postgres psql -U collab collab < backup_20260308.sql
```

### 重置

```bash
make db-reset
# ⚠️ 这会删除所有数据
```

## 七、升级

1. 拉取最新代码
2. `make build` 重新编译
3. `make docker-restart` 重启 Docker 服务
4. `make install-plugin` 更新插件
5. 重启 OpenClaw

## 八、故障排查

| 问题 | 检查方法 | 解决方案 |
|------|---------|---------|
| Go 服务无法连接 PG | `curl localhost:8090/health` 返回错误 | 检查 `DATABASE_DSN`，确认 PG 运行 |
| 端口 5432 被占用 | `lsof -i:5432` | 设置 `PG_PORT=5433` 或复用已有 PG |
| 端口 8090 被占用 | `lsof -i:8090` | 设置 `PORT=8091` |
| 插件未加载 | OpenClaw 日志无 `@team/collab` | 检查插件安装路径，确认 `openclaw.plugin.json` |
| Agent 扫描失败 | Go 日志 `Agent scan warning` | 检查 `OPENCLAW_DIR` 指向正确的 OpenClaw 目录 |
| Docker Compose 版本不对 | `docker compose` 报错 | 换用 `docker-compose`（旧版）或安装 compose 插件 |
