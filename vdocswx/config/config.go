/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 主配置结构
type Config struct {
	// 路径配置
	Paths PathsConfig `yaml:"paths"`

	// 索引配置
	Index IndexConfig `yaml:"index"`

	// LLM配置
	LLM LLMConfig `yaml:"llm"`

	// 模型路由配置
	Router RouterConfig `yaml:"router"`

	// Agent配置
	Agent AgentConfig `yaml:"agent"`

	// 输出配置
	Output OutputConfig `yaml:"output"`

	// 缓存配置
	Cache CacheConfig `yaml:"cache"`

	// 统计配置
	Stats StatsConfig `yaml:"stats"`

	// A2A配置
	A2A A2AConfig `yaml:"a2a"`

	// API配置
	API APIConfig `yaml:"api"`
}

// PathsConfig 路径配置
type PathsConfig struct {
	WorkPath   string `yaml:"work_path"`
	InputPath  string `yaml:"input_path"`
	OutputPath string `yaml:"output_path"`
	SkillsPath string `yaml:"skills_path"`
}

// IndexConfig 索引配置
type IndexConfig struct {
	Type           string          `yaml:"type"` // sqlite | external
	Path           string          `yaml:"path"`
	RebuildOnStart bool            `yaml:"rebuild_on_start"`
	ParallelWorkers int            `yaml:"parallel_workers"`
	External       ExternalDBConfig `yaml:"external"`
}

// ExternalDBConfig 外部数据库配置
type ExternalDBConfig struct {
	Driver string `yaml:"driver"` // mysql | postgres
	DSN    string `yaml:"dsn"`
}

// LLMConfig LLM配置
type LLMConfig struct {
	// 国内模型
	DeepSeek DeepSeekConfig `yaml:"deepseek"`
	Qwen     QwenConfig     `yaml:"qwen"`
	Ernie    ErnieConfig    `yaml:"ernie"`
	GLM      GLMConfig      `yaml:"glm"`
	Moonshot MoonshotConfig `yaml:"moonshot"`

	// 国外模型
	OpenAI    OpenAIConfig    `yaml:"openai"`
	Anthropic AnthropicConfig `yaml:"anthropic"`
	Gemini    GeminiConfig    `yaml:"gemini"`

	// 本地模型
	Ollama OllamaConfig `yaml:"ollama"`
	VLLM   VLLMConfig   `yaml:"vllm"`
}

