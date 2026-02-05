// Package intent 意图识别引擎
package intent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// Engine 意图识别引擎
type Engine struct {
	config *types.IntentConfig

	// 各引擎实例
	ruleEngine Recognizer
	mlEngine   Recognizer
	llmEngine  Recognizer

	// 路由器
	router *Router

	mu sync.RWMutex
}

// NewEngine 创建意图识别引擎
func NewEngine(cfg *types.IntentConfig) (*Engine, error) {
	e := &Engine{
		config: cfg,
	}

	// 初始化规则引擎
	if cfg.Rule.Enabled {
		ruleEngine, err := NewRuleEngine(&cfg.Rule)
		if err != nil {
			return nil, fmt.Errorf("init rule engine: %w", err)
		}
		e.ruleEngine = ruleEngine
	}

	// 初始化ML引擎
	if cfg.ML.Enabled {
		mlEngine, err := NewMLEngine(&cfg.ML)
		if err != nil {
			// ML引擎失败不阻止启动，可以降级
			fmt.Printf("warning: init ml engine failed: %v\n", err)
		} else {
			e.mlEngine = mlEngine
		}
	}

	// 初始化LLM引擎
	if cfg.LLM.Enabled {
		llmEngine, err := NewLLMEngine(&cfg.LLM)
		if err != nil {
			// LLM引擎失败不阻止启动，可以降级
			fmt.Printf("warning: init llm engine failed: %v\n", err)
		} else {
			e.llmEngine = llmEngine
		}
	}

	// 创建路由器
	e.router = NewRouter(cfg, e.ruleEngine, e.mlEngine, e.llmEngine)

	return e, nil
}

// Recognize 识别意图
func (e *Engine) Recognize(ctx context.Context, input *Input) (*Result, error) {
	start := time.Now()

	// 使用路由器选择策略并执行
	result, err := e.router.Route(ctx, input)
	if err != nil {
		return nil, err
	}

	result.Latency = time.Since(start)
	return result, nil
}

// RecognizeWithStrategy 使用指定策略识别意图
func (e *Engine) RecognizeWithStrategy(ctx context.Context, input *Input, strategy string) (*Result, error) {
	start := time.Now()

	var result *Result
	var err error

	switch strategy {
	case "rule":
		if e.ruleEngine != nil {
			result, err = e.ruleEngine.Recognize(ctx, input)
		} else {
			return nil, fmt.Errorf("rule engine not available")
		}
	case "ml":
		if e.mlEngine != nil {
			result, err = e.mlEngine.Recognize(ctx, input)
		} else {
			return nil, fmt.Errorf("ml engine not available")
		}
	case "llm":
		if e.llmEngine != nil {
			result, err = e.llmEngine.Recognize(ctx, input)
		} else {
			return nil, fmt.Errorf("llm engine not available")
		}
	default:
		return nil, fmt.Errorf("unknown strategy: %s", strategy)
	}

	if err != nil {
		return nil, err
	}

	result.Latency = time.Since(start)
	return result, nil
}

// HealthCheck 健康检查
func (e *Engine) HealthCheck(ctx context.Context) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 至少有一个引擎可用即为健康
	var available int
	var lastErr error

	if e.ruleEngine != nil {
		if err := e.ruleEngine.HealthCheck(ctx); err == nil {
			available++
		} else {
			lastErr = err
		}
	}

	if e.mlEngine != nil {
		if err := e.mlEngine.HealthCheck(ctx); err == nil {
			available++
		} else {
			lastErr = err
		}
	}

	if e.llmEngine != nil {
		if err := e.llmEngine.HealthCheck(ctx); err == nil {
			available++
		} else {
			lastErr = err
		}
	}

	if available == 0 {
		if lastErr != nil {
			return fmt.Errorf("no engine available: %w", lastErr)
		}
		return fmt.Errorf("no engine available")
	}

	return nil
}

// Close 关闭引擎
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var errs []error

	if e.ruleEngine != nil {
		if err := e.ruleEngine.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if e.mlEngine != nil {
		if err := e.mlEngine.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if e.llmEngine != nil {
		if err := e.llmEngine.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// AvailableEngines 返回可用引擎列表
func (e *Engine) AvailableEngines() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var engines []string
	if e.ruleEngine != nil {
		engines = append(engines, "rule")
	}
	if e.mlEngine != nil {
		engines = append(engines, "ml")
	}
	if e.llmEngine != nil {
		engines = append(engines, "llm")
	}
	return engines
}
