// Package eastmoney 东方财富数据源
package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/sources"
	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

const (
	// API 地址
	quoteAPI     = "https://push2.eastmoney.com/api/qt/stock/get"
	quotesAPI    = "https://push2.eastmoney.com/api/qt/ulist.np/get"
	klineAPI     = "https://push2his.eastmoney.com/api/qt/stock/kline/get"
	searchAPI    = "https://searchapi.eastmoney.com/api/suggest/get"
	indicesAPI   = "https://push2.eastmoney.com/api/qt/ulist.np/get"
	
	// 字段映射
	quoteFields = "f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13,f14,f15,f16,f17,f18,f19,f20,f21,f22,f23,f24,f25"
)

// Source 东方财富数据源
type Source struct {
	client  *http.Client
	config  *sources.SourceConfig
}

// New 创建东方财富数据源
func New(config *sources.SourceConfig) *Source {
	if config == nil {
		config = sources.DefaultConfig("eastmoney")
		config.BaseURL = "https://push2.eastmoney.com"
	}

	return &Source{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		config: config,
	}
}

// Name 数据源名称
func (s *Source) Name() string {
	return "eastmoney"
}

// Priority 优先级
func (s *Source) Priority() int {
	return 1
}

// SupportedAssets 支持的资产类型
func (s *Source) SupportedAssets() []types.AssetType {
	return []types.AssetType{
		types.AssetTypeStock,
		types.AssetTypeETF,
		types.AssetTypeIndex,
		types.AssetTypeFund,
	}
}

// SupportedMarkets 支持的市场
func (s *Source) SupportedMarkets() []types.Market {
	return []types.Market{
		types.MarketSH,
		types.MarketSZ,
		types.MarketBJ,
		types.MarketHK,
		types.MarketUS,
	}
}

// GetQuote 获取实时行情
func (s *Source) GetQuote(ctx context.Context, symbol string) (*types.Quote, error) {
	secid := s.toSecID(symbol)
	if secid == "" {
		return nil, fmt.Errorf("invalid symbol: %s", symbol)
	}

	params := url.Values{}
	params.Set("secid", secid)
	params.Set("fields", quoteFields)
	params.Set("ut", "fa5fd1943c7b386f172d6893dbfba10b")

	reqURL := fmt.Sprintf("%s?%s", quoteAPI, params.Encode())
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	return s.parseQuoteResponse(body, symbol)
}

