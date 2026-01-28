// Package websearch provides web search capabilities.
package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SearchEngine represents a search engine type.
type SearchEngine string

const (
	SearchEngineBing    SearchEngine = "bing"
	SearchEngineGoogle  SearchEngine = "google"
	SearchEngineBaidu   SearchEngine = "baidu"
	SearchEngineSogou   SearchEngine = "sogou"
	SearchEngineSerpAPI SearchEngine = "serpapi"
)

// SearchConfig represents search configuration.
type SearchConfig struct {
	Engine     SearchEngine `json:"engine"`
	APIKey     string       `json:"api_key"`
	BaseURL    string       `json:"base_url"`
	MaxResults int          `json:"max_results"`
	Timeout    time.Duration `json:"timeout"`
	Language   string       `json:"language"`
	Region     string       `json:"region"`
}

// SearchResult represents a search result.
type SearchResult struct {
	Title       string            `json:"title"`
	URL         string            `json:"url"`
	Snippet     string            `json:"snippet"`
	Source      string            `json:"source"`
	PublishDate time.Time         `json:"publish_date,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SearchResponse represents a search response.
type SearchResponse struct {
	Query      string         `json:"query"`
	Results    []SearchResult `json:"results"`
	TotalCount int            `json:"total_count"`
	Duration   time.Duration  `json:"duration"`
	Error      string         `json:"error,omitempty"`
}

// Searcher defines the search interface.
type Searcher interface {
	Search(ctx context.Context, query string, opts *SearchOptions) (*SearchResponse, error)
}

// SearchOptions represents search options.
type SearchOptions struct {
	MaxResults int
	Site       string   // Limit to specific site
	FileType   string   // Limit to specific file type
	DateRange  string   // Date range filter (e.g., "past_year")
	Language   string
	Region     string
	SafeSearch bool
	Keywords   []string // Additional keywords
}

// WebSearchTool implements web search functionality.
type WebSearchTool struct {
	config   *SearchConfig
	client   *http.Client
	searcher Searcher
}

// NewWebSearchTool creates a new web search tool.
func NewWebSearchTool(config *SearchConfig) *WebSearchTool {
	if config.MaxResults == 0 {
		config.MaxResults = 10
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.Language == "" {
		config.Language = "zh-CN"
	}
	
	tool := &WebSearchTool{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
	
	return tool
}

// Search performs a web search.
func (t *WebSearchTool) Search(ctx context.Context, query string, opts *SearchOptions) (*SearchResponse, error) {
	if opts == nil {
		opts = &SearchOptions{
			MaxResults: t.config.MaxResults,
			Language:   t.config.Language,
			Region:     t.config.Region,
		}
	}
	
	start := time.Now()
	
	var results []SearchResult
	var err error
	
	switch t.config.Engine {
	case SearchEngineSerpAPI:
		results, err = t.searchSerpAPI(ctx, query, opts)
	case SearchEngineBing:
		results, err = t.searchBing(ctx, query, opts)
	case SearchEngineBaidu:
		results, err = t.searchBaidu(ctx, query, opts)
	default:
		return nil, fmt.Errorf("unsupported search engine: %s", t.config.Engine)
	}
	
	duration := time.Since(start)
	
	if err != nil {
		return &SearchResponse{
			Query:    query,
			Results:  nil,
			Duration: duration,
			Error:    err.Error(),
		}, nil
	}
	
	return &SearchResponse{
		Query:      query,
		Results:    results,
		TotalCount: len(results),
		Duration:   duration,
	}, nil
}

func (t *WebSearchTool) searchSerpAPI(ctx context.Context, query string, opts *SearchOptions) ([]SearchResult, error) {
	baseURL := "https://serpapi.com/search"
	if t.config.BaseURL != "" {
		baseURL = t.config.BaseURL
	}
	
	params := url.Values{}
	params.Set("q", query)
	params.Set("api_key", t.config.APIKey)
	params.Set("num", fmt.Sprintf("%d", opts.MaxResults))
	params.Set("hl", opts.Language)
	
	if opts.Site != "" {
		params.Set("q", fmt.Sprintf("site:%s %s", opts.Site, query))
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: %s", string(body))
	}
	
	var serpResp struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
			Source  string `json:"source"`
			Date    string `json:"date"`
		} `json:"organic_results"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&serpResp); err != nil {
		return nil, err
	}
	
	var results []SearchResult
	for _, r := range serpResp.OrganicResults {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.Link,
			Snippet: r.Snippet,
			Source:  r.Source,
		})
	}
	
	return results, nil
}

