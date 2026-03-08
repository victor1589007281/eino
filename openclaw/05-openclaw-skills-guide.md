# OpenClaw Skills 推荐指南：OCR / 文生图 / 文生视频

## 0. 安全前置说明

2026 年 2 月 OpenClaw 发生了 **ClawHavoc 供应链攻击**，约 12-20% 的 ClawHub 技能被注入恶意代码（键盘记录器、密码窃取器等）。安装任何技能前必须确认安全性。

### 安全检查清单

```
安装前必查：
  ✅ 技能有公开的 GitHub 代码仓库
  ✅ 作者有公开的技术背景和活跃记录
  ✅ 使用 secure-install 技能预检（查询 ClawDex API）
  ✅ 查看 ClawSecure Registry 的审计评分（80+ 为验证通过）
  ✅ 检查 SKILL.md 的权限声明，无多余权限请求

  ❌ 红旗：无代码仓库、下载量异常膨胀、描述模糊、要求文件系统/网络权限但功能不需要
```

### 安全工具

| 工具 | 说明 | 地址 |
|-----|------|------|
| **secure-install** | 安装前自动查询恶意技能数据库 | ClawDex API |
| **ClawSecure Registry** | 2,890+ 审计技能，3 层审计协议 | clawsecure.ai/registry |
| **OpenClaw 沙箱** | 建议默认无网络沙箱运行 | 内置功能 |

---

## 1. OCR 技能

### 1.1 推荐一览

| 技能 | 引擎 | 语言支持 | 成本 | 安全性 | 推荐指数 |
|-----|------|---------|------|--------|---------|
| **smart-ocr** | PaddleOCR | 100+ 语言 | 免费（本地） | ✅ 安全 | ⭐⭐⭐⭐⭐ |
| **pdf-text-extractor** | Tesseract.js | 多语言 | 免费（本地） | ✅ 安全 | ⭐⭐⭐⭐ |
| **gemini-image-gen** (OCR模式) | Gemini Vision | 多语言 | API 计费 | ✅ 安全 | ⭐⭐⭐⭐ |

### 1.2 smart-ocr（首推）

```yaml
名称: smart-ocr
作者: openclaw/skills (官方)
引擎: PaddleOCR
安全: ✅ 官方验证
成本: 完全免费（本地运行）

功能:
  - 从照片、截图、扫描件中提取文字
  - 手写体识别
  - 100+ 语言自动检测
  - 返回文字位置坐标和置信度
  - 支持 GPU 加速

安装:
  npx playbooks add skill openclaw/skills --skill smart-ocr

适用场景:
  - 批量文档数字化
  - 截图文字提取
  - 多语言混排文档
  - 表格结构识别

优势:
  - PaddleOCR 是百度开源，成熟稳定
  - 中文识别效果特别好
  - 本地运行，数据不出境
  - 零 API 成本

局限:
  - 需要安装 PaddleOCR 依赖
  - 复杂版式（杂志排版）效果一般
```

### 1.3 pdf-text-extractor

```yaml
名称: pdf-text-extractor
作者: openclaw/skills (官方)
引擎: Tesseract.js + 原生 PDF 解析
安全: ✅ 官方验证
成本: 完全免费（本地运行）

功能:
  - 文本型 PDF 零依赖提取
  - 扫描件 PDF 通过 Tesseract.js OCR
  - 输出格式：纯文本 / JSON / Markdown / HTML
  - 批量处理
  - 元数据提取（作者、日期等）
  - 自动语言检测

安装:
  npx playbooks add skill openclaw/skills --skill pdf-text-extractor

适用场景:
  - PDF 文档批量转文字
  - 学术论文提取
  - 合同/发票数字化

优势:
  - 文本型 PDF 秒级提取
  - 多种输出格式，适配不同下游任务
  - Tesseract 是最老牌的开源 OCR

局限:
  - 扫描件 OCR 精度不如 PaddleOCR / Surya
  - 不支持图片直接输入（仅 PDF）
```

### 1.4 OCR 技能对比

```
精度排名（中文文档）:
  1. smart-ocr (PaddleOCR)   ████████████████████  95%+
  2. Gemini Vision API        ███████████████████   93%+
  3. Surya-OCR (非 OpenClaw)  ██████████████████    92%
  4. pdf-text-extractor       ███████████████       87%
  5. Tesseract 原版            ████████████          80%

速度排名（单页）:
  1. pdf-text-extractor (文本PDF)  █  <0.1s
  2. smart-ocr (GPU)               ██  ~0.3s
  3. smart-ocr (CPU)               █████  ~1s
  4. Gemini Vision API             ████████  ~2s (含网络)
```

