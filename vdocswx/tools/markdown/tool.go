// Package markdown 提供Markdown处理工具
package markdown

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// MarkdownParseTool Markdown解析工具
type MarkdownParseTool struct {
	name        string
	description string
}

// MarkdownFormatTool Markdown格式化工具
type MarkdownFormatTool struct {
	name        string
	description string
}

// MarkdownParseInput 解析输入
type MarkdownParseInput struct {
	Content string `json:"content"`
}

// MarkdownParseOutput 解析输出
type MarkdownParseOutput struct {
	Title      string           `json:"title"`
	Headings   []Heading        `json:"headings"`
	Paragraphs []string         `json:"paragraphs"`
	CodeBlocks []CodeBlock      `json:"code_blocks"`
	Links      []Link           `json:"links"`
	Images     []Image          `json:"images"`
	WordCount  int              `json:"word_count"`
	CharCount  int              `json:"char_count"`
}

// Heading 标题
type Heading struct {
	Level   int    `json:"level"`
	Text    string `json:"text"`
	LineNum int    `json:"line_num"`
}

// CodeBlock 代码块
type CodeBlock struct {
	Language string `json:"language"`
	Code     string `json:"code"`
	LineNum  int    `json:"line_num"`
}

// Link 链接
type Link struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// Image 图片
type Image struct {
	Alt string `json:"alt"`
	URL string `json:"url"`
}

// MarkdownFormatInput 格式化输入
type MarkdownFormatInput struct {
	Content string `json:"content"`
	Options FormatOptions `json:"options,omitempty"`
}

// FormatOptions 格式化选项
type FormatOptions struct {
	AddSpaceBetweenCNAndEN bool `json:"add_space_between_cn_and_en"`
	FixPunctuation         bool `json:"fix_punctuation"`
	UnifyHeadings          bool `json:"unify_headings"`
	RemoveExtraBlankLines  bool `json:"remove_extra_blank_lines"`
}

// MarkdownFormatOutput 格式化输出
type MarkdownFormatOutput struct {
	FormattedContent string   `json:"formatted_content"`
	Changes          []string `json:"changes"`
}

// NewMarkdownParseTool 创建Markdown解析工具
func NewMarkdownParseTool() *MarkdownParseTool {
	return &MarkdownParseTool{
		name:        "markdown_parse",
		description: "解析Markdown文档，提取标题、段落、代码块、链接等结构化信息",
	}
}

// NewMarkdownFormatTool 创建Markdown格式化工具
func NewMarkdownFormatTool() *MarkdownFormatTool {
	return &MarkdownFormatTool{
		name:        "markdown_format",
		description: "格式化Markdown文档，包括修复标点、添加空格、统一格式等",
	}
}

// Info 返回工具信息
func (t *MarkdownParseTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name,
		Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"content": {
				Type:     schema.String,
				Desc:     "Markdown内容",
				Required: true,
			},
		}),
	}, nil
}

// InvokableRun 执行Markdown解析
func (t *MarkdownParseTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input MarkdownParseInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", err
	}
	
	output := t.parse(input.Content)
	
	result, err := json.Marshal(output)
	if err != nil {
		return "", err
	}
	
	return string(result), nil
}

// parse 解析Markdown
func (t *MarkdownParseTool) parse(content string) *MarkdownParseOutput {
	output := &MarkdownParseOutput{
		Headings:   make([]Heading, 0),
		Paragraphs: make([]string, 0),
		CodeBlocks: make([]CodeBlock, 0),
		Links:      make([]Link, 0),
		Images:     make([]Image, 0),
	}
	
	lines := strings.Split(content, "\n")
	
	// 正则表达式
	headingPattern := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	codeBlockStartPattern := regexp.MustCompile("^```(\\w*)$")
	linkPattern := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	imagePattern := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	
	inCodeBlock := false
	var currentCodeBlock CodeBlock
	var currentParagraph strings.Builder
	
	for i, line := range lines {
		// 代码块处理
		if match := codeBlockStartPattern.FindStringSubmatch(line); match != nil {
			if !inCodeBlock {
				inCodeBlock = true
				currentCodeBlock = CodeBlock{
					Language: match[1],
					LineNum:  i + 1,
				}
			} else {
				inCodeBlock = false
				output.CodeBlocks = append(output.CodeBlocks, currentCodeBlock)
				currentCodeBlock = CodeBlock{}
			}
			continue
		}
		
		if inCodeBlock {
			currentCodeBlock.Code += line + "\n"
			continue
		}
		
		// 标题处理
		if match := headingPattern.FindStringSubmatch(line); match != nil {
			level := len(match[1])
			text := match[2]
			
			output.Headings = append(output.Headings, Heading{
				Level:   level,
				Text:    text,
				LineNum: i + 1,
			})
			
			// 第一个一级标题作为文档标题
			if level == 1 && output.Title == "" {
				output.Title = text
			}
			continue
		}
		
		// 链接和图片提取
		for _, match := range imagePattern.FindAllStringSubmatch(line, -1) {
			output.Images = append(output.Images, Image{
				Alt: match[1],
				URL: match[2],
			})
		}
		
		for _, match := range linkPattern.FindAllStringSubmatch(line, -1) {
			// 排除图片链接
			if !strings.HasPrefix(match[0], "!") {
				output.Links = append(output.Links, Link{
					Text: match[1],
					URL:  match[2],
				})
			}
		}
		
		// 段落处理
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			if currentParagraph.Len() > 0 {
				output.Paragraphs = append(output.Paragraphs, currentParagraph.String())
				currentParagraph.Reset()
			}
		} else if !strings.HasPrefix(trimmedLine, "#") && !strings.HasPrefix(trimmedLine, "-") && !strings.HasPrefix(trimmedLine, "*") {
			if currentParagraph.Len() > 0 {
				currentParagraph.WriteString(" ")
			}
			currentParagraph.WriteString(trimmedLine)
		}
	}
	
	// 处理最后一个段落
	if currentParagraph.Len() > 0 {
		output.Paragraphs = append(output.Paragraphs, currentParagraph.String())
	}
	
	// 计算字数
	output.CharCount = len([]rune(content))
	output.WordCount = t.countWords(content)
	
	return output
}

