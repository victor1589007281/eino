// Package algorithm 算法层配置
// 此文件重新导出 types 包的配置类型以保持向后兼容性
package algorithm

import "github.com/cloudwego/eino/vdocstool/algorithm/types"

// 类型别名 - 重新导出 types 包的所有配置类型
type (
	Config              = types.Config
	IntentConfig        = types.IntentConfig
	RuleEngineConfig    = types.RuleEngineConfig
	MLEngineConfig      = types.MLEngineConfig
	LLMEngineConfig     = types.LLMEngineConfig
	FusionConfig        = types.FusionConfig
	EmbeddingConfig     = types.EmbeddingConfig
	EmbeddingCacheConfig = types.EmbeddingCacheConfig
	NLPConfig           = types.NLPConfig
	TokenizerConfig     = types.TokenizerConfig
	NERConfig           = types.NERConfig
	RelationConfig      = types.RelationConfig
	SummarizerConfig    = types.SummarizerConfig
	IndexConfig         = types.IndexConfig
	ESConfig            = types.ESConfig
	MilvusConfig        = types.MilvusConfig
	Neo4jConfig         = types.Neo4jConfig
)

// LLM 提供商常量
const (
	LLMProviderOpenAI   = types.LLMProviderOpenAI
	LLMProviderClaude   = types.LLMProviderClaude
	LLMProviderGemini   = types.LLMProviderGemini
	LLMProviderCohere   = types.LLMProviderCohere
	LLMProviderQwen     = types.LLMProviderQwen
	LLMProviderWenxin   = types.LLMProviderWenxin
	LLMProviderGLM      = types.LLMProviderGLM
	LLMProviderSpark    = types.LLMProviderSpark
	LLMProviderDeepSeek = types.LLMProviderDeepSeek
	LLMProviderBaichuan = types.LLMProviderBaichuan
)

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return types.DefaultConfig()
}
