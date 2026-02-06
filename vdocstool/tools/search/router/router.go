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
	StrategyHealthFirst  Strategy = "health_first"   // 健康优先
	StrategyRoundRobin   Strategy = "round_robin"    // 轮询负载
	StrategyWeighted     Strategy = "weighted"       // 加权选择
	StrategyQualityFirst Strategy = "quality_first"  // 质量优先（新）
	StrategyCostSaving   Strategy = "cost_saving"    // 成本节约（新）
	StrategyBalanced     Strategy = "balanced"       // 均衡（新）
	StrategySmart        Strategy = "smart"          // 智能选择（新）
)

// AdaptiveRouter 自适应路由器
type AdaptiveRouter struct {
	engines       map[string]engines.Engine
	engineStates  map[string]*EngineState
	strategy      Strategy
	HealthChecker *HealthChecker
	config        config.RouterConfig

	// 新增：智能路由组件
	classifier     *QueryClassifier
	quotaManager   *QuotaManager
	qualityTracker *QualityTracker
	scorer         *EngineScorer

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

	// 初始化智能路由组件
	r.classifier = NewQueryClassifier()
	r.quotaManager = NewQuotaManager("")  // 可配置状态持久化路径
	r.qualityTracker = NewQualityTracker()
	r.scorer = NewEngineScorer(r.qualityTracker, r.quotaManager, r.classifier)

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
	r.quotaManager.Start()
}

// Stop 停止路由器
func (r *AdaptiveRouter) Stop() {
	r.HealthChecker.Stop()
	r.quotaManager.Stop()
}

// GetQuotaManager 获取配额管理器
func (r *AdaptiveRouter) GetQuotaManager() *QuotaManager {
	return r.quotaManager
}

// GetQualityTracker 获取质量追踪器
func (r *AdaptiveRouter) GetQualityTracker() *QualityTracker {
	return r.qualityTracker
}

// GetClassifier 获取查询分类器
func (r *AdaptiveRouter) GetClassifier() *QueryClassifier {
	return r.classifier
}

// GetScorer 获取评分器
func (r *AdaptiveRouter) GetScorer() *EngineScorer {
	return r.scorer
}

// GetEngineScores 获取所有引擎对特定查询的得分
func (r *AdaptiveRouter) GetEngineScores(query string) map[string]*ScoreDetails {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*ScoreDetails)
	for name := range r.engines {
		result[name] = r.scorer.ScoreWithDetails(name, query)
	}
	return result
}

// SelectEngine 选择引擎
func (r *AdaptiveRouter) SelectEngine(ctx context.Context, preferred string) (engines.Engine, error) {
	return r.SelectEngineForQuery(ctx, preferred, "")
}

