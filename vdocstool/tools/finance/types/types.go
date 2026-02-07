// Package types 财经数据类型定义
package types

import (
	"time"
)

// AssetType 资产类型
type AssetType string

const (
	AssetTypeStock  AssetType = "stock"  // 股票
	AssetTypeFund   AssetType = "fund"   // 基金
	AssetTypeETF    AssetType = "etf"    // ETF
	AssetTypeGold   AssetType = "gold"   // 黄金
	AssetTypeBond   AssetType = "bond"   // 债券
	AssetTypeFuture AssetType = "future" // 期货
	AssetTypeForex  AssetType = "forex"  // 外汇
	AssetTypeCrypto AssetType = "crypto" // 加密货币
	AssetTypeIndex  AssetType = "index"  // 指数
)

// Market 市场
type Market string

const (
	MarketSH     Market = "sh"     // 上海
	MarketSZ     Market = "sz"     // 深圳
	MarketBJ     Market = "bj"     // 北京
	MarketHK     Market = "hk"     // 香港
	MarketUS     Market = "us"     // 美国
	MarketCN     Market = "cn"     // 中国 (通用)
	MarketGlobal Market = "global" // 全球
)

// Quote 实时行情
type Quote struct {
	Symbol       string    `json:"symbol"`        // 股票代码
	Name         string    `json:"name"`          // 名称
	Market       Market    `json:"market"`        // 市场
	AssetType    AssetType `json:"asset_type"`    // 资产类型
	Price        float64   `json:"price"`         // 最新价
	Open         float64   `json:"open"`          // 开盘价
	High         float64   `json:"high"`          // 最高价
	Low          float64   `json:"low"`           // 最低价
	PreClose     float64   `json:"pre_close"`     // 昨收
	Change       float64   `json:"change"`        // 涨跌额
	ChangePct    float64   `json:"change_pct"`    // 涨跌幅(%)
	Volume       int64     `json:"volume"`        // 成交量
	Amount       float64   `json:"amount"`        // 成交额
	TurnoverRate float64   `json:"turnover_rate"` // 换手率(%)
	MarketCap    float64   `json:"market_cap"`    // 总市值
	PERatio      float64   `json:"pe_ratio"`      // 市盈率
	PBRatio      float64   `json:"pb_ratio"`      // 市净率
	Bid1         float64   `json:"bid1"`          // 买一价
	Bid1Vol      int64     `json:"bid1_vol"`      // 买一量
	Ask1         float64   `json:"ask1"`          // 卖一价
	Ask1Vol      int64     `json:"ask1_vol"`      // 卖一量
	UpdateTime   time.Time `json:"update_time"`   // 更新时间
	Source       string    `json:"source"`        // 数据来源
}

// KLine K线数据
type KLine struct {
	Symbol       string    `json:"symbol"`        // 股票代码
	Market       Market    `json:"market"`        // 市场
	Period       string    `json:"period"`        // 周期: 1m/5m/15m/30m/60m/1d/1w/1M
	Timestamp    time.Time `json:"timestamp"`     // 时间戳
	Open         float64   `json:"open"`          // 开盘价
	High         float64   `json:"high"`          // 最高价
	Low          float64   `json:"low"`           // 最低价
	Close        float64   `json:"close"`         // 收盘价
	Volume       int64     `json:"volume"`        // 成交量
	Amount       float64   `json:"amount"`        // 成交额
	AdjFactor    float64   `json:"adj_factor"`    // 复权因子
	TurnoverRate float64   `json:"turnover_rate"` // 换手率
}

// FundInfo 基金信息
type FundInfo struct {
	Code            string    `json:"code"`              // 基金代码
	Name            string    `json:"name"`              // 基金名称
	Type            string    `json:"type"`              // 基金类型
	NAV             float64   `json:"nav"`               // 单位净值
	AccNAV          float64   `json:"acc_nav"`           // 累计净值
	EstimatedNAV    float64   `json:"estimated_nav"`     // 估算净值
	DayGrowth       float64   `json:"day_growth"`        // 日增长率
	WeekGrowth      float64   `json:"week_growth"`       // 周增长率
	MonthGrowth     float64   `json:"month_growth"`      // 月增长率
	ThreeMonthGrow  float64   `json:"three_month_grow"`  // 三月增长率
	SixMonthGrowth  float64   `json:"six_month_growth"`  // 六月增长率
	YearGrowth      float64   `json:"year_growth"`       // 年增长率
	TotalAsset      float64   `json:"total_asset"`       // 基金规模(亿)
	Manager         string    `json:"manager"`           // 基金经理
	Company         string    `json:"company"`           // 基金公司
	EstablishedDate string    `json:"established_date"`  // 成立日期
	UpdateTime      time.Time `json:"update_time"`       // 更新时间
	Source          string    `json:"source"`            // 数据来源
}

