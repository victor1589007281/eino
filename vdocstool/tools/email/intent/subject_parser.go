// Package intent 主题解析器
package intent

import (
	"regexp"
	"strings"
)

// SubjectParser 主题解析器
type SubjectParser struct {
	patterns map[IntentType][]*regexp.Regexp
	keywords map[IntentType][]string
}

// NewSubjectParser 创建主题解析器
func NewSubjectParser() *SubjectParser {
	p := &SubjectParser{
		patterns: make(map[IntentType][]*regexp.Regexp),
		keywords: map[IntentType][]string{
			IntentWorkMeeting: {
				"会议", "邀请", "议程", "参会", "召开", "meeting", "invite",
				"calendar", "日程", "约定", "讨论会", "研讨",
			},
			IntentWorkReport: {
				"报告", "周报", "月报", "日报", "汇报", "report", "summary",
				"总结", "进度", "工作汇报",
			},
			IntentWorkApproval: {
				"审批", "批准", "申请", "请示", "approval", "request",
				"待审", "需审批",
			},
			IntentFinanceInvoice: {
				"发票", "invoice", "电子发票", "增值税", "专票", "普票",
			},
			IntentFinanceBill: {
				"账单", "明细", "bill", "statement", "对账",
				"消费", "支出",
			},
			IntentFinanceReimburse: {
				"报销", "费用", "expense", "reimburse", "费用申请",
			},
			IntentNotifySystem: {
				"验证码", "登录", "安全", "verification", "code", "verify",
				"密码", "重置", "确认码", "动态码",
			},
			IntentNotifySubscribe: {
				"订阅", "newsletter", "通讯", "周刊", "日报",
				"推送", "更新通知",
			},
			IntentPersonalOrder: {
				"订单", "购买", "order", "purchase", "已下单",
				"订单确认", "交易", "付款",
			},
			IntentPersonalDelivery: {
				"快递", "物流", "派送", "delivery", "shipping", "配送",
				"发货", "签收", "运单", "包裹",
			},
			IntentPersonalTravel: {
				"机票", "酒店", "行程", "flight", "hotel", "itinerary",
				"出行", "预订", "booking", "航班",
			},
		},
	}

	// 编译正则表达式
	p.patterns[IntentNotifySystem] = []*regexp.Regexp{
		regexp.MustCompile(`验证码[：:]\s*\d+`),
		regexp.MustCompile(`\d{4,6}\s*验证码`),
		regexp.MustCompile(`verification\s*code`),
	}

	p.patterns[IntentWorkMeeting] = []*regexp.Regexp{
		regexp.MustCompile(`会议邀请[：:]`),
		regexp.MustCompile(`invite.*meeting`),
		regexp.MustCompile(`\d{1,2}[：:]\d{2}.*会议`),
	}

	return p
}

// SubjectAnalysis 主题分析结果
type SubjectAnalysis struct {
	Original        string       `json:"original"`
	Normalized      string       `json:"normalized"`
	PossibleIntents []IntentType `json:"possible_intents"`
	Keywords        []string     `json:"keywords"`
	Priority        string       `json:"priority,omitempty"` // 高/中/低/普通
	IsReply         bool         `json:"is_reply"`
	IsForward       bool         `json:"is_forward"`
}

// Parse 解析主题
func (p *SubjectParser) Parse(subject string) *SubjectAnalysis {
	result := &SubjectAnalysis{
		Original: subject,
	}

	// 标准化主题
	normalized := strings.TrimSpace(subject)
	normalized = strings.ToLower(normalized)
	result.Normalized = normalized

	// 检测回复/转发
	result.IsReply = p.isReply(subject)
	result.IsForward = p.isForward(subject)

	// 检测优先级
	result.Priority = p.detectPriority(subject)

	// 基于关键词匹配意图
	for intent, keywords := range p.keywords {
		for _, kw := range keywords {
			if strings.Contains(normalized, strings.ToLower(kw)) {
				result.PossibleIntents = append(result.PossibleIntents, intent)
				result.Keywords = append(result.Keywords, kw)
				break
			}
		}
	}

	// 基于正则模式匹配
	for intent, patterns := range p.patterns {
		for _, pattern := range patterns {
			if pattern.MatchString(normalized) {
				// 检查是否已添加
				found := false
				for _, existing := range result.PossibleIntents {
					if existing == intent {
						found = true
						break
					}
				}
				if !found {
					result.PossibleIntents = append(result.PossibleIntents, intent)
				}
				break
			}
		}
	}

	// 去重关键词
	result.Keywords = uniqueStrings(result.Keywords)

	return result
}

// isReply 检测是否为回复
func (p *SubjectParser) isReply(subject string) bool {
	lower := strings.ToLower(subject)
	prefixes := []string{"re:", "re：", "回复:", "回复：", "答复:"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// isForward 检测是否为转发
func (p *SubjectParser) isForward(subject string) bool {
	lower := strings.ToLower(subject)
	prefixes := []string{"fwd:", "fw:", "转发:", "转发："}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// detectPriority 检测优先级
func (p *SubjectParser) detectPriority(subject string) string {
	lower := strings.ToLower(subject)

	highPriority := []string{"紧急", "urgent", "重要", "important", "asap", "立即", "马上"}
	for _, kw := range highPriority {
		if strings.Contains(lower, kw) {
			return "high"
		}
	}

	lowPriority := []string{"fyi", "供参考", "仅供参考"}
	for _, kw := range lowPriority {
		if strings.Contains(lower, kw) {
			return "low"
		}
	}

	return "normal"
}

// uniqueStrings 去重字符串切片
func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
