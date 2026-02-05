// Package engines Bing 搜索引擎
package engines

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// BingEngine Bing 搜索引擎
type BingEngine struct {
	client   *http.Client
	priority int
	region   string // cn 或 global
}

// NewBingEngine 创建 Bing 引擎
func NewBingEngine(priority int, region string) *BingEngine {
	if region == "" {
		region = "cn"
	}
	return &BingEngine{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		priority: priority,
		region:   region,
	}
}

// Name 返回引擎名称
func (e *BingEngine) Name() string {
	return "bing"
}

// Priority 返回优先级
func (e *BingEngine) Priority() int {
	return e.priority
}

// Search 执行搜索
func (e *BingEngine) Search(ctx context.Context, req *SearchRequest) ([]*SearchResult, error) {
	// 选择域名
	var baseURL string
	if e.region == "cn" {
		baseURL = "https://cn.bing.com/search"
	} else {
		baseURL = "https://www.bing.com/search"
	}

	params := url.Values{}
	params.Set("q", req.Query)
	params.Set("count", fmt.Sprintf("%d", req.MaxResults))

	fullURL := baseURL + "?" + params.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	httpReq.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

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

// parseHTML 解析 HTML 响应
func (e *BingEngine) parseHTML(body io.Reader, maxResults int) ([]*SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML failed: %w", err)
	}

	results := make([]*SearchResult, 0)

	// 解析搜索结果
	doc.Find("#b_results .b_algo").Each(func(i int, s *goquery.Selection) {
		if len(results) >= maxResults {
			return
		}

		// 提取标题和链接
		titleElem := s.Find("h2 a")
		title := strings.TrimSpace(titleElem.Text())
		link, _ := titleElem.Attr("href")

		// 提取摘要
		snippet := strings.TrimSpace(s.Find(".b_caption p").Text())

		if title != "" && link != "" {
			results = append(results, &SearchResult{
				Title:   title,
				URL:     link,
				Snippet: snippet,
			})
		}
	})

	return results, nil
}

// HealthCheck 健康检查
func (e *BingEngine) HealthCheck(ctx context.Context) error {
	var baseURL string
	if e.region == "cn" {
		baseURL = "https://cn.bing.com"
	} else {
		baseURL = "https://www.bing.com"
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", baseURL, nil)
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

// cleanText 清理文本
func cleanText(text string) string {
	// 移除多余空白
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}
