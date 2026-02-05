// Package llm 文心一言客户端 (百度)
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// WenxinClient 文心一言客户端
type WenxinClient struct {
	config     *types.LLMEngineConfig
	httpClient *http.Client
	baseURL    string
	
	// Access Token 管理
	accessToken   string
	tokenExpireAt time.Time
	tokenMu       sync.RWMutex
}

// NewWenxinClient 创建文心一言客户端
func NewWenxinClient(cfg *types.LLMEngineConfig) (*WenxinClient, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://aip.baidubce.com"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	client := &WenxinClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
	}

	return client, nil
}

// getAccessToken 获取或刷新 Access Token
func (c *WenxinClient) getAccessToken(ctx context.Context) (string, error) {
	c.tokenMu.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpireAt) {
		token := c.accessToken
		c.tokenMu.RUnlock()
		return token, nil
	}
	c.tokenMu.RUnlock()

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// 再次检查（可能其他协程已刷新）
	if c.accessToken != "" && time.Now().Before(c.tokenExpireAt) {
		return c.accessToken, nil
	}

	// 获取新 token
	url := fmt.Sprintf("%s/oauth/2.0/token?grant_type=client_credentials&client_id=%s&client_secret=%s",
		c.baseURL, c.config.APIKey, c.config.SecretKey)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error,omitempty"`
		ErrorDesc   string `json:"error_description,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("token error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpireAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn-300) * time.Second) // 提前5分钟过期

	return c.accessToken, nil
}

// 文心一言 API 请求/响应结构
type wenxinRequest struct {
	Messages    []wenxinMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	MaxOutputTokens int         `json:"max_output_tokens,omitempty"`
	System      string          `json:"system,omitempty"`
}

type wenxinMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type wenxinResponse struct {
	ID               string `json:"id"`
	Object           string `json:"object"`
	Created          int64  `json:"created"`
	Result           string `json:"result"`
	IsTruncated      bool   `json:"is_truncated"`
	NeedClearHistory bool   `json:"need_clear_history"`
	Usage            struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	ErrorCode int    `json:"error_code,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}

// getModelEndpoint 根据模型获取端点
func (c *WenxinClient) getModelEndpoint() string {
	model := c.config.Model
	switch model {
	case "ernie-4.0-8k", "ernie-4.0":
		return "/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/completions_pro"
	case "ernie-3.5-8k", "ernie-3.5":
		return "/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/completions"
	case "ernie-speed-8k", "ernie-speed":
		return "/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/ernie_speed"
	case "ernie-lite-8k", "ernie-lite":
		return "/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/ernie-lite-8k"
	default:
		return "/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/completions"
	}
}

// Complete 完成请求
func (c *WenxinClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// 获取 access token
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// 构建请求
	var system string
	var messages []wenxinMessage
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
		} else {
			messages = append(messages, wenxinMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	wenxinReq := wenxinRequest{
		Messages:        messages,
		Temperature:     req.Temperature,
		MaxOutputTokens: req.MaxTokens,
		System:          system,
	}

	body, err := json.Marshal(wenxinReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 创建 HTTP 请求
	endpoint := c.getModelEndpoint()
	url := fmt.Sprintf("%s%s?access_token=%s", c.baseURL, endpoint, token)
	
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
	var wenxinResp wenxinResponse
	if err := json.Unmarshal(respBody, &wenxinResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 检查错误
	if wenxinResp.ErrorCode != 0 {
		return nil, fmt.Errorf("wenxin error: %d - %s", wenxinResp.ErrorCode, wenxinResp.ErrorMsg)
	}

	return &CompletionResponse{
		Text:         wenxinResp.Result,
		FinishReason: "stop",
		Usage: &Usage{
			PromptTokens:     wenxinResp.Usage.PromptTokens,
			CompletionTokens: wenxinResp.Usage.CompletionTokens,
			TotalTokens:      wenxinResp.Usage.TotalTokens,
		},
	}, nil
}

// HealthCheck 健康检查
func (c *WenxinClient) HealthCheck(ctx context.Context) error {
	_, err := c.getAccessToken(ctx)
	return err
}

// Close 关闭
func (c *WenxinClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
