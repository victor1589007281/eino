// Package llm 百川客户端
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

// BaichuanClient 百川客户端
type BaichuanClient struct {
	config     *types.LLMEngineConfig
	httpClient *http.Client
	baseURL    string
}

// NewBaichuanClient 创建百川客户端
func NewBaichuanClient(cfg *types.LLMEngineConfig) (*BaichuanClient, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.baichuan-ai.com/v1"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &BaichuanClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
	}, nil
}

// 百川 API 请求/响应结构 (兼容 OpenAI 格式)
type baichuanRequest struct {
	Model       string             `json:"model"`
	Messages    []baichuanMessage  `json:"messages"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
	TopK        int                `json:"top_k,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type baichuanMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type baichuanResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int             `json:"index"`
		Message      baichuanMessage `json:"message"`
		FinishReason string          `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

// Complete 完成请求
func (c *BaichuanClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// 构建请求
	messages := make([]baichuanMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = baichuanMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	model := c.config.Model
	if model == "" {
		model = "Baichuan2-Turbo" // 默认使用 Baichuan2-Turbo
	}

	bcReq := baichuanRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	body, err := json.Marshal(bcReq)
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
	var bcResp baichuanResponse
	if err := json.Unmarshal(respBody, &bcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 检查错误
	if bcResp.Error != nil {
		return nil, fmt.Errorf("baichuan error: %d - %s", bcResp.Error.Code, bcResp.Error.Message)
	}

	if len(bcResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &CompletionResponse{
		Text:         bcResp.Choices[0].Message.Content,
		FinishReason: bcResp.Choices[0].FinishReason,
		Usage: &Usage{
			PromptTokens:     bcResp.Usage.PromptTokens,
			CompletionTokens: bcResp.Usage.CompletionTokens,
			TotalTokens:      bcResp.Usage.TotalTokens,
		},
	}, nil
}

// HealthCheck 健康检查
func (c *BaichuanClient) HealthCheck(ctx context.Context) error {
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
func (c *BaichuanClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
