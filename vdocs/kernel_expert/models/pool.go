// Package models 提供多模型支持
package models

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
)

// ModelPool 模型池
type ModelPool struct {
	registry  *ModelRegistry
	instances map[string]*ModelInstance
	router    Router

	mu sync.RWMutex

	// 健康检查
	healthChecker *HealthChecker
}

// ModelInstance 模型实例
type ModelInstance struct {
	Info       *ModelInfo
	Client     model.ToolCallingChatModel
	Status     ModelStatus
	LastUsed   time.Time
	UseCount   int64
	ErrorCount int64
	mu         sync.Mutex
}

// ModelStatus 模型状态
type ModelStatus int

const (
	ModelStatusReady ModelStatus = iota
	ModelStatusBusy
	ModelStatusError
	ModelStatusDisabled
)

// ModelPoolConfig 模型池配置
type ModelPoolConfig struct {
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration
	MaxRetries          int
}

// NewModelPool 创建模型池
func NewModelPool(registry *ModelRegistry, router Router, config *ModelPoolConfig) *ModelPool {
	pool := &ModelPool{
		registry:  registry,
		instances: make(map[string]*ModelInstance),
		router:    router,
	}

	// 创建健康检查器
	if config != nil && config.HealthCheckInterval > 0 {
		pool.healthChecker = &HealthChecker{
			pool:     pool,
			interval: config.HealthCheckInterval,
			timeout:  config.HealthCheckTimeout,
		}
	}

	return pool
}

// Start 启动模型池
func (p *ModelPool) Start(ctx context.Context) error {
	// 启动健康检查
	if p.healthChecker != nil {
		go p.healthChecker.Start(ctx)
	}
	return nil
}

// Stop 停止模型池
func (p *ModelPool) Stop() {
	// 健康检查会随着context取消自动停止
}

// Acquire 获取模型实例
func (p *ModelPool) Acquire(ctx context.Context, task *Task) (*ModelInstance, error) {
	// 路由选择模型
	info, err := p.router.Route(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("route: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 获取或创建实例
	instance, ok := p.instances[info.Name]
	if !ok {
		// 创建新实例
		client, err := p.createModelClient(ctx, info)
		if err != nil {
			return nil, fmt.Errorf("create client: %w", err)
		}

		instance = &ModelInstance{
			Info:   info,
			Client: client,
			Status: ModelStatusReady,
		}
		p.instances[info.Name] = instance
	}

	// 检查状态
	instance.mu.Lock()
	if instance.Status == ModelStatusError || instance.Status == ModelStatusDisabled {
		instance.mu.Unlock()
		return nil, fmt.Errorf("model %s is not available", info.Name)
	}

	instance.Status = ModelStatusBusy
	instance.UseCount++
	instance.LastUsed = time.Now()
	instance.mu.Unlock()

	return instance, nil
}

// Release 释放模型实例
func (p *ModelPool) Release(instance *ModelInstance, err error) {
	instance.mu.Lock()
	defer instance.mu.Unlock()

	instance.Status = ModelStatusReady
	instance.LastUsed = time.Now()

	if err != nil {
		instance.ErrorCount++
		// 连续错误超过阈值，标记为错误状态
		if instance.ErrorCount > 3 {
			instance.Status = ModelStatusError
		}
	} else {
		// 成功后重置错误计数
		instance.ErrorCount = 0
	}
}

// GetInstance 获取指定模型的实例
func (p *ModelPool) GetInstance(modelName string) (*ModelInstance, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	instance, ok := p.instances[modelName]
	return instance, ok
}

// GetStats 获取模型池统计
func (p *ModelPool) GetStats() map[string]*ModelInstanceStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]*ModelInstanceStats)
	for name, instance := range p.instances {
		instance.mu.Lock()
		stats[name] = &ModelInstanceStats{
			Model:      name,
			Status:     instance.Status,
			UseCount:   instance.UseCount,
			ErrorCount: instance.ErrorCount,
			LastUsed:   instance.LastUsed,
		}
		instance.mu.Unlock()
	}
	return stats
}

// ModelInstanceStats 模型实例统计
type ModelInstanceStats struct {
	Model      string      `json:"model"`
	Status     ModelStatus `json:"status"`
	UseCount   int64       `json:"use_count"`
	ErrorCount int64       `json:"error_count"`
	LastUsed   time.Time   `json:"last_used"`
}

func (p *ModelPool) createModelClient(ctx context.Context, info *ModelInfo) (model.ToolCallingChatModel, error) {
	// 这里需要根据provider创建实际的模型客户端
	// 实际实现需要集成eino-ext的各个模型实现
	provider, ok := p.registry.GetProvider(info.Provider)
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", info.Provider)
	}

	config := &ModelConfig{
		Provider: info.Provider,
		Model:    info.Name,
	}

	return provider.CreateModel(ctx, config)
}

// HealthChecker 健康检查器
type HealthChecker struct {
	pool     *ModelPool
	interval time.Duration
	timeout  time.Duration
}

// Start 启动健康检查
func (c *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkAll(ctx)
		}
	}
}

// checkAll 检查所有模型
func (c *HealthChecker) checkAll(ctx context.Context) {
	c.pool.mu.RLock()
	instances := make([]*ModelInstance, 0, len(c.pool.instances))
	for _, inst := range c.pool.instances {
		instances = append(instances, inst)
	}
	c.pool.mu.RUnlock()

	for _, instance := range instances {
		go c.check(ctx, instance)
	}
}

// check 检查单个模型
func (c *HealthChecker) check(ctx context.Context, instance *ModelInstance) {
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// 获取provider进行健康检查
	provider, ok := c.pool.registry.GetProvider(instance.Info.Provider)
	if !ok {
		return
	}

	err := provider.HealthCheck(checkCtx)

	instance.mu.Lock()
	defer instance.mu.Unlock()

	if err != nil {
		log.Printf("Health check failed for %s: %v", instance.Info.Name, err)
		if instance.Status != ModelStatusDisabled {
			instance.Status = ModelStatusError
		}
	} else {
		if instance.Status == ModelStatusError {
			instance.Status = ModelStatusReady
			instance.ErrorCount = 0
		}
	}
}

// SetModelEnabled 设置模型启用状态
func (p *ModelPool) SetModelEnabled(modelName string, enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if instance, ok := p.instances[modelName]; ok {
		instance.mu.Lock()
		if enabled {
			instance.Status = ModelStatusReady
		} else {
			instance.Status = ModelStatusDisabled
		}
		instance.mu.Unlock()
	}

	// 更新registry中的可用状态
	if info, ok := p.registry.GetModelInfo(modelName); ok {
		info.Available = enabled
	}
}
