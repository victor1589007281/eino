// Package finance 财经追踪工具 - MCP工具入口
package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/mcp"
	"github.com/cloudwego/eino/vdocstool/tools/finance/alert"
	"github.com/cloudwego/eino/vdocstool/tools/finance/indicators"
	"github.com/cloudwego/eino/vdocstool/tools/finance/market"
	"github.com/cloudwego/eino/vdocstool/tools/finance/news"
	"github.com/cloudwego/eino/vdocstool/tools/finance/portfolio"
	"github.com/cloudwego/eino/vdocstool/tools/finance/service"
	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// Tool 财经工具
type Tool struct {
	service          *service.FinanceService
	newsAggregator   *news.Aggregator
	indicatorCalc    *indicators.Calculator
	portfolioManager *portfolio.Manager
	alertManager     *alert.Manager
	marketService    *market.Service
	mu               sync.RWMutex
}

// Config 工具配置
type Config struct {
	RedisURL string // Redis URL (可选)
}

// NewTool 创建财经工具
func NewTool(config *Config) (*Tool, error) {
	var redisURL string
	if config != nil {
		redisURL = config.RedisURL
	}

	svc, err := service.NewFinanceService(&service.ServiceConfig{
		RedisURL:     redisURL,
		EnableCache:  true,
		EnableRouter: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create finance service failed: %w", err)
	}

	return &Tool{
		service:          svc,
		newsAggregator:   news.NewAggregator(),
		indicatorCalc:    indicators.NewCalculator(),
		portfolioManager: portfolio.NewManager(),
		alertManager:     alert.NewManager(),
		marketService:    market.NewService(),
	}, nil
}

// Close 关闭工具
func (t *Tool) Close() error {
	return t.service.Close()
}

// RegisterTools 注册MCP工具
func (t *Tool) RegisterTools(server *mcp.Server) {
	// finance_quote - 实时行情查询
	quoteToolBuilder := mcp.NewToolBuilder("finance_quote", "获取股票、基金、ETF等金融产品的实时行情")
	quoteToolBuilder.AddProperty("symbol", "string", "股票/基金代码，如600519、AAPL、00700.HK", true)
	quoteToolBuilder.AddProperty("symbols", "string", "批量查询多个代码，用逗号分隔，如600519,000001", false)
	server.RegisterTool(quoteToolBuilder.Build(), t.handleQuote)

	// finance_search - 搜索
	searchToolBuilder := mcp.NewToolBuilder("finance_search", "搜索股票、基金、ETF等金融产品")
	searchToolBuilder.AddProperty("keyword", "string", "搜索关键词，支持代码、名称、拼音首字母", true)
	server.RegisterTool(searchToolBuilder.Build(), t.handleSearch)

	// finance_history - 历史K线
	historyToolBuilder := mcp.NewToolBuilder("finance_history", "获取股票/基金的历史K线数据")
	historyToolBuilder.AddProperty("symbol", "string", "股票/基金代码", true)
	historyToolBuilder.AddEnumProperty("period", "K线周期", []string{"1m", "5m", "15m", "30m", "60m", "1d", "1w", "1M"}, false)
	historyToolBuilder.AddProperty("count", "number", "获取数量，默认100，最大1000", false)
	server.RegisterTool(historyToolBuilder.Build(), t.handleHistory)

	// finance_news - 财经新闻
	newsToolBuilder := mcp.NewToolBuilder("finance_news", "获取财经新闻和快讯")
	newsToolBuilder.AddEnumProperty("type", "新闻类型", []string{"flash", "important", "search", "stock"}, false)
	newsToolBuilder.AddProperty("keyword", "string", "搜索关键词(仅type=search时使用)", false)
	newsToolBuilder.AddProperty("symbol", "string", "股票代码(仅type=stock时使用)", false)
	newsToolBuilder.AddProperty("limit", "number", "返回数量，默认20", false)
	server.RegisterTool(newsToolBuilder.Build(), t.handleNews)

	// finance_analysis - 技术分析
	analysisToolBuilder := mcp.NewToolBuilder("finance_analysis", "获取股票的技术分析报告，包含MA、MACD、RSI、KDJ、BOLL等指标")
	analysisToolBuilder.AddProperty("symbol", "string", "股票代码", true)
	analysisToolBuilder.AddProperty("period", "string", "K线周期，默认1d", false)
	server.RegisterTool(analysisToolBuilder.Build(), t.handleAnalysis)

	// finance_portfolio - 投资组合管理
	portfolioToolBuilder := mcp.NewToolBuilder("finance_portfolio", "管理投资组合、持仓和交易记录")
	portfolioToolBuilder.AddEnumProperty("action", "操作类型", []string{"list", "create", "delete", "add_position", "update_position", "remove_position", "summary"}, true)
	portfolioToolBuilder.AddProperty("user_id", "string", "用户ID", true)
	portfolioToolBuilder.AddProperty("name", "string", "组合名称", false)
	portfolioToolBuilder.AddProperty("symbol", "string", "股票代码(持仓操作时使用)", false)
	portfolioToolBuilder.AddProperty("market", "string", "市场(sh/sz/hk/us)", false)
	portfolioToolBuilder.AddProperty("quantity", "number", "数量", false)
	portfolioToolBuilder.AddProperty("price", "number", "价格/成本", false)
	server.RegisterTool(portfolioToolBuilder.Build(), t.handlePortfolio)

	// finance_alert - 价格提醒
	alertToolBuilder := mcp.NewToolBuilder("finance_alert", "设置和管理价格提醒")
	alertToolBuilder.AddEnumProperty("action", "操作类型", []string{"list", "create", "delete", "disable", "enable"}, true)
	alertToolBuilder.AddProperty("user_id", "string", "用户ID", true)
	alertToolBuilder.AddProperty("alert_id", "number", "提醒ID(delete/disable/enable时使用)", false)
	alertToolBuilder.AddProperty("symbol", "string", "股票代码(create时使用)", false)
	alertToolBuilder.AddProperty("market", "string", "市场(sh/sz/hk/us)", false)
	alertToolBuilder.AddEnumProperty("alert_type", "提醒类型", []string{"price_above", "price_below", "change_above", "change_below", "volume_above"}, false)
	alertToolBuilder.AddProperty("threshold", "number", "阈值", false)
	server.RegisterTool(alertToolBuilder.Build(), t.handleAlert)

	// finance_market - 市场概览
	marketToolBuilder := mcp.NewToolBuilder("finance_market", "获取市场概览、板块行情、涨跌排行等")
	marketToolBuilder.AddEnumProperty("type", "查询类型", []string{"overview", "sectors", "top_gainers", "top_losers", "north_flow"}, true)
	marketToolBuilder.AddEnumProperty("market", "市场", []string{"sh", "sz", "hk", "us", "cn"}, false)
	marketToolBuilder.AddEnumProperty("sector_type", "板块类型(type=sectors时使用)", []string{"industry", "concept", "region"}, false)
	marketToolBuilder.AddProperty("limit", "number", "返回数量，默认10", false)
	server.RegisterTool(marketToolBuilder.Build(), t.handleMarket)

	// finance_status - 服务状态
	statusToolBuilder := mcp.NewToolBuilder("finance_status", "获取财经服务状态和数据源健康信息")
	server.RegisterTool(statusToolBuilder.Build(), t.handleStatus)
}

// handleQuote 处理行情查询
func (t *Tool) handleQuote(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// 检查是否批量查询
	if symbolsStr, ok := args["symbols"].(string); ok && symbolsStr != "" {
		symbols := strings.Split(symbolsStr, ",")
		for i := range symbols {
			symbols[i] = strings.TrimSpace(symbols[i])
		}
		return t.getQuotes(ctx, symbols)
	}

	// 单个查询
	symbol, ok := args["symbol"].(string)
	if !ok || symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	return t.getQuote(ctx, symbol)
}

// getQuote 获取单个行情
func (t *Tool) getQuote(ctx context.Context, symbol string) (interface{}, error) {
	quote, err := t.service.GetQuote(ctx, symbol)
	if err != nil {
		return nil, err
	}

	return t.formatQuote(quote), nil
}

// getQuotes 批量获取行情
func (t *Tool) getQuotes(ctx context.Context, symbols []string) (interface{}, error) {
	quotes, err := t.service.GetQuotes(ctx, symbols)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(quotes))
	for _, q := range quotes {
		result = append(result, t.formatQuote(q))
	}

	return map[string]interface{}{
		"count":  len(result),
		"quotes": result,
	}, nil
}

