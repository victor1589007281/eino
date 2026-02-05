// Package llm LLM Prompt模板
package llm

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// PromptTemplates Prompt模板集合
type PromptTemplates struct {
	intentPrompts map[string]*template.Template
	systemPrompts map[string]string
}

// NewPromptTemplates 创建Prompt模板
func NewPromptTemplates() *PromptTemplates {
	p := &PromptTemplates{
		intentPrompts: make(map[string]*template.Template),
		systemPrompts: make(map[string]string),
	}
	p.init()
	return p
}

// init 初始化模板
func (p *PromptTemplates) init() {
	// 系统提示词
	p.systemPrompts[types.DomainEmail] = `你是一个邮件意图分析专家。你的任务是分析用户的查询，识别其意图和相关实体。

可用的意图类型：
- email_search: 搜索邮件
- email_read: 阅读邮件详情
- email_download_attachment: 下载附件
- email_categorize: 邮件分类
- email_summarize: 邮件摘要
- email_send: 发送邮件
- email_reply: 回复邮件

可提取的槽位：
- time_range: 时间范围（如"最近一周"）
- sender: 发件人
- subject: 主题关键词
- email_type: 邮件类型（发票/报告/会议等）
- attachment_type: 附件类型

请以JSON格式返回分析结果。`

	p.systemPrompts[types.DomainMemory] = `你是一个上下文理解专家。你的任务是分析用户查询是否需要历史上下文，以及需要哪些相关信息。

可用的意图类型：
- memory_recall: 回忆/查找之前的内容
- memory_reference: 引用之前的对话
- memory_topic_switch: 切换/回到之前的话题
- memory_new_topic: 开始新话题
- memory_search: 搜索历史记录

需要分析的维度：
1. 是否引用之前的对话（代词指代、话题延续）
2. 是否是全新的独立问题
3. 如果需要历史，需要哪些相关主题
4. 时间相关性（最近的、某个时间的）

请以JSON格式返回分析结果。`

	p.systemPrompts[types.DomainSearch] = `你是一个搜索意图分析专家。你的任务是分析用户的搜索需求。

可用的意图类型：
- web_search: 通用网页搜索
- web_search_news: 新闻搜索
- web_search_image: 图片搜索
- web_search_local: 本地/地点搜索

可提取的槽位：
- query: 搜索关键词
- time_range: 时间范围
- location: 地点
- search_type: 搜索类型偏好

请以JSON格式返回分析结果。`

	p.systemPrompts[types.DomainGeneral] = `你是一个意图分析专家。请分析用户输入的意图。

通用意图类型：
- greet: 问候
- help: 寻求帮助
- confirm: 确认
- cancel: 取消
- unknown: 未知意图

请以JSON格式返回分析结果。`

	// 意图识别 Prompt 模板
	emailPromptTpl := `请分析以下用户查询，识别其意图。

用户查询：{{.Text}}

{{if .Context}}
历史上下文：
{{range .Context}}- {{.Role}}: {{.Content}}
{{end}}
{{end}}

请以以下JSON格式返回：
{
  "intent": "意图名称",
  "confidence": 0.0-1.0,
  "slots": {"槽位名": "值"},
  "entities": [{"text": "实体文本", "type": "实体类型"}],
  "reasoning": "分析理由"
}`

	memoryPromptTpl := `请分析以下用户查询是否需要历史上下文。

用户查询：{{.Text}}

{{if .Context}}
最近对话：
{{range .Context}}- {{.Role}}: {{.Content}}
{{end}}
{{end}}

请以以下JSON格式返回：
{
  "intent": "意图名称",
  "confidence": 0.0-1.0,
  "slots": {
    "needs_history": "true/false",
    "history_type": "recent/topic_specific/entity_related",
    "related_topics": "相关主题"
  },
  "entities": [{"text": "实体文本", "type": "实体类型"}],
  "reasoning": "分析理由"
}`

	searchPromptTpl := `请分析以下搜索查询的意图。

用户查询：{{.Text}}

请以以下JSON格式返回：
{
  "intent": "意图名称",
  "confidence": 0.0-1.0,
  "slots": {
    "query": "搜索关键词",
    "time_range": "时间范围",
    "search_type": "搜索类型"
  },
  "entities": [{"text": "实体文本", "type": "实体类型"}],
  "reasoning": "分析理由"
}`

	generalPromptTpl := `请分析以下用户输入的意图。

用户输入：{{.Text}}

{{if .Context}}
历史上下文：
{{range .Context}}- {{.Role}}: {{.Content}}
{{end}}
{{end}}

请以以下JSON格式返回：
{
  "intent": "意图名称",
  "confidence": 0.0-1.0,
  "slots": {},
  "entities": [{"text": "实体文本", "type": "实体类型"}],
  "reasoning": "分析理由"
}`

	p.intentPrompts[types.DomainEmail] = template.Must(template.New("email").Parse(emailPromptTpl))
	p.intentPrompts[types.DomainMemory] = template.Must(template.New("memory").Parse(memoryPromptTpl))
	p.intentPrompts[types.DomainSearch] = template.Must(template.New("search").Parse(searchPromptTpl))
	p.intentPrompts[types.DomainGeneral] = template.Must(template.New("general").Parse(generalPromptTpl))
}

// SystemPrompt 获取系统提示词
func (p *PromptTemplates) SystemPrompt(domain string) string {
	if prompt, ok := p.systemPrompts[domain]; ok {
		return prompt
	}
	return p.systemPrompts[types.DomainGeneral]
}

// BuildIntentPrompt 构建意图识别Prompt
func (p *PromptTemplates) BuildIntentPrompt(input *types.IntentInput) (string, error) {
	tpl, ok := p.intentPrompts[input.Domain]
	if !ok {
		tpl = p.intentPrompts[types.DomainGeneral]
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}
