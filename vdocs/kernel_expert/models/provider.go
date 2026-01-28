// Package models 提供多模型支持
package models

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"
)

// ModelProvider 模型提供商接口
type ModelProvider interface {
	Name() string
	Models() []string
	CreateModel(ctx context.Context, config *ModelConfig) (model.ToolCallingChatModel, error)
	HealthCheck(ctx context.Context) error
}

// ModelConfig 模型配置
type ModelConfig struct {
	Provider    string            `json:"provider"`
	Model       string            `json:"model"`
	APIKey      string            `json:"api_key"`
	BaseURL     string            `json:"base_url,omitempty"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"max_tokens"`
	TopP        float64           `json:"top_p,omitempty"`
	Extra       map[string]string `json:"extra,omitempty"`
}

// ModelInfo 模型信息
type ModelInfo struct {
	Provider      string  `json:"provider"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	MaxContext    int     `json:"max_context"`
	InputPrice    float64 `json:"input_price"`  // 每1K tokens
	OutputPrice   float64 `json:"output_price"` // 每1K tokens
	SupportsTools bool    `json:"supports_tools"`
	Available     bool    `json:"available"`
}

// ModelRegistry 模型注册中心
type ModelRegistry struct {
	providers map[string]ModelProvider
	models    map[string]*ModelInfo
	mu        sync.RWMutex
}

// NewModelRegistry 创建模型注册中心
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		providers: make(map[string]ModelProvider),
		models:    make(map[string]*ModelInfo),
	}
}

// Register 注册提供商
func (r *ModelRegistry) Register(provider ModelProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
}

// GetProvider 获取提供商
func (r *ModelRegistry) GetProvider(name string) (ModelProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// ListProviders 列出所有提供商
func (r *ModelRegistry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.providers))
	for name := range r.providers {
		result = append(result, name)
	}
	return result
}

// ListModels 列出所有模型
func (r *ModelRegistry) ListModels() []*ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ModelInfo, 0, len(r.models))
	for _, info := range r.models {
		result = append(result, info)
	}
	return result
}

// RegisterModel 注册模型信息
func (r *ModelRegistry) RegisterModel(info *ModelInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models[info.Name] = info
}

// GetModelInfo 获取模型信息
func (r *ModelRegistry) GetModelInfo(name string) (*ModelInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.models[name]
	return info, ok
}

// CreateModel 创建模型实例
func (r *ModelRegistry) CreateModel(ctx context.Context, config *ModelConfig) (model.ToolCallingChatModel, error) {
	r.mu.RLock()
	provider, ok := r.providers[config.Provider]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", config.Provider)
	}

	return provider.CreateModel(ctx, config)
}

// 预定义的模型信息
var DefaultModels = []*ModelInfo{
	// OpenAI
	{Provider: "openai", Name: "gpt-4-turbo", MaxContext: 128000, InputPrice: 0.01, OutputPrice: 0.03, SupportsTools: true},
	{Provider: "openai", Name: "gpt-4o", MaxContext: 128000, InputPrice: 0.005, OutputPrice: 0.015, SupportsTools: true},
	{Provider: "openai", Name: "gpt-4o-mini", MaxContext: 128000, InputPrice: 0.00015, OutputPrice: 0.0006, SupportsTools: true},

	// Anthropic
	{Provider: "anthropic", Name: "claude-3-opus", MaxContext: 200000, InputPrice: 0.015, OutputPrice: 0.075, SupportsTools: true},
	{Provider: "anthropic", Name: "claude-3-sonnet", MaxContext: 200000, InputPrice: 0.003, OutputPrice: 0.015, SupportsTools: true},
	{Provider: "anthropic", Name: "claude-3-haiku", MaxContext: 200000, InputPrice: 0.00025, OutputPrice: 0.00125, SupportsTools: true},

	// 通义千问
	{Provider: "qwen", Name: "qwen-turbo", MaxContext: 8000, InputPrice: 0.001, OutputPrice: 0.002, SupportsTools: true},
	{Provider: "qwen", Name: "qwen-plus", MaxContext: 32000, InputPrice: 0.004, OutputPrice: 0.012, SupportsTools: true},
	{Provider: "qwen", Name: "qwen-max", MaxContext: 32000, InputPrice: 0.02, OutputPrice: 0.06, SupportsTools: true},
	{Provider: "qwen", Name: "qwen-long", MaxContext: 1000000, InputPrice: 0.0005, OutputPrice: 0.002, SupportsTools: true},

	// 智谱
	{Provider: "zhipu", Name: "glm-4", MaxContext: 128000, InputPrice: 0.01, OutputPrice: 0.01, SupportsTools: true},
	{Provider: "zhipu", Name: "glm-4v", MaxContext: 8000, InputPrice: 0.01, OutputPrice: 0.01, SupportsTools: false},

	// Moonshot
	{Provider: "moonshot", Name: "moonshot-v1-8k", MaxContext: 8000, InputPrice: 0.012, OutputPrice: 0.012, SupportsTools: true},
	{Provider: "moonshot", Name: "moonshot-v1-32k", MaxContext: 32000, InputPrice: 0.024, OutputPrice: 0.024, SupportsTools: true},
	{Provider: "moonshot", Name: "moonshot-v1-128k", MaxContext: 128000, InputPrice: 0.06, OutputPrice: 0.06, SupportsTools: true},

	// DeepSeek
	{Provider: "deepseek", Name: "deepseek-chat", MaxContext: 64000, InputPrice: 0.001, OutputPrice: 0.002, SupportsTools: true},
	{Provider: "deepseek", Name: "deepseek-coder", MaxContext: 64000, InputPrice: 0.001, OutputPrice: 0.002, SupportsTools: true},

	// 百川
	{Provider: "baichuan", Name: "baichuan-turbo", MaxContext: 32000, InputPrice: 0.008, OutputPrice: 0.008, SupportsTools: true},

	// 文心一言
	{Provider: "ernie", Name: "ernie-4.0", MaxContext: 8000, InputPrice: 0.12, OutputPrice: 0.12, SupportsTools: true},
	{Provider: "ernie", Name: "ernie-3.5", MaxContext: 8000, InputPrice: 0.008, OutputPrice: 0.008, SupportsTools: true},

	// 本地模型
	{Provider: "ollama", Name: "llama3", MaxContext: 8000, InputPrice: 0, OutputPrice: 0, SupportsTools: false},
	{Provider: "ollama", Name: "codellama", MaxContext: 16000, InputPrice: 0, OutputPrice: 0, SupportsTools: false},
	{Provider: "ollama", Name: "qwen2", MaxContext: 32000, InputPrice: 0, OutputPrice: 0, SupportsTools: false},
}

// InitDefaultModels 初始化默认模型
func (r *ModelRegistry) InitDefaultModels() {
	for _, info := range DefaultModels {
		info.Available = true
		r.RegisterModel(info)
	}
}
