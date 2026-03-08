# OpenClaw 相关方案文档

本目录包含以下方案文档：

## 文档列表

### MediaCraft 多媒体 Agent

| 文档 | 说明 |
|-----|------|
| [01-openclaw-agent-proposal.md](./01-openclaw-agent-proposal.md) | 使用 OpenClaw 实现文生图/视频 Agent 的完整方案 |
| [02-local-llm-um890-guide.md](./02-local-llm-um890-guide.md) | 铭凡 UM890 (96GB) 本地文生图/文生视频模型部署方案 |
| [03-llm-free-tier-comparison.md](./03-llm-free-tier-comparison.md) | 2026年文生图/文生视频 API 免费额度对比（薅羊毛指南） |
| [04-children-growth-handbook-generator.md](./04-children-growth-handbook-generator.md) | 使用 LLM 生成儿童成长手册的完整方案 |
| [05-openclaw-skills-guide.md](./05-openclaw-skills-guide.md) | OpenClaw Skills 推荐：OCR / 文生图 / 文生视频 |
| [06-mediacraft-agent-deploy-guide.md](./06-mediacraft-agent-deploy-guide.md) | MediaCraft AI Agent 完整部署指南 v4（自我进化系统） |
| [07-deploy-checklist.md](./07-deploy-checklist.md) | **部署清单：API 申请 + Skills 安装 Checklist** |
| [agent-config/](./agent-config/) | **全部 Agent 配置**（6 个 Agent + 统一路由 + 备份脚本） |

### 飞书 + 多 Agent 平台

| 文档 | 说明 |
|-----|------|
| [08-rd-agent-team-plan.md](./08-rd-agent-team-plan.md) | **研发 Agent 团队构建方案**（飞书群+项目+7个角色 Agent） |
| [09-multi-user-routing-plan.md](./09-multi-user-routing-plan.md) | **多用户路由隔离方案**（防信息污染、权限分级） |

### 金融 Agent 群

| 文档 | 说明 |
|-----|------|
| [10-finance-agent-team-plan.md](./10-finance-agent-team-plan.md) | **金融分析/荐股/盯盘/炒股 Agent 群方案**（6个角色 Agent） |

### 全平台部署

| 文档 | 说明 |
|-----|------|
| [12-full-platform-guide.md](./12-full-platform-guide.md) | **全平台 Agent 部署指南**（6团队+路由+备份+中国大陆适配） |

### OpenClaw 生态

| 文档 | 说明 |
|-----|------|
| [11-openclaw-use-cases-revenue.md](./11-openclaw-use-cases-revenue.md) | **OpenClaw 真实案例与直接收益整理**（赚钱方向+ROI 数据） |

---

## 快速导航

### 研发 Agent 团队（NEW）

在飞书中构建 AI 员工团队，一个飞书群 = 一个项目：

| 角色 | Agent 昵称 | 职责 |
|------|-----------|------|
| 项目经理 | 小项 | 需求分析、任务拆解、进度跟踪 |
| 架构师 | 小架 | 架构设计、技术方案、Code Review |
| 后端 | 小后 | 后端开发、API 设计、数据库 |
| 前端 | 小前 | 前端开发、UI/UX |
| 测试 | 小测 | 测试用例、Bug 分析 |
| 运维 | 小运 | CI/CD、部署、监控 |
| 文档 | 小文 | 文档生成、变更日志 |

同一角色可服务多个项目；支持 @某角色 群聊和 1v1 私聊

👉 [查看研发团队方案](./08-rd-agent-team-plan.md)

### 多用户路由隔离（NEW）

多人共用一个 OpenClaw，互不干扰：

| 隔离方案 | 适用场景 | 复杂度 |
|---------|---------|--------|
| Session 级（dmScope） | 日常多人使用 | 低 |
| Agent 级（独立 workspace） | 高安全需求 | 中 |
| 混合方案 | 管理员+普通用户+VIP | 推荐 |

👉 [查看路由隔离方案](./09-multi-user-routing-plan.md)

### 金融 Agent 群（NEW）

7x24 小时金融分析团队：

| 角色 | 职责 | 数据源 |
|------|------|--------|
| 首席分析师 | 宏观研判、大盘分析 | Tushare + 新闻 |
| 选股师 | 多维度选股、潜力股推荐 | Tushare + Yahoo |
| 盯盘员 | 实时监控、异动告警 | 实时行情 |
| 交易助手 | 买卖信号、仓位管理 | 综合 |
| 研报分析师 | 财报解读、研报分析 | 财务数据 |
| 风控官 | 风险评估、回撤预警 | 持仓数据 |

支持自动化工作流：早盘研判 → 盘中盯盘 → 收盘复盘 → 周报

👉 [查看金融方案](./10-finance-agent-team-plan.md)

### OpenClaw 赚钱/收益案例（NEW）

| 方向 | 月收益参考 | 难度 |
|------|----------|------|
| AI 内容代写 | ¥3,000-12,000 | 入门 |
| AI 客服外包 | ¥300-3,600/10客户 | 入门 |
| 自动化工作流 | ¥5,000-50,000/项目 | 中级 |
| SaaS 产品 | ¥50,000+ | 高级 |
| 量化交易 | 因人而异 | 高级 |

企业场景：邮件处理 -78% 时间、客户入职 12x 加速、KPI 报表从 4h 到 5min

👉 [查看完整案例](./11-openclaw-use-cases-revenue.md)

### MediaCraft 多媒体 Agent

| 类别 | 首推技能 | 成本 |
|-----|---------|------|
| **OCR** | smart-ocr (PaddleOCR) | 免费 |
| **文生图** | pollinations / gemini | 免费 |
| **文生视频** | 即梦 / 可灵 | 免费额度 |

👉 [查看 MediaCraft 部署指南](./06-mediacraft-agent-deploy-guide.md) | [部署 Checklist](./07-deploy-checklist.md)

## 相关链接

- [OpenClaw 官方文档](https://docs.openclaw.ai/)
- [飞书开放平台](https://open.feishu.cn/)
- [Tushare 金融数据](https://tushare.pro/)
- [ComfyUI](https://github.com/comfyanonymous/ComfyUI) - 本地文生图工作流
- [即梦 AI](https://jimeng.jianying.com/) - 字节文生图/视频
- [可灵 AI](https://klingai.com/) - 快手文生视频

---

*最后更新: 2026-03-08*