// formatQuote 格式化行情
func (t *Tool) formatQuote(q *types.Quote) map[string]interface{} {
	changeSign := ""
	if q.Change > 0 {
		changeSign = "+"
	}

	return map[string]interface{}{
		"symbol":      q.Symbol,
		"name":        q.Name,
		"market":      q.Market,
		"price":       fmt.Sprintf("%.2f", q.Price),
		"change":      fmt.Sprintf("%s%.2f", changeSign, q.Change),
		"change_pct":  fmt.Sprintf("%s%.2f%%", changeSign, q.ChangePct),
		"open":        fmt.Sprintf("%.2f", q.Open),
		"high":        fmt.Sprintf("%.2f", q.High),
		"low":         fmt.Sprintf("%.2f", q.Low),
		"pre_close":   fmt.Sprintf("%.2f", q.PreClose),
		"volume":      q.Volume,
		"amount":      t.formatAmount(q.Amount),
		"turnover":    fmt.Sprintf("%.2f%%", q.TurnoverRate),
		"market_cap":  t.formatAmount(q.MarketCap),
		"pe":          fmt.Sprintf("%.2f", q.PERatio),
		"pb":          fmt.Sprintf("%.2f", q.PBRatio),
		"update_time": q.UpdateTime.Format("2006-01-02 15:04:05"),
		"source":      q.Source,
	}
}

