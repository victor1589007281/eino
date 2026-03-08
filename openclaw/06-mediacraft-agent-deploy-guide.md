# MediaCraft AI Agent 完整部署指南 v4

## 1. 概述

MediaCraft 是一个基于 **Coordinator-Worker Swarm 并发架构**的多媒体 AI Agent，在 OpenClaw 中运行。

### v4 核心变更

| 变更 | 说明 |
|-----|------|
| **自我进化系统** | 三层记忆架构（MEMORY.md + 日志 + 会话），用户偏好自动捕获，错误模式识别与预防 |
| **错误复利效应** | API 可靠性评分 → 动态降级链 → 预防性规避已知问题 |
| **memory-tools Skill** | 持久化记忆搜索与管理，支持语义检索 |

> v3 能力保留：协作最佳实践 / 自动提示词优化 / Worker 协调规范

### 架构图

```
用户请求
    │
    ▼
┌─ Coordinator（qwen3.5-plus via 百炼）─────────────────┐
│  0. 查询 MEMORY.md（偏好 + 错误历史 + 精华提示词）     │
│  1. 意图识别 → 任务拆解                                │
│  2. 自动提示词优化（CRAFTS + 精华库复用）              │
│  3. spawn Workers（自包含指令 + 最小权限）              │
│  4. 监控进度 → 降级处理 → 收集结果                     │
│  5. 汇总回复 → 写入记忆（偏好/错误/精华）             │
└───┬──────────┬──────────┬──────────┬───────────────────┘
    │          │          │          │
    ▼          ▼          ▼          ▼
Worker-A    Worker-B    Worker-C    Worker-D
(OCR)       (文生图)    (文生视频)  (文案生成)
smart-ocr   动态降级链   动态降级链  百炼
            (基于可靠性  (基于可靠性  qwen3.5-plus
             评分排序)    评分排序)
```

## 2. 文件结构

```
agent-config/
├── openclaw.json                              # 主配置（v4: +记忆系统+memoryFlush）
├── workspace/
│   ├── SOUL.md                                # Agent 人格（v4: +自我进化特质）
│   ├── AGENTS.md                              # 行为准则（v4: +自我进化系统 Part 8）
│   ├── USER.md                                # 用户偏好（百炼Plan+零成本）
│   └── MEMORY.md                              # 长期记忆模板（偏好/API评分/精华库）
└── skills/
    ├── api-fallback-guard/                    # 降级守卫 Skill
    │   └── SKILL.md
    └── children-growth-handbook/              # 成长手册 Skill
        ├── SKILL.md
        └── scripts/
            ├── handbook_gen.py                # 生成脚本（百炼 qwen3.5-plus）
            └── requirements.txt
```

## 3. 安装步骤

### 3.1 安装 Skills（16 个）

> 详细的分步安装说明见 [07-deploy-checklist.md](./07-deploy-checklist.md)

```bash
# ── 安全（2个）──
npx playbooks add skill openclaw/skills --skill secure-install
npx playbooks add skill openclaw/skills --skill clawsec-suite

# ── OCR（2个）──
npx playbooks add skill openclaw/skills --skill smart-ocr
npx playbooks add skill openclaw/skills --skill pdf-text-extractor

# ── 文生图（5个，全部免费/免费层）──
npx playbooks add skill openclaw/skills --skill pollinations
npx playbooks add skill openclaw/skills --skill siliconflow-image-gen
npx playbooks add skill openclaw/skills --skill gemini-image-gen
openclaw add @agmmnn/fal-ai
npx playbooks add skill openclaw/skills --skill krea-api

# ── 文/图生视频（3个）──
npx playbooks add skill openclaw/skills --skill video-gen
npx playbooks add skill openclaw/skills --skill siliconflow-video-gen
npx playbooks add skill openclaw/skills --skill ai-video-generation

# ── 提示词优化 & 记忆（2个）──
npx playbooks add skill openclaw/skills --skill prompt-enhancer
npx playbooks add skill openclaw/skills --skill memory-tools

# ── 自定义（2个）──
cp -r agent-config/skills/api-fallback-guard ~/.openclaw/skills/
cp -r agent-config/skills/children-growth-handbook ~/.openclaw/skills/
pip install -r ~/.openclaw/skills/children-growth-handbook/scripts/requirements.txt
```

