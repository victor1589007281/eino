// Package intent 邮件意图分析
package intent

import (
	"strings"
	"time"
)

// IntentType 意图类型
type IntentType string

const (
	IntentWorkMeeting   IntentType = "work_meeting"   // 工作-会议
	IntentWorkReport    IntentType = "work_report"    // 工作-报告
	IntentWorkApproval  IntentType = "work_approval"  // 工作-审批
	IntentFinanceInvoice IntentType = "finance_invoice" // 财务-发票
	IntentFinanceBill   IntentType = "finance_bill"   // 财务-账单
	IntentFinanceReimburse IntentType = "finance_reimburse" // 财务-报销
	IntentNotifySystem  IntentType = "notify_system"  // 通知-系统
	IntentNotifySubscribe IntentType = "notify_subscribe" // 通知-订阅
	IntentPersonalOrder IntentType = "personal_order" // 个人-订单
	IntentPersonalDelivery IntentType = "personal_delivery" // 个人-快递
	IntentPersonalTravel IntentType = "personal_travel" // 个人-行程
	IntentUnknown       IntentType = "unknown"        // 未知
)

// Analyzer 意图分析器
type Analyzer struct {
	subjectParser    *SubjectParser
	contentParser    *ContentParser
	attachmentParser *AttachmentParser
}

// NewAnalyzer 创建意图分析器
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		subjectParser:    NewSubjectParser(),
		contentParser:    NewContentParser(),
		attachmentParser: NewAttachmentParser(),
	}
}

// AnalysisResult 分析结果
type AnalysisResult struct {
	PrimaryIntent     IntentType            `json:"primary_intent"`
	SecondaryIntents  []IntentType          `json:"secondary_intents,omitempty"`
	Confidence        float64               `json:"confidence"`
	SubjectAnalysis   *SubjectAnalysis      `json:"subject_analysis"`
	ContentAnalysis   *ContentAnalysis      `json:"content_analysis,omitempty"`
	AttachmentAnalysis *AttachmentAnalysis  `json:"attachment_analysis,omitempty"`
	ExtractedEntities map[string][]string   `json:"extracted_entities,omitempty"`
	Keywords          []string              `json:"keywords"`
}

// AnalyzeEmail 分析邮件
func (a *Analyzer) AnalyzeEmail(subject, content string, attachments []string) *AnalysisResult {
	result := &AnalysisResult{
		ExtractedEntities: make(map[string][]string),
	}

	// 分析主题
	result.SubjectAnalysis = a.subjectParser.Parse(subject)
	result.Keywords = append(result.Keywords, result.SubjectAnalysis.Keywords...)

	// 分析内容
	if content != "" {
		result.ContentAnalysis = a.contentParser.Parse(content)
		result.Keywords = append(result.Keywords, result.ContentAnalysis.Keywords...)

		// 合并提取的实体
		for k, v := range result.ContentAnalysis.Entities {
			result.ExtractedEntities[k] = append(result.ExtractedEntities[k], v...)
		}
	}

	// 分析附件
	if len(attachments) > 0 {
		result.AttachmentAnalysis = a.attachmentParser.Parse(attachments)
	}

	// 综合判断意图
	result.PrimaryIntent, result.SecondaryIntents, result.Confidence = a.determineIntent(result)

	return result
}

// determineIntent 综合判断意图
func (a *Analyzer) determineIntent(result *AnalysisResult) (IntentType, []IntentType, float64) {
	scores := make(map[IntentType]float64)

	// 基于主题分析
	if result.SubjectAnalysis != nil {
		for _, intent := range result.SubjectAnalysis.PossibleIntents {
			scores[intent] += 0.4 // 主题权重40%
		}
	}

	// 基于内容分析
	if result.ContentAnalysis != nil {
		for _, intent := range result.ContentAnalysis.PossibleIntents {
			scores[intent] += 0.35 // 内容权重35%
		}
	}

	// 基于附件分析
	if result.AttachmentAnalysis != nil {
		for _, intent := range result.AttachmentAnalysis.PossibleIntents {
			scores[intent] += 0.25 // 附件权重25%
		}
	}

	// 找出最高分意图
	var primary IntentType = IntentUnknown
	var maxScore float64
	var secondary []IntentType

	for intent, score := range scores {
		if score > maxScore {
			if primary != IntentUnknown {
				secondary = append(secondary, primary)
			}
			maxScore = score
			primary = intent
		} else if score > 0.2 {
			secondary = append(secondary, intent)
		}
	}

	// 计算置信度
	confidence := maxScore
	if confidence > 1.0 {
		confidence = 1.0
	}

	return primary, secondary, confidence
}

// SearchQuery 搜索查询
type SearchQuery struct {
	IntentQuery   string            `json:"intent_query"`
	TimeRange     *TimeRange        `json:"time_range,omitempty"`
	ParsedIntents []IntentType      `json:"parsed_intents"`
	SubjectKeywords []string        `json:"subject_keywords"`
	ContentKeywords []string        `json:"content_keywords"`
	AttachmentTypes []string        `json:"attachment_types"`
	FromFilter    string            `json:"from_filter,omitempty"`
}

