// Package search 网页搜索工具
package search

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/vdocstool/config"
	"github.com/cloudwego/eino/vdocstool/mcp"
	"github.com/cloudwego/eino/vdocstool/tools/search/engines"
	"github.com/cloudwego/eino/vdocstool/tools/search/router"
)

// WebSearchTool 网页搜索工具
type WebSearchTool struct {
	router    *router.AdaptiveRouter
	extractor *WebExtractor
	config    config.SearchConfig
}

// NewWebSearchTool 创建网页搜索工具
func NewWebSearchTool(cfg config.SearchConfig) (*WebSearchTool, error) {
	// 创建路由器
	r := router.NewAdaptiveRouter(cfg.Router)

	// 注册搜索引擎
	for name, engCfg := range cfg.Engines {
		if !engCfg.Enabled {
			continue
		}

		var engine engines.Engine
		switch name {
		case "duckduckgo":
			engine = engines.NewDuckDuckGoEngine(engCfg.Priority)
		case "bing":
			engine = engines.NewBingEngine(engCfg.Priority, "cn")
		case "baidu":
			engine = engines.NewBaiduEngine(engCfg.Priority)
		case "serper":
			if engCfg.APIKey != "" {
				engine = engines.NewSerperEngine(engCfg.APIKey, engCfg.Priority)
			}
		case "tavily":
			if engCfg.APIKey != "" {
				engine = engines.NewTavilyEngine(engCfg.APIKey, engCfg.Priority)
			}
		case "exa":
			if engCfg.APIKey != "" {
				engine = engines.NewExaEngine(engCfg.APIKey, engCfg.Priority)
			}
		case "brave":
			if engCfg.APIKey != "" {
				engine = engines.NewBraveEngine(engCfg.APIKey, engCfg.Priority)
			}
		case "searxng":
			engine = engines.NewSearXNGEngine(engCfg.Endpoint, engCfg.Priority)
		}

		if engine != nil {
			r.RegisterEngine(engine)
		}
	}

	// 启动路由器
	r.Start()

	return &WebSearchTool{
		router:    r,
		extractor: NewWebExtractor(),
		config:    cfg,
	}, nil
}

// Close 关闭工具
func (t *WebSearchTool) Close() {
	t.router.Stop()
}

// Register 注册到 MCP Server
func (t *WebSearchTool) Register(server *mcp.Server) {
	// 注册搜索工具
	searchTool := mcp.NewToolBuilder("web_search", "执行网页搜索，支持多引擎自动降级和智能路由").
		AddProperty("query", "string", "搜索查询词", true).
		AddEnumProperty("preferred_engine", "首选搜索引擎", []string{
			"serper",    // Google 代理，推荐
			"tavily",    // AI优化搜索
			"exa",       // 语义搜索
			"brave",     // 隐私搜索
			"searxng",   // 开源免费
			"bing",      // 必应
			"duckduckgo", // DuckDuckGo
			"baidu",     // 百度
		}, false).
		AddProperty("max_results", "integer", "最大结果数，默认10", false).
		AddProperty("auto_fallback", "boolean", "是否启用自动降级，默认true", false).
		Build()

	server.RegisterTool(searchTool, t.handleWebSearch)

	// 注册健康检查工具
	healthTool := mcp.NewToolBuilder("check_engine_health", "检查搜索引擎健康状态").
		AddProperty("engine_name", "string", "引擎名称，为空则检查所有", false).
		Build()

	server.RegisterTool(healthTool, t.handleCheckHealth)

	// 注册路由策略工具
	strategyTool := mcp.NewToolBuilder("set_routing_strategy", "设置搜索路由策略").
		AddEnumProperty("strategy", "路由策略", []string{
			"health_first",  // 健康优先
			"round_robin",   // 轮询
			"weighted",      // 加权
			"quality_first", // 质量优先
			"cost_saving",   // 成本节约
			"balanced",      // 均衡
			"smart",         // 智能选择
		}, true).
		Build()

	server.RegisterTool(strategyTool, t.handleSetStrategy)

	// 注册内容提取工具
	extractTool := mcp.NewToolBuilder("extract_content", "提取网页内容").
		AddProperty("url", "string", "网页URL", true).
		AddEnumProperty("extract_type", "提取类型", []string{"text", "html", "metadata"}, false).
		AddProperty("max_length", "integer", "最大内容长度", false).
		Build()

	server.RegisterTool(extractTool, t.handleExtractContent)

	// 注册引擎状态工具（新）
	statusTool := mcp.NewToolBuilder("get_engine_status", "获取所有搜索引擎的完整状态，包括配额、质量、得分").
		AddProperty("query", "string", "可选查询词，用于计算针对特定查询的得分", false).
		Build()

	server.RegisterTool(statusTool, t.handleGetEngineStatus)

	// 注册配额报告工具（新）
	quotaTool := mcp.NewToolBuilder("get_quota_report", "获取搜索引擎配额使用报告").
		Build()

	server.RegisterTool(quotaTool, t.handleGetQuotaReport)

	// 注册查询分类工具（新）
	classifyTool := mcp.NewToolBuilder("classify_query", "分析查询类型，返回分类结果和推荐引擎").
		AddProperty("query", "string", "要分类的查询词", true).
		Build()

	server.RegisterTool(classifyTool, t.handleClassifyQuery)
}