### 3.2 部署配置

```bash
cp agent-config/openclaw.json ~/.openclaw/openclaw.json
cp agent-config/workspace/SOUL.md ~/.openclaw/workspace/SOUL.md
cp agent-config/workspace/AGENTS.md ~/.openclaw/workspace/AGENTS.md
cp agent-config/workspace/USER.md ~/.openclaw/workspace/USER.md
cp agent-config/workspace/MEMORY.md ~/.openclaw/workspace/MEMORY.md
mkdir -p ~/.openclaw/workspace/output/{images,videos,handbooks}
mkdir -p ~/.openclaw/workspace/memory
```

### 3.3 配置 API Keys

```bash
cat > ~/.openclaw/.env << 'ENVEOF'
# ── 阿里百炼 Coding Plan（必填，驱动所有 LLM）──
DASHSCOPE_API_KEY=你的百炼API密钥

# ── 文生图 免费API（至少配1个）──
SILICONFLOW_API_KEY=你的硅基流动密钥
GEMINI_API_KEY=你的google密钥
FAL_KEY=你的fal密钥

# ── 文生视频 免费API（至少配1个）──
KLING_API_KEY=你的可灵密钥
JIMENG_API_KEY=你的即梦密钥

# ── 可选 ──
KREA_API_KEY=你的krea密钥
ENVEOF

echo 'source ~/.openclaw/.env' >> ~/.zshrc && source ~/.openclaw/.env
```

### 3.4 启动验证

```bash
openclaw start
openclaw skill list   # 应看到 16 个 Skills 全部 ✅
openclaw doctor --security
```

## 4. Skills 完整清单（16 个）

| # | Skill | 类型 | 来源 | 作用 | 成本 |
|---|-------|------|------|------|------|
| 1 | secure-install | 安全 | 官方 | 安装前恶意检测 | 免费 |
| 2 | clawsec-suite | 安全 | SentinelOne | 全生命周期防护 | 免费 |
| 3 | smart-ocr | OCR | 官方 | PaddleOCR 100+语言 | 免费（本地） |
| 4 | pdf-text-extractor | OCR | 官方 | PDF 文字提取 | 免费（本地） |
| 5 | pollinations | 文生图 | 官方合作 | Flux/Turbo | **完全免费** |
| 6 | siliconflow-image-gen | 文生图 | 官方 | Flux Schnell | **Schnell 免费** |
| 7 | gemini-image-gen | 文生图/图生图 | 官方 | Gemini+Imagen | **250 请求/天免费** |
| 8 | fal-ai | 文生图/视频 | 社区验证 | 600+ 模型 | 注册免费额度 |
| 9 | krea-api | 文生图/图生图 | 社区验证 | Imagen4/Ideogram3 | 有限免费 |
| 10 | video-gen | 文/图生视频 | 官方 | 可灵/即梦/海螺 | **每日+每月免费** |
| 11 | siliconflow-video-gen | 文/图生视频 | 官方 | Wan2.2 14B | 注册免费额度 |
| 12 | ai-video-generation | 文/图生视频 | 官方 | 40+ 模型 | 按模型 |
| 13 | prompt-enhancer | 提示词优化 | 官方 | 自动优化+分析 | $0.01/次（备用） |
| 14 | memory-tools | 记忆管理 | 官方 | 持久化记忆搜索 | 免费 |
| 15 | api-fallback-guard | 降级守卫 | **自定义** | 免费API降级链管理 | 免费 |
| 16 | children-growth-handbook | 成长手册 | **自定义** | 儿童成长册PDF | 百炼配额内 |

