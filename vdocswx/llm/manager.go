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

package llm

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/vdocswx/config"
)

// Manager LLM管理器
type Manager struct {
	config    *config.LLMConfig
	providers map[string]model.ChatModel
	router    *Router
	mu        sync.RWMutex
}

// NewManager 创建LLM管理器
func NewManager(cfg *config.LLMConfig, routerCfg *config.RouterConfig) (*Manager, error) {
	m := &Manager{
		config:    cfg,
		providers: make(map[string]model.ChatModel),
	}

	// 初始化各个provider
	if err := m.initProviders(); err != nil {
		return nil, err
	}

	// 初始化路由器
	m.router = NewRouter(routerCfg, m.providers)

	return m, nil
}

// initProviders 初始化所有启用的provider
func (m *Manager) initProviders() error {
	// DeepSeek
	if m.config.DeepSeek.Enabled {
		provider, err := NewDeepSeekProvider(&m.config.DeepSeek)
		if err != nil {
			return fmt.Errorf("failed to init deepseek: %w", err)
		}
		m.providers["deepseek"] = provider
	}

	// Qwen
	if m.config.Qwen.Enabled {
		provider, err := NewQwenProvider(&m.config.Qwen)
		if err != nil {
			return fmt.Errorf("failed to init qwen: %w", err)
		}
		m.providers["qwen"] = provider
	}

	// OpenAI
	if m.config.OpenAI.Enabled {
		provider, err := NewOpenAIProvider(&m.config.OpenAI)
		if err != nil {
			return fmt.Errorf("failed to init openai: %w", err)
		}
		m.providers["openai"] = provider
	}

	// Anthropic
	if m.config.Anthropic.Enabled {
		provider, err := NewAnthropicProvider(&m.config.Anthropic)
		if err != nil {
			return fmt.Errorf("failed to init anthropic: %w", err)
		}
		m.providers["anthropic"] = provider
	}

	// Ollama
	if m.config.Ollama.Enabled {
		provider, err := NewOllamaProvider(&m.config.Ollama)
		if err != nil {
			return fmt.Errorf("failed to init ollama: %w", err)
		}
		m.providers["ollama"] = provider
	}

	// GLM
	if m.config.GLM.Enabled {
		provider, err := NewGLMProvider(&m.config.GLM)
		if err != nil {
			return fmt.Errorf("failed to init glm: %w", err)
		}
		m.providers["glm"] = provider
	}

	// Moonshot
	if m.config.Moonshot.Enabled {
		provider, err := NewMoonshotProvider(&m.config.Moonshot)
		if err != nil {
			return fmt.Errorf("failed to init moonshot: %w", err)
		}
		m.providers["moonshot"] = provider
	}

	return nil
}

// GetProvider 获取指定的provider
func (m *Manager) GetProvider(name string) (model.ChatModel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, ok := m.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return provider, nil
}

// Route 路由到合适的模型
func (m *Manager) Route(ctx context.Context, task *Task) (model.ChatModel, error) {
	return m.router.Route(ctx, task)
}

// ListProviders 列出所有可用的provider
func (m *Manager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make([]string, 0, len(m.providers))
	for name := range m.providers {
		providers = append(providers, name)
	}
	return providers
}

// Close 关闭所有provider
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, provider := range m.providers {
		if closer, ok := provider.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				return fmt.Errorf("failed to close provider %s: %w", name, err)
			}
		}
	}
	return nil
}

// Task 任务信息
type Task struct {
	Type       TaskType
	Content    string
	TokenCount int
	Complexity int
	Sensitive  bool
}

// TaskType 任务类型
type TaskType string

const (
	TaskTypeSimple  TaskType = "simple"  // 简单任务
	TaskTypeMedium  TaskType = "medium"  // 中等任务
	TaskTypeComplex TaskType = "complex" // 复杂任务
)
