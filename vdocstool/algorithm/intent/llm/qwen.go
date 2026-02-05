// Package llm 通义千问客户端 (阿里云)
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

// QwenClient 通义千问客户端
type QwenClient struct {
	config     *types.LLMEngineConfig
	httpClient *http.Client
	baseURL    string
}

// NewQwenClient 创建通义千问客户端
func NewQwenClient(cfg *types.LLMEngineConfig) (*QwenClient, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/api/v1"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &QwenClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
	}, nil
}

// 通义千问 API 请求/响应结构
type qwenRequest struct {
	Model      string         `json:"model"`
	Input      qwenInput      `json:"input"`
	Parameters qwenParameters `json:"parameters,omitempty"`
}

type qwenInput struct {
	Messages []qwenMessage `json:"messages"`
}

type qwenMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type qwenParameters struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxTokens       int     `json:"max_tokens,omitempty"`
	ResultFormat    string  `json:"result_format,omitempty"`
	EnableSearch    bool    `json:"enable_search,omitempty"`
}

type qwenResponse struct {
	Output struct {
		Text         string `json:"text"`
		FinishReason string `json:"finish_reason"`
		Choices      []struct {
			FinishReason string      `json:"finish_reason"`
			Message      qwenMessage `json:"message"`
		} `json:"choices"`
	} `json:"output"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
		InputTokens  int `json:"input_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
}

// Complete 完成请求
func (c *QwenClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// 构建请求
	messages := make([]qwenMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = qwenMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	model := c.config.Model
	if model == "" {
		model = "qwen-turbo" // 默认使用 qwen-turbo
	}

	qwenReq := qwenRequest{
		Model: model,
		Input: qwenInput{
			Messages: messages,
		},
		Parameters: qwenParameters{
			Temperature:  req.Temperature,
			MaxTokens:    req.MaxTokens,
			ResultFormat: "message",
		},
	}

	body, err := json.Marshal(qwenReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", 
		c.baseURL+"/services/aigc/text-generation/generation", bytes.NewReader(body))
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
	var qwenResp qwenResponse
	if err := json.Unmarshal(respBody, &qwenResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 检查错误
	if qwenResp.Code != "" {
		return nil, fmt.Errorf("qwen error: %s - %s", qwenResp.Code, qwenResp.Message)
	}

	// 提取文本
	var text string
	if len(qwenResp.Output.Choices) > 0 {
		text = qwenResp.Output.Choices[0].Message.Content
	} else {
		text = qwenResp.Output.Text
	}

	return &CompletionResponse{
		Text:         text,
		FinishReason: qwenResp.Output.FinishReason,
		Usage: &Usage{
			PromptTokens:     qwenResp.Usage.InputTokens,
			CompletionTokens: qwenResp.Usage.OutputTokens,
			TotalTokens:      qwenResp.Usage.TotalTokens,
		},
	}, nil
}

// HealthCheck 健康检查
func (c *QwenClient) HealthCheck(ctx context.Context) error {
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
func (c *QwenClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
