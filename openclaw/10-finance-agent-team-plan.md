# 金融分析 Agent 群构建方案

> **适用场景**：OpenClaw + 飞书 + 阿里百炼 Coding Plan，mini 主机部署  
> **目标市场**：A 股为主，兼顾美股/港股/加密货币  
> **版本**：1.0.0 | 最后更新：2026-03-08

---

## ⚠️ 风险提示与免责声明

**本方案及其中所有 Agent 输出内容仅供参考，不构成任何投资建议、买卖建议或理财建议。**  
股市有风险，投资需谨慎。任何基于本方案做出的投资决策，风险由投资者自行承担。  
请遵守当地法律法规，理性投资，量力而行。

---

## 1. 整体架构

### 1.1 ASCII 架构图

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                              数据源层 (Data Sources)                                      │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │
│  │ Tushare Pro │  │Yahoo Finance│  │   AKShare   │  │  东方财富   │  │  财联社/    │   │
│  │  (A股核心)  │  │(美股/港股)  │  │ (A股备用)   │  │  新浪财经   │  │  金十新闻   │   │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘   │
└─────────┼────────────────┼────────────────┼────────────────┼────────────────┼──────────┘
          │                │                │                │                │
          └────────────────┴────────────────┴────────────────┴────────────────┘
                                          │
                                          ▼
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                              技能层 (Skills Layer)                                       │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│  tushare-finance │ stock-analysis │ stocks/yahoo-finance │ portfolio-watcher │ finance │
└─────────────────────────────────────────────────────────────────────────────────────────┘
                                          │
                                          ▼
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                              金融 Agent 群 (Agent Swarm)                                  │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                │
│  │ 首席分析师   │  │ 选股师      │  │ 盯盘员      │  │ 交易助手    │                │
│  │ Agent        │  │ Agent        │  │ Agent        │  │ Agent        │                │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘                │
│         │                 │                 │                 │                         │
│  ┌──────────────┐  ┌──────────────┐                                                      │
│  │ 研报分析师   │  │ 风控官      │                                                      │
│  │ Agent        │  │ Agent        │                                                      │
│  └──────┬───────┘  └──────┬───────┘                                                      │
│         │                 │                                                              │
│         └─────────────────┴──────────────────────────────────────────────────────────────┤
│                                          │                                                │
│                                          ▼                                                │
│                              ┌───────────────────────┐                                   │
│                              │  Coordinator Agent    │                                   │
│                              │  (任务分发/结果汇总)   │                                   │
│                              └───────────┬───────────┘                                   │
└──────────────────────────────────────────┼──────────────────────────────────────────────┘
                                           │
                                           ▼
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                              输出层 (Output Channels)                                     │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────┐  ┌─────────────────────┐  ┌─────────────────────┐              │
│  │ 飞书群推送          │  │ 飞书私聊            │  │ REST API / WebSocket │              │
│  │ (早盘/盘中/复盘)   │  │ (告警/提醒)         │  │ (外部系统对接)       │              │
│  └─────────────────────┘  └─────────────────────┘  └─────────────────────┘              │
└─────────────────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Agent 群角色设计

| 角色 | 核心职责 | 协作关系 |
|------|----------|----------|
| **Coordinator** | 接收用户请求，分发任务，汇总结果 | 调度所有子 Agent |
| **首席分析师** | 宏观研判、大盘分析、板块轮动 | 为选股师提供方向 |
| **选股师** | 多维度选股、潜力股推荐 | 依赖首席分析师方向 |
| **盯盘员** | 实时监控、异动告警、成交量分析 | 独立运行 + 触发交易助手 |
| **交易助手** | 买卖信号、仓位管理、止盈止损 | 依赖盯盘员 + 选股师 |
| **研报分析师** | 研报解读、财报分析、行业报告 | 为选股师提供依据 |
| **风控官** | 风险评估、持仓集中度、回撤预警 | 约束所有交易建议 |

---

## 2. Agent 角色设计

### 2.1 首席分析师 Agent

| 属性 | 配置 |
|------|------|
| **名称** | ChiefAnalyst |
| **职责** | 宏观经济分析、大盘研判、板块轮动、市场情绪判断 |
| **模型** | dashscope/qwen3.5-plus |
| **Skills** | tushare-finance, stock-analysis, web-search |
| **触发方式** | 定时（开盘前 8:30）、用户提问、Coordinator 调度 |
| **输出** | 早盘研判报告、板块热度排序、大盘支撑/压力位 |

