# 完整部署与测试手册

> 从源码编译 → K8s 部署 → OpenClaw 插件安装 → 创建测试 Agent → 端到端测试

---

## 〇、全局变量约定

```bash
# 项目根目录
export CODE_DIR="/Users/huaquan.liang/Documents/GitHub/eino/code"

# OpenClaw 工作目录
export OPENCLAW_DIR="$HOME/.openclaw"

# 镜像 Registry（可选，留空则使用本地镜像）
export REGISTRY=""

# 镜像 Tag
export TAG="latest"

# Go 服务地址（K8s NodePort 部署后的地址）
export GO_SERVICE_URL="http://localhost:30090"
```

---

## 一、编译构建

### 1.1 编译 Go 服务

```bash
cd $CODE_DIR/go-collab-service

# 安装依赖
go mod tidy

# 编译（输出 bin/collab-service）
go build -o bin/collab-service ./cmd/server

# 验证
./bin/collab-service --help 2>&1 || echo "Binary OK (no --help flag)"
ls -lh bin/collab-service
```

### 1.2 编译 TS 插件

```bash
cd $CODE_DIR/ts-plugins/collab

# 安装依赖
npm install

# 编译（输出 dist/）
npm run build

# 验证
ls dist/index.js && echo "TS plugin build OK"
```

### 1.3 运行单元测试

```bash
cd $CODE_DIR

# Go 测试（需要 PostgreSQL 可用）
cd go-collab-service && go test -v -count=1 ./... && cd ..

# TS 测试（纯 Mock，无外部依赖）
cd ts-plugins/collab && npx jest --no-cache && cd ../..
```

### 1.4 构建 Docker 镜像

```bash
cd $CODE_DIR

# Go 服务镜像
docker build -t collab-go-service:${TAG} ./go-collab-service

# TS 插件镜像（用于提取编译产物）
docker build -t collab-ts-plugin:${TAG} ./ts-plugins/collab

# 验证
docker images | grep collab
```

**如果使用私有仓库：**

```bash
REGISTRY="registry.cn-hangzhou.aliyuncs.com/your-namespace"

docker tag collab-go-service:${TAG} ${REGISTRY}/collab-go-service:${TAG}
docker push ${REGISTRY}/collab-go-service:${TAG}
```

**或使用一键脚本：**

```bash
./deploy/scripts/build-images.sh $REGISTRY
```

---

## 二、K8s 部署

### 2.1 检查 K8s 集群

```bash
kubectl cluster-info
kubectl get nodes
```

### 2.2 如果使用本地镜像（Minikube / Colima）

```bash
# Minikube 场景 — 让 minikube 使用本地 Docker 镜像
eval $(minikube docker-env)
docker build -t collab-go-service:latest ./go-collab-service

# Colima 场景 — 镜像已在本地 Docker 中，K8s 可直接访问
```

### 2.3 部署到 K8s

**方法 A：一键脚本**

```bash
cd $CODE_DIR
./deploy/scripts/k8s-deploy.sh
```

**方法 B：手动逐步部署**

```bash
cd $CODE_DIR/deploy/k8s

# Step 1: 创建 namespace
kubectl apply -f namespace.yaml

# Step 2: 部署 PostgreSQL
kubectl apply -f postgres.yaml
kubectl wait --for=condition=ready pod -l app=postgres -n collab --timeout=120s
echo "PostgreSQL Ready"

# Step 3: 部署 Go 服务
kubectl apply -f go-service.yaml
kubectl wait --for=condition=ready pod -l app=go-service -n collab --timeout=120s
echo "Go Service Ready"
```

**方法 C：Kustomize**

```bash
kubectl apply -k $CODE_DIR/deploy/k8s/
kubectl wait --for=condition=ready pod -l app=go-service -n collab --timeout=120s
```

### 2.4 验证 K8s 部署

```bash
# 查看 Pods
kubectl get pods -n collab
# 期望输出:
#   postgres-xxx     1/1  Running
#   go-service-xxx   1/1  Running

# 查看 Services
kubectl get svc -n collab
# 期望输出:
#   postgres              ClusterIP   ...  5432/TCP
#   go-service            ClusterIP   ...  8090/TCP
#   go-service-nodeport   NodePort    ...  8090:30090/TCP

# 查看日志
kubectl logs -n collab -l app=go-service --tail=20

# Health Check（via NodePort）
curl http://localhost:30090/health
# 或通过 port-forward
kubectl port-forward -n collab svc/go-service 8090:8090 &
curl http://localhost:8090/health
# 期望: {"status":"ok"}
```

### 2.5 运行冒烟测试

