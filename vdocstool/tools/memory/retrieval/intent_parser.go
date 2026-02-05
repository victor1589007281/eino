// Package retrieval 意图解析器
package retrieval

import (
	"regexp"
	"strings"
)

// IntentParser 意图解析器
type IntentParser struct {
	explicitPatterns  []*regexp.Regexp
	referencePatterns []*regexp.Regexp
}

// NewIntentParser 创建意图解析器
func NewIntentParser() *IntentParser {
	p := &IntentParser{}

	// 显式指令模式
	p.explicitPatterns = []*regexp.Regexp{
		regexp.MustCompile(`回到.*(话题|主题|刚才)`),
		regexp.MustCompile(`继续.*(之前|刚才|上次)`),
		regexp.MustCompile(`接着.*(说|讲|聊)`),
		regexp.MustCompile(`(go back|continue|resume).*(topic|conversation)`),
	}

	// 引用检测模式
	p.referencePatterns = []*regexp.Regexp{
		regexp.MustCompile(`之前(说|提到|讨论)的`),
		regexp.MustCompile(`刚才(说|提到|讨论)的`),
		regexp.MustCompile(`上次(说|提到|讨论)的`),
		regexp.MustCompile(`(前面|上面)(说|提到)的`),
		regexp.MustCompile(`那个.*方案`),
		regexp.MustCompile(`(you|we).*(mentioned|discussed|talked about)`),
		regexp.MustCompile(`(earlier|before|previous)`),
	}

	return p
}

// IntentInfo 意图信息
type IntentInfo struct {
	NeedsHistory        bool     `json:"needs_history"`
	ExplicitTopicSwitch bool     `json:"explicit_topic_switch"`
	HasReference        bool     `json:"has_reference"`
	ReferenceHints      []string `json:"reference_hints,omitempty"`
	TopicKeywords       []string `json:"topic_keywords,omitempty"`
	QueryType           string   `json:"query_type"` // question, statement, command
	SimilarityThreshold float64  `json:"similarity_threshold"`
}

// Parse 解析意图
func (p *IntentParser) Parse(query string) *IntentInfo {
	info := &IntentInfo{
		SimilarityThreshold: 0.8, // 默认阈值
	}

	queryLower := strings.ToLower(query)

	// 检测显式主题切换
	for _, pattern := range p.explicitPatterns {
		if pattern.MatchString(queryLower) {
			info.ExplicitTopicSwitch = true
			info.NeedsHistory = true
			break
		}
	}

	// 检测引用
	for _, pattern := range p.referencePatterns {
		if matches := pattern.FindStringSubmatch(queryLower); len(matches) > 0 {
			info.HasReference = true
			info.NeedsHistory = true
			info.ReferenceHints = append(info.ReferenceHints, matches[0])
		}
	}

	// 提取主题关键词
	info.TopicKeywords = p.extractTopicKeywords(query)

	// 判断查询类型
	info.QueryType = p.classifyQueryType(query)

	// 根据查询类型调整是否需要历史
	if info.QueryType == "question" {
		// 问题通常需要上下文
		info.NeedsHistory = true
	}

	// 检测是否是独立的新话题
	if p.isNewTopicIndicator(queryLower) {
		info.NeedsHistory = false
	}

	return info
}

// extractTopicKeywords 提取主题关键词
func (p *IntentParser) extractTopicKeywords(query string) []string {
	var keywords []string

	// 技术关键词
	techKeywords := []string{
		"redis", "mysql", "mongodb", "postgresql", "neo4j", "milvus",
		"react", "vue", "angular", "node", "go", "golang", "python", "java",
		"docker", "kubernetes", "k8s", "aws", "gcp", "azure",
		"api", "rest", "grpc", "graphql",
		"缓存", "数据库", "队列", "微服务", "分布式",
	}

	queryLower := strings.ToLower(query)
	for _, kw := range techKeywords {
		if strings.Contains(queryLower, kw) {
			keywords = append(keywords, kw)
		}
	}

	return keywords
}

// classifyQueryType 分类查询类型
func (p *IntentParser) classifyQueryType(query string) string {
	// 问题类型
	questionIndicators := []string{
		"?", "？", "怎么", "如何", "为什么", "什么", "哪", "几", "多少",
		"how", "what", "why", "when", "where", "which", "who",
		"can you", "could you", "is there", "are there",
	}

	queryLower := strings.ToLower(query)
	for _, indicator := range questionIndicators {
		if strings.Contains(queryLower, indicator) {
			return "question"
		}
	}

	// 命令类型
	commandIndicators := []string{
		"请", "帮我", "给我", "创建", "生成", "写", "修改", "删除",
		"please", "help me", "create", "generate", "write", "modify", "delete",
	}

	for _, indicator := range commandIndicators {
		if strings.HasPrefix(queryLower, indicator) || strings.Contains(queryLower, indicator) {
			return "command"
		}
	}

	return "statement"
}

// isNewTopicIndicator 检测是否是新话题指示
func (p *IntentParser) isNewTopicIndicator(query string) bool {
	newTopicIndicators := []string{
		"新话题", "换个话题", "另外一个问题", "不相关的",
		"by the way", "on another note", "unrelated",
	}

	for _, indicator := range newTopicIndicators {
		if strings.Contains(query, indicator) {
			return true
		}
	}

	return false
}