// GetQuotes 批量获取行情
func (s *Source) GetQuotes(ctx context.Context, symbols []string) ([]*types.Quote, error) {
	if len(symbols) == 0 {
		return nil, nil
	}

	// 转换为 secid 列表
	secids := make([]string, 0, len(symbols))
	symbolMap := make(map[string]string)
	for _, sym := range symbols {
		secid := s.toSecID(sym)
		if secid != "" {
			secids = append(secids, secid)
			symbolMap[secid] = sym
		}
	}

	if len(secids) == 0 {
		return nil, fmt.Errorf("no valid symbols")
	}

	params := url.Values{}
	params.Set("fltt", "2")
	params.Set("secids", strings.Join(secids, ","))
	params.Set("fields", "f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f12,f13,f14,f15,f16,f17,f18")
	params.Set("ut", "fa5fd1943c7b386f172d6893dbfba10b")

	reqURL := fmt.Sprintf("%s?%s", quotesAPI, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	return s.parseQuotesResponse(body, symbolMap)
}

// GetKLine 获取K线数据
func (s *Source) GetKLine(ctx context.Context, symbol string, period string, count int) ([]*types.KLine, error) {
	secid := s.toSecID(symbol)
	if secid == "" {
		return nil, fmt.Errorf("invalid symbol: %s", symbol)
	}

	klt := s.periodToKlt(period)
	
	params := url.Values{}
	params.Set("secid", secid)
	params.Set("fields1", "f1,f2,f3,f4,f5,f6")
	params.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61")
	params.Set("klt", klt)
	params.Set("fqt", "1") // 前复权
	params.Set("lmt", strconv.Itoa(count))
	params.Set("end", "20500101")
	params.Set("ut", "fa5fd1943c7b386f172d6893dbfba10b")

	reqURL := fmt.Sprintf("%s?%s", klineAPI, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	return s.parseKLineResponse(body, symbol, period)
}

// Search 搜索股票/基金
func (s *Source) Search(ctx context.Context, keyword string) ([]*types.SearchResult, error) {
	params := url.Values{}
	params.Set("input", keyword)
	params.Set("type", "14")
	params.Set("token", "D43BF722C8E33BDC906FB84D85E326E8")
	params.Set("count", "20")

	reqURL := fmt.Sprintf("%s?%s", searchAPI, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	return s.parseSearchResponse(body)
}

// HealthCheck 健康检查
func (s *Source) HealthCheck(ctx context.Context) error {
	// 使用上证指数进行健康检查
	_, err := s.GetQuote(ctx, "000001")
	return err
}

// toSecID 转换股票代码为东方财富格式
// 格式: market.code, 如 1.600519 (上证), 0.000001 (深证), 116.00700 (港股), 105.AAPL (美股)
func (s *Source) toSecID(symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	
	// 移除可能的市场后缀
	symbol = strings.TrimSuffix(symbol, ".SH")
	symbol = strings.TrimSuffix(symbol, ".SZ")
	symbol = strings.TrimSuffix(symbol, ".BJ")
	symbol = strings.TrimSuffix(symbol, ".HK")
	symbol = strings.TrimSuffix(symbol, ".US")
	
	// 判断市场
	if matched, _ := regexp.MatchString(`^[A-Z]+$`, symbol); matched {
		// 美股
		return "105." + symbol
	}
	
	if strings.HasSuffix(symbol, ".HK") || (len(symbol) == 5 && symbol[0] == '0') {
		// 港股
		return "116." + strings.TrimPrefix(symbol, "0")
	}
	
	if len(symbol) == 6 {
		code := symbol
		switch {
		case strings.HasPrefix(code, "6"):
			// 上证
			return "1." + code
		case strings.HasPrefix(code, "00"), strings.HasPrefix(code, "30"):
			// 深证主板和创业板
			return "0." + code
		case strings.HasPrefix(code, "4"), strings.HasPrefix(code, "8"):
			// 北交所
			return "0." + code
		case strings.HasPrefix(code, "51"), strings.HasPrefix(code, "56"), strings.HasPrefix(code, "58"):
			// 上证 ETF
			return "1." + code
		case strings.HasPrefix(code, "15"), strings.HasPrefix(code, "16"):
			// 深证 ETF
			return "0." + code
		default:
			// 默认上证
			return "1." + code
		}
	}
	
	return ""
}

// periodToKlt 周期转换
func (s *Source) periodToKlt(period string) string {
	switch period {
	case "1m":
		return "1"
	case "5m":
		return "5"
	case "15m":
		return "15"
	case "30m":
		return "30"
	case "60m", "1h":
		return "60"
	case "1d", "day":
		return "101"
	case "1w", "week":
		return "102"
	case "1M", "month":
		return "103"
	default:
		return "101" // 默认日K
	}
}

// parseQuoteResponse 解析单个行情响应
func (s *Source) parseQuoteResponse(body []byte, symbol string) (*types.Quote, error) {
	var result struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse json failed: %w", err)
	}

	if result.Data == nil {
		return nil, fmt.Errorf("no data returned")
	}

	quote := &types.Quote{
		Symbol:     symbol,
		Source:     s.Name(),
		UpdateTime: time.Now(),
	}

	// 解析字段
	if v, ok := result.Data["f14"].(string); ok {
		quote.Name = v
	}
	if v, ok := result.Data["f2"].(float64); ok {
		quote.Price = v
	}
	if v, ok := result.Data["f17"].(float64); ok {
		quote.Open = v
	}
	if v, ok := result.Data["f15"].(float64); ok {
		quote.High = v
	}
	if v, ok := result.Data["f16"].(float64); ok {
		quote.Low = v
	}
	if v, ok := result.Data["f18"].(float64); ok {
		quote.PreClose = v
	}
	if v, ok := result.Data["f4"].(float64); ok {
		quote.Change = v
	}
	if v, ok := result.Data["f3"].(float64); ok {
		quote.ChangePct = v
	}
	if v, ok := result.Data["f5"].(float64); ok {
		quote.Volume = int64(v)
	}
	if v, ok := result.Data["f6"].(float64); ok {
		quote.Amount = v
	}
	if v, ok := result.Data["f8"].(float64); ok {
		quote.TurnoverRate = v
	}
	if v, ok := result.Data["f20"].(float64); ok {
		quote.MarketCap = v
	}
	if v, ok := result.Data["f9"].(float64); ok {
		quote.PERatio = v
	}
	if v, ok := result.Data["f23"].(float64); ok {
		quote.PBRatio = v
	}

	// 判断市场和类型
	quote.Market = s.detectMarket(symbol)
	quote.AssetType = s.detectAssetType(symbol)

	return quote, nil
}

// parseQuotesResponse 解析批量行情响应
func (s *Source) parseQuotesResponse(body []byte, symbolMap map[string]string) ([]*types.Quote, error) {
	var result struct {
		Data struct {
			Diff []map[string]interface{} `json:"diff"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse json failed: %w", err)
	}

	quotes := make([]*types.Quote, 0, len(result.Data.Diff))
	for _, item := range result.Data.Diff {
		quote := &types.Quote{
			Source:     s.Name(),
			UpdateTime: time.Now(),
		}

		// 获取 secid
		market := ""
		code := ""
		if v, ok := item["f13"].(float64); ok {
			market = strconv.Itoa(int(v))
		}
		if v, ok := item["f12"].(string); ok {
			code = v
		}
		secid := market + "." + code
		if sym, ok := symbolMap[secid]; ok {
			quote.Symbol = sym
		} else {
			quote.Symbol = code
		}

		if v, ok := item["f14"].(string); ok {
			quote.Name = v
		}
		if v, ok := item["f2"].(float64); ok {
			quote.Price = v
		}
		if v, ok := item["f17"].(float64); ok {
			quote.Open = v
		}
		if v, ok := item["f15"].(float64); ok {
			quote.High = v
		}
		if v, ok := item["f16"].(float64); ok {
			quote.Low = v
		}
		if v, ok := item["f18"].(float64); ok {
			quote.PreClose = v
		}
		if v, ok := item["f4"].(float64); ok {
			quote.Change = v
		}
		if v, ok := item["f3"].(float64); ok {
			quote.ChangePct = v
		}
		if v, ok := item["f5"].(float64); ok {
			quote.Volume = int64(v)
		}
		if v, ok := item["f6"].(float64); ok {
			quote.Amount = v
		}

		quote.Market = s.detectMarket(quote.Symbol)
		quote.AssetType = s.detectAssetType(quote.Symbol)

		quotes = append(quotes, quote)
	}

	return quotes, nil
}

// parseKLineResponse 解析K线响应
func (s *Source) parseKLineResponse(body []byte, symbol string, period string) ([]*types.KLine, error) {
	var result struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse json failed: %w", err)
	}

	klines := make([]*types.KLine, 0, len(result.Data.Klines))
	for _, line := range result.Data.Klines {
		parts := strings.Split(line, ",")
		if len(parts) < 7 {
			continue
		}

		kline := &types.KLine{
			Symbol: symbol,
			Period: period,
		}

		// 解析时间
		if t, err := time.Parse("2006-01-02", parts[0]); err == nil {
			kline.Timestamp = t
		} else if t, err := time.Parse("2006-01-02 15:04", parts[0]); err == nil {
			kline.Timestamp = t
		}

		// 解析价格
		if v, err := strconv.ParseFloat(parts[1], 64); err == nil {
			kline.Open = v
		}
		if v, err := strconv.ParseFloat(parts[2], 64); err == nil {
			kline.Close = v
		}
		if v, err := strconv.ParseFloat(parts[3], 64); err == nil {
			kline.High = v
		}
		if v, err := strconv.ParseFloat(parts[4], 64); err == nil {
			kline.Low = v
		}
		if v, err := strconv.ParseFloat(parts[5], 64); err == nil {
			kline.Volume = int64(v)
		}
		if v, err := strconv.ParseFloat(parts[6], 64); err == nil {
			kline.Amount = v
		}

		klines = append(klines, kline)
	}

	return klines, nil
}

// parseSearchResponse 解析搜索响应
func (s *Source) parseSearchResponse(body []byte) ([]*types.SearchResult, error) {
	var result struct {
		QuotationCodeTable struct {
			Data []struct {
				Code       string `json:"Code"`
				Name       string `json:"Name"`
				MarketType string `json:"MktNum"`
				SecurityType string `json:"SecurityType"`
			} `json:"Data"`
		} `json:"QuotationCodeTable"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse json failed: %w", err)
	}

	results := make([]*types.SearchResult, 0, len(result.QuotationCodeTable.Data))
	for _, item := range result.QuotationCodeTable.Data {
		sr := &types.SearchResult{
			Symbol: item.Code,
			Name:   item.Name,
		}

		// 判断市场
		switch item.MarketType {
		case "1":
			sr.Market = types.MarketSH
			sr.Exchange = "上海证券交易所"
		case "2":
			sr.Market = types.MarketSZ
			sr.Exchange = "深圳证券交易所"
		case "116":
			sr.Market = types.MarketHK
			sr.Exchange = "香港交易所"
		case "105":
			sr.Market = types.MarketUS
			sr.Exchange = "美国"
		}

		// 判断类型
		sr.AssetType = s.detectAssetType(item.Code)

		results = append(results, sr)
	}

	return results, nil
}

// detectMarket 检测市场
func (s *Source) detectMarket(symbol string) types.Market {
	symbol = strings.ToUpper(symbol)
	
	if matched, _ := regexp.MatchString(`^[A-Z]+$`, symbol); matched {
		return types.MarketUS
	}
	
	if len(symbol) == 5 && symbol[0] == '0' {
		return types.MarketHK
	}
	
	if len(symbol) == 6 {
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
func (s *Source) detectAssetType(symbol string) types.AssetType {
	symbol = strings.ToUpper(symbol)
	
	if len(symbol) == 6 {
		switch {
		case strings.HasPrefix(symbol, "51"), strings.HasPrefix(symbol, "15"),
			strings.HasPrefix(symbol, "56"), strings.HasPrefix(symbol, "16"),
			strings.HasPrefix(symbol, "58"):
			return types.AssetTypeETF
		case strings.HasPrefix(symbol, "000"), strings.HasPrefix(symbol, "399"):
			return types.AssetTypeIndex
		}
	}
	
	return types.AssetTypeStock
}

// 确保实现接口
var _ sources.DataSource = (*Source)(nil)