### 2.2 选股师 Agent

| 属性 | 配置 |
|------|------|
| **名称** | StockPicker |
| **职责** | 多维度选股、基本面筛选、技术面筛选、潜力股推荐 |
| **模型** | dashscope/qwen3.5-plus |
| **Skills** | tushare-finance, stock-analysis, stocks, portfolio-watcher |
| **触发方式** | 定时（早盘 9:00）、用户提问「推荐股票」、策略触发 |
| **输出** | 潜力股列表、8 维度评分、买入理由、风险提示 |

### 2.3 盯盘员 Agent

| 属性 | 配置 |
|------|------|
| **名称** | MarketWatcher |
| **职责** | 实时监控、价格告警、异动检测、成交量分析 |
| **模型** | dashscope/qwen3-coder-plus（轻量快速） |
| **Skills** | tushare-finance, stocks, portfolio-watcher, finance_alert |
| **触发方式** | heartbeat 每 5–15 分钟、价格告警触发、用户订阅 |
| **输出** | 异动推送、价格告警、成交量异常、涨跌榜 |

### 2.4 交易助手 Agent

| 属性 | 配置 |
|------|------|
| **名称** | TradingAssistant |
| **职责** | 买卖信号、仓位管理、止盈止损、执行建议 |
| **模型** | dashscope/qwen3.5-plus |
| **Skills** | portfolio-watcher, stock-analysis, tushare-finance |
| **触发方式** | 盯盘员告警触发、用户提问「该不该卖」、收盘前 14:50 |
| **输出** | 买卖建议、仓位建议、止盈止损位、**明确标注仅供参考** |

### 2.5 研报分析师 Agent

| 属性 | 配置 |
|------|------|
| **名称** | ResearchAnalyst |
| **职责** | 研报解读、财报分析、行业报告、公告摘要 |
| **模型** | dashscope/qwen3.5-plus |
| **Skills** | tushare-finance, web-search, pdf-text-extractor |
| **触发方式** | 用户提问、财报季定时、研报更新触发 |
| **输出** | 研报摘要、财报要点、行业对比、投资逻辑 |

### 2.6 风控官 Agent

| 属性 | 配置 |
|------|------|
| **名称** | RiskOfficer |
| **职责** | 风险评估、持仓集中度、回撤预警、杠杆监控 |
| **模型** | dashscope/qwen3.5-plus |
| **Skills** | portfolio-watcher, tushare-finance |
| **触发方式** | 每日收盘后、持仓变动后、用户提问 |
| **输出** | 风险评分、集中度告警、回撤预警、**免责声明** |

---

## 3. 数据源接入

### 3.1 Tushare Pro（A 股核心）

| 项目 | 说明 |
|------|------|
| **官网** | https://tushare.pro |
| **注册** | 注册即送 100 积分，完善资料再送 20 积分 |
| **Token 获取** | 登录 → 个人中心 → 接口 TOKEN |
| **接口数量** | 220+ 接口，覆盖 A 股/港股/美股/基金/期货/债券 |
| **积分规则** | 积分有 1 年有效期；推荐用户 50 分/人；捐助可增加权限 |

**配置方法：**

```bash
# 环境变量
export TUSHARE_TOKEN="your-tushare-token"

# 或在 ~/.openclaw/.env 中
TUSHARE_TOKEN=your-tushare-token
```

**Python 验证：**

```python
import tushare as ts
ts.set_token('your-tushare-token')
pro = ts.pro_api()
df = pro.daily(ts_code='600519.SH', start_date='20260101', end_date='20260308')
print(df.tail())
```

### 3.2 Yahoo Finance（美股/港股/加密货币）

| 项目 | 说明 |
|------|------|
| **API Key** | 无需 API Key，免费使用 |
| **覆盖** | 美股、港股、加密货币、指数、外汇 |
| **延迟** | 免费数据约 15–20 分钟延迟 |
| **依赖** | yfinance（pip install yfinance） |

**A 股注意**：Yahoo Finance 对 A 股支持有限，建议 A 股用 Tushare/AKShare。

### 3.3 AKShare（A 股备用）

| 项目 | 说明 |
|------|------|
| **安装** | pip install akshare |
| **API Key** | 无需 |
| **数据源** | 东方财富、新浪、腾讯等 |
| **用途** | Tushare 限流或故障时的备用 |

### 3.4 加密货币数据源