// formatAmount 格式化金额
func (t *Tool) formatAmount(amount float64) string {
	if amount >= 100000000 {
		return fmt.Sprintf("%.2f亿", amount/100000000)
	}
	if amount >= 10000 {
		return fmt.Sprintf("%.2f万", amount/10000)
	}
	return fmt.Sprintf("%.2f", amount)
}

// handleSearch 处理搜索
func (t *Tool) handleSearch(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	keyword, ok := args["keyword"].(string)
	if !ok || keyword == "" {
		return nil, fmt.Errorf("keyword is required")
	}

	results, err := t.service.Search(ctx, keyword)
	if err != nil {
		return nil, err
	}

	formatted := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		formatted = append(formatted, map[string]interface{}{
			"symbol":     r.Symbol,
			"name":       r.Name,
			"market":     r.Market,
			"asset_type": r.AssetType,
			"exchange":   r.Exchange,
		})
	}

	return map[string]interface{}{
		"count":   len(formatted),
		"results": formatted,
	}, nil
}

// handleHistory 处理历史K线
func (t *Tool) handleHistory(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok || symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	period := "1d"
	if p, ok := args["period"].(string); ok && p != "" {
		period = p
	}

	count := 100
	if c, ok := args["count"].(float64); ok && c > 0 {
		count = int(c)
	}

	klines, err := t.service.GetKLine(ctx, symbol, period, count)
	if err != nil {
		return nil, err
	}

	formatted := make([]map[string]interface{}, 0, len(klines))
	for _, k := range klines {
		formatted = append(formatted, map[string]interface{}{
			"date":   k.Timestamp.Format("2006-01-02"),
			"open":   fmt.Sprintf("%.2f", k.Open),
			"high":   fmt.Sprintf("%.2f", k.High),
			"low":    fmt.Sprintf("%.2f", k.Low),
			"close":  fmt.Sprintf("%.2f", k.Close),
			"volume": k.Volume,
			"amount": t.formatAmount(k.Amount),
		})
	}

	return map[string]interface{}{
		"symbol": symbol,
		"period": period,
		"count":  len(formatted),
		"klines": formatted,
	}, nil
}

// handleNews 处理新闻查询
func (t *Tool) handleNews(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	newsType := "flash"
	if tp, ok := args["type"].(string); ok && tp != "" {
		newsType = tp
	}

	limit := 20
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	var newsList []*types.News
	var err error

	switch newsType {
	case "flash":
		newsList, err = t.newsAggregator.GetFlashNews(ctx, limit)
	case "important":
		newsList, err = t.newsAggregator.GetImportantNews(ctx, limit)
	case "search":
		keyword, _ := args["keyword"].(string)
		if keyword == "" {
			return nil, fmt.Errorf("keyword is required for search type")
		}
		newsList, err = t.newsAggregator.SearchNews(ctx, keyword, limit)
	case "stock":
		symbol, _ := args["symbol"].(string)
		if symbol == "" {
			return nil, fmt.Errorf("symbol is required for stock type")
		}
		newsList, err = t.newsAggregator.GetStockNews(ctx, symbol, limit)
	default:
		newsList, err = t.newsAggregator.GetNews(ctx, &news.NewsOptions{Limit: limit})
	}

	if err != nil {
		return nil, err
	}

	formatted := make([]map[string]interface{}, 0, len(newsList))
	for _, n := range newsList {
		formatted = append(formatted, map[string]interface{}{
			"id":           n.ID,
			"title":        n.Title,
			"content":      n.Content,
			"source":       n.Source,
			"category":     n.Category,
			"importance":   n.Importance,
			"symbols":      n.Symbols,
			"tags":         n.Tags,
			"publish_time": n.PublishTime.Format("2006-01-02 15:04:05"),
		})
	}

	return map[string]interface{}{
		"type":  newsType,
		"count": len(formatted),
		"news":  formatted,
	}, nil
}

