// Package router 健康检查器
package router

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/search/engines"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	router   *AdaptiveRouter
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(router *AdaptiveRouter, interval time.Duration) *HealthChecker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &HealthChecker{
		router:   router,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动健康检查
func (h *HealthChecker) Start() {
	h.wg.Add(1)
	go h.checkLoop()
}

// Stop 停止健康检查
func (h *HealthChecker) Stop() {
	close(h.stopCh)
	h.wg.Wait()
}

// checkLoop 检查循环
func (h *HealthChecker) checkLoop() {
	defer h.wg.Done()

	// 启动时立即检查一次
	h.checkAll()

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.checkAll()
		case <-h.stopCh:
			return
		}
	}
}

// checkAll 检查所有引擎
func (h *HealthChecker) checkAll() {
	engineList := h.router.GetEngines()

	var wg sync.WaitGroup
	for name, engine := range engineList {
		wg.Add(1)
		go func(name string, engine engines.Engine) {
			defer wg.Done()
			h.checkEngine(name, engine)
		}(name, engine)
	}
	wg.Wait()
}

// checkEngine 检查单个引擎
func (h *HealthChecker) checkEngine(name string, engine engines.Engine) {
	state := h.router.GetEngineState(name)
	if state == nil {
		return
	}

	state.mu.Lock()
	currentStatus := state.Status
	circuitOpenTime := state.CircuitOpenTime
	state.mu.Unlock()

	// 如果引擎处于熔断状态，检查是否可以恢复
	if currentStatus == engines.StatusCircuitOpen {
		if time.Since(circuitOpenTime) < h.router.config.CircuitBreakerTimeout {
			// 熔断未超时，跳过检查
			return
		}
		// 熔断超时，尝试半开状态探测
		log.Printf("引擎 %s 熔断超时，尝试恢复探测", name)
	}

	// 执行健康检查
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := engine.HealthCheck(ctx)

	state.mu.Lock()
	defer state.mu.Unlock()

	state.LastCheck = time.Now()

	if err != nil {
		log.Printf("引擎 %s 健康检查失败: %v", name, err)
		if currentStatus != engines.StatusCircuitOpen {
			state.Status = engines.StatusUnhealthy
		}
	} else {
		// 健康检查通过
		state.Status = engines.StatusHealthy
		state.ConsecutiveFails = 0

		if currentStatus == engines.StatusCircuitOpen {
			log.Printf("引擎 %s 从熔断状态恢复", name)
		}
	}
}

// CheckEngineNow 立即检查指定引擎
func (h *HealthChecker) CheckEngineNow(name string) error {
	engineList := h.router.GetEngines()
	engine, ok := engineList[name]
	if !ok {
		return nil
	}

	h.checkEngine(name, engine)
	return nil
}

// GetHealthStatus 获取所有引擎健康状态
func (h *HealthChecker) GetHealthStatus() map[string]HealthStatus {
	states := h.router.GetEngineStates()
	result := make(map[string]HealthStatus)

	for name, info := range states {
		state := h.router.GetEngineState(name)
		if state == nil {
			continue
		}

		state.mu.RLock()
		result[name] = HealthStatus{
			Name:             name,
			Status:           info.Status.String(),
			LastCheck:        state.LastCheck,
			LastSuccess:      state.LastSuccess,
			ConsecutiveFails: state.ConsecutiveFails,
			Metrics:          info.Metrics,
		}
		state.mu.RUnlock()
	}

	return result
}

// HealthStatus 健康状态
type HealthStatus struct {
	Name             string                `json:"name"`
	Status           string                `json:"status"`
	LastCheck        time.Time             `json:"last_check"`
	LastSuccess      time.Time             `json:"last_success"`
	ConsecutiveFails int                   `json:"consecutive_fails"`
	Metrics          engines.EngineMetrics `json:"metrics"`
}