| 数据源 | 说明 |
|--------|------|
| **Yahoo Finance** | BTC-USD, ETH-USD, SOL-USD 等，免费 |
| **Tushare** | 部分加密货币相关接口（需积分） |
| **CoinGecko** | 免费 API，可配合 Python 脚本 |

### 3.5 数据源对比表

| 数据源 | 市场 | API Key | 成本 | 实时性 | 推荐场景 |
|--------|------|---------|------|--------|----------|
| **Tushare Pro** | A股/港股/美股/基金 | 需 Token | 免费~付费 | 日线实时 | A 股核心 |
| **Yahoo Finance** | 美股/港股/加密货币 | 无需 | 免费 | 15–20min 延迟 | 美股/加密货币 |
| **AKShare** | A股/基金 | 无需 | 免费 | 实时 | A 股备用 |
| **东方财富/新浪** | A股 | 无需 | 免费 | 实时 | 爬虫/脚本 |

---

## 4. Skills 安装清单

### 4.1 金融 Skills 安装命令

```bash
# 1. Yahoo Finance（美股/港股/加密货币，免费无 Key）
npx playbooks add skill openclaw/skills --skill yahoofinance
# 依赖：pip install yfinance

# 2. Stock Analysis（若 ClawHub 有对应 skill）
# 14 个 Python 脚本：实时行情、板块筛选、潜力股、早盘报告、8 维度评分
# 安装前请用 secure-install 检查安全性
# npx playbooks add skill <author>/<repo> --skill stock-analysis

# 3. Tushare Finance（A 股核心，需 Token）
# 若 OpenClaw 官方/社区有 tushare-finance skill：
# npx playbooks add skill <author>/<repo> --skill tushare-finance
# 或自行编写 SKILL.md + Python 脚本封装 Tushare API

# 4. Portfolio Watcher（持仓跟踪）
# 若存在：npx playbooks add skill <author>/<repo> --skill portfolio-watcher

# 5. 安全检查（安装前必做）
npx playbooks add skill openclaw/skills --skill secure-install
# 使用 secure-install 预检待安装的 skill
```

### 4.2 自建 Tushare Skill 示例

若 ClawHub 无现成 tushare-finance，可在 `~/.openclaw/skills/tushare-finance/` 下自建：

```
~/.openclaw/skills/tushare-finance/
├── SKILL.md
├── run
└── requirements.txt  # tushare
```

**SKILL.md 核心内容：**

```yaml
---
name: tushare-finance
description: A股/港股/美股行情、基本面、财报数据，基于 Tushare Pro API
metadata:
  openclaw:
    requires:
      bins: [python3]
    install:
      - id: python
        kind: pip
        package: tushare
    env:
      TUSHARE_TOKEN: "${TUSHARE_TOKEN}"
---
```

### 4.3 API Key 配置汇总

| Skill/数据源 | 环境变量 | 获取方式 |
|--------------|----------|----------|
| Tushare Pro | TUSHARE_TOKEN | tushare.pro 注册 |
| Yahoo Finance | 无 | 无需 |
| AKShare | 无 | 无需 |
| 阿里百炼 | DASHSCOPE_API_KEY | 阿里云百炼，Coding Plan |
| 飞书 | FEISHU_APP_ID, FEISHU_APP_SECRET | open.feishu.cn 创建应用 |

---

## 5. 自动化工作流

### 5.1 开盘前（8:00–9:15）

| 时间 | 任务 | 执行 Agent | 输出 |
|------|------|------------|------|
| 8:00 | 隔夜外盘复盘 | 首席分析师 | 美股/港股隔夜涨跌摘要 |
| 8:15 | 早盘热点扫描 | 选股师 | 板块异动、资金流入前 10 |
| 8:30 | 早盘研判报告 | 首席分析师 | 大盘预判、支撑压力、操作建议 |
| 9:00 | 潜力股推荐 | 选股师 | 今日关注列表（3–5 只） |
| 9:10 | 推送到飞书群 | — | 早盘报告 + 关注列表 |

### 5.2 盘中（9:30–15:00）

| 时间 | 任务 | 执行 Agent | 输出 |
|------|------|------------|------|
| 每 15 分钟 | 异动检测 | 盯盘员 | 涨跌超阈值、成交量异常 |
| 实时 | 价格告警 | 盯盘员 | 触发用户预设价格时推送 |
| 10:30 | 午盘小结 | 盯盘员 | 上午涨跌榜、异动股 |
| 11:30 | 午间推送 | — | 午盘小结到飞书 |
| 14:30 | 尾盘提醒 | 交易助手 | 持仓股操作建议 |
| 14:50 | 收盘前检查 | 交易助手 | 止盈止损提醒 |

