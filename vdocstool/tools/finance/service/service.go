// Package service 财经服务核心逻辑
package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/vdocstool/tools/finance/cache"
	"github.com/cloudwego/eino/vdocstool/tools/finance/sources"
	"github.com/cloudwego/eino/vdocstool/tools/finance/sources/eastmoney"
	"github.com/cloudwego/eino/vdocstool/tools/finance/sources/sina"
	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// FinanceService 财经服务
type FinanceService struct {
	router *sources.Router
	cache  cache.Cache
	config *ServiceConfig
	mu     sync.RWMutex
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	RedisURL     string // Redis URL
	EnableCache  bool   // 启用缓存
	EnableRouter bool   // 启用智能路由
}

// DefaultServiceConfig 默认服务配置
func DefaultServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		EnableCache:  true,
		EnableRouter: true,
	}
}

// NewFinanceService 创建财经服务
func NewFinanceService(config *ServiceConfig) (*FinanceService, error) {
	if config == nil {
		config = DefaultServiceConfig()
	}

	// 创建路由器
	routerConfig := sources.DefaultRouterConfig()
	router := sources.NewRouter(routerConfig)

	// 注册数据源
	router.RegisterSource(eastmoney.New(nil))
	router.RegisterSource(sina.New(nil))

	// 创建缓存
	var cacheInstance cache.Cache
	if config.EnableCache {
		if config.RedisURL != "" {
			redisCache, err := cache.NewRedisCache(config.RedisURL, nil)
			if err != nil {
				// Redis 不可用,使用内存缓存
				cacheInstance = cache.NewMemoryCache(nil)
			} else {
				cacheInstance = redisCache
			}
		} else {
			cacheInstance = cache.NewMemoryCache(nil)
		}
	}

	svc := &FinanceService{
		router: router,
		cache:  cacheInstance,
		config: config,
	}

	// 启动路由器
	router.Start()

	return svc, nil
}

// Close 关闭服务
func (s *FinanceService) Close() error {
	s.router.Stop()
	if s.cache != nil {
		return s.cache.Close()
	}
	return nil
}

// GetQuote 获取实时行情
func (s *FinanceService) GetQuote(ctx context.Context, symbol string) (*types.Quote, error) {
	symbol = s.normalizeSymbol(symbol)

	// 尝试从缓存获取
	if s.cache != nil {
		quote, err := s.cache.GetQuote(ctx, symbol)
		if err == nil && quote != nil {
			return quote, nil
		}
	}

	// 从数据源获取
	quote, err := s.router.GetQuote(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("get quote failed: %w", err)
	}

	// 写入缓存
	if s.cache != nil {
		s.cache.SetQuote(ctx, quote)
	}

	return quote, nil
}

// GetQuotes 批量获取行情
func (s *FinanceService) GetQuotes(ctx context.Context, symbols []string) ([]*types.Quote, error) {
	// 标准化代码
	normalizedSymbols := make([]string, len(symbols))
	for i, sym := range symbols {
		normalizedSymbols[i] = s.normalizeSymbol(sym)
	}

	// 尝试从缓存获取
	var cachedQuotes map[string]*types.Quote
	var missedSymbols []string

	if s.cache != nil {
		cachedQuotes, _ = s.cache.GetQuotes(ctx, normalizedSymbols)
		for _, sym := range normalizedSymbols {
			if _, ok := cachedQuotes[sym]; !ok {
				missedSymbols = append(missedSymbols, sym)
			}
		}
	} else {
		missedSymbols = normalizedSymbols
	}

	// 合并结果
	result := make([]*types.Quote, 0, len(symbols))

	// 添加缓存命中的
	for _, q := range cachedQuotes {
		result = append(result, q)
	}

	// 从数据源获取未命中的
	if len(missedSymbols) > 0 {
		quotes, err := s.router.GetQuotes(ctx, missedSymbols)
		if err != nil {
			// 如果有部分缓存命中,不返回错误
			if len(result) > 0 {
				return result, nil
			}
			return nil, fmt.Errorf("get quotes failed: %w", err)
		}

		// 写入缓存
		if s.cache != nil {
			s.cache.SetQuotes(ctx, quotes)
		}

		result = append(result, quotes...)
	}

	return result, nil
}

// GetKLine 获取K线数据
func (s *FinanceService) GetKLine(ctx context.Context, symbol string, period string, count int) ([]*types.KLine, error) {
	symbol = s.normalizeSymbol(symbol)
	period = s.normalizePeriod(period)

	if count <= 0 {
		count = 100
	}
	if count > 1000 {
		count = 1000
	}

	// 尝试从缓存获取
	if s.cache != nil {
		klines, err := s.cache.GetKLine(ctx, symbol, period, count)
		if err == nil && len(klines) > 0 {
			return klines, nil
		}
	}

	// 从数据源获取
	klines, err := s.router.GetKLine(ctx, symbol, period, count)
	if err != nil {
		return nil, fmt.Errorf("get kline failed: %w", err)
	}

	// 写入缓存
	if s.cache != nil {
		s.cache.SetKLine(ctx, symbol, period, klines)
	}

	return klines, nil
}

