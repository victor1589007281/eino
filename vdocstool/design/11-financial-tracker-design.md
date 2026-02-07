# 金融资产追踪工具设计文档

## 1. 概述

### 1.1 目标
开发一个面向 AI Agent 的金融资产追踪 MCP 工具，支持股票、基金、黄金、债券等多种资产类型的实时行情查询、历史数据分析、组合管理、智能提醒和财经情报搜集。

### 1.2 核心能力
- **多资产支持**：股票、基金、ETF、黄金、债券、期货、外汇、加密货币
- **多市场覆盖**：A股、港股、美股、全球主要市场
- **实时数据**：行情推送、涨跌提醒
- **分析能力**：技术指标、趋势分析、对比分析
- **组合管理**：自选股、投资组合、收益计算
- **财经情报**：财经新闻、公告、研报、舆情监控

### 1.3 设计原则
- **国内可用**：优先选择国内可直接访问的免费API
- **多源互备**：同一数据多个来源，自动故障转移
- **高效存储**：时序数据库优化历史数据查询
- **实时缓存**：Redis缓存热点行情数据

## 2. 系统架构

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           Financial Tracker MCP Tool                             │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │
│  │  Quote       │  │  Portfolio   │  │  Analysis    │  │   Alert      │        │
│  │  Service     │  │  Manager     │  │  Engine      │  │   System     │        │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘        │
│         │                 │                 │                 │                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                          │
│  │   News       │  │  Research    │  │  Sentiment   │                          │
│  │   Aggregator │  │  Collector   │  │  Monitor     │                          │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘                          │
│         │                 │                 │                                   │
│  ┌──────┴─────────────────┴─────────────────┴─────────────────────────────┐    │
│  │                     Core Service Layer                                  │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │    │
│  │  │ DataFetcher │  │ SourceRouter│  │ RateLimiter │  │HealthChecker│   │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │    │
│  └────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│  ┌─────────────────────────────────┴─────────────────────────────────────┐    │
│  │                          Storage Layer                                 │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                   │    │
│  │  │    Redis    │  │  PostgreSQL │  │  TimescaleDB│                   │    │
│  │  │  (缓存层)   │  │  (关系数据) │  │  (时序数据) │                   │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘                   │    │
│  └────────────────────────────────────────────────────────────────────────┘    │
│                                    │                                            │
│  ┌─────────────────────────────────┴─────────────────────────────────────┐    │
│  │                     Data Source Adapters (自动路由+故障转移)           │    │
│  │  ┌─────────────────────────────────────────────────────────────────┐ │    │
│  │  │  A股数据源组 (互为备份)                                          │ │    │
│  │  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐              │ │    │
│  │  │  │ 东方财富 │ │ 新浪财经 │ │ 腾讯财经 │ │ 网易财经 │              │ │    │
│  │  │  │ Primary │ │ Backup1 │ │ Backup2 │ │ Backup3 │              │ │    │
│  │  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘              │ │    │
│  │  └─────────────────────────────────────────────────────────────────┘ │    │
│  │  ┌─────────────────────────────────────────────────────────────────┐ │    │
│  │  │  基金数据源组                                                    │ │    │
│  │  │  ┌─────────┐ ┌─────────┐ ┌─────────┐                          │ │    │
│  │  │  │ 天天基金 │ │ 东方财富 │ │ 好买基金 │                          │ │    │
│  │  │  └─────────┘ └─────────┘ └─────────┘                          │ │    │
│  │  └─────────────────────────────────────────────────────────────────┘ │    │
│  │  ┌─────────────────────────────────────────────────────────────────┐ │    │
│  │  │  港美股/全球数据源组                                             │ │    │
│  │  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐              │ │    │
│  │  │  │ 新浪港美 │ │ 东方财富 │ │ 雪球    │ │ 富途(备) │              │ │    │
│  │  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘              │ │    │
│  │  └─────────────────────────────────────────────────────────────────┘ │    │
│  │  ┌─────────────────────────────────────────────────────────────────┐ │    │
│  │  │  财经新闻源组                                                    │ │    │
│  │  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ │ │    │
│  │  │  │ 东方财富 │ │ 新浪财经 │ │ 同花顺  │ │ 财联社  │ │ 金十数据 │ │ │    │
│  │  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ │ │    │
│  │  └─────────────────────────────────────────────────────────────────┘ │    │
│  └────────────────────────────────────────────────────────────────────────┘    │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## 3. 数据库选型

### 3.1 存储架构总览

| 存储层 | 技术选型 | 用途 | 数据特点 |
|--------|----------|------|----------|
| **缓存层** | Redis | 实时行情缓存 | 高频读写、TTL过期 |
| **关系层** | PostgreSQL | 业务数据 | 结构化、事务性 |
| **时序层** | TimescaleDB | 历史行情 | 时间序列、聚合查询 |

### 3.2 为什么选择 TimescaleDB

**时序数据库对比分析：**

| 特性 | TimescaleDB | InfluxDB | TDengine | ClickHouse |
|------|-------------|----------|----------|------------|
| **SQL兼容** | ✅ 完全兼容PostgreSQL | ❌ InfluxQL/Flux | ⚠️ 类SQL | ✅ SQL |
| **与PG集成** | ✅ 原生扩展 | ❌ 独立系统 | ❌ 独立系统 | ❌ 独立系统 |
| **学习成本** | 低（已有PG经验） | 中 | 中 | 中 |
| **聚合查询** | ✅ 连续聚合 | ✅ 好 | ✅ 好 | ✅ 极好 |
| **压缩率** | ✅ 90%+ | ✅ 好 | ✅ 好 | ✅ 极好 |
| **实时写入** | ✅ 好 | ✅ 极好 | ✅ 极好 | ⚠️ 批量优 |
| **运维复杂度** | 低（PG生态） | 中 | 中 | 高 |
| **开源协议** | Apache 2.0 | MIT | AGPL | Apache 2.0 |

**选择 TimescaleDB 的原因：**

1. **无缝集成 PostgreSQL**
   - 作为 PG 扩展，共享同一实例，简化运维
   - 关系数据和时序数据可以 JOIN 查询
   - 复用 PG 的备份、复制、监控工具

