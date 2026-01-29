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

// Package llm provides LLM implementations for the kernel expert agent.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// DeepSeekConfig configures the DeepSeek model.
type DeepSeekConfig struct {
	APIKey      string
	BaseURL     string
	Model       string
	Temperature float64
	MaxTokens   int
	Timeout     time.Duration
}

// DeepSeekModel implements ToolCallingChatModel for DeepSeek API.
type DeepSeekModel struct {
	config     *DeepSeekConfig
	httpClient *http.Client
	tools      []*schema.ToolInfo
}

// NewDeepSeekModel creates a new DeepSeek model instance.
func NewDeepSeekModel(config *DeepSeekConfig) (*DeepSeekModel, error) {
	if config.APIKey == "" {
		config.APIKey = os.Getenv("DEEPSEEK_API_KEY")
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("DeepSeek API key not provided")
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api.deepseek.com/v1"
	}
	if config.Model == "" {
		config.Model = "deepseek-chat"
	}
	if config.Temperature == 0 {
		config.Temperature = 0.1
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 4096
	}
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}

	return &DeepSeekModel{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// chatRequest represents the OpenAI-compatible request format.
type chatRequest struct {
	Model       string          `json:"model"`
	Messages    []chatMessage   `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Tools       []chatTool      `json:"tools,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type chatMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  []chatToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type chatToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function chatFuncCall `json:"function"`
}

type chatFuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Generate implements BaseChatModel.Generate.
func (m *DeepSeekModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	req := m.buildRequest(input)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", m.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.config.APIKey)

	resp, err := m.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return m.convertResponse(&chatResp.Choices[0].Message), nil
}

// Stream implements BaseChatModel.Stream.
func (m *DeepSeekModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	// For simplicity, we'll use non-streaming and convert to stream format
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}

	reader, writer := schema.Pipe[*schema.Message](1)
	go func() {
		defer writer.Close()
		writer.Send(msg, nil)
	}()

	return reader, nil
}

// WithTools implements ToolCallingChatModel.WithTools.
func (m *DeepSeekModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	newModel := &DeepSeekModel{
		config:     m.config,
		httpClient: m.httpClient,
		tools:      tools,
	}
	return newModel, nil
}

func (m *DeepSeekModel) buildRequest(input []*schema.Message) *chatRequest {
	req := &chatRequest{
		Model:       m.config.Model,
		Temperature: m.config.Temperature,
		MaxTokens:   m.config.MaxTokens,
	}

	// Convert messages
	for _, msg := range input {
		chatMsg := chatMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}

		// Handle tool calls
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				chatMsg.ToolCalls = append(chatMsg.ToolCalls, chatToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: chatFuncCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
		}

		// Handle tool response
		if msg.Role == schema.Tool {
			chatMsg.ToolCallID = msg.ToolCallID
		}

		req.Messages = append(req.Messages, chatMsg)
	}

	// Convert tools
	for _, tool := range m.tools {
		var params json.RawMessage
		if tool.ParamsOneOf != nil {
			// Convert ParamsOneOf to JSONSchema format
			jsonSchema, err := tool.ParamsOneOf.ToJSONSchema()
			if err == nil && jsonSchema != nil {
				params, _ = json.Marshal(jsonSchema)
			}
		}
		// If no params, create empty object schema
		if params == nil {
			params = json.RawMessage(`{"type": "object", "properties": {}}`)
		}
		req.Tools = append(req.Tools, chatTool{
			Type: "function",
			Function: chatFunction{
				Name:        tool.Name,
				Description: tool.Desc,
				Parameters:  params,
			},
		})
	}

	return req
}

func (m *DeepSeekModel) convertResponse(msg *chatMessage) *schema.Message {
	result := &schema.Message{
		Role:    schema.RoleType(msg.Role),
		Content: msg.Content,
	}

	// Convert tool calls
	for _, tc := range msg.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, schema.ToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: schema.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return result
}