```bash
cd $CODE_DIR

# 针对 NodePort
./deploy/scripts/smoke-test.sh http://localhost:30090

# 或针对 port-forward
kubectl port-forward -n collab svc/go-service 8090:8090 &
./deploy/scripts/smoke-test.sh http://localhost:8090
```

期望输出：

```
--- [1/10] Health Check ---
  ✅ health
--- [2/10] Register Agent ---
  ✅ register agent
...
  结果: 10/10 通过
  ✅ 全部通过
```

---

## 三、安装 OpenClaw 插件

### 3.1 提取 TS 编译产物

**方法 A：本地编译产物直接拷贝（推荐）**

```bash
cd $CODE_DIR

# 编译并安装
OPENCLAW_DIR=$HOME/.openclaw make install-plugin

# 等价于手动：
mkdir -p $OPENCLAW_DIR/plugins/@team/collab
cp -r ts-plugins/collab/dist $OPENCLAW_DIR/plugins/@team/collab/
cp ts-plugins/collab/package.json $OPENCLAW_DIR/plugins/@team/collab/
cp ts-plugins/collab/openclaw.plugin.json $OPENCLAW_DIR/plugins/@team/collab/
```

**方法 B：从 Docker 镜像提取**

```bash
# 从构建好的镜像中提取
CONTAINER_ID=$(docker create collab-ts-plugin:latest)
docker cp $CONTAINER_ID:/app/dist $OPENCLAW_DIR/plugins/@team/collab/dist
docker cp $CONTAINER_ID:/app/package.json $OPENCLAW_DIR/plugins/@team/collab/
docker cp $CONTAINER_ID:/app/openclaw.plugin.json $OPENCLAW_DIR/plugins/@team/collab/
docker rm $CONTAINER_ID
```

### 3.2 验证插件文件

```bash
ls -la $OPENCLAW_DIR/plugins/@team/collab/
# 期望:
#   dist/
#   package.json
#   openclaw.plugin.json

ls $OPENCLAW_DIR/plugins/@team/collab/dist/index.js && echo "Plugin files OK"
```

### 3.3 配置 openclaw.json

编辑 `$OPENCLAW_DIR/openclaw.json`，添加插件配置和 Go 服务地址：

```bash
# 如果你的 Go 服务在 K8s 上通过 NodePort 暴露
GO_SERVICE_URL="http://localhost:30090"

# 如果 OpenClaw 和 Go 服务在同一台机器
GO_SERVICE_URL="http://localhost:8090"
```

在 `openclaw.json` 中添加：

```json5
{
  // 已有配置...

  "plugins": {
    "@team/collab": {
      "goServiceUrl": "http://localhost:30090",  // 改为你的 Go 服务地址
      "feishuEnabled": true,
      "contextMaxTokens": 3000
    }
  }
}
```

### 3.4 重启 OpenClaw

```bash
# 重启 OpenClaw 加载新插件
openclaw stop && openclaw start

# 或者
openclaw restart

# 检查日志确认插件加载
openclaw logs | grep -i "collab"
# 期望看到:
#   [plugin] Loaded @team/collab v1.0.0
#   [collab] Starting Go bridge health monitor
```

---

## 四、创建测试 Agent

### 4.1 安装测试 Agent

**一键脚本：**

```bash
cd $CODE_DIR
./deploy/scripts/install-test-agent.sh $OPENCLAW_DIR
```

**手动安装：**

```bash
# 拷贝测试 Agent 文件
mkdir -p $OPENCLAW_DIR/agents/test-coordinator/agent
cp $CODE_DIR/deploy/test-agent/agent/* $OPENCLAW_DIR/agents/test-coordinator/agent/

# 验证
ls $OPENCLAW_DIR/agents/test-coordinator/agent/
# 期望: AGENTS.md  SOUL.md  SYSTEM_PROMPT.md
```

### 4.2 在 openclaw.json 中注册测试 Agent

在 `agents.list` 数组中添加：

```json5
{
  "id": "TEST_COORDINATOR",
  "name": "测试协调员",
  "agentDir": "/Users/huaquan.liang/.openclaw/agents/test-coordinator/agent",
  "subagents": { "allowAgents": ["*"] }
}
```

### 4.3 向 Go 服务注册测试 Agent

```bash
curl -X POST ${GO_SERVICE_URL}/api/v1/agents/register \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "TEST_COORDINATOR",
    "display_name": "测试协调员",
    "emoji": "🧪",
    "role": "test-coordinator",
    "skills": ["testing", "task-management", "project-management"],
    "status": "idle"
  }'
```

### 4.4 重启 OpenClaw 加载新 Agent

```bash
openclaw restart
```

---

## 五、测试方案

### 5.1 测试层级

