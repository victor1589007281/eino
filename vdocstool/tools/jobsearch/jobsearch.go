// Package jobsearch 合规的招聘信息搜索工具
// 使用公开 API 和合规方式获取招聘信息
package jobsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/mcp"
)

// JobSearchTool 招聘搜索工具
type JobSearchTool struct {
	config     JobSearchConfig
	httpClient *http.Client
	rateLimiter *RateLimiter
	cache      *SearchCache
}

// JobSearchConfig 招聘搜索配置
type JobSearchConfig struct {
	// Adzuna API 配置（免费公开 API）
	AdzunaAppID  string `json:"adzuna_app_id"`
	AdzunaAppKey string `json:"adzuna_app_key"`
	// 缓存配置
	CacheTTL time.Duration `json:"cache_ttl"`
	// 速率限制（每分钟请求数）
	RateLimit int `json:"rate_limit"`
	// 默认国家/地区
	DefaultCountry string `json:"default_country"`
}

// Job 职位信息
type Job struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	Location    string   `json:"location"`
	Salary      string   `json:"salary"`
	SalaryMin   float64  `json:"salary_min,omitempty"`
	SalaryMax   float64  `json:"salary_max,omitempty"`
	Currency    string   `json:"currency,omitempty"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	PostedDate  string   `json:"posted_date"`
	Source      string   `json:"source"`
	Tags        []string `json:"tags,omitempty"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Query      string `json:"query"`
	Location   string `json:"location"`
	TotalCount int    `json:"total_count"`
	Jobs       []Job  `json:"jobs"`
	Source     string `json:"source"`
	CacheHit   bool   `json:"cache_hit"`
	Timestamp  string `json:"timestamp"`
}

// RateLimiter 速率限制器
type RateLimiter struct {
	mu       sync.Mutex
	tokens   int
	maxTokens int
	lastRefill time.Time
	refillRate time.Duration
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		tokens:     requestsPerMinute,
		maxTokens:  requestsPerMinute,
		lastRefill: time.Now(),
		refillRate: time.Minute / time.Duration(requestsPerMinute),
	}
}

// Allow 检查是否允许请求
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	newTokens := int(elapsed / r.refillRate)
	if newTokens > 0 {
		r.tokens = min(r.maxTokens, r.tokens+newTokens)
		r.lastRefill = now
	}

	if r.tokens > 0 {
		r.tokens--
		return true
	}
	return false
}

// SearchCache 搜索缓存
type SearchCache struct {
	mu     sync.RWMutex
	cache  map[string]*cacheEntry
	ttl    time.Duration
}

type cacheEntry struct {
	result    *SearchResult
	expiresAt time.Time
}

// NewSearchCache 创建搜索缓存
func NewSearchCache(ttl time.Duration) *SearchCache {
	c := &SearchCache{
		cache: make(map[string]*cacheEntry),
		ttl:   ttl,
	}
	go c.cleanup()
	return c
}

func (c *SearchCache) Get(key string) (*SearchResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		result := *entry.result
		result.CacheHit = true
		return &result, true
	}
	return nil, false
}

func (c *SearchCache) Set(key string, result *SearchResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *SearchCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.cache {
			if now.After(v.expiresAt) {
				delete(c.cache, k)
			}
		}
		c.mu.Unlock()
	}
}

// NewJobSearchTool 创建招聘搜索工具
func NewJobSearchTool(cfg JobSearchConfig) *JobSearchTool {
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 30 * time.Minute
	}
	if cfg.RateLimit == 0 {
		cfg.RateLimit = 30 // 默认每分钟30次请求
	}
	if cfg.DefaultCountry == "" {
		cfg.DefaultCountry = "cn"
	}

	return &JobSearchTool{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: NewRateLimiter(cfg.RateLimit),
		cache:       NewSearchCache(cfg.CacheTTL),
	}
}

