// Package llm Claude客户端
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// ClaudeClient Claude客户端
type ClaudeClient struct {
	config     *types.LLMEngineConfig
	httpClient *http.Client
	baseURL    string
}

// NewClaudeClient 创建Claude客户端
func NewClaudeClient(cfg *types.LLMEngineConfig) (*ClaudeClient, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	return &ClaudeClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL: baseURL,
	}, nil
}

// Claude API 请求/响应结构
type claudeRequest struct {
	Model       string           `json:"model"`
	MaxTokens   int              `json:"max_tokens"`
	System      string           `json:"system,omitempty"`
	Messages    []claudeMessage  `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
	StopSequences []string       `json:"stop_sequences,omitempty"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	Content      []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence,omitempty"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete 完成请求
func (c *ClaudeClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// 提取系统消息和用户消息
	var system string
	var messages []claudeMessage

	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
		} else {
			messages = append(messages, claudeMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	// 构建请求
	claudeReq := claudeRequest{
		Model:       c.config.Model,
		MaxTokens:   req.MaxTokens,
		System:      system,
		Messages:    messages,
		Temperature: req.Temperature,
		StopSequences: req.Stop,
	}

	// 序列化请求
	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.config.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// 发送请求
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 解析响应
	var claudeResp claudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 检查错误
	if claudeResp.Error != nil {
		return nil, fmt.Errorf("claude error: %s", claudeResp.Error.Message)
	}

	// 提取文本
	var text string
	for _, content := range claudeResp.Content {
		if content.Type == "text" {
			text += content.Text
		}
	}

	return &CompletionResponse{
		Text:         text,
		FinishReason: claudeResp.StopReason,
		Usage: &Usage{
			PromptTokens:     claudeResp.Usage.InputTokens,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		},
	}, nil
}

// HealthCheck 健康检查
func (c *ClaudeClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.Complete(ctx, &CompletionRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "ping"},
		},
		MaxTokens:   5,
		Temperature: 0,
	})

	return err
}

// Close 关闭
func (c *ClaudeClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
