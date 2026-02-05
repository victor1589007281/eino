// Package rule 规则引擎实现
package rule

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// Engine 规则引擎
type Engine struct {
	config   *types.RuleEngineConfig
	patterns map[string][]*Pattern
}

// Pattern 意图模式
type Pattern struct {
	Intent     string
	Regexps    []*regexp.Regexp
	Keywords   []string
	Slots      map[string]*SlotPattern
	Confidence float64
	Category   string
}

// SlotPattern 槽位模式
type SlotPattern struct {
	Name    string
	Regexp  *regexp.Regexp
	Default string
}

// NewEngine 创建规则引擎
func NewEngine(cfg *types.RuleEngineConfig) (*Engine, error) {
	e := &Engine{
		config:   cfg,
		patterns: make(map[string][]*Pattern),
	}

	// 加载内置模式
	e.loadBuiltinPatterns()

	return e, nil
}

// loadBuiltinPatterns 加载内置模式
func (e *Engine) loadBuiltinPatterns() {
	// 邮件领域模式
	e.patterns[types.DomainEmail] = e.emailPatterns()
	// 记忆领域模式
	e.patterns[types.DomainMemory] = e.memoryPatterns()
	// 搜索领域模式
	e.patterns[types.DomainSearch] = e.searchPatterns()
	// 通用模式
	e.patterns[types.DomainGeneral] = e.generalPatterns()
}

// emailPatterns 邮件领域模式
func (e *Engine) emailPatterns() []*Pattern {
	return []*Pattern{
		{
			Intent: types.IntentEmailSearch,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(搜索|查找|找一下|帮我找).*(邮件|email|mail)`),
				regexp.MustCompile(`(?i)(邮件|email|mail).*(搜索|查找|在哪)`),
				regexp.MustCompile(`(?i)(有没有|是否有).*(邮件|email)`),
			},
			Keywords:   []string{"搜索邮件", "查找邮件", "找邮件", "search email"},
			Confidence: 0.9,
			Category:   "email",
			Slots: map[string]*SlotPattern{
				"time_range": {
					Name:   "time_range",
					Regexp: regexp.MustCompile(`(今天|昨天|本周|上周|本月|上月|最近\d+天|最近一周|最近一个月)`),
				},
				"sender": {
					Name:   "sender",
					Regexp: regexp.MustCompile(`(?:来自|from|发件人[是为]?)\s*([^\s,，]+)`),
				},
				"keyword": {
					Name:   "keyword",
					Regexp: regexp.MustCompile(`(?:关于|包含|含有|主题[是为]?)\s*([^\s,，]+)`),
				},
			},
		},
		{
			Intent: types.IntentEmailRead,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(打开|查看|阅读|看看|读).*(邮件|email|mail)`),
				regexp.MustCompile(`(?i)(最新|最近|上一封|下一封).*(邮件|email)`),
			},
			Keywords:   []string{"打开邮件", "阅读邮件", "查看邮件", "read email"},
			Confidence: 0.85,
			Category:   "email",
		},
		{
			Intent: types.IntentEmailDownload,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(下载|保存|获取).*(附件|attachment)`),
				regexp.MustCompile(`(?i)(附件|attachment).*(下载|保存)`),
			},
			Keywords:   []string{"下载附件", "保存附件", "download attachment"},
			Confidence: 0.9,
			Category:   "email",
			Slots: map[string]*SlotPattern{
				"attachment_type": {
					Name:   "attachment_type",
					Regexp: regexp.MustCompile(`(pdf|word|excel|doc|xls|ppt|图片|文档|表格)`),
				},
			},
		},
		{
			Intent: types.IntentEmailSummarize,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(总结|摘要|概括).*(邮件|email)`),
				regexp.MustCompile(`(?i)(邮件|email).*(总结|摘要|概括)`),
			},
			Keywords:   []string{"邮件总结", "邮件摘要", "summarize email"},
			Confidence: 0.85,
			Category:   "email",
		},
	}
}

// memoryPatterns 记忆领域模式
func (e *Engine) memoryPatterns() []*Pattern {
	return []*Pattern{
		{
			Intent: types.IntentMemoryReference,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(之前|刚才|上次|前面|上面)(说|提到|讨论|聊)的`),
				regexp.MustCompile(`(?i)(你|我们)(说过|提到过|讨论过)`),
				regexp.MustCompile(`(?i)(earlier|before|previous).*(mention|discuss|talk)`),
			},
			Keywords:   []string{"之前说的", "刚才提到的", "上次讨论的"},
			Confidence: 0.9,
			Category:   "memory",
		},
		{
			Intent: types.IntentMemoryTopicSwitch,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(回到|继续|接着).*(话题|主题|之前|刚才)`),
				regexp.MustCompile(`(?i)(go back|continue|resume).*(topic|conversation)`),
			},
			Keywords:   []string{"回到话题", "继续之前", "接着说"},
			Confidence: 0.9,
			Category:   "memory",
		},
		{
			Intent: types.IntentMemoryNewTopic,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(新话题|换个话题|另外一个问题|不相关的)`),
				regexp.MustCompile(`(?i)(by the way|on another note|unrelated)`),
			},
			Keywords:   []string{"新话题", "换个话题", "另外一个问题"},
			Confidence: 0.9,
			Category:   "memory",
		},
		{
			Intent: types.IntentMemorySearch,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(搜索|查找|找一下).*(历史|记录|之前的对话)`),
				regexp.MustCompile(`(?i)(历史|记录).*(搜索|查找)`),
			},
			Keywords:   []string{"搜索历史", "查找记录", "历史对话"},
			Confidence: 0.85,
			Category:   "memory",
		},
	}
}