// handleWebSearch 处理搜索请求
func (t *WebSearchTool) handleWebSearch(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	preferred, _ := args["preferred_engine"].(string)

	maxResults := t.config.MaxResults
	if v, ok := args["max_results"].(float64); ok && v > 0 {
		maxResults = int(v)
	}

	autoFallback := t.config.AutoFallback
	if v, ok := args["auto_fallback"].(bool); ok {
		autoFallback = v
	}

	// 执行搜索
	req := &engines.SearchRequest{
		Query:      query,
		MaxResults: maxResults,
	}

	result, err := t.router.Route(ctx, req, preferred, autoFallback)
	if err != nil {
		return nil, err
	}

	return SearchResponse{
		Query:             query,
		Results:           result.Results,
		EngineUsed:        result.EngineUsed,
		FallbackTriggered: result.FallbackTriggered,
		OriginalEngine:    result.OriginalEngine,
		LatencyMs:         result.Latency.Milliseconds(),
	}, nil
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Query             string                  `json:"query"`
	Results           []*engines.SearchResult `json:"results"`
	EngineUsed        string                  `json:"engine_used"`
	FallbackTriggered bool                    `json:"fallback_triggered"`
	OriginalEngine    string                  `json:"original_engine,omitempty"`
	LatencyMs         int64                   `json:"latency_ms"`
}

// handleCheckHealth 处理健康检查请求
func (t *WebSearchTool) handleCheckHealth(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	engineName, _ := args["engine_name"].(string)

	if engineName != "" {
		// 检查特定引擎
		t.router.HealthChecker.CheckEngineNow(engineName)
	}

	// 返回所有引擎状态
	return t.router.HealthChecker.GetHealthStatus(), nil
}

// handleSetStrategy 处理设置路由策略请求
func (t *WebSearchTool) handleSetStrategy(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	strategy, _ := args["strategy"].(string)
	if strategy == "" {
		return nil, fmt.Errorf("strategy is required")
	}

	t.router.SetStrategy(router.Strategy(strategy))

	return map[string]interface{}{
		"success":  true,
		"strategy": strategy,
	}, nil
}

// handleExtractContent 处理内容提取请求
func (t *WebSearchTool) handleExtractContent(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}

	extractType, _ := args["extract_type"].(string)
	if extractType == "" {
		extractType = "text"
	}

	maxLength := 10000
	if v, ok := args["max_length"].(float64); ok && v > 0 {
		maxLength = int(v)
	}

	return t.extractor.Extract(ctx, &ExtractRequest{
		URL:         url,
		ExtractType: extractType,
		MaxLength:   maxLength,
	})
}

