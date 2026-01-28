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
	"github.com/cloudwego/eino/components/model/openai"
	"github.com/cloudwego/eino/vdocswx/config"
)

// DeepSeekProvider DeepSeek模型提供者
type DeepSeekProvider struct {
	chatModel model.ChatModel
	config    *config.DeepSeekConfig
}

// NewDeepSeekProvider 创建DeepSeek提供者
func NewDeepSeekProvider(cfg *config.DeepSeekConfig) (*DeepSeekProvider, error) {
	apiKey := config.GetAPIKey(cfg.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("DeepSeek API key not found in env %s", cfg.APIKeyEnv)
	}

	chatModel, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL:     cfg.BaseURL,
		APIKey:      apiKey,
		Model:       cfg.Model,
		MaxTokens:   &cfg.MaxTokens,
		Temperature: &cfg.Temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create DeepSeek chat model: %w", err)
	}

	return &DeepSeekProvider{
		chatModel: chatModel,
		config:    cfg,
	}, nil
}

// Generate 生成回复
func (p *DeepSeekProvider) Generate(ctx context.Context, input []*model.Message, opts ...model.Option) (*model.Message, error) {
	return p.chatModel.Generate(ctx, input, opts...)
}

// Stream 流式生成回复
func (p *DeepSeekProvider) Stream(ctx context.Context, input []*model.Message, opts ...model.Option) (*model.StreamReader[*model.Message], error) {
	return p.chatModel.Stream(ctx, input, opts...)
}

// BindTools 绑定工具
func (p *DeepSeekProvider) BindTools(tools []*model.ToolInfo) error {
	if binder, ok := p.chatModel.(model.ToolCallingChatModel); ok {
		return binder.BindTools(tools)
	}
	return fmt.Errorf("DeepSeek model does not support tool binding")
}
