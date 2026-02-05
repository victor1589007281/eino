// Package search 网页内容提取器
package search

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// WebExtractor 网页内容提取器
type WebExtractor struct {
	client *http.Client
}

// NewWebExtractor 创建网页提取器
func NewWebExtractor() *WebExtractor {
	return &WebExtractor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ExtractRequest 提取请求
type ExtractRequest struct {
	URL         string `json:"url"`
	ExtractType string `json:"extract_type"` // text, html, metadata
	MaxLength   int    `json:"max_length"`
}

// ExtractResult 提取结果
type ExtractResult struct {
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Metadata    map[string]string `json:"metadata"`
	ContentType string            `json:"content_type"`
	StatusCode  int               `json:"status_code"`
}

// Extract 提取网页内容
func (e *WebExtractor) Extract(ctx context.Context, req *ExtractRequest) (*ExtractResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	result := &ExtractResult{
		URL:         req.URL,
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Metadata:    make(map[string]string),
	}

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 检查内容类型
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml") {
		// 非 HTML 内容，读取原始文本
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return result, fmt.Errorf("read body failed: %w", err)
		}
		result.Content = truncateContent(string(body), req.MaxLength)
		return result, nil
	}

	// 解析 HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return result, fmt.Errorf("parse HTML failed: %w", err)
	}

	// 提取标题
	result.Title = strings.TrimSpace(doc.Find("title").Text())

	// 提取元数据
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		property, _ := s.Attr("property")
		content, _ := s.Attr("content")

		if name != "" && content != "" {
			result.Metadata[name] = content
		}
		if property != "" && content != "" {
			result.Metadata[property] = content
		}
	})

	// 根据提取类型返回内容
	switch req.ExtractType {
	case "html":
		html, _ := doc.Html()
		result.Content = truncateContent(html, req.MaxLength)
	case "metadata":
		// 只返回元数据，不提取内容
	default:
		// 提取纯文本
		result.Content = e.extractText(doc, req.MaxLength)
	}

	return result, nil
}

// extractText 提取文本内容
func (e *WebExtractor) extractText(doc *goquery.Document, maxLength int) string {
	// 移除不需要的元素
	doc.Find("script, style, noscript, iframe, nav, footer, header, aside").Remove()

	// 优先提取文章内容
	var content string

	// 尝试常见的文章容器
	selectors := []string{
		"article",
		"main",
		".article-content",
		".post-content",
		".entry-content",
		".content",
		"#content",
		".main-content",
	}

	for _, selector := range selectors {
		elem := doc.Find(selector)
		if elem.Length() > 0 {
			content = strings.TrimSpace(elem.Text())
			if len(content) > 100 {
				break
			}
		}
	}

	// 如果没找到，提取 body
	if content == "" {
		content = strings.TrimSpace(doc.Find("body").Text())
	}

	// 清理文本
	content = cleanExtractedText(content)

	return truncateContent(content, maxLength)
}

// cleanExtractedText 清理提取的文本
func cleanExtractedText(text string) string {
	// 替换多个空白为单个空格
	lines := strings.Split(text, "\n")
	var cleanLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// 合并连续空格
			line = strings.Join(strings.Fields(line), " ")
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}

// truncateContent 截断内容
func truncateContent(content string, maxLength int) string {
	if maxLength <= 0 {
		maxLength = 10000
	}

	if len(content) <= maxLength {
		return content
	}

	// 尝试在句子边界截断
	truncated := content[:maxLength]
	lastSentence := strings.LastIndexAny(truncated, ".!?。！？")
	if lastSentence > maxLength/2 {
		return truncated[:lastSentence+1]
	}

	// 在词边界截断
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLength/2 {
		return truncated[:lastSpace] + "..."
	}

	return truncated + "..."
}

// ExtractMultiple 批量提取
func (e *WebExtractor) ExtractMultiple(ctx context.Context, urls []string, extractType string, maxLength int) ([]*ExtractResult, error) {
	results := make([]*ExtractResult, 0, len(urls))

	for _, url := range urls {
		result, err := e.Extract(ctx, &ExtractRequest{
			URL:         url,
			ExtractType: extractType,
			MaxLength:   maxLength,
		})
		if err != nil {
			// 记录错误但继续处理
			results = append(results, &ExtractResult{
				URL:      url,
				Metadata: map[string]string{"error": err.Error()},
			})
			continue
		}
		results = append(results, result)
	}

	return results, nil
}