// handleAnalysis 处理技术分析
func (t *Tool) handleAnalysis(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	symbol, ok := args["symbol"].(string)
	if !ok || symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	period := "1d"
	if p, ok := args["period"].(string); ok && p != "" {
		period = p
	}

	// 获取K线数据
	klines, err := t.service.GetKLine(ctx, symbol, period, 100)
	if err != nil {
		return nil, err
	}

	if len(klines) < 30 {
		return nil, fmt.Errorf("insufficient data for analysis, need at least 30 data points")
	}

	// 计算技术指标
	report := t.indicatorCalc.Analyze(klines)
	if report == nil {
		return nil, fmt.Errorf("analysis failed")
	}

	// 获取最新行情补充报告
	quote, _ := t.service.GetQuote(ctx, symbol)
	if quote != nil {
		report.Name = quote.Name
	}

	// 格式化指标
	indicatorsFormatted := make([]map[string]interface{}, 0, len(report.Indicators))
	for _, ind := range report.Indicators {
		indicatorsFormatted = append(indicatorsFormatted, map[string]interface{}{
			"name":   ind.Name,
			"values": ind.Values,
			"signal": ind.Signal,
		})
	}

	return map[string]interface{}{
		"symbol":      report.Symbol,
		"name":        report.Name,
		"trend":       report.Trend,
		"signal":      report.Signal,
		"support":     fmt.Sprintf("%.2f", report.Support),
		"resistance":  fmt.Sprintf("%.2f", report.Resistance),
		"indicators":  indicatorsFormatted,
		"summary":     report.Summary,
		"update_time": time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// handlePortfolio 处理投资组合
func (t *Tool) handlePortfolio(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action, _ := args["action"].(string)
	userID, _ := args["user_id"].(string)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	switch action {
	case "list":
		portfolios, err := t.portfolioManager.GetPortfolios(ctx, userID)
		if err != nil {
			return nil, err
		}
		result := make([]map[string]interface{}, 0, len(portfolios))
		for _, p := range portfolios {
			result = append(result, map[string]interface{}{
				"name":           p.Name,
				"description":    p.Description,
				"currency":       p.Currency,
				"initial_capital": p.InitialCapital,
				"total_value":    p.TotalValue,
				"total_profit":   p.TotalProfit,
				"position_count": len(p.Positions),
				"created_at":     p.CreatedAt.Format("2006-01-02"),
			})
		}
		return map[string]interface{}{"portfolios": result}, nil

	case "create":
		name, _ := args["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("name is required for create")
		}
		desc, _ := args["description"].(string)
		currency := "CNY"
		if c, ok := args["currency"].(string); ok && c != "" {
			currency = c
		}
		initialCapital := 0.0
		if ic, ok := args["initial_capital"].(float64); ok {
			initialCapital = ic
		}
		p, err := t.portfolioManager.CreatePortfolio(ctx, userID, name, desc, currency, initialCapital)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"message": "Portfolio created successfully",
			"name":    p.Name,
		}, nil

	case "delete":
		name, _ := args["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("name is required for delete")
		}
		err := t.portfolioManager.DeletePortfolio(ctx, userID, name)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"message": "Portfolio deleted successfully"}, nil

	case "add_position":
		name, _ := args["name"].(string)
		symbol, _ := args["symbol"].(string)
		if name == "" || symbol == "" {
			return nil, fmt.Errorf("name and symbol are required for add_position")
		}
		marketStr, _ := args["market"].(string)
		quantity, _ := args["quantity"].(float64)
		price, _ := args["price"].(float64)
		pos := &types.Position{
			Symbol:   symbol,
			Market:   types.Market(marketStr),
			Quantity: quantity,
			AvgCost:  price,
		}
		err := t.portfolioManager.AddPosition(ctx, userID, name, pos)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"message": "Position added successfully"}, nil

	case "summary":
		name, _ := args["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("name is required for summary")
		}
		summary, err := t.portfolioManager.GetPortfolioSummary(ctx, userID, name)
		if err != nil {
			return nil, err
		}
		positions := make([]map[string]interface{}, 0, len(summary.Portfolio.Positions))
		for _, p := range summary.Portfolio.Positions {
			positions = append(positions, map[string]interface{}{
				"symbol":        p.Symbol,
				"name":          p.Name,
				"quantity":      p.Quantity,
				"avg_cost":      fmt.Sprintf("%.2f", p.AvgCost),
				"current_price": fmt.Sprintf("%.2f", p.CurrentPrice),
				"market_value":  fmt.Sprintf("%.2f", p.MarketValue),
				"profit_loss":   fmt.Sprintf("%.2f", p.ProfitLoss),
				"profit_pct":    fmt.Sprintf("%.2f%%", p.ProfitPct),
			})
		}
		return map[string]interface{}{
			"name":             summary.Portfolio.Name,
			"position_count":   summary.PositionCount,
			"total_cost":       fmt.Sprintf("%.2f", summary.TotalCost),
			"total_value":      fmt.Sprintf("%.2f", summary.TotalValue),
			"total_profit":     fmt.Sprintf("%.2f", summary.TotalProfit),
			"total_profit_pct": fmt.Sprintf("%.2f%%", summary.TotalProfitPct),
			"asset_allocation": summary.AssetAllocation,
			"positions":        positions,
		}, nil

	default:
		return nil, fmt.Errorf("invalid action: %s", action)
	}
}