// searchPatterns 搜索领域模式
func (e *Engine) searchPatterns() []*Pattern {
	return []*Pattern{
		{
			Intent: types.IntentWebSearch,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(搜索|搜一下|查一下|帮我查|百度|谷歌|google|bing)`),
				regexp.MustCompile(`(?i)(什么是|如何|怎么|为什么|哪里).+[?？]?$`),
			},
			Keywords:   []string{"搜索", "查一下", "帮我查", "search"},
			Confidence: 0.8,
			Category:   "search",
		},
		{
			Intent: types.IntentWebSearchNews,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(最新|新闻|资讯|热点).*(消息|报道|动态)`),
				regexp.MustCompile(`(?i)(news|latest).*(about|on)`),
			},
			Keywords:   []string{"最新消息", "新闻", "热点"},
			Confidence: 0.85,
			Category:   "search",
		},
	}
}

// generalPatterns 通用模式
func (e *Engine) generalPatterns() []*Pattern {
	return []*Pattern{
		{
			Intent: types.IntentGreet,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)^(你好|您好|hi|hello|hey|嗨|早上好|下午好|晚上好)[\s!！。.]*$`),
			},
			Keywords:   []string{"你好", "您好", "hello", "hi"},
			Confidence: 0.95,
			Category:   "general",
		},
		{
			Intent: types.IntentHelp,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(帮助|help|怎么用|如何使用|功能|能做什么)`),
			},
			Keywords:   []string{"帮助", "help", "怎么用"},
			Confidence: 0.9,
			Category:   "general",
		},
		{
			Intent: types.IntentConfirm,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)^(是|是的|对|好的|可以|OK|yes|确认|没问题)[\s!！。.]*$`),
			},
			Keywords:   []string{"是", "好的", "可以", "yes"},
			Confidence: 0.9,
			Category:   "general",
		},
		{
			Intent: types.IntentCancel,
			Regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)^(取消|不要|不用|算了|no|cancel)[\s!！。.]*$`),
			},
			Keywords:   []string{"取消", "不要", "不用", "cancel"},
			Confidence: 0.9,
			Category:   "general",
		},
	}
}

// Recognize 识别意图
func (e *Engine) Recognize(ctx context.Context, input *types.IntentInput) (*types.IntentResult, error) {
	start := time.Now()

	// 获取适用的模式列表
	patterns := e.getPatternsForDomain(input.Domain)

	var bestMatch *Pattern
	var bestScore float64
	var matchedSlots map[string]string

	text := strings.TrimSpace(input.Text)

	for _, pattern := range patterns {
		score, slots := e.matchPattern(text, pattern)
		if score > bestScore {
			bestScore = score
			bestMatch = pattern
			matchedSlots = slots
		}
	}

	if bestMatch == nil || bestScore < 0.5 {
		return &types.IntentResult{
			Intent: &types.Intent{
				Name:       types.IntentUnknown,
				Confidence: 0,
			},
			Confidence: 0,
			Source:     "rule",
			Latency:    time.Since(start),
		}, nil
	}

	return &types.IntentResult{
		Intent: &types.Intent{
			Name:       bestMatch.Intent,
			Confidence: bestScore,
			Slots:      matchedSlots,
			Category:   bestMatch.Category,
		},
		Confidence: bestScore,
		Source:     "rule",
		Latency:    time.Since(start),
	}, nil
}

// matchPattern 匹配模式
func (e *Engine) matchPattern(text string, pattern *Pattern) (float64, map[string]string) {
	var score float64

	// 正则匹配
	for _, re := range pattern.Regexps {
		if re.MatchString(text) {
			score = pattern.Confidence
			break
		}
	}

	// 关键词匹配（如果正则未匹配）
	if score == 0 {
		textLower := strings.ToLower(text)
		for _, kw := range pattern.Keywords {
			if strings.Contains(textLower, strings.ToLower(kw)) {
				score = pattern.Confidence * 0.8 // 关键词匹配置信度稍低
				break
			}
		}
	}

	// 提取槽位
	slots := make(map[string]string)
	if score > 0 && pattern.Slots != nil {
		for name, slotPattern := range pattern.Slots {
			if matches := slotPattern.Regexp.FindStringSubmatch(text); len(matches) > 1 {
				slots[name] = matches[1]
			} else if slotPattern.Default != "" {
				slots[name] = slotPattern.Default
			}
		}
	}

	return score, slots
}

// getPatternsForDomain 获取领域模式
func (e *Engine) getPatternsForDomain(domain string) []*Pattern {
	var patterns []*Pattern

	// 添加领域特定模式
	if domainPatterns, ok := e.patterns[domain]; ok {
		patterns = append(patterns, domainPatterns...)
	}

	// 添加通用模式
	if generalPatterns, ok := e.patterns[types.DomainGeneral]; ok {
		patterns = append(patterns, generalPatterns...)
	}

	return patterns
}

// Name 引擎名称
func (e *Engine) Name() string {
	return "rule"
}

// Domains 支持的领域
func (e *Engine) Domains() []string {
	domains := make([]string, 0, len(e.patterns))
	for domain := range e.patterns {
		domains = append(domains, domain)
	}
	return domains
}

// HealthCheck 健康检查
func (e *Engine) HealthCheck(ctx context.Context) error {
	return nil // 规则引擎始终可用
}

// Close 关闭引擎
func (e *Engine) Close() error {
	return nil
}