---

## 2. 文生图技能

### 2.1 推荐一览

| 技能 | 支持模型 | 免费额度 | 质量 | 安全性 | 推荐指数 |
|-----|---------|---------|------|--------|---------|
| **fal-ai** | Flux/SDXL/Recraft + 600+ | 有免费额度 | ⭐⭐⭐⭐⭐ | ✅ 社区验证 | ⭐⭐⭐⭐⭐ |
| **gemini-image-gen** | Gemini + Imagen | Google 免费层 | ⭐⭐⭐⭐⭐ | ✅ 官方 | ⭐⭐⭐⭐⭐ |
| **siliconflow-image-gen** | Flux/SD3.5 | Schnell 免费 | ⭐⭐⭐⭐ | ✅ 社区验证 | ⭐⭐⭐⭐ |
| **pollinations** | Flux/Turbo | 完全免费 | ⭐⭐⭐⭐ | ✅ 官方合作 | ⭐⭐⭐⭐ |
| **image-generation** | GPT Image/Flux/MJ 等 | 取决于 Provider | ⭐⭐⭐⭐⭐ | ✅ 官方 | ⭐⭐⭐⭐ |
| **krea-api** | Flux/Imagen4/Ideogram3 | 有限免费 | ⭐⭐⭐⭐⭐ | ✅ 社区验证 | ⭐⭐⭐⭐ |
| **reve-ai** | Reve AI | 有限免费 | ⭐⭐⭐⭐ | ⚠️ 需审查 | ⭐⭐⭐ |
| **venice-ai** | 30+ 模型 | 有免费 | ⭐⭐⭐⭐ | ⚠️ 需审查 | ⭐⭐⭐ |

### 2.2 fal-ai（综合最佳）

```yaml
名称: fal-ai
作者: agmmnn (社区)
GitHub: github.com/agmmnn/fal-ai-openclaw
安全: ✅ 2026 媒体技能评测第 1 名，代码开源
成本: fal.ai 新用户有免费额度，Schnell 模型约 $0.003/张

功能:
  - 600+ AI 模型（文生图/视频/音频）
  - 图片生成：Flux Schnell/Dev/Pro, SDXL, Recraft
  - 视频生成：MiniMax, WAN
  - 语音转文字：Whisper
  - 异步队列机制（提交-轮询-获取）
  - 无外部依赖（纯标准库）

安装:
  openclaw add @agmmnn/fal-ai
  
  # 配置 API Key
  export FAL_KEY="your-api-key"  # 从 fal.ai/dashboard/keys 获取

质量评测 (40+ 技能横评):
  输出质量: ⭐⭐⭐⭐⭐
  可靠性:   ⭐⭐⭐⭐⭐
  速度:     ⭐⭐⭐⭐
  文档:     ⭐⭐⭐⭐⭐

推荐理由:
  - 2026 年度最佳媒体技能（Oh My OpenClaw 评测）
  - 单个技能覆盖图片+视频+音频，减少安装多个技能的风险
  - 代码开源可审查
  - Flux Schnell 非常便宜
```

### 2.3 gemini-image-gen（免费首选）

```yaml
名称: gemini-image-gen
作者: openclaw/skills (官方)
安全: ✅ 官方验证
成本: Google Gemini 免费层即可使用

功能:
  - 文生图（Gemini 原生 + Imagen 3）
  - 图片编辑（文字指令修改已有图片）
  - 10 种风格预设：
    photo / anime / watercolor / cyberpunk / minimalist
    oil-painting / pixel-art / sketch / 3D-render / pop-art
  - HTML 画廊输出（批量预览）
  - 多种宽高比（1:1, 16:9, 9:16, 4:3, 3:4）
  - 纯 Python 标准库实现

安装:
  npx playbooks add skill openclaw/skills --skill gemini-image-gen

  # 配置 Google API Key
  export GEMINI_API_KEY="your-key"  # 从 aistudio.google.com 获取

免费额度 (Gemini):
  Gemini 2.5 Flash: 250 请求/天（含图片生成）
  Imagen 3: 通过 Vertex AI 有限免费

推荐理由:
  - Google 免费层每天可生成大量图片
  - 图片编辑功能是独特卖点
  - 多种风格预设开箱即用
  - 官方技能，安全有保障
```

### 2.4 siliconflow-image-gen（国内免费）