2. **金融数据分析优势**
   ```sql
   -- 连续聚合：自动计算日/周/月K线
   CREATE MATERIALIZED VIEW daily_kline
   WITH (timescaledb.continuous) AS
   SELECT 
       symbol,
       time_bucket('1 day', timestamp) AS bucket,
       first(open, timestamp) AS open,
       max(high) AS high,
       min(low) AS low,
       last(close, timestamp) AS close,
       sum(volume) AS volume
   FROM tick_data
   GROUP BY symbol, bucket;

   -- 高效时间范围查询
   SELECT * FROM quotes 
   WHERE symbol = '600519' 
   AND timestamp > NOW() - INTERVAL '30 days';

   -- 技术指标计算（移动平均）
   SELECT symbol, timestamp, close,
       AVG(close) OVER (ORDER BY timestamp ROWS 4 PRECEDING) AS ma5,
       AVG(close) OVER (ORDER BY timestamp ROWS 9 PRECEDING) AS ma10,
       AVG(close) OVER (ORDER BY timestamp ROWS 19 PRECEDING) AS ma20
   FROM daily_kline WHERE symbol = '600519';
   ```

3. **自动数据管理**
   - 自动分区（按时间分块）
   - 自动压缩历史数据
   - 自动数据保留策略

### 3.3 数据库 Schema 设计

```sql
-- ============================================================
-- PostgreSQL 基础表（业务数据）
-- ============================================================

-- 资产信息表
CREATE TABLE assets (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    name VARCHAR(100) NOT NULL,
    asset_type VARCHAR(20) NOT NULL,  -- stock/fund/bond/gold/crypto
    market VARCHAR(20) NOT NULL,       -- cn_sh/cn_sz/hk/us
    exchange VARCHAR(50),
    industry VARCHAR(100),
    list_date DATE,
    status VARCHAR(20) DEFAULT 'active',
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(symbol, market)
);

CREATE INDEX idx_assets_type_market ON assets(asset_type, market);
CREATE INDEX idx_assets_name ON assets USING gin(name gin_trgm_ops);

-- 投资组合表
CREATE TABLE portfolios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    user_id VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 持仓表
CREATE TABLE holdings (
    id SERIAL PRIMARY KEY,
    portfolio_id UUID REFERENCES portfolios(id) ON DELETE CASCADE,
    symbol VARCHAR(20) NOT NULL,
    market VARCHAR(20) NOT NULL,
    quantity DECIMAL(18, 4) NOT NULL,
    avg_cost DECIMAL(18, 4) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(portfolio_id, symbol, market)
);

-- 交易记录表
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    portfolio_id UUID REFERENCES portfolios(id),
    symbol VARCHAR(20) NOT NULL,
    market VARCHAR(20) NOT NULL,
    type VARCHAR(10) NOT NULL,  -- buy/sell
    quantity DECIMAL(18, 4) NOT NULL,
    price DECIMAL(18, 4) NOT NULL,
    amount DECIMAL(18, 4) NOT NULL,
    fee DECIMAL(18, 4) DEFAULT 0,
    executed_at TIMESTAMP NOT NULL,
    note TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_transactions_portfolio ON transactions(portfolio_id, executed_at);

-- 价格提醒表
CREATE TABLE price_alerts (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    market VARCHAR(20) NOT NULL,
    condition VARCHAR(30) NOT NULL,  -- price_above/price_below/change_above...
    value DECIMAL(18, 4) NOT NULL,
    status VARCHAR(20) DEFAULT 'active',  -- active/triggered/disabled
    triggered_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_alerts_active ON price_alerts(status) WHERE status = 'active';

-- 自选股表
CREATE TABLE watchlist (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(100),
    symbol VARCHAR(20) NOT NULL,
    market VARCHAR(20) NOT NULL,
    group_name VARCHAR(50) DEFAULT 'default',
    sort_order INT DEFAULT 0,
    added_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, symbol, market)
);

-- ============================================================
-- TimescaleDB 时序表（行情数据）
-- ============================================================

-- 启用 TimescaleDB 扩展
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- 分钟级行情数据
CREATE TABLE quote_ticks (
    timestamp TIMESTAMPTZ NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    market VARCHAR(20) NOT NULL,
    current_price DECIMAL(18, 4),
    open DECIMAL(18, 4),
    high DECIMAL(18, 4),
    low DECIMAL(18, 4),
    pre_close DECIMAL(18, 4),
    volume BIGINT,
    amount DECIMAL(18, 2),
    turnover DECIMAL(8, 4),
    pe DECIMAL(10, 2),
    pb DECIMAL(10, 2),
    market_cap DECIMAL(18, 2)
);

-- 转换为超表（按天分区）
SELECT create_hypertable('quote_ticks', 'timestamp', chunk_time_interval => INTERVAL '1 day');

-- 创建复合索引
CREATE INDEX idx_quote_ticks_symbol ON quote_ticks (symbol, market, timestamp DESC);

-- K线数据表
CREATE TABLE kline_data (
    timestamp TIMESTAMPTZ NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    market VARCHAR(20) NOT NULL,
    period VARCHAR(10) NOT NULL,  -- 1m/5m/15m/30m/1h/1d/1w/1M
    open DECIMAL(18, 4),
    high DECIMAL(18, 4),
    low DECIMAL(18, 4),
    close DECIMAL(18, 4),
    volume BIGINT,
    amount DECIMAL(18, 2)
);

SELECT create_hypertable('kline_data', 'timestamp', chunk_time_interval => INTERVAL '1 month');
CREATE INDEX idx_kline_symbol_period ON kline_data (symbol, market, period, timestamp DESC);

-- 连续聚合：自动生成日K线
CREATE MATERIALIZED VIEW daily_kline_agg
WITH (timescaledb.continuous) AS
SELECT 
    time_bucket('1 day', timestamp) AS bucket,
    symbol,
    market,
    first(open, timestamp) AS open,
    max(high) AS high,
    min(low) AS low,
    last(close, timestamp) AS close,
    sum(volume) AS volume,
    sum(amount) AS amount
FROM quote_ticks
GROUP BY symbol, market, bucket
WITH NO DATA;

-- 自动刷新策略
SELECT add_continuous_aggregate_policy('daily_kline_agg',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour');

-- 数据保留策略：保留2年数据
SELECT add_retention_policy('quote_ticks', INTERVAL '2 years');

-- 压缩策略：7天后压缩
SELECT add_compression_policy('quote_ticks', INTERVAL '7 days');

-- ============================================================
-- 财经新闻表
-- ============================================================

CREATE TABLE news_articles (
    id SERIAL PRIMARY KEY,
    source VARCHAR(50) NOT NULL,       -- eastmoney/sina/cls/jin10
    external_id VARCHAR(100),
    title TEXT NOT NULL,
    summary TEXT,
    content TEXT,
    url VARCHAR(500),
    publish_time TIMESTAMPTZ NOT NULL,
    category VARCHAR(50),              -- 宏观/行业/公司/市场
    tags TEXT[],                       -- 标签数组
    symbols VARCHAR(20)[],             -- 关联股票代码
    sentiment DECIMAL(3, 2),           -- 情感分数 -1~1
    importance INT DEFAULT 0,          -- 重要程度 0-5
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(source, external_id)
);

CREATE INDEX idx_news_time ON news_articles(publish_time DESC);
CREATE INDEX idx_news_symbols ON news_articles USING gin(symbols);
CREATE INDEX idx_news_tags ON news_articles USING gin(tags);

-- 公司公告表
CREATE TABLE announcements (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    market VARCHAR(20) NOT NULL,
    title TEXT NOT NULL,
    type VARCHAR(50),                  -- 定期报告/临时公告/股权变动...
    publish_time TIMESTAMPTZ NOT NULL,
    url VARCHAR(500),
    importance INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_announcements_symbol ON announcements(symbol, market, publish_time DESC);
```

