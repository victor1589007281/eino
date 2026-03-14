# 测试指南

## 一、测试总览

| 层级 | 类型 | 数量 | 工具 | 需要 PG | 需要 Go 服务 |
|------|------|------|------|---------|------------|
| Go 单元测试 | Unit + Integration | 25 | go test | ✅ | ❌ |
| TS 单元测试 | Unit (Mock) | 26 | Jest | ❌ | ❌ |
| TS 集成测试 | Integration | 11 | Jest | ✅ | ✅ |
| E2E 测试 | End-to-End | 5 | curl + Make | ✅ | ✅ |

## 二、前置环境

### 2.1 数据库

所有测试都需要一个可访问的 PostgreSQL 实例。

**方案 A：Docker 启动专用 PG**

```bash
make docker-up
# 会启动 PG on :5432 + Go 服务 on :8090
```

**方案 B：复用已有 PG**

如果 5432 已被占用：

```bash
# 在已有 PG 中创建测试数据库
psql -U your_admin -c "CREATE DATABASE collab;"
psql -U your_admin -c "CREATE USER collab WITH PASSWORD 'collab' SUPERUSER;"

# 确保连接可用
psql "host=localhost user=collab password=collab dbname=collab" -c "SELECT 1;"
```

### 2.2 Node.js 依赖

```bash
cd ts-plugins/collab && npm install
```

## 三、运行测试

### 3.1 一键全量测试

```bash
make test
```

输出示例：

```
>>> Running Go unit tests...
PASS  registry (25 tests)
PASS  task (8 tests)
PASS  project (5 tests)
PASS  feishu (4 tests)
PASS  dashboard (2 tests)
✅ All Go tests passed.

>>> Running TS unit tests...
PASS  test/hooks.test.ts (8 tests)
PASS  test/tools.test.ts (7 tests)
PASS  test/commands.test.ts (8 tests)
PASS  test/go-bridge.test.ts (11 tests)
✅ All TS tests passed.
```

### 3.2 Go 单元测试

```bash
make test-go

# 只跑某个模块
cd go-collab-service
go test -v -count=1 ./internal/task/         # 任务模块
go test -v -count=1 ./internal/registry/     # Agent 注册
go test -v -count=1 ./internal/project/      # 项目管理
go test -v -count=1 ./internal/feishu/       # 飞书通道
go test -v -count=1 ./internal/dashboard/    # Dashboard

# 只跑某个测试
go test -v -count=1 -run TestDAGDependency ./internal/task/

# 带覆盖率
go test -coverprofile=cover.out ./...
go tool cover -html=cover.out -o cover.html
```

### 3.3 TS 单元测试

```bash
make test-ts

# 只跑某个测试文件
cd ts-plugins/collab
npx jest test/hooks.test.ts     # Hook 测试
npx jest test/tools.test.ts     # Tool 测试
npx jest test/commands.test.ts  # Command 测试

# 带覆盖率报告
npx jest --coverage

# Watch 模式（开发时使用）
npx jest --watch
```

### 3.4 TS 集成测试（需要 Go 服务）

```bash
# 1. 确保 Go 服务运行中
make run-go-bg

# 2. 运行集成测试
cd ts-plugins/collab
npx jest test/go-bridge.test.ts

# 3. 停止 Go 服务
make stop-go
```

### 3.5 E2E 端到端测试

```bash
make e2e
```

这个命令会自动：
1. 编译 Go 服务
2. 后台启动 Go 服务
3. 等待就绪
4. 执行一系列 curl API 调用
5. 验证响应
6. 停止 Go 服务

## 四、测试用例详情

### 4.1 Go — Registry 模块 (4 tests)

| 测试 | 验证内容 |
|------|---------|
| `TestRegisterAndGet` | Agent 注册 + 获取 + 字段正确性 |
| `TestListWithFilters` | 按 project_id / status 过滤列表 |
| `TestUpdateStatus` | 状态更新 + last_active_at 刷新 |
| `TestParseAgentIDFromSessionKey` | 解析 `agent:xxx:subagent:yyy` 格式 |

### 4.2 Go — Task 模块 (8 tests)

| 测试 | 验证内容 |
|------|---------|
| `TestCreateTask` | 创建任务 + 自动生成 ID/Path + 子任务层级 |
| `TestTaskStatusTransition` | 完整状态机: created → assigned → in_progress → review → completed |
| `TestInvalidTransitionRejected` | 非法状态跳转被拒绝 (created → completed) |
| `TestDAGDependency` | 依赖任务未完成时阻止启动 + 完成后解锁 |
| `TestQueryTasks` | 按 project/assignee/status 多维查询 |
| `TestFindStuckTasks` | 超时未更新的任务检测 |
| `TestArtifacts` | 交付物保存 + 关联到任务 |
| `TestGetStats` | 按项目统计各状态任务数 |

### 4.3 Go — Project 模块 (5 tests)

| 测试 | 验证内容 |
|------|---------|
| `TestCreateAndGetProject` | 创建 + 按 ID/GroupID 查询 |
| `TestProjectMemory` | 添加记忆 + 按 category/keyword 查询 |
| `TestIterations` | 创建迭代 + 列表查询 |
| `TestGetContext` | 协作上下文加载（项目+迭代+记忆+任务+Agent） |
| `TestExperiences` | 经验添加 + 关键词搜索 |

