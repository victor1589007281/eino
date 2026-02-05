// Package ml ML引擎实现
package ml

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// Engine ML引擎
type Engine struct {
	config     *types.MLEngineConfig
	classifier Classifier
	loaded     bool
}

// Classifier 分类器接口
type Classifier interface {
	// Predict 预测
	Predict(ctx context.Context, text string) (*PredictionResult, error)
	// Load 加载模型
	Load(modelPath string) error
	// Close 关闭
	Close() error
}

// PredictionResult 预测结果
type PredictionResult struct {
	Labels      []string  `json:"labels"`
	Scores      []float64 `json:"scores"`
	BestLabel   string    `json:"best_label"`
	BestScore   float64   `json:"best_score"`
}

// NewEngine 创建ML引擎
func NewEngine(cfg *types.MLEngineConfig) (*Engine, error) {
	e := &Engine{
		config: cfg,
	}

	// 根据模型类型创建分类器
	switch cfg.ModelType {
	case "fasttext":
		e.classifier = NewFastTextClassifier()
	case "bert":
		e.classifier = NewBERTClassifier()
	default:
		// 默认使用简单分类器
		e.classifier = NewSimpleClassifier()
	}

	// 尝试加载模型
	if cfg.ModelPath != "" {
		if err := e.classifier.Load(cfg.ModelPath); err != nil {
			// 模型加载失败不阻止启动，使用简单分类器降级
			fmt.Printf("warning: load model failed: %v, using simple classifier\n", err)
			e.classifier = NewSimpleClassifier()
		} else {
			e.loaded = true
		}
	}

	return e, nil
}

// Recognize 识别意图
func (e *Engine) Recognize(ctx context.Context, input *types.IntentInput) (*types.IntentResult, error) {
	start := time.Now()

	// 预处理
	text := preprocess(input.Text)

	// 预测
	prediction, err := e.classifier.Predict(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("prediction failed: %w", err)
	}

	// 检查置信度
	if prediction.BestScore < e.config.ConfidenceThreshold {
		return &types.IntentResult{
			Intent: &types.Intent{
				Name:       prediction.BestLabel,
				Confidence: prediction.BestScore,
			},
			Confidence: prediction.BestScore,
			Source:     "ml",
			Latency:    time.Since(start),
		}, nil
	}

	// 构建备选意图
	var alternatives []*types.Intent
	for i := 1; i < len(prediction.Labels) && i < 3; i++ {
		alternatives = append(alternatives, &types.Intent{
			Name:       prediction.Labels[i],
			Confidence: prediction.Scores[i],
		})
	}

	return &types.IntentResult{
		Intent: &types.Intent{
			Name:       prediction.BestLabel,
			Confidence: prediction.BestScore,
		},
		Alternatives: alternatives,
		Confidence:   prediction.BestScore,
		Source:       "ml",
		Latency:      time.Since(start),
	}, nil
}

// Name 引擎名称
func (e *Engine) Name() string {
	return "ml"
}

// Domains 支持的领域
func (e *Engine) Domains() []string {
	return []string{types.DomainEmail, types.DomainMemory, types.DomainSearch, types.DomainGeneral}
}

// HealthCheck 健康检查
func (e *Engine) HealthCheck(ctx context.Context) error {
	if e.classifier == nil {
		return fmt.Errorf("classifier not initialized")
	}
	return nil
}

// Close 关闭引擎
func (e *Engine) Close() error {
	if e.classifier != nil {
		return e.classifier.Close()
	}
	return nil
}

// preprocess 预处理文本
func preprocess(text string) string {
	// 简单预处理：去除多余空白
	return text
}
