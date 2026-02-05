// Package engines 百度搜索引擎
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

// BaiduEngine 百度搜索引擎
type BaiduEngine struct {
	client   *http.Client
	priority int
}

// NewBaiduEngine 创建百度引擎
func NewBaiduEngine(priority int) *BaiduEngine {
	return &BaiduEngine{
		client: &http.Client{
			Timeout: 30 * time.Second,
			// 不自动跟随重定向，手动处理
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		priority: priority,
	}
}

// Name 返回引擎名称
func (e *BaiduEngine) Name() string {
	return "baidu"
}

// Priority 返回优先级
func (e *BaiduEngine) Priority() int {
	return e.priority
}

// Search 执行搜索
func (e *BaiduEngine) Search(ctx context.Context, req *SearchRequest) ([]*SearchResult, error) {
	baseURL := "https://www.baidu.com/s"

	params := url.Values{}
	params.Set("wd", req.Query)
	params.Set("rn", fmt.Sprintf("%d", req.MaxResults)) // 每页结果数
	params.Set("ie", "utf-8")

	fullURL := baseURL + "?" + params.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	// 设置请求头，模拟浏览器
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	httpReq.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	httpReq.Header.Set("Cookie", "BAIDUID=random")

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

// parseHTML 解析百度 HTML 响应
func (e *BaiduEngine) parseHTML(body io.Reader, maxResults int) ([]*SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML failed: %w", err)
	}

	results := make([]*SearchResult, 0)

	// 解析搜索结果 - 百度的结果在 .result 或 .c-container 中
	doc.Find("#content_left .result, #content_left .c-container").Each(func(i int, s *goquery.Selection) {
		if len(results) >= maxResults {
			return
		}

		// 提取标题
		titleElem := s.Find("h3 a").First()
		if titleElem.Length() == 0 {
			titleElem = s.Find(".t a").First()
		}

		title := strings.TrimSpace(titleElem.Text())
		link, exists := titleElem.Attr("href")

		if !exists || title == "" {
			return
		}

		// 提取摘要
		snippet := ""
		snippetElem := s.Find(".c-abstract")
		if snippetElem.Length() > 0 {
			snippet = strings.TrimSpace(snippetElem.Text())
		} else {
			snippetElem = s.Find(".c-span-last")
			if snippetElem.Length() > 0 {
				snippet = strings.TrimSpace(snippetElem.Text())
			}
		}

		// 百度的链接是重定向链接，需要解析真实URL
		// 这里我们保留原始链接，实际使用时可以进一步解析
		results = append(results, &SearchResult{
			Title:   cleanBaiduText(title),
			URL:     link,
			Snippet: cleanBaiduText(snippet),
		})
	})

	return results, nil
}

// HealthCheck 健康检查
func (e *BaiduEngine) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", "https://www.baidu.com", nil)
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

// cleanBaiduText 清理百度文本
func cleanBaiduText(text string) string {
	// 移除百度的高亮标签
	text = strings.ReplaceAll(text, "<em>", "")
	text = strings.ReplaceAll(text, "</em>", "")

	// 移除多余空白
	text = strings.Join(strings.Fields(text), " ")

	return strings.TrimSpace(text)
}

// ResolveBaiduLink 解析百度重定向链接（获取真实URL）
func ResolveBaiduLink(ctx context.Context, baiduLink string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 返回错误以获取重定向目标
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", baiduLink, nil)
	if err != nil {
		return baiduLink, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AgentTools/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return baiduLink, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {
			return location, nil
		}
	}

	return baiduLink, nil
}