// handleAlert 处理价格提醒
func (t *Tool) handleAlert(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action, _ := args["action"].(string)
	userID, _ := args["user_id"].(string)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	switch action {
	case "list":
		alerts, err := t.alertManager.GetActiveAlerts(ctx, userID)
		if err != nil {
			return nil, err
		}
		result := make([]map[string]interface{}, 0, len(alerts))
		for _, a := range alerts {
			result = append(result, map[string]interface{}{
				"id":         a.ID,
				"symbol":     a.Symbol,
				"name":       a.Name,
				"market":     a.Market,
				"alert_type": a.AlertType,
				"threshold":  a.Threshold,
				"status":     a.Status,
				"created_at": a.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		return map[string]interface{}{"alerts": result}, nil

	case "create":
		symbol, _ := args["symbol"].(string)
		if symbol == "" {
			return nil, fmt.Errorf("symbol is required for create")
		}
		marketStr, _ := args["market"].(string)
		alertType, _ := args["alert_type"].(string)
		threshold, _ := args["threshold"].(float64)
		if alertType == "" || threshold == 0 {
			return nil, fmt.Errorf("alert_type and threshold are required for create")
		}
		al := &types.Alert{
			UserID:    userID,
			Symbol:    symbol,
			Market:    types.Market(marketStr),
			AlertType: alertType,
			Threshold: threshold,
		}
		// 获取名称
		if quote, err := t.service.GetQuote(ctx, symbol); err == nil {
			al.Name = quote.Name
		}
		err := t.alertManager.CreateAlert(ctx, al)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"message":  "Alert created successfully",
			"alert_id": al.ID,
		}, nil

	case "delete":
		alertID, _ := args["alert_id"].(float64)
		if alertID == 0 {
			return nil, fmt.Errorf("alert_id is required for delete")
		}
		err := t.alertManager.DeleteAlert(ctx, int64(alertID), userID)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"message": "Alert deleted successfully"}, nil

	case "disable":
		alertID, _ := args["alert_id"].(float64)
		if alertID == 0 {
			return nil, fmt.Errorf("alert_id is required for disable")
		}
		err := t.alertManager.DisableAlert(ctx, int64(alertID))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"message": "Alert disabled successfully"}, nil

	case "enable":
		alertID, _ := args["alert_id"].(float64)
		if alertID == 0 {
			return nil, fmt.Errorf("alert_id is required for enable")
		}
		err := t.alertManager.EnableAlert(ctx, int64(alertID))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"message": "Alert enabled successfully"}, nil

	default:
		return nil, fmt.Errorf("invalid action: %s", action)
	}
}