> - `prompt-enhancer` 为备用，**主提示词优化由 Coordinator 内置**（百炼 qwen3.5-plus）
> - `memory-tools` 提供 memory_get / memory_search，支撑自我进化系统

## 5. Coordinator-Worker 协作机制

### 5.1 协作四原则

| 原则 | 说明 |
|------|------|
| **指令自包含** | 每个 Worker 的 instruction 包含完整任务、参数、返回格式、禁止事项 |
| **最小权限** | 只给 Worker 分配执行所需的 Skills |
| **统一返回格式** | `SUCCESS\|skill\|result`、`FALLBACK\|skill\|error`、`ERROR\|skill\|error` |
| **Coordinator 中转** | Worker 之间不直接通信，所有协调通过 Coordinator |

### 5.2 Worker 指令模板

```
任务：{task_type}
参数：{params_json}
使用技能：{skill_name}

执行流程：
1. 使用 {skill_name} 执行任务
2. 成功 → 返回: SUCCESS|{skill_name}|{result_path_or_data}
3. 错误含 quota/limit/exceeded/402/429/billing/payment
   → 返回: FALLBACK|{skill_name}|{error_summary}
4. 其他错误 → 重试1次，仍失败: ERROR|{skill_name}|{error_summary}

禁止：不授权付费，不调用其他技能，不修改提示词。
```

### 5.3 结果收集方式

| 方式 | 适用场景 |
|------|---------|
| 自动公告（默认） | Worker 完成后自动向 Coordinator 推送结果 |
| sessions_history | Coordinator 主动拉取 Worker 的执行过程细节 |
| sessions_send | 向运行中的 Worker 追加指令（如参数调整） |

### 5.4 DAG 依赖编排

有依赖的任务分阶段执行：

```
Phase 1（并行）: 文案生成 + 提示词优化
Phase 2（等 Phase 1）: 图片生成（用优化后的提示词）
Phase 3（等 Phase 2）: 图生视频（用生成的图片）
汇总：所有产出一并返回
```

### 5.5 探针策略

批量任务先 spawn 1 个 Worker 试水：
- 验证 API 可用性和参数正确性
- 展示给用户确认质量
- 确认后再并行执行剩余任务

## 6. 自动提示词优化

### 6.1 优化流程

```
用户: "画一只猫"
  │
  ├── Step 1: 语言检测 → 中文，需翻译
  ├── Step 2: CRAFTS 补全（场景/角色/动作/格式/情绪/风格）
  ├── Step 3: 质量增强词注入
  └── Step 4: 输出英文提示词
  │
  ▼
"A fluffy orange tabby cat sitting on a sunlit windowsill,
 curious expression, warm natural lighting, illustration style,
 detailed fur texture, cozy atmosphere, high quality"
```

### 6.2 图片 vs 视频优化差异

| 维度 | 图片 | 视频 |
|------|------|------|
| 长度 | 50-120 词 | 30-60 词（简洁优先） |
| 运动 | 不需要 | **必须描述动作和运动方向** |
| 镜头 | 可选 | **必须用电影术语**（pan, dolly, tracking） |
| 节奏 | 无 | 需指定（slow-motion, normal, fast） |

### 6.3 风格预设

| 主题 | 默认风格 | 增强词 |
|------|---------|--------|
| 人物 | cinematic portrait | professional lighting, 85mm, shallow DOF |
| 风景 | landscape photography | golden hour, dramatic sky, wide angle |
| 动物 | warm illustration | soft colors, detailed fur, cozy |
| 科幻 | sci-fi concept art | volumetric lighting, neon, cyberpunk |
| 儿童 | kawaii illustration | pastel colors, round shapes, cheerful |
| 食物 | food photography | overhead shot, warm tones, appetizing |

### 6.4 优化时机

