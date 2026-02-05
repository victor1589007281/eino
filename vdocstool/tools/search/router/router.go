// Package router 自适应路由
package router

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/config"
	"github.com/cloudwego/eino/vdocstool/tools/search/engines"
)

// Strategy 路由策略
type Strategy string

const (
	StrategyHealthFirst Strategy = "health_first" // 健康优先
	StrategyRoundRobin  Strategy = "round_robin"  // 轮询负载
	StrategyWeighted    Strategy = "weighted"     // 加权选择
)

// AdaptiveRouter 自适应路由器
type AdaptiveRouter struct {
	engines       map[string]engines.Engine
	engineStates  map[string]*EngineState
	strategy      Strategy
	HealthChecker *HealthChecker
	config        config.RouterConfig

	mu           sync.RWMutex
	roundRobinIdx int
}

// EngineState 引擎状态
type EngineState struct {
	Name              string
	Status            engines.EngineStatus
	ConsecutiveFails  int
	LastCheck         time.Time
	LastSuccess       time.Time
	CircuitOpenTime   time.Time
	Metrics           engines.EngineMetrics
	mu                sync.RWMutex
}

// NewAdaptiveRouter 创建自适应路由器
func NewAdaptiveRouter(cfg config.RouterConfig) *AdaptiveRouter {
	r := &AdaptiveRouter{
		engines:      make(map[string]engines.Engine),
		engineStates: make(map[string]*EngineState),
		strategy:     Strategy(cfg.Strategy),
		config:       cfg,
	}

	r.HealthChecker = NewHealthChecker(r, cfg.HealthCheckInterval)

	return r
}

// RegisterEngine 注册引擎
func (r *AdaptiveRouter) RegisterEngine(engine engines.Engine) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := engine.Name()
	r.engines[name] = engine
	r.engineStates[name] = &EngineState{
		Name:   name,
		Status: engines.StatusHealthy,
	}
}

// Start 启动路由器（包括健康检查）
func (r *AdaptiveRouter) Start() {
	r.HealthChecker.Start()
}

// Stop 停止路由器
func (r *AdaptiveRouter) Stop() {
	r.HealthChecker.Stop()
}

// SelectEngine 选择引擎
func (r *AdaptiveRouter) SelectEngine(ctx context.Context, preferred string) (engines.Engine, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 如果指定了首选引擎且健康，直接返回
	if preferred != "" {
		if engine, ok := r.engines[preferred]; ok {
			if state := r.engineStates[preferred]; state != nil {
				state.mu.RLock()
				status := state.Status
				state.mu.RUnlock()
				if status == engines.StatusHealthy || status == engines.StatusDegraded {
					return engine, nil
				}
			}
		}
	}

	// 根据策略选择引擎
	switch r.strategy {
	case StrategyHealthFirst:
		return r.selectHealthFirst()
	case StrategyRoundRobin:
		return r.selectRoundRobin()
	case StrategyWeighted:
		return r.selectWeighted()
	default:
		return r.selectHealthFirst()
	}
}

// selectHealthFirst 健康优先策略
func (r *AdaptiveRouter) selectHealthFirst() (engines.Engine, error) {
	// 收集健康引擎
	type enginePriority struct {
		engine   engines.Engine
		priority int
	}
	var healthyEngines []enginePriority

	for name, engine := range r.engines {
		state := r.engineStates[name]
		if state == nil {
			continue
		}

		state.mu.RLock()
		status := state.Status
		state.mu.RUnlock()

		if status == engines.StatusHealthy || status == engines.StatusDegraded {
			healthyEngines = append(healthyEngines, enginePriority{
				engine:   engine,
				priority: engine.Priority(),
			})
		}
	}

	if len(healthyEngines) == 0 {
		return nil, fmt.Errorf("no healthy engines available")
	}

	// 按优先级排序
	sort.Slice(healthyEngines, func(i, j int) bool {
		return healthyEngines[i].priority < healthyEngines[j].priority
	})

	return healthyEngines[0].engine, nil
}

// selectRoundRobin 轮询策略
func (r *AdaptiveRouter) selectRoundRobin() (engines.Engine, error) {
	var healthyEngines []engines.Engine

	for name, engine := range r.engines {
		state := r.engineStates[name]
		if state == nil {
			continue
		}

		state.mu.RLock()
		status := state.Status
		state.mu.RUnlock()

		if status == engines.StatusHealthy || status == engines.StatusDegraded {
			healthyEngines = append(healthyEngines, engine)
		}
	}

	if len(healthyEngines) == 0 {
		return nil, fmt.Errorf("no healthy engines available")
	}

	// 轮询选择
	idx := r.roundRobinIdx % len(healthyEngines)
	r.roundRobinIdx++

	return healthyEngines[idx], nil
}

