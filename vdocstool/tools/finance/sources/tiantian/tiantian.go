// Package tiantian 天天基金数据源
package tiantian

import (
	"context"
	"encoding/json"
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
	fundInfoAPI   = "https://fundgz.1234567.com.cn/js/"
	fundDetailAPI = "https://fund.eastmoney.com/pingzhongdata/"
	fundNavAPI    = "https://api.fund.eastmoney.com/f10/lsjz"
	fundSearchAPI = "https://fundsuggest.eastmoney.com/FundSearch/api/FundSearchAPI.ashx"
)

// Source 天天基金数据源
type Source struct {
	client *http.Client
	config *sources.SourceConfig
}

// New 创建天天基金数据源
func New(config *sources.SourceConfig) *Source {
	if config == nil {
		config = sources.DefaultConfig("tiantian")
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
	return "tiantian"
}

// Priority 优先级
func (s *Source) Priority() int {
	return 1 // 基金数据首选
}

// SupportedAssets 支持的资产类型
func (s *Source) SupportedAssets() []types.AssetType {
	return []types.AssetType{
		types.AssetTypeFund,
	}
}

// SupportedMarkets 支持的市场
func (s *Source) SupportedMarkets() []types.Market {
	return []types.Market{
		types.MarketCN,
	}
}

// GetFundInfo 获取基金信息
func (s *Source) GetFundInfo(ctx context.Context, code string) (*types.FundInfo, error) {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return nil, fmt.Errorf("invalid fund code: %s", code)
	}

	// 获取实时估值
	reqURL := fundInfoAPI + code + ".js"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://fund.eastmoney.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	return s.parseFundInfoResponse(string(body), code)
}

// parseFundInfoResponse 解析基金信息响应
func (s *Source) parseFundInfoResponse(body string, code string) (*types.FundInfo, error) {
	// JSONP格式: jsonpgz({"fundcode":"000001","name":"华夏成长","jzrq":"2024-01-01","dwjz":"1.2345","gsz":"1.2400","gszzl":"0.45",...});
	start := strings.Index(body, "{")
	end := strings.LastIndex(body, "}")
	if start == -1 || end == -1 || start >= end {
		return nil, fmt.Errorf("invalid response format")
	}

	jsonData := body[start : end+1]

	var data struct {
		FundCode string `json:"fundcode"` // 基金代码
		Name     string `json:"name"`     // 基金名称
		Jzrq     string `json:"jzrq"`     // 净值日期
		Dwjz     string `json:"dwjz"`     // 单位净值
		Gsz      string `json:"gsz"`      // 估算净值
		Gszzl    string `json:"gszzl"`    // 估算涨幅
		Gztime   string `json:"gztime"`   // 估值时间
	}

	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, fmt.Errorf("parse json failed: %w", err)
	}

	fund := &types.FundInfo{
		Code:       data.FundCode,
		Name:       data.Name,
		Source:     s.Name(),
		UpdateTime: time.Now(),
	}

	fund.NAV, _ = strconv.ParseFloat(data.Dwjz, 64)
	fund.EstimatedNAV, _ = strconv.ParseFloat(data.Gsz, 64)
	fund.DayGrowth, _ = strconv.ParseFloat(data.Gszzl, 64)

	return fund, nil
}

// GetFundDetail 获取基金详细信息
func (s *Source) GetFundDetail(ctx context.Context, code string) (*types.FundInfo, error) {
	reqURL := fundDetailAPI + code + ".js"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://fund.eastmoney.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	return s.parseFundDetailResponse(string(body), code)
}

