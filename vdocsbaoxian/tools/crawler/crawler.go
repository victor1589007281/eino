// Package crawler provides web crawling capabilities.
package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// CrawlerConfig represents crawler configuration.
type CrawlerConfig struct {
	UserAgent       string        `json:"user_agent"`
	Timeout         time.Duration `json:"timeout"`
	MaxDepth        int           `json:"max_depth"`
	MaxPages        int           `json:"max_pages"`
	RateLimit       time.Duration `json:"rate_limit"`
	AllowedDomains  []string      `json:"allowed_domains"`
	DisallowedPaths []string      `json:"disallowed_paths"`
	Proxy           string        `json:"proxy"`
}

// PageContent represents crawled page content.
type PageContent struct {
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Links       []string          `json:"links"`
	Metadata    map[string]string `json:"metadata"`
	CrawledAt   time.Time         `json:"crawled_at"`
	StatusCode  int               `json:"status_code"`
	ContentType string            `json:"content_type"`
}

// Crawler implements web crawling functionality.
type Crawler struct {
	config     *CrawlerConfig
	client     *http.Client
	visited    map[string]bool
	mu         sync.RWMutex
	rateLimiter <-chan time.Time
}

// NewCrawler creates a new crawler.
func NewCrawler(config *CrawlerConfig) *Crawler {
	if config.UserAgent == "" {
		config.UserAgent = "InsuranceExpertBot/1.0"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxDepth == 0 {
		config.MaxDepth = 3
	}
	if config.MaxPages == 0 {
		config.MaxPages = 100
	}
	if config.RateLimit == 0 {
		config.RateLimit = time.Second
	}
	
	return &Crawler{
		config:      config,
		client:      &http.Client{Timeout: config.Timeout},
		visited:     make(map[string]bool),
		rateLimiter: time.Tick(config.RateLimit),
	}
}

// Crawl crawls a single page.
func (c *Crawler) Crawl(ctx context.Context, pageURL string) (*PageContent, error) {
	// Wait for rate limiter
	select {
	case <-c.rateLimiter:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	
	// Parse URL
	u, err := url.Parse(pageURL)
	if err != nil {
		return nil, err
	}
	
	// Check if domain is allowed
	if !c.isDomainAllowed(u.Host) {
		return nil, fmt.Errorf("domain not allowed: %s", u.Host)
	}
	
	// Check if path is disallowed
	if c.isPathDisallowed(u.Path) {
		return nil, fmt.Errorf("path disallowed: %s", u.Path)
	}
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	
	// Perform request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	// Parse HTML
	content := c.parseHTML(string(body), u)
	content.URL = pageURL
	content.CrawledAt = time.Now()
	content.StatusCode = resp.StatusCode
	content.ContentType = resp.Header.Get("Content-Type")
	
	// Mark as visited
	c.mu.Lock()
	c.visited[pageURL] = true
	c.mu.Unlock()
	
	return content, nil
}

// CrawlRecursive crawls pages recursively.
func (c *Crawler) CrawlRecursive(ctx context.Context, startURL string, depth int) ([]*PageContent, error) {
	if depth > c.config.MaxDepth {
		return nil, nil
	}
	
	var results []*PageContent
	var mu sync.Mutex
	
	// Crawl start page
	content, err := c.Crawl(ctx, startURL)
	if err != nil {
		return nil, err
	}
	
	results = append(results, content)
	
	if depth >= c.config.MaxDepth || len(results) >= c.config.MaxPages {
		return results, nil
	}
	
	// Crawl linked pages
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // Limit concurrent crawls
	
	for _, link := range content.Links {
		c.mu.RLock()
		visited := c.visited[link]
		c.mu.RUnlock()
		
		if visited {
			continue
		}
		
		wg.Add(1)
		go func(pageURL string) {
			defer wg.Done()
			
			sem <- struct{}{}
			defer func() { <-sem }()
			
			subResults, err := c.CrawlRecursive(ctx, pageURL, depth+1)
			if err != nil {
				return
			}
			
			mu.Lock()
			results = append(results, subResults...)
			mu.Unlock()
		}(link)
		
		// Check page limit
		mu.Lock()
		if len(results) >= c.config.MaxPages {
			mu.Unlock()
			break
		}
		mu.Unlock()
	}
	
	wg.Wait()
	return results, nil
}

func (c *Crawler) isDomainAllowed(domain string) bool {
	if len(c.config.AllowedDomains) == 0 {
		return true
	}
	
	for _, allowed := range c.config.AllowedDomains {
		if strings.HasSuffix(domain, allowed) {
			return true
		}
	}
	return false
}

func (c *Crawler) isPathDisallowed(path string) bool {
	for _, disallowed := range c.config.DisallowedPaths {
		if strings.HasPrefix(path, disallowed) {
			return true
		}
	}
	return false
}

func (c *Crawler) parseHTML(htmlContent string, baseURL *url.URL) *PageContent {
	content := &PageContent{
		Links:    make([]string, 0),
		Metadata: make(map[string]string),
	}
	
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		content.Content = c.extractText(htmlContent)
		return content
	}
	
	var extractText func(*html.Node)
	var textBuilder strings.Builder
	
	extractText = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Extract title
			if n.Data == "title" && n.FirstChild != nil {
				content.Title = n.FirstChild.Data
			}
			
			// Extract meta tags
			if n.Data == "meta" {
				var name, metaContent string
				for _, attr := range n.Attr {
					if attr.Key == "name" || attr.Key == "property" {
						name = attr.Val
					}
					if attr.Key == "content" {
						metaContent = attr.Val
					}
				}
				if name != "" && metaContent != "" {
					content.Metadata[name] = metaContent
				}
			}
			
			// Extract links
			if n.Data == "a" {
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						link := c.resolveURL(attr.Val, baseURL)
						if link != "" {
							content.Links = append(content.Links, link)
						}
					}
				}
			}
			
			// Skip script and style
			if n.Data == "script" || n.Data == "style" {
				return
			}
		}
		
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				textBuilder.WriteString(text)
				textBuilder.WriteString(" ")
			}
		}
		
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			extractText(child)
		}
	}
	
	extractText(doc)
	content.Content = c.cleanText(textBuilder.String())
	
	return content
}