```yaml
名称: siliconflow-image-gen
作者: openclaw/skills (官方)
安全: ✅ 官方验证
成本: Flux.1 Schnell 模型免费

功能:
  - Flux.1 Schnell（免费）
  - Flux.1 Dev
  - Stable Diffusion 3.5

安装:
  npx playbooks add skill openclaw/skills --skill siliconflow-image-gen

免费额度:
  硅基流动注册后 2000 万 Token
  Flux.1 Schnell 模型完全免费

推荐理由:
  - 国内访问无障碍
  - Schnell 模型免费不限量
  - 速度快，延迟低
```

### 2.5 pollinations（零成本无门槛）

```yaml
名称: pollinations
作者: 官方合作（Pollinations.ai × OpenClaw）
安全: ✅ 官方合作项目（2026 柏林 Hackathon 发布）
成本: 完全免费，注册送免费 API Key

功能:
  - 图片生成：Flux / Turbo 等
  - 25+ AI 模型统一接入
  - 参数配置：质量、增强、NSFW 过滤
  - 30 秒完成配置

安装:
  # 在 OpenClaw 中直接使用
  # 获取 API Key: openclaw.pollinations.ai

推荐理由:
  - 完全免费，适合原型验证和低频使用
  - OpenClaw 官方合作，安全性有保障
  - 配置极简
  
局限:
  - 模型更新可能滞后于官方
  - 高并发可能限速
```

### 2.6 gemini-image-simple（极简方案）

```yaml
名称: gemini-image-simple
作者: openclaw/skills (官方)
安全: ✅ 官方验证
成本: Google 免费层

特点:
  - 零依赖（纯 Python 标准库）
  - 适用于无法安装 pip/uv 的受限环境
  - 最简单的图片生成方案

适用: 安全要求高、环境受限的场景
```

### 2.7 文生图技能对比

```
                    免费额度    质量    模型数   安全性    综合
fal-ai              ██         █████   █████    ████     ⭐⭐⭐⭐⭐
gemini-image-gen    ████       █████   ██       █████    ⭐⭐⭐⭐⭐
siliconflow         █████      ████    ███      █████    ⭐⭐⭐⭐
pollinations        █████      ████    ███      █████    ⭐⭐⭐⭐
image-generation    ██         █████   █████    █████    ⭐⭐⭐⭐
krea-api            ███        █████   ████     ████     ⭐⭐⭐⭐
reve-ai             ███        ████    ██       ███      ⭐⭐⭐
venice-ai           ███        ████    ████     ███      ⭐⭐⭐
```

---

## 3. 文/图生视频技能

### 3.1 推荐一览

| 技能 | 支持模型 | 免费额度 | 质量 | 安全性 | 推荐指数 |
|-----|---------|---------|------|--------|---------|
| **fal-ai** | MiniMax/WAN | fal.ai 免费额度 | ⭐⭐⭐⭐ | ✅ 社区验证 | ⭐⭐⭐⭐⭐ |
| **ai-video-generation** | Veo3/Seedance/Wan2.5/Grok | 取决于 Provider | ⭐⭐⭐⭐⭐ | ✅ 官方 | ⭐⭐⭐⭐⭐ |
| **ai-video-gen** | Luma/Runway/Replicate | 部分免费 | ⭐⭐⭐⭐ | ✅ 社区验证 | ⭐⭐⭐⭐ |
| **siliconflow-video-gen** | Wan2.2 14B | ¥2/段 | ⭐⭐⭐⭐ | ✅ 官方 | ⭐⭐⭐⭐ |
| **video-gen** | 可灵/海螺/Vidu/即梦 | 取决于 Provider | ⭐⭐⭐⭐⭐ | ✅ 官方 | ⭐⭐⭐⭐ |
| **openclaw-media-gen** | Wan 2.6 (视频) + Gemini 3 (图) | 单 Key | ⭐⭐⭐⭐ | ⚠️ 需审查 | ⭐⭐⭐ |

### 3.2 ai-video-generation（模型最全）

```yaml
名称: ai-video-generation
作者: openclaw/skills (官方)
安全: ✅ 官方验证
成本: 取决于选用的模型和 Provider

支持 40+ 模型:
  文生视频:
    - Google Veo 3.1 / Veo 3 / Veo 2
    - Seedance 1.5 Pro / Seedance 1.0
    - Wan 2.5
    - Grok Imagine Video
  图生视频:
    - Google Veo (I2V 模式)
    - Seedance (I2V 模式)
    - Wan 2.5 (I2V)
  人物动画:
    - OmniHuman（虚拟人/动画）
  口型同步:
    - Fabric 1.0

通过 inference.sh CLI 统一调用

安装:
  npx playbooks add skill openclaw/skills --skill ai-video-generation

推荐理由:
  - 模型数量最多，覆盖最全
  - 官方维护，安全可靠
  - 一个技能满足所有视频生成需求
  
局限:
  - 依赖 inference.sh CLI
  - 各模型计费分散
```

