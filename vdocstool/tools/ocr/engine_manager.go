// Package ocr 引擎管理器
package ocr

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// EngineManager 引擎管理器
type EngineManager struct {
	config  *Config
	engines map[EngineType]Engine
	stats   map[EngineType]*engineStats
	mu      sync.RWMutex

	// 内存管理
	memoryWatcher *time.Ticker
	stopCh        chan struct{}
}

// engineStats 引擎统计信息
type engineStats struct {
	lastUsed    time.Time
	totalCalls  int64
	failedCalls int64
	totalLatency int64
}

// NewEngineManager 创建引擎管理器
func NewEngineManager(config *Config) *EngineManager {
	if config == nil {
		config = DefaultConfig()
	}

	em := &EngineManager{
		config:  config,
		engines: make(map[EngineType]Engine),
		stats:   make(map[EngineType]*engineStats),
		stopCh:  make(chan struct{}),
	}

	return em
}

// RegisterEngine 注册引擎
func (m *EngineManager) RegisterEngine(engine Engine) {
	m.mu.Lock()
	defer m.mu.Unlock()

	engineType := engine.Name()
	m.engines[engineType] = engine
	m.stats[engineType] = &engineStats{}
}

// Start 启动管理器
func (m *EngineManager) Start(ctx context.Context) error {
	// 加载常驻引擎
	for _, engineType := range m.config.ResidentEngines {
		if engine, ok := m.engines[engineType]; ok {
			if err := engine.Load(ctx); err != nil {
				// 常驻引擎加载失败只警告，不中断
				fmt.Printf("warning: failed to load resident engine %s: %v\n", engineType, err)
			}
		}
	}

	// 启动内存监控
	m.memoryWatcher = time.NewTicker(30 * time.Second)
	go m.watchMemory()

	return nil
}

// Stop 停止管理器
func (m *EngineManager) Stop() error {
	close(m.stopCh)
	if m.memoryWatcher != nil {
		m.memoryWatcher.Stop()
	}

	// 卸载所有引擎
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, engine := range m.engines {
		if engine.IsLoaded() {
			_ = engine.Unload()
		}
	}

	return nil
}

// GetEngine 获取引擎 (按需加载)
func (m *EngineManager) GetEngine(ctx context.Context, engineType EngineType) (Engine, error) {
	m.mu.RLock()
	engine, ok := m.engines[engineType]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("engine %s not registered", engineType)
	}

	// 检查是否需要加载
	if !engine.IsLoaded() {
		// 检查内存限制
		if err := m.ensureMemory(ctx, engine.MemoryUsage()); err != nil {
			return nil, fmt.Errorf("insufficient memory: %w", err)
		}

		// 加载引擎
		if err := engine.Load(ctx); err != nil {
			return nil, fmt.Errorf("load engine %s failed: %w", engineType, err)
		}
	}

	// 更新使用时间
	m.mu.Lock()
	if stats, ok := m.stats[engineType]; ok {
		stats.lastUsed = time.Now()
	}
	m.mu.Unlock()

	return engine, nil
}