### 3.4 Redis 缓存设计

```go
// Redis Key 设计
const (
    // 实时行情缓存 (TTL: 3-10秒)
    KeyQuote       = "quote:{market}:{symbol}"           // 单个行情
    KeyQuoteBatch  = "quote:batch:{market}"              // 批量行情 Hash
    
    // 行情快照 (TTL: 1分钟)
    KeySnapshot    = "snapshot:{market}:{date}"          // 全市场快照
    
    // 搜索缓存 (TTL: 1小时)
    KeySearch      = "search:{keyword}"
    
    // 热门榜单 (TTL: 30秒)
    KeyTopGainers  = "rank:gainers:{market}"
    KeyTopLosers   = "rank:losers:{market}"
    KeyTopVolume   = "rank:volume:{market}"
    
    // 新闻缓存 (TTL: 5分钟)
    KeyNewsLatest  = "news:latest:{category}"
    KeyNewsSymbol  = "news:symbol:{symbol}"
    
    // 数据源健康状态
    KeySourceHealth = "source:health:{source_name}"
    
    // 频率限制
    KeyRateLimit   = "ratelimit:{source}:{window}"
)

// 缓存 TTL 配置
var CacheTTL = map[string]time.Duration{
    "quote:stock":    3 * time.Second,   // 股票行情
    "quote:fund":     5 * time.Minute,   // 基金净值
    "quote:crypto":   5 * time.Second,   // 加密货币（24h交易）
    "quote:gold":     10 * time.Second,  // 黄金
    "quote:forex":    5 * time.Second,   // 外汇
    "news":           5 * time.Minute,   // 新闻
    "search":         1 * time.Hour,     // 搜索
    "rank":           30 * time.Second,  // 排行榜
}
```

## 4. 数据源详细配置（中国大陆可用+免费）

### 4.1 数据源矩阵

| 数据源 | A股 | 港股 | 美股 | 基金 | 黄金 | 外汇 | 加密 | 新闻 | 免费 | 限频 |
|--------|-----|------|------|------|------|------|------|------|------|------|
| **东方财富** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | 3/s |
| **新浪财经** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | 5/s |
| **腾讯财经** | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | 5/s |
| **网易财经** | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | 3/s |
| **天天基金** | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ | 5/s |
| **同花顺** | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ | 2/s |
| **雪球** | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | 1/s |
| **财联社** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | 2/s |
| **金十数据** | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ | ✅ | 2/s |
| **CoinGecko** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ✅ | 10/m |

### 4.2 数据源优先级配置

```go
// SourcePriority 数据源优先级配置
var SourcePriority = map[AssetType]map[Market][]string{
    AssetTypeStock: {
        MarketCNSH: {"eastmoney", "sina", "tencent", "netease"},
        MarketCNSZ: {"eastmoney", "sina", "tencent", "netease"},
        MarketHK:   {"sina", "eastmoney", "xueqiu", "tencent"},
        MarketUS:   {"sina", "eastmoney", "xueqiu", "tencent"},
    },
    AssetTypeFund: {
        MarketCN: {"tiantian", "eastmoney", "xueqiu"},
    },
    AssetTypeGold: {
        MarketGlobal: {"jin10", "eastmoney", "sina"},
    },
    AssetTypeForex: {
        MarketGlobal: {"jin10", "sina", "eastmoney"},
    },
    AssetTypeCrypto: {
        MarketGlobal: {"coingecko"},  // 需要代理或国内镜像
    },
}

// NewsSourcePriority 新闻源优先级
var NewsSourcePriority = []string{
    "cls",        // 财联社 - 最快最全
    "jin10",      // 金十数据 - 全球宏观
    "eastmoney",  // 东方财富 - 综合
    "sina",       // 新浪财经 - 综合
    "10jqka",     // 同花顺 - 综合
}
```

### 4.3 数据源 API 详情

#### 4.3.1 东方财富 API

```go
// EastMoney API 端点
const (
    // 实时行情
    EMQuoteAPI = "https://push2.eastmoney.com/api/qt/stock/get"
    // 参数: secid=1.600519 (1=沪市, 0=深市)
    // fields: f43(现价),f44(最高),f45(最低),f46(开盘),f47(成交量),f48(成交额)...
    
    // K线数据
    EMKLineAPI = "https://push2his.eastmoney.com/api/qt/stock/kline/get"
    // 参数: secid, klt(周期), fqt(复权), lmt(数量)
    
    // 搜索
    EMSearchAPI = "https://searchadapter.eastmoney.com/api/suggest/get"
    
    // 板块行情
    EMSectorAPI = "https://push2.eastmoney.com/api/qt/clist/get"
    
    // 新闻资讯
    EMNewsAPI = "https://np-listapi.eastmoney.com/comm/web/getNewsByColumns"
    
    // 公司公告
    EMAnnouncementAPI = "https://np-anotice-stock.eastmoney.com/api/security/ann"
    
    // 基金数据
    EMFundAPI = "https://fundgz.1234567.com.cn/js/{code}.js"
)

// EastMoneySource 东方财富数据源
type EastMoneySource struct {
    client      *http.Client
    rateLimiter *rate.Limiter
    health      *SourceHealth
}

func (s *EastMoneySource) GetQuote(ctx context.Context, symbol string) (*Quote, error) {
    // 转换代码格式: 600519 -> 1.600519
    secid := s.convertSecID(symbol)
    
    url := fmt.Sprintf("%s?secid=%s&fields=f43,f44,f45,f46,f47,f48,f50,f51,f52,f55,f57,f58,f60,f116,f117", 
        EMQuoteAPI, secid)
    
    // ... 请求和解析
}
```

