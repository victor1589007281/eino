// Package algorithm 算法层统一入口
package algorithm

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/vdocstool/algorithm/embedding"
	"github.com/cloudwego/eino/vdocstool/algorithm/index"
	"github.com/cloudwego/eino/vdocstool/algorithm/intent"
	"github.com/cloudwego/eino/vdocstool/algorithm/nlp"
)

// Algorithm 算法层统一入口
type Algorithm struct {
	// Intent 意图识别引擎
	Intent *intent.Engine
	// Embedding 向量化引擎
	Embedding *embedding.Engine
	// NLP NLP处理管道
	NLP *nlp.Pipeline
	// Index 索引管理器
	Index *index.Manager

	config  *Config
	metrics *Metrics
	mu      sync.RWMutex
}

// New 创建算法层实例
func New(cfg *Config) (*Algorithm, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	alg := &Algorithm{
		config:  cfg,
		metrics: NewMetrics(),
	}

	var err error

	// 初始化意图引擎
	alg.Intent, err = intent.NewEngine(&cfg.Intent)
	if err != nil {
		return nil, fmt.Errorf("init intent engine: %w", err)
	}

	// 初始化向量化引擎
	alg.Embedding, err = embedding.NewEngine(&cfg.Embedding)
	if err != nil {
		return nil, fmt.Errorf("init embedding engine: %w", err)
	}

	// 初始化 NLP 管道
	alg.NLP, err = nlp.NewPipeline(&cfg.NLP)
	if err != nil {
		return nil, fmt.Errorf("init nlp pipeline: %w", err)
	}

	// 初始化索引管理器（如果配置了索引服务）
	if len(cfg.Index.Elasticsearch.Addresses) > 0 || cfg.Index.Milvus.Address != "" || cfg.Index.Neo4j.URI != "" {
		alg.Index, err = index.NewManager(&cfg.Index)
		if err != nil {
			return nil, fmt.Errorf("init index manager: %w", err)
		}
	}

	return alg, nil
}

// Close 关闭算法层
func (a *Algorithm) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var errs []error

	if a.Intent != nil {
		if err := a.Intent.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close intent: %w", err))
		}
	}

	if a.Embedding != nil {
		if err := a.Embedding.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close embedding: %w", err))
		}
	}

	if a.NLP != nil {
		if err := a.NLP.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close nlp: %w", err))
		}
	}

	if a.Index != nil {
		if err := a.Index.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close index: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// HealthCheck 健康检查
func (a *Algorithm) HealthCheck(ctx context.Context) *HealthStatus {
	status := &HealthStatus{
		Components: make(map[string]*ComponentHealth),
	}

	// 并行检查各组件
	var wg sync.WaitGroup
	var mu sync.Mutex

	checkComponent := func(name string, checker func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			health := &ComponentHealth{Name: name, Healthy: true}
			if err := checker(ctx); err != nil {
				health.Healthy = false
				health.Error = err.Error()
			}
			mu.Lock()
			status.Components[name] = health
			mu.Unlock()
		}()
	}

	if a.Intent != nil {
		checkComponent("intent", a.Intent.HealthCheck)
	}
	if a.Embedding != nil {
		checkComponent("embedding", a.Embedding.HealthCheck)
	}
	if a.NLP != nil {
		checkComponent("nlp", a.NLP.HealthCheck)
	}
	if a.Index != nil {
		checkComponent("index", a.Index.HealthCheck)
	}

	wg.Wait()

	// 计算整体状态
	status.Healthy = true
	for _, c := range status.Components {
		if !c.Healthy {
			status.Healthy = false
			break
		}
	}

	return status
}

// HealthStatus 健康状态
type HealthStatus struct {
	Healthy    bool                        `json:"healthy"`
	Components map[string]*ComponentHealth `json:"components"`
}

// ComponentHealth 组件健康状态
type ComponentHealth struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
}

// GetMetrics 获取指标
func (a *Algorithm) GetMetrics() *Metrics {
	return a.metrics
}