// GoldPrice 黄金价格
type GoldPrice struct {
	Symbol     string    `json:"symbol"`      // 代码 (如 XAUUSD, AU9999)
	Name       string    `json:"name"`        // 名称
	Price      float64   `json:"price"`       // 最新价
	Open       float64   `json:"open"`        // 开盘价
	High       float64   `json:"high"`        // 最高价
	Low        float64   `json:"low"`         // 最低价
	PreClose   float64   `json:"pre_close"`   // 昨收
	Change     float64   `json:"change"`      // 涨跌额
	ChangePct  float64   `json:"change_pct"`  // 涨跌幅
	Unit       string    `json:"unit"`        // 单位 (元/克, USD/oz)
	UpdateTime time.Time `json:"update_time"` // 更新时间
	Source     string    `json:"source"`      // 数据来源
}

// BondInfo 债券信息
type BondInfo struct {
	Code         string    `json:"code"`          // 债券代码
	Name         string    `json:"name"`          // 债券名称
	Type         string    `json:"type"`          // 债券类型
	Price        float64   `json:"price"`         // 最新价
	YTM          float64   `json:"ytm"`           // 到期收益率
	ModDuration  float64   `json:"mod_duration"`  // 修正久期
	CouponRate   float64   `json:"coupon_rate"`   // 票面利率
	MaturityDate string    `json:"maturity_date"` // 到期日
	Rating       string    `json:"rating"`        // 评级
	UpdateTime   time.Time `json:"update_time"`   // 更新时间
	Source       string    `json:"source"`        // 数据来源
}

// IndexInfo 指数信息
type IndexInfo struct {
	Code       string    `json:"code"`        // 指数代码
	Name       string    `json:"name"`        // 指数名称
	Price      float64   `json:"price"`       // 最新点位
	Open       float64   `json:"open"`        // 开盘
	High       float64   `json:"high"`        // 最高
	Low        float64   `json:"low"`         // 最低
	PreClose   float64   `json:"pre_close"`   // 昨收
	Change     float64   `json:"change"`      // 涨跌
	ChangePct  float64   `json:"change_pct"`  // 涨跌幅
	Volume     int64     `json:"volume"`      // 成交量(手)
	Amount     float64   `json:"amount"`      // 成交额(亿)
	UpCount    int       `json:"up_count"`    // 上涨家数
	DownCount  int       `json:"down_count"`  // 下跌家数
	UpdateTime time.Time `json:"update_time"` // 更新时间
	Source     string    `json:"source"`      // 数据来源
}

// SearchResult 搜索结果
type SearchResult struct {
	Symbol    string    `json:"symbol"`     // 代码
	Name      string    `json:"name"`       // 名称
	Market    Market    `json:"market"`     // 市场
	AssetType AssetType `json:"asset_type"` // 类型
	Exchange  string    `json:"exchange"`   // 交易所
}

// News 财经新闻
type News struct {
	ID          string    `json:"id"`           // 新闻ID
	Title       string    `json:"title"`        // 标题
	Summary     string    `json:"summary"`      // 摘要
	Content     string    `json:"content"`      // 内容
	Source      string    `json:"source"`       // 来源
	Category    string    `json:"category"`     // 分类
	Symbols     []string  `json:"symbols"`      // 相关股票
	Tags        []string  `json:"tags"`         // 标签
	URL         string    `json:"url"`          // 原文链接
	PublishTime time.Time `json:"publish_time"` // 发布时间
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	Importance  int       `json:"importance"`   // 重要程度 1-5
}

// Announcement 公告
type Announcement struct {
	ID          string    `json:"id"`           // 公告ID
	Symbol      string    `json:"symbol"`       // 股票代码
	Title       string    `json:"title"`        // 标题
	Type        string    `json:"type"`         // 类型
	Content     string    `json:"content"`      // 内容
	URL         string    `json:"url"`          // 原文链接
	PublishTime time.Time `json:"publish_time"` // 发布时间
}