#### 4.3.2 新浪财经 API

```go
// Sina API 端点
const (
    // 实时行情 (最稳定，支持批量)
    SinaQuoteAPI = "https://hq.sinajs.cn/list="
    // 参数: sh600519,sz000001,hk00700,gb_aapl (多个逗号分隔)
    
    // K线数据
    SinaKLineAPI = "https://quotes.sina.cn/cn/api/json_v2.php/CN_MarketDataService.getKLineData"
    
    // 港美股
    SinaUSHKAPI = "https://stock.finance.sina.com.cn/usstock/api/json_v2.php"
    
    // 新闻
    SinaNewsAPI = "https://feed.mix.sina.com.cn/api/roll/get"
)

// SinaSource 新浪数据源
type SinaSource struct {
    client      *http.Client
    rateLimiter *rate.Limiter
    health      *SourceHealth
}

// 批量获取行情（新浪优势）
func (s *SinaSource) GetQuotes(ctx context.Context, symbols []string) ([]*Quote, error) {
    // 新浪支持一次请求多个，效率高
    codes := s.convertCodes(symbols)  // sh600519,sz000001
    url := SinaQuoteAPI + strings.Join(codes, ",")
    
    // ... 解析特殊格式响应
}
```

#### 4.3.3 天天基金 API

```go
// TianTian API
const (
    // 基金实时估值
    TTFundGzAPI = "https://fundgz.1234567.com.cn/js/{code}.js"
    
    // 基金详情
    TTFundDetailAPI = "https://fundmobapi.eastmoney.com/FundMApi/FundBaseInfo.ashx"
    
    // 基金净值历史
    TTFundNavAPI = "https://api.fund.eastmoney.com/f10/lsjz"
    
    // 基金排行
    TTFundRankAPI = "https://fundmobapi.eastmoney.com/FundMApi/FundRankNewList.ashx"
)
```

#### 4.3.4 财联社 API

```go
// CLS (财联社) API - 实时财经新闻
const (
    // 电报流（最快）
    CLSTelegraphAPI = "https://www.cls.cn/nodeapi/telegraphs"
    
    // 深度文章
    CLSArticleAPI = "https://www.cls.cn/nodeapi/articles"
    
    // 快讯搜索
    CLSSearchAPI = "https://www.cls.cn/nodeapi/search"
)

// CLSNews 财联社新闻
type CLSNews struct {
    ID         int64     `json:"id"`
    Content    string    `json:"content"`
    Title      string    `json:"title"`
    Brief      string    `json:"brief"`
    CTime      int64     `json:"ctime"`       // 时间戳
    Modified   int64     `json:"modified"`
    Level      string    `json:"level"`       // 重要程度
    Subjects   []Subject `json:"subjects"`    // 关联主题
}
```

#### 4.3.5 金十数据 API

```go
// Jin10 API - 全球宏观/黄金/外汇
const (
    // 实时快讯
    Jin10FlashAPI = "https://flash-api.jin10.com/get_flash_list"
    
    // 日历数据
    Jin10CalendarAPI = "https://cdn-rili.jin10.com/web_data/{date}/economics.json"
    
    // 黄金行情
    Jin10GoldAPI = "https://hq.jin10.com/real/quotation"
)
```

## 5. 智能路由系统

### 5.1 路由器设计

```go
// SourceRouter 数据源路由器
type SourceRouter struct {
    sources       map[string]DataSource
    healthChecker *HealthChecker
    rateLimiters  map[string]*rate.Limiter
    metrics       *RouterMetrics
    mu            sync.RWMutex
}

// RouteConfig 路由配置
type RouteConfig struct {
    PrimarySource   string            // 主数据源
    BackupSources   []string          // 备份数据源列表
    FailoverDelay   time.Duration     // 故障转移延迟
    MaxRetries      int               // 最大重试次数
    CircuitBreaker  *CircuitBreakerConfig
}

// SelectSource 选择数据源（带故障转移）
func (r *SourceRouter) SelectSource(assetType AssetType, market Market) (DataSource, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    // 获取该资产类型和市场的数据源优先级
    sources := SourcePriority[assetType][market]
    
    for _, sourceName := range sources {
        source := r.sources[sourceName]
        
        // 检查健康状态
        if !r.healthChecker.IsHealthy(sourceName) {
            continue
        }
        
        // 检查频率限制
        if !r.rateLimiters[sourceName].Allow() {
            continue
        }
        
        return source, nil
    }
    
    return nil, ErrNoAvailableSource
}

// ExecuteWithFallback 带故障转移执行
func (r *SourceRouter) ExecuteWithFallback(ctx context.Context, 
    assetType AssetType, market Market, 
    fn func(DataSource) (interface{}, error)) (interface{}, error) {
    
    sources := SourcePriority[assetType][market]
    var lastErr error
    
    for i, sourceName := range sources {
        source := r.sources[sourceName]
        
        // 检查健康状态
        if !r.healthChecker.IsHealthy(sourceName) {
            r.metrics.RecordSkip(sourceName, "unhealthy")
            continue
        }
        
        // 等待频率限制
        if err := r.rateLimiters[sourceName].Wait(ctx); err != nil {
            continue
        }
        
        // 执行请求
        startTime := time.Now()
        result, err := fn(source)
        latency := time.Since(startTime)
        
        if err == nil {
            r.metrics.RecordSuccess(sourceName, latency)
            r.healthChecker.RecordSuccess(sourceName)
            return result, nil
        }
        
        // 记录失败
        lastErr = err
        r.metrics.RecordFailure(sourceName, err)
        r.healthChecker.RecordFailure(sourceName)
        
        log.Printf("[Router] Source %s failed (attempt %d/%d): %v", 
            sourceName, i+1, len(sources), err)
    }
    
    return nil, fmt.Errorf("all sources failed: %w", lastErr)
}
```

### 5.2 健康检查器

