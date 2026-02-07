// Package sources 智能路由器
package sources

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// Router 智能路由器
type Router struct {
	sources       []DataSource
	healthChecker *HealthChecker
	rateLimiter   *RateLimiter
	config        *RouterConfig
	mu            sync.RWMutex
}

// RouterConfig 路由器配置
type RouterConfig struct {
	MaxRetries      int           // 最大重试次数
	RetryDelay      time.Duration // 重试延迟
	Timeout         time.Duration // 请求超时
	EnableFallback  bool          // 启用故障转移
	EnableRateLimit bool          // 启用频率限制
}

// DefaultRouterConfig 默认路由器配置
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		MaxRetries:      3,
		RetryDelay:      500 * time.Millisecond,
		Timeout:         10 * time.Second,
		EnableFallback:  true,
		EnableRateLimit: true,
	}
}

// NewRouter 创建路由器
func NewRouter(config *RouterConfig) *Router {
	if config == nil {
		config = DefaultRouterConfig()
	}

	healthConfig := DefaultHealthConfig()
	
	r := &Router{
		sources:       make([]DataSource, 0),
		healthChecker: NewHealthChecker(healthConfig),
		rateLimiter:   NewRateLimiter(),
		config:        config,
	}

	return r
}

// RegisterSource 注册数据源
func (r *Router) RegisterSource(source DataSource) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sources = append(r.sources, source)
	r.healthChecker.RegisterSource(source)
	
	// 设置默认限速
	r.rateLimiter.SetLimit(source.Name(), 10) // 默认每秒10次
}

// Start 启动路由器
func (r *Router) Start() {
	r.healthChecker.Start()
}

// Stop 停止路由器
func (r *Router) Stop() {
	r.healthChecker.Stop()
}

// GetQuote 获取行情 (自动路由和故障转移)
func (r *Router) GetQuote(ctx context.Context, symbol string) (*types.Quote, error) {
	result, err := r.ExecuteWithFallback(ctx, "GetQuote", func(ctx context.Context, source DataSource) (interface{}, error) {
		return source.GetQuote(ctx, symbol)
	})
	if err != nil {
		return nil, err
	}
	return result.(*types.Quote), nil
}

// GetQuotes 批量获取行情
func (r *Router) GetQuotes(ctx context.Context, symbols []string) ([]*types.Quote, error) {
	result, err := r.ExecuteWithFallback(ctx, "GetQuotes", func(ctx context.Context, source DataSource) (interface{}, error) {
		return source.GetQuotes(ctx, symbols)
	})
	if err != nil {
		return nil, err
	}
	return result.([]*types.Quote), nil
}

// GetKLine 获取K线数据
func (r *Router) GetKLine(ctx context.Context, symbol string, period string, count int) ([]*types.KLine, error) {
	result, err := r.ExecuteWithFallback(ctx, "GetKLine", func(ctx context.Context, source DataSource) (interface{}, error) {
		return source.GetKLine(ctx, symbol, period, count)
	})
	if err != nil {
		return nil, err
	}
	return result.([]*types.KLine), nil
}

// Search 搜索
func (r *Router) Search(ctx context.Context, keyword string) ([]*types.SearchResult, error) {
	result, err := r.ExecuteWithFallback(ctx, "Search", func(ctx context.Context, source DataSource) (interface{}, error) {
		return source.Search(ctx, keyword)
	})
	if err != nil {
		return nil, err
	}
	return result.([]*types.SearchResult), nil
}

