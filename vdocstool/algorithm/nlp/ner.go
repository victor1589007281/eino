// Package nlp 命名实体识别实现
package nlp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/cloudwego/eino/vdocstool/algorithm/intent/llm"
	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// NewNERExtractor 创建NER提取器
func NewNERExtractor(cfg *types.NERConfig) (NERExtractor, error) {
	switch cfg.Provider {
	case "llm":
		return NewLLMNER(cfg)
	case "rule":
		return NewRuleNER(), nil
	default:
		return NewRuleNER(), nil
	}
}

// LLMNER LLM NER提取器
type LLMNER struct {
	config *types.NERConfig
	client llm.Client
}

// NewLLMNER 创建LLM NER
func NewLLMNER(cfg *types.NERConfig) (*LLMNER, error) {
	// 复用 LLM 配置创建客户端
	llmCfg := &types.LLMEngineConfig{
		Provider: "openai",
		Model:    cfg.Model,
		// 其他配置从环境变量读取
	}
	
	client, err := llm.NewOpenAIClient(llmCfg)
	if err != nil {
		return nil, err
	}

	return &LLMNER{
		config: cfg,
		client: client,
	}, nil
}

// Extract 提取实体
func (n *LLMNER) Extract(ctx context.Context, text string) ([]*Entity, error) {
	prompt := fmt.Sprintf(`请从以下文本中提取命名实体。

文本：%s

请以JSON数组格式返回，每个实体包含：
- text: 实体文本
- type: 实体类型（PERSON/ORGANIZATION/LOCATION/TIME/DATE/TECHNOLOGY/PRODUCT）
- start: 在原文中的起始位置（字符索引）
- end: 在原文中的结束位置

只返回JSON数组，不要其他文字。`, text)

	resp, err := n.client.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.ChatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   500,
		Temperature: 0.1,
	})
	if err != nil {
		return nil, err
	}

	// 解析响应
	var entities []*Entity
	if err := json.Unmarshal([]byte(resp.Text), &entities); err != nil {
		// 尝试提取 JSON
		jsonText := extractJSONArray(resp.Text)
		if jsonText != "" {
			json.Unmarshal([]byte(jsonText), &entities)
		}
	}

	return entities, nil
}

// Close 关闭
func (n *LLMNER) Close() error {
	if n.client != nil {
		return n.client.Close()
	}
	return nil
}

// extractJSONArray 从文本中提取JSON数组
func extractJSONArray(text string) string {
	start := -1
	end := -1
	depth := 0

	for i, c := range text {
		if c == '[' {
			if start == -1 {
				start = i
			}
			depth++
		} else if c == ']' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}

	if start >= 0 && end > start {
		return text[start:end]
	}
	return ""
}

// RuleNER 规则NER提取器
type RuleNER struct {
	patterns map[string]*regexp.Regexp
}

// NewRuleNER 创建规则NER
func NewRuleNER() *RuleNER {
	n := &RuleNER{
		patterns: make(map[string]*regexp.Regexp),
	}
	n.initPatterns()
	return n
}

// initPatterns 初始化模式
func (n *RuleNER) initPatterns() {
	// 时间模式
	n.patterns[EntityTime] = regexp.MustCompile(`\d{1,2}:\d{2}(:\d{2})?`)
	
	// 日期模式
	n.patterns[EntityDate] = regexp.MustCompile(`(\d{4}[-/年]\d{1,2}[-/月]\d{1,2}日?)|(\d{1,2}[-/月]\d{1,2}日?)|(今天|昨天|明天|本周|上周|本月|上月)`)
	
	// 邮箱模式
	n.patterns["EMAIL"] = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	
	// URL模式
	n.patterns["URL"] = regexp.MustCompile(`https?://[^\s]+`)
	
	// 金额模式
	n.patterns[EntityMoney] = regexp.MustCompile(`[¥$€]\s*\d+(\.\d{2})?|\d+(\.\d{2})?\s*(元|美元|欧元|块)`)
	
	// 百分比模式
	n.patterns[EntityPercent] = regexp.MustCompile(`\d+(\.\d+)?%`)
	
	// 技术词汇（简单匹配）
	n.patterns[EntityTechnology] = regexp.MustCompile(`(?i)(redis|mysql|mongodb|postgresql|elasticsearch|milvus|neo4j|kafka|docker|kubernetes|go|golang|python|java|javascript|react|vue|angular)`)
}

// Extract 提取实体
func (n *RuleNER) Extract(ctx context.Context, text string) ([]*Entity, error) {
	var entities []*Entity

	for entityType, pattern := range n.patterns {
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			entities = append(entities, &Entity{
				Text:       text[match[0]:match[1]],
				Type:       entityType,
				Start:      match[0],
				End:        match[1],
				Confidence: 0.9,
			})
		}
	}

	return entities, nil
}

// Close 关闭
func (n *RuleNER) Close() error {
	return nil
}
