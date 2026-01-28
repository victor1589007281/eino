---
name: 技术文章代码示例规范
category: tech
description: 技术类文章中代码示例的写作规范
keywords: 代码,示例,技术,编程
---

# 技术文章代码示例规范

## 代码块格式

### Markdown代码块
```
使用三个反引号包裹代码
指定语言以获得语法高亮
```

### 语言标识
常用语言标识：
- `python` - Python代码
- `go` - Go代码
- `javascript` 或 `js` - JavaScript
- `typescript` 或 `ts` - TypeScript
- `bash` 或 `shell` - Shell命令
- `sql` - SQL语句
- `json` - JSON数据
- `yaml` - YAML配置

## 代码注释

### 注释原则
1. 关键步骤必须注释
2. 复杂逻辑详细说明
3. 避免无意义注释

### 注释示例
```python
# 1. 加载数据
data = load_data("input.csv")

# 2. 数据预处理
# 移除空值，确保数据完整性
cleaned_data = data.dropna()

# 3. 特征工程
# 这里使用标准化处理，使特征在同一尺度
normalized = normalize(cleaned_data)
```

## 代码完整性

### 可运行原则
- 代码应该可以直接复制运行
- 包含必要的import语句
- 提供示例数据或mock数据

### 依赖说明
在代码前说明依赖：
```
# 需要安装以下依赖
# pip install pandas numpy scikit-learn
```

## 代码长度

### 简洁原则
- 单个代码块不超过50行
- 过长的代码分段展示
- 省略不重要的部分用注释标注

### 分段技巧
```python
# 第一步：数据加载
# ... 代码 ...

# 第二步：数据处理（关键代码）
# 这是我们重点关注的部分
def process_data(data):
    # 核心处理逻辑
    pass

# 第三步：结果输出
# ... 代码 ...
```

## 输出展示

### 运行结果
代码后展示运行结果：
```python
result = calculate(10, 20)
print(result)
# 输出: 200
```

### 错误处理
展示可能的错误和处理方式：
```python
try:
    result = dangerous_operation()
except ValueError as e:
    print(f"参数错误: {e}")
except Exception as e:
    print(f"未知错误: {e}")
```

## 版本兼容

### 版本说明
- 明确说明使用的语言/库版本
- 提示版本兼容性问题

### 示例
```
本文代码基于:
- Python 3.9+
- pandas 1.3.0
- 注意：Python 2.x 不兼容
```

## 示例

### 输入
下面是代码:
def hello():
print("hello")
hello()

### 输出
下面是代码示例：

```python
def hello():
    """打印问候语"""
    print("Hello, World!")

# 调用函数
hello()
# 输出: Hello, World!
```