### 3.3 ai-video-gen（端到端管线）

```yaml
名称: ai-video-gen
作者: rhanbourinajd (社区)
GitHub: 开源可审查
安全: ✅ 8.8k Stars，社区验证
成本: 取决于 Provider

完整管线:
  1. 图片生成 → DALL-E 3 / SD / Flux
  2. 视频生成 → Luma Dream Machine / Runway / Replicate
  3. 配音     → OpenAI TTS / ElevenLabs
  4. 剪辑合成 → FFmpeg（转场、叠加、字幕）

功能:
  - 文字直接到成品视频（全自动管线）
  - 多场景视频拼接
  - 图片序列转视频
  - 配音集成
  - 免费和付费工作流均支持

安装:
  openclaw add @rhanbourinajd/ai-video-gen

推荐理由:
  - 端到端解决方案，不只是生成视频
  - 包含配音和后期制作
  - 适合制作完整的短视频内容
  
局限:
  - 依赖多个外部 API
  - 完整管线成本较高
```

### 3.4 siliconflow-video-gen（国内低价）

```yaml
名称: siliconflow-video-gen
作者: openclaw/skills (官方)
安全: ✅ 官方验证
成本: ¥2/段视频

模型: Wan2.2 14B
功能:
  - 文生视频 (Text-to-Video)
  - 图生视频 (Image-to-Video)
  - 电影级画面质量
  
安装:
  npx playbooks add skill openclaw/skills --skill siliconflow-video-gen

推荐理由:
  - 国内访问无障碍
  - ¥2/段是目前最低价之一
  - Wan2.2 14B 画质好
  - 官方技能安全可靠
```

### 3.5 video-gen（国产模型聚合）

```yaml
名称: video-gen
作者: openclaw/skills (官方)
安全: ✅ 官方验证

支持模型:
  - 可灵 Kling（最高 v2.6）
  - 海螺 Hailuo (MiniMax)
  - Vidu
  - 即梦 Jimeng

功能:
  - 文生视频
  - 图生视频
  - 多种分辨率

推荐理由:
  - 聚合了主流国产视频模型
  - 可灵和即梦有免费额度
  - 官方技能安全可靠
```

### 3.6 文/图生视频技能对比

```
                     免费额度   质量    模型数   管线完整度  安全    综合
fal-ai (视频)        ██        ████    ███     ██         ████   ⭐⭐⭐⭐⭐
ai-video-generation  ██        █████   █████   ███        █████  ⭐⭐⭐⭐⭐
ai-video-gen         ██        ████    ███     █████      ████   ⭐⭐⭐⭐
siliconflow-video    ██        ████    █       ██         █████  ⭐⭐⭐⭐
video-gen            ████      █████   ████    ██         █████  ⭐⭐⭐⭐
openclaw-media-gen   ███       ████    ██      ██         ███    ⭐⭐⭐
```

---

## 4. 推荐组合方案

### 4.1 零成本方案

```yaml
目标: 完全免费使用

OCR:
  技能: smart-ocr
  成本: ¥0（本地 PaddleOCR）

文生图:
  技能: pollinations + gemini-image-gen
  成本: ¥0
  额度: Pollinations 免费 + Google 免费层 250 请求/天

文生视频:
  技能: video-gen（用可灵/即梦免费额度）
  成本: ¥0
  额度: 即梦 ~6 段/天 + 可灵 ~18 段/月

月成本: ¥0
月产量: ~7,500 张图 + ~200 段视频 + 无限 OCR
```

### 4.2 性价比方案（推荐）

```yaml
目标: 低成本高质量

OCR:
  技能: smart-ocr
  成本: ¥0

文生图:
  技能: fal-ai（Flux Schnell）+ siliconflow-image-gen（免费 Schnell）
  成本: ~$5/月
  说明: fal-ai 用于高质量生成，siliconflow 免费兜底

文生视频:
  技能: fal-ai（MiniMax/WAN）+ video-gen（可灵/即梦免费）
  成本: ~¥50/月
  说明: 免费额度日常用，fal-ai 补充质量需求

月成本: ~¥85
月产量: 10,000+ 张图 + 500+ 段视频 + 无限 OCR
```

### 4.3 高质量方案

