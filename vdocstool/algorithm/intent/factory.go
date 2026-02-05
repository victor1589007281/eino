// Package intent 引擎工厂
package intent

import (
	"github.com/cloudwego/eino/vdocstool/algorithm/intent/llm"
	"github.com/cloudwego/eino/vdocstool/algorithm/intent/ml"
	"github.com/cloudwego/eino/vdocstool/algorithm/intent/rule"
	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// NewRuleEngine 创建规则引擎
func NewRuleEngine(cfg *types.RuleEngineConfig) (Recognizer, error) {
	return rule.NewEngine(cfg)
}

// NewMLEngine 创建ML引擎
func NewMLEngine(cfg *types.MLEngineConfig) (Recognizer, error) {
	return ml.NewEngine(cfg)
}

// NewLLMEngine 创建LLM引擎
func NewLLMEngine(cfg *types.LLMEngineConfig) (Recognizer, error) {
	return llm.NewEngine(cfg)
}
