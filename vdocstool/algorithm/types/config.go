// Package types 算法层共享类型定义
package types

import (
	"context"
	"time"
)

// ======================
// 意图识别相关类型
// ======================

// Recognizer 意图识别器接口
type Recognizer interface {
	// Recognize 识别意图
	Recognize(ctx context.Context, input *IntentInput) (*IntentResult, error)

	// Name 引擎名称
	Name() string

	// Domains 支持的领域
	Domains() []string

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error

	// Close 关闭引擎
	Close() error
}

// IntentInput 识别输入
type IntentInput struct {
	// Text 输入文本
	Text string `json:"text"`
	// Domain 领域: email, memory, search, general
	Domain string `json:"domain"`
	// Context 上下文消息
	Context []*IntentMessage `json:"context,omitempty"`
	// Language 语言: zh, en
	Language string `json:"language"`
	// SessionID 会话ID
	SessionID string `json:"session_id,omitempty"`
	// Options 额外选项
	Options map[string]string `json:"options,omitempty"`
}

// IntentMessage 上下文消息
type IntentMessage struct {
	Role      string    `json:"role"` // user, assistant, system
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// IntentResult 识别结果
type IntentResult struct {
	// Intent 主意图
	Intent *Intent `json:"intent"`
	// Alternatives 备选意图
	Alternatives []*Intent `json:"alternatives,omitempty"`
	// Entities 提取的实体
	Entities []*Entity `json:"entities,omitempty"`
	// Confidence 整体置信度
	Confidence float64 `json:"confidence"`
	// Source 来源引擎: rule, ml, llm
	Source string `json:"source"`
	// Reasoning 推理过程说明
	Reasoning string `json:"reasoning,omitempty"`
	// Latency 处理延迟
	Latency time.Duration `json:"latency"`
	// Fallback 是否为降级结果
	Fallback bool `json:"fallback"`
}

// Intent 意图
type Intent struct {
	// Name 意图名称
	Name string `json:"name"`
	// Confidence 置信度
	Confidence float64 `json:"confidence"`
	// Slots 槽位
	Slots map[string]string `json:"slots,omitempty"`
	// Category 意图分类
	Category string `json:"category,omitempty"`
}

// Entity 实体
type Entity struct {
	// Text 实体文本
	Text string `json:"text"`
	// Type 实体类型
	Type string `json:"type"`
	// Start 起始位置
	Start int `json:"start"`
	// End 结束位置
	End int `json:"end"`
	// Score 置信度
	Score float64 `json:"score"`
	// Normalized 标准化值
	Normalized string `json:"normalized,omitempty"`
}

// 常用意图类型
const (
	// 邮件领域意图
	IntentEmailSearch     = "email_search"
	IntentEmailRead       = "email_read"
	IntentEmailDownload   = "email_download_attachment"
	IntentEmailCategorize = "email_categorize"
	IntentEmailSummarize  = "email_summarize"
	IntentEmailSend       = "email_send"
	IntentEmailReply      = "email_reply"

	// 记忆领域意图
	IntentMemoryRecall      = "memory_recall"
	IntentMemoryReference   = "memory_reference"
	IntentMemoryTopicSwitch = "memory_topic_switch"
	IntentMemoryNewTopic    = "memory_new_topic"
	IntentMemorySearch      = "memory_search"

	// 搜索领域意图
	IntentWebSearch      = "web_search"
	IntentWebSearchNews  = "web_search_news"
	IntentWebSearchImage = "web_search_image"
	IntentWebSearchLocal = "web_search_local"

	// 通用意图
	IntentUnknown = "unknown"
	IntentGreet   = "greet"
	IntentConfirm = "confirm"
	IntentCancel  = "cancel"
	IntentHelp    = "help"
)

// 常用实体类型
const (
	EntityTypePerson     = "PERSON"
	EntityTypeTime       = "TIME"
	EntityTypeDate       = "DATE"
	EntityTypeDuration   = "DURATION"
	EntityTypeLocation   = "LOCATION"
	EntityTypeOrg        = "ORGANIZATION"
	EntityTypeEmail      = "EMAIL"
	EntityTypeURL        = "URL"
	EntityTypeNumber     = "NUMBER"
	EntityTypeMoney      = "MONEY"
	EntityTypeKeyword    = "KEYWORD"
	EntityTypeFileType   = "FILE_TYPE"
	EntityTypeTechnology = "TECHNOLOGY"
)

// 领域
const (
	DomainEmail   = "email"
	DomainMemory  = "memory"
	DomainSearch  = "search"
	DomainGeneral = "general"
)

// ======================
// 配置相关类型
// ======================

// Config 算法层配置
type Config struct {
	Intent    IntentConfig    `yaml:"intent" json:"intent"`
	Embedding EmbeddingConfig `yaml:"embedding" json:"embedding"`
	NLP       NLPConfig       `yaml:"nlp" json:"nlp"`
	Index     IndexConfig     `yaml:"index" json:"index"`
}

// IntentConfig 意图引擎配置
type IntentConfig struct {
	// Strategy 识别策略: cascade(级联), voting(投票), adaptive(自适应)
	Strategy      string        `yaml:"strategy" json:"strategy"`
	DefaultDomain string        `yaml:"default_domain" json:"default_domain"`
	Timeout       time.Duration `yaml:"timeout" json:"timeout"`

	// 规则引擎配置
	Rule RuleEngineConfig `yaml:"rule" json:"rule"`
	// ML引擎配置
	ML MLEngineConfig `yaml:"ml" json:"ml"`
	// LLM引擎配置
	LLM LLMEngineConfig `yaml:"llm" json:"llm"`

	// 融合配置
	Fusion FusionConfig `yaml:"fusion" json:"fusion"`
}

// RuleEngineConfig 规则引擎配置
type RuleEngineConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	PatternsDir string `yaml:"patterns_dir" json:"patterns_dir"`
}

