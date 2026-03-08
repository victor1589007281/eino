# MediaCraft AI Agent 部署清单

> 部署前申请 API + 部署后安装 Skills 的完整 Checklist

---

## 一、部署前：API 申请清单

按优先级排列。标注 `必须` 的为核心依赖，`推荐` 的为增加冗余，`可选` 的按需申请。

### 1.1 LLM 引擎（必须）

| # | 平台 | 申请地址 | 获取内容 | 环境变量 | 优先级 | 说明 |
|---|------|---------|---------|---------|--------|------|
| 1 | **阿里百炼** | https://bailian.console.aliyun.com/ | API Key | `DASHSCOPE_API_KEY` | **必须** | Coding Plan，驱动全部 LLM 调用 |

### 1.2 文生图 API（至少申请 2 个）

| # | 平台 | 申请地址 | 获取内容 | 环境变量 | 优先级 | 免费额度 |
|---|------|---------|---------|---------|--------|---------|
| 2 | **Pollinations** | https://pollinations.ai/ | 无需 Key | 无 | **必须** | 完全免费，无限制 |
| 3 | **硅基流动** | https://cloud.siliconflow.cn/ | API Key | `SILICONFLOW_API_KEY` | **必须** | Flux Schnell 免费 |
| 4 | **Google Gemini** | https://aistudio.google.com/apikey | API Key | `GEMINI_API_KEY` | **推荐** | 250 请求/天免费 |
| 5 | **fal.ai** | https://fal.ai/dashboard | API Key | `FAL_KEY` | **推荐** | 注册赠送 $10 免费额度 |
| 6 | **Krea** | https://krea.ai/ | API Key | `KREA_API_KEY` | 可选 | 有限免费 |

### 1.3 文/图生视频 API（至少申请 1 个）

| # | 平台 | 申请地址 | 获取内容 | 环境变量 | 优先级 | 免费额度 |
|---|------|---------|---------|---------|--------|---------|
| 7 | **即梦 AI** | https://jimeng.jianying.com/ai-tool/home → API 开放平台 | API Key | `JIMENG_API_KEY` | **必须** | 每日 66 积分 |
| 8 | **可灵 AI** | https://klingai.com/ → 开发者平台 | API Key | `KLING_API_KEY` | **推荐** | 每月 366 积分 |

### 1.4 备用/可选

| # | 平台 | 申请地址 | 获取内容 | 环境变量 | 优先级 | 说明 |
|---|------|---------|---------|---------|--------|------|
| 9 | DeepSeek | https://platform.deepseek.com/ | API Key | `DEEPSEEK_API_KEY` | 可选 | LLM fallback 备用 |

### 1.5 申请顺序建议

```
Day 1（核心，5 分钟搞定）：
  ✅ 1. 阿里百炼 → DASHSCOPE_API_KEY
  ✅ 2. 硅基流动 → SILICONFLOW_API_KEY
  ✅ 3. Pollinations → 无需申请

Day 1（扩展，10 分钟）：
  ✅ 4. Google Gemini → GEMINI_API_KEY
  ✅ 5. 即梦 AI → JIMENG_API_KEY

Day 2（增加冗余，可选）：
  ✅ 6. fal.ai → FAL_KEY
  ✅ 7. 可灵 AI → KLING_API_KEY
  ✅ 8. Krea → KREA_API_KEY
  ✅ 9. DeepSeek → DEEPSEEK_API_KEY
```

### 1.6 API Key 配置

所有 Key 申请完毕后，统一写入环境变量：

```bash
cat > ~/.openclaw/.env << 'ENVEOF'
# ── 必须 ──
DASHSCOPE_API_KEY=your_key_here
SILICONFLOW_API_KEY=your_key_here

# ── 推荐 ──
GEMINI_API_KEY=your_key_here
JIMENG_API_KEY=your_key_here

# ── 可选 ──
FAL_KEY=your_key_here
KLING_API_KEY=your_key_here
KREA_API_KEY=your_key_here
DEEPSEEK_API_KEY=your_key_here
ENVEOF

echo 'source ~/.openclaw/.env' >> ~/.zshrc && source ~/.openclaw/.env
```

---

## 二、部署后：外部 Skills 安装清单

### 2.1 安装命令一览（16 个 Skill）

按类别分组，可复制整块执行：

```bash
# ══════════════════════════════════════════
# 安全 Skills（2 个）
# ══════════════════════════════════════════
npx playbooks add skill openclaw/skills --skill secure-install
npx playbooks add skill openclaw/skills --skill clawsec-suite

# ══════════════════════════════════════════
# OCR Skills（2 个）
# ══════════════════════════════════════════
npx playbooks add skill openclaw/skills --skill smart-ocr
npx playbooks add skill openclaw/skills --skill pdf-text-extractor

# ══════════════════════════════════════════
# 文生图 Skills（5 个）
# ══════════════════════════════════════════
npx playbooks add skill openclaw/skills --skill pollinations
npx playbooks add skill openclaw/skills --skill siliconflow-image-gen
npx playbooks add skill openclaw/skills --skill gemini-image-gen
openclaw add @agmmnn/fal-ai
npx playbooks add skill openclaw/skills --skill krea-api

# ══════════════════════════════════════════
# 文/图生视频 Skills（3 个）
# ══════════════════════════════════════════
npx playbooks add skill openclaw/skills --skill video-gen
npx playbooks add skill openclaw/skills --skill siliconflow-video-gen
npx playbooks add skill openclaw/skills --skill ai-video-generation

# ══════════════════════════════════════════
# 提示词优化 & 记忆 Skills（2 个）
# ══════════════════════════════════════════
npx playbooks add skill openclaw/skills --skill prompt-enhancer
npx playbooks add skill openclaw/skills --skill memory-tools

# ══════════════════════════════════════════
# 自定义 Skills（2 个）
# ══════════════════════════════════════════
cp -r agent-config/skills/api-fallback-guard ~/.openclaw/skills/
cp -r agent-config/skills/children-growth-handbook ~/.openclaw/skills/
pip install -r ~/.openclaw/skills/children-growth-handbook/scripts/requirements.txt
```

