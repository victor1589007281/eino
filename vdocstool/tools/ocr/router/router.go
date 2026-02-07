// Package router OCR 智能路由
package router

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
	"github.com/cloudwego/eino/vdocstool/tools/ocr/quota"
)

// Router OCR 智能路由器
type Router struct {
	config         *Config
	engineManager  *ocr.EngineManager
	sceneClassifier *SceneClassifier
	quotaManager   *quota.Manager
	mu             sync.RWMutex
	routingStats   map[ocr.EngineType]*RoutingStats
}

// Config 路由配置
type Config struct {
	// 默认策略
	DefaultStrategy ocr.RoutingStrategy
	// 质量阈值 (图像质量低于此值时使用更强的引擎)
	QualityThreshold float64
	// 回退链
	FallbackChain []ocr.EngineType
	// 是否启用场景分类
	EnableSceneClassifier bool
	// 是否启用配额管理
	EnableQuotaManager bool
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		DefaultStrategy:       ocr.StrategySmart,
		QualityThreshold:      0.5,
		EnableSceneClassifier: true,
		EnableQuotaManager:    true,
		FallbackChain: []ocr.EngineType{
			ocr.EngineRapidOCR,
			ocr.EngineTesseract,
			ocr.EngineBaidu,
			ocr.EngineTencent,
			ocr.EngineQwenVL,
		},
	}
}

// RoutingStats 路由统计
type RoutingStats struct {
	TotalRouted   int64
	SuccessCount  int64
	FailedCount   int64
	AvgLatency    float64
	TotalLatency  int64
}

// NewRouter 创建路由器
func NewRouter(config *Config, engineManager *ocr.EngineManager) *Router {
	if config == nil {
		config = DefaultConfig()
	}

	r := &Router{
		config:        config,
		engineManager: engineManager,
		routingStats:  make(map[ocr.EngineType]*RoutingStats),
	}

	if config.EnableSceneClassifier {
		r.sceneClassifier = NewSceneClassifier()
	}

	if config.EnableQuotaManager {
		r.quotaManager = quota.NewManager(nil)
	}

	return r
}

// RouteRequest 路由 OCR 请求
func (r *Router) RouteRequest(ctx context.Context, req *ocr.OCRRequest, strategy ocr.RoutingStrategy) (ocr.Engine, error) {
	if strategy == "" {
		strategy = r.config.DefaultStrategy
	}

	// 获取候选引擎
	candidates := r.getCandidateEngines(ctx, req)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available engines")
	}

	// 根据策略选择引擎
	switch strategy {
	case ocr.StrategyQuality:
		return r.selectByQuality(ctx, req, candidates)
	case ocr.StrategyCost:
		return r.selectByCost(ctx, candidates)
	case ocr.StrategySpeed:
		return r.selectBySpeed(ctx, candidates)
	case ocr.StrategySmart:
		return r.selectSmart(ctx, req, candidates)
	default:
		return r.selectSmart(ctx, req, candidates)
	}
}

// getCandidateEngines 获取支持请求场景的候选引擎
func (r *Router) getCandidateEngines(ctx context.Context, req *ocr.OCRRequest) []ocr.Engine {
	var candidates []ocr.Engine
	
	statuses := r.engineManager.GetStatus()
	for _, status := range statuses {
		if !status.Available {
			continue
		}

		engine, err := r.engineManager.GetEngine(ctx, status.Engine)
		if err != nil {
			continue
		}

		// 检查是否支持该场景
		if r.supportsScene(engine, req.Scene) {
			candidates = append(candidates, engine)
		}
	}

	return candidates
}

// supportsScene 检查引擎是否支持场景
func (r *Router) supportsScene(engine ocr.Engine, scene ocr.SceneType) bool {
	if scene == "" {
		return true // 未指定场景时所有引擎都可以
	}

	for _, s := range engine.SupportedScenes() {
		if s == scene || s == ocr.SceneGeneral {
			return true
		}
	}
	return false
}