// SelectEngine 根据场景选择最佳引擎
func (m *EngineManager) SelectEngine(ctx context.Context, scene SceneType, strategy RoutingStrategy) (Engine, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 收集可用引擎
	candidates := make([]Engine, 0)
	for _, engine := range m.engines {
		if m.supportsScene(engine, scene) {
			candidates = append(candidates, engine)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no engine available for scene %s", scene)
	}

	// 根据策略排序
	switch strategy {
	case StrategyQualityFirst:
		sort.Slice(candidates, func(i, j int) bool {
			return m.getQualityScore(candidates[i]) > m.getQualityScore(candidates[j])
		})
	case StrategyCostFirst:
		sort.Slice(candidates, func(i, j int) bool {
			return m.getCostScore(candidates[i]) < m.getCostScore(candidates[j])
		})
	case StrategySpeedFirst:
		sort.Slice(candidates, func(i, j int) bool {
			return m.getSpeedScore(candidates[i]) > m.getSpeedScore(candidates[j])
		})
	default: // StrategySmart
		sort.Slice(candidates, func(i, j int) bool {
			return m.getSmartScore(candidates[i], scene) > m.getSmartScore(candidates[j], scene)
		})
	}

	// 尝试获取最佳引擎
	for _, engine := range candidates {
		if !engine.IsLoaded() {
			if err := m.ensureMemory(ctx, engine.MemoryUsage()); err != nil {
				continue // 内存不足，尝试下一个
			}
			if err := engine.Load(ctx); err != nil {
				continue // 加载失败，尝试下一个
			}
		}
		return engine, nil
	}

	return nil, fmt.Errorf("all engines unavailable for scene %s", scene)
}

// GetFallbackChain 获取降级链
func (m *EngineManager) GetFallbackChain() []EngineType {
	return m.config.FallbackChain
}

// supportsScene 检查引擎是否支持场景
func (m *EngineManager) supportsScene(engine Engine, scene SceneType) bool {
	for _, s := range engine.SupportedScenes() {
		if s == scene || s == SceneGeneral {
			return true
		}
	}
	return false
}

// getQualityScore 获取质量评分
func (m *EngineManager) getQualityScore(engine Engine) float64 {
	// VLM > 小模型 > 传统OCR
	switch engine.Layer() {
	case LayerVLM:
		return 1.0
	case LayerSmallModel:
		return 0.7
	case LayerTraditional:
		return 0.5
	default:
		return 0.3
	}
}

// getCostScore 获取成本评分 (越低越好)
func (m *EngineManager) getCostScore(engine Engine) float64 {
	// 本地免费 > 本地收费 > 云端收费
	switch engine.Name() {
	case EngineTesseract, EngineRapidOCR:
		return 0.0
	case EnginePaddleOCR:
		return 0.1
	case EngineBaidu, EngineTencent:
		return 0.5
	case EngineQwenVL:
		return 0.7
	case EngineGPT4V, EngineClaude:
		return 1.0
	default:
		return 0.5
	}
}

// getSpeedScore 获取速度评分
func (m *EngineManager) getSpeedScore(engine Engine) float64 {
	// 传统OCR > 小模型 > VLM
	switch engine.Layer() {
	case LayerTraditional:
		return 1.0
	case LayerSmallModel:
		return 0.7
	case LayerVLM:
		return 0.3
	default:
		return 0.5
	}
}

// getSmartScore 获取智能评分
func (m *EngineManager) getSmartScore(engine Engine, scene SceneType) float64 {
	baseScore := 0.5

	// 根据场景调整
	switch scene {
	case SceneHandwriting:
		// 手写体 VLM 效果最好
		if engine.Layer() == LayerVLM {
			baseScore = 1.0
		}
	case SceneTable:
		// 表格 PaddleOCR PP-Structure 效果好
		if engine.Name() == EnginePaddleOCR {
			baseScore = 0.9
		}
	case SceneInvoice, SceneIDCard:
		// 票据证件云服务效果好
		if engine.Name() == EngineBaidu || engine.Name() == EngineTencent {
			baseScore = 0.9
		}
	case ScenePrint, SceneDocument:
		// 印刷体文档本地模型足够
		if engine.Layer() == LayerSmallModel {
			baseScore = 0.8
		}
	}

	// 已加载的引擎加分
	if engine.IsLoaded() {
		baseScore += 0.1
	}

	return min(baseScore, 1.0)
}

// ensureMemory 确保有足够内存
func (m *EngineManager) ensureMemory(ctx context.Context, requiredMB int) error {
	currentUsage := m.getCurrentMemoryUsage()
	available := m.config.MaxMemoryMB - currentUsage

	if available >= requiredMB {
		return nil
	}

	// 需要释放内存
	needToFree := requiredMB - available
	return m.freeMemory(ctx, needToFree)
}

// getCurrentMemoryUsage 获取当前内存使用
func (m *EngineManager) getCurrentMemoryUsage() int {
	total := 0
	for _, engine := range m.engines {
		if engine.IsLoaded() {
			total += engine.MemoryUsage()
		}
	}
	return total
}

// freeMemory 释放内存 (LRU 策略)
func (m *EngineManager) freeMemory(ctx context.Context, needMB int) error {
	// 按最后使用时间排序
	type engineWithTime struct {
		engine   Engine
		lastUsed time.Time
	}

	var loaded []engineWithTime
	for engineType, engine := range m.engines {
		if engine.IsLoaded() {
			// 跳过常驻引擎
			isResident := false
			for _, resident := range m.config.ResidentEngines {
				if resident == engineType {
					isResident = true
					break
				}
			}
			if isResident {
				continue
			}

			lastUsed := time.Now()
			if stats, ok := m.stats[engineType]; ok {
				lastUsed = stats.lastUsed
			}
			loaded = append(loaded, engineWithTime{engine, lastUsed})
		}
	}

	// 按最后使用时间升序排序 (最久未用的在前)
	sort.Slice(loaded, func(i, j int) bool {
		return loaded[i].lastUsed.Before(loaded[j].lastUsed)
	})

	// 逐个卸载直到释放足够内存
	freed := 0
	for _, item := range loaded {
		if freed >= needMB {
			break
		}

		memUsage := item.engine.MemoryUsage()
		if err := item.engine.Unload(); err != nil {
			continue
		}
		freed += memUsage
	}

	if freed < needMB {
		return fmt.Errorf("unable to free enough memory: need %dMB, freed %dMB", needMB, freed)
	}

	return nil
}

// watchMemory 内存监控
func (m *EngineManager) watchMemory() {
	for {
		select {
		case <-m.stopCh:
			return
		case <-m.memoryWatcher.C:
			m.cleanupIdleEngines()
		}
	}
}

// cleanupIdleEngines 清理空闲引擎
func (m *EngineManager) cleanupIdleEngines() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for engineType, engine := range m.engines {
		if !engine.IsLoaded() {
			continue
		}

		// 跳过常驻引擎
		isResident := false
		for _, resident := range m.config.ResidentEngines {
			if resident == engineType {
				isResident = true
				break
			}
		}
		if isResident {
			continue
		}

		// 检查空闲时间
		if stats, ok := m.stats[engineType]; ok {
			if now.Sub(stats.lastUsed) > m.config.IdleTimeout {
				_ = engine.Unload()
			}
		}
	}
}

