// Package embedding OpenAI 向量化实现
package embedding

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

// OpenAIEmbedder OpenAI 向量化器
type OpenAIEmbedder struct {
	config     *types.EmbeddingConfig
	httpClient *http.Client
	baseURL    string
	dimension  int
}

// NewOpenAIEmbedder 创建 OpenAI 向量化器
func NewOpenAIEmbedder(cfg *types.EmbeddingConfig) (*OpenAIEmbedder, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	dimension := cfg.Dimension
	if dimension == 0 {
		// 根据模型设置默认维度
		switch cfg.Model {
		case "text-embedding-3-small":
			dimension = 1536
		case "text-embedding-3-large":
			dimension = 3072
		case "text-embedding-ada-002":
			dimension = 1536
		default:
			dimension = 1536
		}
	}

	return &OpenAIEmbedder{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:   baseURL,
		dimension: dimension,
	}, nil
}

// OpenAI Embedding API 请求/响应结构
type openAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Embed 单文本向量化
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	vectors, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return vectors[0], nil
}

// EmbedBatch 批量向量化
func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// 构建请求
	req := openAIEmbeddingRequest{
		Model: e.config.Model,
		Input: texts,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	// 重试逻辑
	var lastErr error
	maxRetries := e.config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := e.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		var openaiResp openAIEmbeddingResponse
		if err := json.Unmarshal(respBody, &openaiResp); err != nil {
			lastErr = err
			continue
		}

		if openaiResp.Error != nil {
			lastErr = fmt.Errorf("openai error: %s", openaiResp.Error.Message)
			// 如果是 rate limit，等待后重试
			if resp.StatusCode == 429 {
				time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
				continue
			}
			return nil, lastErr
		}

		// 按索引排序结果
		results := make([][]float64, len(texts))
		for _, data := range openaiResp.Data {
			if data.Index < len(results) {
				results[data.Index] = data.Embedding
			}
		}

		return results, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// Dimension 向量维度
func (e *OpenAIEmbedder) Dimension() int {
	return e.dimension
}

// Name 引擎名称
func (e *OpenAIEmbedder) Name() string {
	return "openai"
}

// HealthCheck 健康检查
func (e *OpenAIEmbedder) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := e.Embed(ctx, "test")
	return err
}

// Close 关闭
func (e *OpenAIEmbedder) Close() error {
	e.httpClient.CloseIdleConnections()
	return nil
}