// countWords 统计词数
func (t *MarkdownParseTool) countWords(content string) int {
	// 简单实现：中文按字符计数，英文按空格分词
	count := 0
	
	// 移除markdown标记
	cleanContent := regexp.MustCompile(`[#*_\[\]()!]`).ReplaceAllString(content, "")
	
	for _, r := range cleanContent {
		if r >= 0x4e00 && r <= 0x9fa5 {
			count++ // 中文字符
		}
	}
	
	// 英文单词
	words := strings.Fields(cleanContent)
	for _, word := range words {
		// 检查是否是纯英文
		isEnglish := true
		for _, r := range word {
			if r >= 0x4e00 && r <= 0x9fa5 {
				isEnglish = false
				break
			}
		}
		if isEnglish && len(word) > 0 {
			count++
		}
	}
	
	return count
}

// Info 返回工具信息
func (t *MarkdownFormatTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name,
		Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"content": {
				Type:     schema.String,
				Desc:     "Markdown内容",
				Required: true,
			},
			"options": {
				Type:     schema.Object,
				Desc:     "格式化选项",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun 执行Markdown格式化
func (t *MarkdownFormatTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input MarkdownFormatInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", err
	}
	
	// 默认选项
	if input.Options == (FormatOptions{}) {
		input.Options = FormatOptions{
			AddSpaceBetweenCNAndEN: true,
			FixPunctuation:         true,
			UnifyHeadings:          true,
			RemoveExtraBlankLines:  true,
		}
	}
	
	output := t.format(input.Content, input.Options)
	
	result, err := json.Marshal(output)
	if err != nil {
		return "", err
	}
	
	return string(result), nil
}

// format 格式化Markdown
func (t *MarkdownFormatTool) format(content string, options FormatOptions) *MarkdownFormatOutput {
	output := &MarkdownFormatOutput{
		FormattedContent: content,
		Changes:          make([]string, 0),
	}
	
	if options.AddSpaceBetweenCNAndEN {
		newContent := t.addSpaceBetweenCNAndEN(output.FormattedContent)
		if newContent != output.FormattedContent {
			output.Changes = append(output.Changes, "添加中英文之间的空格")
			output.FormattedContent = newContent
		}
	}
	
	if options.FixPunctuation {
		newContent := t.fixPunctuation(output.FormattedContent)
		if newContent != output.FormattedContent {
			output.Changes = append(output.Changes, "修复标点符号")
			output.FormattedContent = newContent
		}
	}
	
	if options.RemoveExtraBlankLines {
		newContent := t.removeExtraBlankLines(output.FormattedContent)
		if newContent != output.FormattedContent {
			output.Changes = append(output.Changes, "移除多余空行")
			output.FormattedContent = newContent
		}
	}
	
	return output
}

// addSpaceBetweenCNAndEN 在中英文之间添加空格
func (t *MarkdownFormatTool) addSpaceBetweenCNAndEN(content string) string {
	// 中文后接英文
	cnToEn := regexp.MustCompile(`([\x{4e00}-\x{9fa5}])([a-zA-Z0-9])`)
	content = cnToEn.ReplaceAllString(content, "$1 $2")
	
	// 英文后接中文
	enToCn := regexp.MustCompile(`([a-zA-Z0-9])([\x{4e00}-\x{9fa5}])`)
	content = enToCn.ReplaceAllString(content, "$1 $2")
	
	return content
}

// fixPunctuation 修复标点符号
func (t *MarkdownFormatTool) fixPunctuation(content string) string {
	// 移除重复标点
	content = regexp.MustCompile(`([。，！？]){2,}`).ReplaceAllString(content, "$1")
	
	// 修复空格问题（标点前不应有空格）
	content = regexp.MustCompile(`\s+([。，！？：；])`).ReplaceAllString(content, "$1")
	
	return content
}

// removeExtraBlankLines 移除多余空行
func (t *MarkdownFormatTool) removeExtraBlankLines(content string) string {
	return regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")
}
