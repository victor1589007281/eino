// Package style 实现风格优化子Agent
package style

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/vdocswx/agent"
	"github.com/cloudwego/eino/vdocswx/llm"
)

// StyleAgent 风格优化Agent
type StyleAgent struct {
	llmManager *llm.LLMManager
	guidelines *WeChatStyleGuide
}

// WeChatStyleGuide 微信公众号风格指南
type WeChatStyleGuide struct {
	TitleRules      []StyleRule
	ParagraphRules  []StyleRule
	ToneRules       []StyleRule
	FormattingRules []StyleRule
}

// StyleRule 风格规则
type StyleRule struct {
	Name        string
	Description string
	Check       func(string) (bool, string)
}

// NewStyleAgent 创建风格优化Agent
func NewStyleAgent(llmMgr *llm.LLMManager) *StyleAgent {
	sa := &StyleAgent{
		llmManager: llmMgr,
	}
	sa.initGuidelines()
	return sa
}

// initGuidelines 初始化风格指南
func (sa *StyleAgent) initGuidelines() {
	sa.guidelines = &WeChatStyleGuide{
		TitleRules: []StyleRule{
			{
				Name:        "title_length",
				Description: "标题长度建议在15-30字之间",
				Check: func(title string) (bool, string) {
					length := len([]rune(title))
					if length < 15 {
						return false, "标题过短，建议增加一些吸引眼球的描述"
					}
					if length > 30 {
						return false, "标题过长，建议精简到30字以内"
					}
					return true, ""
				},
			},
			{
				Name:        "title_hook",
				Description: "标题应具有吸引力",
				Check: func(title string) (bool, string) {
					hooks := []string{"如何", "为什么", "揭秘", "必看", "干货", "实战", "最全", "深度"}
					for _, hook := range hooks {
						if strings.Contains(title, hook) {
							return true, ""
						}
					}
					return false, "建议标题增加吸引力元素，如'如何'、'揭秘'、'干货'等"
				},
			},
		},
		ParagraphRules: []StyleRule{
			{
				Name:        "paragraph_length",
				Description: "段落长度适中，方便手机阅读",
				Check: func(paragraph string) (bool, string) {
					length := len([]rune(paragraph))
					if length > 300 {
						return false, "段落过长，建议拆分以提高可读性"
					}
					return true, ""
				},
			},
			{
				Name:        "sentence_variety",
				Description: "句式应多样化",
				Check: func(paragraph string) (bool, string) {
					// 简单检查是否都是相同句式开头
					sentences := strings.Split(paragraph, "。")
					if len(sentences) >= 3 {
						firstChars := make(map[rune]int)
						for _, s := range sentences {
							s = strings.TrimSpace(s)
							if len(s) > 0 {
								firstChars[[]rune(s)[0]]++
							}
						}
						for _, count := range firstChars {
							if count >= 3 {
								return false, "句式开头较为单一，建议增加变化"
							}
						}
					}
					return true, ""
				},
			},
		},
		ToneRules: []StyleRule{
			{
				Name:        "friendly_tone",
				Description: "使用亲切友好的语气",
				Check: func(content string) (bool, string) {
					formalWords := []string{"本文", "笔者", "贵方", "阁下"}
					for _, word := range formalWords {
						if strings.Contains(content, word) {
							return false, fmt.Sprintf("建议将"%s"改为更亲切的表达", word)
						}
					}
					return true, ""
				},
			},
		},
		FormattingRules: []StyleRule{
			{
				Name:        "emoji_usage",
				Description: "适当使用表情符号增加亲和力",
				// 检查逻辑在具体方法中实现
			},
			{
				Name:        "subheading",
				Description: "长文应有小标题分隔",
				// 检查逻辑在具体方法中实现
			},
		},
	}
}

// Name 返回Agent名称
func (sa *StyleAgent) Name() string {
	return "style"
}

// CanHandle 判断是否能处理该类型文章
func (sa *StyleAgent) CanHandle(articleType agent.ArticleType) bool {
	return true
}