// parseFundDetailResponse 解析基金详情响应
func (s *Source) parseFundDetailResponse(body string, code string) (*types.FundInfo, error) {
	fund := &types.FundInfo{
		Code:       code,
		Source:     s.Name(),
		UpdateTime: time.Now(),
	}

	// 提取基金名称
	nameRe := regexp.MustCompile(`var fS_name = "([^"]+)"`)
	if matches := nameRe.FindStringSubmatch(body); len(matches) > 1 {
		fund.Name = matches[1]
	}

	// 提取基金类型
	typeRe := regexp.MustCompile(`var fS_code = "([^"]+)"`)
	if matches := typeRe.FindStringSubmatch(body); len(matches) > 1 {
		fund.Code = matches[1]
	}

	// 提取单位净值
	navRe := regexp.MustCompile(`var Data_netWorthTrend = (\[.*?\]);`)
	if matches := navRe.FindStringSubmatch(body); len(matches) > 1 {
		var navData []struct {
			X int64   `json:"x"` // 时间戳(毫秒)
			Y float64 `json:"y"` // 净值
		}
		if err := json.Unmarshal([]byte(matches[1]), &navData); err == nil && len(navData) > 0 {
			latest := navData[len(navData)-1]
			fund.NAV = latest.Y
		}
	}

	// 提取累计净值
	accNavRe := regexp.MustCompile(`var Data_ACWorthTrend = (\[.*?\]);`)
	if matches := accNavRe.FindStringSubmatch(body); len(matches) > 1 {
		var accNavData [][]float64
		if err := json.Unmarshal([]byte(matches[1]), &accNavData); err == nil && len(accNavData) > 0 {
			fund.AccNAV = accNavData[len(accNavData)-1][1]
		}
	}

	// 提取收益率
	ratesRe := regexp.MustCompile(`var syl_1n="([^"]*)".*?var syl_6y="([^"]*)".*?var syl_3y="([^"]*)".*?var syl_1y="([^"]*)"`)
	if matches := ratesRe.FindStringSubmatch(body); len(matches) > 4 {
		fund.YearGrowth, _ = strconv.ParseFloat(matches[1], 64)
		fund.SixMonthGrowth, _ = strconv.ParseFloat(matches[2], 64)
		fund.ThreeMonthGrow, _ = strconv.ParseFloat(matches[3], 64)
		fund.MonthGrowth, _ = strconv.ParseFloat(matches[4], 64)
	}

	return fund, nil
}

// GetFundNav 获取历史净值
func (s *Source) GetFundNav(ctx context.Context, code string, pageIndex, pageSize int) ([]*types.FundNav, error) {
	reqURL := fmt.Sprintf("%s?fundCode=%s&pageIndex=%d&pageSize=%d",
		fundNavAPI, code, pageIndex, pageSize)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://fund.eastmoney.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	return s.parseFundNavResponse(body, code)
}

// parseFundNavResponse 解析基金净值响应
func (s *Source) parseFundNavResponse(body []byte, code string) ([]*types.FundNav, error) {
	var result struct {
		Data struct {
			LSJZList []struct {
				FSRQ  string `json:"FSRQ"`  // 净值日期
				DWJZ  string `json:"DWJZ"`  // 单位净值
				LJJZ  string `json:"LJJZ"`  // 累计净值
				JZZZL string `json:"JZZZL"` // 日涨幅
			} `json:"LSJZList"`
		} `json:"Data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse json failed: %w", err)
	}

	navs := make([]*types.FundNav, 0, len(result.Data.LSJZList))
	for _, item := range result.Data.LSJZList {
		nav := &types.FundNav{
			Code: code,
		}

		if t, err := time.Parse("2006-01-02", item.FSRQ); err == nil {
			nav.Date = t
		}
		nav.NAV, _ = strconv.ParseFloat(item.DWJZ, 64)
		nav.AccNAV, _ = strconv.ParseFloat(item.LJJZ, 64)
		nav.DailyReturn, _ = strconv.ParseFloat(item.JZZZL, 64)

		navs = append(navs, nav)
	}

	return navs, nil
}

// SearchFund 搜索基金
func (s *Source) SearchFund(ctx context.Context, keyword string) ([]*types.FundInfo, error) {
	reqURL := fmt.Sprintf("%s?m=1&key=%s", fundSearchAPI, keyword)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://fund.eastmoney.com/")

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

// parseSearchResponse 解析搜索响应
func (s *Source) parseSearchResponse(body []byte) ([]*types.FundInfo, error) {
	var result struct {
		Datas []struct {
			CODE      string `json:"CODE"`      // 基金代码
			NAME      string `json:"NAME"`      // 基金名称
			FundType  string `json:"FundType"`  // 基金类型
			SHORTNAME string `json:"SHORTNAME"` // 简称
		} `json:"Datas"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse json failed: %w", err)
	}

	funds := make([]*types.FundInfo, 0, len(result.Datas))
	for _, item := range result.Datas {
		fund := &types.FundInfo{
			Code:       item.CODE,
			Name:       item.NAME,
			Type:       item.FundType,
			Source:     s.Name(),
			UpdateTime: time.Now(),
		}
		funds = append(funds, fund)
	}

	return funds, nil
}