// MLEngineConfig ML引擎配置
type MLEngineConfig struct {
	Enabled             bool    `yaml:"enabled" json:"enabled"`
	ModelType           string  `yaml:"model_type" json:"model_type"` // fasttext, bert
	ModelPath           string  `yaml:"model_path" json:"model_path"`
	ConfidenceThreshold float64 `yaml:"confidence_threshold" json:"confidence_threshold"`
}

// LLMEngineConfig LLM引擎配置
type LLMEngineConfig struct {
	Enabled     bool          `yaml:"enabled" json:"enabled"`
	Provider    string        `yaml:"provider" json:"provider"` // 见下方支持列表
	Model       string        `yaml:"model" json:"model"`
	APIKey      string        `yaml:"api_key" json:"api_key"`
	SecretKey   string        `yaml:"secret_key" json:"secret_key"` // 部分提供商需要
	AppID       string        `yaml:"app_id" json:"app_id"`         // 讯飞星火等需要
	BaseURL     string        `yaml:"base_url" json:"base_url"`
	Temperature float64       `yaml:"temperature" json:"temperature"`
	MaxTokens   int           `yaml:"max_tokens" json:"max_tokens"`
	Timeout     time.Duration `yaml:"timeout" json:"timeout"`

	// 支持的提供商:
	// 国外: openai, claude, gemini, cohere
	// 国内: qwen(通义千问), wenxin(文心一言), glm(智谱), spark(讯飞星火), deepseek, baichuan(百川)
}

// LLM 提供商常量
const (
	// 国外大模型
	LLMProviderOpenAI  = "openai"
	LLMProviderClaude  = "claude"
	LLMProviderGemini  = "gemini"
	LLMProviderCohere  = "cohere"

	// 国内大模型
	LLMProviderQwen     = "qwen"     // 通义千问 (阿里)
	LLMProviderWenxin   = "wenxin"   // 文心一言 (百度)
	LLMProviderGLM      = "glm"      // 智谱 ChatGLM
	LLMProviderSpark    = "spark"    // 讯飞星火
	LLMProviderDeepSeek = "deepseek" // DeepSeek
	LLMProviderBaichuan = "baichuan" // 百川
)

// FusionConfig 融合配置
type FusionConfig struct {
	Strategy            string             `yaml:"strategy" json:"strategy"` // weighted_vote, cascade, max_confidence
	Weights             map[string]float64 `yaml:"weights" json:"weights"`   // rule, ml, llm
	ConfidenceThreshold float64            `yaml:"confidence_threshold" json:"confidence_threshold"`
}

// EmbeddingConfig 向量化引擎配置
type EmbeddingConfig struct {
	Provider   string `yaml:"provider" json:"provider"` // openai, bge, sentence-transformer
	Model      string `yaml:"model" json:"model"`
	APIKey     string `yaml:"api_key" json:"api_key"`
	BaseURL    string `yaml:"base_url" json:"base_url"`
	Dimension  int    `yaml:"dimension" json:"dimension"`
	BatchSize  int    `yaml:"batch_size" json:"batch_size"`
	MaxRetries int    `yaml:"max_retries" json:"max_retries"`

	// 缓存配置
	Cache EmbeddingCacheConfig `yaml:"cache" json:"cache"`
}

// EmbeddingCacheConfig 向量缓存配置
type EmbeddingCacheConfig struct {
	Enabled  bool          `yaml:"enabled" json:"enabled"`
	TTL      time.Duration `yaml:"ttl" json:"ttl"`
	MaxSize  int           `yaml:"max_size" json:"max_size"`
	RedisURL string        `yaml:"redis_url" json:"redis_url"`
}

