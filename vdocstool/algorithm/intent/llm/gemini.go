// Package llm Google Gemini 客户端
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

// GeminiClient Google Gemini 客户端
type GeminiClient struct {
	config     *types.LLMEngineConfig
	httpClient *http.Client
	baseURL    string
}

// NewGeminiClient 创建 Gemini 客户端
func NewGeminiClient(cfg *types.LLMEngineConfig) (*GeminiClient, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &GeminiClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
	}, nil
}

// Gemini API 请求/响应结构
type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
	SafetySettings   []geminiSafetySetting  `json:"safetySettings,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature     float64  `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	TopP            float64  `json:"topP,omitempty"`
	TopK            int      `json:"topK,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// Complete 完成请求
func (c *GeminiClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// 构建请求
	var contents []geminiContent
	var systemInstruction string
	
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemInstruction = m.Content
			continue
		}
		
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		
		contents = append(contents, geminiContent{
			Role: role,
			Parts: []geminiPart{
				{Text: m.Content},
			},
		})
	}

	// 如果有系统指令，添加到第一条用户消息前
	if systemInstruction != "" && len(contents) > 0 {
		contents[0].Parts[0].Text = systemInstruction + "\n\n" + contents[0].Parts[0].Text
	}

	geminiReq := geminiRequest{
		Contents: contents,
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
			StopSequences:   req.Stop,
		},
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 获取模型
	model := c.config.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	// 创建 HTTP 请求
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, model, c.config.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

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
	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 检查错误
	if geminiResp.Error != nil {
		return nil, fmt.Errorf("gemini error: %d - %s", geminiResp.Error.Code, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	// 提取文本
	var text string
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		text += part.Text
	}

	return &CompletionResponse{
		Text:         text,
		FinishReason: geminiResp.Candidates[0].FinishReason,
		Usage: &Usage{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

// HealthCheck 健康检查
func (c *GeminiClient) HealthCheck(ctx context.Context) error {
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
func (c *GeminiClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
