// Package models 提供多模型支持
package models

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Router 模型路由器接口
type Router interface {
	Route(ctx context.Context, task *Task) (*ModelInfo, error)
}

// Task 任务信息
type Task struct {
	Intent      string
	Complexity  TaskComplexity
	InputTokens int
	Priority    Priority
	Constraints *Constraints
	Query       string
}

// TaskComplexity 任务复杂度
type TaskComplexity int

const (
	ComplexitySimple   TaskComplexity = iota // 简单查询
	ComplexityModerate                       // 中等分析
	ComplexityComplex                        // 复杂推理
	ComplexityExpert                         // 专家级
)

// Priority 优先级
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// Constraints 约束条件
type Constraints struct {
	MaxLatency  time.Duration
	MaxCost     float64
	PreferLocal bool
	Models      []string // 指定模型列表
}

// ComplexityRouter 复杂度路由器
type ComplexityRouter struct {
	registry     *ModelRegistry
	modelMapping map[TaskComplexity][]string
}

// NewComplexityRouter 创建复杂度路由器
func NewComplexityRouter(registry *ModelRegistry) *ComplexityRouter {
	return &ComplexityRouter{
		registry: registry,
		modelMapping: map[TaskComplexity][]string{
			ComplexitySimple:   {"gpt-4o-mini", "qwen-turbo", "claude-3-haiku", "deepseek-chat"},
			ComplexityModerate: {"gpt-4o", "qwen-plus", "claude-3-sonnet", "glm-4"},
			ComplexityComplex:  {"gpt-4-turbo", "qwen-max", "claude-3-sonnet", "deepseek-coder"},
			ComplexityExpert:   {"gpt-4-turbo", "claude-3-opus", "qwen-max"},
		},
	}
}

// Route 路由到合适的模型
func (r *ComplexityRouter) Route(ctx context.Context, task *Task) (*ModelInfo, error) {
	// 如果指定了模型，直接使用
	if task.Constraints != nil && len(task.Constraints.Models) > 0 {
		for _, modelName := range task.Constraints.Models {
			if info, ok := r.registry.GetModelInfo(modelName); ok && info.Available {
				return info, nil
			}
		}
	}

	// 根据复杂度选择模型
	preferred := r.modelMapping[task.Complexity]
	for _, modelName := range preferred {
		if info, ok := r.registry.GetModelInfo(modelName); ok && info.Available {
			return info, nil
		}
	}

	return nil, fmt.Errorf("no available model for complexity %d", task.Complexity)
}

// CostRouter 成本优先路由器
type CostRouter struct {
	registry *ModelRegistry
}

// NewCostRouter 创建成本路由器
func NewCostRouter(registry *ModelRegistry) *CostRouter {
	return &CostRouter{registry: registry}
}