// handleGetEngineStatus 处理获取引擎状态请求
func (t *WebSearchTool) handleGetEngineStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)

	// 获取基础状态
	engineStates := t.router.GetEngineStates()

	// 获取配额状态
	quotaStatus := t.router.GetQuotaManager().GetAllQuotaStatus()

	// 构建响应
	type EngineStatusResponse struct {
		Name         string                  `json:"name"`
		Status       string                  `json:"status"`
		Priority     int                     `json:"priority"`
		QuotaStatus  string                  `json:"quota_status"`
		QuotaHealth  float64                 `json:"quota_health"`
		MonthlyUsed  int64                   `json:"monthly_used"`
		MonthlyLimit int64                   `json:"monthly_limit"`
		IsFree       bool                    `json:"is_free"`
		Score        float64                 `json:"score,omitempty"`
		ScoreDetails *router.ScoreDetails    `json:"score_details,omitempty"`
		Metrics      interface{}             `json:"metrics"`
	}

	result := make(map[string]*EngineStatusResponse)

	for name, state := range engineStates {
		resp := &EngineStatusResponse{
			Name:     name,
			Status:   state.Status.String(),
			Priority: state.Priority,
			Metrics:  state.Metrics,
		}

		if quota, ok := quotaStatus[name]; ok {
			resp.QuotaStatus = string(quota.Status)
			resp.QuotaHealth = quota.Health
			resp.MonthlyUsed = quota.MonthlyUsed
			resp.MonthlyLimit = quota.MonthlyLimit
			resp.IsFree = quota.IsFree
		}

		// 如果提供了查询，计算得分
		if query != "" {
			details := t.router.GetScorer().ScoreWithDetails(name, query)
			resp.Score = details.FinalScore
			resp.ScoreDetails = details
		}

		result[name] = resp
	}

	return map[string]interface{}{
		"engines":         result,
		"current_strategy": t.config.Router.Strategy,
		"query":           query,
	}, nil
}

// handleGetQuotaReport 处理获取配额报告请求
func (t *WebSearchTool) handleGetQuotaReport(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	quotaStatus := t.router.GetQuotaManager().GetAllQuotaStatus()

	// 计算总体统计
	var totalUsed, totalLimit int64
	var freeCount, paidCount int
	var exhaustedEngines []string
	var warningEngines []string

	for name, quota := range quotaStatus {
		if quota.IsFree {
			freeCount++
		} else {
			paidCount++
			totalUsed += quota.MonthlyUsed
			totalLimit += quota.MonthlyLimit
		}

		switch quota.Status {
		case router.QuotaExhausted:
			exhaustedEngines = append(exhaustedEngines, name)
		case router.QuotaWarning, router.QuotaCritical:
			warningEngines = append(warningEngines, name)
		}
	}

	return map[string]interface{}{
		"quotas": quotaStatus,
		"summary": map[string]interface{}{
			"total_paid_engines":  paidCount,
			"total_free_engines":  freeCount,
			"total_paid_used":     totalUsed,
			"total_paid_limit":    totalLimit,
			"exhausted_engines":   exhaustedEngines,
			"warning_engines":     warningEngines,
		},
	}, nil
}

// handleClassifyQuery 处理查询分类请求
func (t *WebSearchTool) handleClassifyQuery(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// 分类查询
	classification := t.router.GetClassifier().Classify(query)

	// 获取各引擎得分
	scores := t.router.GetEngineScores(query)

	// 按得分排序推荐引擎
	type engineScore struct {
		Name  string  `json:"name"`
		Score float64 `json:"score"`
	}
	var recommendations []engineScore
	for name, details := range scores {
		recommendations = append(recommendations, engineScore{name, details.FinalScore})
	}
	// 排序
	for i := 0; i < len(recommendations)-1; i++ {
		for j := i + 1; j < len(recommendations); j++ {
			if recommendations[i].Score < recommendations[j].Score {
				recommendations[i], recommendations[j] = recommendations[j], recommendations[i]
			}
		}
	}

	return map[string]interface{}{
		"query":           query,
		"classification":  classification,
		"recommendations": recommendations,
	}, nil
}