// Register 注册到 MCP Server
func (t *JobSearchTool) Register(server *mcp.Server) {
	// 注册职位搜索工具
	searchTool := mcp.NewToolBuilder("search_jobs", "搜索招聘职位信息（合规方式）").
		AddProperty("query", "string", "搜索关键词，如 DBA、数据库架构师", true).
		AddProperty("location", "string", "工作地点，如 北京、上海、深圳", false).
		AddProperty("salary_min", "integer", "最低年薪（万元）", false).
		AddProperty("salary_max", "integer", "最高年薪（万元）", false).
		AddProperty("max_results", "integer", "最大结果数，默认20", false).
		AddEnumProperty("sort_by", "排序方式", []string{"relevance", "date", "salary"}, false).
		Build()

	server.RegisterTool(searchTool, t.handleSearchJobs)

	// 注册热门职位工具
	hotJobsTool := mcp.NewToolBuilder("get_hot_jobs", "获取热门高薪职位").
		AddProperty("category", "string", "职位类别，如 IT、金融、互联网", false).
		AddProperty("location", "string", "工作地点", false).
		AddProperty("min_salary", "integer", "最低年薪（万元）", false).
		Build()

	server.RegisterTool(hotJobsTool, t.handleGetHotJobs)

	// 注册职位推荐链接工具
	linksTool := mcp.NewToolBuilder("get_job_portal_links", "获取主流招聘网站的搜索链接").
		AddProperty("query", "string", "搜索关键词", true).
		AddProperty("location", "string", "工作地点", false).
		Build()

	server.RegisterTool(linksTool, t.handleGetPortalLinks)
}

// handleSearchJobs 处理职位搜索
func (t *JobSearchTool) handleSearchJobs(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	location, _ := args["location"].(string)
	if location == "" {
		location = "北京"
	}

	salaryMin := 0
	if v, ok := args["salary_min"].(float64); ok {
		salaryMin = int(v)
	}

	salaryMax := 0
	if v, ok := args["salary_max"].(float64); ok {
		salaryMax = int(v)
	}

	maxResults := 20
	if v, ok := args["max_results"].(float64); ok && v > 0 {
		maxResults = int(v)
		if maxResults > 50 {
			maxResults = 50
		}
	}

	sortBy, _ := args["sort_by"].(string)
	if sortBy == "" {
		sortBy = "relevance"
	}

	// 检查缓存
	cacheKey := fmt.Sprintf("%s:%s:%d:%d:%d:%s", query, location, salaryMin, salaryMax, maxResults, sortBy)
	if cached, ok := t.cache.Get(cacheKey); ok {
		return cached, nil
	}

	// 速率限制检查
	if !t.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded, please try again later")
	}

	// 执行搜索（使用多个合规来源）
	result := t.aggregateSearch(ctx, query, location, salaryMin, salaryMax, maxResults, sortBy)

	// 缓存结果
	t.cache.Set(cacheKey, result)

	return result, nil
}

// aggregateSearch 聚合多个来源的搜索结果
func (t *JobSearchTool) aggregateSearch(ctx context.Context, query, location string, salaryMin, salaryMax, maxResults int, sortBy string) *SearchResult {
	result := &SearchResult{
		Query:     query,
		Location:  location,
		Source:    "aggregated",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	var allJobs []Job
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 使用 Adzuna API（如果配置了）
	if t.config.AdzunaAppID != "" && t.config.AdzunaAppKey != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			jobs, err := t.searchAdzuna(ctx, query, location, salaryMin, maxResults)
			if err == nil {
				mu.Lock()
				allJobs = append(allJobs, jobs...)
				mu.Unlock()
			}
		}()
	}

	// 生成主流招聘平台的搜索建议
	wg.Add(1)
	go func() {
		defer wg.Done()
		jobs := t.generatePortalSuggestions(query, location, salaryMin, salaryMax)
		mu.Lock()
		allJobs = append(allJobs, jobs...)
		mu.Unlock()
	}()

	wg.Wait()

	// 去重和排序
	uniqueJobs := t.deduplicateJobs(allJobs)

	// 根据薪资过滤
	if salaryMin > 0 || salaryMax > 0 {
		uniqueJobs = t.filterBySalary(uniqueJobs, salaryMin, salaryMax)
	}

	// 排序
	uniqueJobs = t.sortJobs(uniqueJobs, sortBy)

	// 限制结果数
	if len(uniqueJobs) > maxResults {
		uniqueJobs = uniqueJobs[:maxResults]
	}

	result.Jobs = uniqueJobs
	result.TotalCount = len(uniqueJobs)

	return result
}

