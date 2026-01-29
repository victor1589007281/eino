// Package llm 提供LLM管理功能
package llm

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/vdocswx/config"
)

// LLMManager LLM管理器
type LLMManager struct {
	cfg          *config.LLMConfig
	deepseek     *DeepSeekClient
	defaultModel string
	mu           sync.RWMutex
}

// NewLLMManager 创建LLM管理器
func NewLLMManager(cfg *config.LLMConfig) (*LLMManager, error) {
	mgr := &LLMManager{
		cfg:          cfg,
		defaultModel: cfg.DefaultModel,
	}

	// 初始化DeepSeek客户端
	if apiKey, ok := cfg.APIKeys["deepseek"]; ok && apiKey != "" {
		model := cfg.DefaultModel
		if model == "" {
			model = "deepseek-chat"
		}
		mgr.deepseek = NewDeepSeekClient(apiKey, model)
	}

	if mgr.deepseek == nil {
		return nil, fmt.Errorf("no LLM provider configured, please set DEEPSEEK_API_KEY or configure api_keys in config")
	}

	return mgr, nil
}

// GenerateText 生成文本
func (m *LLMManager) GenerateText(ctx context.Context, prompt string, intentType string) (string, error) {
	m.mu.RLock()
	client := m.deepseek
	m.mu.RUnlock()

	if client == nil {
		return "", fmt.Errorf("no LLM client available")
	}

	result, _, err := client.GenerateText(ctx, prompt)
	return result, err
}

// GenerateTextWithSystem 带系统提示生成文本
func (m *LLMManager) GenerateTextWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	m.mu.RLock()
	client := m.deepseek
	m.mu.RUnlock()

	if client == nil {
		return "", fmt.Errorf("no LLM client available")
	}

	result, _, err := client.GenerateTextWithSystem(ctx, systemPrompt, userPrompt)
	return result, err
}

// GetTokenUsage 获取token使用量（简化实现）
func (m *LLMManager) GetTokenUsage() int {
	return 0 // TODO: 实现token追踪
}
