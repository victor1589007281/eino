// Package sina 新浪财经数据源
package sina

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/sources"
	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

const (
	// API 地址
	quoteAPI  = "https://hq.sinajs.cn/list="
	klineAPI  = "https://quotes.sina.cn/cn/api/jsonp_v2.php/var%%20_%s=/CN_MarketDataService.getKLineData"
	searchAPI = "https://suggest3.sinajs.cn/suggest/type=&key="
)

// Source 新浪财经数据源
type Source struct {
	client *http.Client
	config *sources.SourceConfig
}

// New 创建新浪财经数据源
func New(config *sources.SourceConfig) *Source {
	if config == nil {
		config = sources.DefaultConfig("sina")
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
	return "sina"
}

// Priority 优先级
func (s *Source) Priority() int {
	return 2 // 作为备份数据源
}

// SupportedAssets 支持的资产类型
func (s *Source) SupportedAssets() []types.AssetType {
	return []types.AssetType{
		types.AssetTypeStock,
		types.AssetTypeETF,
		types.AssetTypeIndex,
	}
}

// SupportedMarkets 支持的市场
func (s *Source) SupportedMarkets() []types.Market {
	return []types.Market{
		types.MarketSH,
		types.MarketSZ,
		types.MarketHK,
		types.MarketUS,
	}
}

// GetQuote 获取实时行情
func (s *Source) GetQuote(ctx context.Context, symbol string) (*types.Quote, error) {
	sinaCode := s.toSinaCode(symbol)
	if sinaCode == "" {
		return nil, fmt.Errorf("invalid symbol: %s", symbol)
	}

	reqURL := quoteAPI + sinaCode

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://finance.sina.com.cn/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	// GBK转UTF-8
	body, _ = sources.GBKToUTF8(body)

	return s.parseQuoteResponse(string(body), symbol)
}

// GetQuotes 批量获取行情
func (s *Source) GetQuotes(ctx context.Context, symbols []string) ([]*types.Quote, error) {
	if len(symbols) == 0 {
		return nil, nil
	}

	// 转换为新浪代码
	sinaCodes := make([]string, 0, len(symbols))
	codeMap := make(map[string]string) // sinaCode -> symbol
	for _, sym := range symbols {
		sinaCode := s.toSinaCode(sym)
		if sinaCode != "" {
			sinaCodes = append(sinaCodes, sinaCode)
			codeMap[sinaCode] = sym
		}
	}

	if len(sinaCodes) == 0 {
		return nil, fmt.Errorf("no valid symbols")
	}

	reqURL := quoteAPI + strings.Join(sinaCodes, ",")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://finance.sina.com.cn/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	// GBK转UTF-8
	body, _ = sources.GBKToUTF8(body)

	return s.parseQuotesResponse(string(body), codeMap)
}

// GetKLine 获取K线数据
func (s *Source) GetKLine(ctx context.Context, symbol string, period string, count int) ([]*types.KLine, error) {
	sinaCode := s.toSinaCode(symbol)
	if sinaCode == "" {
		return nil, fmt.Errorf("invalid symbol: %s", symbol)
	}

	scale := s.periodToScale(period)
	
	// 构建请求URL
	callback := fmt.Sprintf("_%s_%d", symbol, time.Now().UnixNano())
	params := fmt.Sprintf("symbol=%s&scale=%s&ma=no&datalen=%d", sinaCode, scale, count)
	reqURL := fmt.Sprintf(klineAPI, callback) + "?" + params

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://finance.sina.com.cn/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	// GBK转UTF-8
	body, _ = sources.GBKToUTF8(body)

	return s.parseKLineResponse(string(body), symbol, period)
}

// Search 搜索股票/基金
func (s *Source) Search(ctx context.Context, keyword string) ([]*types.SearchResult, error) {
	reqURL := searchAPI + keyword

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://finance.sina.com.cn/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	// GBK转UTF-8
	body, _ = sources.GBKToUTF8(body)

	return s.parseSearchResponse(string(body))
}

// HealthCheck 健康检查
func (s *Source) HealthCheck(ctx context.Context) error {
	_, err := s.GetQuote(ctx, "sh000001") // 上证指数
	return err
}

// toSinaCode 转换为新浪代码格式
func (s *Source) toSinaCode(symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	
	// 移除可能的市场后缀
	symbol = strings.TrimSuffix(symbol, ".SH")
	symbol = strings.TrimSuffix(symbol, ".SZ")
	symbol = strings.TrimSuffix(symbol, ".BJ")
	symbol = strings.TrimSuffix(symbol, ".HK")
	symbol = strings.TrimSuffix(symbol, ".US")

	// 美股
	if matched, _ := regexp.MatchString(`^[A-Z]+$`, symbol); matched {
		return "gb_" + strings.ToLower(symbol)
	}

	// 港股
	if len(symbol) == 5 && symbol[0] == '0' {
		return "hk" + symbol
	}

	// A股
	if len(symbol) == 6 {
		switch {
		case strings.HasPrefix(symbol, "6"), strings.HasPrefix(symbol, "5"):
			return "sh" + symbol
		case strings.HasPrefix(symbol, "0"), strings.HasPrefix(symbol, "3"),
			strings.HasPrefix(symbol, "1"), strings.HasPrefix(symbol, "4"),
			strings.HasPrefix(symbol, "8"):
			return "sz" + symbol
		}
	}

	// 已经带市场前缀
	if strings.HasPrefix(strings.ToLower(symbol), "sh") || 
		strings.HasPrefix(strings.ToLower(symbol), "sz") {
		return strings.ToLower(symbol)
	}

	return ""
}

// periodToScale 周期转换
func (s *Source) periodToScale(period string) string {
	switch period {
	case "5m":
		return "5"
	case "15m":
		return "15"
	case "30m":
		return "30"
	case "60m", "1h":
		return "60"
	case "1d", "day":
		return "240"
	case "1w", "week":
		return "1680"
	default:
		return "240"
	}
}

// parseQuoteResponse 解析单个行情响应
func (s *Source) parseQuoteResponse(body string, symbol string) (*types.Quote, error) {
	// 新浪返回格式: var hq_str_sh600519="贵州茅台,1835.00,1833.00,..."
	re := regexp.MustCompile(`var hq_str_[a-z]+\d+="([^"]*)"`)
	matches := re.FindStringSubmatch(body)
	if len(matches) < 2 || matches[1] == "" {
		return nil, fmt.Errorf("no data returned for %s", symbol)
	}

	parts := strings.Split(matches[1], ",")
	market := s.detectMarket(symbol)

	// A股格式
	if market == types.MarketSH || market == types.MarketSZ || market == types.MarketBJ {
		return s.parseAShareQuote(parts, symbol)
	}

	// 港股格式
	if market == types.MarketHK {
		return s.parseHKQuote(parts, symbol)
	}

	// 美股格式
	if market == types.MarketUS {
		return s.parseUSQuote(parts, symbol)
	}

	return nil, fmt.Errorf("unsupported market for %s", symbol)
}

// parseAShareQuote 解析A股行情
func (s *Source) parseAShareQuote(parts []string, symbol string) (*types.Quote, error) {
	if len(parts) < 32 {
		return nil, fmt.Errorf("invalid A-share data format")
	}

	quote := &types.Quote{
		Symbol:     symbol,
		Source:     s.Name(),
		Market:     s.detectMarket(symbol),
		AssetType:  s.detectAssetType(symbol),
		UpdateTime: time.Now(),
	}

	quote.Name = parts[0]
	quote.Open, _ = strconv.ParseFloat(parts[1], 64)
	quote.PreClose, _ = strconv.ParseFloat(parts[2], 64)
	quote.Price, _ = strconv.ParseFloat(parts[3], 64)
	quote.High, _ = strconv.ParseFloat(parts[4], 64)
	quote.Low, _ = strconv.ParseFloat(parts[5], 64)
	quote.Bid1, _ = strconv.ParseFloat(parts[6], 64)
	quote.Ask1, _ = strconv.ParseFloat(parts[7], 64)
	vol, _ := strconv.ParseFloat(parts[8], 64)
	quote.Volume = int64(vol / 100) // 转换为手
	quote.Amount, _ = strconv.ParseFloat(parts[9], 64)

	// 计算涨跌
	if quote.PreClose > 0 {
		quote.Change = quote.Price - quote.PreClose
		quote.ChangePct = quote.Change / quote.PreClose * 100
	}

	return quote, nil
}

// parseHKQuote 解析港股行情
func (s *Source) parseHKQuote(parts []string, symbol string) (*types.Quote, error) {
	if len(parts) < 18 {
		return nil, fmt.Errorf("invalid HK data format")
	}

	quote := &types.Quote{
		Symbol:     symbol,
		Source:     s.Name(),
		Market:     types.MarketHK,
		AssetType:  types.AssetTypeStock,
		UpdateTime: time.Now(),
	}

	// 港股格式: 名称(英文),名称(繁体),名称(简体),今开,昨收,日高,日低,最新价,涨跌额,涨跌幅,...
	quote.Name = parts[1] // 繁体名
	if len(parts) > 2 && parts[2] != "" {
		quote.Name = parts[2] // 简体名
	}
	quote.Open, _ = strconv.ParseFloat(parts[3], 64)
	quote.PreClose, _ = strconv.ParseFloat(parts[4], 64)
	quote.High, _ = strconv.ParseFloat(parts[5], 64)
	quote.Low, _ = strconv.ParseFloat(parts[6], 64)
	quote.Price, _ = strconv.ParseFloat(parts[7], 64)
	quote.Change, _ = strconv.ParseFloat(parts[8], 64)
	quote.ChangePct, _ = strconv.ParseFloat(parts[9], 64)

	if len(parts) > 12 {
		vol, _ := strconv.ParseFloat(parts[12], 64)
		quote.Volume = int64(vol)
	}
	if len(parts) > 13 {
		quote.Amount, _ = strconv.ParseFloat(parts[13], 64)
	}

	return quote, nil
}

// parseUSQuote 解析美股行情
func (s *Source) parseUSQuote(parts []string, symbol string) (*types.Quote, error) {
	if len(parts) < 27 {
		return nil, fmt.Errorf("invalid US data format")
	}

	quote := &types.Quote{
		Symbol:     symbol,
		Source:     s.Name(),
		Market:     types.MarketUS,
		AssetType:  types.AssetTypeStock,
		UpdateTime: time.Now(),
	}

	quote.Name = parts[0]
	quote.Price, _ = strconv.ParseFloat(parts[1], 64)
	quote.Change, _ = strconv.ParseFloat(parts[2], 64)
	quote.ChangePct, _ = strconv.ParseFloat(parts[3], 64)
	// 解析时间
	// timeStr := parts[3] + " " + parts[4]
	quote.PreClose, _ = strconv.ParseFloat(parts[26], 64)
	quote.Open, _ = strconv.ParseFloat(parts[5], 64)
	quote.High, _ = strconv.ParseFloat(parts[6], 64)
	quote.Low, _ = strconv.ParseFloat(parts[7], 64)

	vol, _ := strconv.ParseFloat(parts[10], 64)
	quote.Volume = int64(vol)

	return quote, nil
}

// parseQuotesResponse 解析批量行情响应
func (s *Source) parseQuotesResponse(body string, codeMap map[string]string) ([]*types.Quote, error) {
	// 匹配所有行情数据
	re := regexp.MustCompile(`var hq_str_([a-z]+\d+)="([^"]*)"`)
	matches := re.FindAllStringSubmatch(body, -1)

	quotes := make([]*types.Quote, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 || match[2] == "" {
			continue
		}

		sinaCode := match[1]
		symbol, ok := codeMap[sinaCode]
		if !ok {
			// 从代码提取
			symbol = strings.TrimPrefix(sinaCode, "sh")
			symbol = strings.TrimPrefix(symbol, "sz")
			symbol = strings.TrimPrefix(symbol, "hk")
			symbol = strings.TrimPrefix(symbol, "gb_")
		}

		parts := strings.Split(match[2], ",")
		market := s.detectMarket(sinaCode)

		var quote *types.Quote
		var err error

		switch market {
		case types.MarketSH, types.MarketSZ, types.MarketBJ:
			quote, err = s.parseAShareQuote(parts, symbol)
		case types.MarketHK:
			quote, err = s.parseHKQuote(parts, symbol)
		case types.MarketUS:
			quote, err = s.parseUSQuote(parts, symbol)
		}

		if err == nil && quote != nil {
			quotes = append(quotes, quote)
		}
	}

	return quotes, nil
}

// parseKLineResponse 解析K线响应
func (s *Source) parseKLineResponse(body string, symbol string, period string) ([]*types.KLine, error) {
	// 提取JSON部分
	start := strings.Index(body, "[")
	end := strings.LastIndex(body, "]")
	if start == -1 || end == -1 || start >= end {
		return nil, fmt.Errorf("invalid kline response format")
	}

	// 简单解析K线数据
	// 格式: [{day:"2024-01-01",open:"100.00",high:"101.00",low:"99.00",close:"100.50",volume:"10000"},...]
	data := body[start : end+1]
	
	klines := make([]*types.KLine, 0)
	
	// 使用正则提取每条K线
	re := regexp.MustCompile(`\{day:"([^"]+)",open:"([^"]+)",high:"([^"]+)",low:"([^"]+)",close:"([^"]+)",volume:"([^"]+)"`)
	matches := re.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) < 7 {
			continue
		}

		kline := &types.KLine{
			Symbol: symbol,
			Period: period,
		}

		if t, err := time.Parse("2006-01-02", match[1]); err == nil {
			kline.Timestamp = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", match[1]); err == nil {
			kline.Timestamp = t
		}

		kline.Open, _ = strconv.ParseFloat(match[2], 64)
		kline.High, _ = strconv.ParseFloat(match[3], 64)
		kline.Low, _ = strconv.ParseFloat(match[4], 64)
		kline.Close, _ = strconv.ParseFloat(match[5], 64)
		vol, _ := strconv.ParseFloat(match[6], 64)
		kline.Volume = int64(vol)

		klines = append(klines, kline)
	}

	return klines, nil
}

