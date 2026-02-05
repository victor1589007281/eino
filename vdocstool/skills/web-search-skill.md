# Web Search Skill 使用指南

## 概述

网页搜索工具提供多引擎搜索能力，支持自适应路由和自动降级，确保搜索的可靠性。

## 工具列表

### 1. web_search - 网页搜索

执行网页搜索，支持多引擎和自动降级。

**参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| query | string | 是 | 搜索查询词 |
| preferred_engine | string | 否 | 首选引擎(duckduckgo/bing/baidu/serper) |
| max_results | integer | 否 | 最大结果数，默认10 |
| auto_fallback | boolean | 否 | 是否自动降级，默认true |

**使用示例：**
```json
{
  "query": "Python 异步编程教程",
  "preferred_engine": "bing",
  "max_results": 5
}
```

**返回结果：**
```json
{
  "query": "Python 异步编程教程",
  "results": [
    {
      "title": "Python异步编程入门指南",
      "url": "https://example.com/async-python",
      "snippet": "本文介绍Python asyncio的基本用法..."
    }
  ],
  "engine_used": "bing",
  "fallback_triggered": false,
  "latency_ms": 234
}
```

### 2. check_engine_health - 检查引擎健康状态

检查搜索引擎的健康状态和性能指标。

**参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| engine_name | string | 否 | 引擎名称，为空则检查所有 |

### 3. set_routing_strategy - 设置路由策略

设置搜索引擎的路由策略。

**参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| strategy | string | 是 | 路由策略(health_first/round_robin/weighted) |

## 搜索引擎说明

### 可用引擎

| 引擎 | 说明 | 国内可用性 | 适用场景 |
|------|------|------------|----------|
| **duckduckgo** | 隐私友好搜索 | ⚠️ 需代理 | 英文搜索、技术文档 |
| **bing** | 微软必应 | ✅ 可用(cn.bing.com) | 中英文搜索、学术内容 |
| **baidu** | 百度搜索 | ✅ 可用 | 中文搜索、国内内容 |
| **serper** | Google API代理 | ⚠️ 需API Key | 全面搜索、最新信息 |

### 选择建议

1. **中文搜索**：优先使用 `baidu` 或 `bing`
2. **英文/技术搜索**：优先使用 `bing` 或 `serper`
3. **隐私敏感**：使用 `duckduckgo`
4. **最全面结果**：使用 `serper`（需要API Key）

## 自适应路由机制

### 工作原理

```
用户请求 → 路由器 → 选择引擎 → 执行搜索
                ↓
            健康检查
                ↓
        引擎异常? → 自动降级到备选引擎
```

### 路由策略

| 策略 | 说明 | 适用场景 |
|------|------|----------|
| **health_first** | 优先选择健康状态最佳的引擎 | 默认策略，推荐使用 |
| **round_robin** | 在健康引擎间轮询分流 | 高并发场景，负载均衡 |
| **weighted** | 基于成功率加权选择 | 需要智能路由时 |

### 熔断机制

- **熔断条件**：连续3次请求失败
- **熔断时间**：60秒
- **自动恢复**：熔断超时后自动探测恢复

## 最佳实践

### 1. 查询优化

```
❌ 不推荐：query="怎么"
✅ 推荐：query="Python 怎么实现异步HTTP请求"

❌ 不推荐：query="最新新闻"
✅ 推荐：query="2024年AI技术发展最新动态"
```

### 2. 引擎选择

```
场景：搜索中文技术博客
推荐：preferred_engine="baidu" 或 "bing"

场景：搜索英文开源项目文档
推荐：preferred_engine="bing" 或 "duckduckgo"

场景：需要最新、最全面的搜索结果
推荐：preferred_engine="serper"（如果配置了API Key）
```

### 3. 降级处理

当 `fallback_triggered: true` 时，说明首选引擎不可用，已自动切换。
返回的 `original_engine` 字段会显示原本选择的引擎。

## 错误处理

### 常见错误

| 错误 | 原因 | 解决方案 |
|------|------|----------|
| "no healthy engines available" | 所有引擎都不可用 | 检查网络连接，等待引擎恢复 |
| "search failed" | 搜索请求失败 | 尝试换一个引擎或稍后重试 |
| "API key not configured" | Serper API未配置 | 配置SERPER_API_KEY环境变量 |

## 配置示例

### 环境变量

```bash
# Serper API Key（可选，启用Google搜索）
export SERPER_API_KEY=your_api_key

# 默认搜索引擎
export SEARCH_DEFAULT_ENGINE=bing
```

### 配置文件

```json
{
  "search": {
    "default_engine": "bing",
    "max_results": 10,
    "auto_fallback": true,
    "engines": {
      "duckduckgo": {"enabled": true, "priority": 2},
      "bing": {"enabled": true, "priority": 1},
      "baidu": {"enabled": true, "priority": 3},
      "serper": {"enabled": true, "api_key": "xxx", "priority": 4}
    },
    "router": {
      "strategy": "health_first",
      "circuit_breaker_threshold": 3,
      "circuit_breaker_timeout": "60s"
    }
  }
}
```