// handleMarket 处理市场概览
func (t *Tool) handleMarket(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	queryType, _ := args["type"].(string)
	marketStr := "cn"
	if m, ok := args["market"].(string); ok && m != "" {
		marketStr = m
	}
	mkt := types.Market(marketStr)

	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	switch queryType {
	case "overview":
		overview, err := t.marketService.GetMarketOverview(ctx, mkt)
		if err != nil {
			return nil, err
		}
		indices := make([]map[string]interface{}, 0, len(overview.Indices))
		for _, idx := range overview.Indices {
			changeSign := ""
			if idx.Change > 0 {
				changeSign = "+"
			}
			indices = append(indices, map[string]interface{}{
				"code":       idx.Code,
				"name":       idx.Name,
				"price":      fmt.Sprintf("%.2f", idx.Price),
				"change":     fmt.Sprintf("%s%.2f", changeSign, idx.Change),
				"change_pct": fmt.Sprintf("%s%.2f%%", changeSign, idx.ChangePct),
			})
		}
		return map[string]interface{}{
			"market":         marketStr,
			"index_name":     overview.IndexName,
			"index_price":    fmt.Sprintf("%.2f", overview.IndexPrice),
			"index_change":   fmt.Sprintf("%.2f", overview.IndexChange),
			"index_pct":      fmt.Sprintf("%.2f%%", overview.IndexChangePct),
			"advance_count":  overview.AdvanceCount,
			"decline_count":  overview.DeclineCount,
			"indices":        indices,
			"update_time":    overview.UpdateTime.Format("2006-01-02 15:04:05"),
		}, nil

	case "sectors":
		sectorType := "industry"
		if st, ok := args["sector_type"].(string); ok && st != "" {
			sectorType = st
		}
		sectors, err := t.marketService.GetSectors(ctx, sectorType, limit)
		if err != nil {
			return nil, err
		}
		result := make([]map[string]interface{}, 0, len(sectors))
		for _, s := range sectors {
			changeSign := ""
			if s.ChangePct > 0 {
				changeSign = "+"
			}
			result = append(result, map[string]interface{}{
				"code":        s.Code,
				"name":        s.Name,
				"change_pct":  fmt.Sprintf("%s%.2f%%", changeSign, s.ChangePct),
				"leader_name": s.LeaderName,
				"leader_code": s.LeaderCode,
			})
		}
		return map[string]interface{}{
			"sector_type": sectorType,
			"count":       len(result),
			"sectors":     result,
		}, nil

	case "top_gainers", "top_losers":
		overview, err := t.marketService.GetMarketOverview(ctx, mkt)
		if err != nil {
			return nil, err
		}
		var quotes []*types.Quote
		if queryType == "top_gainers" {
			quotes = overview.TopGainers
		} else {
			quotes = overview.TopLosers
		}
		result := make([]map[string]interface{}, 0, len(quotes))
		for i, q := range quotes {
			if i >= limit {
				break
			}
			changeSign := ""
			if q.ChangePct > 0 {
				changeSign = "+"
			}
			result = append(result, map[string]interface{}{
				"symbol":     q.Symbol,
				"name":       q.Name,
				"price":      fmt.Sprintf("%.2f", q.Price),
				"change_pct": fmt.Sprintf("%s%.2f%%", changeSign, q.ChangePct),
				"volume":     q.Volume,
			})
		}
		return map[string]interface{}{
			"type":   queryType,
			"market": marketStr,
			"count":  len(result),
			"items":  result,
		}, nil

	case "north_flow":
		flow, err := t.marketService.GetNorthFlow(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"sh_connect": fmt.Sprintf("%.2f亿", flow.SHConnect/10000),
			"sz_connect": fmt.Sprintf("%.2f亿", flow.SZConnect/10000),
			"total":      fmt.Sprintf("%.2f亿", flow.Total/10000),
			"time":       flow.Time.Format("2006-01-02 15:04:05"),
		}, nil

	default:
		return nil, fmt.Errorf("invalid type: %s", queryType)
	}
}

