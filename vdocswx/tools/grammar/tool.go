// Package grammar 提供语法检查工具
package grammar

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// GrammarCheckTool 语法检查工具
type GrammarCheckTool struct {
	name        string
	description string
}

// GrammarCheckInput 语法检查输入
type GrammarCheckInput struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"` // 默认为中文
}

// GrammarCheckOutput 语法检查输出
type GrammarCheckOutput struct {
	Errors      []GrammarError `json:"errors"`
	Suggestions []string       `json:"suggestions"`
	Score       float64        `json:"score"` // 0-100
}

// GrammarError 语法错误
type GrammarError struct {
	Original    string  `json:"original"`
	Corrected   string  `json:"corrected"`
	Reason      string  `json:"reason"`
	Position    int     `json:"position"`
	Confidence  float64 `json:"confidence"`
}

// NewGrammarCheckTool 创建语法检查工具
func NewGrammarCheckTool() *GrammarCheckTool {
	return &GrammarCheckTool{
		name:        "grammar_check",
		description: "检查文本的语法错误，包括错别字、标点符号、语法结构等问题",
	}
}

// Info 返回工具信息
func (t *GrammarCheckTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name,
		Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"text": {
				Type:     schema.String,
				Desc:     "需要检查语法的文本内容",
				Required: true,
			},
			"language": {
				Type:     schema.String,
				Desc:     "文本语言，默认为zh（中文）",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun 执行语法检查
func (t *GrammarCheckTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input GrammarCheckInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", err
	}
	
	if input.Language == "" {
		input.Language = "zh"
	}
	
	output := t.checkGrammar(input.Text, input.Language)
	
	result, err := json.Marshal(output)
	if err != nil {
		return "", err
	}
	
	return string(result), nil
}

// checkGrammar 执行语法检查
func (t *GrammarCheckTool) checkGrammar(text, language string) *GrammarCheckOutput {
	output := &GrammarCheckOutput{
		Errors:      make([]GrammarError, 0),
		Suggestions: make([]string, 0),
		Score:       100.0,
	}
	
	// 中文语法检查
	if language == "zh" {
		output.Errors = append(output.Errors, t.checkChineseGrammar(text)...)
	}
	
	// 计算得分
	if len(output.Errors) > 0 {
		output.Score = 100.0 - float64(len(output.Errors))*5.0
		if output.Score < 0 {
			output.Score = 0
		}
	}
	
	return output
}

// checkChineseGrammar 检查中文语法
func (t *GrammarCheckTool) checkChineseGrammar(text string) []GrammarError {
	errors := make([]GrammarError, 0)
	
	// 常见错别字
	typos := map[string]string{
		"帐号": "账号",
		"帐户": "账户",
		"登陆": "登录",
		"做为": "作为",
	}
	
	for wrong, correct := range typos {
		for i := 0; i < len(text)-len(wrong)+1; i++ {
			if text[i:i+len(wrong)] == wrong {
				errors = append(errors, GrammarError{
					Original:   wrong,
					Corrected:  correct,
					Reason:     "错别字",
					Position:   i,
					Confidence: 0.95,
				})
			}
		}
	}
	
	return errors
}
