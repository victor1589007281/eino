---
name: api-fallback-guard
version: 1.0.0
author: mediacraft
description: |
  API 免费额度降级守卫。当任何外部 API 返回收费、配额耗尽、限流错误时，
  自动识别并返回降级信号。所有 Worker Sub-Agent 的任务指令中必须引用此技能的规则。
  此技能不直接调用任何工具，它是一套错误识别和降级决策的标准规范。
tools: Read
---

# API 免费额度降级守卫

## 核心原则

**只使用免费额度。收到任何收费信号，立即停止并切换。**

## 错误识别规则

以下关键词出现在 API 响应、错误信息、异常消息中的任何一个，判定为**额度耗尽/需要付费**：

### 英文关键词
```
quota, exceeded, limit, rate_limit, rate limit, too_many_requests,
insufficient, exhausted, billing, payment, payment_required,
credit, balance, subscription, upgrade, plan_limit
```

### HTTP 状态码
```
402 (Payment Required)
429 (Too Many Requests)
403 (Forbidden，且消息含 quota/limit/billing)
```

### 中文关键词
```
额度, 配额, 余额不足, 需要付费, 请充值, 免费额度已用完,
套餐, 升级, 限流, 请求过多, 积分不足, 灵感值不足
```

## 降级链定义

### 文生图

```
Chain: pollinations → siliconflow-image-gen → gemini-image-gen → fal-ai → krea-api → EXHAUSTED
```

| 顺序 | Skill | 免费额度 | 说明 |
|-----|-------|---------|------|
| 1 | pollinations | 无限 | 完全免费，质量中等 |
| 2 | siliconflow-image-gen | Schnell 免费 | 国内快，Flux 质量 |
| 3 | gemini-image-gen | 250 请求/天 | Google 免费层 |
| 4 | fal-ai | 注册免费额度 | 600+ 模型，质量最高 |
| 5 | krea-api | 有限免费 | Imagen4/Ideogram3 |

### 图生图（编辑/风格转换）

```
Chain: gemini-image-gen → krea-api → fal-ai → EXHAUSTED
```

### 文生视频

```
Chain: video-gen(jimeng) → video-gen(kling) → siliconflow-video-gen → ai-video-generation → EXHAUSTED
```

| 顺序 | Skill (Provider) | 免费额度 |
|-----|------------------|---------|
| 1 | video-gen (即梦) | 每日 66 积分 |
| 2 | video-gen (可灵) | 每月 366 积分 |
| 3 | siliconflow-video-gen | 注册免费额度 |
| 4 | ai-video-generation | 按模型 |

### 图生视频

```
Chain: video-gen(kling-i2v) → video-gen(jimeng-i2v) → siliconflow-video-gen → EXHAUSTED
```

## Worker 指令模板

### 文生图 Worker

```
你是一个图片生成 Worker。严格按以下流程执行：

1. 使用 {current_skill} 生成图片
   提示词: {prompt}
   尺寸: {size}

2. 成功 → 返回格式:
   SUCCESS|{skill_name}|{image_path}

3. 报错且错误包含以下任一关键词:
   quota, exceeded, limit, 402, 429, billing, payment,
   insufficient, exhausted, rate_limit, 额度, 配额, 余额不足
   → 返回格式:
   FALLBACK|{skill_name}|{error_summary}

4. 其他错误 → 重试 1 次，仍失败:
   ERROR|{skill_name}|{error_summary}

绝对禁止：不要授权付费、不要确认扣费、不要输入支付信息。
```

### 文生视频 Worker

```
你是一个视频生成 Worker。严格按以下流程执行：

1. 使用 {current_skill} 生成视频
   提示词: {prompt}
   时长: {duration}s
   分辨率: {resolution}

2. 成功 → 返回格式:
   SUCCESS|{skill_name}|{video_path}

3. 报错且错误包含收费/限流关键词:
   → 返回格式:
   FALLBACK|{skill_name}|{error_summary}

4. 超时（视频生成超过 {timeout}s）:
   TIMEOUT|{skill_name}|已等待{elapsed}s

绝对禁止：不要授权付费。
```

## Coordinator 降级处理流程

```
收到 Worker 返回 FALLBACK|xxx|yyy 后：

1. 记录 {xxx} 为"本次会话已耗尽"
2. 查找降级链中 {xxx} 的下一个 Skill
3. 如果有下一个 → spawn 新 Worker 用下一个 Skill 重试
4. 如果降级链到 EXHAUSTED → 告知用户:
   "⚠️ 所有免费 API 额度已用完。建议：
    a) 等待明天额度刷新
    b) 更换其他平台的 API Key
    c) 降低生成数量"
5. 向用户通知降级事件（简洁一行）
```

## 会话状态维护

在每次会话开始时，所有 API 默认可用。随着使用过程中的降级事件，维护状态：

```
本次会话 API 状态：
  [IMG] pollinations: ✅
  [IMG] siliconflow-image-gen: ❌ (14:30 quota exceeded)
  [IMG] gemini-image-gen: ✅
  [VID] video-gen(jimeng): ✅
  [VID] video-gen(kling): ⚠️ (剩余约 50 积分)
```

后续请求直接跳过已标记 ❌ 的 API，节省重试时间。