```
┌──────────────────────────────────────────────────────┐
│  Level 5: 飞书端到端                                  │
│  在飞书群中 @测试协调员 执行协作流程                    │
├──────────────────────────────────────────────────────┤
│  Level 4: OpenClaw Agent 端到端                       │
│  通过 OpenClaw CLI 与测试 Agent 对话                   │
├──────────────────────────────────────────────────────┤
│  Level 3: 插件 Hook 验证                              │
│  验证 6 个 Hook 生效（上下文注入、消息格式化等）        │
├──────────────────────────────────────────────────────┤
│  Level 2: Go API 冒烟测试                             │
│  10 个 curl 验证全量 API                               │
├──────────────────────────────────────────────────────┤
│  Level 1: 单元测试                                    │
│  Go 25 tests + TS 37 tests                            │
└──────────────────────────────────────────────────────┘
```

### 5.2 Level 1：单元测试

```bash
cd $CODE_DIR

# Go（需要 PG）
cd go-collab-service && go test -v -cover -count=1 ./...

# TS（纯 Mock）
cd ../ts-plugins/collab && npx jest --no-cache --coverage
```

通过标准：Go 25/25, TS 37/37 全部通过。

### 5.3 Level 2：Go API 冒烟测试

```bash
cd $CODE_DIR
./deploy/scripts/smoke-test.sh ${GO_SERVICE_URL}
```

测试内容（10 个检查点）：

| # | 测试 | API | 验证 |
|---|------|-----|------|
| 1 | Health | `GET /health` | 返回 `{"status":"ok"}` |
| 2 | 注册 Agent | `POST /api/v1/agents/register` | 返回 agent 对象 |
| 3 | 列表 Agent | `GET /api/v1/agents` | 包含刚注册的 agent |
| 4 | 创建项目 | `POST /api/v1/projects` | 返回项目 ID |
| 5 | 创建任务 | `POST /api/v1/tasks` | 返回任务 ID |
| 6 | 任务→assigned | `PATCH /api/v1/tasks/:id` | 状态变更成功 |
| 7 | 任务→in_progress | `PATCH /api/v1/tasks/:id` | 状态变更成功 |
| 8 | 保存决策 | `POST /api/v1/projects/:id/memory` | 返回记忆 ID |
| 9 | 获取上下文 | `GET /api/v1/projects/:id/context` | 包含项目名+任务 |
| 10 | Dashboard | `GET /api/v1/dashboard/overview` | 返回项目列表 |

通过标准：10/10 通过。

### 5.4 Level 3：插件 Hook 验证

重启 OpenClaw 后，对每个 Hook 逐一验证：

**Hook 1: before_agent_start（上下文注入）**

```bash
# 先创建一个项目并绑定群
curl -X POST ${GO_SERVICE_URL}/api/v1/projects \
  -H 'Content-Type: application/json' \
  -d '{"name":"Hook测试项目","group_id":"oc_hook_test","team_agents":["TEST_COORDINATOR"]}'

# 通过 OpenClaw 与 Agent 对话（在 oc_hook_test 群中）
# 观察 Agent 回复是否提到了 "Hook测试项目" 等上下文
```

验证：Agent 回复中能引用项目名称、任务统计等注入的上下文。

**Hook 2: message_sending（消息格式化）**

验证：Agent 发送的消息带有 `【🧪 测试协调员】` 前缀。

**Hook 3: before_tool_call（通信监控）**

```bash
# 让 Agent 调用 sessions_spawn
# 查看 Go 服务中是否有 task_activity 记录
curl ${GO_SERVICE_URL}/api/v1/tasks/{task_id}/activities
# 期望看到 type=subagent_spawn 的记录
```

**Hook 4: after_tool_call（结果监控）**

验证：工具调用失败时，`agent_experiences` 表中有 pitfall 记录。

```bash
curl "${GO_SERVICE_URL}/api/v1/agents/TEST_COORDINATOR/experiences"
```

**Hook 5: agent_end（异常处理）**

验证：Agent 正常结束后状态变为 idle。

```bash
curl "${GO_SERVICE_URL}/api/v1/agents" | python3 -c "
import json,sys
agents=json.load(sys.stdin)['agents']
for a in agents:
  if a['id']=='TEST_COORDINATOR':
    print(f'Status: {a[\"status\"]}')
"
```

**Hook 6: before_compaction（压缩同步）**

验证：当 Agent 会话被压缩时，Go 服务中的 task_activity 包含 `compaction_sync` 记录。

### 5.5 Level 4：OpenClaw Agent 端到端

通过 OpenClaw CLI 与测试 Agent 对话：

```bash
# 进入 Agent 对话
openclaw chat TEST_COORDINATOR
```

**测试对话脚本：**