// NLPConfig NLP管道配置
type NLPConfig struct {
	Tokenizer  TokenizerConfig  `yaml:"tokenizer" json:"tokenizer"`
	NER        NERConfig        `yaml:"ner" json:"ner"`
	Relation   RelationConfig   `yaml:"relation" json:"relation"`
	Summarizer SummarizerConfig `yaml:"summarizer" json:"summarizer"`
}

// TokenizerConfig 分词器配置
type TokenizerConfig struct {
	Provider string `yaml:"provider" json:"provider"` // jieba, whitespace, bert
	DictPath string `yaml:"dict_path" json:"dict_path"`
}

// NERConfig NER配置
type NERConfig struct {
	Provider string `yaml:"provider" json:"provider"` // llm, hanlp, spacy
	Model    string `yaml:"model" json:"model"`
}

// RelationConfig 关系抽取配置
type RelationConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Provider string `yaml:"provider" json:"provider"`
}

// SummarizerConfig 摘要配置
type SummarizerConfig struct {
	Provider  string `yaml:"provider" json:"provider"`
	MaxLength int    `yaml:"max_length" json:"max_length"`
}

// IndexConfig 索引引擎配置
type IndexConfig struct {
	// Elasticsearch 配置
	Elasticsearch ESConfig `yaml:"elasticsearch" json:"elasticsearch"`
	// Milvus 配置
	Milvus MilvusConfig `yaml:"milvus" json:"milvus"`
	// Neo4j 配置
	Neo4j Neo4jConfig `yaml:"neo4j" json:"neo4j"`
}

// ESConfig Elasticsearch 配置
type ESConfig struct {
	Addresses []string `yaml:"addresses" json:"addresses"`
	Username  string   `yaml:"username" json:"username"`
	Password  string   `yaml:"password" json:"password"`
	IndexName string   `yaml:"index_name" json:"index_name"`
	Shards    int      `yaml:"shards" json:"shards"`
	Replicas  int      `yaml:"replicas" json:"replicas"`
}

// MilvusConfig Milvus 配置
type MilvusConfig struct {
	Address        string `yaml:"address" json:"address"`
	CollectionName string `yaml:"collection_name" json:"collection_name"`
	Dimension      int    `yaml:"dimension" json:"dimension"`
	IndexType      string `yaml:"index_type" json:"index_type"`   // HNSW, IVF_FLAT
	MetricType     string `yaml:"metric_type" json:"metric_type"` // L2, IP
	NProbe         int    `yaml:"nprobe" json:"nprobe"`
}

// Neo4jConfig Neo4j 配置
type Neo4jConfig struct {
	URI      string `yaml:"uri" json:"uri"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	Database string `yaml:"database" json:"database"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Intent: IntentConfig{
			Strategy:      "cascade",
			DefaultDomain: "general",
			Timeout:       5 * time.Second,
			Rule: RuleEngineConfig{
				Enabled:     true,
				PatternsDir: "config/intent_patterns",
			},
			ML: MLEngineConfig{
				Enabled:             true,
				ModelType:           "fasttext",
				ModelPath:           "models/intent_classifier.bin",
				ConfidenceThreshold: 0.85,
			},
			LLM: LLMEngineConfig{
				Enabled:     true,
				Provider:    "openai",
				Model:       "gpt-4o-mini",
				Temperature: 0.1,
				MaxTokens:   500,
				Timeout:     10 * time.Second,
			},
			Fusion: FusionConfig{
				Strategy: "cascade",
				Weights: map[string]float64{
					"rule": 0.2,
					"ml":   0.3,
					"llm":  0.5,
				},
				ConfidenceThreshold: 0.7,
			},
		},
		Embedding: EmbeddingConfig{
			Provider:   "openai",
			Model:      "text-embedding-3-small",
			Dimension:  1536,
			BatchSize:  100,
			MaxRetries: 3,
			Cache: EmbeddingCacheConfig{
				Enabled: true,
				TTL:     24 * time.Hour,
				MaxSize: 10000,
			},
		},
		NLP: NLPConfig{
			Tokenizer: TokenizerConfig{
				Provider: "jieba",
			},
			NER: NERConfig{
				Provider: "llm",
				Model:    "gpt-4o-mini",
			},
			Relation: RelationConfig{
				Enabled:  true,
				Provider: "llm",
			},
			Summarizer: SummarizerConfig{
				Provider:  "llm",
				MaxLength: 200,
			},
		},
		Index: IndexConfig{
			Elasticsearch: ESConfig{
				Addresses: []string{"http://localhost:9200"},
				IndexName: "vdocs",
				Shards:    1,
				Replicas:  0,
			},
			Milvus: MilvusConfig{
				Address:        "localhost:19530",
				CollectionName: "vdocs_vectors",
				Dimension:      1536,
				IndexType:      "HNSW",
				MetricType:     "L2",
				NProbe:         10,
			},
			Neo4j: Neo4jConfig{
				URI:      "bolt://localhost:7687",
				Database: "vdocs",
			},
		},
	}
}