// parseSearchResponse 解析搜索响应
func (s *Source) parseSearchResponse(body string) ([]*types.SearchResult, error) {
	// 格式: var suggestvalue="股票代码,股票名称,交易所,...;..."
	re := regexp.MustCompile(`var suggestvalue="([^"]*)"`)
	matches := re.FindStringSubmatch(body)
	if len(matches) < 2 || matches[1] == "" {
		return nil, nil
	}

	items := strings.Split(matches[1], ";")
	results := make([]*types.SearchResult, 0, len(items))

	for _, item := range items {
		parts := strings.Split(item, ",")
		if len(parts) < 4 {
			continue
		}

		sr := &types.SearchResult{
			Symbol: parts[2],
			Name:   parts[4],
		}

		// 判断市场
		switch parts[1] {
		case "11":
			sr.Market = types.MarketSH
			sr.Exchange = "上海证券交易所"
		case "22":
			sr.Market = types.MarketSZ
			sr.Exchange = "深圳证券交易所"
		case "31":
			sr.Market = types.MarketHK
			sr.Exchange = "香港交易所"
		case "41":
			sr.Market = types.MarketUS
			sr.Exchange = "美国"
		}

		sr.AssetType = s.detectAssetType(sr.Symbol)
		results = append(results, sr)
	}

	return results, nil
}