```go
// HealthChecker 健康检查器
type HealthChecker struct {
    states    map[string]*SourceHealth
    redis     *redis.Client
    checkFunc map[string]func(context.Context) error
    mu        sync.RWMutex
}

// SourceHealth 数据源健康状态
type SourceHealth struct {
    Name            string
    Status          HealthStatus  // healthy/degraded/unhealthy
    LastCheck       time.Time
    LastSuccess     time.Time
    LastFailure     time.Time
    ConsecutiveFails int
    SuccessRate     float64       // 最近100次成功率
    AvgLatency      time.Duration
    CircuitState    CircuitState  // closed/open/half-open
}

type HealthStatus string
const (
    HealthStatusHealthy   HealthStatus = "healthy"
    HealthStatusDegraded  HealthStatus = "degraded"
    HealthStatusUnhealthy HealthStatus = "unhealthy"
)

type CircuitState string
const (
    CircuitClosed   CircuitState = "closed"    // 正常
    CircuitOpen     CircuitState = "open"      // 熔断
    CircuitHalfOpen CircuitState = "half-open" // 半开（试探）
)

// 健康检查配置
var HealthCheckConfig = struct {
    CheckInterval       time.Duration
    UnhealthyThreshold  int     // 连续失败多少次标记为不健康
    DegradedThreshold   float64 // 成功率低于多少标记为降级
    RecoveryInterval    time.Duration // 熔断恢复间隔
}{
    CheckInterval:      30 * time.Second,
    UnhealthyThreshold: 5,
    DegradedThreshold:  0.8,
    RecoveryInterval:   60 * time.Second,
}

// IsHealthy 检查是否健康（考虑熔断状态）
func (h *HealthChecker) IsHealthy(sourceName string) bool {
    h.mu.RLock()
    state, ok := h.states[sourceName]
    h.mu.RUnlock()
    
    if !ok {
        return true // 未知源默认健康
    }
    
    // 熔断状态检查
    switch state.CircuitState {
    case CircuitOpen:
        // 检查是否可以尝试恢复
        if time.Since(state.LastFailure) > HealthCheckConfig.RecoveryInterval {
            h.SetCircuitState(sourceName, CircuitHalfOpen)
            return true // 允许试探请求
        }
        return false
    case CircuitHalfOpen:
        return true // 允许试探
    }
    
    return state.Status != HealthStatusUnhealthy
}
```

### 5.3 频率限制器

```go
// RateLimiterConfig 频率限制配置
var RateLimiterConfig = map[string]rate.Limit{
    "eastmoney": rate.Every(time.Second / 3),   // 3次/秒
    "sina":      rate.Every(time.Second / 5),   // 5次/秒
    "tencent":   rate.Every(time.Second / 5),   // 5次/秒
    "netease":   rate.Every(time.Second / 3),   // 3次/秒
    "tiantian":  rate.Every(time.Second / 5),   // 5次/秒
    "xueqiu":    rate.Every(time.Second),       // 1次/秒
    "10jqka":    rate.Every(time.Second / 2),   // 2次/秒
    "cls":       rate.Every(time.Second / 2),   // 2次/秒
    "jin10":     rate.Every(time.Second / 2),   // 2次/秒
    "coingecko": rate.Every(time.Minute / 10),  // 10次/分钟
}
```

## 6. MCP 工具定义

### 6.1 行情查询工具

```json
{
    "name": "finance_quote",
    "description": "查询股票、基金、黄金、债券等金融资产的实时行情",
    "inputSchema": {
        "type": "object",
        "properties": {
            "symbol": {
                "type": "string",
                "description": "资产代码，如 600519(贵州茅台), 005827(易方达蓝筹), AAPL(苹果), BTC(比特币)"
            },
            "asset_type": {
                "type": "string",
                "enum": ["stock", "fund", "etf", "gold", "bond", "futures", "forex", "crypto", "index"],
                "description": "资产类型，可选，系统会自动识别"
            },
            "market": {
                "type": "string",
                "enum": ["cn_sh", "cn_sz", "hk", "us", "global"],
                "description": "市场，可选，系统会自动识别"
            }
        },
        "required": ["symbol"]
    }
}
```

### 6.2 财经新闻工具

```json
{
    "name": "finance_news",
    "description": "获取财经新闻、快讯、公告等情报信息",
    "inputSchema": {
        "type": "object",
        "properties": {
            "action": {
                "type": "string",
                "enum": ["latest", "search", "symbol", "flash"],
                "description": "操作类型：latest(最新新闻), search(搜索), symbol(个股新闻), flash(实时快讯)"
            },
            "keyword": {
                "type": "string",
                "description": "搜索关键词 (search时必填)"
            },
            "symbol": {
                "type": "string",
                "description": "股票代码 (symbol时必填)"
            },
            "category": {
                "type": "string",
                "enum": ["all", "macro", "industry", "company", "market", "global"],
                "description": "新闻分类：宏观/行业/公司/市场/国际"
            },
            "sources": {
                "type": "array",
                "items": {"type": "string"},
                "description": "指定新闻来源，如 ['cls', 'jin10']"
            },
            "limit": {
                "type": "integer",
                "default": 20,
                "maximum": 100
            },
            "since": {
                "type": "string",
                "description": "起始时间，如 '1h'(1小时内), '1d'(1天内), '2024-01-01'"
            }
        },
        "required": ["action"]
    }
}
```

### 6.3 技术分析工具

```json
{
    "name": "finance_analysis",
    "description": "技术指标分析：计算MA、MACD、RSI、KDJ等指标，生成技术分析报告",
    "inputSchema": {
        "type": "object",
        "properties": {
            "symbol": {
                "type": "string",
                "description": "资产代码"
            },
            "indicators": {
                "type": "array",
                "items": {
                    "type": "string",
                    "enum": ["ma", "ema", "macd", "rsi", "kdj", "boll", "wr", "cci", "atr", "obv", "vol"]
                },
                "description": "技术指标列表"
            },
            "period": {
                "type": "string",
                "enum": ["1d", "1w", "1M"],
                "default": "1d",
                "description": "K线周期"
            },
            "params": {
                "type": "object",
                "description": "指标参数"
            },
            "generate_report": {
                "type": "boolean",
                "default": true,
                "description": "是否生成综合分析报告"
            }
        },
        "required": ["symbol"]
    }
}
```

### 6.4 市场概览工具

