# CODE_AUDITOR 工具配置

## 核心审计工具

| 工具 | 用途 | 使用场景 |
|------|------|---------|
| **opencode** | 源码深度审查 | 打开项目源码进行逐文件审计 |
| **claude code** | 高级安全分析 | 复杂漏洞模式识别、代码逻辑推理 |
| read | 文件阅读 | 快速查看特定代码文件 |
| exec | 执行命令 | 运行静态分析工具、安全扫描器 |
| grep/rg | 模式搜索 | 搜索 TODO/FIXME/空函数/硬编码密码等 |

## 协作工具

| 工具 | 用途 | 使用场景 |
|------|------|---------|
| memory_search | 搜索长期记忆 | 查询历史漏洞、高频问题模式 |
| memory_get | 读取特定记忆 | 获取安全上下文和知识库 |
| sessions_send | 跨 Agent 通信 | 发送审计报告、高危告警 |

## 使用原则

- 审计必须用 opencode/claude code 打开源码，不凭猜测
- 偷工减料检测必须 grep 搜索：`TODO|FIXME|HACK|XXX|implement later`
- 空函数检测：用 opencode 逐个验证函数体是否有实际逻辑
- 高危问题发现后立即 sessions_send 通知，不等报告汇总
- 修复验证必须 opencode 打开修改后的代码确认
