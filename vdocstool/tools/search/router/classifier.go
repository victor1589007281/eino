// Package router 查询分类器
package router

import (
	"regexp"
	"strings"
	"unicode"
)

// QueryType 查询类型
type QueryType string

const (
	QueryTypeTechnical QueryType = "technical" // 技术/代码/API文档
	QueryTypeNews      QueryType = "news"      // 新闻/时事
	QueryTypeAcademic  QueryType = "academic"  // 学术/研究
	QueryTypeLocal     QueryType = "local"     // 本地化/中文
	QueryTypeGeneral   QueryType = "general"   // 通用查询
	QueryTypeShopping  QueryType = "shopping"  // 购物/产品
	QueryTypeJob       QueryType = "job"       // 招聘/职位
)

// QueryClassification 查询分类结果
type QueryClassification struct {
	PrimaryType   QueryType         `json:"primary_type"`
	SecondaryType QueryType         `json:"secondary_type,omitempty"`
	Language      string            `json:"language"`
	Confidence    float64           `json:"confidence"`
	Features      map[string]bool   `json:"features"`
}

// QueryClassifier 查询分类器
type QueryClassifier struct {
	// 关键词规则
	technicalKeywords []string
	newsKeywords      []string
	academicKeywords  []string
	shoppingKeywords  []string
	jobKeywords       []string
	
	// 中文比例阈值
	chineseRatioThreshold float64
	
	// 正则模式
	codePatterns []*regexp.Regexp
}

// NewQueryClassifier 创建查询分类器
func NewQueryClassifier() *QueryClassifier {
	c := &QueryClassifier{
		chineseRatioThreshold: 0.3,
	}
	
	// 技术关键词
	c.technicalKeywords = []string{
		// 编程语言
		"golang", "go语言", "python", "java", "javascript", "typescript", "rust", "c++",
		// 技术术语
		"api", "sdk", "framework", "library", "error", "debug", "bug", "exception",
		"函数", "方法", "接口", "类", "对象", "变量", "常量",
		// 框架/工具
		"docker", "kubernetes", "k8s", "redis", "mysql", "postgresql", "mongodb",
		"react", "vue", "angular", "spring", "django", "flask",
		"git", "github", "gitlab",
		// 操作
		"install", "setup", "configure", "deploy", "build", "compile",
		"安装", "配置", "部署", "编译", "调试",
	}
	
	// 新闻关键词
	c.newsKeywords = []string{
		"最新", "今日", "新闻", "breaking", "news", "update", "latest",
		"发布", "宣布", "公告", "报道", "事件", "动态",
		"今天", "昨天", "本周", "本月",
	}
	
	// 学术关键词
	c.academicKeywords = []string{
		"论文", "研究", "paper", "research", "study", "analysis",
		"实验", "experiment", "methodology", "hypothesis",
		"参考文献", "citation", "journal", "conference",
		"arxiv", "ieee", "acm", "springer",
	}
	
	// 购物关键词
	c.shoppingKeywords = []string{
		"价格", "多少钱", "购买", "买", "淘宝", "京东", "天猫",
		"price", "buy", "purchase", "amazon", "shop",
		"优惠", "折扣", "促销", "便宜",
	}
	
	// 招聘关键词
	c.jobKeywords = []string{
		"招聘", "职位", "工作", "求职", "简历", "面试",
		"job", "career", "hiring", "salary", "薪资", "年薪",
		"猎聘", "boss直聘", "智联", "前程无忧", "linkedin",
	}
	
	// 代码模式
	c.codePatterns = []*regexp.Regexp{
		regexp.MustCompile(`func\s+\w+`),           // Go 函数
		regexp.MustCompile(`def\s+\w+`),            // Python 函数
		regexp.MustCompile(`class\s+\w+`),          // 类定义
		regexp.MustCompile(`import\s+`),            // 导入语句
		regexp.MustCompile(`package\s+`),           // 包声明
		regexp.MustCompile(`\w+\(\)`),              // 函数调用
		regexp.MustCompile(`\w+\.\w+\(`),           // 方法调用
		regexp.MustCompile(`error:\s*`),            // 错误信息
		regexp.MustCompile(`Exception`),            // 异常
		regexp.MustCompile(`\$\{?\w+\}?`),          // 变量引用
	}
	
	return c
}

