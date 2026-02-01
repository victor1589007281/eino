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
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/model"

	"github.com/cloudwego/eino/vdocsMap/config"
)

// ArkProvider 字节跳动火山引擎 ARK 提供商
type ArkProvider struct {
	config *config.LLMConfig
	client *http.Client
}

// NewArkProvider 创建 ARK 提供商
func NewArkProvider(cfg *config.LLMConfig) (*ArkProvider, error) {
	// 获取 ARK 特定配置
	if arkCfg, ok := cfg.Providers["ark"]; ok && arkCfg.Enabled {
		cfg.APIKey = arkCfg.APIKey
		cfg.BaseURL = arkCfg.BaseURL
		cfg.Model = arkCfg.Model
	}

	return &ArkProvider{
		config: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}, nil
}

// GetChatModel 获取聊天模型
func (p *ArkProvider) GetChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	// ARK API 兼容 OpenAI 格式
	openaiProvider := &OpenAIProvider{
		config: p.config,
		client: p.client,
	}
	return openaiProvider.GetChatModel(ctx)
}

// Name 提供商名称
func (p *ArkProvider) Name() string {
	return "ark"
}

// Close 关闭连接
func (p *ArkProvider) Close() error {
	return nil
}

// QwenProvider 阿里云通义千问提供商
type QwenProvider struct {
	config *config.LLMConfig
	client *http.Client
}

// NewQwenProvider 创建通义千问提供商
func NewQwenProvider(cfg *config.LLMConfig) (*QwenProvider, error) {
	// 获取 Qwen 特定配置
	if qwenCfg, ok := cfg.Providers["qwen"]; ok && qwenCfg.Enabled {
		cfg.APIKey = qwenCfg.APIKey
		cfg.BaseURL = qwenCfg.BaseURL
		cfg.Model = qwenCfg.Model
	}

	return &QwenProvider{
		config: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}, nil
}

// GetChatModel 获取聊天模型
func (p *QwenProvider) GetChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	// 通义千问 API 兼容 OpenAI 格式
	openaiProvider := &OpenAIProvider{
		config: p.config,
		client: p.client,
	}
	return openaiProvider.GetChatModel(ctx)
}

// Name 提供商名称
func (p *QwenProvider) Name() string {
	return "qwen"
}

// Close 关闭连接
func (p *QwenProvider) Close() error {
	return nil
}

// DeepSeekProvider DeepSeek 提供商
type DeepSeekProvider struct {
	config *config.LLMConfig
	client *http.Client
}

// NewDeepSeekProvider 创建 DeepSeek 提供商
func NewDeepSeekProvider(cfg *config.LLMConfig) (*DeepSeekProvider, error) {
	// 获取 DeepSeek 特定配置
	if dsCfg, ok := cfg.Providers["deepseek"]; ok && dsCfg.Enabled {
		cfg.APIKey = dsCfg.APIKey
		cfg.BaseURL = dsCfg.BaseURL
		cfg.Model = dsCfg.Model
	}

	return &DeepSeekProvider{
		config: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}, nil
}

// GetChatModel 获取聊天模型
func (p *DeepSeekProvider) GetChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	// DeepSeek API 兼容 OpenAI 格式
	openaiProvider := &OpenAIProvider{
		config: p.config,
		client: p.client,
	}
	return openaiProvider.GetChatModel(ctx)
}

// Name 提供商名称
func (p *DeepSeekProvider) Name() string {
	return "deepseek"
}

// Close 关闭连接
func (p *DeepSeekProvider) Close() error {
	return nil
}