```json
{
    "name": "finance_market",
    "description": "获取市场概览：大盘指数、板块热度、涨跌统计、资金流向",
    "inputSchema": {
        "type": "object",
        "properties": {
            "market": {
                "type": "string",
                "enum": ["cn", "hk", "us"],
                "default": "cn"
            },
            "include": {
                "type": "array",
                "items": {
                    "type": "string",
                    "enum": ["indices", "sectors", "top_gainers", "top_losers", "top_volume", "money_flow", "hot_concepts"]
                },
                "description": "包含内容"
            },
            "limit": {
                "type": "integer",
                "default": 10
            }
        },
        "required": ["market"]
    }
}
```

### 6.5 投资组合工具

```json
{
    "name": "finance_portfolio",
    "description": "投资组合管理：创建组合、添加持仓、计算收益、分析配置",
    "inputSchema": {
        "type": "object",
        "properties": {
            "action": {
                "type": "string",
                "enum": ["create", "list", "get", "add_holding", "update_holding", "remove_holding", "summary", "delete"],
                "description": "操作类型"
            },
            "portfolio_id": {
                "type": "string",
                "description": "组合ID"
            },
            "name": {
                "type": "string",
                "description": "组合名称"
            },
            "holding": {
                "type": "object",
                "properties": {
                    "symbol": {"type": "string"},
                    "market": {"type": "string"},
                    "quantity": {"type": "number"},
                    "cost": {"type": "number"}
                }
            }
        },
        "required": ["action"]
    }
}
```

### 6.6 价格提醒工具

```json
{
    "name": "finance_alert",
    "description": "设置价格提醒：当价格、涨跌幅、技术指标达到条件时触发",
    "inputSchema": {
        "type": "object",
        "properties": {
            "action": {
                "type": "string",
                "enum": ["create", "list", "delete", "check"],
                "description": "操作类型"
            },
            "symbol": {
                "type": "string"
            },
            "condition": {
                "type": "string",
                "enum": [
                    "price_above", "price_below",
                    "change_above", "change_below",
                    "volume_above",
                    "ma_cross_up", "ma_cross_down",
                    "rsi_overbought", "rsi_oversold",
                    "macd_golden", "macd_death"
                ]
            },
            "value": {
                "type": "number"
            },
            "alert_id": {
                "type": "string"
            }
        },
        "required": ["action"]
    }
}
```

### 6.7 历史数据工具

```json
{
    "name": "finance_history",
    "description": "查询历史K线数据，支持多种周期和时间范围",
    "inputSchema": {
        "type": "object",
        "properties": {
            "symbol": {
                "type": "string"
            },
            "period": {
                "type": "string",
                "enum": ["1m", "5m", "15m", "30m", "1h", "1d", "1w", "1M"],
                "default": "1d"
            },
            "count": {
                "type": "integer",
                "default": 30,
                "maximum": 500
            },
            "start_date": {
                "type": "string",
                "format": "date"
            },
            "end_date": {
                "type": "string",
                "format": "date"
            },
            "adjust": {
                "type": "string",
                "enum": ["none", "forward", "backward"],
                "default": "forward",
                "description": "复权方式"
            }
        },
        "required": ["symbol"]
    }
}
```

### 6.8 资产搜索工具

```json
{
    "name": "finance_search",
    "description": "搜索股票、基金、债券等金融资产",
    "inputSchema": {
        "type": "object",
        "properties": {
            "keyword": {
                "type": "string",
                "description": "搜索关键词（代码、名称、拼音首字母）"
            },
            "asset_types": {
                "type": "array",
                "items": {"type": "string"}
            },
            "markets": {
                "type": "array",
                "items": {"type": "string"}
            },
            "limit": {
                "type": "integer",
                "default": 10
            }
        },
        "required": ["keyword"]
    }
}
```

## 7. 项目结构

```
tools/finance/
├── finance.go                 # 主入口，MCP工具注册
├── types.go                   # 数据类型定义
├── config.go                  # 配置管理
│
├── sources/                   # 数据源适配器
│   ├── interface.go           # 数据源接口定义
│   ├── router.go              # 智能路由器
│   ├── health.go              # 健康检查
│   ├── ratelimit.go           # 频率限制
│   │
│   ├── eastmoney/             # 东方财富
│   │   ├── client.go
│   │   ├── quote.go
│   │   ├── kline.go
│   │   └── news.go
│   │
│   ├── sina/                  # 新浪财经
│   │   ├── client.go
│   │   ├── quote.go
│   │   └── kline.go
│   │
│   ├── tencent/               # 腾讯财经
│   │   └── client.go
│   │
│   ├── tiantian/              # 天天基金
│   │   └── fund.go
│   │
│   ├── cls/                   # 财联社
│   │   └── news.go
│   │
│   ├── jin10/                 # 金十数据
│   │   └── news.go
│   │
│   └── coingecko/             # CoinGecko
│       └── crypto.go
│
├── services/                  # 业务服务
│   ├── quote.go               # 行情服务
│   ├── history.go             # 历史数据
│   ├── search.go              # 搜索服务
│   ├── portfolio.go           # 组合管理
│   ├── alert.go               # 提醒服务
│   ├── news.go                # 新闻聚合
│   └── analysis.go            # 分析服务
│
├── indicators/                # 技术指标
│   ├── ma.go                  # 移动平均
│   ├── macd.go
│   ├── rsi.go
│   ├── kdj.go
│   ├── boll.go
│   └── engine.go              # 指标引擎
│
├── storage/                   # 存储层
│   ├── redis/                 # Redis 缓存
│   │   └── cache.go
│   ├── postgres/              # PostgreSQL
│   │   ├── migrations/        # 数据库迁移
│   │   ├── assets.go
│   │   ├── portfolio.go
│   │   └── news.go
│   └── timescale/             # TimescaleDB 时序
│       ├── quotes.go
│       └── klines.go
│
└── tests/
    ├── sources_test.go
    ├── services_test.go
    └── integration_test.go
```

## 8. 实现优先级

### Phase 1: 基础行情 (MVP)
- [ ] 数据类型定义和配置
- [ ] 东方财富数据源适配器
- [ ] 新浪财经数据源适配器（备份）
- [ ] 智能路由器和健康检查
- [ ] Redis 缓存层
- [ ] finance_quote 工具
- [ ] finance_search 工具

### Phase 2: 历史数据与存储
- [ ] PostgreSQL Schema 和连接
- [ ] TimescaleDB 时序表
- [ ] 历史K线数据获取和存储
- [ ] finance_history 工具
- [ ] 基金数据源（天天基金）