```yaml
目标: 最高质量输出

OCR:
  技能: smart-ocr + Gemini Vision（复杂版式兜底）
  成本: ~$5/月

文生图:
  技能: fal-ai（Flux Pro/Dev）+ image-generation（GPT Image 1.5）
  成本: ~$30/月
  说明: Flux Pro 日常用，GPT Image 用于需要精确文字的场景

文生视频:
  技能: ai-video-generation（Veo 3.1 / Seedance 1.5 Pro）
  成本: ~$50/月

月成本: ~$85（~¥600）
月产量: 5,000+ 张高质量图 + 200+ 段高质量视频
```

---

## 5. 安装与配置速查

### 5.1 一键安装推荐技能

```bash
# 1. 安装安全检查工具（先装这个）
npx playbooks add skill openclaw/skills --skill secure-install

# 2. 安装 OCR
npx playbooks add skill openclaw/skills --skill smart-ocr
npx playbooks add skill openclaw/skills --skill pdf-text-extractor

# 3. 安装文生图
openclaw add @agmmnn/fal-ai
npx playbooks add skill openclaw/skills --skill gemini-image-gen
npx playbooks add skill openclaw/skills --skill siliconflow-image-gen

# 4. 安装文生视频
npx playbooks add skill openclaw/skills --skill ai-video-generation
npx playbooks add skill openclaw/skills --skill video-gen
npx playbooks add skill openclaw/skills --skill siliconflow-video-gen
```

### 5.2 API Key 配置

```bash
# ~/.openclaw/.env 或通过 openclaw config set

# fal.ai（文生图/视频综合）
FAL_KEY=your-fal-key            # fal.ai/dashboard/keys

# Google（Gemini 图片生成 + OCR）
GEMINI_API_KEY=your-google-key  # aistudio.google.com/apikey

# 硅基流动（国内免费 Flux）
SILICONFLOW_API_KEY=your-key    # siliconflow.cn

# 可灵（高质量视频）
KLING_API_KEY=your-key          # klingai.com

# 即梦（通过火山引擎）
JIMENG_API_KEY=your-key         # volcengine.com
```

### 5.3 验证安装

```bash
# 测试 OCR
echo "请识别这张图片中的文字" | openclaw chat
# 附加一张图片测试

# 测试文生图
echo "用 Flux 生成一只在星空下奔跑的小狐狸，水彩风格" | openclaw chat

# 测试文生视频
echo "生成一段 5 秒的海浪拍打沙滩视频" | openclaw chat
```

---

## 6. 安全性总结

### 6.1 本文推荐技能的安全等级

| 技能 | 来源 | 安全等级 | 说明 |
|-----|------|---------|------|
| smart-ocr | 官方 | ✅ 安全 | 本地运行，数据不出境 |
| pdf-text-extractor | 官方 | ✅ 安全 | 本地运行 |
| fal-ai | 社区开源 | ✅ 安全 | 代码开源，评测第一 |
| gemini-image-gen | 官方 | ✅ 安全 | Google API |
| gemini-image-simple | 官方 | ✅ 安全 | 零依赖，最小攻击面 |
| siliconflow-image-gen | 官方 | ✅ 安全 | 硅基流动 API |
| siliconflow-video-gen | 官方 | ✅ 安全 | 硅基流动 API |
| pollinations | 官方合作 | ✅ 安全 | 官方 Hackathon 发布 |
| image-generation | 官方 | ✅ 安全 | 多 Provider 聚合 |
| ai-video-generation | 官方 | ✅ 安全 | inference.sh CLI |
| ai-video-gen | 社区开源 | ✅ 安全 | 8.8k Stars，代码可审查 |
| video-gen | 官方 | ✅ 安全 | 国产模型聚合 |
| krea-api | 社区 | ⚠️ 需审查 | 检查最新代码再安装 |
| reve-ai | 社区 | ⚠️ 需审查 | 检查最新代码再安装 |
| venice-ai | 社区 | ⚠️ 需审查 | 隐私好但需验证代码 |
| openclaw-media-gen | 社区 | ⚠️ 需审查 | 第三方 API 聚合 |

### 6.2 安全原则

```
1. 优先选择"官方"和"官方合作"技能
2. 社区技能必须有开源代码仓库
3. 安装前用 secure-install 预检
4. 查看 ClawSecure Registry 审计评分
5. OCR 类优先本地运行（smart-ocr），避免敏感文档上传
6. 定期检查已安装技能的更新（防止后门注入）
```

---

*文档版本: 1.0.0 | 最后更新: 2026-03-08*
*技能信息可能随 ClawHub 更新变化，安装前请核实最新状态*