// SelectEngineForQuery 根据查询选择引擎
func (r *AdaptiveRouter) SelectEngineForQuery(ctx context.Context, preferred string, query string) (engines.Engine, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 如果指定了首选引擎且健康且配额可用，直接返回
	if preferred != "" {
		if engine, ok := r.engines[preferred]; ok {
			if state := r.engineStates[preferred]; state != nil {
				state.mu.RLock()
				status := state.Status
				state.mu.RUnlock()
				if (status == engines.StatusHealthy || status == engines.StatusDegraded) &&
					r.quotaManager.IsAvailable(preferred) {
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
	case StrategyQualityFirst:
		return r.selectQualityFirst(query)
	case StrategyCostSaving:
		return r.selectCostSaving(query)
	case StrategyBalanced:
		return r.selectBalanced(query)
	case StrategySmart:
		return r.selectSmart(query)
	default:
		return r.selectSmart(query) // 默认使用智能策略
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

// selectQualityFirst 质量优先策略
func (r *AdaptiveRouter) selectQualityFirst(query string) (engines.Engine, error) {
	var bestEngine engines.Engine
	var bestScore float64 = -1

	for name, engine := range r.engines {
		// 检查引擎状态
		state := r.engineStates[name]
		if state == nil {
			continue
		}

		state.mu.RLock()
		status := state.Status
		state.mu.RUnlock()

		if status != engines.StatusHealthy && status != engines.StatusDegraded {
			continue
		}

		// 检查配额
		if !r.quotaManager.IsAvailable(name) {
			continue
		}

		// 计算得分
		score := r.scorer.Score(name, query)
		if score > bestScore {
			bestScore = score
			bestEngine = engine
		}
	}

	if bestEngine == nil {
		return nil, fmt.Errorf("no healthy engines available")
	}

	return bestEngine, nil
}

// selectCostSaving 成本节约策略
func (r *AdaptiveRouter) selectCostSaving(query string) (engines.Engine, error) {
	// 优先选择免费引擎
	var freeEngines []struct {
		engine engines.Engine
		score  float64
	}
	var paidEngines []struct {
		engine engines.Engine
		score  float64
	}

	for name, engine := range r.engines {
		state := r.engineStates[name]
		if state == nil {
			continue
		}

		state.mu.RLock()
		status := state.Status
		state.mu.RUnlock()

		if status != engines.StatusHealthy && status != engines.StatusDegraded {
			continue
		}

		if !r.quotaManager.IsAvailable(name) {
			continue
		}

		score := r.scorer.Score(name, query)
		
		if r.quotaManager.IsFree(name) {
			freeEngines = append(freeEngines, struct {
				engine engines.Engine
				score  float64
			}{engine, score})
		} else {
			paidEngines = append(paidEngines, struct {
				engine engines.Engine
				score  float64
			}{engine, score})
		}
	}

	// 如果有免费引擎
	if len(freeEngines) > 0 {
		// 选择得分最高的免费引擎
		var bestFree engines.Engine
		var bestFreeScore float64 = -1
		for _, e := range freeEngines {
			if e.score > bestFreeScore {
				bestFreeScore = e.score
				bestFree = e.engine
			}
		}

		// 检查付费引擎是否显著更好
		if len(paidEngines) > 0 {
			var bestPaidScore float64 = -1
			var bestPaid engines.Engine
			for _, e := range paidEngines {
				if e.score > bestPaidScore {
					bestPaidScore = e.score
					bestPaid = e.engine
				}
			}

			// 只有付费引擎得分超过免费引擎 40% 以上才选择付费
			if bestPaidScore > bestFreeScore*1.4 {
				return bestPaid, nil
			}
		}

		return bestFree, nil
	}

	// 没有免费引擎，选择付费中最优的
	if len(paidEngines) > 0 {
		var bestPaid engines.Engine
		var bestPaidScore float64 = -1
		for _, e := range paidEngines {
			if e.score > bestPaidScore {
				bestPaidScore = e.score
				bestPaid = e.engine
			}
		}
		return bestPaid, nil
	}

	return nil, fmt.Errorf("no healthy engines available")
}

// selectBalanced 均衡策略
func (r *AdaptiveRouter) selectBalanced(query string) (engines.Engine, error) {
	// 综合考虑质量和成本，使用加权随机选择
	type candidate struct {
		engine engines.Engine
		score  float64
	}
	var candidates []candidate
	var totalScore float64

	for name, engine := range r.engines {
		state := r.engineStates[name]
		if state == nil {
			continue
		}

		state.mu.RLock()
		status := state.Status
		state.mu.RUnlock()

		if status != engines.StatusHealthy && status != engines.StatusDegraded {
			continue
		}

		if !r.quotaManager.IsAvailable(name) {
			continue
		}

		score := r.scorer.Score(name, query)
		
		// 免费引擎给予额外加成
		if r.quotaManager.IsFree(name) {
			score *= 1.2
		}

		candidates = append(candidates, candidate{engine, score})
		totalScore += score
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no healthy engines available")
	}

	// 选择得分最高的（可改为加权随机以增加多样性）
	var best candidate
	for _, c := range candidates {
		if c.score > best.score {
			best = c
		}
	}

	return best.engine, nil
}

// selectSmart 智能选择策略
func (r *AdaptiveRouter) selectSmart(query string) (engines.Engine, error) {
	// 分析查询类型
	classification := r.classifier.Classify(query)
	
	type candidate struct {
		engine engines.Engine
		name   string
		score  float64
	}
	var candidates []candidate

	for name, engine := range r.engines {
		state := r.engineStates[name]
		if state == nil {
			continue
		}

		state.mu.RLock()
		status := state.Status
		state.mu.RUnlock()

		if status != engines.StatusHealthy && status != engines.StatusDegraded {
			continue
		}

		if !r.quotaManager.IsAvailable(name) {
			continue
		}

		// 获取详细得分
		details := r.scorer.ScoreWithDetails(name, query)
		score := details.FinalScore

		// 根据查询类型额外调整
		if classification.Language == "zh" && name == "baidu" {
			score *= 1.2 // 中文查询提升百度权重
		}
		if classification.PrimaryType == QueryTypeTechnical && name == "exa" {
			score *= 1.15 // 技术查询提升 Exa 权重
		}
		if classification.PrimaryType == QueryTypeNews && name == "serper" {
			score *= 1.1 // 新闻查询提升 Serper 权重
		}

		candidates = append(candidates, candidate{engine, name, score})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no healthy engines available")
	}

	// 按得分排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	return candidates[0].engine, nil
}

// Route 执行路由和搜索
func (r *AdaptiveRouter) Route(ctx context.Context, req *engines.SearchRequest, preferred string, autoFallback bool) (*RouteResult, error) {
	result := &RouteResult{
		Request: req,
	}

	// 使用查询感知的引擎选择
	engine, err := r.SelectEngineForQuery(ctx, preferred, req.Query)
	if err != nil {
		return nil, err
	}

	result.EngineUsed = engine.Name()

	// 分类查询（用于质量追踪）
	classification := r.classifier.Classify(req.Query)

	// 执行搜索
	startTime := time.Now()
	results, err := engine.Search(ctx, req)
	latency := time.Since(startTime)

	// 更新指标
	r.updateMetrics(engine.Name(), err == nil, latency)

	// 记录质量数据
	r.qualityTracker.Record(engine.Name(), &SearchRecord{
		Timestamp:   time.Now(),
		QueryType:   classification.PrimaryType,
		ResultCount: len(results),
		Latency:     latency,
		Success:     err == nil,
		ErrorType:   getErrorType(err),
	})

	// 更新配额
	if err == nil {
		r.quotaManager.IncrementUsage(engine.Name())
	}

	if err != nil {
		// 记录失败
		r.recordFailure(engine.Name())

		// 使评分缓存失效
		r.scorer.InvalidateEngine(engine.Name())

		// 尝试降级
		if autoFallback {
			fallbackEngine, fallbackErr := r.selectSmartFallback(engine.Name(), req.Query)
			if fallbackErr == nil {
				result.FallbackTriggered = true
				result.OriginalEngine = engine.Name()
				result.EngineUsed = fallbackEngine.Name()

				// 使用备选引擎重试
				startTime = time.Now()
				results, err = fallbackEngine.Search(ctx, req)
				latency = time.Since(startTime)
				r.updateMetrics(fallbackEngine.Name(), err == nil, latency)

				// 记录备选引擎的质量数据
				r.qualityTracker.Record(fallbackEngine.Name(), &SearchRecord{
					Timestamp:   time.Now(),
					QueryType:   classification.PrimaryType,
					ResultCount: len(results),
					Latency:     latency,
					Success:     err == nil,
					ErrorType:   getErrorType(err),
				})

				if err == nil {
					r.quotaManager.IncrementUsage(fallbackEngine.Name())
				} else {
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

// getErrorType 获取错误类型
func getErrorType(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// selectSmartFallback 智能选择备选引擎
func (r *AdaptiveRouter) selectSmartFallback(excludeName string, query string) (engines.Engine, error) {
	type candidate struct {
		engine engines.Engine
		score  float64
	}
	var candidates []candidate

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

		if status != engines.StatusHealthy && status != engines.StatusDegraded {
			continue
		}

		if !r.quotaManager.IsAvailable(name) {
			continue
		}

		score := r.scorer.Score(name, query)
		candidates = append(candidates, candidate{engine, score})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no fallback engine available")
	}

	// 选择得分最高的
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	return candidates[0].engine, nil
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
