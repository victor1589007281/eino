// Package llm 智谱 GLM 客户端
package llm

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// GLMClient 智谱 GLM 客户端
type GLMClient struct {
	config     *types.LLMEngineConfig
	httpClient *http.Client
	baseURL    string
}

// NewGLMClient 创建智谱 GLM 客户端
func NewGLMClient(cfg *types.LLMEngineConfig) (*GLMClient, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn/api/paas/v4"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &GLMClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
	}, nil
}

// generateToken 生成 JWT Token
func (c *GLMClient) generateToken() (string, error) {
	apiKey := c.config.APIKey
	parts := strings.Split(apiKey, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid api key format, expected 'id.secret'")
	}

	id := parts[0]
	secret := parts[1]

	// 构建 JWT Header
	header := map[string]string{
		"alg":       "HS256",
		"sign_type": "SIGN",
	}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// 构建 JWT Payload
	now := time.Now().UnixMilli()
	payload := map[string]interface{}{
		"api_key":   id,
		"exp":       now + 3600000, // 1小时后过期
		"timestamp": now,
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// 签名
	signInput := headerB64 + "." + payloadB64
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signInput))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return signInput + "." + signature, nil
}

// GLM API 请求/响应结构
type glmRequest struct {
	Model       string       `json:"model"`
	Messages    []glmMessage `json:"messages"`
	Temperature float64      `json:"temperature,omitempty"`
	TopP        float64      `json:"top_p,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Stop        []string     `json:"stop,omitempty"`
}

type glmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type glmResponse struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int        `json:"index"`
		FinishReason string     `json:"finish_reason"`
		Message      glmMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete 完成请求
func (c *GLMClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// 生成 token
	token, err := c.generateToken()
	if err != nil {
		return nil, err
	}

	// 构建请求
	messages := make([]glmMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = glmMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	model := c.config.Model
	if model == "" {
		model = "glm-4" // 默认使用 GLM-4
	}

	glmReq := glmRequest{
		Model:       model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stop:        req.Stop,
	}

	body, err := json.Marshal(glmReq)
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
	httpReq.Header.Set("Authorization", "Bearer "+token)

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
	var glmResp glmResponse
	if err := json.Unmarshal(respBody, &glmResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 检查错误
	if glmResp.Error != nil {
		return nil, fmt.Errorf("glm error: %s - %s", glmResp.Error.Code, glmResp.Error.Message)
	}

	if len(glmResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &CompletionResponse{
		Text:         glmResp.Choices[0].Message.Content,
		FinishReason: glmResp.Choices[0].FinishReason,
		Usage: &Usage{
			PromptTokens:     glmResp.Usage.PromptTokens,
			CompletionTokens: glmResp.Usage.CompletionTokens,
			TotalTokens:      glmResp.Usage.TotalTokens,
		},
	}, nil
}

// HealthCheck 健康检查
func (c *GLMClient) HealthCheck(ctx context.Context) error {
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
func (c *GLMClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