// Search 搜索股票/基金
func (s *FinanceService) Search(ctx context.Context, keyword string) ([]*types.SearchResult, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, fmt.Errorf("keyword is required")
	}

	// 尝试从缓存获取
	if s.cache != nil {
		results, err := s.cache.GetSearchResult(ctx, keyword)
		if err == nil && len(results) > 0 {
			return results, nil
		}
	}

	// 从数据源获取
	results, err := s.router.Search(ctx, keyword)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 写入缓存
	if s.cache != nil {
		s.cache.SetSearchResult(ctx, keyword, results)
	}

	return results, nil
}

// GetSourceHealth 获取数据源健康状态
func (s *FinanceService) GetSourceHealth() map[string]*sources.SourceHealth {
	return s.router.GetAllSourceHealth()
}

// GetRouterStats 获取路由器统计
func (s *FinanceService) GetRouterStats() *sources.RouterStats {
	return s.router.GetStats()
}

// normalizeSymbol 标准化股票代码
func (s *FinanceService) normalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// 移除市场后缀
	suffixes := []string{".SH", ".SZ", ".BJ", ".HK", ".US", ".SS", ".SZ"}
	for _, suffix := range suffixes {
		symbol = strings.TrimSuffix(symbol, suffix)
	}

	return symbol
}

// normalizePeriod 标准化周期
func (s *FinanceService) normalizePeriod(period string) string {
	period = strings.ToLower(strings.TrimSpace(period))

	switch period {
	case "1", "1min", "1分钟":
		return "1m"
	case "5", "5min", "5分钟":
		return "5m"
	case "15", "15min", "15分钟":
		return "15m"
	case "30", "30min", "30分钟":
		return "30m"
	case "60", "60min", "1h", "1hour", "1小时":
		return "60m"
	case "d", "day", "daily", "日", "日线":
		return "1d"
	case "w", "week", "weekly", "周", "周线":
		return "1w"
	case "m", "month", "monthly", "月", "月线":
		return "1M"
	default:
		if period == "" {
			return "1d"
		}
		return period
	}
}

// ParseSymbol 解析股票代码
func (s *FinanceService) ParseSymbol(input string) (symbol string, market types.Market, assetType types.AssetType) {
	input = strings.TrimSpace(input)
	upper := strings.ToUpper(input)

	// 检查是否带市场后缀
	if strings.HasSuffix(upper, ".SH") {
		symbol = strings.TrimSuffix(upper, ".SH")
		market = types.MarketSH
	} else if strings.HasSuffix(upper, ".SZ") {
		symbol = strings.TrimSuffix(upper, ".SZ")
		market = types.MarketSZ
	} else if strings.HasSuffix(upper, ".HK") {
		symbol = strings.TrimSuffix(upper, ".HK")
		market = types.MarketHK
	} else if strings.HasSuffix(upper, ".US") {
		symbol = strings.TrimSuffix(upper, ".US")
		market = types.MarketUS
	} else {
		symbol = upper
		market = s.detectMarket(symbol)
	}

	assetType = s.detectAssetType(symbol)
	return
}

// detectMarket 检测市场
func (s *FinanceService) detectMarket(symbol string) types.Market {
	// 纯字母 - 美股
	if isAllLetters(symbol) {
		return types.MarketUS
	}

	// 5位数字开头为0 - 港股
	if len(symbol) == 5 && symbol[0] == '0' {
		return types.MarketHK
	}

	// 6位数字 - A股
	if len(symbol) == 6 && isAllDigits(symbol) {
		switch {
		case strings.HasPrefix(symbol, "6"), strings.HasPrefix(symbol, "5"):
			return types.MarketSH
		case strings.HasPrefix(symbol, "0"), strings.HasPrefix(symbol, "3"), strings.HasPrefix(symbol, "1"):
			return types.MarketSZ
		case strings.HasPrefix(symbol, "4"), strings.HasPrefix(symbol, "8"):
			return types.MarketBJ
		}
	}

	return types.MarketCN
}

// detectAssetType 检测资产类型
func (s *FinanceService) detectAssetType(symbol string) types.AssetType {
	if len(symbol) == 6 {
		switch {
		case strings.HasPrefix(symbol, "51"), strings.HasPrefix(symbol, "15"),
			strings.HasPrefix(symbol, "56"), strings.HasPrefix(symbol, "16"),
			strings.HasPrefix(symbol, "58"), strings.HasPrefix(symbol, "159"):
			return types.AssetTypeETF
		case strings.HasPrefix(symbol, "000"), strings.HasPrefix(symbol, "399"):
			return types.AssetTypeIndex
		}
	}

	return types.AssetTypeStock
}

// 辅助函数
func isAllLetters(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return len(s) > 0
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
