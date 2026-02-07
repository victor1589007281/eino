-- 财经工具数据库 Schema
-- PostgreSQL + TimescaleDB

-- 启用TimescaleDB扩展
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- =====================================================
-- 1. 资产基础信息表
-- =====================================================
CREATE TABLE IF NOT EXISTS assets (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    name VARCHAR(100),
    asset_type VARCHAR(20) NOT NULL,  -- stock, etf, fund, index, bond, forex, crypto
    market VARCHAR(10) NOT NULL,       -- sh, sz, hk, us, cn, global
    exchange VARCHAR(50),
    industry VARCHAR(100),
    sector VARCHAR(100),
    list_date DATE,
    delist_date DATE,
    status VARCHAR(10) DEFAULT 'active',  -- active, suspended, delisted
    extra_info JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(symbol, market)
);

CREATE INDEX idx_assets_symbol ON assets(symbol);
CREATE INDEX idx_assets_type ON assets(asset_type);
CREATE INDEX idx_assets_market ON assets(market);
CREATE INDEX idx_assets_status ON assets(status);

-- =====================================================
-- 2. K线数据表 (TimescaleDB Hypertable)
-- =====================================================
CREATE TABLE IF NOT EXISTS kline (
    time TIMESTAMPTZ NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    market VARCHAR(10) NOT NULL,
    period VARCHAR(10) NOT NULL,  -- 1m, 5m, 15m, 30m, 60m, 1d, 1w, 1M
    open DECIMAL(20, 4),
    high DECIMAL(20, 4),
    low DECIMAL(20, 4),
    close DECIMAL(20, 4),
    volume BIGINT,
    amount DECIMAL(20, 4),
    turnover_rate DECIMAL(10, 4),
    adj_factor DECIMAL(10, 6) DEFAULT 1.0,  -- 复权因子
    PRIMARY KEY (time, symbol, market, period)
);

-- 转换为TimescaleDB超表
SELECT create_hypertable('kline', 'time', 
    chunk_time_interval => INTERVAL '1 week',
    if_not_exists => TRUE
);

-- 创建索引
CREATE INDEX idx_kline_symbol ON kline(symbol, time DESC);
CREATE INDEX idx_kline_market ON kline(market, time DESC);

-- 创建连续聚合视图(日线汇总)
CREATE MATERIALIZED VIEW IF NOT EXISTS kline_daily_stats
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', time) AS bucket,
    symbol,
    market,
    first(open, time) AS open,
    max(high) AS high,
    min(low) AS low,
    last(close, time) AS close,
    sum(volume) AS volume,
    sum(amount) AS amount
FROM kline
WHERE period = '1d'
GROUP BY bucket, symbol, market
WITH NO DATA;

-- =====================================================
-- 3. 实时行情快照表
-- =====================================================
CREATE TABLE IF NOT EXISTS quote_snapshot (
    time TIMESTAMPTZ NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    market VARCHAR(10) NOT NULL,
    name VARCHAR(100),
    price DECIMAL(20, 4),
    open DECIMAL(20, 4),
    high DECIMAL(20, 4),
    low DECIMAL(20, 4),
    pre_close DECIMAL(20, 4),
    change DECIMAL(20, 4),
    change_pct DECIMAL(10, 4),
    volume BIGINT,
    amount DECIMAL(20, 4),
    bid1 DECIMAL(20, 4),
    ask1 DECIMAL(20, 4),
    bid1_vol BIGINT,
    ask1_vol BIGINT,
    turnover_rate DECIMAL(10, 4),
    pe_ratio DECIMAL(10, 2),
    pb_ratio DECIMAL(10, 2),
    market_cap DECIMAL(20, 2),
    source VARCHAR(20),
    PRIMARY KEY (time, symbol, market)
);

