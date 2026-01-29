// Package master 实现意图识别器
package master

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/vdocswx/agent"
	"github.com/cloudwego/eino/vdocswx/llm"
)

// IntentRecognizer 意图识别器
type IntentRecognizer struct {
	llmManager *llm.LLMManager
	patterns   map[agent.IntentType][]string
}

// NewIntentRecognizer 创建意图识别器
func NewIntentRecognizer(llmMgr *llm.LLMManager) *IntentRecognizer {
	ir := &IntentRecognizer{
		llmManager: llmMgr,
		patterns:   make(map[agent.IntentType][]string),
	}
	ir.initPatterns()
	return ir
}

// initPatterns 初始化意图模式
func (ir *IntentRecognizer) initPatterns() {
	ir.patterns[agent.IntentTypePolish] = []string{
		"润色", "优化", "改善", "修改全文", "帮我改", "整体优化",
		"polish", "improve", "enhance",
	}
	
	ir.patterns[agent.IntentTypeGrammar] = []string{
		"语法", "错别字", "拼写", "标点", "grammar", "typo",
	}
	
	ir.patterns[agent.IntentTypeStyle] = []string{
		"风格", "语气", "口吻", "style", "tone",
	}
	
	ir.patterns[agent.IntentTypeStructure] = []string{
		"结构", "段落", "章节", "大纲", "structure", "outline",
	}
	
	ir.patterns[agent.IntentTypeQuestion] = []string{
		"什么", "为什么", "怎么", "如何", "是不是", "能不能",
		"what", "why", "how", "can", "?", "？",
	}
	
	ir.patterns[agent.IntentTypeModify] = []string{
		"修改", "改成", "替换", "删除", "添加", "插入",
		"change", "replace", "delete", "add", "insert",
	}
	
	ir.patterns[agent.IntentTypeRollback] = []string{
		"撤销", "回滚", "恢复", "取消", "undo", "rollback", "revert",
	}
	
	ir.patterns[agent.IntentTypeExport] = []string{
		"导出", "保存", "下载", "export", "save", "download",
	}
}

// Recognize 识别用户意图
func (ir *IntentRecognizer) Recognize(ctx context.Context, userInput, articleContent string) *agent.Intent {
	// 如果没有用户输入，默认为润色
	if userInput == "" {
		return &agent.Intent{
			Type:       agent.IntentTypePolish,
			Confidence: 1.0,
			RawQuery:   "",
		}
	}
	
	// 首先尝试规则匹配
	intent := ir.ruleBasedRecognition(userInput)
	if intent.Confidence >= 0.8 {
		return intent
	}
	
	// 规则匹配置信度不足时，如果有LLM则使用LLM增强
	if ir.llmManager != nil {
		llmIntent := ir.llmBasedRecognition(ctx, userInput, articleContent)
		if llmIntent.Confidence > intent.Confidence {
			return llmIntent
		}
	}
	
	return intent
}

// ruleBasedRecognition 基于规则的意图识别
func (ir *IntentRecognizer) ruleBasedRecognition(input string) *agent.Intent {
	input = strings.ToLower(input)
	
	bestIntent := agent.IntentTypePolish // 默认意图
	bestScore := 0.0
	
	for intentType, patterns := range ir.patterns {
		score := ir.calculatePatternScore(input, patterns)
		if score > bestScore {
			bestScore = score
			bestIntent = intentType
		}
	}
	
	// 将分数转换为置信度
	confidence := bestScore / float64(len(ir.patterns[bestIntent]))
	if confidence > 1.0 {
		confidence = 1.0
	}
	
	return &agent.Intent{
		Type:       bestIntent,
		Confidence: confidence,
		RawQuery:   input,
		Entities:   ir.extractEntities(input, bestIntent),
	}
}

// calculatePatternScore 计算模式匹配分数
func (ir *IntentRecognizer) calculatePatternScore(input string, patterns []string) float64 {
	score := 0.0
	for _, pattern := range patterns {
		if strings.Contains(input, pattern) {
			score += 1.0
		}
	}
	return score
}

// extractEntities 提取实体
func (ir *IntentRecognizer) extractEntities(input string, intentType agent.IntentType) map[string]string {
	entities := make(map[string]string)
	
	switch intentType {
	case agent.IntentTypeModify:
		// 尝试提取修改目标和修改内容
		if idx := strings.Index(input, "改成"); idx != -1 {
			entities["target"] = strings.TrimSpace(input[:idx])
			entities["replacement"] = strings.TrimSpace(input[idx+6:])
		} else if idx := strings.Index(input, "替换为"); idx != -1 {
			entities["target"] = strings.TrimSpace(input[:idx])
			entities["replacement"] = strings.TrimSpace(input[idx+9:])
		}
		
	case agent.IntentTypeQuestion:
		// 提取问题主题
		entities["topic"] = input
	}
	
	return entities
}

// llmBasedRecognition 基于LLM的意图识别
func (ir *IntentRecognizer) llmBasedRecognition(ctx context.Context, userInput, articleContent string) *agent.Intent {
	prompt := `你是一个意图识别助手。分析用户输入，识别其意图类型。

可能的意图类型：
1. polish - 全文润色优化
2. grammar - 语法检查和纠正
3. style - 风格优化
4. structure - 结构调整
5. question - 咨询问题
6. modify - 指定位置修改
7. rollback - 撤销修改
8. export - 导出结果

用户输入：` + userInput + `

请只返回意图类型（如 polish），不要返回其他内容。`

	result, err := ir.llmManager.GenerateText(ctx, prompt, "intent")
	if err != nil {
		// LLM失败时返回默认意图
		return &agent.Intent{
			Type:       agent.IntentTypePolish,
			Confidence: 0.5,
			RawQuery:   userInput,
		}
	}
	
	// 解析LLM返回的意图
	intentType := ir.parseIntentType(strings.TrimSpace(result))
	
	return &agent.Intent{
		Type:       intentType,
		Confidence: 0.85, // LLM识别给予较高置信度
		RawQuery:   userInput,
		Entities:   ir.extractEntities(userInput, intentType),
	}
}

// parseIntentType 解析意图类型字符串
func (ir *IntentRecognizer) parseIntentType(s string) agent.IntentType {
	s = strings.ToLower(s)
	switch s {
	case "polish":
		return agent.IntentTypePolish
	case "grammar":
		return agent.IntentTypeGrammar
	case "style":
		return agent.IntentTypeStyle
	case "structure":
		return agent.IntentTypeStructure
	case "question":
		return agent.IntentTypeQuestion
	case "modify":
		return agent.IntentTypeModify
	case "rollback":
		return agent.IntentTypeRollback
	case "export":
		return agent.IntentTypeExport
	default:
		return agent.IntentTypePolish
	}
}