// GetFundRanking 获取基金排行
func (s *Source) GetFundRanking(ctx context.Context, fundType string, sortBy string, limit int) ([]*types.FundInfo, error) {
	// 基金类型映射
	typeMap := map[string]string{
		"stock":  "gp",  // 股票型
		"mixed":  "hh",  // 混合型
		"bond":   "zq",  // 债券型
		"index":  "zs",  // 指数型
		"qdii":   "qdii", // QDII
		"money":  "hb",  // 货币型
	}

	ft := typeMap[fundType]
	if ft == "" {
		ft = "all"
	}

	// 排序字段映射
	sortMap := map[string]string{
		"day":   "rzdf",  // 日涨幅
		"week":  "zzf",   // 周涨幅
		"month": "1yzf",  // 月涨幅
		"3month": "3yzf", // 3月涨幅
		"6month": "6yzf", // 6月涨幅
		"year":  "1nzf",  // 年涨幅
	}

	sort := sortMap[sortBy]
	if sort == "" {
		sort = "rzdf"
	}

	reqURL := fmt.Sprintf("https://fund.eastmoney.com/data/rankhandler.aspx?op=ph&dt=kf&ft=%s&rs=&gs=0&sc=%sdesc&st=desc&pi=1&pn=%d",
		ft, sort, limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://fund.eastmoney.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	return s.parseRankingResponse(string(body))
}

// parseRankingResponse 解析排行响应
func (s *Source) parseRankingResponse(body string) ([]*types.FundInfo, error) {
	// 格式: var rankData = {datas:["000001,基金名,日期,单位净值,累计净值,日涨幅,..."],allRecords:100,...}
	start := strings.Index(body, `"`)
	if start == -1 {
		return nil, fmt.Errorf("invalid response format")
	}

	// 提取所有基金数据
	re := regexp.MustCompile(`"([^"]+)"`)
	matches := re.FindAllStringSubmatch(body, -1)

	funds := make([]*types.FundInfo, 0)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		parts := strings.Split(match[1], ",")
		if len(parts) < 20 {
			continue
		}

		fund := &types.FundInfo{
			Code:       parts[0],
			Name:       parts[1],
			Source:     s.Name(),
			UpdateTime: time.Now(),
		}

		fund.NAV, _ = strconv.ParseFloat(parts[3], 64)
		fund.AccNAV, _ = strconv.ParseFloat(parts[4], 64)
		fund.DayGrowth, _ = strconv.ParseFloat(parts[5], 64)
		fund.WeekGrowth, _ = strconv.ParseFloat(parts[6], 64)
		fund.MonthGrowth, _ = strconv.ParseFloat(parts[7], 64)
		fund.ThreeMonthGrow, _ = strconv.ParseFloat(parts[8], 64)
		fund.SixMonthGrowth, _ = strconv.ParseFloat(parts[9], 64)
		fund.YearGrowth, _ = strconv.ParseFloat(parts[10], 64)

		funds = append(funds, fund)
	}

	return funds, nil
}

// HealthCheck 健康检查
func (s *Source) HealthCheck(ctx context.Context) error {
	_, err := s.GetFundInfo(ctx, "000001")
	return err
}

// 确保实现FundSource接口
var _ sources.FundSource = (*Source)(nil)
