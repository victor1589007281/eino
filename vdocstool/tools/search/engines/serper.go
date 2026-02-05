// Package engines Serper API 搜索引擎（Google 代理）
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

// SerperEngine Serper API 搜索引擎
type SerperEngine struct {
	client   *http.Client
	apiKey   string
	priority int
}

// NewSerperEngine 创建 Serper 引擎
func NewSerperEngine(apiKey string, priority int) *SerperEngine {
	return &SerperEngine{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey:   apiKey,
		priority: priority,
	}
}

// Name 返回引擎名称
func (e *SerperEngine) Name() string {
	return "serper"
}

// Priority 返回优先级
func (e *SerperEngine) Priority() int {
	return e.priority
}

// Search 执行搜索
func (e *SerperEngine) Search(ctx context.Context, req *SearchRequest) ([]*SearchResult, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("serper API key not configured")
	}

	apiURL := "https://google.serper.dev/search"

	// 构建请求体
	requestBody := serperRequest{
		Q:   req.Query,
		Num: req.MaxResults,
	}

	if req.Language != "" {
		requestBody.HL = req.Language
	}
	if req.Region != "" {
		requestBody.GL = req.Region
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
	httpReq.Header.Set("X-API-KEY", e.apiKey)

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

	var serperResp serperResponse
	if err := json.Unmarshal(body, &serperResp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	results := make([]*SearchResult, 0)

	// 添加知识图谱结果
	if serperResp.KnowledgeGraph.Title != "" {
		results = append(results, &SearchResult{
			Title:   serperResp.KnowledgeGraph.Title,
			URL:     serperResp.KnowledgeGraph.Website,
			Snippet: serperResp.KnowledgeGraph.Description,
		})
	}

	// 添加有机搜索结果
	for _, organic := range serperResp.Organic {
		if len(results) >= req.MaxResults {
			break
		}
		results = append(results, &SearchResult{
			Title:   organic.Title,
			URL:     organic.Link,
			Snippet: organic.Snippet,
		})
	}

	// 添加相关问题
	for _, paa := range serperResp.PeopleAlsoAsk {
		if len(results) >= req.MaxResults {
			break
		}
		results = append(results, &SearchResult{
			Title:   paa.Question,
			URL:     paa.Link,
			Snippet: paa.Snippet,
		})
	}

	return results, nil
}

// HealthCheck 健康检查
func (e *SerperEngine) HealthCheck(ctx context.Context) error {
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

// serperRequest Serper API 请求
type serperRequest struct {
	Q   string `json:"q"`
	Num int    `json:"num,omitempty"`
	HL  string `json:"hl,omitempty"` // 语言
	GL  string `json:"gl,omitempty"` // 地区
}

// serperResponse Serper API 响应
type serperResponse struct {
	SearchParameters searchParameters `json:"searchParameters"`
	KnowledgeGraph   knowledgeGraph   `json:"knowledgeGraph"`
	Organic          []organicResult  `json:"organic"`
	PeopleAlsoAsk    []paaResult      `json:"peopleAlsoAsk"`
	RelatedSearches  []relatedSearch  `json:"relatedSearches"`
}

type searchParameters struct {
	Q    string `json:"q"`
	Type string `json:"type"`
}

type knowledgeGraph struct {
	Title       string `json:"title"`
	Type        string `json:"type"`
	Website     string `json:"website"`
	Description string `json:"description"`
}

type organicResult struct {
	Title    string `json:"title"`
	Link     string `json:"link"`
	Snippet  string `json:"snippet"`
	Position int    `json:"position"`
}

type paaResult struct {
	Question string `json:"question"`
	Snippet  string `json:"snippet"`
	Title    string `json:"title"`
	Link     string `json:"link"`
}

type relatedSearch struct {
	Query string `json:"query"`
}
