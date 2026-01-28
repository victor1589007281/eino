---
name: kernel_analysis
description: Linux内核代码分析技能，提供标准化的分析流程
---

# Linux内核代码分析技能

当你需要分析Linux内核代码时，请按照以下流程执行。

## 1. 函数分析流程

### 步骤1: 定位函数
使用 `grep_code` 工具搜索函数定义：
```json
{
  "pattern": "^(static\\s+)?[\\w\\s\\*]+\\s+{函数名}\\s*\\(",
  "file_types": ["c", "h"],
  "max_results": 10
}
```

### 步骤2: 读取源码
使用 `read_source` 工具读取函数实现：
```json
{
  "file_path": "{定位到的文件}",
  "start_line": {函数起始行},
  "end_line": {函数结束行},
  "context_lines": 10
}
```

### 步骤3: 分析调用关系
使用 `call_graph` 工具分析：
```json
{
  "function": "{函数名}",
  "direction": "both",
  "max_depth": 5
}
```

### 步骤4: 输出报告
报告应包含：
- 函数签名和位置
- 参数说明
- 核心逻辑分析
- 调用关系图
- 相关代码引用

## 2. 系统调用分析流程

### 步骤1: 找到系统调用定义
搜索 `SYSCALL_DEFINE` 宏：
```json
{
  "pattern": "SYSCALL_DEFINE[0-6]\\s*\\(\\s*{系统调用名}",
  "file_types": ["c"],
  "directories": ["kernel", "fs", "mm", "net"]
}
```

### 步骤2: 追踪实现
从系统调用入口追踪到核心实现函数。

### 步骤3: 分析关键路径
识别系统调用的关键执行路径和数据结构。

## 3. 子系统架构分析流程

### 步骤1: 识别核心文件
使用 `index_search` 找到相关文件：
```json
{
  "keywords": ["{子系统关键词}"],
  "operator": "AND",
  "limit": 30
}
```

### 步骤2: 分析接口
搜索公开的API和数据结构：
```json
{
  "pattern": "^(struct|extern|EXPORT_SYMBOL)",
  "directories": ["{子系统目录}"]
}
```

### 步骤3: 构建架构图
根据分析结果构建：
- 组件关系图
- 数据流图
- 调用层次图

## 4. 输出格式要求

### 代码引用格式
```
文件: kernel/fork.c:1234
```c
// 代码内容
```
```

### 调用链格式
```
函数名 - 文件路径:行号
├── 子函数1 - 路径:行号
│   └── 孙函数 - 路径:行号
└── 子函数2 - 路径:行号
```

### Mermaid图表要求
1. 使用浅色背景：#ffe1e1, #e1ffe1, #e1f5ff, #fff3e1, #f5e1ff
2. 黑色加粗文字
3. 节点文本中避免使用 (), [], {} 等特殊字符
4. 函数名不加括号，如 `do_fork` 而不是 `do_fork()`