### 5.3 收盘后（15:30–18:00）

| 时间 | 任务 | 执行 Agent | 输出 |
|------|------|------------|------|
| 15:30 | 复盘总结 | 首席分析师 | 今日大盘复盘、板块表现 |
| 16:00 | 持仓分析 | 风控官 + 交易助手 | 持仓盈亏、集中度、风险提示 |
| 16:30 | 次日策略 | 选股师 | 明日关注、操作思路 |
| 17:00 | 推送到飞书群 | — | 复盘报告 + 持仓分析 |

### 5.4 周末

| 任务 | 执行 Agent | 输出 |
|------|------------|------|
| 周报生成 | 首席分析师 | 本周大盘、板块轮动、下周展望 |
| 持仓调整建议 | 风控官 + 选股师 | 调仓建议、风险提示 |
| 研报摘要 | 研报分析师 | 本周重要研报解读 |

### 5.5 Heartbeat 配置

OpenClaw 支持 `heartbeat_interval`，可配置定时任务。在 `config.yaml` 或 `openclaw.json` 对应位置设置：

```yaml
system:
  heartbeat_interval: 15m   # 每 15 分钟执行一次盯盘任务
```

或通过 cron/systemd timer 调用 OpenClaw 执行特定 prompt，实现 8:00、9:00 等固定时间任务。

---

## 6. 告警推送配置

### 6.1 飞书群推送配置

**步骤：**

1. 创建飞书应用：https://open.feishu.cn/app  
2. 获取 App ID、App Secret  
3. 开启机器人能力，添加「接收消息」权限  
4. 配置事件订阅：`im.message.receive_v1`，选择「长连接接收」  
5. 将机器人拉入目标飞书群  

**OpenClaw 配置：**

```bash
openclaw plugins install @openclaw/feishu
openclaw channels add  # 选择 Feishu，输入 App ID 和 App Secret
openclaw gateway restart
```

### 6.2 告警类型和级别

| 级别 | 类型 | 触发条件 | 推送频率 |
|------|------|----------|----------|
| **高** | 价格告警 | 股价触及用户预设阈值 | 立即 |
| **高** | 异动告警 | 单日涨跌幅 >7% 或成交量 >2 倍均量 | 立即 |
| **中** | 止盈止损 | 持仓盈亏触及预设比例 | 立即 |
| **中** | 集中度告警 | 单只持仓占比 >20% | 每日一次 |
| **低** | 早盘/复盘报告 | 定时任务 | 每日 2–3 次 |

### 6.3 推送频率控制

- 同一标的 5 分钟内不重复推送同类型告警  
- 每日单用户告警上限可设（如 20 条）  
- 非交易时段（21:00–8:00）可降低推送频率或静默  

---

## 7. 荐股策略示例

### 7.1 价值投资策略

**筛选条件（Tushare/选股脚本）：**

- PE（市盈率）< 15  
- ROE（净资产收益率）> 15%  
- 近 3 年分红率稳定且 > 3%  
- 负债率 < 60%  
- 市值 > 100 亿（避免小盘妖股）  

**选股师 Prompt 示例：**

```
请根据以下价值投资条件筛选 A 股：
PE<15, ROE>15%, 分红率>3%, 负债率<60%, 市值>100亿。
输出前 5 只股票，并说明推荐理由。每只股票必须附带风险提示。
```

### 7.2 趋势突破策略

**筛选条件：**

- 近 5 日成交量 > 20 日均量 1.5 倍  
- MACD 金叉（DIF 上穿 DEA）  
- 股价突破 20 日均线  
- 所属板块近 5 日涨幅排名前 20%  

**选股师 Prompt 示例：**

```
请筛选满足以下技术面条件的 A 股：
放量突破（量比>1.5）、MACD 金叉、突破 20 日均线、板块强势。
输出 3–5 只，并标注止损位建议。仅供参考，不构成投资建议。
```

### 7.3 热点题材策略

**筛选条件：**

- 所属板块近 3 日涨幅 > 5%  
- 主力资金净流入 > 5000 万  
- 换手率 3%–15%（排除冷门与过度炒作）  
- 有近期研报或公告催化  

