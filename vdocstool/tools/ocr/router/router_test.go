package router

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config.DefaultStrategy == "" {
		t.Error("expected default strategy")
	}
	if config.QualityThreshold <= 0 {
		t.Error("expected positive quality threshold")
	}
	if len(config.FallbackChain) == 0 {
		t.Error("expected fallback chain")
	}
}

// MockEngine 用于测试的模拟引擎
type MockEngine struct {
	name       ocr.EngineType
	layer      ocr.EngineLayer
	scenes     []ocr.SceneType
	loaded     bool
	memoryMB   int
	calls      int
	shouldFail bool
}

func (m *MockEngine) Name() ocr.EngineType         { return m.name }
func (m *MockEngine) Layer() ocr.EngineLayer       { return m.layer }
func (m *MockEngine) SupportedScenes() []ocr.SceneType { return m.scenes }
func (m *MockEngine) MemoryUsage() int             { return m.memoryMB }
func (m *MockEngine) IsLoaded() bool               { return m.loaded }
func (m *MockEngine) Load(ctx context.Context) error { 
	m.loaded = true 
	return nil 
}
func (m *MockEngine) Unload() error { 
	m.loaded = false 
	return nil 
}
func (m *MockEngine) Health(ctx context.Context) error { return nil }
func (m *MockEngine) Stats() ocr.EngineStatus {
	return ocr.EngineStatus{
		Engine:    m.name,
		Available: true,
		Loaded:    m.loaded,
		MemoryMB:  m.memoryMB,
	}
}
func (m *MockEngine) Recognize(ctx context.Context, req *ocr.OCRRequest) (*ocr.OCRResult, error) {
	m.calls++
	if m.shouldFail {
		return &ocr.OCRResult{Success: false, Error: "mock failure"}, nil
	}
	return &ocr.OCRResult{
		Success:  true,
		FullText: "Mock OCR Result",
		Engine:   m.name,
	}, nil
}
func (m *MockEngine) RecognizeTable(ctx context.Context, req *ocr.OCRRequest) (*ocr.TableResult, error) {
	return &ocr.TableResult{OCRResult: ocr.OCRResult{Success: true}}, nil
}
func (m *MockEngine) RecognizeInvoice(ctx context.Context, req *ocr.OCRRequest) (*ocr.InvoiceResult, error) {
	return &ocr.InvoiceResult{OCRResult: ocr.OCRResult{Success: true}}, nil
}
func (m *MockEngine) RecognizeIDCard(ctx context.Context, req *ocr.OCRRequest) (*ocr.IDCardResult, error) {
	return &ocr.IDCardResult{OCRResult: ocr.OCRResult{Success: true}}, nil
}

func createMockEngineManager() *ocr.EngineManager {
	config := &ocr.Config{
		MaxMemoryMB: 2000,
		IdleTimeout: time.Minute,
	}
	manager := ocr.NewEngineManager(config)
	
	// 注册模拟引擎
	manager.RegisterEngine(&MockEngine{
		name:     ocr.EngineTesseract,
		layer:    ocr.LayerTraditional,
		scenes:   []ocr.SceneType{ocr.SceneGeneral, ocr.ScenePrint},
		memoryMB: 100,
	})
	manager.RegisterEngine(&MockEngine{
		name:     ocr.EngineRapidOCR,
		layer:    ocr.LayerSmallModel,
		scenes:   []ocr.SceneType{ocr.SceneGeneral, ocr.ScenePrint, ocr.SceneDocument},
		memoryMB: 300,
	})
	manager.RegisterEngine(&MockEngine{
		name:     ocr.EngineQwenVL,
		layer:    ocr.LayerVLM,
		scenes:   []ocr.SceneType{ocr.SceneGeneral, ocr.ScenePrint, ocr.SceneDocument, ocr.SceneHandwriting},
		memoryMB: 10,
	})

	return manager
}

