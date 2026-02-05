// Package embedding 向量化引擎
package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sync"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// Engine 向量化引擎
type Engine struct {
	config   *types.EmbeddingConfig
	embedder Embedder
	cache    Cache
	mu       sync.RWMutex
}

// NewEngine 创建向量化引擎
func NewEngine(cfg *types.EmbeddingConfig) (*Engine, error) {
	e := &Engine{
		config: cfg,
	}

	// 根据 provider 创建 embedder
	var err error
	switch cfg.Provider {
	case "openai":
		e.embedder, err = NewOpenAIEmbedder(cfg)
	case "bge":
		e.embedder, err = NewBGEEmbedder(cfg)
	default:
		e.embedder, err = NewOpenAIEmbedder(cfg) // 默认 OpenAI
	}

	if err != nil {
		return nil, fmt.Errorf("create embedder: %w", err)
	}

	// 初始化缓存
	if cfg.Cache.Enabled {
		e.cache = NewMemoryCache(cfg.Cache.MaxSize, cfg.Cache.TTL)
	}

	return e, nil
}

// Embed 单文本向量化（带缓存）
func (e *Engine) Embed(ctx context.Context, text string) ([]float64, error) {
	// 检查缓存
	if e.cache != nil {
		cacheKey := e.cacheKey(text)
		if vector, found, err := e.cache.Get(ctx, cacheKey); err == nil && found {
			return vector, nil
		}
	}

	// 调用 embedder
	vector, err := e.embedder.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	// 写入缓存
	if e.cache != nil {
		cacheKey := e.cacheKey(text)
		_ = e.cache.Set(ctx, cacheKey, vector)
	}

	return vector, nil
}

// EmbedBatch 批量向量化（带缓存）
func (e *Engine) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	results := make([][]float64, len(texts))
	var uncachedTexts []string
	var uncachedIndexes []int

	// 先检查缓存
	if e.cache != nil {
		for i, text := range texts {
			cacheKey := e.cacheKey(text)
			if vector, found, err := e.cache.Get(ctx, cacheKey); err == nil && found {
				results[i] = vector
			} else {
				uncachedTexts = append(uncachedTexts, text)
				uncachedIndexes = append(uncachedIndexes, i)
			}
		}
	} else {
		uncachedTexts = texts
		for i := range texts {
			uncachedIndexes = append(uncachedIndexes, i)
		}
	}

	// 如果全部命中缓存
	if len(uncachedTexts) == 0 {
		return results, nil
	}

	// 批量处理未缓存的
	batchSize := e.config.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	for i := 0; i < len(uncachedTexts); i += batchSize {
		end := i + batchSize
		if end > len(uncachedTexts) {
			end = len(uncachedTexts)
		}

		batch := uncachedTexts[i:end]
		vectors, err := e.embedder.EmbedBatch(ctx, batch)
		if err != nil {
			return nil, err
		}

		// 填充结果并写入缓存
		for j, vector := range vectors {
			idx := uncachedIndexes[i+j]
			results[idx] = vector

			if e.cache != nil {
				cacheKey := e.cacheKey(batch[j])
				_ = e.cache.Set(ctx, cacheKey, vector)
			}
		}
	}

	return results, nil
}

// Dimension 向量维度
func (e *Engine) Dimension() int {
	return e.embedder.Dimension()
}

// cacheKey 生成缓存键
func (e *Engine) cacheKey(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

// HealthCheck 健康检查
func (e *Engine) HealthCheck(ctx context.Context) error {
	if e.embedder == nil {
		return fmt.Errorf("embedder not initialized")
	}
	return e.embedder.HealthCheck(ctx)
}

// Close 关闭引擎
func (e *Engine) Close() error {
	if e.embedder != nil {
		return e.embedder.Close()
	}
	return nil
}

// 相似度计算函数

// CosineSimilarity 余弦相似度
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// EuclideanDistance 欧氏距离
func EuclideanDistance(a, b []float64) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}

	var sum float64
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return math.Sqrt(sum)
}

// DotProduct 点积
func DotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}

	return sum
}

// Normalize 向量归一化
func Normalize(v []float64) []float64 {
	var norm float64
	for _, val := range v {
		norm += val * val
	}
	norm = math.Sqrt(norm)

	if norm == 0 {
		return v
	}

	result := make([]float64, len(v))
	for i, val := range v {
		result[i] = val / norm
	}

	return result
}