// Classify 分类查询
func (c *QueryClassifier) Classify(query string) *QueryClassification {
	result := &QueryClassification{
		PrimaryType: QueryTypeGeneral,
		Confidence:  0.5,
		Features:    make(map[string]bool),
	}
	
	queryLower := strings.ToLower(query)
	
	// 检测语言
	result.Language = c.detectLanguage(query)
	
	// 检测各类型特征
	scores := make(map[QueryType]float64)
	
	// 技术类检测
	techScore := c.detectTechnical(queryLower)
	scores[QueryTypeTechnical] = techScore
	if techScore > 0.3 {
		result.Features["has_tech_keywords"] = true
	}
	
	// 新闻类检测
	newsScore := c.detectNews(queryLower)
	scores[QueryTypeNews] = newsScore
	if newsScore > 0.3 {
		result.Features["has_news_keywords"] = true
	}
	
	// 学术类检测
	academicScore := c.detectAcademic(queryLower)
	scores[QueryTypeAcademic] = academicScore
	if academicScore > 0.3 {
		result.Features["has_academic_keywords"] = true
	}
	
	// 购物类检测
	shoppingScore := c.detectShopping(queryLower)
	scores[QueryTypeShopping] = shoppingScore
	if shoppingScore > 0.3 {
		result.Features["has_shopping_keywords"] = true
	}
	
	// 招聘类检测
	jobScore := c.detectJob(queryLower)
	scores[QueryTypeJob] = jobScore
	if jobScore > 0.3 {
		result.Features["has_job_keywords"] = true
	}
	
	// 代码模式检测
	if c.hasCodePattern(query) {
		scores[QueryTypeTechnical] += 0.3
		result.Features["has_code_pattern"] = true
	}
	
	// 本地化检测
	if result.Language == "zh" {
		scores[QueryTypeLocal] = 0.5
		result.Features["is_chinese"] = true
	}
	
	// 选择最高分类型
	maxScore := 0.0
	for qType, score := range scores {
		if score > maxScore {
			maxScore = score
			result.PrimaryType = qType
			result.Confidence = score
		}
	}
	
	// 选择次高分类型
	secondMaxScore := 0.0
	for qType, score := range scores {
		if qType != result.PrimaryType && score > secondMaxScore {
			secondMaxScore = score
			result.SecondaryType = qType
		}
	}
	
	// 如果没有明显特征，保持通用类型
	if maxScore < 0.3 {
		result.PrimaryType = QueryTypeGeneral
		result.Confidence = 0.5
	}
	
	return result
}

// detectLanguage 检测语言
func (c *QueryClassifier) detectLanguage(query string) string {
	chineseCount := 0
	totalCount := 0
	
	for _, r := range query {
		if unicode.Is(unicode.Han, r) {
			chineseCount++
		}
		if !unicode.IsSpace(r) {
			totalCount++
		}
	}
	
	if totalCount == 0 {
		return "unknown"
	}
	
	ratio := float64(chineseCount) / float64(totalCount)
	if ratio >= c.chineseRatioThreshold {
		return "zh"
	}
	return "en"
}