// Portfolio 投资组合
type Portfolio struct {
	ID             int64       `json:"id"`               // 主键
	UserID         string      `json:"user_id"`          // 用户ID
	Name           string      `json:"name"`             // 组合名称
	Description    string      `json:"description"`      // 描述
	Currency       string      `json:"currency"`         // 币种
	InitialCapital float64     `json:"initial_capital"`  // 初始资金
	TotalValue     float64     `json:"total_value"`      // 总市值
	TotalProfit    float64     `json:"total_profit"`     // 总盈亏
	TotalProfitPct float64     `json:"total_profit_pct"` // 总收益率
	Positions      []*Position `json:"positions"`        // 持仓列表
	CreatedAt      time.Time   `json:"created_at"`       // 创建时间
	UpdatedAt      time.Time   `json:"updated_at"`       // 更新时间
}

// Position 持仓
type Position struct {
	ID           int64     `json:"id"`            // 主键
	PortfolioID  int64     `json:"portfolio_id"`  // 组合ID
	Symbol       string    `json:"symbol"`        // 代码
	Name         string    `json:"name"`          // 名称
	Market       Market    `json:"market"`        // 市场
	AssetType    AssetType `json:"asset_type"`    // 资产类型
	Quantity     float64   `json:"quantity"`      // 数量
	AvgCost      float64   `json:"avg_cost"`      // 平均成本
	CurrentPrice float64   `json:"current_price"` // 现价
	MarketValue  float64   `json:"market_value"`  // 市值
	ProfitLoss   float64   `json:"profit_loss"`   // 盈亏
	ProfitPct    float64   `json:"profit_pct"`    // 盈亏比例
	UpdatedAt    time.Time `json:"updated_at"`    // 更新时间
}

// Transaction 交易记录
type Transaction struct {
	ID          int64     `json:"id"`           // 主键
	PortfolioID int64     `json:"portfolio_id"` // 组合ID
	Symbol      string    `json:"symbol"`       // 代码
	Market      Market    `json:"market"`       // 市场
	TradeType   string    `json:"trade_type"`   // buy/sell
	Quantity    float64   `json:"quantity"`     // 数量
	Price       float64   `json:"price"`        // 价格
	Amount      float64   `json:"amount"`       // 金额
	Fee         float64   `json:"fee"`          // 手续费
	Tax         float64   `json:"tax"`          // 印花税
	Note        string    `json:"note"`         // 备注
	TradeTime   time.Time `json:"trade_time"`   // 交易时间
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
}

// Alert 价格提醒
type Alert struct {
	ID             int64      `json:"id"`              // 主键
	UserID         string     `json:"user_id"`         // 用户ID
	Symbol         string     `json:"symbol"`          // 代码
	Name           string     `json:"name"`            // 名称
	Market         Market     `json:"market"`          // 市场
	AlertType      string     `json:"alert_type"`      // price_above/price_below/change_above/change_below/volume_above
	Threshold      float64    `json:"threshold"`       // 阈值
	CurrentValue   float64    `json:"current_value"`   // 当前值
	Status         string     `json:"status"`          // active/triggered/disabled
	TriggeredAt    *time.Time `json:"triggered_at"`    // 触发时间
	TriggeredValue float64    `json:"triggered_value"` // 触发时的值
	NotifyMethods  []string   `json:"notify_methods"`  // 通知方式
	Note           string     `json:"note"`            // 备注
	CreatedAt      time.Time  `json:"created_at"`      // 创建时间
	UpdatedAt      time.Time  `json:"updated_at"`      // 更新时间
}

// MarketOverview 市场概览
type MarketOverview struct {
	Time           time.Time    `json:"time"`             // 时间
	Market         Market       `json:"market"`           // 市场
	IndexSymbol    string       `json:"index_symbol"`     // 指数代码
	IndexName      string       `json:"index_name"`       // 指数名称
	IndexPrice     float64      `json:"index_price"`      // 指数价格
	IndexChange    float64      `json:"index_change"`     // 涨跌
	IndexChangePct float64      `json:"index_change_pct"` // 涨跌幅
	Indices        []*IndexInfo `json:"indices"`          // 主要指数
	TopGainers     []*Quote     `json:"top_gainers"`      // 涨幅榜
	TopLosers      []*Quote     `json:"top_losers"`       // 跌幅榜
	TopVolume      []*Quote     `json:"top_volume"`       // 成交量榜
	TopAmount      []*Quote     `json:"top_amount"`       // 成交额榜
	AdvanceCount   int          `json:"advance_count"`    // 上涨家数
	DeclineCount   int          `json:"decline_count"`    // 下跌家数
	UnchangedCount int          `json:"unchanged_count"`  // 平盘家数
	TotalVolume    int64        `json:"total_volume"`     // 总成交量
	TotalAmount    float64      `json:"total_amount"`     // 总成交额
	LimitUpCount   int          `json:"limit_up_count"`   // 涨停家数
	LimitDownCount int          `json:"limit_down_count"` // 跌停家数
	NorthInflow    float64      `json:"north_inflow"`     // 北向资金净流入
	UpdateTime     time.Time    `json:"update_time"`      // 更新时间
}

