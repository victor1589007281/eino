// Package engines Tavily AI 搜索引擎
// Tavily 是专为 AI Agent 设计的搜索 API
package engines

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TavilyEngine Tavily 搜索引擎
type TavilyEngine struct {
	client   *http.Client
	apiKey   string
	priority int
}

// NewTavilyEngine 创建 Tavily 引擎
func NewTavilyEngine(apiKey string, priority int) *TavilyEngine {
	return &TavilyEngine{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey:   apiKey,
		priority: priority,
	}
}

// Name 返回引擎名称
func (e *TavilyEngine) Name() string {
	return "tavily"
}

// Priority 返回优先级
func (e *TavilyEngine) Priority() int {
	return e.priority
}

// Search 执行搜索
func (e *TavilyEngine) Search(ctx context.Context, req *SearchRequest) ([]*SearchResult, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("tavily API key not configured")
	}

	apiURL := "https://api.tavily.com/search"

	// 构建请求体
	requestBody := tavilyRequest{
		APIKey:            e.apiKey,
		Query:             req.Query,
		SearchDepth:       "basic", // basic 或 advanced
		IncludeAnswer:     true,
		IncludeRawContent: false,
		MaxResults:        req.MaxResults,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var tavilyResp tavilyResponse
	if err := json.Unmarshal(body, &tavilyResp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	results := make([]*SearchResult, 0)

	// 添加搜索结果
	for _, r := range tavilyResp.Results {
		if len(results) >= req.MaxResults {
			break
		}
		results = append(results, &SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}

	return results, nil
}

// HealthCheck 健康检查
func (e *TavilyEngine) HealthCheck(ctx context.Context) error {
	if e.apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	// 执行一个简单的搜索来验证 API
	_, err := e.Search(ctx, &SearchRequest{
		Query:      "test",
		MaxResults: 1,
	})
	return err
}

// tavilyRequest Tavily API 请求
type tavilyRequest struct {
	APIKey            string   `json:"api_key"`
	Query             string   `json:"query"`
	SearchDepth       string   `json:"search_depth,omitempty"`       // basic 或 advanced
	IncludeAnswer     bool     `json:"include_answer,omitempty"`     // 是否包含AI答案
	IncludeRawContent bool     `json:"include_raw_content,omitempty"` // 是否包含原始内容
	MaxResults        int      `json:"max_results,omitempty"`
	IncludeDomains    []string `json:"include_domains,omitempty"`    // 限制搜索域名
	ExcludeDomains    []string `json:"exclude_domains,omitempty"`    // 排除域名
}

// tavilyResponse Tavily API 响应
type tavilyResponse struct {
	Query   string         `json:"query"`
	Answer  string         `json:"answer"`
	Results []tavilyResult `json:"results"`
}

type tavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}