// detectTechnical 检测技术类查询
func (c *QueryClassifier) detectTechnical(query string) float64 {
	matchCount := 0
	for _, keyword := range c.technicalKeywords {
		if strings.Contains(query, strings.ToLower(keyword)) {
			matchCount++
		}
	}
	
	if matchCount == 0 {
		return 0
	}
	
	// 归一化得分
	score := float64(matchCount) / 3.0
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// detectNews 检测新闻类查询
func (c *QueryClassifier) detectNews(query string) float64 {
	matchCount := 0
	for _, keyword := range c.newsKeywords {
		if strings.Contains(query, strings.ToLower(keyword)) {
			matchCount++
		}
	}
	
	if matchCount == 0 {
		return 0
	}
	
	score := float64(matchCount) / 2.0
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// detectAcademic 检测学术类查询
func (c *QueryClassifier) detectAcademic(query string) float64 {
	matchCount := 0
	for _, keyword := range c.academicKeywords {
		if strings.Contains(query, strings.ToLower(keyword)) {
			matchCount++
		}
	}
	
	if matchCount == 0 {
		return 0
	}
	
	score := float64(matchCount) / 2.0
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// detectShopping 检测购物类查询
func (c *QueryClassifier) detectShopping(query string) float64 {
	matchCount := 0
	for _, keyword := range c.shoppingKeywords {
		if strings.Contains(query, strings.ToLower(keyword)) {
			matchCount++
		}
	}
	
	if matchCount == 0 {
		return 0
	}
	
	score := float64(matchCount) / 2.0
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// detectJob 检测招聘类查询
func (c *QueryClassifier) detectJob(query string) float64 {
	matchCount := 0
	for _, keyword := range c.jobKeywords {
		if strings.Contains(query, strings.ToLower(keyword)) {
			matchCount++
		}
	}
	
	if matchCount == 0 {
		return 0
	}
	
	score := float64(matchCount) / 2.0
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// hasCodePattern 检测代码模式
func (c *QueryClassifier) hasCodePattern(query string) bool {
	for _, pattern := range c.codePatterns {
		if pattern.MatchString(query) {
			return true
		}
	}
	return false
}

// GetQueryTypeMatchScore 获取查询类型与引擎的匹配得分
func GetQueryTypeMatchScore(engineName string, queryType QueryType) float64 {
	// 引擎-查询类型匹配矩阵
	matchMatrix := map[string]map[QueryType]float64{
		"serper": {
			QueryTypeTechnical: 0.95,
			QueryTypeNews:      0.95,
			QueryTypeAcademic:  0.85,
			QueryTypeLocal:     0.70,
			QueryTypeGeneral:   0.90,
			QueryTypeShopping:  0.85,
			QueryTypeJob:       0.80,
		},
		"tavily": {
			QueryTypeTechnical: 0.90,
			QueryTypeNews:      0.85,
			QueryTypeAcademic:  0.80,
			QueryTypeLocal:     0.60,
			QueryTypeGeneral:   0.85,
			QueryTypeShopping:  0.75,
			QueryTypeJob:       0.70,
		},
		"exa": {
			QueryTypeTechnical: 0.98,
			QueryTypeNews:      0.60,
			QueryTypeAcademic:  0.95,
			QueryTypeLocal:     0.50,
			QueryTypeGeneral:   0.75,
			QueryTypeShopping:  0.50,
			QueryTypeJob:       0.60,
		},
		"brave": {
			QueryTypeTechnical: 0.80,
			QueryTypeNews:      0.85,
			QueryTypeAcademic:  0.75,
			QueryTypeLocal:     0.50,
			QueryTypeGeneral:   0.80,
			QueryTypeShopping:  0.80,
			QueryTypeJob:       0.70,
		},
		"duckduckgo": {
			QueryTypeTechnical: 0.70,
			QueryTypeNews:      0.75,
			QueryTypeAcademic:  0.70,
			QueryTypeLocal:     0.40,
			QueryTypeGeneral:   0.75,
			QueryTypeShopping:  0.70,
			QueryTypeJob:       0.60,
		},
		"searxng": {
			QueryTypeTechnical: 0.65,
			QueryTypeNews:      0.70,
			QueryTypeAcademic:  0.75,
			QueryTypeLocal:     0.50,
			QueryTypeGeneral:   0.70,
			QueryTypeShopping:  0.65,
			QueryTypeJob:       0.55,
		},
		"bing": {
			QueryTypeTechnical: 0.75,
			QueryTypeNews:      0.80,
			QueryTypeAcademic:  0.75,
			QueryTypeLocal:     0.60,
			QueryTypeGeneral:   0.80,
			QueryTypeShopping:  0.80,
			QueryTypeJob:       0.70,
		},
		"baidu": {
			QueryTypeTechnical: 0.50,
			QueryTypeNews:      0.80,
			QueryTypeAcademic:  0.60,
			QueryTypeLocal:     0.90,
			QueryTypeGeneral:   0.70,
			QueryTypeShopping:  0.85,
			QueryTypeJob:       0.85,
		},
	}
	
	if engineScores, ok := matchMatrix[engineName]; ok {
		if score, ok := engineScores[queryType]; ok {
			return score
		}
	}
	
	return 0.5 // 默认得分
}