### 4.4 Go — Feishu 模块 (4 tests)

| 测试 | 验证内容 |
|------|---------|
| `TestSelectBot` | 默认 Bot + 群专属 Bot 映射 |
| `TestBuildHeader` | 普通 Agent 头 + Subagent 头（使用父 Agent 身份） |
| `TestSend` | 消息发送（日志输出验证） |
| `TestBotMappingCRUD` | Bot 映射的增删改查 |

### 4.5 Go — Dashboard 模块 (2 tests)

| 测试 | 验证内容 |
|------|---------|
| `TestDashboardOverview` | 全局概览（项目列表+Agent 列表+告警） |
| `TestProjectDetail` | 单项目详情+任务列表+统计 |

### 4.6 TS — Hooks (8 tests)

| 测试 | 验证内容 |
|------|---------|
| `before_agent_start: 群聊注入` | 检测 oc_ session → 加载上下文 → 返回 prependContext |
| `before_agent_start: 私聊跳过` | DM session → 不注入上下文 |
| `message_sending: 角色前缀` | 消息自动添加 `【emoji name】` |
| `message_sending: 已格式化跳过` | 已有前缀的不重复添加 |
| `message_sending: 子 Agent` | Subagent 标注 `【parent → Sub:name】` |
| `before_tool_call: 放行非监控` | 非 spawn/send 工具直接通过 |
| `before_tool_call: 记录 spawn` | sessions_spawn 调用被记录到活动日志 |
| `after_tool_call: 错误记经验` | 工具失败时自动保存为 pitfall 经验 |
| `agent_end: 正常重置` | 正常结束 → status=idle, load=0 |
| `agent_end: 异常上报` | 错误结束 → status=error + 经验记录 |
| `before_compaction: 同步` | 压缩前同步任务状态 + 活动记录 |

### 4.7 TS — Tools (7 tests)

| 测试 | 验证内容 |
|------|---------|
| `create_task: 正常创建` | 必填字段 + assigned_by 来自 context.agentId |
| `create_task: 错误处理` | DB 异常时返回 `{success: false, error}` |
| `update_task: 状态更新` | 传递 status + result 字段 |
| `query_tasks: 多维过滤` | 按 project/status 查询 |
| `save_artifact: 保存` | 交付物保存 + created_by |
| `create_project: 创建` | 群 ID 绑定 + team_agents |
| `save_decision: 记忆` | 带 category + pinned |
| `query_project_memory: 搜索` | 关键词 + 分类过滤 |

### 4.8 TS — Commands (8 tests)

| 测试 | 验证内容 |
|------|---------|
| `/status 无参数` | 全局 Dashboard 概览输出 |
| `/status P1` | 单项目详情输出 |
| `/pause T1` | 暂停单个任务 |
| `/pause P1` | 暂停项目所有进行中任务 |
| `/pause 无参数` | 显示用法说明 |
| `/tasks 带过滤` | 按 project + status 过滤 |
| `/tasks 无过滤` | 显示所有任务 |

### 4.9 TS — GoBridge 集成 (11 tests)

| 测试 | 验证内容 |
|------|---------|
| `health` | 连接检测 + 超时处理 |
| `listAgents` | Agent 列表 API |
| `registerAgent` | 注册 + 回查 |
| `updateAgentStatus` | 状态变更 |
| `createTask` | 任务创建 |
| `queryTasks` | 任务查询 |
| `updateTask` | 任务更新 |
| `createProject` | 项目创建 |
| `saveDecision` | 记忆保存 |
| `queryExperiences` | 经验查询 |
| `getDashboard` | Dashboard |

> 注：GoBridge 集成测试在没有 Go 服务运行时会自动跳过（不会失败）。

## 五、CI 集成建议

### GitHub Actions 示例

```yaml
name: Test
on: [push, pull_request]

jobs:
  go-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: collab
          POSTGRES_PASSWORD: collab
          POSTGRES_DB: collab
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: cd code/go-collab-service && go test -v -cover ./...

  ts-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
      - run: cd code/ts-plugins/collab && npm ci && npx jest --no-cache --coverage
```

## 六、添加新测试

### Go 测试规范

```go
func TestNewFeature(t *testing.T) {
    db := setupTestDB(t) // 复用标准 setup
    svc := NewService(db)

    // 测试数据前缀 "test-" 便于清理
    defer db.Exec("DELETE FROM xxx WHERE id LIKE 'test-%'")

    // Arrange → Act → Assert
    err := svc.DoSomething(...)
    require.NoError(t, err)
    assert.Equal(t, expected, actual)
}
```

### TS 测试规范

```typescript
// 使用 MockBridge 做纯单元测试（不依赖 Go 服务）
class MockBridge {
  calls: { method: string; args: any[] }[] = [];
  async someMethod(...args: any[]) {
    this.calls.push({ method: "someMethod", args });
    return { /* mock response */ };
  }
}

describe("MyFeature", () => {
  it("should do something", async () => {
    const mock = new MockBridge();
    const handler = createMyHandler(mock as unknown as GoBridge);
    const result = await handler(input);
    expect(result).toEqual(expected);
    expect(mock.calls).toHaveLength(1);
  });
});
```
