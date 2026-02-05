// Package embedding BGE 向量化实现
// BGE (BAAI General Embedding) 是一个高质量的中文/多语言嵌入模型
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

// BGEEmbedder BGE 向量化器
// 支持本地部署的 BGE 模型服务或兼容的 API
type BGEEmbedder struct {
	config     *types.EmbeddingConfig
	httpClient *http.Client
	baseURL    string
	dimension  int
}

// NewBGEEmbedder 创建 BGE 向量化器
func NewBGEEmbedder(cfg *types.EmbeddingConfig) (*BGEEmbedder, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		// 默认本地服务地址
		baseURL = "http://localhost:8080"
	}

	dimension := cfg.Dimension
	if dimension == 0 {
		// BGE 模型默认维度
		switch cfg.Model {
		case "bge-large-zh-v1.5":
			dimension = 1024
		case "bge-base-zh-v1.5":
			dimension = 768
		case "bge-small-zh-v1.5":
			dimension = 512
		case "bge-m3":
			dimension = 1024
		default:
			dimension = 1024
		}
	}

	return &BGEEmbedder{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:   baseURL,
		dimension: dimension,
	}, nil
}

// BGE API 请求/响应结构（兼容多种部署方式）
type bgeEmbeddingRequest struct {
	Model  string   `json:"model,omitempty"`
	Input  []string `json:"input"`
	Inputs []string `json:"inputs,omitempty"` // 某些实现使用 inputs
}

type bgeEmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data,omitempty"`
	
	// 某些实现直接返回 embeddings 数组
	Embeddings [][]float64 `json:"embeddings,omitempty"`
	
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Embed 单文本向量化
func (e *BGEEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
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
func (e *BGEEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// 构建请求
	req := bgeEmbeddingRequest{
		Model:  e.config.Model,
		Input:  texts,
		Inputs: texts, // 兼容不同实现
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
	if e.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)
	}

	// 发送请求
	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var bgeResp bgeEmbeddingResponse
	if err := json.Unmarshal(respBody, &bgeResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if bgeResp.Error != nil {
		return nil, fmt.Errorf("bge error: %s", bgeResp.Error.Message)
	}

	// 处理响应（兼容两种格式）
	var results [][]float64
	
	if len(bgeResp.Data) > 0 {
		// OpenAI 兼容格式
		results = make([][]float64, len(texts))
		for _, data := range bgeResp.Data {
			if data.Index < len(results) {
				results[data.Index] = data.Embedding
			}
		}
	} else if len(bgeResp.Embeddings) > 0 {
		// 直接数组格式
		results = bgeResp.Embeddings
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}

	return results, nil
}

// Dimension 向量维度
func (e *BGEEmbedder) Dimension() int {
	return e.dimension
}

// Name 引擎名称
func (e *BGEEmbedder) Name() string {
	return "bge"
}

// HealthCheck 健康检查
func (e *BGEEmbedder) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := e.Embed(ctx, "test")
	return err
}

// Close 关闭
func (e *BGEEmbedder) Close() error {
	e.httpClient.CloseIdleConnections()
	return nil
}
