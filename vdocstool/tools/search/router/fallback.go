// Package router 降级策略
package router

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/search/engines"
)

// FallbackStrategy 降级策略
type FallbackStrategy struct {
	router      *AdaptiveRouter
	maxRetries  int
	retryDelay  time.Duration
}

// NewFallbackStrategy 创建降级策略
func NewFallbackStrategy(router *AdaptiveRouter) *FallbackStrategy {
	return &FallbackStrategy{
		router:     router,
		maxRetries: 2,
		retryDelay: 100 * time.Millisecond,
	}
}

// ExecuteWithFallback 带降级的执行
func (f *FallbackStrategy) ExecuteWithFallback(
	ctx context.Context,
	req *engines.SearchRequest,
	preferred string,
) (*FallbackResult, error) {
	result := &FallbackResult{
		Attempts: make([]AttemptRecord, 0),
	}

	// 获取可用引擎列表（按优先级排序）
	availableEngines := f.getSortedAvailableEngines(preferred)
	if len(availableEngines) == 0 {
		return nil, fmt.Errorf("no available engines")
	}

	var lastErr error

	for i, engine := range availableEngines {
		if i > f.maxRetries {
			break
		}

		attempt := AttemptRecord{
			EngineName: engine.Name(),
			AttemptNum: i + 1,
			StartTime:  time.Now(),
		}

		// 执行搜索
		results, err := engine.Search(ctx, req)
		attempt.Duration = time.Since(attempt.StartTime)

		if err != nil {
			attempt.Error = err.Error()
			attempt.Success = false
			lastErr = err

			log.Printf("引擎 %s 搜索失败 (尝试 %d): %v", engine.Name(), i+1, err)

			// 记录失败
			f.router.recordFailure(engine.Name())

			result.Attempts = append(result.Attempts, attempt)

			// 短暂延迟后重试下一个引擎
			if i < len(availableEngines)-1 && i < f.maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(f.retryDelay):
				}
			}
			continue
		}

		// 成功
		attempt.Success = true
		result.Attempts = append(result.Attempts, attempt)
		result.Results = results
		result.FinalEngine = engine.Name()
		result.FallbackUsed = i > 0

		// 更新成功指标
		f.router.updateMetrics(engine.Name(), true, attempt.Duration)

		return result, nil
	}

	// 所有尝试都失败
	return nil, fmt.Errorf("all engines failed, last error: %w", lastErr)
}

// getSortedAvailableEngines 获取排序后的可用引擎
func (f *FallbackStrategy) getSortedAvailableEngines(preferred string) []engines.Engine {
	engineMap := f.router.GetEngines()
	states := f.router.GetEngineStates()

	type engineWithPriority struct {
		engine   engines.Engine
		priority int
		isPreferred bool
	}

	var available []engineWithPriority

	for name, engine := range engineMap {
		info := states[name]
		if info == nil {
			continue
		}

		// 跳过熔断和不健康的引擎
		if info.Status == engines.StatusCircuitOpen || info.Status == engines.StatusUnhealthy {
			continue
		}

		available = append(available, engineWithPriority{
			engine:      engine,
			priority:    engine.Priority(),
			isPreferred: name == preferred,
		})
	}

	// 排序：首选 > 优先级
	sort.Slice(available, func(i, j int) bool {
		if available[i].isPreferred != available[j].isPreferred {
			return available[i].isPreferred
		}
		return available[i].priority < available[j].priority
	})

	result := make([]engines.Engine, len(available))
	for i, e := range available {
		result[i] = e.engine
	}

	return result
}

// FallbackResult 降级执行结果
type FallbackResult struct {
	Results      []*engines.SearchResult
	FinalEngine  string
	FallbackUsed bool
	Attempts     []AttemptRecord
}

// AttemptRecord 尝试记录
type AttemptRecord struct {
	EngineName string        `json:"engine_name"`
	AttemptNum int           `json:"attempt_num"`
	Success    bool          `json:"success"`
	Error      string        `json:"error,omitempty"`
	StartTime  time.Time     `json:"start_time"`
	Duration   time.Duration `json:"duration"`
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	threshold       int           // 失败阈值
	timeout         time.Duration // 熔断超时
	halfOpenMaxReqs int           // 半开状态最大请求数
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:       threshold,
		timeout:         timeout,
		halfOpenMaxReqs: 1,
	}
}

// ShouldTrip 检查是否应该熔断
func (cb *CircuitBreaker) ShouldTrip(consecutiveFails int) bool {
	return consecutiveFails >= cb.threshold
}

// ShouldAttemptRecovery 检查是否应该尝试恢复
func (cb *CircuitBreaker) ShouldAttemptRecovery(circuitOpenTime time.Time) bool {
	return time.Since(circuitOpenTime) >= cb.timeout
}

// RetryPolicy 重试策略
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Multiplier float64
}

// DefaultRetryPolicy 默认重试策略
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries: 2,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   2 * time.Second,
		Multiplier: 2.0,
	}
}

// GetDelay 获取重试延迟
func (p *RetryPolicy) GetDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return p.BaseDelay
	}

	delay := p.BaseDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * p.Multiplier)
		if delay > p.MaxDelay {
			delay = p.MaxDelay
			break
		}
	}
	return delay
}