// selectWeighted 加权选择策略
func (r *AdaptiveRouter) selectWeighted() (engines.Engine, error) {
	// 简单实现：基于成功率加权
	type weightedEngine struct {
		engine engines.Engine
		weight float64
	}
	var candidates []weightedEngine
	var totalWeight float64

	for name, engine := range r.engines {
		state := r.engineStates[name]
		if state == nil {
			continue
		}

		state.mu.RLock()
		status := state.Status
		metrics := state.Metrics
		state.mu.RUnlock()

		if status == engines.StatusHealthy || status == engines.StatusDegraded {
			// 权重 = (1 - 错误率) * (1 / 优先级)
			weight := (1 - metrics.ErrorRate) * (1.0 / float64(engine.Priority()))
			if weight <= 0 {
				weight = 0.1
			}
			candidates = append(candidates, weightedEngine{
				engine: engine,
				weight: weight,
			})
			totalWeight += weight
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no healthy engines available")
	}

	// 简单选择权重最高的
	var best weightedEngine
	for _, c := range candidates {
		if c.weight > best.weight {
			best = c
		}
	}

	return best.engine, nil
}

// Route 执行路由和搜索
func (r *AdaptiveRouter) Route(ctx context.Context, req *engines.SearchRequest, preferred string, autoFallback bool) (*RouteResult, error) {
	result := &RouteResult{
		Request: req,
	}

	// 选择引擎
	engine, err := r.SelectEngine(ctx, preferred)
	if err != nil {
		return nil, err
	}

	result.EngineUsed = engine.Name()

	// 执行搜索
	startTime := time.Now()
	results, err := engine.Search(ctx, req)
	latency := time.Since(startTime)

	// 更新指标
	r.updateMetrics(engine.Name(), err == nil, latency)

	if err != nil {
		// 记录失败
		r.recordFailure(engine.Name())

		// 尝试降级
		if autoFallback {
			fallbackEngine, fallbackErr := r.selectFallback(engine.Name())
			if fallbackErr == nil {
				result.FallbackTriggered = true
				result.OriginalEngine = engine.Name()
				result.EngineUsed = fallbackEngine.Name()

				// 使用备选引擎重试
				startTime = time.Now()
				results, err = fallbackEngine.Search(ctx, req)
				latency = time.Since(startTime)
				r.updateMetrics(fallbackEngine.Name(), err == nil, latency)

				if err != nil {
					r.recordFailure(fallbackEngine.Name())
				}
			}
		}

		if err != nil {
			return nil, fmt.Errorf("search failed: %w", err)
		}
	}

	result.Results = results
	result.Latency = latency

	return result, nil
}

// RouteResult 路由结果
type RouteResult struct {
	Request           *engines.SearchRequest
	Results           []*engines.SearchResult
	EngineUsed        string
	OriginalEngine    string
	FallbackTriggered bool
	Latency           time.Duration
}

// selectFallback 选择备选引擎
func (r *AdaptiveRouter) selectFallback(excludeName string) (engines.Engine, error) {
	type enginePriority struct {
		engine   engines.Engine
		priority int
	}
	var candidates []enginePriority

	for name, engine := range r.engines {
		if name == excludeName {
			continue
		}

		state := r.engineStates[name]
		if state == nil {
			continue
		}

		state.mu.RLock()
		status := state.Status
		state.mu.RUnlock()

		if status == engines.StatusHealthy || status == engines.StatusDegraded {
			candidates = append(candidates, enginePriority{
				engine:   engine,
				priority: engine.Priority(),
			})
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no fallback engine available")
	}

	// 按优先级排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].priority < candidates[j].priority
	})

	return candidates[0].engine, nil
}

// recordFailure 记录失败
func (r *AdaptiveRouter) recordFailure(name string) {
	r.mu.RLock()
	state := r.engineStates[name]
	r.mu.RUnlock()

	if state == nil {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	state.ConsecutiveFails++
	state.Metrics.FailedRequests++
	state.Metrics.TotalRequests++

	// 更新错误率
	if state.Metrics.TotalRequests > 0 {
		state.Metrics.ErrorRate = float64(state.Metrics.FailedRequests) / float64(state.Metrics.TotalRequests)
	}

	// 检查是否需要熔断
	if state.ConsecutiveFails >= r.config.CircuitBreakerThreshold {
		state.Status = engines.StatusCircuitOpen
		state.CircuitOpenTime = time.Now()
	}
}

// updateMetrics 更新指标
func (r *AdaptiveRouter) updateMetrics(name string, success bool, latency time.Duration) {
	r.mu.RLock()
	state := r.engineStates[name]
	r.mu.RUnlock()

	if state == nil {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	latencyMs := float64(latency.Milliseconds())
	state.Metrics.LastLatencyMs = latencyMs
	state.Metrics.TotalRequests++

	if success {
		state.Metrics.SuccessRequests++
		state.ConsecutiveFails = 0
		state.LastSuccess = time.Now()
	} else {
		state.Metrics.FailedRequests++
	}

	// 更新平均延迟
	if state.Metrics.TotalRequests > 0 {
		state.Metrics.AvgLatencyMs = (state.Metrics.AvgLatencyMs*float64(state.Metrics.TotalRequests-1) + latencyMs) / float64(state.Metrics.TotalRequests)
	}

	// 更新错误率
	if state.Metrics.TotalRequests > 0 {
		state.Metrics.ErrorRate = float64(state.Metrics.FailedRequests) / float64(state.Metrics.TotalRequests)
	}
}

// GetEngineStates 获取所有引擎状态
func (r *AdaptiveRouter) GetEngineStates() map[string]*engines.EngineInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*engines.EngineInfo)
	for name, engine := range r.engines {
		state := r.engineStates[name]
		if state == nil {
			continue
		}

		state.mu.RLock()
		info := &engines.EngineInfo{
			Name:     name,
			Status:   state.Status,
			Priority: engine.Priority(),
			Metrics:  state.Metrics,
		}
		state.mu.RUnlock()

		result[name] = info
	}

	return result
}

// GetEngines 获取所有引擎
func (r *AdaptiveRouter) GetEngines() map[string]engines.Engine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]engines.Engine)
	for name, engine := range r.engines {
		result[name] = engine
	}
	return result
}

// GetEngineState 获取引擎状态
func (r *AdaptiveRouter) GetEngineState(name string) *EngineState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.engineStates[name]
}

// SetStrategy 设置路由策略
func (r *AdaptiveRouter) SetStrategy(strategy Strategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategy = strategy
}