func (c *Crawler) resolveURL(href string, base *url.URL) string {
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
		return ""
	}
	
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	
	resolved := base.ResolveReference(u)
	
	// Only allow http/https
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	
	return resolved.String()
}

func (c *Crawler) extractText(htmlContent string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(htmlContent, " ")
	return c.cleanText(text)
}

func (c *Crawler) cleanText(text string) string {
	// Remove excessive whitespace
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// ExtractInsuranceInfo extracts insurance-related information from content.
func (c *Crawler) ExtractInsuranceInfo(content *PageContent) *InsuranceInfo {
	info := &InsuranceInfo{
		Source:    content.URL,
		CrawledAt: content.CrawledAt,
	}
	
	text := content.Content
	
	// Extract law references
	lawPattern := regexp.MustCompile(`《[^》]+》(第[一二三四五六七八九十百千\d]+条)?`)
	info.LawReferences = lawPattern.FindAllString(text, -1)
	
	// Extract insurance terms
	termPatterns := []string{
		`保险责任`, `除外责任`, `等待期`, `犹豫期`, `免赔额`,
		`赔付比例`, `保险期间`, `保险金额`, `保险费`, `受益人`,
	}
	
	for _, pattern := range termPatterns {
		re := regexp.MustCompile(pattern + `[：:][^。]+。`)
		matches := re.FindAllString(text, -1)
		info.InsuranceTerms = append(info.InsuranceTerms, matches...)
	}
	
	// Extract amounts
	amountPattern := regexp.MustCompile(`\d+(?:,\d{3})*(?:\.\d+)?(?:元|万|亿)`)
	info.Amounts = amountPattern.FindAllString(text, -1)
	
	// Extract dates
	datePattern := regexp.MustCompile(`\d{4}年\d{1,2}月\d{1,2}日`)
	info.Dates = datePattern.FindAllString(text, -1)
	
	return info
}

// InsuranceInfo represents extracted insurance information.
type InsuranceInfo struct {
	Source         string    `json:"source"`
	CrawledAt      time.Time `json:"crawled_at"`
	LawReferences  []string  `json:"law_references"`
	InsuranceTerms []string  `json:"insurance_terms"`
	Amounts        []string  `json:"amounts"`
	Dates          []string  `json:"dates"`
}

// Reset resets the crawler state.
func (c *Crawler) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.visited = make(map[string]bool)
}

// IsVisited checks if a URL has been visited.
func (c *Crawler) IsVisited(pageURL string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.visited[pageURL]
}

// VisitedCount returns the number of visited pages.
func (c *Crawler) VisitedCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.visited)
}