### Phase 3: 财经新闻
- [ ] 财联社数据源
- [ ] 金十数据源
- [ ] 新闻聚合服务
- [ ] finance_news 工具
- [ ] 新闻存储和搜索

### Phase 4: 技术分析
- [ ] 技术指标引擎
- [ ] MA/EMA/MACD/RSI/KDJ/BOLL
- [ ] finance_analysis 工具
- [ ] 分析报告生成

### Phase 5: 组合与提醒
- [ ] 投资组合管理
- [ ] 交易记录
- [ ] 收益计算
- [ ] 价格提醒系统
- [ ] finance_portfolio 工具
- [ ] finance_alert 工具

### Phase 6: 市场概览与优化
- [ ] 市场指数、板块
- [ ] 资金流向
- [ ] finance_market 工具
- [ ] 性能优化
- [ ] 监控告警

## 9. Skills 定义（Agent 使用指南）

```yaml
name: financial_tracker
description: |
  金融资产追踪工具，支持A股/港股/美股/基金/黄金等多种资产的行情查询、
  技术分析、投资组合管理和财经情报搜集。

capabilities:
  - 实时行情查询（多数据源互备）
  - 历史K线数据
  - 技术指标分析（MA/MACD/RSI/KDJ/BOLL等）
  - 财经新闻快讯聚合
  - 投资组合管理
  - 价格和技术指标提醒
  - 市场概览和板块热度

data_sources:
  primary: [东方财富, 新浪财经]
  backup: [腾讯财经, 网易财经, 同花顺]
  news: [财联社, 金十数据, 东方财富]
  fund: [天天基金, 东方财富]

usage_examples:
  - query: "贵州茅台现在多少钱"
    tool: finance_quote
    params: {symbol: "600519"}
    
  - query: "看下腾讯最近一个月的走势"
    tool: finance_history
    params: {symbol: "00700.HK", period: "1d", count: 30}
    
  - query: "分析一下比亚迪的技术面"
    tool: finance_analysis
    params: {symbol: "002594", indicators: ["ma", "macd", "rsi", "kdj"]}
    
  - query: "今天有什么重要财经新闻"
    tool: finance_news
    params: {action: "latest", category: "macro", limit: 20}
    
  - query: "茅台有什么最新消息"
    tool: finance_news
    params: {action: "symbol", symbol: "600519"}
    
  - query: "帮我建一个投资组合"
    tool: finance_portfolio
    params: {action: "create", name: "我的组合"}
    
  - query: "茅台跌到1500提醒我"
    tool: finance_alert
    params: {action: "create", symbol: "600519", condition: "price_below", value: 1500}
    
  - query: "今天A股大盘怎么样"
    tool: finance_market
    params: {market: "cn", include: ["indices", "top_gainers", "top_losers"]}

symbol_formats:
  A股上海: "600519, 601318"
  A股深圳: "000001, 002594, 300750"
  港股: "00700.HK, 09988.HK"
  美股: "AAPL, TSLA, GOOGL"
  基金: "005827, 110011"
  ETF: "510300, 159919"
  加密货币: "BTC, ETH"
  黄金: "XAUUSD, AU9999"
  
tips:
  - 股票代码可以直接输入数字，系统自动识别市场
  - 支持中文名称和拼音首字母搜索
  - 财联社快讯更新最快，金十侧重全球宏观
  - 技术分析默认生成综合报告和交易建议
  - 基金净值通常每日更新，盘中为估值
```

## 10. 开发进度跟踪

### 10.1 TODO 清单

#### Phase 1: 基础行情 (MVP) - 预计 2-3 天
| ID | 任务 | 状态 | 备注 |
|----|------|------|------|
| 1.1 | 创建 `tools/finance/` 目录结构 | ✅ 完成 | 2024-02-07 |
| 1.2 | 定义基础数据类型 (`types/types.go`) | ✅ 完成 | Quote/KLine/Asset 等 |
| 1.3 | 实现数据源接口 (`sources/interface.go`) | ✅ 完成 | DataSource 接口定义 |
| 1.4 | 实现东方财富数据源 (`sources/eastmoney/`) | ✅ 完成 | A股/港股/美股行情 |
| 1.5 | 实现新浪财经数据源 (`sources/sina/`) | ✅ 完成 | 备份数据源 |
| 1.6 | 实现智能路由器 (`sources/router.go`) | ✅ 完成 | 故障转移逻辑 |
| 1.7 | 实现健康检查 (`sources/health.go`) | ✅ 完成 | 熔断机制 |
| 1.8 | 实现频率限制 (`sources/ratelimit.go`) | ✅ 完成 | 令牌桶限速 |
| 1.9 | 实现 Redis 缓存 (`cache/cache.go`) | ✅ 完成 | 行情/K线/搜索缓存 |
| 1.10 | 实现 `finance_quote` 工具 | ✅ 完成 | MCP工具注册 |
| 1.11 | 实现 `finance_search` 工具 | ✅ 完成 | 搜索功能 |
| 1.12 | 集成到 MCP Server | ✅ 完成 | 注册工具 |
| 1.13 | 单元测试 | ✅ 完成 | 覆盖核心逻辑 |
| 1.14 | 集成测试 | ✅ 完成 | 端到端验证 |

#### Phase 2: 历史数据与存储 - ✅ 已完成
| ID | 任务 | 状态 | 备注 |
|----|------|------|------|
| 2.1 | PostgreSQL Schema 设计 | ✅ 完成 | storage/schema.sql |
| 2.2 | TimescaleDB 时序表设计 | ✅ 完成 | hypertable配置 |
| 2.3 | 数据库连接池 | ✅ 完成 | storage/db.go |
| 2.4 | K线数据获取 | ✅ 完成 | 东方财富/新浪 |
| 2.5 | K线数据存储 | ✅ 完成 | TimescaleDB |
| 2.6 | 实现 `finance_history` 工具 | ✅ 完成 | MCP集成 |
| 2.7 | 天天基金数据源 | ✅ 完成 | sources/tiantian/ |
| 2.8 | 单元测试 | ✅ 完成 | |

