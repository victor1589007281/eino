// Package engines Brave Search API
// Brave Search 提供免费额度（每月2000次请求）
package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// BraveEngine Brave Search 引擎
type BraveEngine struct {
	client   *http.Client
	apiKey   string
	priority int
}

// NewBraveEngine 创建 Brave 引擎
func NewBraveEngine(apiKey string, priority int) *BraveEngine {
	return &BraveEngine{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey:   apiKey,
		priority: priority,
	}
}

// Name 返回引擎名称
func (e *BraveEngine) Name() string {
	return "brave"
}

// Priority 返回优先级
func (e *BraveEngine) Priority() int {
	return e.priority
}

// Search 执行搜索
func (e *BraveEngine) Search(ctx context.Context, req *SearchRequest) ([]*SearchResult, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("brave API key not configured")
	}

	apiURL := "https://api.search.brave.com/res/v1/web/search"

	params := url.Values{}
	params.Set("q", req.Query)
	params.Set("count", fmt.Sprintf("%d", req.MaxResults))

	fullURL := apiURL + "?" + params.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-Subscription-Token", e.apiKey)

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

	var braveResp braveResponse
	if err := json.Unmarshal(body, &braveResp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	results := make([]*SearchResult, 0)

	// 添加搜索结果
	for _, r := range braveResp.Web.Results {
		if len(results) >= req.MaxResults {
			break
		}
		results = append(results, &SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Description,
		})
	}

	return results, nil
}

// HealthCheck 健康检查
func (e *BraveEngine) HealthCheck(ctx context.Context) error {
	if e.apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	_, err := e.Search(ctx, &SearchRequest{
		Query:      "test",
		MaxResults: 1,
	})
	return err
}

// braveResponse Brave API 响应
type braveResponse struct {
	Query   braveQuery   `json:"query"`
	Web     braveWeb     `json:"web"`
}

type braveQuery struct {
	Original string `json:"original"`
}

type braveWeb struct {
	Results []braveResult `json:"results"`
}

type braveResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}
