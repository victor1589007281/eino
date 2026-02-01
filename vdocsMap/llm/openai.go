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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino/vdocsMap/config"
)

// OpenAIProvider OpenAI 提供商实现
type OpenAIProvider struct {
	config *config.LLMConfig
	client *http.Client
}

// NewOpenAIProvider 创建 OpenAI 提供商
func NewOpenAIProvider(cfg *config.LLMConfig) (*OpenAIProvider, error) {
	return &OpenAIProvider{
		config: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}, nil
}

// GetChatModel 获取聊天模型
func (p *OpenAIProvider) GetChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	return &OpenAIChatModel{
		provider: p,
	}, nil
}

// Name 提供商名称
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Close 关闭连接
func (p *OpenAIProvider) Close() error {
	return nil
}

// OpenAIChatModel OpenAI 聊天模型实现
type OpenAIChatModel struct {
	provider  *OpenAIProvider
	toolInfos []*schema.ToolInfo
}

// Generate 生成响应
func (m *OpenAIChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	// 构建请求
	messages := make([]map[string]any, 0, len(input))
	for _, msg := range input {
		messages = append(messages, m.convertMessage(msg))
	}

	reqBody := map[string]any{
		"model":       m.provider.config.Model,
		"messages":    messages,
		"max_tokens":  m.provider.config.MaxTokens,
		"temperature": m.provider.config.Temperature,
	}

	// 添加工具定义
	if len(m.toolInfos) > 0 {
		tools := make([]map[string]any, 0, len(m.toolInfos))
		for _, ti := range m.toolInfos {
			tools = append(tools, m.convertToolInfo(ti))
		}
		reqBody["tools"] = tools
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", m.provider.config.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.provider.config.APIKey)

	resp, err := m.provider.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var apiResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return m.convertResponseMessage(&apiResp.Choices[0].Message, &apiResp), nil
}

// Stream 流式生成响应
func (m *OpenAIChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	// 构建请求
	messages := make([]map[string]any, 0, len(input))
	for _, msg := range input {
		messages = append(messages, m.convertMessage(msg))
	}

	reqBody := map[string]any{
		"model":       m.provider.config.Model,
		"messages":    messages,
		"max_tokens":  m.provider.config.MaxTokens,
		"temperature": m.provider.config.Temperature,
		"stream":      true,
	}

	// 添加工具定义
	if len(m.toolInfos) > 0 {
		tools := make([]map[string]any, 0, len(m.toolInfos))
		for _, ti := range m.toolInfos {
			tools = append(tools, m.convertToolInfo(ti))
		}
		reqBody["tools"] = tools
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", m.provider.config.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.provider.config.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := m.provider.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	// 创建流读取器
	return m.createStreamReader(resp.Body), nil
}

// WithTools 设置工具
func (m *OpenAIChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	newModel := &OpenAIChatModel{
		provider:  m.provider,
		toolInfos: tools,
	}
	return newModel, nil
}

// BindTools 绑定工具（兼容接口）
func (m *OpenAIChatModel) BindTools(tools []*schema.ToolInfo) error {
	m.toolInfos = tools
	return nil
}

// convertMessage 转换消息格式
func (m *OpenAIChatModel) convertMessage(msg *schema.Message) map[string]any {
	result := map[string]any{
		"role": string(msg.Role),
	}

	if msg.Content != "" {
		result["content"] = msg.Content
	}

	if len(msg.ToolCalls) > 0 {
		toolCalls := make([]map[string]any, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			toolCalls = append(toolCalls, map[string]any{
				"id":   tc.ID,
				"type": "function",
				"function": map[string]any{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			})
		}
		result["tool_calls"] = toolCalls
	}

	if msg.ToolCallID != "" {
		result["tool_call_id"] = msg.ToolCallID
	}

	if msg.Name != "" {
		result["name"] = msg.Name
	}

	return result
}

// convertToolInfo 转换工具信息
func (m *OpenAIChatModel) convertToolInfo(ti *schema.ToolInfo) map[string]any {
	properties := make(map[string]any)
	required := make([]string, 0)

	for name, param := range ti.Parameters {
		properties[name] = map[string]any{
			"type":        string(param.Type),
			"description": param.Description,
		}
		if param.Required {
			required = append(required, name)
		}
	}

	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        ti.Name,
			"description": ti.Description,
			"parameters": map[string]any{
				"type":       "object",
				"properties": properties,
				"required":   required,
			},
		},
	}
}

// convertResponseMessage 转换响应消息
func (m *OpenAIChatModel) convertResponseMessage(respMsg *OpenAIMessage, fullResp *OpenAIResponse) *schema.Message {
	msg := &schema.Message{
		Role:    schema.RoleType(respMsg.Role),
		Content: respMsg.Content,
	}

	// 处理工具调用
	if len(respMsg.ToolCalls) > 0 {
		msg.ToolCalls = make([]schema.ToolCall, 0, len(respMsg.ToolCalls))
		for _, tc := range respMsg.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	// 添加响应元数据
	if fullResp != nil {
		msg.ResponseMeta = &schema.ResponseMeta{
			FinishReason: fullResp.Choices[0].FinishReason,
		}
		if fullResp.Usage != nil {
			msg.ResponseMeta.Usage = &schema.TokenUsage{
				PromptTokens:     fullResp.Usage.PromptTokens,
				CompletionTokens: fullResp.Usage.CompletionTokens,
				TotalTokens:      fullResp.Usage.TotalTokens,
			}
		}
	}

	return msg
}

// createStreamReader 创建流读取器
func (m *OpenAIChatModel) createStreamReader(body io.ReadCloser) *schema.StreamReader[*schema.Message] {
	reader, writer := schema.Pipe[*schema.Message](10)

	go func() {
		defer body.Close()
		defer writer.Close()

		decoder := json.NewDecoder(body)
		for {
			var chunk OpenAIStreamChunk
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					break
				}
				writer.Send(nil, err)
				return
			}

			if len(chunk.Choices) > 0 {
				delta := &chunk.Choices[0].Delta
				msg := &schema.Message{
					Role:    schema.RoleType(delta.Role),
					Content: delta.Content,
				}

				if len(delta.ToolCalls) > 0 {
					msg.ToolCalls = make([]schema.ToolCall, 0, len(delta.ToolCalls))
					for _, tc := range delta.ToolCalls {
						idx := tc.Index
						msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
							Index: &idx,
							ID:    tc.ID,
							Type:  tc.Type,
							Function: schema.FunctionCall{
								Name:      tc.Function.Name,
								Arguments: tc.Function.Arguments,
							},
						})
					}
				}

				if chunk.Choices[0].FinishReason != "" {
					msg.ResponseMeta = &schema.ResponseMeta{
						FinishReason: chunk.Choices[0].FinishReason,
					}
				}

				writer.Send(msg, nil)
			}
		}
	}()

	return reader
}

// OpenAI API 数据结构

// OpenAIResponse OpenAI API 响应
type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

// OpenAIChoice OpenAI 选择
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIMessage OpenAI 消息
type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// OpenAIToolCall OpenAI 工具调用
type OpenAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// OpenAIUsage OpenAI 使用量
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIStreamChunk OpenAI 流式响应块
type OpenAIStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string `json:"role,omitempty"`
			Content   string `json:"content,omitempty"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id,omitempty"`
				Type     string `json:"type,omitempty"`
				Function struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}
