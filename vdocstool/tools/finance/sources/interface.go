// Package sources 数据源接口定义
package sources

import (
	"context"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// DataSource 数据源接口
type DataSource interface {
	// Name 数据源名称
	Name() string

	// Priority 优先级 (数字越小优先级越高)
	Priority() int

	// SupportedAssets 支持的资产类型
	SupportedAssets() []types.AssetType

	// SupportedMarkets 支持的市场
	SupportedMarkets() []types.Market

	// GetQuote 获取实时行情
	GetQuote(ctx context.Context, symbol string) (*types.Quote, error)

	// GetQuotes 批量获取行情
	GetQuotes(ctx context.Context, symbols []string) ([]*types.Quote, error)

	// GetKLine 获取K线数据
	GetKLine(ctx context.Context, symbol string, period string, count int) ([]*types.KLine, error)

	// Search 搜索股票/基金
	Search(ctx context.Context, keyword string) ([]*types.SearchResult, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error
}

// NewsSource 新闻数据源接口
type NewsSource interface {
	// Name 数据源名称
	Name() string

	// GetLatestNews 获取最新新闻
	GetLatestNews(ctx context.Context, category string, limit int) ([]*types.News, error)

	// GetSymbolNews 获取股票相关新闻
	GetSymbolNews(ctx context.Context, symbol string, limit int) ([]*types.News, error)

	// GetFlash 获取快讯
	GetFlash(ctx context.Context, limit int) ([]*types.News, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error
}

// FundSource 基金数据源接口
type FundSource interface {
	// Name 数据源名称
	Name() string

	// GetFundInfo 获取基金信息
	GetFundInfo(ctx context.Context, code string) (*types.FundInfo, error)

	// GetFundNav 获取基金净值历史
	GetFundNav(ctx context.Context, code string, pageIndex, pageSize int) ([]*types.FundNav, error)

	// SearchFund 搜索基金
	SearchFund(ctx context.Context, keyword string) ([]*types.FundInfo, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error
}

// MarketSource 市场数据源接口
type MarketSource interface {
	// Name 数据源名称
	Name() string

	// GetIndices 获取主要指数
	GetIndices(ctx context.Context, market types.Market) ([]*types.IndexInfo, error)

	// GetTopGainers 获取涨幅榜
	GetTopGainers(ctx context.Context, market types.Market, limit int) ([]*types.Quote, error)

	// GetTopLosers 获取跌幅榜
	GetTopLosers(ctx context.Context, market types.Market, limit int) ([]*types.Quote, error)

	// GetSectors 获取板块信息
	GetSectors(ctx context.Context) ([]*types.SectorInfo, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error
}

// SourceConfig 数据源配置
type SourceConfig struct {
	Name           string        `json:"name"`             // 数据源名称
	BaseURL        string        `json:"base_url"`         // 基础URL
	Timeout        time.Duration `json:"timeout"`          // 超时时间
	RetryCount     int           `json:"retry_count"`      // 重试次数
	RetryDelay     time.Duration `json:"retry_delay"`      // 重试间隔
	RateLimit      int           `json:"rate_limit"`       // 每秒请求数限制
	Token          string        `json:"token"`            // API Token (可选)
	EnableProxy    bool          `json:"enable_proxy"`     // 是否启用代理
	ProxyURL       string        `json:"proxy_url"`        // 代理地址
}

// DefaultConfig 默认配置
func DefaultConfig(name string) *SourceConfig {
	return &SourceConfig{
		Name:       name,
		Timeout:    10 * time.Second,
		RetryCount: 3,
		RetryDelay: time.Second,
		RateLimit:  10,
	}
}

// SourceStatus 数据源状态
type SourceStatus string

const (
	StatusHealthy   SourceStatus = "healthy"   // 健康
	StatusDegraded  SourceStatus = "degraded"  // 降级
	StatusUnhealthy SourceStatus = "unhealthy" // 不健康
)

// SourceHealth 数据源健康状态
type SourceHealth struct {
	Name           string        `json:"name"`            // 数据源名称
	Status         SourceStatus  `json:"status"`          // 状态
	LastCheck      time.Time     `json:"last_check"`      // 最后检查时间
	LastSuccess    time.Time     `json:"last_success"`    // 最后成功时间
	SuccessRate    float64       `json:"success_rate"`    // 成功率
	AvgLatency     time.Duration `json:"avg_latency"`     // 平均延迟
	ConsecutiveFails int         `json:"consecutive_fails"` // 连续失败次数
	ErrorMessage   string        `json:"error_message"`   // 错误信息
}

// QuoteRequest 行情请求
type QuoteRequest struct {
	Symbols   []string        `json:"symbols"`    // 股票代码列表
	AssetType types.AssetType `json:"asset_type"` // 资产类型
	Market    types.Market    `json:"market"`     // 市场
}

// KLineRequest K线请求
type KLineRequest struct {
	Symbol    string          `json:"symbol"`     // 股票代码
	Period    string          `json:"period"`     // 周期
	Count     int             `json:"count"`      // 数量
	StartDate time.Time       `json:"start_date"` // 开始日期
	EndDate   time.Time       `json:"end_date"`   // 结束日期
	AdjType   string          `json:"adj_type"`   // 复权类型: none/qfq/hfq
}