func (t *WebSearchTool) searchBing(ctx context.Context, query string, opts *SearchOptions) ([]SearchResult, error) {
	baseURL := "https://api.bing.microsoft.com/v7.0/search"
	if t.config.BaseURL != "" {
		baseURL = t.config.BaseURL
	}
	
	params := url.Values{}
	params.Set("q", query)
	params.Set("count", fmt.Sprintf("%d", opts.MaxResults))
	params.Set("mkt", opts.Language)
	
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Ocp-Apim-Subscription-Key", t.config.APIKey)
	
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: %s", string(body))
	}
	
	var bingResp struct {
		WebPages struct {
			Value []struct {
				Name           string `json:"name"`
				URL            string `json:"url"`
				Snippet        string `json:"snippet"`
				DateLastCrawled string `json:"dateLastCrawled"`
			} `json:"value"`
		} `json:"webPages"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&bingResp); err != nil {
		return nil, err
	}
	
	var results []SearchResult
	for _, r := range bingResp.WebPages.Value {
		var pubDate time.Time
		if r.DateLastCrawled != "" {
			pubDate, _ = time.Parse(time.RFC3339, r.DateLastCrawled)
		}
		results = append(results, SearchResult{
			Title:       r.Name,
			URL:         r.URL,
			Snippet:     r.Snippet,
			PublishDate: pubDate,
		})
	}
	
	return results, nil
}

func (t *WebSearchTool) searchBaidu(ctx context.Context, query string, opts *SearchOptions) ([]SearchResult, error) {
	// Baidu search implementation
	// Note: Baidu doesn't have an official API, this is a placeholder
	return nil, fmt.Errorf("baidu search not implemented")
}

// SearchInsurance searches for insurance-related content.
func (t *WebSearchTool) SearchInsurance(ctx context.Context, query string) (*SearchResponse, error) {
	// Add insurance-specific keywords
	insuranceQuery := query
	
	// Check if query already contains insurance keywords
	keywords := []string{"保险", "理赔", "条款", "保单", "投保", "核保"}
	hasKeyword := false
	for _, kw := range keywords {
		if strings.Contains(query, kw) {
			hasKeyword = true
			break
		}
	}
	
	if !hasKeyword {
		insuranceQuery = "保险 " + query
	}
	
	return t.Search(ctx, insuranceQuery, &SearchOptions{
		MaxResults: t.config.MaxResults,
		Language:   "zh-CN",
		SafeSearch: true,
	})
}

// SearchLaws searches for insurance laws and regulations.
func (t *WebSearchTool) SearchLaws(ctx context.Context, query string) (*SearchResponse, error) {
	lawQuery := fmt.Sprintf("保险法 法规 %s site:gov.cn OR site:circ.gov.cn OR site:pkulaw.com", query)
	
	return t.Search(ctx, lawQuery, &SearchOptions{
		MaxResults: t.config.MaxResults,
		Language:   "zh-CN",
		SafeSearch: true,
	})
}

// SearchCases searches for insurance claim cases.
func (t *WebSearchTool) SearchCases(ctx context.Context, query string) (*SearchResponse, error) {
	caseQuery := fmt.Sprintf("保险理赔案例 %s site:court.gov.cn OR site:circ.gov.cn", query)
	
	return t.Search(ctx, caseQuery, &SearchOptions{
		MaxResults: t.config.MaxResults,
		Language:   "zh-CN",
		SafeSearch: true,
	})
}

// SearchProducts searches for insurance product information.
func (t *WebSearchTool) SearchProducts(ctx context.Context, productName string) (*SearchResponse, error) {
	productQuery := fmt.Sprintf("%s 保险产品 条款 责任", productName)
	
	return t.Search(ctx, productQuery, &SearchOptions{
		MaxResults: t.config.MaxResults,
		Language:   "zh-CN",
		SafeSearch: true,
	})
}