// handleStatus 处理服务状态
func (t *Tool) handleStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	stats := t.service.GetRouterStats()
	health := t.service.GetSourceHealth()

	sources := make([]map[string]interface{}, 0, len(health))
	for name, h := range health {
		sources = append(sources, map[string]interface{}{
			"name":              name,
			"status":            h.Status,
			"success_rate":      fmt.Sprintf("%.2f%%", h.SuccessRate*100),
			"avg_latency":       h.AvgLatency.String(),
			"consecutive_fails": h.ConsecutiveFails,
			"last_check":        h.LastCheck.Format("2006-01-02 15:04:05"),
			"error":             h.ErrorMessage,
		})
	}

	// 获取新闻源健康状态
	newsHealth := t.newsAggregator.HealthCheck(ctx)
	newsSources := make([]map[string]interface{}, 0)
	for name, err := range newsHealth {
		status := "healthy"
		errMsg := ""
		if err != nil {
			status = "unhealthy"
			errMsg = err.Error()
		}
		newsSources = append(newsSources, map[string]interface{}{
			"name":   name,
			"status": status,
			"error":  errMsg,
		})
	}

	// 获取提醒统计
	alertStats := t.alertManager.Stats()

	return map[string]interface{}{
		"quote_sources": map[string]interface{}{
			"total":   stats.TotalSources,
			"healthy": stats.HealthySources,
			"sources": sources,
		},
		"news_sources": newsSources,
		"alerts": map[string]interface{}{
			"total":     alertStats.TotalAlerts,
			"active":    alertStats.ActiveAlerts,
			"triggered": alertStats.TriggeredAlerts,
		},
	}, nil
}

// GetSkillsYAML 获取Skills配置
func GetSkillsYAML() string {
	return `
name: financial_tracker
description: 实时金融市场追踪工具，支持A股、港股、美股、基金、ETF等行情查询、技术分析、组合管理

capabilities:
  - 实时行情查询 (股票/基金/ETF)
  - 历史K线数据获取
  - 股票/基金搜索
  - 批量行情查询
  - 财经新闻快讯
  - 技术指标分析 (MA/MACD/RSI/KDJ/BOLL)
  - 投资组合管理
  - 价格提醒设置
  - 市场概览/板块热度/涨跌榜

tools:
  - finance_quote: 获取实时行情
  - finance_search: 搜索金融产品
  - finance_history: 获取历史K线
  - finance_news: 获取财经新闻
  - finance_analysis: 技术分析报告
  - finance_portfolio: 投资组合管理
  - finance_alert: 价格提醒
  - finance_market: 市场概览
  - finance_status: 查看服务状态

examples:
  - query: "茅台现在多少钱"
    tool: finance_quote
    params: {symbol: "600519"}
    
  - query: "查下腾讯和阿里的股价"
    tool: finance_quote
    params: {symbols: "00700.HK,BABA"}
    
  - query: "搜索一下新能源相关的股票"
    tool: finance_search
    params: {keyword: "新能源"}
    
  - query: "看下茅台最近一个月的走势"
    tool: finance_history
    params: {symbol: "600519", period: "1d", count: 30}

  - query: "今天有什么重要财经新闻"
    tool: finance_news
    params: {type: "important", limit: 10}

  - query: "分析下比亚迪的技术面"
    tool: finance_analysis
    params: {symbol: "002594"}

  - query: "帮我创建一个投资组合"
    tool: finance_portfolio
    params: {action: "create", user_id: "user1", name: "我的组合"}

  - query: "设置茅台跌破1800提醒我"
    tool: finance_alert
    params: {action: "create", user_id: "user1", symbol: "600519", alert_type: "price_below", threshold: 1800}

  - query: "今天A股市场怎么样"
    tool: finance_market
    params: {type: "overview", market: "cn"}

symbol_formats:
  A股上海: "600519, 601318 (以6开头)"
  A股深圳: "000001, 002594, 300750 (以0/3开头)"
  港股: "00700.HK, 09988.HK (5位数字+.HK)"
  美股: "AAPL, TSLA, GOOGL (英文字母)"
  基金: "005827, 110011 (6位数字)"
  ETF: "510300, 159919 (以51/15开头)"

tips:
  - 股票代码可以直接输入数字，系统自动识别市场
  - 支持中文名称和拼音首字母搜索
  - 批量查询用逗号分隔多个代码
  - 默认返回日K线，可指定period参数
  - 技术分析需要至少30条K线数据
  - 新闻支持财联社和金十两个数据源
  - 提醒功能需要指定用户ID
`
}

// ToJSON 转换为JSON
func ToJSON(v interface{}) string {
	data, _ := json.MarshalIndent(v, "", "  ")
	return string(data)
}
