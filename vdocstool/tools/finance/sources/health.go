// Package sources 数据源健康检查
package sources

import (
	"context"
	"sync"
	"time"
)

// CircuitState 熔断状态
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // 关闭 (正常)
	CircuitOpen                         // 打开 (熔断)
	CircuitHalfOpen                     // 半开 (尝试恢复)
)

// HealthChecker 健康检查器
type HealthChecker struct {
	sources       map[string]DataSource
	health        map[string]*SourceHealth
	circuits      map[string]*CircuitBreaker
	config        *HealthConfig
	mu            sync.RWMutex
	stopCh        chan struct{}
	checkInterval time.Duration
}

// HealthConfig 健康检查配置
type HealthConfig struct {
	CheckInterval     time.Duration // 检查间隔
	FailureThreshold  int           // 失败阈值 (触发熔断)
	SuccessThreshold  int           // 成功阈值 (恢复服务)
	RecoveryTimeout   time.Duration // 熔断恢复超时
	HealthyThreshold  float64       // 健康成功率阈值
	DegradedThreshold float64       // 降级成功率阈值
}

// DefaultHealthConfig 默认健康检查配置
func DefaultHealthConfig() *HealthConfig {
	return &HealthConfig{
		CheckInterval:     30 * time.Second,
		FailureThreshold:  3,
		SuccessThreshold:  2,
		RecoveryTimeout:   60 * time.Second,
		HealthyThreshold:  0.9,
		DegradedThreshold: 0.5,
	}
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	state            CircuitState
	failures         int
	successes        int
	lastFailureTime  time.Time
	lastStateChange  time.Time
	config           *HealthConfig
	mu               sync.Mutex
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(config *HealthConfig) *CircuitBreaker {
	return &CircuitBreaker{
		state:           CircuitClosed,
		config:          config,
		lastStateChange: time.Now(),
	}
}

// CanExecute 检查是否可以执行
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// 检查是否可以进入半开状态
		if time.Since(cb.lastStateChange) > cb.config.RecoveryTimeout {
			cb.state = CircuitHalfOpen
			cb.lastStateChange = time.Now()
			cb.successes = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0

	if cb.state == CircuitHalfOpen {
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.state = CircuitClosed
			cb.lastStateChange = time.Now()
		}
	}
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()

	if cb.state == CircuitHalfOpen {
		// 半开状态下失败，立即熔断
		cb.state = CircuitOpen
		cb.lastStateChange = time.Now()
	} else if cb.state == CircuitClosed && cb.failures >= cb.config.FailureThreshold {
		// 关闭状态下连续失败达到阈值，熔断
		cb.state = CircuitOpen
		cb.lastStateChange = time.Now()
	}
}

// GetState 获取状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Reset 重置
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
	cb.lastStateChange = time.Now()
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(config *HealthConfig) *HealthChecker {
	if config == nil {
		config = DefaultHealthConfig()
	}

	return &HealthChecker{
		sources:       make(map[string]DataSource),
		health:        make(map[string]*SourceHealth),
		circuits:      make(map[string]*CircuitBreaker),
		config:        config,
		stopCh:        make(chan struct{}),
		checkInterval: config.CheckInterval,
	}
}

// RegisterSource 注册数据源
func (hc *HealthChecker) RegisterSource(source DataSource) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	name := source.Name()
	hc.sources[name] = source
	hc.health[name] = &SourceHealth{
		Name:        name,
		Status:      StatusHealthy,
		LastCheck:   time.Now(),
		LastSuccess: time.Now(),
		SuccessRate: 1.0,
	}
	hc.circuits[name] = NewCircuitBreaker(hc.config)
}

// Start 启动健康检查
func (hc *HealthChecker) Start() {
	go hc.checkLoop()
}

// Stop 停止健康检查
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

// checkLoop 检查循环
func (hc *HealthChecker) checkLoop() {
	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.checkAll()
		}
	}
}

