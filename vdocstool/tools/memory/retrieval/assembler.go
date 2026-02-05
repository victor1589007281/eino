// Package retrieval 上下文组装器
package retrieval

import (
	"fmt"
	"strings"
)

// ContextAssembler 上下文组装器
type ContextAssembler struct {
	defaultBudget int
}

// NewContextAssembler 创建上下文组装器
func NewContextAssembler(budget int) *ContextAssembler {
	if budget <= 0 {
		budget = 4000
	}
	return &ContextAssembler{
		defaultBudget: budget,
	}
}

// Assemble 组装上下文
func (a *ContextAssembler) Assemble(items []*ContextItem, tokenBudget int) []*ContextItem {
	if tokenBudget <= 0 {
		tokenBudget = a.defaultBudget
	}

	// 预算分配策略
	// - 60% 给高相关度内容
	// - 30% 给中等相关度内容
	// - 10% 预留缓冲

	highRelevanceBudget := int(float64(tokenBudget) * 0.6)
	mediumRelevanceBudget := int(float64(tokenBudget) * 0.3)
	_ = tokenBudget - highRelevanceBudget - mediumRelevanceBudget // buffer for future use

	var result []*ContextItem
	usedTokens := 0

	// 分类
	var highItems, mediumItems, lowItems []*ContextItem
	for _, item := range items {
		if item.Relevance >= 0.7 {
			highItems = append(highItems, item)
		} else if item.Relevance >= 0.4 {
			mediumItems = append(mediumItems, item)
		} else {
			lowItems = append(lowItems, item)
		}
	}

	// 添加高相关度内容
	budget := highRelevanceBudget
	for _, item := range highItems {
		if usedTokens+item.TokenCount > budget {
			// 尝试截断
			truncated := a.truncateItem(item, budget-usedTokens)
			if truncated != nil {
				result = append(result, truncated)
				usedTokens += truncated.TokenCount
			}
			break
		}
		result = append(result, item)
		usedTokens += item.TokenCount
	}

	// 添加中等相关度内容
	budget = highRelevanceBudget + mediumRelevanceBudget
	for _, item := range mediumItems {
		if usedTokens+item.TokenCount > budget {
			truncated := a.truncateItem(item, budget-usedTokens)
			if truncated != nil {
				result = append(result, truncated)
				usedTokens += truncated.TokenCount
			}
			break
		}
		result = append(result, item)
		usedTokens += item.TokenCount
	}

	// 使用缓冲预算添加低相关度内容（如果还有空间）
	budget = tokenBudget
	for _, item := range lowItems {
		if usedTokens+item.TokenCount > budget {
			break
		}
		result = append(result, item)
		usedTokens += item.TokenCount
	}

	return result
}

// truncateItem 截断项目以适应预算
func (a *ContextAssembler) truncateItem(item *ContextItem, maxTokens int) *ContextItem {
	if maxTokens <= 0 {
		return nil
	}

	// 估算每个token对应的字符数
	charsPerToken := 2.5 // 中英文混合估算
	maxChars := int(float64(maxTokens) * charsPerToken)

	if maxChars <= 20 {
		return nil
	}

	content := item.Content
	if len(content) <= maxChars {
		return item
	}

	// 截断并保持句子完整
	truncated := content[:maxChars]

	// 找到最后一个句子结束
	lastEnd := strings.LastIndexAny(truncated, ".。!！?？")
	if lastEnd > maxChars/2 {
		truncated = truncated[:lastEnd+1]
	} else {
		truncated += "..."
	}

	return &ContextItem{
		Content:    truncated,
		Role:       item.Role,
		Source:     item.Source,
		Relevance:  item.Relevance * 0.9, // 截断内容略微降权
		TokenCount: estimateTokenCount(truncated),
	}
}

// AssembleWithTemplate 使用模板组装
func (a *ContextAssembler) AssembleWithTemplate(items []*ContextItem, tokenBudget int, template string) string {
	assembled := a.Assemble(items, tokenBudget)

	if template == "" {
		template = DefaultTemplate
	}

	// 构建上下文字符串
	var contextParts []string
	for _, item := range assembled {
		part := fmt.Sprintf("[%s] %s", item.Role, item.Content)
		contextParts = append(contextParts, part)
	}

	contextStr := strings.Join(contextParts, "\n\n")

	// 替换模板
	result := strings.Replace(template, "{{context}}", contextStr, 1)

	return result
}

// DefaultTemplate 默认模板
const DefaultTemplate = `以下是相关的历史上下文信息：

{{context}}

请基于以上上下文回答用户的问题。`

// StructuredAssembler 结构化组装器
type StructuredAssembler struct {
	defaultBudget int
}

// NewStructuredAssembler 创建结构化组装器
func NewStructuredAssembler(budget int) *StructuredAssembler {
	return &StructuredAssembler{
		defaultBudget: budget,
	}
}

// AssembleStructured 结构化组装
func (s *StructuredAssembler) AssembleStructured(items []*ContextItem, tokenBudget int) *StructuredContext {
	if tokenBudget <= 0 {
		tokenBudget = s.defaultBudget
	}

	result := &StructuredContext{
		Sections: make(map[string][]*ContextItem),
	}

	usedTokens := 0

	// 按来源分组
	for _, item := range items {
		if usedTokens+item.TokenCount > tokenBudget {
			break
		}

		section := getSectionFromSource(item.Source)
		result.Sections[section] = append(result.Sections[section], item)
		usedTokens += item.TokenCount
	}

	result.TotalTokens = usedTokens

	return result
}

// StructuredContext 结构化上下文
type StructuredContext struct {
	Sections    map[string][]*ContextItem `json:"sections"`
	TotalTokens int                       `json:"total_tokens"`
}

// ToPrompt 转换为提示词
func (s *StructuredContext) ToPrompt() string {
	var parts []string

	// 按固定顺序输出
	sectionOrder := []string{"recent", "related", "history"}
	sectionTitles := map[string]string{
		"recent":  "最近对话",
		"related": "相关上下文",
		"history": "历史记录",
	}

	for _, section := range sectionOrder {
		items := s.Sections[section]
		if len(items) == 0 {
			continue
		}

		title := sectionTitles[section]
		parts = append(parts, fmt.Sprintf("## %s", title))

		for _, item := range items {
			parts = append(parts, fmt.Sprintf("- [%s] %s", item.Role, item.Content))
		}
	}

	return strings.Join(parts, "\n\n")
}

func getSectionFromSource(source string) string {
	if strings.HasPrefix(source, "L1") {
		return "recent"
	}
	if strings.HasPrefix(source, "L2") {
		return "related"
	}
	return "history"
}
