// Package llm Cohere 客户端
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

// CohereClient Cohere 客户端
type CohereClient struct {
	config     *types.LLMEngineConfig
	httpClient *http.Client
	baseURL    string
}

// NewCohereClient 创建 Cohere 客户端
func NewCohereClient(cfg *types.LLMEngineConfig) (*CohereClient, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.cohere.ai/v1"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &CohereClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
	}, nil
}

// Cohere API 请求/响应结构
type cohereRequest struct {
	Model          string           `json:"model,omitempty"`
	Message        string           `json:"message"`
	ChatHistory    []cohereMessage  `json:"chat_history,omitempty"`
	Preamble       string           `json:"preamble,omitempty"`
	Temperature    float64          `json:"temperature,omitempty"`
	MaxTokens      int              `json:"max_tokens,omitempty"`
	StopSequences  []string         `json:"stop_sequences,omitempty"`
}

type cohereMessage struct {
	Role    string `json:"role"` // USER, CHATBOT, SYSTEM
	Message string `json:"message"`
}

type cohereResponse struct {
	ResponseID     string `json:"response_id"`
	Text           string `json:"text"`
	GenerationID   string `json:"generation_id"`
	FinishReason   string `json:"finish_reason"`
	ChatHistory    []cohereMessage `json:"chat_history"`
	Meta           struct {
		Tokens struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"tokens"`
		BilledUnits struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"billed_units"`
	} `json:"meta"`
	Message string `json:"message,omitempty"` // error message
}

// Complete 完成请求
func (c *CohereClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// 构建请求
	var chatHistory []cohereMessage
	var preamble string
	var lastMessage string

	for i, m := range req.Messages {
		if m.Role == "system" {
			preamble = m.Content
			continue
		}

		if i == len(req.Messages)-1 && m.Role == "user" {
			lastMessage = m.Content
			continue
		}

		role := "USER"
		if m.Role == "assistant" {
			role = "CHATBOT"
		}

		chatHistory = append(chatHistory, cohereMessage{
			Role:    role,
			Message: m.Content,
		})
	}

	if lastMessage == "" && len(chatHistory) > 0 {
		// 没有最后的用户消息，取最后一个 USER 消息
		for i := len(chatHistory) - 1; i >= 0; i-- {
			if chatHistory[i].Role == "USER" {
				lastMessage = chatHistory[i].Message
				chatHistory = append(chatHistory[:i], chatHistory[i+1:]...)
				break
			}
		}
	}

	model := c.config.Model
	if model == "" {
		model = "command-r-plus"
	}

	cohereReq := cohereRequest{
		Model:         model,
		Message:       lastMessage,
		ChatHistory:   chatHistory,
		Preamble:      preamble,
		Temperature:   req.Temperature,
		MaxTokens:     req.MaxTokens,
		StopSequences: req.Stop,
	}

	body, err := json.Marshal(cohereReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", 
		c.baseURL+"/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	httpReq.Header.Set("Accept", "application/json")

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
	var cohereResp cohereResponse
	if err := json.Unmarshal(respBody, &cohereResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 检查错误
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("cohere error: %s", cohereResp.Message)
	}

	return &CompletionResponse{
		Text:         cohereResp.Text,
		FinishReason: cohereResp.FinishReason,
		Usage: &Usage{
			PromptTokens:     cohereResp.Meta.Tokens.InputTokens,
			CompletionTokens: cohereResp.Meta.Tokens.OutputTokens,
			TotalTokens:      cohereResp.Meta.Tokens.InputTokens + cohereResp.Meta.Tokens.OutputTokens,
		},
	}, nil
}

// HealthCheck 健康检查
func (c *CohereClient) HealthCheck(ctx context.Context) error {
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
func (c *CohereClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
