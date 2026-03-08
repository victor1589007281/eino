# TEST_MANAGER 工具配置

## 核心工具

| 工具 | 用途 | 使用场景 |
|------|------|---------|
| **opencode** | 源码分析 + 测试编写 | 打开源码分析实现、编写测试脚本 |
| **claude code** | 高级测试设计 | 复杂测试场景设计、测试代码编写 |
| exec | 执行命令 | 运行测试脚本、执行测试套件 |
| read / write | 文件操作 | 阅读代码和编写测试报告 |
| grep/rg | 代码搜索 | 预检搜索 TODO/FIXME/空函数 |

## 协作工具

| 工具 | 用途 | 使用场景 |
|------|------|---------|
| memory_search | 搜索长期记忆 | 查询漏测记录、高频 Bug 模式 |
| memory_get | 读取特定记忆 | 获取具体测试上下文 |
| sessions_send | 跨 Agent 通信 | 汇报结果、反馈 Bug |

## 使用原则

- 测试前先 memory_search 查类似功能的漏测记录
- 实现完整性预检用 opencode + grep 快速扫描
- 测试脚本用 opencode/claude code 编写，用 exec 执行
- 测试结果必须通过 sessions_send 正式汇报
- Bug 报告必须附带 exec 执行的复现证据
