// Package grammar 实现语法检查子Agent
package grammar

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/cloudwego/eino/vdocswx/agent"
	"github.com/cloudwego/eino/vdocswx/llm"
)

// GrammarAgent 语法检查Agent
type GrammarAgent struct {
	llmManager *llm.LLMManager
	rules      []GrammarRule
}

// GrammarRule 语法规则
type GrammarRule struct {
	Name        string
	Pattern     *regexp.Regexp
	Replacement string
	Description string
}

// NewGrammarAgent 创建语法检查Agent
func NewGrammarAgent(llmMgr *llm.LLMManager) *GrammarAgent {
	ga := &GrammarAgent{
		llmManager: llmMgr,
	}
	ga.initRules()
	return ga
}

// initRules 初始化语法规则
func (ga *GrammarAgent) initRules() {
	ga.rules = []GrammarRule{
		// 标点符号规则
		{
			Name:        "chinese_period",
			Pattern:     regexp.MustCompile(`([^a-zA-Z0-9])\.$`),
			Replacement: "$1。",
			Description: "使用中文句号",
		},
		{
			Name:        "chinese_comma",
			Pattern:     regexp.MustCompile(`([^a-zA-Z0-9]),([^a-zA-Z0-9])`),
			Replacement: "$1，$2",
			Description: "使用中文逗号",
		},
		{
			Name:        "double_punctuation",
			Pattern:     regexp.MustCompile(`([。，！？]){2,}`),
			Replacement: "$1",
			Description: "去除重复标点",
		},
		// 空格规则
		{
			Name:        "chinese_english_space",
			Pattern:     regexp.MustCompile(`([\x{4e00}-\x{9fa5}])([a-zA-Z0-9])`),
			Replacement: "$1 $2",
			Description: "中英文之间添加空格",
		},
		{
			Name:        "english_chinese_space",
			Pattern:     regexp.MustCompile(`([a-zA-Z0-9])([\x{4e00}-\x{9fa5}])`),
			Replacement: "$1 $2",
			Description: "英中文之间添加空格",
		},
		// 常见错别字
		{
			Name:        "typo_de",
			Pattern:     regexp.MustCompile(`的的`),
			Replacement: "的",
			Description: "去除重复的"的"",
		},
	}
}

// Name 返回Agent名称
func (ga *GrammarAgent) Name() string {
	return "grammar"
}

// CanHandle 判断是否能处理该类型文章
func (ga *GrammarAgent) CanHandle(articleType agent.ArticleType) bool {
	// 语法检查适用于所有类型文章
	return true
}

// Process 处理文章内容
func (ga *GrammarAgent) Process(ctx context.Context, content string, articleType agent.ArticleType) (*agent.SubAgentResult, error) {
	startTime := time.Now()
	
	result := &agent.SubAgentResult{
		AgentName: ga.Name(),
		Success:   true,
		Changes:   make([]agent.Change, 0),
	}
	
	// 1. 应用规则检查
	ruleChanges, ruleOutput := ga.applyRules(content)
	result.Changes = append(result.Changes, ruleChanges...)
	
	// 2. 使用LLM进行深度语法检查
	llmChanges, llmOutput, tokens, err := ga.llmGrammarCheck(ctx, ruleOutput, articleType)
	if err != nil {
		// LLM检查失败时仍返回规则检查结果
		result.Output = ruleOutput
		result.TokensUsed = 0
	} else {
		result.Changes = append(result.Changes, llmChanges...)
		result.Output = llmOutput
		result.TokensUsed = tokens
	}
	
	result.Duration = time.Since(startTime).Milliseconds()
	
	return result, nil
}

// applyRules 应用语法规则
func (ga *GrammarAgent) applyRules(content string) ([]agent.Change, string) {
	changes := make([]agent.Change, 0)
	result := content
	
	for _, rule := range ga.rules {
		matches := rule.Pattern.FindAllStringSubmatchIndex(result, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				original := result[match[0]:match[1]]
				modified := rule.Pattern.ReplaceAllString(original, rule.Replacement)
				
				if original != modified {
					changes = append(changes, agent.Change{
						Type:       agent.ChangeTypeGrammar,
						Original:   original,
						Modified:   modified,
						Reason:     rule.Description,
						Confidence: 0.95,
					})
				}
			}
		}
		result = rule.Pattern.ReplaceAllString(result, rule.Replacement)
	}
	
	// 额外的标点检查
	result, punctChanges := ga.checkPunctuation(result)
	changes = append(changes, punctChanges...)
	
	return changes, result
}

// checkPunctuation 检查标点符号
func (ga *GrammarAgent) checkPunctuation(content string) (string, []agent.Change) {
	changes := make([]agent.Change, 0)
	result := content
	
	// 检查引号配对
	result, quoteChanges := ga.checkQuotePairs(result)
	changes = append(changes, quoteChanges...)
	
	// 检查括号配对
	result, bracketChanges := ga.checkBracketPairs(result)
	changes = append(changes, bracketChanges...)
	
	return result, changes
}

