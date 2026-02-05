// Package engines SearXNG 开源元搜索引擎
// SearXNG 是免费的开源搜索引擎，聚合多个搜索源
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

// SearXNGEngine SearXNG 搜索引擎
type SearXNGEngine struct {
	client   *http.Client
	endpoint string // SearXNG 实例地址
	priority int
}

// 公共 SearXNG 实例列表
var PublicSearXNGInstances = []string{
	"https://searx.be",
	"https://search.bus-hit.me",
	"https://searx.tiekoetter.com",
	"https://searx.work",
	"https://search.ononoki.org",
}

// NewSearXNGEngine 创建 SearXNG 引擎
func NewSearXNGEngine(endpoint string, priority int) *SearXNGEngine {
	if endpoint == "" {
		endpoint = PublicSearXNGInstances[0]
	}
	return &SearXNGEngine{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		endpoint: endpoint,
		priority: priority,
	}
}

// Name 返回引擎名称
func (e *SearXNGEngine) Name() string {
	return "searxng"
}

// Priority 返回优先级
func (e *SearXNGEngine) Priority() int {
	return e.priority
}

// Search 执行搜索
func (e *SearXNGEngine) Search(ctx context.Context, req *SearchRequest) ([]*SearchResult, error) {
	return e.searchWithInstance(ctx, req, e.endpoint, 0)
}

// searchWithInstance 使用指定实例搜索
func (e *SearXNGEngine) searchWithInstance(ctx context.Context, req *SearchRequest, instance string, retry int) ([]*SearchResult, error) {
	if retry >= len(PublicSearXNGInstances) {
		return nil, fmt.Errorf("all SearXNG instances failed")
	}

	apiURL := instance + "/search"

	params := url.Values{}
	params.Set("q", req.Query)
	params.Set("format", "json")
	params.Set("categories", "general")

	fullURL := apiURL + "?" + params.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AgentTools/1.0)")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		// 尝试下一个公共实例
		nextRetry := retry + 1
		if nextRetry < len(PublicSearXNGInstances) {
			nextInstance := PublicSearXNGInstances[nextRetry]
			return e.searchWithInstance(ctx, req, nextInstance, nextRetry)
		}
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

	var searxResp searxngResponse
	if err := json.Unmarshal(body, &searxResp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	results := make([]*SearchResult, 0)

	for _, r := range searxResp.Results {
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
func (e *SearXNGEngine) HealthCheck(ctx context.Context) error {
	_, err := e.Search(ctx, &SearchRequest{
		Query:      "test",
		MaxResults: 1,
	})
	return err
}

// searxngResponse SearXNG API 响应
type searxngResponse struct {
	Query   string         `json:"query"`
	Results []searxResult  `json:"results"`
}

type searxResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
	Engine  string `json:"engine"`
}
