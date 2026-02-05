// Package engines Exa.ai 搜索引擎
// Exa 是基于嵌入向量的语义搜索引擎
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

// ExaEngine Exa.ai 搜索引擎
type ExaEngine struct {
	client   *http.Client
	apiKey   string
	priority int
}

// NewExaEngine 创建 Exa 引擎
func NewExaEngine(apiKey string, priority int) *ExaEngine {
	return &ExaEngine{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey:   apiKey,
		priority: priority,
	}
}

// Name 返回引擎名称
func (e *ExaEngine) Name() string {
	return "exa"
}

// Priority 返回优先级
func (e *ExaEngine) Priority() int {
	return e.priority
}

// Search 执行搜索
func (e *ExaEngine) Search(ctx context.Context, req *SearchRequest) ([]*SearchResult, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("exa API key not configured")
	}

	apiURL := "https://api.exa.ai/search"

	// 构建请求体
	requestBody := exaRequest{
		Query:      req.Query,
		NumResults: req.MaxResults,
		Type:       "neural", // neural 或 keyword
		Contents: &exaContents{
			Text: true,
		},
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
	httpReq.Header.Set("x-api-key", e.apiKey)

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

	var exaResp exaResponse
	if err := json.Unmarshal(body, &exaResp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	results := make([]*SearchResult, 0)

	// 添加搜索结果
	for _, r := range exaResp.Results {
		if len(results) >= req.MaxResults {
			break
		}
		
		snippet := r.Text
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}
		
		results = append(results, &SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: snippet,
		})
	}

	return results, nil
}

// HealthCheck 健康检查
func (e *ExaEngine) HealthCheck(ctx context.Context) error {
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

// exaRequest Exa API 请求
type exaRequest struct {
	Query            string       `json:"query"`
	NumResults       int          `json:"numResults,omitempty"`
	Type             string       `json:"type,omitempty"`             // neural 或 keyword
	UseAutoprompt    bool         `json:"useAutoprompt,omitempty"`    // 自动优化查询
	IncludeDomains   []string     `json:"includeDomains,omitempty"`   // 限制搜索域名
	ExcludeDomains   []string     `json:"excludeDomains,omitempty"`   // 排除域名
	StartCrawlDate   string       `json:"startCrawlDate,omitempty"`   // 开始抓取日期
	EndCrawlDate     string       `json:"endCrawlDate,omitempty"`     // 结束抓取日期
	StartPublishedDate string     `json:"startPublishedDate,omitempty"` // 开始发布日期
	EndPublishedDate   string     `json:"endPublishedDate,omitempty"`   // 结束发布日期
	Contents         *exaContents `json:"contents,omitempty"`
}

type exaContents struct {
	Text      bool `json:"text,omitempty"`
	Highlights *exaHighlights `json:"highlights,omitempty"`
}

type exaHighlights struct {
	NumSentences int    `json:"numSentences,omitempty"`
	Query        string `json:"query,omitempty"`
}

// exaResponse Exa API 响应
type exaResponse struct {
	Results []exaResult `json:"results"`
}

type exaResult struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	ID          string  `json:"id"`
	Score       float64 `json:"score"`
	PublishedDate string `json:"publishedDate"`
	Author      string  `json:"author"`
	Text        string  `json:"text"`
}