// searchAdzuna 使用 Adzuna API 搜索（免费公开API）
func (t *JobSearchTool) searchAdzuna(ctx context.Context, query, location string, salaryMin, maxResults int) ([]Job, error) {
	// Adzuna API 支持中国区
	baseURL := fmt.Sprintf("https://api.adzuna.com/v1/api/jobs/%s/search/1", t.config.DefaultCountry)

	params := url.Values{}
	params.Set("app_id", t.config.AdzunaAppID)
	params.Set("app_key", t.config.AdzunaAppKey)
	params.Set("what", query)
	if location != "" {
		params.Set("where", location)
	}
	params.Set("results_per_page", fmt.Sprintf("%d", maxResults))
	if salaryMin > 0 {
		params.Set("salary_min", fmt.Sprintf("%d", salaryMin*10000)) // 转换为年薪
	}

	fullURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "AgentTools/1.0 (Job Search; Contact: support@example.com)")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Adzuna API error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var adzunaResp struct {
		Results []struct {
			ID          string  `json:"id"`
			Title       string  `json:"title"`
			Company     struct {
				DisplayName string `json:"display_name"`
			} `json:"company"`
			Location struct {
				DisplayName string `json:"display_name"`
			} `json:"location"`
			SalaryMin   float64 `json:"salary_min"`
			SalaryMax   float64 `json:"salary_max"`
			Description string  `json:"description"`
			RedirectURL string  `json:"redirect_url"`
			Created     string  `json:"created"`
			Category    struct {
				Label string `json:"label"`
			} `json:"category"`
		} `json:"results"`
		Count int `json:"count"`
	}

	if err := json.Unmarshal(body, &adzunaResp); err != nil {
		return nil, err
	}

	var jobs []Job
	for _, r := range adzunaResp.Results {
		salary := ""
		if r.SalaryMin > 0 && r.SalaryMax > 0 {
			salary = fmt.Sprintf("%.0f-%.0f/年", r.SalaryMin, r.SalaryMax)
		} else if r.SalaryMin > 0 {
			salary = fmt.Sprintf("%.0f+/年", r.SalaryMin)
		}

		jobs = append(jobs, Job{
			ID:          r.ID,
			Title:       r.Title,
			Company:     r.Company.DisplayName,
			Location:    r.Location.DisplayName,
			Salary:      salary,
			SalaryMin:   r.SalaryMin,
			SalaryMax:   r.SalaryMax,
			Description: truncateString(r.Description, 200),
			URL:         r.RedirectURL,
			PostedDate:  r.Created,
			Source:      "Adzuna",
			Tags:        []string{r.Category.Label},
		})
	}

	return jobs, nil
}