**选股师 Prompt 示例：**

```
请根据板块异动和资金流入，筛选当前热点题材股：
板块近3日涨幅>5%、主力净流入>5000万、换手率3–15%。
输出 3–5 只，并说明题材逻辑。注意：热点股波动大，务必控制仓位。
```

---

## 8. 风险控制

### 8.1 投资建议免责声明

**所有 Agent 输出必须包含以下或等效表述：**

> 以上内容仅供参考，不构成任何投资建议。股市有风险，投资需谨慎。请根据自身风险承受能力独立决策。

可在 Agent 的 system prompt 或 output 模板中强制追加。

### 8.2 仓位管理规则

| 规则 | 建议值 |
|------|--------|
| 单只个股仓位上限 | ≤ 20% |
| 单一板块仓位上限 | ≤ 40% |
| 现金保留比例 | ≥ 10% |
| 最大持仓数量 | 5–10 只 |

### 8.3 最大回撤预警

- 当组合回撤超过 10% 时，风控官推送预警  
- 当单只持仓回撤超过 15% 时，建议复核是否止损  
- 回撤计算：相对近期高点的跌幅  

### 8.4 Agent 输出规范

- 交易助手、选股师、研报分析师的所有「建议」类输出，必须带免责声明  
- 不给出具体买卖价格和数量建议，仅提供思路和参考  
- 明确标注「历史表现不代表未来」「过往业绩不预示未来收益」  

---

## 9. 飞书群配置

### 9.1 金融 Agent 群飞书绑定

1. **创建飞书群**：「金融分析 Agent 群」  
2. **添加机器人**：将创建的飞书应用机器人加入该群  
3. **权限**：机器人需有「接收群消息」「发送群消息」权限  
4. **@ 触发**：用户 @ 机器人提问，或配置为接收所有群消息（按需）  

### 9.2 openclaw.json 配置示例

在现有 `openclaw.json` 基础上，增加金融 Agent 群及 Skills 配置：

```json
{
  "agent": {
    "model": "dashscope/qwen3.5-plus",
    "fallbackModels": [
      "dashscope/qwen3-coder-plus",
      "dashscope/kimi-k2.5",
      "deepseek/deepseek-chat"
    ],
    "name": "FinanceTeam",
    "theme": "金融分析 Agent 群 - 分析/选股/盯盘/交易辅助",
    "emoji": "📈"
  },

  "agents": {
    "defaults": {
      "bootstrapMaxChars": 20000,
      "maxContextTokens": 128000,
      "maxResponseTokens": 8192,
      "temperature": 0.5,
      "subagents": {
        "maxSpawnDepth": 2,
        "maxChildrenPerAgent": 6,
        "maxConcurrent": 10
      },
      "sandbox": {
        "mode": "non-main",
        "docker": {
          "env": {
            "DASHSCOPE_API_KEY": "${DASHSCOPE_API_KEY}",
            "TUSHARE_TOKEN": "${TUSHARE_TOKEN}"
          }
        }
      }
    }
  },

  "skills": {
    "load": {
      "extraDirs": ["~/.openclaw/skills"],
      "watch": true,
      "watchDebounceMs": 250
    },
    "entries": {
      "secure-install": { "enabled": true },
      "yahoofinance": {
        "enabled": true,
        "env": {}
      },
      "tushare-finance": {
        "enabled": true,
        "env": {
          "TUSHARE_TOKEN": "${TUSHARE_TOKEN}"
        }
      }
    }
  }
}
```

### 9.3 飞书插件环境变量

```bash
# ~/.openclaw/.env
FEISHU_APP_ID=cli_xxxxxxxx
FEISHU_APP_SECRET=xxxxxxxx
DASHSCOPE_API_KEY=sk-xxxxxxxx
TUSHARE_TOKEN=xxxxxxxx
```

---

## 10. 成本估算

### 10.1 数据源成本

| 数据源 | 月成本 | 说明 |
|--------|--------|------|
| Tushare Pro | ¥0–200 | 免费 120 积分起步，高频需积分/捐助 |
| Yahoo Finance | ¥0 | 免费 |
| AKShare | ¥0 | 免费 |
| **小计** | **¥0–200** | |

### 10.2 LLM 消耗（阿里百炼 Coding Plan）

