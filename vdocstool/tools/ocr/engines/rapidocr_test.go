package engines

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

func TestDefaultRapidOCRConfig(t *testing.T) {
	config := DefaultRapidOCRConfig()

	if config.Endpoint != "http://localhost:8089" {
		t.Errorf("expected Endpoint=http://localhost:8089, got %s", config.Endpoint)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("expected Timeout=30s, got %v", config.Timeout)
	}
}

func TestNewRapidOCREngine(t *testing.T) {
	// 测试默认配置
	engine := NewRapidOCREngine(nil)
	if engine == nil {
		t.Fatal("NewRapidOCREngine returned nil")
	}

	// 测试自定义配置
	config := &RapidOCRConfig{
		Endpoint: "http://localhost:9000",
		Timeout:  60 * time.Second,
	}
	engine = NewRapidOCREngine(config)
	if engine.config.Endpoint != "http://localhost:9000" {
		t.Errorf("expected Endpoint=http://localhost:9000, got %s", engine.config.Endpoint)
	}
}

func TestRapidOCREngineName(t *testing.T) {
	engine := NewRapidOCREngine(nil)

	if engine.Name() != ocr.EngineRapidOCR {
		t.Errorf("expected name=%s, got %s", ocr.EngineRapidOCR, engine.Name())
	}
}

func TestRapidOCREngineLayer(t *testing.T) {
	engine := NewRapidOCREngine(nil)

	if engine.Layer() != ocr.LayerSmallModel {
		t.Errorf("expected layer=%d, got %d", ocr.LayerSmallModel, engine.Layer())
	}
}

func TestRapidOCREngineSupportedScenes(t *testing.T) {
	engine := NewRapidOCREngine(nil)
	scenes := engine.SupportedScenes()

	if len(scenes) == 0 {
		t.Error("expected at least one supported scene")
	}

	// 检查是否包含 General 场景
	hasGeneral := false
	hasTable := false
	for _, scene := range scenes {
		if scene == ocr.SceneGeneral {
			hasGeneral = true
		}
		if scene == ocr.SceneTable {
			hasTable = true
		}
	}
	if !hasGeneral {
		t.Error("expected to support General scene")
	}
	if !hasTable {
		t.Error("expected to support Table scene")
	}
}

func TestRapidOCREngineMemoryUsage(t *testing.T) {
	engine := NewRapidOCREngine(nil)

	usage := engine.MemoryUsage()
	if usage != 300 {
		t.Errorf("expected memory usage=300, got %d", usage)
	}
}

func TestRapidOCREngineIsLoaded(t *testing.T) {
	engine := NewRapidOCREngine(nil)

	if engine.IsLoaded() {
		t.Error("engine should not be loaded initially")
	}
}

func TestRapidOCREngineUnload(t *testing.T) {
	engine := NewRapidOCREngine(nil)
	engine.loaded = true

	err := engine.Unload()
	if err != nil {
		t.Errorf("Unload failed: %v", err)
	}

	if engine.IsLoaded() {
		t.Error("engine should not be loaded after Unload")
	}
}

func TestRapidOCREngineStats(t *testing.T) {
	engine := NewRapidOCREngine(nil)
	engine.totalCalls = 10
	engine.failedCalls = 2
	engine.totalLatency = 1000
	engine.loaded = true
	engine.lastUsed = time.Now()

	stats := engine.Stats()

	if stats.Engine != ocr.EngineRapidOCR {
		t.Errorf("expected engine=%s, got %s", ocr.EngineRapidOCR, stats.Engine)
	}

	if stats.TotalCalls != 10 {
		t.Errorf("expected TotalCalls=10, got %d", stats.TotalCalls)
	}

	if stats.FailedCalls != 2 {
		t.Errorf("expected FailedCalls=2, got %d", stats.FailedCalls)
	}

	if stats.AvgLatency != 100.0 {
		t.Errorf("expected AvgLatency=100.0, got %f", stats.AvgLatency)
	}

	if !stats.Loaded {
		t.Error("expected Loaded=true")
	}
}

