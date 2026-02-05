// Package embedding 向量化引擎类型定义
package embedding

import (
	"context"
)

// Embedder 向量化接口
type Embedder interface {
	// Embed 单文本向量化
	Embed(ctx context.Context, text string) ([]float64, error)

	// EmbedBatch 批量向量化
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)

	// Dimension 向量维度
	Dimension() int

	// Name 引擎名称
	Name() string

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error

	// Close 关闭
	Close() error
}

// EmbeddingResult 向量化结果
type EmbeddingResult struct {
	Vector    []float64 `json:"vector"`
	Text      string    `json:"text"`
	TokenCount int      `json:"token_count,omitempty"`
}

// BatchResult 批量向量化结果
type BatchResult struct {
	Embeddings []*EmbeddingResult `json:"embeddings"`
	TotalTokens int               `json:"total_tokens"`
}

// Similarity 相似度计算接口
type Similarity interface {
	// CosineSimilarity 余弦相似度
	CosineSimilarity(a, b []float64) float64

	// EuclideanDistance 欧氏距离
	EuclideanDistance(a, b []float64) float64

	// DotProduct 点积
	DotProduct(a, b []float64) float64
}

// Cache 向量缓存接口
type Cache interface {
	// Get 获取缓存
	Get(ctx context.Context, key string) ([]float64, bool, error)

	// Set 设置缓存
	Set(ctx context.Context, key string, vector []float64) error

	// Delete 删除缓存
	Delete(ctx context.Context, key string) error

	// Clear 清空缓存
	Clear(ctx context.Context) error
}
