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

package provider

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/model/ollama"
	"github.com/cloudwego/eino/vdocswx/config"
)

// OllamaProvider Ollama模型提供者
type OllamaProvider struct {
	chatModel model.ChatModel
	config    *config.OllamaConfig
}

// NewOllamaProvider 创建Ollama提供者
func NewOllamaProvider(cfg *config.OllamaConfig) (*OllamaProvider, error) {
	chatModel, err := ollama.NewChatModel(context.Background(), &ollama.ChatModelConfig{
		BaseURL:     cfg.BaseURL,
		Model:       cfg.Model,
		MaxTokens:   &cfg.MaxTokens,
		Temperature: &cfg.Temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama chat model: %w", err)
	}

	return &OllamaProvider{
		chatModel: chatModel,
		config:    cfg,
	}, nil
}

// Generate 生成回复
func (p *OllamaProvider) Generate(ctx context.Context, input []*model.Message, opts ...model.Option) (*model.Message, error) {
	return p.chatModel.Generate(ctx, input, opts...)
}

// Stream 流式生成回复
func (p *OllamaProvider) Stream(ctx context.Context, input []*model.Message, opts ...model.Option) (*model.StreamReader[*model.Message], error) {
	return p.chatModel.Stream(ctx, input, opts...)
}

// BindTools 绑定工具
func (p *OllamaProvider) BindTools(tools []*model.ToolInfo) error {
	if binder, ok := p.chatModel.(model.ToolCallingChatModel); ok {
		return binder.BindTools(tools)
	}
	return fmt.Errorf("Ollama model does not support tool binding")
}