// Process 处理文章内容
func (sa *StyleAgent) Process(ctx context.Context, content string, articleType agent.ArticleType) (*agent.SubAgentResult, error) {
	startTime := time.Now()
	
	result := &agent.SubAgentResult{
		AgentName: sa.Name(),
		Success:   true,
		Changes:   make([]agent.Change, 0),
	}
	
	// 1. 应用风格规则检查
	ruleChanges := sa.applyRules(content, articleType)
	result.Changes = append(result.Changes, ruleChanges...)
	
	// 2. 使用LLM进行深度风格优化
	llmChanges, llmOutput, tokens, err := sa.llmStyleOptimize(ctx, content, articleType)
	if err != nil {
		result.Output = content
		result.TokensUsed = 0
	} else {
		result.Changes = append(result.Changes, llmChanges...)
		result.Output = llmOutput
		result.TokensUsed = tokens
	}
	
	result.Duration = time.Since(startTime).Milliseconds()
	
	return result, nil
}

// applyRules 应用风格规则
func (sa *StyleAgent) applyRules(content string, articleType agent.ArticleType) []agent.Change {
	changes := make([]agent.Change, 0)
	
	// 提取标题（假设第一行是标题）
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		title := strings.TrimPrefix(lines[0], "# ")
		for _, rule := range sa.guidelines.TitleRules {
			if ok, suggestion := rule.Check(title); !ok {
				changes = append(changes, agent.Change{
					Type:       agent.ChangeTypeStyle,
					Original:   title,
					Modified:   "",
					Reason:     suggestion,
					Confidence: 0.7,
				})
			}
		}
	}
	
	// 检查段落
	paragraphs := sa.extractParagraphs(content)
	for _, para := range paragraphs {
		for _, rule := range sa.guidelines.ParagraphRules {
			if ok, suggestion := rule.Check(para); !ok {
				changes = append(changes, agent.Change{
					Type:       agent.ChangeTypeStyle,
					Original:   para[:min(50, len(para))] + "...",
					Modified:   "",
					Reason:     suggestion,
					Confidence: 0.7,
				})
			}
		}
	}
	
	// 检查语气
	for _, rule := range sa.guidelines.ToneRules {
		if ok, suggestion := rule.Check(content); !ok {
			changes = append(changes, agent.Change{
				Type:       agent.ChangeTypeStyle,
				Original:   "",
				Modified:   "",
				Reason:     suggestion,
				Confidence: 0.75,
			})
		}
	}
	
	// 检查格式
	changes = append(changes, sa.checkFormatting(content, articleType)...)
	
	return changes
}

// extractParagraphs 提取段落
func (sa *StyleAgent) extractParagraphs(content string) []string {
	paragraphs := make([]string, 0)
	lines := strings.Split(content, "\n")
	
	var currentPara strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if currentPara.Len() > 0 {
				paragraphs = append(paragraphs, currentPara.String())
				currentPara.Reset()
			}
		} else if !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "```") {
			if currentPara.Len() > 0 {
				currentPara.WriteString(" ")
			}
			currentPara.WriteString(line)
		}
	}
	
	if currentPara.Len() > 0 {
		paragraphs = append(paragraphs, currentPara.String())
	}
	
	return paragraphs
}

// checkFormatting 检查格式
func (sa *StyleAgent) checkFormatting(content string, articleType agent.ArticleType) []agent.Change {
	changes := make([]agent.Change, 0)
	
	// 检查是否有小标题
	headingCount := strings.Count(content, "\n## ") + strings.Count(content, "\n### ")
	contentLength := len([]rune(content))
	
	if contentLength > 1000 && headingCount == 0 {
		changes = append(changes, agent.Change{
			Type:       agent.ChangeTypeStyle,
			Original:   "",
			Modified:   "",
			Reason:     "文章较长但缺少小标题，建议添加小标题分隔内容，提高可读性",
			Confidence: 0.8,
		})
	}
	
	// 检查是否过度使用emoji
	emojiCount := sa.countEmojis(content)
	if emojiCount > contentLength/200 {
		changes = append(changes, agent.Change{
			Type:       agent.ChangeTypeStyle,
			Original:   "",
			Modified:   "",
			Reason:     "表情符号使用较多，建议适度使用",
			Confidence: 0.6,
		})
	}
	
	// 技术文章特殊检查
	if articleType == agent.ArticleTypeTech {
		// 检查是否有代码块
		if strings.Contains(content, "代码") && !strings.Contains(content, "```") {
			changes = append(changes, agent.Change{
				Type:       agent.ChangeTypeStyle,
				Original:   "",
				Modified:   "",
				Reason:     "技术文章中提到代码但未使用代码块格式，建议使用```包裹代码",
				Confidence: 0.85,
			})
		}
	}
	
	return changes
}