```
你: 运行协作测试

期望 Agent 执行:
1. create_project → 返回项目 ID
2. create_task x3 → 返回 3 个任务 ID
3. update_task → 第一个任务改为 in_progress
4. save_decision → 保存一条决策
5. query_tasks → 列出所有任务
6. query_project_memory → 查询决策记忆
7. 输出测试汇总
```

```
你: /status

期望: 输出项目概览，包含刚创建的项目和任务统计
```

```
你: /tasks

期望: 列出所有任务（包含刚创建的 3 个）
```

```
你: /pause {第一个任务ID}

期望: 任务被暂停
```

验证结果：

```bash
# 检查 Go 服务中的数据
curl ${GO_SERVICE_URL}/api/v1/projects | python3 -m json.tool
curl ${GO_SERVICE_URL}/api/v1/tasks?assignee=TEST_COORDINATOR | python3 -m json.tool
curl ${GO_SERVICE_URL}/api/v1/dashboard/overview | python3 -m json.tool
```

### 5.6 Level 5：飞书端到端

前提：已配置飞书通道 + binding。

| 步骤 | 操作 | 预期结果 |
|------|------|---------|
| 1 | 在飞书群中 @测试协调员 "你好" | 收到带 `【🧪 测试协调员】` 前缀的回复 |
| 2 | @测试协调员 "运行协作测试" | Agent 调用 7 个协作工具并汇报 |
| 3 | 输入 `/status` | 输出项目状态概览 |
| 4 | 输入 `/tasks` | 列出任务列表 |
| 5 | @测试协调员 "保存一条决策：使用 Go 开发后端" | 调用 save_decision |
| 6 | @测试协调员 "查询项目记忆" | 调用 query_project_memory，返回包含刚保存的决策 |
| 7 | 检查 Dashboard API | `curl ${GO_SERVICE_URL}/api/v1/dashboard/overview` 包含该项目 |

---

## 六、故障排查

### K8s 常见问题

```bash
# Pod 启动失败
kubectl describe pod -n collab -l app=go-service
kubectl logs -n collab -l app=go-service --previous

# Go 服务连不上 PG
kubectl exec -n collab -it deploy/go-service -- wget -qO- http://localhost:8090/health

# 重启 Go 服务
kubectl rollout restart deployment/go-service -n collab
```

### 插件常见问题

```bash
# 插件未加载
ls $OPENCLAW_DIR/plugins/@team/collab/dist/index.js
cat $OPENCLAW_DIR/plugins/@team/collab/openclaw.plugin.json

# Go 服务不可达（从 OpenClaw 容器/进程角度）
curl http://localhost:30090/health
# 如果不通，检查 OpenClaw 中配置的 goServiceUrl 地址
```

### 数据库问题

```bash
# 连接数据库
kubectl exec -n collab -it deploy/postgres -- psql -U collab -d collab

# 查看表
\dt

# 查看 Agent 记录
SELECT * FROM agent_records;

# 查看任务
SELECT id, title, status, assignee FROM tasks ORDER BY created_at DESC LIMIT 10;
```

---

## 七、清理

```bash
# 删除 K8s 资源
kubectl delete namespace collab

# 删除本地镜像
docker rmi collab-go-service:latest collab-ts-plugin:latest

# 卸载插件
rm -rf $OPENCLAW_DIR/plugins/@team/collab

# 删除测试 Agent
rm -rf $OPENCLAW_DIR/agents/test-coordinator

# 重启 OpenClaw
openclaw restart
```

---

## 八、快速命令速查

```bash
# ===== 编译 =====
make build                                      # 编译全部
make docker-build-images                        # 构建 Docker 镜像
./deploy/scripts/build-images.sh $REGISTRY      # 构建+推送镜像

# ===== K8s 部署 =====
./deploy/scripts/k8s-deploy.sh                  # 一键 K8s 部署
kubectl apply -k deploy/k8s/                    # Kustomize 部署
kubectl get pods -n collab                      # 查看 Pod 状态

# ===== 插件安装 =====
make install-plugin                             # 安装 TS 插件到 OpenClaw
./deploy/scripts/install-test-agent.sh          # 安装测试 Agent

# ===== 测试 =====
make test                                       # 单元测试（Go + TS）
./deploy/scripts/smoke-test.sh $GO_SERVICE_URL  # 冒烟测试（10 个 API）
openclaw chat TEST_COORDINATOR                  # Agent 对话测试

# ===== 运维 =====
kubectl logs -n collab -l app=go-service -f     # 实时日志
kubectl port-forward -n collab svc/go-service 8090:8090  # 端口转发
curl http://localhost:8090/api/v1/dashboard/overview      # Dashboard
```