// checkAll 检查所有数据源
func (hc *HealthChecker) checkAll() {
	hc.mu.RLock()
	sources := make([]DataSource, 0, len(hc.sources))
	for _, s := range hc.sources {
		sources = append(sources, s)
	}
	hc.mu.RUnlock()

	for _, source := range sources {
		go hc.checkSource(source)
	}
}

// checkSource 检查单个数据源
func (hc *HealthChecker) checkSource(source DataSource) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	name := source.Name()
	start := time.Now()
	err := source.HealthCheck(ctx)
	latency := time.Since(start)

	hc.mu.Lock()
	defer hc.mu.Unlock()

	health, ok := hc.health[name]
	if !ok {
		return
	}

	health.LastCheck = time.Now()
	health.AvgLatency = (health.AvgLatency + latency) / 2

	circuit, ok := hc.circuits[name]
	if !ok {
		return
	}

	if err != nil {
		circuit.RecordFailure()
		health.ConsecutiveFails++
		health.ErrorMessage = err.Error()
		
		// 更新成功率
		if health.SuccessRate > 0 {
			health.SuccessRate = health.SuccessRate * 0.9 // 指数衰减
		}
	} else {
		circuit.RecordSuccess()
		health.LastSuccess = time.Now()
		health.ConsecutiveFails = 0
		health.ErrorMessage = ""
		
		// 更新成功率
		health.SuccessRate = health.SuccessRate*0.9 + 0.1
	}

	// 更新状态
	health.Status = hc.calculateStatus(health, circuit)
}

// calculateStatus 计算状态
func (hc *HealthChecker) calculateStatus(health *SourceHealth, circuit *CircuitBreaker) SourceStatus {
	state := circuit.GetState()
	
	if state == CircuitOpen {
		return StatusUnhealthy
	}

	if health.SuccessRate >= hc.config.HealthyThreshold {
		return StatusHealthy
	}

	if health.SuccessRate >= hc.config.DegradedThreshold {
		return StatusDegraded
	}

	return StatusUnhealthy
}

// GetHealth 获取数据源健康状态
func (hc *HealthChecker) GetHealth(name string) *SourceHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if health, ok := hc.health[name]; ok {
		// 返回副本
		h := *health
		return &h
	}
	return nil
}

// GetAllHealth 获取所有数据源健康状态
func (hc *HealthChecker) GetAllHealth() map[string]*SourceHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result := make(map[string]*SourceHealth, len(hc.health))
	for name, health := range hc.health {
		h := *health
		result[name] = &h
	}
	return result
}

// IsHealthy 检查数据源是否健康
func (hc *HealthChecker) IsHealthy(name string) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	health, ok := hc.health[name]
	if !ok {
		return false
	}

	return health.Status == StatusHealthy || health.Status == StatusDegraded
}

// CanUse 检查数据源是否可用 (熔断检查)
func (hc *HealthChecker) CanUse(name string) bool {
	hc.mu.RLock()
	circuit, ok := hc.circuits[name]
	hc.mu.RUnlock()

	if !ok {
		return false
	}

	return circuit.CanExecute()
}

// RecordSuccess 记录成功
func (hc *HealthChecker) RecordSuccess(name string) {
	hc.mu.RLock()
	circuit, ok := hc.circuits[name]
	hc.mu.RUnlock()

	if ok {
		circuit.RecordSuccess()
	}
}

// RecordFailure 记录失败
func (hc *HealthChecker) RecordFailure(name string) {
	hc.mu.RLock()
	circuit, ok := hc.circuits[name]
	hc.mu.RUnlock()

	if ok {
		circuit.RecordFailure()
	}
}

// GetHealthySourcesSorted 获取健康的数据源 (按优先级排序)
func (hc *HealthChecker) GetHealthySourcesSorted() []DataSource {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	var result []DataSource
	for name, source := range hc.sources {
		health, ok := hc.health[name]
		if !ok {
			continue
		}

		circuit, ok := hc.circuits[name]
		if !ok {
			continue
		}

		// 只返回可用的数据源
		if circuit.CanExecute() && (health.Status == StatusHealthy || health.Status == StatusDegraded) {
			result = append(result, source)
		}
	}

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
