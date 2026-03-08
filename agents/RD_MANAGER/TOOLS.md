# RD_MANAGER 工具配置

## 核心工具

| 工具 | 用途 | 使用场景 |
|------|------|---------|
| memory_search | 搜索长期记忆 | 处理新需求前先查历史经验 |
| memory_get | 读取特定记忆 | 获取具体项目/决策记录 |
| sessions_spawn | 创建子 Agent 会话 | 并行处理多个子任务 |
| sessions_send | 跨 Agent 通信 | 分配任务、询问进度、接收汇报 |

## 技术工具（资深研发能力）

| 工具 | 用途 | 使用场景 |
|------|------|---------|
| **opencode** | 代码阅读与编写 | 技术评审、定位问题、紧急修复时直接上手 |
| **claude code** | 深度代码分析 | 复杂问题定位、架构级技术判断 |
| read / write / edit | 文件操作 | 编写项目文档、更新进度 |
| exec | 执行命令 | 验证构建、运行检查 |
| grep/rg | 代码搜索 | 质量巡检时搜索 TODO/FIXME/空函数 |

## 协调工具

| 工具 | 用途 |
|------|------|
| @ARCHITECT | 分配架构设计/预研任务 |
| @DEV_MANAGER | 分配开发任务 |
| @TEST_MANAGER | 分配测试任务 |
| @CODE_AUDITOR | 分配审计任务 |

## 使用原则

- 分配任务前先 memory_search 查是否有类似历史
- 并行无依赖的子任务用 sessions_spawn 同时启动
- 质量巡检时用 opencode + grep 直接检查代码
- 技术评审时用 claude code 分析核心逻辑
