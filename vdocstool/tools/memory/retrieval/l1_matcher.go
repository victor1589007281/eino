// Package retrieval L1 工作记忆匹配器
package retrieval

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/vdocstool/tools/memory/storage"
)

// L1Matcher L1 匹配器
type L1Matcher struct {
	l1      *storage.L1WorkingMemory
	topN    int
	minSim  float64
}

// NewL1Matcher 创建 L1 匹配器
func NewL1Matcher(l1 *storage.L1WorkingMemory) *L1Matcher {
	return &L1Matcher{
		l1:     l1,
		topN:   5,
		minSim: 0.5,
	}
}

// Match 匹配 L1 工作记忆
func (m *L1Matcher) Match(ctx context.Context, sessionID, query, topicID string) ([]*ContextItem, bool, error) {
	// 获取最近消息
	messages, err := m.l1.GetRecentMessages(ctx, sessionID, m.topN*2)
	if err != nil {
		return nil, false, err
	}

	if len(messages) == 0 {
		return nil, false, nil
	}

	// 如果指定了 topicID，过滤匹配的消息
	if topicID != "" {
		var filtered []*storage.Message
		for _, msg := range messages {
			if msg.TopicID == topicID || msg.TopicID == "" {
				filtered = append(filtered, msg)
			}
		}
		messages = filtered
	}

	// 计算相似度
	var results []*ContextItem
	maxSim := 0.0

	for _, msg := range messages {
		sim := m.calculateSimilarity(query, msg.Content)
		if sim > maxSim {
			maxSim = sim
		}

		results = append(results, &ContextItem{
			Content:    msg.Content,
			Role:       msg.Role,
			Source:     "L1",
			Relevance:  sim,
			TokenCount: msg.TokenCount,
		})
	}

	// 按相似度排序
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Relevance > results[i].Relevance {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// 限制返回数量
	if len(results) > m.topN {
		results = results[:m.topN]
	}

	// 判断是否命中（最高相似度是否超过阈值）
	hit := maxSim >= m.minSim

	return results, hit, nil
}

// calculateSimilarity 计算文本相似度
func (m *L1Matcher) calculateSimilarity(query, content string) float64 {
	// 简单的词汇重叠相似度
	queryWords := tokenize(strings.ToLower(query))
	contentWords := tokenize(strings.ToLower(content))

	if len(queryWords) == 0 || len(contentWords) == 0 {
		return 0
	}

	// 计算 Jaccard 相似度
	querySet := make(map[string]bool)
	for _, w := range queryWords {
		querySet[w] = true
	}

	contentSet := make(map[string]bool)
	for _, w := range contentWords {
		contentSet[w] = true
	}

	intersection := 0
	for w := range querySet {
		if contentSet[w] {
			intersection++
		}
	}

	union := len(querySet) + len(contentSet) - intersection
	if union == 0 {
		return 0
	}

	jaccard := float64(intersection) / float64(union)

	// 额外考虑关键词匹配
	keywordBonus := m.calculateKeywordBonus(query, content)

	return jaccard*0.7 + keywordBonus*0.3
}

// calculateKeywordBonus 计算关键词加分
func (m *L1Matcher) calculateKeywordBonus(query, content string) float64 {
	// 技术关键词
	keywords := []string{
		"redis", "mysql", "mongodb", "api", "function", "class",
		"error", "bug", "fix", "implement", "create",
	}

	queryLower := strings.ToLower(query)
	contentLower := strings.ToLower(content)

	matchCount := 0
	for _, kw := range keywords {
		if strings.Contains(queryLower, kw) && strings.Contains(contentLower, kw) {
			matchCount++
		}
	}

	if matchCount == 0 {
		return 0
	}

	return float64(matchCount) / float64(len(keywords))
}

// tokenize 分词
func tokenize(text string) []string {
	// 简单分词：按空格和标点分割
	var tokens []string
	current := ""

	for _, r := range text {
		if isAlphaNumeric(r) || r > 127 { // 保留中文
			current += string(r)
		} else {
			if current != "" && len(current) > 1 {
				tokens = append(tokens, current)
			}
			current = ""
		}
	}

	if current != "" && len(current) > 1 {
		tokens = append(tokens, current)
	}

	return tokens
}

func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