// ExecuteWithFallback 带故障转移的执行 (导出方法)
func (r *Router) ExecuteWithFallback(ctx context.Context, operation string, fn func(context.Context, DataSource) (interface{}, error)) (interface{}, error) {
	// 获取可用的数据源
	sources := r.getAvailableSources()
	if len(sources) == 0 {
		return nil, fmt.Errorf("no available data sources")
	}

	var lastErr error
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		for _, source := range sources {
			// 检查频率限制
			if r.config.EnableRateLimit && !r.rateLimiter.Allow(source.Name()) {
				continue
			}

			// 检查熔断
			if !r.healthChecker.CanUse(source.Name()) {
				continue
			}

			// 创建带超时的上下文
			execCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)

			// 执行操作
			result, err := fn(execCtx, source)
			cancel()

			if err != nil {
				lastErr = fmt.Errorf("[%s] %s failed: %w", source.Name(), operation, err)
				r.healthChecker.RecordFailure(source.Name())
				
				if !r.config.EnableFallback {
					return nil, lastErr
				}
				continue
			}

			// 成功
			r.healthChecker.RecordSuccess(source.Name())
			return result, nil
		}

		// 所有数据源都失败,等待后重试
		if attempt < r.config.MaxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(r.config.RetryDelay):
			}
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all sources failed after %d retries: %w", r.config.MaxRetries, lastErr)
	}
	return nil, fmt.Errorf("no available data sources")
}

// getAvailableSources 获取可用数据源 (按优先级排序)
func (r *Router) getAvailableSources() []DataSource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 获取健康的数据源
	healthySources := r.healthChecker.GetHealthySourcesSorted()
	if len(healthySources) > 0 {
		return healthySources
	}

	// 如果没有健康的数据源,返回所有数据源 (降级)
	result := make([]DataSource, len(r.sources))
	copy(result, r.sources)
	
	// 按优先级排序
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Priority() > result[j].Priority() {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	
	return result
}

// GetSourceHealth 获取数据源健康状态
func (r *Router) GetSourceHealth(name string) *SourceHealth {
	return r.healthChecker.GetHealth(name)
}

// GetAllSourceHealth 获取所有数据源健康状态
func (r *Router) GetAllSourceHealth() map[string]*SourceHealth {
	return r.healthChecker.GetAllHealth()
}

// SetRateLimit 设置数据源限速
func (r *Router) SetRateLimit(sourceName string, rps int) {
	r.rateLimiter.SetLimit(sourceName, rps)
}

// GetSources 获取所有数据源
func (r *Router) GetSources() []DataSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	result := make([]DataSource, len(r.sources))
	copy(result, r.sources)
	return result
}

// ForceHealthCheck 强制健康检查
func (r *Router) ForceHealthCheck() {
	r.healthChecker.checkAll()
}

// 实现 DataSource 接口的包装方法

// GetQuoteFromSource 从指定数据源获取行情
func (r *Router) GetQuoteFromSource(ctx context.Context, sourceName string, symbol string) (*types.Quote, error) {
	source := r.getSourceByName(sourceName)
	if source == nil {
		return nil, fmt.Errorf("source not found: %s", sourceName)
	}

	if r.config.EnableRateLimit && !r.rateLimiter.Allow(sourceName) {
		return nil, fmt.Errorf("rate limit exceeded for %s", sourceName)
	}

	if !r.healthChecker.CanUse(sourceName) {
		return nil, fmt.Errorf("source %s is unavailable (circuit open)", sourceName)
	}

	quote, err := source.GetQuote(ctx, symbol)
	if err != nil {
		r.healthChecker.RecordFailure(sourceName)
		return nil, err
	}

	r.healthChecker.RecordSuccess(sourceName)
	return quote, nil
}

// getSourceByName 通过名称获取数据源
func (r *Router) getSourceByName(name string) DataSource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, source := range r.sources {
		if source.Name() == name {
			return source
		}
	}
	return nil
}

// ResetCircuit 重置熔断器
func (r *Router) ResetCircuit(sourceName string) {
	r.mu.RLock()
	circuit, ok := r.healthChecker.circuits[sourceName]
	r.mu.RUnlock()

	if ok {
		circuit.Reset()
	}
}

// GetRouterStats 获取路由器统计
type RouterStats struct {
	TotalSources   int                       `json:"total_sources"`
	HealthySources int                       `json:"healthy_sources"`
	SourceHealth   map[string]*SourceHealth  `json:"source_health"`
}

// GetStats 获取统计信息
func (r *Router) GetStats() *RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	health := r.healthChecker.GetAllHealth()
	
	healthy := 0
	for _, h := range health {
		if h.Status == StatusHealthy || h.Status == StatusDegraded {
			healthy++
		}
	}

	return &RouterStats{
		TotalSources:   len(r.sources),
		HealthySources: healthy,
		SourceHealth:   health,
	}
}
