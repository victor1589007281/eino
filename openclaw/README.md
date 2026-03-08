# OpenClaw 相关方案文档

本目录包含以下方案文档：

## 文档列表

| 文档 | 说明 |
|-----|------|
| [01-openclaw-agent-proposal.md](./01-openclaw-agent-proposal.md) | 使用 OpenClaw 实现文生图/视频 Agent 的完整方案 |
| [02-local-llm-um890-guide.md](./02-local-llm-um890-guide.md) | 铭凡 UM890 (96GB) 本地文生图/文生视频模型部署方案 |
| [03-llm-free-tier-comparison.md](./03-llm-free-tier-comparison.md) | 2026年文生图/文生视频 API 免费额度对比（薅羊毛指南） |
| [04-children-growth-handbook-generator.md](./04-children-growth-handbook-generator.md) | 使用 LLM 生成儿童成长手册的完整方案 |
| [05-openclaw-skills-guide.md](./05-openclaw-skills-guide.md) | OpenClaw Skills 推荐：OCR / 文生图 / 文生视频 |
| [06-mediacraft-agent-deploy-guide.md](./06-mediacraft-agent-deploy-guide.md) | MediaCraft AI Agent 完整部署指南 v4（+自我进化系统） |
| [07-deploy-checklist.md](./07-deploy-checklist.md) | **部署清单：API 申请 + Skills 安装 Checklist** |
| [agent-config/](./agent-config/) | Agent 配置文件目录（可直接部署） |

## 快速导航

### 1. OpenClaw Agent 方案

如果你想用 OpenClaw 代替自研 Agent 来处理 LLM 相关逻辑：

- OpenClaw 是一个开源的 AI Agent 平台（180k+ Stars）
- 支持 50+ 消息平台（Telegram、Discord、微信等）
- 通过简单的 `SKILL.md` 文件定义技能
- 自动处理 LLM 调用、会话管理、意图识别

👉 [查看完整方案](./01-openclaw-agent-proposal.md)

### 2. 本地文生图/视频部署

铭凡 UM890 (96GB DDR5) 的文生图/视频能力评估：

| 方案 | 可行性 | 说明 |
|-----|--------|------|
| CPU + Flux GGUF | 勉强 | 5-15 分钟/张，偶尔用 |
| 780M 核显 | 不可行 | ROCm 不支持 780M |
| 外接 eGPU | 推荐 | RTX 4060+ 即可流畅使用 |
| 云 GPU 租用 | 推荐 | AutoDL RTX 4090 ¥2.5/h |

结论：UM890 适合做编排/后处理节点，文生图/视频建议 eGPU 或 API

👉 [查看完整方案](./02-local-llm-um890-guide.md)

### 3. 文生图/视频免费额度薅羊毛

2026年可免费使用的文生图/视频额度汇总：

**文生图：**
| 平台 | 免费额度 | 推荐指数 |
|-----|---------|---------|
| 即梦 (字节) | 66张/天 | ⭐⭐⭐⭐⭐ |
| 通义万相 | ~50张/天 | ⭐⭐⭐⭐⭐ |
| 可灵 | 366积分/月 | ⭐⭐⭐⭐ |

**文生视频：**
| 平台 | 免费额度 | 推荐指数 |
|-----|---------|---------|
| 即梦 | ~6段/天 | ⭐⭐⭐⭐⭐ |
| 可灵 | ~18段/月 | ⭐⭐⭐⭐⭐ |
| Pika | 80积分/月 | ⭐⭐⭐⭐ |
| Luma | 30段/月 | ⭐⭐⭐ |

理论月免费总量：**4,000+ 张图 + 300+ 段视频**

👉 [查看完整对比](./03-llm-free-tier-comparison.md)

### 4. 儿童成长手册生成

使用 LLM 自动生成儿童成长手册：

- 支持 5 种风格：森系水彩、北欧极简、绘本故事等
- 自动生成文案、排版、装饰
- 输出 A4 尺寸 PNG/PDF
- 成本：每本 < ¥1（不含印刷）

👉 [查看完整方案](./04-children-growth-handbook-generator.md)

### 5. OpenClaw Skills 推荐

精选安全、低成本、高质量的 OCR / 文生图 / 文生视频技能：

| 类别 | 首推技能 | 成本 | 安全性 |
|-----|---------|------|--------|
| **OCR** | smart-ocr (PaddleOCR) | 免费本地 | 官方 |
| **文生图** | fal-ai (Flux/SDXL 600+模型) | ~$0.003/张 | 社区验证第1 |
| **文生图(免费)** | pollinations / gemini-image-gen | 完全免费 | 官方合作 |
| **文生视频** | ai-video-generation (40+模型) | 取决于Provider | 官方 |
| **文生视频(低价)** | siliconflow-video-gen (Wan2.2) | ¥2/段 | 官方 |

零成本方案月产量：~7,500 张图 + ~200 段视频 + 无限 OCR

👉 [查看完整指南](./05-openclaw-skills-guide.md)

## 相关链接

- [OpenClaw 官方文档](https://docs.openclaw.ai/)
- [ComfyUI](https://github.com/comfyanonymous/ComfyUI) - 本地文生图工作流
- [即梦 AI](https://jimeng.jianying.com/) - 字节文生图/视频
- [可灵 AI](https://klingai.com/) - 快手文生视频
- [Pika](https://pika.art/) - AI 视频生成
- [Runway](https://runwayml.com/) - Gen-4 视频生成

---

*最后更新: 2026-03-08*
