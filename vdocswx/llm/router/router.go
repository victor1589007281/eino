/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package router

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/vdocswx/config"
)

// Router 模型路由器
type Router struct {
	config     *config.RouterConfig
	providers  map[string]model.ChatModel
	strategies map[string]*Strategy
	fallbacks  []string
}

// Strategy 路由策略
type Strategy struct {
	Primary    string
	Fallback   string
	Conditions []Condition
}

// Condition 路由条件
type Condition struct {
	Metric   string
	Operator string
	Value    interface{}
	Target   string
}

// Task 任务信息
type Task struct {
	Type       TaskType
	Content    string
	TokenCount int
	Complexity int
	Sensitive  bool
}

// TaskType 任务类型
type TaskType string

const (
	TaskTypeSimple  TaskType = "simple"
	TaskTypeMedium  TaskType = "medium"
	TaskTypeComplex TaskType = "complex"
)

// NewRouter 创建路由器
func NewRouter(cfg *config.RouterConfig, providers map[string]model.ChatModel) *Router {
	r := &Router{
		config:     cfg,
		providers:  providers,
		strategies: make(map[string]*Strategy),
		fallbacks:  cfg.Fallbacks,
	}

	// 加载策略
	for name, strategyCfg := range cfg.Strategies {
		strategy := &Strategy{
			Primary:  strategyCfg.Primary,
			Fallback: strategyCfg.Fallback,
		}
		for _, condCfg := range strategyCfg.Conditions {
			strategy.Conditions = append(strategy.Conditions, Condition{
				Metric:   condCfg.Metric,
				Operator: condCfg.Operator,
				Value:    condCfg.Value,
				Target:   condCfg.Target,
			})
		}
		r.strategies[name] = strategy
	}

	// 默认策略
	if r.strategies["simple"] == nil {
		r.strategies["simple"] = &Strategy{Primary: cfg.DefaultModel}
	}
	if r.strategies["medium"] == nil {
		r.strategies["medium"] = &Strategy{Primary: cfg.DefaultModel}
	}
	if r.strategies["complex"] == nil {
		r.strategies["complex"] = &Strategy{Primary: cfg.DefaultModel}
	}

	return r
}

// Route 路由到合适的模型
func (r *Router) Route(ctx context.Context, task *Task) (model.ChatModel, error) {
	// 敏感内容使用本地模型
	if task.Sensitive {
		if provider, ok := r.providers["ollama"]; ok {
			return provider, nil
		}
	}

	// 获取对应任务类型的策略
	strategy := r.strategies[string(task.Type)]
	if strategy == nil {
		strategy = r.strategies["simple"]
	}

	// 检查条件
	for _, cond := range strategy.Conditions {
		if r.matchCondition(task, cond) {
			if provider, ok := r.providers[cond.Target]; ok {
				return provider, nil
			}
		}
	}

	// 尝试主模型
	if provider, ok := r.providers[strategy.Primary]; ok {
		return provider, nil
	}

	// 尝试备用模型
	if strategy.Fallback != "" {
		if provider, ok := r.providers[strategy.Fallback]; ok {
			return provider, nil
		}
	}

	// 尝试全局fallback
	for _, fallback := range r.fallbacks {
		if provider, ok := r.providers[fallback]; ok {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("no available provider for task type %s", task.Type)
}

// matchCondition 检查条件是否匹配
func (r *Router) matchCondition(task *Task, cond Condition) bool {
	var value interface{}
	switch cond.Metric {
	case "token_count":
		value = task.TokenCount
	case "complexity":
		value = task.Complexity
	case "type":
		value = string(task.Type)
	default:
		return false
	}

	switch cond.Operator {
	case "gt":
		return compareGT(value, cond.Value)
	case "lt":
		return compareLT(value, cond.Value)
	case "eq":
		return value == cond.Value
	default:
		return false
	}
}

func compareGT(a, b interface{}) bool {
	switch v := a.(type) {
	case int:
		if bv, ok := b.(int); ok {
			return v > bv
		}
		if bv, ok := b.(float64); ok {
			return float64(v) > bv
		}
	case float64:
		if bv, ok := b.(float64); ok {
			return v > bv
		}
		if bv, ok := b.(int); ok {
			return v > float64(bv)
		}
	}
	return false
}

func compareLT(a, b interface{}) bool {
	switch v := a.(type) {
	case int:
		if bv, ok := b.(int); ok {
			return v < bv
		}
		if bv, ok := b.(float64); ok {
			return float64(v) < bv
		}
	case float64:
		if bv, ok := b.(float64); ok {
			return v < bv
		}
		if bv, ok := b.(int); ok {
			return v < float64(bv)
		}
	}
	return false
}

// GetDefaultModel 获取默认模型
func (r *Router) GetDefaultModel() (model.ChatModel, error) {
	if provider, ok := r.providers[r.config.DefaultModel]; ok {
		return provider, nil
	}
	return nil, fmt.Errorf("default model %s not found", r.config.DefaultModel)
}