// detectMarket 检测市场
func (s *Source) detectMarket(code string) types.Market {
	code = strings.ToLower(code)
	
	if strings.HasPrefix(code, "sh") {
		return types.MarketSH
	}
	if strings.HasPrefix(code, "sz") {
		return types.MarketSZ
	}
	if strings.HasPrefix(code, "hk") {
		return types.MarketHK
	}
	if strings.HasPrefix(code, "gb_") {
		return types.MarketUS
	}

	// 纯数字代码
	code = strings.ToUpper(code)
	if len(code) == 6 {
		switch {
		case strings.HasPrefix(code, "6"), strings.HasPrefix(code, "5"):
			return types.MarketSH
		case strings.HasPrefix(code, "0"), strings.HasPrefix(code, "3"), strings.HasPrefix(code, "1"):
			return types.MarketSZ
		case strings.HasPrefix(code, "4"), strings.HasPrefix(code, "8"):
			return types.MarketBJ
		}
	}

	return types.MarketCN
}

// detectAssetType 检测资产类型
func (s *Source) detectAssetType(symbol string) types.AssetType {
	symbol = strings.ToUpper(symbol)
	symbol = strings.TrimPrefix(strings.ToLower(symbol), "sh")
	symbol = strings.TrimPrefix(symbol, "sz")
	symbol = strings.ToUpper(symbol)

	if len(symbol) == 6 {
		switch {
		case strings.HasPrefix(symbol, "51"), strings.HasPrefix(symbol, "15"),
			strings.HasPrefix(symbol, "56"), strings.HasPrefix(symbol, "16"):
			return types.AssetTypeETF
		case strings.HasPrefix(symbol, "000"), strings.HasPrefix(symbol, "399"):
			return types.AssetTypeIndex
		}
	}

	return types.AssetTypeStock
}

// 确保实现接口
var _ sources.DataSource = (*Source)(nil)