// checkQuotePairs 检查引号配对
func (ga *GrammarAgent) checkQuotePairs(content string) (string, []agent.Change) {
	// 简单实现：计数检查
	changes := make([]agent.Change, 0)
	
	leftQuote := strings.Count(content, """)
	rightQuote := strings.Count(content, """)
	
	if leftQuote != rightQuote {
		changes = append(changes, agent.Change{
			Type:       agent.ChangeTypePunctuation,
			Original:   "",
			Modified:   "",
			Reason:     fmt.Sprintf("引号不匹配：左引号%d个，右引号%d个", leftQuote, rightQuote),
			Confidence: 0.8,
		})
	}
	
	return content, changes
}

// checkBracketPairs 检查括号配对
func (ga *GrammarAgent) checkBracketPairs(content string) (string, []agent.Change) {
	changes := make([]agent.Change, 0)
	
	brackets := map[rune]rune{
		'（': '）',
		'(': ')',
		'[': ']',
		'【': '】',
		'{': '}',
	}
	
	for left, right := range brackets {
		leftCount := strings.Count(content, string(left))
		rightCount := strings.Count(content, string(right))
		
		if leftCount != rightCount {
			changes = append(changes, agent.Change{
				Type:       agent.ChangeTypePunctuation,
				Original:   "",
				Modified:   "",
				Reason:     fmt.Sprintf("括号%c和%c不匹配", left, right),
				Confidence: 0.8,
			})
		}
	}
	
	return content, changes
}

// llmGrammarCheck 使用LLM进行语法检查
func (ga *GrammarAgent) llmGrammarCheck(ctx context.Context, content string, articleType agent.ArticleType) ([]agent.Change, string, int, error) {
	prompt := ga.buildGrammarPrompt(content, articleType)
	
	result, err := ga.llmManager.GenerateText(ctx, prompt, "grammar")
	if err != nil {
		return nil, content, 0, err
	}
	
	// 解析LLM返回结果
	changes, correctedContent := ga.parseLLMResult(content, result)
	
	// 估算token数
	tokens := len(prompt)/4 + len(result)/4
	
	return changes, correctedContent, tokens, nil
}

// buildGrammarPrompt 构建语法检查提示
func (ga *GrammarAgent) buildGrammarPrompt(content string, articleType agent.ArticleType) string {
	typeDesc := "通用"
	if articleType == agent.ArticleTypeTech {
		typeDesc = "技术类"
	}
	
	return fmt.Sprintf(`你是一位专业的中文语法检查专家。请检查以下%s微信公众号文章的语法问题。

检查要点：
1. 错别字和同音字错误
2. 语法错误（主谓宾搭配、修饰语位置等）
3. 标点符号使用是否正确
4. 中英文混排是否规范（中英文之间加空格）
5. 专业术语拼写是否正确

请按以下格式返回修改建议：
[原文]: 原始文本
[修改]: 修改后文本
[原因]: 修改原因

如果没有需要修改的地方，请直接返回"无语法错误"。

文章内容：
%s`, typeDesc, content)
}

// parseLLMResult 解析LLM返回结果
func (ga *GrammarAgent) parseLLMResult(original, result string) ([]agent.Change, string) {
	changes := make([]agent.Change, 0)
	corrected := original
	
	if strings.Contains(result, "无语法错误") {
		return changes, original
	}
	
	// 解析修改建议
	lines := strings.Split(result, "\n")
	var currentOriginal, currentModified, currentReason string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[原文]:") || strings.HasPrefix(line, "[原文]：") {
			currentOriginal = strings.TrimPrefix(strings.TrimPrefix(line, "[原文]:"), "[原文]：")
			currentOriginal = strings.TrimSpace(currentOriginal)
		} else if strings.HasPrefix(line, "[修改]:") || strings.HasPrefix(line, "[修改]：") {
			currentModified = strings.TrimPrefix(strings.TrimPrefix(line, "[修改]:"), "[修改]：")
			currentModified = strings.TrimSpace(currentModified)
		} else if strings.HasPrefix(line, "[原因]:") || strings.HasPrefix(line, "[原因]：") {
			currentReason = strings.TrimPrefix(strings.TrimPrefix(line, "[原因]:"), "[原因]：")
			currentReason = strings.TrimSpace(currentReason)
			
			// 记录修改
			if currentOriginal != "" && currentModified != "" {
				changes = append(changes, agent.Change{
					Type:       agent.ChangeTypeGrammar,
					Original:   currentOriginal,
					Modified:   currentModified,
					Reason:     currentReason,
					Confidence: 0.85,
				})
				
				// 应用修改
				corrected = strings.Replace(corrected, currentOriginal, currentModified, 1)
			}
			
			// 重置
			currentOriginal = ""
			currentModified = ""
			currentReason = ""
		}
	}
	
	return changes, corrected
}

// CheckCommonErrors 检查常见错误
func (ga *GrammarAgent) CheckCommonErrors(content string) []agent.Change {
	changes := make([]agent.Change, 0)
	
	// 常见错别字映射
	commonErrors := map[string]string{
		"帐号": "账号",
		"帐户": "账户",
		"登陆": "登录",
		"即时": "及时",
		"做为": "作为",
		"象是": "像是",
		"在于": "在于",
	}
	
	for wrong, correct := range commonErrors {
		if strings.Contains(content, wrong) {
			changes = append(changes, agent.Change{
				Type:       agent.ChangeTypeSpelling,
				Original:   wrong,
				Modified:   correct,
				Reason:     fmt.Sprintf("常见错别字："%s"应为"%s"", wrong, correct),
				Confidence: 0.95,
			})
		}
	}
	
	return changes
}