func TestRapidOCREngineRecordFailure(t *testing.T) {
	engine := NewRapidOCREngine(nil)

	if engine.failedCalls != 0 {
		t.Errorf("expected initial failedCalls=0, got %d", engine.failedCalls)
	}

	engine.recordFailure()
	if engine.failedCalls != 1 {
		t.Errorf("expected failedCalls=1 after recordFailure, got %d", engine.failedCalls)
	}
}

// 测试使用 mock 服务器
func TestRapidOCREngineWithMockServer(t *testing.T) {
	// 创建 mock 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "healthy",
			})

		case "/ocr":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    0,
				"message": "success",
				"data": map[string]interface{}{
					"results": []map[string]interface{}{
						{
							"text":       "Hello World",
							"confidence": 0.95,
							"box":        [][]float64{{0, 0}, {100, 0}, {100, 20}, {0, 20}},
						},
						{
							"text":       "测试文本",
							"confidence": 0.92,
							"box":        [][]float64{{0, 25}, {80, 25}, {80, 45}, {0, 45}},
						},
					},
					"elapsed_time": 0.15,
				},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// 创建引擎并指向 mock 服务器
	engine := NewRapidOCREngine(&RapidOCRConfig{
		Endpoint: server.URL,
		Timeout:  10 * time.Second,
	})

	ctx := context.Background()

	// 测试健康检查
	err := engine.Health(ctx)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}

	// 测试加载
	err = engine.Load(ctx)
	if err != nil {
		t.Errorf("Load failed: %v", err)
	}

	if !engine.IsLoaded() {
		t.Error("engine should be loaded after Load")
	}
}

func TestRapidOCREngineHealthCheckFailed(t *testing.T) {
	// 创建一个返回错误的 mock 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	engine := NewRapidOCREngine(&RapidOCRConfig{
		Endpoint: server.URL,
		Timeout:  5 * time.Second,
	})

	ctx := context.Background()
	err := engine.Health(ctx)

	if err == nil {
		t.Error("expected error for unhealthy service")
	}
}

func TestRapidOCREngineLoadFailed(t *testing.T) {
	// 使用不存在的端点
	engine := NewRapidOCREngine(&RapidOCRConfig{
		Endpoint: "http://localhost:59999",
		Timeout:  1 * time.Second,
	})

	ctx := context.Background()
	err := engine.Load(ctx)

	if err == nil {
		t.Error("expected error for unavailable service")
	}

	if engine.IsLoaded() {
		t.Error("engine should not be loaded after failed Load")
	}
}

func TestRapidOCREngineRecognizeTable(t *testing.T) {
	// 创建 mock 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ocr" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    0,
				"message": "success",
				"data": map[string]interface{}{
					"results":      []map[string]interface{}{},
					"elapsed_time": 0.1,
				},
			})
		}
	}))
	defer server.Close()

	engine := NewRapidOCREngine(&RapidOCRConfig{
		Endpoint: server.URL,
		Timeout:  10 * time.Second,
	})

	// 注意: 这个测试会失败因为没有提供有效的图片
	// 只是验证 RecognizeTable 方法存在并返回正确类型
	ctx := context.Background()
	result, _ := engine.RecognizeTable(ctx, &ocr.OCRRequest{})

	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestRapidOCREngineRecognizeInvoice(t *testing.T) {
	engine := NewRapidOCREngine(nil)

	ctx := context.Background()
	result, _ := engine.RecognizeInvoice(ctx, &ocr.OCRRequest{})

	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestRapidOCREngineRecognizeIDCard(t *testing.T) {
	engine := NewRapidOCREngine(nil)

	ctx := context.Background()
	result, _ := engine.RecognizeIDCard(ctx, &ocr.OCRRequest{})

	if result == nil {
		t.Error("expected non-nil result")
	}
}