// GetStatus 获取所有引擎状态
func (m *EngineManager) GetStatus() []EngineStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make([]EngineStatus, 0, len(m.engines))
	for engineType, engine := range m.engines {
		s := EngineStatus{
			Engine:    engineType,
			Available: true,
			Loaded:    engine.IsLoaded(),
			MemoryMB:  engine.MemoryUsage(),
		}

		if stats, ok := m.stats[engineType]; ok {
			s.LastUsed = stats.lastUsed
			s.TotalCalls = stats.totalCalls
			s.FailedCalls = stats.failedCalls
			if stats.totalCalls > 0 {
				s.AvgLatency = float64(stats.totalLatency) / float64(stats.totalCalls)
			}
		}

		status = append(status, s)
	}

	return status
}

// UpdateStats 更新引擎统计
func (m *EngineManager) UpdateStats(engineType EngineType, latencyMs int64, failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if stats, ok := m.stats[engineType]; ok {
		stats.lastUsed = time.Now()
		stats.totalCalls++
		stats.totalLatency += latencyMs
		if failed {
			stats.failedCalls++
		}
	}
}

// HealthCheck 健康检查
func (m *EngineManager) HealthCheck(ctx context.Context) map[EngineType]error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[EngineType]error)
	for engineType, engine := range m.engines {
		results[engineType] = engine.Health(ctx)
	}
	return results
}
