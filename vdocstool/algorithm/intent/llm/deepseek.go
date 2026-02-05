// Package llm DeepSeek 客户端
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

// DeepSeekClient DeepSeek 客户端
type DeepSeekClient struct {
	config     *types.LLMEngineConfig
	httpClient *http.Client
	baseURL    string
}

// NewDeepSeekClient 创建 DeepSeek 客户端
func NewDeepSeekClient(cfg *types.LLMEngineConfig) (*DeepSeekClient, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &DeepSeekClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
	}, nil
}

// DeepSeek API 请求/响应结构 (兼容 OpenAI 格式)
type deepseekRequest struct {
	Model       string             `json:"model"`
	Messages    []deepseekMessage  `json:"messages"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Stop        []string           `json:"stop,omitempty"`
}

type deepseekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepseekResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int             `json:"index"`
		Message      deepseekMessage `json:"message"`
		FinishReason string          `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Complete 完成请求
func (c *DeepSeekClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// 构建请求
	messages := make([]deepseekMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = deepseekMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	model := c.config.Model
	if model == "" {
		model = "deepseek-chat" // 默认使用 deepseek-chat
	}

	dsReq := deepseekRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stop:        req.Stop,
	}

	body, err := json.Marshal(dsReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", 
		c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

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
	var dsResp deepseekResponse
	if err := json.Unmarshal(respBody, &dsResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 检查错误
	if dsResp.Error != nil {
		return nil, fmt.Errorf("deepseek error: %s - %s", dsResp.Error.Code, dsResp.Error.Message)
	}

	if len(dsResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &CompletionResponse{
		Text:         dsResp.Choices[0].Message.Content,
		FinishReason: dsResp.Choices[0].FinishReason,
		Usage: &Usage{
			PromptTokens:     dsResp.Usage.PromptTokens,
			CompletionTokens: dsResp.Usage.CompletionTokens,
			TotalTokens:      dsResp.Usage.TotalTokens,
		},
	}, nil
}

// HealthCheck 健康检查
func (c *DeepSeekClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := c.Complete(ctx, &CompletionRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
		MaxTokens:   5,
		Temperature: 0,
	})

	return err
}

// Close 关闭
func (c *DeepSeekClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