// selectByQuality 按质量选择 (优先使用 VLM)
func (r *Router) selectByQuality(ctx context.Context, req *ocr.OCRRequest, candidates []ocr.Engine) (ocr.Engine, error) {
	// 优先顺序: VLM > 云服务 > 小模型 > 传统
	layerPriority := map[ocr.EngineLayer]int{
		ocr.LayerVLM:         4,
		ocr.LayerSmallModel:  2,
		ocr.LayerTraditional: 1,
	}

	var best ocr.Engine
	bestPriority := 0

	for _, engine := range candidates {
		// 检查配额
		if r.quotaManager != nil {
			if ok, _ := r.quotaManager.CheckQuota(engine.Name()); !ok {
				continue
			}
		}

		priority := layerPriority[engine.Layer()]
		if priority > bestPriority {
			best = engine
			bestPriority = priority
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no available engine for quality strategy")
	}

	return best, nil
}

// selectByCost 按成本选择 (优先使用免费/本地)
func (r *Router) selectByCost(ctx context.Context, candidates []ocr.Engine) (ocr.Engine, error) {
	// 成本优先级 (越低越好)
	costPriority := map[ocr.EngineType]int{
		ocr.EngineTesseract:  1,  // 完全免费
		ocr.EngineRapidOCR:   2,  // 免费
		ocr.EnginePaddleOCR:  3,  // 免费
		ocr.EngineBaidu:      10, // 有免费额度
		ocr.EngineTencent:    10,
		ocr.EngineQwenVL:     20,
		ocr.EngineGPT4V:      30, // 最贵
		ocr.EngineClaude:     30,
	}

	var best ocr.Engine
	bestCost := 100

	for _, engine := range candidates {
		cost := costPriority[engine.Name()]
		if cost == 0 {
			cost = 50 // 未知引擎
		}

		if cost < bestCost {
			// 检查配额
			if r.quotaManager != nil {
				if ok, _ := r.quotaManager.CheckQuota(engine.Name()); !ok {
					continue
				}
			}
			best = engine
			bestCost = cost
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no available engine for cost strategy")
	}

	return best, nil
}

// selectBySpeed 按速度选择 (基于历史延迟)
func (r *Router) selectBySpeed(ctx context.Context, candidates []ocr.Engine) (ocr.Engine, error) {
	var best ocr.Engine
	bestLatency := float64(999999)

	for _, engine := range candidates {
		// 检查配额
		if r.quotaManager != nil {
			if ok, _ := r.quotaManager.CheckQuota(engine.Name()); !ok {
				continue
			}
		}

		stats := engine.Stats()
		latency := stats.AvgLatency
		if latency == 0 {
			// 未使用过的引擎给一个默认值
			switch engine.Layer() {
			case ocr.LayerTraditional:
				latency = 500
			case ocr.LayerSmallModel:
				latency = 1000
			case ocr.LayerVLM:
				latency = 3000
			}
		}

		if latency < bestLatency {
			best = engine
			bestLatency = latency
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no available engine for speed strategy")
	}

	return best, nil
}

// selectSmart 智能选择
func (r *Router) selectSmart(ctx context.Context, req *ocr.OCRRequest, candidates []ocr.Engine) (ocr.Engine, error) {
	// 1. 根据图像质量评估
	var quality *ocr.ImageQuality
	if req.ImagePath != "" || req.ImageURL != "" || req.ImageBase64 != "" {
		preprocessor := ocr.NewPreprocessor(nil)
		imgData, err := preprocessor.LoadImage(ctx, req)
		if err == nil {
			quality = preprocessor.AssessQuality(imgData)
		}
	}

	// 2. 根据场景分类 (如果启用)
	scene := req.Scene
	if scene == "" && r.sceneClassifier != nil && quality != nil {
		scene = r.sceneClassifier.Classify(quality)
	}

	// 3. 根据质量和场景选择引擎
	if quality != nil && quality.Score < r.config.QualityThreshold {
		// 低质量图像使用更强的引擎
		return r.selectByQuality(ctx, req, candidates)
	}

	// 4. 根据场景选择
	switch scene {
	case ocr.SceneHandwriting:
		// 手写体优先使用 VLM
		return r.selectByQuality(ctx, req, candidates)
	case ocr.SceneInvoice, ocr.SceneIDCard:
		// 票据/证件优先使用云服务
		for _, engine := range candidates {
			if engine.Layer() == ocr.LayerSmallModel {
				if r.quotaManager != nil {
					if ok, _ := r.quotaManager.CheckQuota(engine.Name()); !ok {
						continue
					}
				}
				return engine, nil
			}
		}
	case ocr.SceneTable:
		// 表格优先使用专业引擎
		for _, engine := range candidates {
			if engine.Name() == ocr.EnginePaddleOCR {
				return engine, nil
			}
		}
	}

	// 5. 默认按成本选择
	return r.selectByCost(ctx, candidates)
}

// ExecuteWithFallback 执行 OCR 并支持回退
func (r *Router) ExecuteWithFallback(ctx context.Context, req *ocr.OCRRequest, strategy ocr.RoutingStrategy) (*ocr.OCRResult, error) {
	startTime := time.Now()

	// 获取初始引擎
	engine, err := r.RouteRequest(ctx, req, strategy)
	if err != nil {
		return nil, err
	}

	// 执行识别
	result, err := r.executeEngine(ctx, engine, req)
	if err == nil && result.Success {
		r.recordSuccess(engine.Name(), time.Since(startTime).Milliseconds())
		return result, nil
	}

	// 尝试回退链
	tried := map[ocr.EngineType]bool{engine.Name(): true}

	for _, fallbackType := range r.config.FallbackChain {
		if tried[fallbackType] {
			continue
		}

		fallbackEngine, err := r.engineManager.GetEngine(ctx, fallbackType)
		if err != nil {
			continue
		}

		// 检查配额
		if r.quotaManager != nil {
			if ok, _ := r.quotaManager.CheckQuota(fallbackType); !ok {
				continue
			}
		}

		result, err = r.executeEngine(ctx, fallbackEngine, req)
		if err == nil && result.Success {
			r.recordSuccess(fallbackType, time.Since(startTime).Milliseconds())
			return result, nil
		}

		tried[fallbackType] = true
	}

	// 所有引擎都失败
	r.recordFailure(engine.Name())
	if result != nil {
		return result, fmt.Errorf("all engines failed, last error: %s", result.Error)
	}
	return nil, fmt.Errorf("all engines failed")
}

// executeEngine 执行单个引擎
func (r *Router) executeEngine(ctx context.Context, engine ocr.Engine, req *ocr.OCRRequest) (*ocr.OCRResult, error) {
	// 确保引擎已加载
	if !engine.IsLoaded() {
		if err := engine.Load(ctx); err != nil {
			return nil, err
		}
	}

	// 消耗配额
	if r.quotaManager != nil {
		_ = r.quotaManager.UseQuota(engine.Name())
	}

	return engine.Recognize(ctx, req)
}

// recordSuccess 记录成功
func (r *Router) recordSuccess(engine ocr.EngineType, latency int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats, ok := r.routingStats[engine]
	if !ok {
		stats = &RoutingStats{}
		r.routingStats[engine] = stats
	}

	stats.TotalRouted++
	stats.SuccessCount++
	stats.TotalLatency += latency
	stats.AvgLatency = float64(stats.TotalLatency) / float64(stats.SuccessCount)
}

// recordFailure 记录失败
func (r *Router) recordFailure(engine ocr.EngineType) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats, ok := r.routingStats[engine]
	if !ok {
		stats = &RoutingStats{}
		r.routingStats[engine] = stats
	}

	stats.TotalRouted++
	stats.FailedCount++
}

// GetStats 获取路由统计
func (r *Router) GetStats() map[ocr.EngineType]*RoutingStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[ocr.EngineType]*RoutingStats)
	for k, v := range r.routingStats {
		copied := *v
		result[k] = &copied
	}
	return result
}

// SetStrategy 设置默认策略
func (r *Router) SetStrategy(strategy ocr.RoutingStrategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.DefaultStrategy = strategy
}

// SetFallbackChain 设置回退链
func (r *Router) SetFallbackChain(chain []ocr.EngineType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.FallbackChain = chain
}

// GetQuotaManager 获取配额管理器
func (r *Router) GetQuotaManager() *quota.Manager {
	return r.quotaManager
}