SELECT create_hypertable('quote_snapshot', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- =====================================================
-- 4. 基金信息表
-- =====================================================
CREATE TABLE IF NOT EXISTS funds (
    id SERIAL PRIMARY KEY,
    code VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(200),
    short_name VARCHAR(100),
    fund_type VARCHAR(50),    -- 股票型, 混合型, 债券型, 指数型, QDII, 货币型
    management VARCHAR(100),  -- 基金公司
    manager VARCHAR(100),     -- 基金经理
    establish_date DATE,
    benchmark VARCHAR(500),
    fee_rate DECIMAL(5, 4),   -- 管理费率
    status VARCHAR(20) DEFAULT 'active',
    extra_info JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_funds_code ON funds(code);
CREATE INDEX idx_funds_type ON funds(fund_type);

-- =====================================================
-- 5. 基金净值表
-- =====================================================
CREATE TABLE IF NOT EXISTS fund_nav (
    time DATE NOT NULL,
    code VARCHAR(20) NOT NULL,
    nav DECIMAL(10, 4),           -- 单位净值
    acc_nav DECIMAL(10, 4),       -- 累计净值
    daily_return DECIMAL(10, 4),  -- 日收益率
    PRIMARY KEY (time, code)
);

SELECT create_hypertable('fund_nav', 'time',
    chunk_time_interval => INTERVAL '1 month',
    if_not_exists => TRUE
);

CREATE INDEX idx_fund_nav_code ON fund_nav(code, time DESC);

-- =====================================================
-- 6. 财经新闻表
-- =====================================================
CREATE TABLE IF NOT EXISTS news (
    id SERIAL PRIMARY KEY,
    news_id VARCHAR(100) UNIQUE,  -- 外部ID，用于去重
    title VARCHAR(500) NOT NULL,
    content TEXT,
    summary VARCHAR(1000),
    source VARCHAR(50) NOT NULL,  -- cls, jin10, eastmoney
    category VARCHAR(50),         -- 宏观, 公司, 行业, 全球
    importance INT DEFAULT 0,     -- 重要性 0-5
    symbols TEXT[],               -- 关联股票代码
    tags TEXT[],
    url VARCHAR(500),
    publish_time TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_news_source ON news(source, publish_time DESC);
CREATE INDEX idx_news_category ON news(category);
CREATE INDEX idx_news_symbols ON news USING GIN(symbols);
CREATE INDEX idx_news_tags ON news USING GIN(tags);
CREATE INDEX idx_news_publish_time ON news(publish_time DESC);

-- =====================================================
-- 7. 投资组合表
-- =====================================================
CREATE TABLE IF NOT EXISTS portfolios (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(100) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    currency VARCHAR(10) DEFAULT 'CNY',
    initial_capital DECIMAL(20, 2) DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, name)
);

CREATE INDEX idx_portfolios_user ON portfolios(user_id);

-- =====================================================
-- 8. 持仓表
-- =====================================================
CREATE TABLE IF NOT EXISTS positions (
    id SERIAL PRIMARY KEY,
    portfolio_id INT NOT NULL REFERENCES portfolios(id) ON DELETE CASCADE,
    symbol VARCHAR(20) NOT NULL,
    market VARCHAR(10) NOT NULL,
    asset_type VARCHAR(20) NOT NULL,
    quantity DECIMAL(20, 4) NOT NULL,
    avg_cost DECIMAL(20, 4) NOT NULL,
    current_price DECIMAL(20, 4),
    market_value DECIMAL(20, 2),
    profit_loss DECIMAL(20, 2),
    profit_pct DECIMAL(10, 4),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(portfolio_id, symbol, market)
);

CREATE INDEX idx_positions_portfolio ON positions(portfolio_id);
CREATE INDEX idx_positions_symbol ON positions(symbol);

-- =====================================================
-- 9. 交易记录表
-- =====================================================
CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    portfolio_id INT NOT NULL REFERENCES portfolios(id) ON DELETE CASCADE,
    symbol VARCHAR(20) NOT NULL,
    market VARCHAR(10) NOT NULL,
    trade_type VARCHAR(10) NOT NULL,  -- buy, sell
    quantity DECIMAL(20, 4) NOT NULL,
    price DECIMAL(20, 4) NOT NULL,
    amount DECIMAL(20, 2) NOT NULL,
    fee DECIMAL(20, 4) DEFAULT 0,
    tax DECIMAL(20, 4) DEFAULT 0,
    note TEXT,
    trade_time TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_transactions_portfolio ON transactions(portfolio_id, trade_time DESC);
CREATE INDEX idx_transactions_symbol ON transactions(symbol);

-- =====================================================
-- 10. 价格提醒表
-- =====================================================
CREATE TABLE IF NOT EXISTS alerts (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(100) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    market VARCHAR(10) NOT NULL,
    name VARCHAR(100),
    alert_type VARCHAR(20) NOT NULL,  -- price_above, price_below, change_above, change_below, volume_above
    threshold DECIMAL(20, 4) NOT NULL,
    current_value DECIMAL(20, 4),
    status VARCHAR(20) DEFAULT 'active',  -- active, triggered, disabled
    triggered_at TIMESTAMPTZ,
    triggered_value DECIMAL(20, 4),
    notify_methods TEXT[] DEFAULT '{}',  -- email, sms, push
    note TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_alerts_user ON alerts(user_id);
CREATE INDEX idx_alerts_symbol ON alerts(symbol, market);
CREATE INDEX idx_alerts_status ON alerts(status);

-- =====================================================
-- 11. 市场概览缓存表
-- =====================================================
CREATE TABLE IF NOT EXISTS market_overview (
    time TIMESTAMPTZ NOT NULL,
    market VARCHAR(10) NOT NULL,
    index_symbol VARCHAR(20),
    index_name VARCHAR(50),
    index_price DECIMAL(20, 4),
    index_change DECIMAL(20, 4),
    index_change_pct DECIMAL(10, 4),
    advance_count INT,      -- 上涨家数
    decline_count INT,      -- 下跌家数
    unchanged_count INT,    -- 平盘家数
    total_volume BIGINT,    -- 总成交量
    total_amount DECIMAL(20, 2),  -- 总成交额
    limit_up_count INT,     -- 涨停家数
    limit_down_count INT,   -- 跌停家数
    north_inflow DECIMAL(20, 2),  -- 北向资金净流入
    extra_data JSONB DEFAULT '{}',
    PRIMARY KEY (time, market)
);

SELECT create_hypertable('market_overview', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- =====================================================
-- 12. 板块行情表
-- =====================================================
CREATE TABLE IF NOT EXISTS sector_quotes (
    time TIMESTAMPTZ NOT NULL,
    sector_code VARCHAR(20) NOT NULL,
    sector_name VARCHAR(50) NOT NULL,
    sector_type VARCHAR(20) NOT NULL,  -- industry, concept, region
    change_pct DECIMAL(10, 4),
    lead_stock VARCHAR(20),      -- 领涨股
    lead_stock_name VARCHAR(50),
    lead_change_pct DECIMAL(10, 4),
    total_amount DECIMAL(20, 2),
    net_inflow DECIMAL(20, 2),   -- 资金净流入
    stock_count INT,
    advance_count INT,
    decline_count INT,
    PRIMARY KEY (time, sector_code)
);

SELECT create_hypertable('sector_quotes', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

CREATE INDEX idx_sector_type ON sector_quotes(sector_type, time DESC);

-- =====================================================
-- 13. 数据刷新策略(TimescaleDB)
-- =====================================================

-- 自动刷新连续聚合
SELECT add_continuous_aggregate_policy('kline_daily_stats',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE
);

-- 数据保留策略 - K线数据保留2年
SELECT add_retention_policy('kline', INTERVAL '2 years', if_not_exists => TRUE);

-- 行情快照保留30天
SELECT add_retention_policy('quote_snapshot', INTERVAL '30 days', if_not_exists => TRUE);

-- 市场概览保留1年
SELECT add_retention_policy('market_overview', INTERVAL '1 year', if_not_exists => TRUE);

-- 板块行情保留1年
SELECT add_retention_policy('sector_quotes', INTERVAL '1 year', if_not_exists => TRUE);

-- =====================================================
-- 14. 常用函数
-- =====================================================

-- 计算技术指标 - 简单移动平均
CREATE OR REPLACE FUNCTION calc_sma(symbol_in VARCHAR, period_days INT)
RETURNS DECIMAL AS $$
SELECT AVG(close)
FROM (
    SELECT close
    FROM kline
    WHERE symbol = symbol_in AND period = '1d'
    ORDER BY time DESC
    LIMIT period_days
) t;
$$ LANGUAGE SQL;

-- 获取最新行情
CREATE OR REPLACE FUNCTION get_latest_quote(symbol_in VARCHAR, market_in VARCHAR)
RETURNS TABLE(
    price DECIMAL,
    change DECIMAL,
    change_pct DECIMAL,
    volume BIGINT,
    update_time TIMESTAMPTZ
) AS $$
SELECT price, change, change_pct, volume, time
FROM quote_snapshot
WHERE symbol = symbol_in AND market = market_in
ORDER BY time DESC
LIMIT 1;
$$ LANGUAGE SQL;
