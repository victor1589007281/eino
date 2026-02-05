// Package intent 内容解析器
package intent

import (
	"regexp"
	"strings"
)

// ContentParser 内容解析器
type ContentParser struct {
	entityPatterns map[string]*regexp.Regexp
	intentPatterns map[IntentType][]*regexp.Regexp
}

// NewContentParser 创建内容解析器
func NewContentParser() *ContentParser {
	p := &ContentParser{
		entityPatterns: map[string]*regexp.Regexp{
			"phone":         regexp.MustCompile(`1[3-9]\d{9}`),
			"email":         regexp.MustCompile(`[\w.-]+@[\w.-]+\.\w+`),
			"money":         regexp.MustCompile(`[¥￥$]\s*[\d,]+\.?\d*|[\d,]+\.?\d*\s*[元美]`),
			"date":          regexp.MustCompile(`\d{4}[-/年]\d{1,2}[-/月]\d{1,2}日?`),
			"time":          regexp.MustCompile(`\d{1,2}[：:]\d{2}(:\d{2})?`),
			"tracking_no":   regexp.MustCompile(`[A-Z]{2}\d{9}[A-Z]{2}|\d{12,15}`),
			"order_no":      regexp.MustCompile(`订单[号编]?[：:]\s*[\w-]+|[A-Z]*\d{10,}`),
			"invoice_code":  regexp.MustCompile(`发票[代号码][：:]\s*\d+`),
			"verify_code":   regexp.MustCompile(`验证码[是：:]+\s*\d{4,6}|\d{4,6}\s*[是为]?验证码|[验动]态?码[：:]\s*\d{4,6}`),
			"meeting_link":  regexp.MustCompile(`https?://[^\s]+(?:zoom|meeting|teams|webex|tencent)[^\s]*`),
			"address":       regexp.MustCompile(`\p{Han}{2,}[省市区县镇乡村路街道号楼室]+[\p{Han}\d]+`),
		},
		intentPatterns: make(map[IntentType][]*regexp.Regexp),
	}

	// 意图相关的内容模式
	p.intentPatterns[IntentNotifySystem] = []*regexp.Regexp{
		regexp.MustCompile(`[验动]态?码[：:]\s*\d{4,6}`),
		regexp.MustCompile(`请勿泄露`),
		regexp.MustCompile(`有效期\s*\d+\s*分钟`),
	}

	p.intentPatterns[IntentWorkMeeting] = []*regexp.Regexp{
		regexp.MustCompile(`会议时间[：:]`),
		regexp.MustCompile(`会议地点[：:]`),
		regexp.MustCompile(`参会人[：:]`),
		regexp.MustCompile(`zoom|teams|webex|腾讯会议`),
	}

	p.intentPatterns[IntentPersonalDelivery] = []*regexp.Regexp{
		regexp.MustCompile(`快递单号[：:]`),
		regexp.MustCompile(`运单号[：:]`),
		regexp.MustCompile(`已发货`),
		regexp.MustCompile(`正在派送`),
		regexp.MustCompile(`已签收`),
	}

	p.intentPatterns[IntentFinanceInvoice] = []*regexp.Regexp{
		regexp.MustCompile(`发票[代号码][：:]`),
		regexp.MustCompile(`开票金额[：:]`),
		regexp.MustCompile(`纳税人识别号`),
	}

	p.intentPatterns[IntentPersonalOrder] = []*regexp.Regexp{
		regexp.MustCompile(`订单[号编][：:]`),
		regexp.MustCompile(`订单已确认`),
		regexp.MustCompile(`付款成功`),
		regexp.MustCompile(`商品[：:]`),
	}

	return p
}

// ContentAnalysis 内容分析结果
type ContentAnalysis struct {
	PossibleIntents []IntentType          `json:"possible_intents"`
	Keywords        []string              `json:"keywords"`
	Entities        map[string][]string   `json:"entities"`
	Summary         string                `json:"summary,omitempty"`
	SentimentHint   string                `json:"sentiment_hint,omitempty"` // positive/negative/neutral
}

// Parse 解析内容
func (p *ContentParser) Parse(content string) *ContentAnalysis {
	result := &ContentAnalysis{
		Entities: make(map[string][]string),
	}

	// 提取实体
	for entityType, pattern := range p.entityPatterns {
		matches := pattern.FindAllString(content, -1)
		if len(matches) > 0 {
			result.Entities[entityType] = uniqueStrings(matches)
		}
	}

	// 基于模式匹配意图
	for intent, patterns := range p.intentPatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(content) {
				result.PossibleIntents = append(result.PossibleIntents, intent)
				break
			}
		}
	}

	// 提取关键词（简单实现）
	result.Keywords = p.extractKeywords(content)

	// 情感倾向检测
	result.SentimentHint = p.detectSentiment(content)

	// 生成摘要
	result.Summary = p.generateSummary(content)

	return result
}

// extractKeywords 提取关键词
func (p *ContentParser) extractKeywords(content string) []string {
	// 预定义的重要关键词
	importantKeywords := []string{
		"紧急", "重要", "请", "需要", "确认", "审批", "付款", "发票",
		"会议", "报告", "订单", "快递", "验证码", "密码",
	}

	var found []string
	lower := strings.ToLower(content)

	for _, kw := range importantKeywords {
		if strings.Contains(lower, kw) {
			found = append(found, kw)
		}
	}

	return uniqueStrings(found)
}

// detectSentiment 检测情感倾向
func (p *ContentParser) detectSentiment(content string) string {
	positiveWords := []string{
		"感谢", "谢谢", "恭喜", "祝贺", "成功", "完成", "好的", "收到",
		"thanks", "congratulations", "success", "completed",
	}

	negativeWords := []string{
		"抱歉", "遗憾", "失败", "问题", "错误", "取消", "拒绝",
		"sorry", "failed", "error", "cancel", "reject",
	}

	lower := strings.ToLower(content)
	positiveCount := 0
	negativeCount := 0

	for _, w := range positiveWords {
		if strings.Contains(lower, w) {
			positiveCount++
		}
	}

	for _, w := range negativeWords {
		if strings.Contains(lower, w) {
			negativeCount++
		}
	}

	if positiveCount > negativeCount {
		return "positive"
	} else if negativeCount > positiveCount {
		return "negative"
	}
	return "neutral"
}

// generateSummary 生成摘要
func (p *ContentParser) generateSummary(content string) string {
	// 简单实现：取前200字符
	content = strings.TrimSpace(content)
	
	// 移除多余空白
	content = strings.Join(strings.Fields(content), " ")
	
	if len(content) <= 200 {
		return content
	}
	
	// 尝试在句子边界截断
	truncated := content[:200]
	lastPeriod := strings.LastIndexAny(truncated, ".。!！?？")
	if lastPeriod > 100 {
		return truncated[:lastPeriod+1]
	}
	
	return truncated + "..."
}