// TimeRange 时间范围
type TimeRange struct {
	Since  time.Time `json:"since"`
	Before time.Time `json:"before"`
}

// ParseSearchQuery 解析搜索查询
func (a *Analyzer) ParseSearchQuery(query string) *SearchQuery {
	result := &SearchQuery{
		IntentQuery: query,
	}

	queryLower := strings.ToLower(query)

	// 解析时间范围
	result.TimeRange = a.parseTimeRange(queryLower)

	// 解析意图
	result.ParsedIntents = a.parseIntentsFromQuery(queryLower)

	// 提取关键词
	result.SubjectKeywords, result.ContentKeywords = a.extractKeywords(queryLower)

	// 提取附件类型
	result.AttachmentTypes = a.extractAttachmentTypes(queryLower)

	return result
}

// parseTimeRange 解析时间范围
func (a *Analyzer) parseTimeRange(query string) *TimeRange {
	now := time.Now()
	tr := &TimeRange{}

	// 最近N天
	if strings.Contains(query, "最近") || strings.Contains(query, "近") {
		if strings.Contains(query, "周") || strings.Contains(query, "7天") {
			tr.Since = now.AddDate(0, 0, -7)
		} else if strings.Contains(query, "月") || strings.Contains(query, "30天") {
			tr.Since = now.AddDate(0, -1, 0)
		} else if strings.Contains(query, "3天") {
			tr.Since = now.AddDate(0, 0, -3)
		} else {
			// 默认30天
			tr.Since = now.AddDate(0, -1, 0)
		}
		tr.Before = now
		return tr
	}

	// 今天
	if strings.Contains(query, "今天") || strings.Contains(query, "today") {
		tr.Since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		tr.Before = now
		return tr
	}

	// 昨天
	if strings.Contains(query, "昨天") || strings.Contains(query, "yesterday") {
		yesterday := now.AddDate(0, 0, -1)
		tr.Since = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		tr.Before = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return tr
	}

	// 本周
	if strings.Contains(query, "本周") || strings.Contains(query, "this week") {
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		tr.Since = now.AddDate(0, 0, -weekday+1)
		tr.Before = now
		return tr
	}

	// 本月
	if strings.Contains(query, "本月") || strings.Contains(query, "this month") {
		tr.Since = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		tr.Before = now
		return tr
	}

	return nil
}

// parseIntentsFromQuery 从查询中解析意图
func (a *Analyzer) parseIntentsFromQuery(query string) []IntentType {
	var intents []IntentType

	intentKeywords := map[IntentType][]string{
		IntentWorkMeeting:     {"会议", "邀请", "议程", "meeting", "invite"},
		IntentWorkReport:      {"报告", "周报", "月报", "汇报", "report"},
		IntentWorkApproval:    {"审批", "批准", "申请", "approve"},
		IntentFinanceInvoice:  {"发票", "invoice", "电子发票"},
		IntentFinanceBill:     {"账单", "明细", "bill", "statement"},
		IntentFinanceReimburse: {"报销", "费用", "expense", "reimburse"},
		IntentNotifySystem:    {"验证码", "登录", "安全", "verification", "code"},
		IntentNotifySubscribe: {"订阅", "newsletter", "通讯"},
		IntentPersonalOrder:   {"订单", "购买", "order", "purchase"},
		IntentPersonalDelivery: {"快递", "物流", "派送", "delivery", "shipping"},
		IntentPersonalTravel:  {"机票", "酒店", "行程", "flight", "hotel", "itinerary"},
	}

	for intent, keywords := range intentKeywords {
		for _, kw := range keywords {
			if strings.Contains(query, kw) {
				intents = append(intents, intent)
				break
			}
		}
	}

	return intents
}

// extractKeywords 提取关键词
func (a *Analyzer) extractKeywords(query string) ([]string, []string) {
	// 简单分词
	words := strings.Fields(query)
	
	var subjectKw, contentKw []string
	for _, w := range words {
		// 过滤停用词
		if len(w) < 2 || isStopWord(w) {
			continue
		}
		subjectKw = append(subjectKw, w)
		contentKw = append(contentKw, w)
	}

	return subjectKw, contentKw
}

// extractAttachmentTypes 提取附件类型
func (a *Analyzer) extractAttachmentTypes(query string) []string {
	var types []string

	typeKeywords := map[string][]string{
		"pdf":   {"pdf", "文档"},
		"excel": {"excel", "表格", "xlsx", "xls"},
		"word":  {"word", "doc", "docx"},
		"image": {"图片", "照片", "image", "jpg", "png"},
		"ics":   {"日历", "calendar", "ics"},
	}

	for t, keywords := range typeKeywords {
		for _, kw := range keywords {
			if strings.Contains(query, kw) {
				types = append(types, t)
				break
			}
		}
	}

	return types
}

// isStopWord 检查是否为停用词
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"的": true, "了": true, "是": true, "在": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "个": true, "上": true, "也": true,
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"to": true, "of": true, "and": true, "in": true, "that": true,
		"查找": true, "查询": true, "搜索": true, "找": true, "看看": true,
	}
	return stopWords[word]
}