| 场景 | 日调用量估算 | 月 Token 估算 | 备注 |
|------|--------------|--------------|------|
| 早盘报告 | 3–5 次 | ~50K | 首席+选股 |
| 盘中盯盘 | 20–40 次 | ~200K | 每 15 分钟 |
| 复盘报告 | 2–3 次 | ~30K | 收盘后 |
| 用户提问 | 10–30 次 | ~100K | 不定 |
| **合计** | — | **~400K–500K/月** | 视使用频率浮动 |

**Coding Plan 包月**：若已购买，通常含一定额度，超出部分按量计费，具体以阿里云百炼定价为准。

### 10.3 总月度成本

| 项目 | 金额 |
|------|------|
| 数据源 | ¥0–200 |
| LLM（按量） | ¥50–200（若超出包月额度） |
| 飞书 | ¥0（基础版） |
| 服务器/mini 主机 | 已有则 ¥0 |
| **合计** | **¥50–400/月** |

---

## 11. 部署步骤

### 11.1 从零到运行

**Step 1：环境准备**

```bash
# 确保已安装
# - Node.js 18+
# - Python 3.9+
# - OpenClaw (npm install -g openclaw)
```

**Step 2：安装 OpenClaw 与飞书插件**

```bash
npm install -g openclaw@latest
openclaw plugins install @openclaw/feishu
openclaw channels add  # 选择 Feishu，输入 App ID、App Secret
```

**Step 3：申请 API Key**

- Tushare：https://tushare.pro 注册并获取 Token  
- 阿里百炼：开通 Coding Plan，获取 DASHSCOPE_API_KEY  

**Step 4：配置环境变量**

```bash
mkdir -p ~/.openclaw
cat > ~/.openclaw/.env << 'EOF'
DASHSCOPE_API_KEY=sk-xxx
TUSHARE_TOKEN=xxx
FEISHU_APP_ID=cli_xxx
FEISHU_APP_SECRET=xxx
EOF
```

**Step 5：安装金融 Skills**

```bash
npx playbooks add skill openclaw/skills --skill secure-install
npx playbooks add skill openclaw/skills --skill yahoofinance
pip install yfinance tushare akshare
# 若有 tushare-finance/portfolio-watcher 等 skill，按 4.1 节安装
```

**Step 6：复制/创建 openclaw.json**

```bash
cp /path/to/agent-config/openclaw.json ~/.openclaw/
# 按 9.2 节修改为金融 Agent 群配置
```

**Step 7：启动 OpenClaw**

```bash
openclaw gateway restart
# 或 openclaw start
```

**Step 8：验证**

- 在飞书群 @ 机器人提问：「茅台现在多少钱」  
- 应能返回行情数据（若 yahoofinance 支持 A 股）或通过 Tushare 查询  

**Step 9：配置定时任务（可选）**

使用 cron 或 systemd timer 在 8:00、9:00、15:30 等时间调用 OpenClaw 执行对应 prompt，实现早盘、复盘自动化。

### 11.2 检查清单

- [ ] OpenClaw 已安装且版本 ≥ 2026.2.26  
- [ ] 飞书应用已创建，机器人已加入目标群  
- [ ] TUSHARE_TOKEN、DASHSCOPE_API_KEY 已配置  
- [ ] yahoofinance skill 已安装，yfinance 可正常导入  
- [ ] 所有输出均包含免责声明  
- [ ] 告警推送频率已做限流，避免骚扰  

---

## 附录 A：A 股代码格式速查

| 市场 | 格式 | 示例 |
|------|------|------|
| 上海主板 | 6 位数字，Tushare 后缀 .SH | 600519.SH |
| 深圳主板 | 6 位数字，Tushare 后缀 .SZ | 000001.SZ |
| 创业板 | 3 开头，.SZ | 300750.SZ |
| 科创板 | 6 开头，.SH | 688981.SH |
| 港股 | 5 位+.HK | 00700.HK |
| 美股 | 字母 | AAPL, TSLA |

---

## 附录 B：参考链接

- OpenClaw 飞书对接：https://docs.openclaw.ai/zh-CN/channels/feishu  
- Tushare Pro 文档：https://tushare.pro/document/2  
- Yahoo Finance skill：https://playbooks.com/skills/openclaw/skills/yahoofinance  
- 阿里百炼：https://bailian.console.aliyun.com  

---

*文档版本: 1.0.0 | 最后更新: 2026-03-08*  
*本方案基于 OpenClaw 生态及公开信息整理，具体 skill 可用性以 ClawHub/Playbooks 为准。*
