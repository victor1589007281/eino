// Package engines DuckDuckGo 搜索引擎
package engines

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// DuckDuckGoEngine DuckDuckGo 搜索引擎
type DuckDuckGoEngine struct {
	client   *http.Client
	priority int
}

// NewDuckDuckGoEngine 创建 DuckDuckGo 引擎
func NewDuckDuckGoEngine(priority int) *DuckDuckGoEngine {
	return &DuckDuckGoEngine{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		priority: priority,
	}
}

// Name 返回引擎名称
func (e *DuckDuckGoEngine) Name() string {
	return "duckduckgo"
}

// Priority 返回优先级
func (e *DuckDuckGoEngine) Priority() int {
	return e.priority
}

// Search 执行搜索
func (e *DuckDuckGoEngine) Search(ctx context.Context, req *SearchRequest) ([]*SearchResult, error) {
	// 使用 DuckDuckGo HTML 搜索
	baseURL := "https://html.duckduckgo.com/html/"

	params := url.Values{}
	params.Set("q", req.Query)

	fullURL := baseURL + "?" + params.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	// 设置请求头，模拟浏览器
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return e.parseHTML(resp.Body, req.MaxResults)
}

// parseHTML 解析 DuckDuckGo HTML 响应
func (e *DuckDuckGoEngine) parseHTML(body io.Reader, maxResults int) ([]*SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML failed: %w", err)
	}

	results := make([]*SearchResult, 0)

	// DuckDuckGo HTML 版本的结果在 .result 中
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if len(results) >= maxResults {
			return
		}

		// 提取标题和链接
		titleElem := s.Find(".result__title .result__a")
		title := strings.TrimSpace(titleElem.Text())
		link, exists := titleElem.Attr("href")

		if !exists || title == "" {
			return
		}

		// 提取摘要
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())

		// 清理链接（DuckDuckGo 可能有重定向链接）
		cleanedLink := cleanDDGLink(link)

		results = append(results, &SearchResult{
			Title:   title,
			URL:     cleanedLink,
			Snippet: snippet,
		})
	})

	return results, nil
}

// cleanDDGLink 清理 DuckDuckGo 链接
func cleanDDGLink(link string) string {
	// DuckDuckGo 的链接格式：//duckduckgo.com/l/?uddg=实际URL
	if strings.Contains(link, "uddg=") {
		if u, err := url.Parse(link); err == nil {
			if uddg := u.Query().Get("uddg"); uddg != "" {
				if decoded, err := url.QueryUnescape(uddg); err == nil {
					return decoded
				}
			}
		}
	}
	
	// 处理相对链接
	if strings.HasPrefix(link, "//") {
		return "https:" + link
	}
	
	return link
}

// HealthCheck 健康检查
func (e *DuckDuckGoEngine) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", "https://html.duckduckgo.com/html/", nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AgentTools/1.0)")

	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}