// countEmojis 统计emoji数量
func (sa *StyleAgent) countEmojis(content string) int {
	count := 0
	for _, r := range content {
		if r >= 0x1F600 && r <= 0x1F64F || // Emoticons
			r >= 0x1F300 && r <= 0x1F5FF || // Misc Symbols and Pictographs
			r >= 0x1F680 && r <= 0x1F6FF || // Transport and Map
			r >= 0x2600 && r <= 0x26FF || // Misc symbols
			r >= 0x2700 && r <= 0x27BF { // Dingbats
			count++
		}
	}
	return count
}

// llmStyleOptimize 使用LLM进行风格优化
func (sa *StyleAgent) llmStyleOptimize(ctx context.Context, content string, articleType agent.ArticleType) ([]agent.Change, string, int, error) {
	prompt := sa.buildStylePrompt(content, articleType)
	
	result, err := sa.llmManager.GenerateText(ctx, prompt, "style")
	if err != nil {
		return nil, content, 0, err
	}
	
	changes, optimizedContent := sa.parseLLMResult(content, result)
	tokens := len(prompt)/4 + len(result)/4
	
	return changes, optimizedContent, tokens, nil
}

// buildStylePrompt 构建风格优化提示
func (sa *StyleAgent) buildStylePrompt(content string, articleType agent.ArticleType) string {
	typeDesc := "通用"
	if articleType == agent.ArticleTypeTech {
		typeDesc = "技术类"
	}
	
	return fmt.Sprintf(`你是一位资深的微信公众号编辑。请优化以下%s文章的写作风格。

微信公众号风格要求：
1. 语言亲切友好，像朋友聊天
2. 段落简短，适合手机阅读
3. 句式多样，避免单调
4. 适当使用过渡句，增强连贯性
5. 标题有吸引力
6. 技术文章要通俗易懂

请按以下格式返回优化建议：
[原文]: 原始文本
[优化]: 优化后文本
[说明]: 优化原因

如果无需优化，请返回"风格良好，无需调整"。

文章内容：
%s`, typeDesc, content)
}

// parseLLMResult 解析LLM返回结果
func (sa *StyleAgent) parseLLMResult(original, result string) ([]agent.Change, string) {
	changes := make([]agent.Change, 0)
	optimized := original
	
	if strings.Contains(result, "无需调整") || strings.Contains(result, "风格良好") {
		return changes, original
	}
	
	lines := strings.Split(result, "\n")
	var currentOriginal, currentOptimized, currentReason string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[原文]:") || strings.HasPrefix(line, "[原文]：") {
			currentOriginal = strings.TrimPrefix(strings.TrimPrefix(line, "[原文]:"), "[原文]：")
			currentOriginal = strings.TrimSpace(currentOriginal)
		} else if strings.HasPrefix(line, "[优化]:") || strings.HasPrefix(line, "[优化]：") {
			currentOptimized = strings.TrimPrefix(strings.TrimPrefix(line, "[优化]:"), "[优化]：")
			currentOptimized = strings.TrimSpace(currentOptimized)
		} else if strings.HasPrefix(line, "[说明]:") || strings.HasPrefix(line, "[说明]：") {
			currentReason = strings.TrimPrefix(strings.TrimPrefix(line, "[说明]:"), "[说明]：")
			currentReason = strings.TrimSpace(currentReason)
			
			if currentOriginal != "" && currentOptimized != "" {
				changes = append(changes, agent.Change{
					Type:       agent.ChangeTypeStyle,
					Original:   currentOriginal,
					Modified:   currentOptimized,
					Reason:     currentReason,
					Confidence: 0.8,
				})
				
				optimized = strings.Replace(optimized, currentOriginal, currentOptimized, 1)
			}
			
			currentOriginal = ""
			currentOptimized = ""
			currentReason = ""
		}
	}
	
	return changes, optimized
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