### 2.2 Skills 明细表

| # | Skill 名称 | 类别 | 安装方式 | 需要 API Key | 成本 |
|---|-----------|------|---------|-------------|------|
| 1 | secure-install | 安全 | playbooks add | 无 | 免费 |
| 2 | clawsec-suite | 安全 | playbooks add | 无 | 免费 |
| 3 | smart-ocr | OCR | playbooks add | 无 | 免费（本地） |
| 4 | pdf-text-extractor | OCR | playbooks add | 无 | 免费（本地） |
| 5 | pollinations | 文生图 | playbooks add | 无 | 完全免费 |
| 6 | siliconflow-image-gen | 文生图 | playbooks add | `SILICONFLOW_API_KEY` | Schnell 免费 |
| 7 | gemini-image-gen | 文生图/图生图 | playbooks add | `GEMINI_API_KEY` | 250次/天免费 |
| 8 | fal-ai | 文生图/视频 | openclaw add | `FAL_KEY` | $10 注册额度 |
| 9 | krea-api | 文生图/图生图 | playbooks add | `KREA_API_KEY` | 有限免费 |
| 10 | video-gen | 文/图生视频 | playbooks add | `KLING_API_KEY` / `JIMENG_API_KEY` | 每日+每月免费 |
| 11 | siliconflow-video-gen | 文/图生视频 | playbooks add | `SILICONFLOW_API_KEY` | 注册额度 |
| 12 | ai-video-generation | 文/图生视频 | playbooks add | 按模型 | 按模型 |
| 13 | prompt-enhancer | 提示词优化 | playbooks add | 无 | $0.01/次（备用） |
| 14 | memory-tools | 记忆管理 | playbooks add | 无 | 免费 |
| 15 | api-fallback-guard | 降级守卫 | 手动复制 | 无 | 免费 |
| 16 | children-growth-handbook | 成长手册 | 手动复制 + pip | `DASHSCOPE_API_KEY` | 百炼配额内 |

### 2.3 Skills 与 API Key 依赖关系图

```
DASHSCOPE_API_KEY ──→ Coordinator (LLM 引擎)
       │            → children-growth-handbook (文案生成)
       │
SILICONFLOW_API_KEY ─→ siliconflow-image-gen
       │              → siliconflow-video-gen
       │
GEMINI_API_KEY ──────→ gemini-image-gen
       │
FAL_KEY ─────────────→ fal-ai
       │
KLING_API_KEY ───────→ video-gen (可灵)
       │
JIMENG_API_KEY ──────→ video-gen (即梦)
       │
KREA_API_KEY ────────→ krea-api
       │
DEEPSEEK_API_KEY ────→ fallback LLM (备用)
       │
无需 Key ────────────→ pollinations
                     → smart-ocr
                     → pdf-text-extractor
                     → secure-install
                     → clawsec-suite
                     → prompt-enhancer
                     → memory-tools
                     → api-fallback-guard
```

---

## 三、部署配置文件

```bash
# 复制配置文件到 OpenClaw 目录
cp agent-config/openclaw.json ~/.openclaw/openclaw.json
cp agent-config/workspace/SOUL.md ~/.openclaw/workspace/SOUL.md
cp agent-config/workspace/AGENTS.md ~/.openclaw/workspace/AGENTS.md
cp agent-config/workspace/USER.md ~/.openclaw/workspace/USER.md
cp agent-config/workspace/MEMORY.md ~/.openclaw/workspace/MEMORY.md

# 创建输出目录
mkdir -p ~/.openclaw/workspace/output/{images,videos,handbooks}
mkdir -p ~/.openclaw/workspace/memory
```

---

## 四、验证清单

部署完成后逐项验证：

```bash
# 1. 启动 OpenClaw
openclaw start

# 2. 检查 Skills 是否全部加载
openclaw skill list
# 期望看到 16 个 Skills 全部 ✅ enabled

# 3. 安全检查
openclaw doctor --security

# 4. 功能验证（逐项测试）
```

| 测试项 | 输入 | 期望结果 |
|-------|------|---------|
| LLM 连通 | "你好" | 正常回复 |
| 文生图 | "画一只猫" | 提示词优化 → 生成图片 |
| OCR | 发送一张含文字的图片 | 识别出文字 |
| 降级测试 | 手动禁用 pollinations 后请求文生图 | 自动切换 siliconflow |
| 记忆写入 | "我喜欢水彩风格" → 新会话 → "画一朵花" | 自动使用水彩风格 |
| 成长手册 | "帮我做一份宝宝成长手册" | 生成 PDF |

---

## 五、成本总结

| 项目 | 费用 | 频率 |
|-----|------|------|
| 阿里百炼 Coding Plan | ¥7.9-200/月 | 已购 |
| 所有外部 API | **¥0** | 仅用免费额度 |
| 所有外部 Skills | **¥0** | 开源/免费 |
| **月度外部支出** | **¥0** | |

---

*文档版本: 1.0 | 日期: 2026-03-08*
