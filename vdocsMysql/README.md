# MySQL内核专家Agent

基于Eino框架开发的MySQL内核分析智能Agent，从Percona Server源码角度解答MySQL内核相关问题。

## 功能特点

### 核心能力

- **代码搜索**: 使用ripgrep在源码中快速搜索代码
- **符号查找**: 使用ctags定位函数/变量/类型定义
- **调用链分析**: 追踪函数调用关系
- **架构理解**: 理解MySQL内核模块结构
- **负载模拟**: 模拟负载并分析性能瓶颈
- **技能扩展**: 支持MySQL专用知识技能

### 输出格式

- **总结模式**: 简短回答，包含关键结论和代码位置
- **文档模式**: 详细分析，包含Mermaid图表、代码片段、调用链

## 目录结构

```
vdocsMysql/
├── agent/              # Agent实现
│   ├── master.go       # 主控Agent
│   └── skill_backend.go # MySQL技能后端
├── config/             # 配置管理
│   └── config.go
├── design/             # 设计文档
│   ├── architecture.md # 整体架构设计
│   ├── modules.md      # 模块设计
│   ├── tech-stack.md   # 技术选型
│   ├── tools.md        # 思维工具使用方案
│   ├── index-system.md # 索引系统设计
│   └── simulation.md   # 模拟功能设计
├── index/              # 索引系统
├── output/             # 输出格式化
├── simulation/         # 负载模拟
│   ├── model.go        # 模拟模型
│   └── simulator.go    # 模拟引擎
├── skills/             # MySQL技能
├── storage/            # 存储层
├── tools/              # 工具实现
│   ├── grep.go         # Grep工具
│   └── symbol.go       # 符号查找工具
├── cmd/                # 主程序
│   └── main.go
├── go.mod
└── README.md
```

## 快速开始

### 环境要求

- Go 1.21+
- ripgrep (`brew install ripgrep`)
- Universal Ctags (`brew install universal-ctags`)
- Percona Server源码

### 安装

```bash
cd vdocsMysql
go build -o mysql-expert ./cmd/main.go
```

### 配置

创建配置文件 `config.yaml`:

```yaml
source:
  path: "/path/to/percona-server"
  include_patterns:
    - "*.cc"
    - "*.h"
    - "*.cpp"
  exclude_patterns:
    - "*/unittest/*"
    - "*/test/*"

llm:
  provider: "openai"
  model: "gpt-4-turbo"
  api_key_env: "OPENAI_API_KEY"
  max_tokens: 8192

agent:
  max_iterations: 20
  max_concurrent_agents: 5
  timeout: 300s

output:
  mode: "document"
  include_diagrams: true
  include_code_refs: true
```

### 使用

```bash
# 交互模式
./mysql-expert -i

# 单次查询
./mysql-expert -q "mysql_execute_command的调用链是什么?"

# 使用配置文件
./mysql-expert -config config.yaml -i
```

## 支持的问题类型

| 类型 | 示例问题 | 处理策略 |
|------|----------|----------|
| 代码搜索 | "找到处理SELECT语句的函数" | GrepTool + IndexSearch |
| 原理解释 | "InnoDB如何实现MVCC" | CallChainTracer + ArchitectureMapper |
| 调用链 | "mysql_execute_command的调用链" | CallChainTracer |
| 性能分析 | "查询执行的瓶颈在哪里" | PerformanceProfiler + SimulationTool |
| 架构理解 | "MySQL的线程模型是什么" | ArchitectureMapper |
| 负载模拟 | "模拟高并发SELECT场景" | SimulationTool |

## 内置技能

- `innodb_transaction` - InnoDB事务处理
- `innodb_lock` - InnoDB锁机制
- `innodb_buffer_pool` - Buffer Pool管理
- `innodb_redo_log` - Redo Log
- `sql_parser` - SQL解析器
- `sql_optimizer` - 查询优化器

## 模拟功能

### 负载规格定义

```json
{
  "workload_type": "oltp",
  "qps": 1000,
  "concurrency": 50,
  "sql_distribution": {
    "SELECT": 0.7,
    "INSERT": 0.2,
    "UPDATE": 0.1
  },
  "buffer_pool_hit_ratio": 0.95
}
```

### 输出

- 函数调用图可视化
- 瓶颈分析报告
- 优化建议

## 设计文档

详细设计文档位于 `design/` 目录:

- [整体架构设计](design/architecture.md)
- [模块设计](design/modules.md)
- [技术选型](design/tech-stack.md)
- [思维工具使用方案](design/tools.md)
- [索引系统设计](design/index-system.md)
- [模拟功能设计](design/simulation.md)

## License

Apache License 2.0