// generatePortalSuggestions 生成主流招聘平台的搜索建议
func (t *JobSearchTool) generatePortalSuggestions(query, location string, salaryMin, salaryMax int) []Job {
	var jobs []Job

	// 编码查询参数
	encodedQuery := url.QueryEscape(query)
	encodedLocation := url.QueryEscape(location)

	// 根据薪资范围生成描述
	salaryDesc := ""
	if salaryMin > 0 && salaryMax > 0 {
		salaryDesc = fmt.Sprintf("%d-%d万/年", salaryMin, salaryMax)
	} else if salaryMin > 0 {
		salaryDesc = fmt.Sprintf("%d万+/年", salaryMin)
	}

	// 猎聘网
	jobs = append(jobs, Job{
		ID:          "liepin-search",
		Title:       fmt.Sprintf("在猎聘网搜索: %s (%s)", query, location),
		Company:     "猎聘网 - 中高端人才招聘平台",
		Location:    location,
		Salary:      salaryDesc,
		Description: "猎聘专注中高端人才招聘，适合寻找年薪50万以上的职位",
		URL:         fmt.Sprintf("https://www.liepin.com/zhaopin/?key=%s&dqs=010", encodedQuery),
		Source:      "猎聘网",
		Tags:        []string{"高端招聘", "猎头服务"},
	})

	// BOSS直聘
	jobs = append(jobs, Job{
		ID:          "boss-search",
		Title:       fmt.Sprintf("在BOSS直聘搜索: %s (%s)", query, location),
		Company:     "BOSS直聘 - 直接和老板谈",
		Location:    location,
		Salary:      salaryDesc,
		Description: "BOSS直聘支持直接和招聘方沟通，适合快速获取职位反馈",
		URL:         fmt.Sprintf("https://www.zhipin.com/web/geek/job?city=101010100&query=%s", encodedQuery),
		Source:      "BOSS直聘",
		Tags:        []string{"直聊", "快速响应"},
	})

	// 拉勾网
	jobs = append(jobs, Job{
		ID:          "lagou-search",
		Title:       fmt.Sprintf("在拉勾网搜索: %s (%s)", query, location),
		Company:     "拉勾网 - 互联网招聘平台",
		Location:    location,
		Salary:      salaryDesc,
		Description: "拉勾专注互联网行业招聘，适合IT、技术类职位",
		URL:         fmt.Sprintf("https://www.lagou.com/wn/zhaopin?kd=%s&city=%s", encodedQuery, encodedLocation),
		Source:      "拉勾网",
		Tags:        []string{"互联网", "技术岗位"},
	})

	// 脉脉
	jobs = append(jobs, Job{
		ID:          "maimai-search",
		Title:       fmt.Sprintf("在脉脉搜索: %s (%s)", query, location),
		Company:     "脉脉 - 职场社交招聘",
		Location:    location,
		Salary:      salaryDesc,
		Description: "脉脉支持内推和职场社交，适合获取内部推荐机会",
		URL:         fmt.Sprintf("https://maimai.cn/web/search_center?type=job&query=%s", encodedQuery),
		Source:      "脉脉",
		Tags:        []string{"内推", "职场社交"},
	})

	// 智联招聘
	jobs = append(jobs, Job{
		ID:          "zhilian-search",
		Title:       fmt.Sprintf("在智联招聘搜索: %s (%s)", query, location),
		Company:     "智联招聘 - 综合招聘平台",
		Location:    location,
		Salary:      salaryDesc,
		Description: "智联招聘覆盖各行业职位，职位数量多",
		URL:         fmt.Sprintf("https://sou.zhaopin.com/?kw=%s&city=530", encodedQuery),
		Source:      "智联招聘",
		Tags:        []string{"综合招聘", "大量职位"},
	})

	// LinkedIn
	jobs = append(jobs, Job{
		ID:          "linkedin-search",
		Title:       fmt.Sprintf("在LinkedIn搜索: %s (China)", query),
		Company:     "LinkedIn - 全球职业社交平台",
		Location:    location,
		Salary:      salaryDesc,
		Description: "LinkedIn适合外企和国际化公司的职位",
		URL:         fmt.Sprintf("https://www.linkedin.com/jobs/search/?keywords=%s&location=China", encodedQuery),
		Source:      "LinkedIn",
		Tags:        []string{"外企", "国际化"},
	})

	return jobs
}

// deduplicateJobs 职位去重
func (t *JobSearchTool) deduplicateJobs(jobs []Job) []Job {
	seen := make(map[string]bool)
	var unique []Job

	for _, job := range jobs {
		key := job.Title + job.Company
		if !seen[key] {
			seen[key] = true
			unique = append(unique, job)
		}
	}

	return unique
}

// filterBySalary 按薪资过滤
func (t *JobSearchTool) filterBySalary(jobs []Job, salaryMin, salaryMax int) []Job {
	var filtered []Job
	for _, job := range jobs {
		// 来自招聘平台建议的总是保留
		if strings.HasSuffix(job.ID, "-search") {
			filtered = append(filtered, job)
			continue
		}

		// 有薪资信息的进行过滤
		if job.SalaryMin > 0 {
			if salaryMin > 0 && job.SalaryMax < float64(salaryMin*10000) {
				continue
			}
			if salaryMax > 0 && job.SalaryMin > float64(salaryMax*10000) {
				continue
			}
		}
		filtered = append(filtered, job)
	}
	return filtered
}

// sortJobs 职位排序
func (t *JobSearchTool) sortJobs(jobs []Job, sortBy string) []Job {
	// 简单排序实现
	switch sortBy {
	case "salary":
		// 按薪资降序
		for i := 0; i < len(jobs)-1; i++ {
			for j := i + 1; j < len(jobs); j++ {
				if jobs[i].SalaryMax < jobs[j].SalaryMax {
					jobs[i], jobs[j] = jobs[j], jobs[i]
				}
			}
		}
	case "date":
		// 按日期降序（假设日期格式一致）
		for i := 0; i < len(jobs)-1; i++ {
			for j := i + 1; j < len(jobs); j++ {
				if jobs[i].PostedDate < jobs[j].PostedDate {
					jobs[i], jobs[j] = jobs[j], jobs[i]
				}
			}
		}
	}
	return jobs
}

