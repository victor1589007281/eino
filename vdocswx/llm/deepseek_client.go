// Package llm 提供LLM客户端实现
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DeepSeekClient DeepSeek API客户端
type DeepSeekClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// ChatRequest DeepSeek聊天请求
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse DeepSeek聊天响应
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewDeepSeekClient 创建DeepSeek客户端
func NewDeepSeekClient(apiKey, model string) *DeepSeekClient {
	return &DeepSeekClient{
		apiKey:  apiKey,
		baseURL: "https://api.deepseek.com/v1",
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Chat 发送聊天请求
func (c *DeepSeekClient) Chat(ctx context.Context, messages []ChatMessage, maxTokens int, temperature float64) (*ChatResponse, error) {
	req := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      false,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &chatResp, nil
}

// GenerateText 生成文本（便捷方法）
func (c *DeepSeekClient) GenerateText(ctx context.Context, prompt string) (string, int, error) {
	messages := []ChatMessage{
		{Role: "user", Content: prompt},
	}

	resp, err := c.Chat(ctx, messages, 4096, 0.3)
	if err != nil {
		return "", 0, err
	}

	if len(resp.Choices) == 0 {
		return "", 0, fmt.Errorf("no choices in response")
	}

	return resp.Choices[0].Message.Content, resp.Usage.TotalTokens, nil
}

// GenerateTextWithSystem 带系统提示生成文本
func (c *DeepSeekClient) GenerateTextWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, int, error) {
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := c.Chat(ctx, messages, 4096, 0.3)
	if err != nil {
		return "", 0, err
	}

	if len(resp.Choices) == 0 {
		return "", 0, fmt.Errorf("no choices in response")
	}

	return resp.Choices[0].Message.Content, resp.Usage.TotalTokens, nil
}
