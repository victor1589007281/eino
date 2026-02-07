// Package market 市场概览服务
package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// Service 市场服务
type Service struct {
	client *http.Client
	cache  sync.Map // 缓存
}

// NewService 创建市场服务
func NewService() *Service {
	return &Service{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetMarketOverview 获取市场概览
func (s *Service) GetMarketOverview(ctx context.Context, market types.Market) (*types.MarketOverview, error) {
	// 检查缓存
	cacheKey := fmt.Sprintf("overview_%s", market)
	if cached, ok := s.cache.Load(cacheKey); ok {
		if overview, ok := cached.(*cachedOverview); ok {
			if time.Since(overview.time) < 30*time.Second {
				return overview.data, nil
			}
		}
	}

	overview := &types.MarketOverview{
		Time:       time.Now(),
		Market:     market,
		UpdateTime: time.Now(),
	}

	switch market {
	case types.MarketSH:
		return s.getSHOverview(ctx, overview)
	case types.MarketSZ:
		return s.getSZOverview(ctx, overview)
	case types.MarketHK:
		return s.getHKOverview(ctx, overview)
	case types.MarketUS:
		return s.getUSOverview(ctx, overview)
	default:
		return s.getCNOverview(ctx, overview)
	}
}

// cachedOverview 缓存的市场概览
type cachedOverview struct {
	data *types.MarketOverview
	time time.Time
}

// getSHOverview 获取上海市场概览
func (s *Service) getSHOverview(ctx context.Context, overview *types.MarketOverview) (*types.MarketOverview, error) {
	// 获取上证指数
	index, err := s.getIndex(ctx, "sh000001")
	if err == nil {
		overview.IndexSymbol = "000001"
		overview.IndexName = index.Name
		overview.IndexPrice = index.Price
		overview.IndexChange = index.Change
		overview.IndexChangePct = index.ChangePct
		overview.AdvanceCount = index.UpCount
		overview.DeclineCount = index.DownCount
		overview.Indices = append(overview.Indices, index)
	}

	// 获取其他指数
	otherIndices := []string{"sh000016", "sh000300", "sh000688"}
	for _, code := range otherIndices {
		if idx, err := s.getIndex(ctx, code); err == nil {
			overview.Indices = append(overview.Indices, idx)
		}
	}

	// 获取涨跌榜
	overview.TopGainers, _ = s.getTopList(ctx, types.MarketSH, "gainer", 10)
	overview.TopLosers, _ = s.getTopList(ctx, types.MarketSH, "loser", 10)
	overview.TopVolume, _ = s.getTopList(ctx, types.MarketSH, "volume", 10)
	overview.TopAmount, _ = s.getTopList(ctx, types.MarketSH, "amount", 10)

	// 缓存结果
	s.cache.Store(fmt.Sprintf("overview_%s", types.MarketSH), &cachedOverview{
		data: overview,
		time: time.Now(),
	})

	return overview, nil
}

// getSZOverview 获取深圳市场概览
func (s *Service) getSZOverview(ctx context.Context, overview *types.MarketOverview) (*types.MarketOverview, error) {
	// 获取深证成指
	index, err := s.getIndex(ctx, "sz399001")
	if err == nil {
		overview.IndexSymbol = "399001"
		overview.IndexName = index.Name
		overview.IndexPrice = index.Price
		overview.IndexChange = index.Change
		overview.IndexChangePct = index.ChangePct
		overview.AdvanceCount = index.UpCount
		overview.DeclineCount = index.DownCount
		overview.Indices = append(overview.Indices, index)
	}

	// 获取其他指数
	otherIndices := []string{"sz399006", "sz399005"} // 创业板指, 中小板指
	for _, code := range otherIndices {
		if idx, err := s.getIndex(ctx, code); err == nil {
			overview.Indices = append(overview.Indices, idx)
		}
	}

	// 获取涨跌榜
	overview.TopGainers, _ = s.getTopList(ctx, types.MarketSZ, "gainer", 10)
	overview.TopLosers, _ = s.getTopList(ctx, types.MarketSZ, "loser", 10)

	// 缓存结果
	s.cache.Store(fmt.Sprintf("overview_%s", types.MarketSZ), &cachedOverview{
		data: overview,
		time: time.Now(),
	})

	return overview, nil
}

// getHKOverview 获取香港市场概览
func (s *Service) getHKOverview(ctx context.Context, overview *types.MarketOverview) (*types.MarketOverview, error) {
	// 恒生指数
	index, err := s.getIndex(ctx, "hkHSI")
	if err == nil {
		overview.IndexSymbol = "HSI"
		overview.IndexName = index.Name
		overview.IndexPrice = index.Price
		overview.IndexChange = index.Change
		overview.IndexChangePct = index.ChangePct
		overview.Indices = append(overview.Indices, index)
	}

	// 缓存结果
	s.cache.Store(fmt.Sprintf("overview_%s", types.MarketHK), &cachedOverview{
		data: overview,
		time: time.Now(),
	})

	return overview, nil
}

// getUSOverview 获取美国市场概览
func (s *Service) getUSOverview(ctx context.Context, overview *types.MarketOverview) (*types.MarketOverview, error) {
	// 道琼斯、纳斯达克、标普500
	indices := []struct {
		code string
		name string
	}{
		{"gb_$DJI", "道琼斯"},
		{"gb_$IXIC", "纳斯达克"},
		{"gb_$INX", "标普500"},
	}

	for i, idx := range indices {
		if index, err := s.getIndex(ctx, idx.code); err == nil {
			if i == 0 {
				overview.IndexSymbol = idx.code
				overview.IndexName = index.Name
				overview.IndexPrice = index.Price
				overview.IndexChange = index.Change
				overview.IndexChangePct = index.ChangePct
			}
			overview.Indices = append(overview.Indices, index)
		}
	}

	// 缓存结果
	s.cache.Store(fmt.Sprintf("overview_%s", types.MarketUS), &cachedOverview{
		data: overview,
		time: time.Now(),
	})

	return overview, nil
}

// getCNOverview 获取中国市场综合概览
func (s *Service) getCNOverview(ctx context.Context, overview *types.MarketOverview) (*types.MarketOverview, error) {
	// 获取上海和深圳的概览
	sh, _ := s.getSHOverview(ctx, &types.MarketOverview{Market: types.MarketSH})
	sz, _ := s.getSZOverview(ctx, &types.MarketOverview{Market: types.MarketSZ})

	// 合并指数
	if sh != nil {
		overview.Indices = append(overview.Indices, sh.Indices...)
		overview.AdvanceCount += sh.AdvanceCount
		overview.DeclineCount += sh.DeclineCount
	}
	if sz != nil {
		overview.Indices = append(overview.Indices, sz.Indices...)
		overview.AdvanceCount += sz.AdvanceCount
		overview.DeclineCount += sz.DeclineCount
	}

	// 使用上证指数作为主指数
	if len(overview.Indices) > 0 {
		idx := overview.Indices[0]
		overview.IndexSymbol = idx.Code
		overview.IndexName = idx.Name
		overview.IndexPrice = idx.Price
		overview.IndexChange = idx.Change
		overview.IndexChangePct = idx.ChangePct
	}

	return overview, nil
}

// getIndex 获取指数行情
func (s *Service) getIndex(ctx context.Context, code string) (*types.IndexInfo, error) {
	// 使用新浪API获取指数
	reqURL := fmt.Sprintf("https://hq.sinajs.cn/list=%s", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://finance.sina.com.cn/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return s.parseIndexResponse(string(body), code)
}

// parseIndexResponse 解析指数响应
func (s *Service) parseIndexResponse(body string, code string) (*types.IndexInfo, error) {
	// 新浪返回格式: var hq_str_sh000001="上证指数,3000.00,2999.00,..."
	start := strings.Index(body, "\"")
	end := strings.LastIndex(body, "\"")
	if start == -1 || end == -1 || start >= end {
		return nil, fmt.Errorf("invalid response format")
	}

	data := body[start+1 : end]
	if data == "" {
		return nil, fmt.Errorf("empty data")
	}

	parts := strings.Split(data, ",")
	if len(parts) < 10 {
		return nil, fmt.Errorf("insufficient data fields")
	}

	index := &types.IndexInfo{
		Code:       code,
		Name:       parts[0],
		UpdateTime: time.Now(),
	}

	// A股指数格式
	if len(parts) >= 32 {
		index.Open, _ = strconv.ParseFloat(parts[1], 64)
		index.PreClose, _ = strconv.ParseFloat(parts[2], 64)
		index.Price, _ = strconv.ParseFloat(parts[3], 64)
		index.High, _ = strconv.ParseFloat(parts[4], 64)
		index.Low, _ = strconv.ParseFloat(parts[5], 64)
		vol, _ := strconv.ParseFloat(parts[8], 64)
		index.Volume = int64(vol)
		index.Amount, _ = strconv.ParseFloat(parts[9], 64)

		if index.PreClose > 0 {
			index.Change = index.Price - index.PreClose
			index.ChangePct = index.Change / index.PreClose * 100
		}
	}

	return index, nil
}

// getTopList 获取排行榜
func (s *Service) getTopList(ctx context.Context, market types.Market, listType string, limit int) ([]*types.Quote, error) {
	// 使用东方财富排行榜API
	var fsCode string
	switch market {
	case types.MarketSH:
		fsCode = "m:1+t:2,m:1+t:23"
	case types.MarketSZ:
		fsCode = "m:0+t:6,m:0+t:80"
	default:
		fsCode = "m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23"
	}

	var orderField string
	switch listType {
	case "gainer":
		orderField = "f3"
	case "loser":
		orderField = "-f3"
	case "volume":
		orderField = "-f5"
	case "amount":
		orderField = "-f6"
	default:
		orderField = "-f3"
	}

	reqURL := fmt.Sprintf("https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=%d&po=1&np=1&fltt=2&invt=2&fid=%s&fs=%s&fields=f2,f3,f4,f5,f6,f12,f14",
		limit, orderField, fsCode)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return s.parseTopListResponse(body, market)
}

// parseTopListResponse 解析排行榜响应
func (s *Service) parseTopListResponse(body []byte, market types.Market) ([]*types.Quote, error) {
	var result struct {
		Data struct {
			Diff []struct {
				F2  float64 `json:"f2"`  // 最新价
				F3  float64 `json:"f3"`  // 涨跌幅
				F4  float64 `json:"f4"`  // 涨跌额
				F5  int64   `json:"f5"`  // 成交量
				F6  float64 `json:"f6"`  // 成交额
				F12 string  `json:"f12"` // 代码
				F14 string  `json:"f14"` // 名称
			} `json:"diff"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	quotes := make([]*types.Quote, 0, len(result.Data.Diff))
	for _, item := range result.Data.Diff {
		quote := &types.Quote{
			Symbol:     item.F12,
			Name:       item.F14,
			Market:     market,
			AssetType:  types.AssetTypeStock,
			Price:      item.F2,
			ChangePct:  item.F3,
			Change:     item.F4,
			Volume:     item.F5,
			Amount:     item.F6,
			UpdateTime: time.Now(),
		}
		quotes = append(quotes, quote)
	}

	return quotes, nil
}

// GetSectors 获取板块行情
func (s *Service) GetSectors(ctx context.Context, sectorType string, limit int) ([]*types.SectorInfo, error) {
	// 使用东方财富板块API
	var fsCode string
	switch sectorType {
	case "industry":
		fsCode = "m:90+t:2"
	case "concept":
		fsCode = "m:90+t:3"
	case "region":
		fsCode = "m:90+t:1"
	default:
		fsCode = "m:90+t:2"
	}

	reqURL := fmt.Sprintf("https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=%d&po=1&np=1&fltt=2&invt=2&fid=f3&fs=%s&fields=f2,f3,f4,f12,f14,f128,f140,f141",
		limit, fsCode)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return s.parseSectorResponse(body)
}

// parseSectorResponse 解析板块响应
func (s *Service) parseSectorResponse(body []byte) ([]*types.SectorInfo, error) {
	var result struct {
		Data struct {
			Diff []struct {
				F3   float64 `json:"f3"`   // 涨跌幅
				F12  string  `json:"f12"`  // 板块代码
				F14  string  `json:"f14"`  // 板块名称
				F128 string  `json:"f128"` // 领涨股名称
				F140 string  `json:"f140"` // 领涨股代码
				F141 float64 `json:"f141"` // 领涨股涨幅
			} `json:"diff"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	sectors := make([]*types.SectorInfo, 0, len(result.Data.Diff))
	for _, item := range result.Data.Diff {
		sector := &types.SectorInfo{
			Code:       item.F12,
			Name:       item.F14,
			ChangePct:  item.F3,
			LeaderCode: item.F140,
			LeaderName: item.F128,
			UpdateTime: time.Now(),
		}
		sectors = append(sectors, sector)
	}

	// 按涨跌幅排序
	sort.Slice(sectors, func(i, j int) bool {
		return sectors[i].ChangePct > sectors[j].ChangePct
	})

	return sectors, nil
}

// GetNorthFlow 获取北向资金流向
func (s *Service) GetNorthFlow(ctx context.Context) (*NorthFlowData, error) {
	// 使用东方财富北向资金API
	reqURL := "https://push2.eastmoney.com/api/qt/kamtbs.rtmin/get?fields1=f1,f2,f3&fields2=f51,f52,f53,f54,f55,f56"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			S2N []string `json:"s2n"` // 沪股通 + 深股通
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	flow := &NorthFlowData{
		Time: time.Now(),
	}

	// 解析最后一条数据
	if len(result.Data.S2N) > 0 {
		lastData := result.Data.S2N[len(result.Data.S2N)-1]
		parts := strings.Split(lastData, ",")
		if len(parts) >= 6 {
			flow.SHConnect, _ = strconv.ParseFloat(parts[1], 64)
			flow.SZConnect, _ = strconv.ParseFloat(parts[2], 64)
			flow.Total, _ = strconv.ParseFloat(parts[3], 64)
		}
	}

	return flow, nil
}

// NorthFlowData 北向资金数据
type NorthFlowData struct {
	Time      time.Time `json:"time"`
	SHConnect float64   `json:"sh_connect"` // 沪股通净流入(亿)
	SZConnect float64   `json:"sz_connect"` // 深股通净流入(亿)
	Total     float64   `json:"total"`      // 总净流入(亿)
}