// handleGetHotJobs 获取热门高薪职位
func (t *JobSearchTool) handleGetHotJobs(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	category, _ := args["category"].(string)
	if category == "" {
		category = "IT"
	}

	location, _ := args["location"].(string)
	if location == "" {
		location = "北京"
	}

	minSalary := 50
	if v, ok := args["min_salary"].(float64); ok && v > 0 {
		minSalary = int(v)
	}

	// 热门高薪职位关键词
	hotKeywords := map[string][]string{
		"IT": {
			"数据库架构师",
			"DBA总监",
			"技术VP",
			"CTO",
			"首席架构师",
			"数据治理专家",
		},
		"金融": {
			"风控总监",
			"量化研究员",
			"投资总监",
			"金融科技架构师",
		},
		"互联网": {
			"算法专家",
			"AI研究员",
			"技术总监",
			"产品VP",
		},
	}

	keywords, ok := hotKeywords[category]
	if !ok {
		keywords = hotKeywords["IT"]
	}

	var allJobs []Job
	for _, keyword := range keywords[:3] { // 只取前3个关键词
		result := t.aggregateSearch(ctx, keyword, location, minSalary, 0, 5, "salary")
		allJobs = append(allJobs, result.Jobs...)
	}

	return &SearchResult{
		Query:      fmt.Sprintf("热门%s高薪职位", category),
		Location:   location,
		TotalCount: len(allJobs),
		Jobs:       allJobs,
		Source:     "hot_jobs",
		Timestamp:  time.Now().Format(time.RFC3339),
	}, nil
}

// handleGetPortalLinks 获取招聘平台搜索链接
func (t *JobSearchTool) handleGetPortalLinks(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		query = "DBA 数据库"
	}

	location, _ := args["location"].(string)
	if location == "" {
		location = "北京"
	}

	encodedQuery := url.QueryEscape(query)
	encodedLocation := url.QueryEscape(location)

	links := []map[string]string{
		{
			"name":        "猎聘网",
			"description": "中高端人才招聘平台，适合寻找50万+年薪职位",
			"url":         fmt.Sprintf("https://www.liepin.com/zhaopin/?key=%s&dqs=010", encodedQuery),
		},
		{
			"name":        "BOSS直聘",
			"description": "直接和招聘方沟通，快速获取反馈",
			"url":         fmt.Sprintf("https://www.zhipin.com/web/geek/job?city=101010100&query=%s", encodedQuery),
		},
		{
			"name":        "拉勾网",
			"description": "专注互联网行业，适合技术岗位",
			"url":         fmt.Sprintf("https://www.lagou.com/wn/zhaopin?kd=%s&city=%s", encodedQuery, encodedLocation),
		},
		{
			"name":        "脉脉",
			"description": "职场社交平台，可获取内推机会",
			"url":         fmt.Sprintf("https://maimai.cn/web/search_center?type=job&query=%s", encodedQuery),
		},
		{
			"name":        "智联招聘",
			"description": "综合招聘平台，职位数量多",
			"url":         fmt.Sprintf("https://sou.zhaopin.com/?kw=%s&city=530", encodedQuery),
		},
		{
			"name":        "前程无忧",
			"description": "老牌招聘网站，覆盖各行业",
			"url":         fmt.Sprintf("https://search.51job.com/list/010000,000000,0000,00,9,99,%s,2,1.html", encodedQuery),
		},
		{
			"name":        "LinkedIn",
			"description": "适合外企和国际化公司职位",
			"url":         fmt.Sprintf("https://www.linkedin.com/jobs/search/?keywords=%s&location=China", encodedQuery),
		},
	}

	return map[string]interface{}{
		"query":     query,
		"location":  location,
		"links":     links,
		"timestamp": time.Now().Format(time.RFC3339),
		"tip":       "建议同时在多个平台投递简历，并开启职位推送通知",
	}, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