| 场景 | 是否优化 |
|------|---------|
| 中文描述 | 必须（翻译+增强） |
| 简短英文 (<20词) | 建议（补充细节） |
| 详细英文 (>50词) | 轻微调整（只加质量词） |
| 用户说"按原文" | 不优化 |
| 批量同系列 | 不重复（只替换变化部分） |

### 6.5 零成本实现

提示词优化由 Coordinator 直接执行（百炼 qwen3.5-plus），**不消耗任何外部 API 额度**。这是在免费 API 上获得最佳质量的关键手段。

## 7. 并发参数

```json5
"subagents": {
  "maxSpawnDepth": 2,
  "maxChildrenPerAgent": 6,
  "maxConcurrent": 10
}
```

| 场景 | 并行 Worker 数 | 预期提速 |
|-----|---------------|---------|
| 4 张不同风格图片 | 4 | ~3.5x |
| 5 张图片 OCR | 5 | ~4x |
| 20 页手册文案 | 4（每组5页） | ~3x |
| 图片+视频同时 | 2 | ~1.8x |

## 8. API 降级链

```
文生图:   pollinations → siliconflow → gemini → fal-ai → krea → STOP
图生图:   gemini → krea → fal-ai → STOP
文生视频: 即梦 → 可灵 → siliconflow → ai-video-gen → STOP
图生视频: 可灵I2V → 即梦I2V → siliconflow → STOP
```

触发关键词：`quota, exceeded, limit, 402, 429, billing, payment, 额度, 配额, 余额不足, 需要付费`

全部耗尽时告知用户等待次日刷新。

> v4 增强：降级链顺序可根据 MEMORY.md 中的 API 可靠性评分**动态调整**。

## 9. 自我进化系统

### 9.1 三层记忆

| 层 | 文件 | 内容 | 生命周期 |
|----|------|------|---------|
| 长期 | `MEMORY.md` | 用户偏好、API 评分、提示词精华库、错误模式 | 永久，自动+手动维护 |
| 日志 | `memory/YYYY-MM-DD.md` | 每日调用记录、错误明细、会话摘要 | 30 天衰减 |
| 会话 | 上下文 | 本轮 API 状态、降级记录 | 会话结束清除 |

### 9.2 学习触发点

- 用户表达偏好 → 写入 MEMORY.md
- API 调用完成 → 记录成功/失败到日志
- 降级发生 → 更新错误模式 + API 评分
- 用户满意结果 → 提示词入精华库
- 每周日 → 汇总 API 可靠性评分

### 9.3 复利效应

```
Week 1: 学习用户偏好（水彩风格）、记录 API 基线
Week 2: 识别 siliconflow 月中额度紧张模式 → 自动调整降级优先级
Week 3: 积累 10 条精华提示词 → 40% 请求直接复用，节省百炼配额
Week 4: 根据内容审核历史 → 预防性规避 gemini 的敏感词拦截
→ 结果: 降级次数 ↓60%，平均耗时 ↓38%，无效调用 ↓70%
```

## 10. 月度成本

| 项目 | 成本 | 说明 |
|-----|------|------|
| 阿里百炼 Coding Plan | ¥7.9-200/月 | 用户已购 |
| 外部文生图 API | **¥0** | 全部免费额度 |
| 外部文生视频 API | **¥0** | 全部免费额度 |
| 提示词优化 | **¥0** | 百炼 Coding Plan 内 |
| 记忆系统 | **¥0** | 本地 Markdown 文件 |
| OCR | **¥0** | 本地 PaddleOCR |
| 成长手册 | **¥0** | 百炼配额内 |
| **外部 API 总计** | **¥0** | |

---

*文档版本: 4.0.0 | 最后更新: 2026-03-08*
*v4 变更：自我进化记忆系统 / 错误复利效应 / 动态降级链 / memory-tools Skill*
*v3 保留：协作最佳实践 / 自动提示词优化 / Worker 协调规范 / DAG 编排*