#### Phase 3: 财经新闻 - ✅ 已完成
| ID | 任务 | 状态 | 备注 |
|----|------|------|------|
| 3.1 | 财联社数据源 | ✅ 完成 | news/cls.go |
| 3.2 | 金十数据源 | ✅ 完成 | news/jin10.go |
| 3.3 | 东财新闻解析 | ✅ 完成 | 公告/新闻 |
| 3.4 | 新闻聚合服务 | ✅ 完成 | news/aggregator.go |
| 3.5 | 新闻存储 | ✅ 完成 | storage/db.go |
| 3.6 | 实现 `finance_news` 工具 | ✅ 完成 | MCP集成 |
| 3.7 | 单元测试 | ✅ 完成 | news/news_test.go |

#### Phase 4: 技术分析 - ✅ 已完成
| ID | 任务 | 状态 | 备注 |
|----|------|------|------|
| 4.1 | 技术指标引擎架构 | ✅ 完成 | indicators/indicators.go |
| 4.2 | MA/EMA 实现 | ✅ 完成 | |
| 4.3 | MACD 实现 | ✅ 完成 | |
| 4.4 | RSI 实现 | ✅ 完成 | |
| 4.5 | KDJ 实现 | ✅ 完成 | |
| 4.6 | BOLL 实现 | ✅ 完成 | 另有WR/CCI/ATR/OBV/VOL |
| 4.7 | 分析报告生成 | ✅ 完成 | Analyze函数 |
| 4.8 | 实现 `finance_analysis` 工具 | ✅ 完成 | MCP集成 |
| 4.9 | 单元测试 | ✅ 完成 | indicators/indicators_test.go |

#### Phase 5: 组合与提醒 - ✅ 已完成
| ID | 任务 | 状态 | 备注 |
|----|------|------|------|
| 5.1 | 投资组合存储 | ✅ 完成 | portfolio/portfolio.go |
| 5.2 | 持仓管理 | ✅ 完成 | |
| 5.3 | 交易记录 | ✅ 完成 | |
| 5.4 | 收益计算 | ✅ 完成 | |
| 5.5 | 价格提醒系统 | ✅ 完成 | alert/alert.go |
| 5.6 | 提醒检查器 | ✅ 完成 | CheckQuote/CheckQuotes |
| 5.7 | 实现 `finance_portfolio` 工具 | ✅ 完成 | MCP集成 |
| 5.8 | 实现 `finance_alert` 工具 | ✅ 完成 | MCP集成 |
| 5.9 | 单元测试 | ✅ 完成 | portfolio_test.go, alert_test.go |

#### Phase 6: 市场概览与优化 - ✅ 已完成
| ID | 任务 | 状态 | 备注 |
|----|------|------|------|
| 6.1 | 市场指数获取 | ✅ 完成 | market/market.go |
| 6.2 | 板块热度 | ✅ 完成 | GetSectors |
| 6.3 | 涨跌排行 | ✅ 完成 | TopGainers/TopLosers |
| 6.4 | 实现 `finance_market` 工具 | ✅ 完成 | MCP集成 |
| 6.5 | 性能优化 | ✅ 完成 | 缓存机制 |
| 6.6 | 完整文档 | ✅ 完成 | Skills YAML |

### 10.2 设计偏差记录

| 日期 | 模块 | 原设计 | 实际实现 | 原因 |
|------|------|--------|----------|------|
| 2024-02-07 | 类型定义 | types.go在finance包根目录 | 独立types包 | 避免包循环引用 |
| 2024-02-07 | 新浪API | UTF-8编码 | GBK编码转换 | API返回GBK编码,已添加转换 |
| 2024-02-07 | 投资组合 | PostgreSQL存储 | 内存存储 | 简化首版,后续可扩展DB |
| 2024-02-07 | 提醒系统 | 数据库持久化 | 内存管理 | 简化首版,后续可扩展DB |

### 10.3 开发日志

| 日期 | 完成任务 | 遇到问题 | 解决方案 |
|------|----------|----------|----------|
| 2024-02-07 | Phase 1 核心功能开发完成 | 包循环引用 | 将types.go移至独立的types包 |
| 2024-02-07 | 东方财富/新浪数据源实现 | API响应格式复杂 | 逐字段解析，健壮处理 |
| 2024-02-07 | 智能路由/健康检查/熔断机制 | - | 参考搜索工具架构 |
| 2024-02-07 | MCP工具集成 | - | 添加FinanceConfig到配置 |
| 2024-02-07 | Phase 2-6 全部功能开发 | 新浪API返回GBK | 添加encoding.go进行转换 |
| 2024-02-07 | 数据库Schema设计 | TimescaleDB配置 | storage/schema.sql |
| 2024-02-07 | 天天基金数据源 | 基金净值接口 | sources/tiantian/ |
| 2024-02-07 | 财联社/金十新闻源 | 不同API格式 | news/cls.go, jin10.go |
| 2024-02-07 | 新闻聚合器 | 并发去重排序 | news/aggregator.go |
| 2024-02-07 | 技术指标引擎 | 11种指标算法 | indicators/indicators.go |
| 2024-02-07 | 投资组合管理 | 持仓/交易记录 | portfolio/portfolio.go |
| 2024-02-07 | 价格提醒系统 | 条件触发检测 | alert/alert.go |
| 2024-02-07 | 市场概览服务 | 指数/板块/排行 | market/market.go |
| 2024-02-07 | 全部MCP工具 | 9个工具注册 | finance.go |
| 2024-02-07 | 单元测试完成 | 测试覆盖核心逻辑 | *_test.go |
| 2024-02-07 | 集成测试完成 | 端到端验证 | tests/integration/ |

---

## 11. 部署配置

### 11.1 Docker Compose

```yaml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes
    
  postgres:
    image: timescale/timescaledb:latest-pg15
    ports:
      - "5432:5432"
    environment:
      POSTGRES_USER: finance
      POSTGRES_PASSWORD: ${PG_PASSWORD}
      POSTGRES_DB: finance_tracker
    volumes:
      - pg_data:/var/lib/postgresql/data
      - ./storage/postgres/migrations:/docker-entrypoint-initdb.d

volumes:
  redis_data:
  pg_data:
```

### 11.2 环境变量

```bash
# 数据库配置
REDIS_URL=redis://localhost:6379
PG_URL=postgres://finance:password@localhost:5432/finance_tracker

# 数据源配置（可选，用于付费接口）
EASTMONEY_TOKEN=
XUEQIU_TOKEN=

# 功能开关
ENABLE_CRYPTO=false  # 加密货币需要代理
ENABLE_NEWS_SENTIMENT=true  # 新闻情感分析

# 缓存TTL（秒）
CACHE_TTL_QUOTE=3
CACHE_TTL_NEWS=300
```