func TestNewRouter(t *testing.T) {
	manager := createMockEngineManager()
	router := NewRouter(nil, manager)

	if router == nil {
		t.Fatal("expected router")
	}
	if router.config == nil {
		t.Error("expected config")
	}
}

func TestRouterSupportsScene(t *testing.T) {
	manager := createMockEngineManager()
	router := NewRouter(nil, manager)

	// 测试不包含 SceneGeneral 的引擎
	mockEngine := &MockEngine{
		name:   ocr.EngineTesseract,
		scenes: []ocr.SceneType{ocr.ScenePrint, ocr.SceneDocument},
	}

	tests := []struct {
		scene    ocr.SceneType
		expected bool
	}{
		{ocr.ScenePrint, true},
		{ocr.SceneDocument, true},
		{ocr.SceneHandwriting, false},
		{"", true}, // 空场景支持所有引擎
	}

	for _, tc := range tests {
		result := router.supportsScene(mockEngine, tc.scene)
		if result != tc.expected {
			t.Errorf("supportsScene(%s) = %v, expected %v", tc.scene, result, tc.expected)
		}
	}
}

func TestRouterSetStrategy(t *testing.T) {
	manager := createMockEngineManager()
	router := NewRouter(nil, manager)

	router.SetStrategy(ocr.StrategyQuality)
	if router.config.DefaultStrategy != ocr.StrategyQuality {
		t.Errorf("expected quality strategy")
	}

	router.SetStrategy(ocr.StrategyCost)
	if router.config.DefaultStrategy != ocr.StrategyCost {
		t.Errorf("expected cost strategy")
	}
}

func TestRouterSetFallbackChain(t *testing.T) {
	manager := createMockEngineManager()
	router := NewRouter(nil, manager)

	chain := []ocr.EngineType{ocr.EngineTesseract, ocr.EngineRapidOCR}
	router.SetFallbackChain(chain)

	if len(router.config.FallbackChain) != 2 {
		t.Errorf("expected 2 engines in chain, got %d", len(router.config.FallbackChain))
	}
}

func TestRouterGetStats(t *testing.T) {
	manager := createMockEngineManager()
	router := NewRouter(nil, manager)

	stats := router.GetStats()
	if stats == nil {
		t.Error("expected stats map")
	}
}

func TestRouterRecordSuccess(t *testing.T) {
	manager := createMockEngineManager()
	router := NewRouter(nil, manager)

	router.recordSuccess(ocr.EngineTesseract, 100)
	router.recordSuccess(ocr.EngineTesseract, 200)

	stats := router.GetStats()
	if stats[ocr.EngineTesseract].SuccessCount != 2 {
		t.Errorf("expected 2 successes, got %d", stats[ocr.EngineTesseract].SuccessCount)
	}
	if stats[ocr.EngineTesseract].AvgLatency != 150 {
		t.Errorf("expected avg latency 150, got %f", stats[ocr.EngineTesseract].AvgLatency)
	}
}

func TestRouterRecordFailure(t *testing.T) {
	manager := createMockEngineManager()
	router := NewRouter(nil, manager)

	router.recordFailure(ocr.EngineTesseract)

	stats := router.GetStats()
	if stats[ocr.EngineTesseract].FailedCount != 1 {
		t.Errorf("expected 1 failure, got %d", stats[ocr.EngineTesseract].FailedCount)
	}
}

func TestRouterGetQuotaManager(t *testing.T) {
	manager := createMockEngineManager()
	router := NewRouter(&Config{EnableQuotaManager: true}, manager)

	qm := router.GetQuotaManager()
	if qm == nil {
		t.Error("expected quota manager")
	}
}

func TestRouterGetQuotaManagerDisabled(t *testing.T) {
	manager := createMockEngineManager()
	router := NewRouter(&Config{EnableQuotaManager: false}, manager)

	qm := router.GetQuotaManager()
	if qm != nil {
		t.Error("expected nil quota manager when disabled")
	}
}