// DeepSeekConfig DeepSeek配置
type DeepSeekConfig struct {
	Enabled     bool    `yaml:"enabled"`
	APIKeyEnv   string  `yaml:"api_key_env"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// QwenConfig 通义千问配置
type QwenConfig struct {
	Enabled     bool    `yaml:"enabled"`
	APIKeyEnv   string  `yaml:"api_key_env"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// ErnieConfig 文心一言配置
type ErnieConfig struct {
	Enabled      bool    `yaml:"enabled"`
	APIKeyEnv    string  `yaml:"api_key_env"`
	SecretKeyEnv string  `yaml:"secret_key_env"`
	Model        string  `yaml:"model"`
	MaxTokens    int     `yaml:"max_tokens"`
	Temperature  float64 `yaml:"temperature"`
}

// GLMConfig 智谱AI配置
type GLMConfig struct {
	Enabled     bool    `yaml:"enabled"`
	APIKeyEnv   string  `yaml:"api_key_env"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// MoonshotConfig 月之暗面配置
type MoonshotConfig struct {
	Enabled     bool    `yaml:"enabled"`
	APIKeyEnv   string  `yaml:"api_key_env"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// OpenAIConfig OpenAI配置
type OpenAIConfig struct {
	Enabled     bool    `yaml:"enabled"`
	APIKeyEnv   string  `yaml:"api_key_env"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// AnthropicConfig Anthropic配置
type AnthropicConfig struct {
	Enabled     bool    `yaml:"enabled"`
	APIKeyEnv   string  `yaml:"api_key_env"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// GeminiConfig Google Gemini配置
type GeminiConfig struct {
	Enabled     bool    `yaml:"enabled"`
	APIKeyEnv   string  `yaml:"api_key_env"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// OllamaConfig Ollama配置
type OllamaConfig struct {
	Enabled     bool    `yaml:"enabled"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// VLLMConfig vLLM配置
type VLLMConfig struct {
	Enabled     bool    `yaml:"enabled"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// RouterConfig 模型路由配置
type RouterConfig struct {
	DefaultModel string                     `yaml:"default_model"`
	Strategies   map[string]StrategyConfig  `yaml:"strategies"`
	Fallbacks    []string                   `yaml:"fallbacks"`
}

// StrategyConfig 路由策略配置
type StrategyConfig struct {
	Primary    string            `yaml:"primary"`
	Fallback   string            `yaml:"fallback"`
	Conditions []ConditionConfig `yaml:"conditions"`
}

// ConditionConfig 路由条件配置
type ConditionConfig struct {
	Metric   string      `yaml:"metric"`   // token_count, complexity, type
	Operator string      `yaml:"operator"` // gt, lt, eq
	Value    interface{} `yaml:"value"`
	Target   string      `yaml:"target"`
}

// AgentConfig Agent配置
type AgentConfig struct {
	MaxIterations       int           `yaml:"max_iterations"`
	MaxConcurrentAgents int           `yaml:"max_concurrent_agents"`
	Timeout             time.Duration `yaml:"timeout"`
	Agents              []string      `yaml:"agents"`
}

// OutputConfig 输出配置
type OutputConfig struct {
	Mode               string `yaml:"mode"` // summary | document
	IncludeDiff        bool   `yaml:"include_diff"`
	IncludeStats       bool   `yaml:"include_stats"`
	IncludeSuggestions bool   `yaml:"include_suggestions"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled  bool         `yaml:"enabled"`
	Type     string       `yaml:"type"` // memory | redis
	TTL      time.Duration `yaml:"ttl"`
	MaxSize  int          `yaml:"max_size"`
	L1       L1CacheConfig `yaml:"l1"`
	L2       L2CacheConfig `yaml:"l2"`
	L3       L3CacheConfig `yaml:"l3"`
	LLM      LLMCacheConfig `yaml:"llm"`
	Index    IndexCacheConfig `yaml:"index"`
	Memory   MemoryConfig `yaml:"memory"`
}

// L1CacheConfig L1缓存配置
type L1CacheConfig struct {
	MaxSize int `yaml:"max_size"`
}

// L2CacheConfig L2缓存配置
type L2CacheConfig struct {
	Path         string        `yaml:"path"`
	SyncInterval time.Duration `yaml:"sync_interval"`
}

// L3CacheConfig L3缓存配置
type L3CacheConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Addr       string `yaml:"addr"`
	Password   string `yaml:"password"`
	DB         int    `yaml:"db"`
	PoolSize   int    `yaml:"pool_size"`
	Prefix     string `yaml:"prefix"`
	PasswordEnv string `yaml:"password_env"`
}

// LLMCacheConfig LLM缓存配置
type LLMCacheConfig struct {
	TTL                  time.Duration `yaml:"ttl"`
	MaxTokens            int           `yaml:"max_tokens"`
	SimilarityThreshold  float64       `yaml:"similarity_threshold"`
}

// IndexCacheConfig 索引缓存配置
type IndexCacheConfig struct {
	TermCacheSize    int `yaml:"term_cache_size"`
	DocCacheSize     int `yaml:"doc_cache_size"`
	SummaryCacheSize int `yaml:"summary_cache_size"`
}

// MemoryConfig 记忆配置
type MemoryConfig struct {
	SessionTTL      time.Duration `yaml:"session_ttl"`
	MaxHistory      int           `yaml:"max_history"`
	PersistInterval time.Duration `yaml:"persist_interval"`
}

// StatsConfig 统计配置
type StatsConfig struct {
	Enabled        bool          `yaml:"enabled"`
	ExportInterval time.Duration `yaml:"export_interval"`
	ExportFormat   string        `yaml:"export_format"` // json | prometheus
	ExportPath     string        `yaml:"export_path"`
}

// A2AConfig A2A配置
type A2AConfig struct {
	Enabled    bool           `yaml:"enabled"`
	ServerPort int            `yaml:"server_port"`
	Discovery  DiscoveryConfig `yaml:"discovery"`
}

// DiscoveryConfig 服务发现配置
type DiscoveryConfig struct {
	Type      string   `yaml:"type"` // static | consul | etcd
	Endpoints []string `yaml:"endpoints"`
}

// APIConfig API配置
type APIConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Host    string `yaml:"host"`
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 展开环境变量
	data = []byte(os.ExpandEnv(string(data)))

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 设置默认值
	setDefaults(&config)

	// 验证配置
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults 设置默认值
func setDefaults(config *Config) {
	// 路径默认值
	if config.Paths.WorkPath == "" {
		config.Paths.WorkPath = "./workspace"
	}
	if config.Paths.InputPath == "" {
		config.Paths.InputPath = "./input"
	}
	if config.Paths.OutputPath == "" {
		config.Paths.OutputPath = "./output"
	}
	if config.Paths.SkillsPath == "" {
		config.Paths.SkillsPath = "./skills"
	}

	// 索引默认值
	if config.Index.Type == "" {
		config.Index.Type = "sqlite"
	}
	if config.Index.Path == "" {
		config.Index.Path = "./data/index.db"
	}
	if config.Index.ParallelWorkers == 0 {
		config.Index.ParallelWorkers = 4
	}

	// LLM默认值
	if config.LLM.DeepSeek.BaseURL == "" {
		config.LLM.DeepSeek.BaseURL = "https://api.deepseek.com/v1"
	}
	if config.LLM.DeepSeek.Model == "" {
		config.LLM.DeepSeek.Model = "deepseek-chat"
	}
	if config.LLM.DeepSeek.MaxTokens == 0 {
		config.LLM.DeepSeek.MaxTokens = 4096
	}
	if config.LLM.DeepSeek.Temperature == 0 {
		config.LLM.DeepSeek.Temperature = 0.3
	}

	if config.LLM.Qwen.BaseURL == "" {
		config.LLM.Qwen.BaseURL = "https://dashscope.aliyuncs.com/api/v1"
	}
	if config.LLM.Qwen.Model == "" {
		config.LLM.Qwen.Model = "qwen-max"
	}

	if config.LLM.Ollama.BaseURL == "" {
		config.LLM.Ollama.BaseURL = "http://localhost:11434"
	}
	if config.LLM.Ollama.Model == "" {
		config.LLM.Ollama.Model = "qwen2.5:14b"
	}

	// 路由默认值
	if config.Router.DefaultModel == "" {
		config.Router.DefaultModel = "deepseek"
	}

	// Agent默认值
	if config.Agent.MaxIterations == 0 {
		config.Agent.MaxIterations = 20
	}
	if config.Agent.MaxConcurrentAgents == 0 {
		config.Agent.MaxConcurrentAgents = 5
	}
	if config.Agent.Timeout == 0 {
		config.Agent.Timeout = 5 * time.Minute
	}
	if len(config.Agent.Agents) == 0 {
		config.Agent.Agents = []string{"grammar", "logic", "structure", "style", "content"}
	}

	// 输出默认值
	if config.Output.Mode == "" {
		config.Output.Mode = "document"
	}

	// 缓存默认值
	if config.Cache.TTL == 0 {
		config.Cache.TTL = 30 * time.Minute
	}
	if config.Cache.MaxSize == 0 {
		config.Cache.MaxSize = 10000
	}
	if config.Cache.L1.MaxSize == 0 {
		config.Cache.L1.MaxSize = 10000
	}
	if config.Cache.L2.Path == "" {
		config.Cache.L2.Path = "./data/cache.db"
	}
	if config.Cache.Index.TermCacheSize == 0 {
		config.Cache.Index.TermCacheSize = 5000
	}
	if config.Cache.Index.DocCacheSize == 0 {
		config.Cache.Index.DocCacheSize = 1000
	}
	if config.Cache.Index.SummaryCacheSize == 0 {
		config.Cache.Index.SummaryCacheSize = 500
	}
	if config.Cache.Memory.SessionTTL == 0 {
		config.Cache.Memory.SessionTTL = 24 * time.Hour
	}
	if config.Cache.Memory.MaxHistory == 0 {
		config.Cache.Memory.MaxHistory = 20
	}
	if config.Cache.LLM.TTL == 0 {
		config.Cache.LLM.TTL = 1 * time.Hour
	}
	if config.Cache.LLM.SimilarityThreshold == 0 {
		config.Cache.LLM.SimilarityThreshold = 0.85
	}

	// 统计默认值
	if config.Stats.ExportInterval == 0 {
		config.Stats.ExportInterval = 60 * time.Second
	}
	if config.Stats.ExportFormat == "" {
		config.Stats.ExportFormat = "json"
	}
	if config.Stats.ExportPath == "" {
		config.Stats.ExportPath = "./data/stats"
	}

	// API默认值
	if config.API.Port == 0 {
		config.API.Port = 8080
	}
	if config.API.Host == "" {
		config.API.Host = "0.0.0.0"
	}
}

// validate 验证配置
func validate(config *Config) error {
	// 验证至少有一个LLM启用
	hasLLM := config.LLM.DeepSeek.Enabled ||
		config.LLM.Qwen.Enabled ||
		config.LLM.Ernie.Enabled ||
		config.LLM.GLM.Enabled ||
		config.LLM.Moonshot.Enabled ||
		config.LLM.OpenAI.Enabled ||
		config.LLM.Anthropic.Enabled ||
		config.LLM.Gemini.Enabled ||
		config.LLM.Ollama.Enabled ||
		config.LLM.VLLM.Enabled

	if !hasLLM {
		return fmt.Errorf("at least one LLM provider must be enabled")
	}

	return nil
}

// GetAPIKey 获取API密钥
func GetAPIKey(envName string) string {
	if envName == "" {
		return ""
	}
	return os.Getenv(envName)
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	config := &Config{}
	setDefaults(config)
	config.LLM.Ollama.Enabled = true // 默认启用本地模型
	return config
}
