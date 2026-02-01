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

	"github.com/cloudwego/eino/components/model"

	"github.com/cloudwego/eino/vdocsMap/config"
)

// Provider LLM 提供商接口
type Provider interface {
	// GetChatModel 获取聊天模型
	GetChatModel(ctx context.Context) (model.ToolCallingChatModel, error)
	
	// Name 提供商名称
	Name() string
	
	// Close 关闭连接
	Close() error
}

// NewProvider 根据配置创建 LLM 提供商
func NewProvider(cfg *config.LLMConfig) (Provider, error) {
	switch cfg.Provider {
	case "openai":
		return NewOpenAIProvider(cfg)
	case "ark":
		return NewArkProvider(cfg)
	case "qwen":
		return NewQwenProvider(cfg)
	case "deepseek":
		return NewDeepSeekProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}

// NewChatModel 创建聊天模型的便捷方法
func NewChatModel(ctx context.Context, cfg *config.LLMConfig) (model.ToolCallingChatModel, error) {
	provider, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}
	
	return provider.GetChatModel(ctx)
}