// Route 路由到成本最低的模型
func (r *CostRouter) Route(ctx context.Context, task *Task) (*ModelInfo, error) {
	models := r.registry.ListModels()

	var best *ModelInfo
	var bestCost float64 = math.MaxFloat64

	for _, m := range models {
		if !m.Available || !m.SupportsTools {
			continue
		}

		// 估算成本 (假设输出2000 tokens)
		estimatedOutputTokens := 2000
		cost := float64(task.InputTokens)/1000*m.InputPrice + float64(estimatedOutputTokens)/1000*m.OutputPrice

		// 检查约束
		if task.Constraints != nil && task.Constraints.MaxCost > 0 && cost > task.Constraints.MaxCost {
			continue
		}

		if cost < bestCost {
			best = m
			bestCost = cost
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no suitable model found within cost constraints")
	}

	return best, nil
}

// LatencyRouter 延迟优先路由器
type LatencyRouter struct {
	registry     *ModelRegistry
	latencyStats map[string]*LatencyStats
	mu           sync.RWMutex
}

// LatencyStats 延迟统计
type LatencyStats struct {
	Model      string
	AvgLatency time.Duration
	P95Latency time.Duration
	Samples    int
}

// NewLatencyRouter 创建延迟路由器
func NewLatencyRouter(registry *ModelRegistry) *LatencyRouter {
	return &LatencyRouter{
		registry:     registry,
		latencyStats: make(map[string]*LatencyStats),
	}
}

// Route 路由到延迟最低的模型
func (r *LatencyRouter) Route(ctx context.Context, task *Task) (*ModelInfo, error) {
	models := r.registry.ListModels()

	r.mu.RLock()
	defer r.mu.RUnlock()

	// 按延迟排序
	sort.Slice(models, func(i, j int) bool {
		statsI := r.latencyStats[models[i].Name]
		statsJ := r.latencyStats[models[j].Name]

		if statsI == nil {
			return false
		}
		if statsJ == nil {
			return true
		}
		return statsI.AvgLatency < statsJ.AvgLatency
	})

	for _, m := range models {
		if m.Available && m.SupportsTools {
			// 检查延迟约束
			if task.Constraints != nil && task.Constraints.MaxLatency > 0 {
				if stats := r.latencyStats[m.Name]; stats != nil {
					if stats.P95Latency > task.Constraints.MaxLatency {
						continue
					}
				}
			}
			return m, nil
		}
	}

	return nil, fmt.Errorf("no suitable model found within latency constraints")
}

// RecordLatency 记录延迟
func (r *LatencyRouter) RecordLatency(model string, latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats, ok := r.latencyStats[model]
	if !ok {
		stats = &LatencyStats{Model: model}
		r.latencyStats[model] = stats
	}

	// 简单的移动平均
	stats.Samples++
	stats.AvgLatency = time.Duration((int64(stats.AvgLatency)*int64(stats.Samples-1) + int64(latency)) / int64(stats.Samples))
}

// LoadBalanceRouter 负载均衡路由器
type LoadBalanceRouter struct {
	registry *ModelRegistry
	weights  map[string]int
	current  map[string]int
	mu       sync.Mutex
}

// NewLoadBalanceRouter 创建负载均衡路由器
func NewLoadBalanceRouter(registry *ModelRegistry, weights map[string]int) *LoadBalanceRouter {
	return &LoadBalanceRouter{
		registry: registry,
		weights:  weights,
		current:  make(map[string]int),
	}
}

// Route 加权轮询路由
func (r *LoadBalanceRouter) Route(ctx context.Context, task *Task) (*ModelInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var best *ModelInfo
	var maxWeight int = -1

	for modelName, weight := range r.weights {
		info, ok := r.registry.GetModelInfo(modelName)
		if !ok || !info.Available {
			continue
		}

		r.current[modelName] += weight
		if r.current[modelName] > maxWeight {
			maxWeight = r.current[modelName]
			best = info
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no available model")
	}

	r.current[best.Name] -= r.totalWeight()
	return best, nil
}

func (r *LoadBalanceRouter) totalWeight() int {
	total := 0
	for _, w := range r.weights {
		total += w
	}
	return total
}

// CompositeRouter 组合路由器
type CompositeRouter struct {
	routers   []Router
	evaluator *ComplexityEvaluator
}

// NewCompositeRouter 创建组合路由器
func NewCompositeRouter(routers ...Router) *CompositeRouter {
	return &CompositeRouter{
		routers:   routers,
		evaluator: NewComplexityEvaluator(),
	}
}

// Route 组合路由
func (r *CompositeRouter) Route(ctx context.Context, task *Task) (*ModelInfo, error) {
	// 自动评估复杂度
	if task.Complexity == 0 {
		task.Complexity = r.evaluator.Evaluate(task.Query, task.Intent)
	}

	// 尝试所有路由器
	for _, router := range r.routers {
		info, err := router.Route(ctx, task)
		if err == nil && info != nil {
			return info, nil
		}
	}

	return nil, fmt.Errorf("all routers failed")
}

// ComplexityEvaluator 复杂度评估器
type ComplexityEvaluator struct {
	intentScores map[string]int
}

// NewComplexityEvaluator 创建复杂度评估器
func NewComplexityEvaluator() *ComplexityEvaluator {
	return &ComplexityEvaluator{
		intentScores: map[string]int{
			"concept":      1,
			"function":     2,
			"callchain":    3,
			"architecture": 4,
			"comparison":   3,
			"debug":        4,
			"performance":  4,
		},
	}
}

// Evaluate 评估任务复杂度
func (e *ComplexityEvaluator) Evaluate(query string, intent string) TaskComplexity {
	score := 0

	// 基于意图
	if s, ok := e.intentScores[intent]; ok {
		score += s
	}

	// 基于查询长度
	if len(query) > 200 {
		score += 1
	}
	if len(query) > 500 {
		score += 1
	}

	// 基于关键词
	complexKeywords := []string{"为什么", "深入", "详细", "完整", "所有", "性能", "优化", "对比"}
	for _, kw := range complexKeywords {
		if containsString(query, kw) {
			score += 1
		}
	}

	// 映射到复杂度级别
	switch {
	case score <= 2:
		return ComplexitySimple
	case score <= 4:
		return ComplexityModerate
	case score <= 6:
		return ComplexityComplex
	default:
		return ComplexityExpert
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