// TechnicalIndicator 技术指标
type TechnicalIndicator struct {
	Name   string             `json:"name"`   // 指标名称
	Values map[string]float64 `json:"values"` // 指标值
	Signal string             `json:"signal"` // 信号: buy/sell/neutral
}

// AnalysisReport 分析报告
type AnalysisReport struct {
	Symbol     string                `json:"symbol"`      // 代码
	Name       string                `json:"name"`        // 名称
	Indicators []*TechnicalIndicator `json:"indicators"`  // 技术指标
	Trend      string                `json:"trend"`       // 趋势: up/down/sideways
	Support    float64               `json:"support"`     // 支撑位
	Resistance float64               `json:"resistance"`  // 阻力位
	Signal     string                `json:"signal"`      // 综合信号
	Summary    string                `json:"summary"`     // 分析摘要
	UpdateTime time.Time             `json:"update_time"` // 更新时间
}

// SectorInfo 板块信息
type SectorInfo struct {
	Code       string    `json:"code"`        // 板块代码
	Name       string    `json:"name"`        // 板块名称
	ChangePct  float64   `json:"change_pct"`  // 涨跌幅
	LeaderCode string    `json:"leader_code"` // 领涨股代码
	LeaderName string    `json:"leader_name"` // 领涨股名称
	UpCount    int       `json:"up_count"`    // 上涨家数
	DownCount  int       `json:"down_count"`  // 下跌家数
	Amount     float64   `json:"amount"`      // 成交额(亿)
	UpdateTime time.Time `json:"update_time"` // 更新时间
}

// EconomicEvent 经济事件
type EconomicEvent struct {
	ID         string    `json:"id"`         // 事件ID
	Time       time.Time `json:"time"`       // 发布时间
	Title      string    `json:"title"`      // 事件名称
	Country    string    `json:"country"`    // 国家
	Importance int       `json:"importance"` // 重要性 1-5
	Previous   string    `json:"previous"`   // 前值
	Consensus  string    `json:"consensus"`  // 预期值
	Actual     string    `json:"actual"`     // 实际值
	Unit       string    `json:"unit"`       // 单位
}

// FundNav 基金净值
type FundNav struct {
	Code        string    `json:"code"`         // 基金代码
	Date        time.Time `json:"date"`         // 日期
	NAV         float64   `json:"nav"`          // 单位净值
	AccNAV      float64   `json:"acc_nav"`      // 累计净值
	DailyReturn float64   `json:"daily_return"` // 日收益率
}

// SectorQuote 板块行情
type SectorQuote struct {
	Time          time.Time `json:"time"`            // 时间
	SectorCode    string    `json:"sector_code"`     // 板块代码
	SectorName    string    `json:"sector_name"`     // 板块名称
	SectorType    string    `json:"sector_type"`     // 板块类型: industry/concept/region
	ChangePct     float64   `json:"change_pct"`      // 涨跌幅
	LeadStock     string    `json:"lead_stock"`      // 领涨股代码
	LeadStockName string    `json:"lead_stock_name"` // 领涨股名称
	LeadChangePct float64   `json:"lead_change_pct"` // 领涨股涨幅
	TotalAmount   float64   `json:"total_amount"`    // 总成交额
	NetInflow     float64   `json:"net_inflow"`      // 资金净流入
	StockCount    int       `json:"stock_count"`     // 成分股数量
	AdvanceCount  int       `json:"advance_count"`   // 上涨家数
	DeclineCount  int       `json:"decline_count"`   // 下跌家数
}

// TopList 排行榜数据
type TopList struct {
	Type       string    `json:"type"`        // 榜单类型: gainer/loser/volume/amount/turnover
	Market     Market    `json:"market"`      // 市场
	Items      []*Quote  `json:"items"`       // 榜单数据
	UpdateTime time.Time `json:"update_time"` // 更新时间
}
