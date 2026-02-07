package engines

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

func TestDefaultBaiduOCRConfig(t *testing.T) {
	config := DefaultBaiduOCRConfig()
	if config.Timeout == 0 {
		t.Error("expected default timeout")
	}
}

func TestNewBaiduOCREngine(t *testing.T) {
	// 测试 nil 配置
	engine := NewBaiduOCREngine(nil)
	if engine == nil {
		t.Fatal("expected engine")
	}
	if engine.Name() != ocr.EngineBaidu {
		t.Errorf("expected baidu, got %s", engine.Name())
	}

	// 测试自定义配置
	config := &BaiduOCRConfig{
		APIKey:    "test-key",
		SecretKey: "test-secret",
	}
	engine = NewBaiduOCREngine(config)
	if engine.config.APIKey != "test-key" {
		t.Errorf("expected test-key")
	}
}

func TestBaiduOCREngineProperties(t *testing.T) {
	engine := NewBaiduOCREngine(nil)

	if engine.Name() != ocr.EngineBaidu {
		t.Errorf("expected baidu")
	}

	if engine.Layer() != ocr.LayerSmallModel {
		t.Errorf("expected small model layer")
	}

	scenes := engine.SupportedScenes()
	if len(scenes) == 0 {
		t.Error("expected supported scenes")
	}

	// 检查是否支持常见场景
	sceneMap := make(map[ocr.SceneType]bool)
	for _, s := range scenes {
		sceneMap[s] = true
	}

	if !sceneMap[ocr.SceneGeneral] {
		t.Error("should support general scene")
	}
	if !sceneMap[ocr.SceneInvoice] {
		t.Error("should support invoice scene")
	}
	if !sceneMap[ocr.SceneIDCard] {
		t.Error("should support idcard scene")
	}
}

func TestBaiduOCREngineMemoryUsage(t *testing.T) {
	engine := NewBaiduOCREngine(nil)
	memory := engine.MemoryUsage()
	if memory <= 0 {
		t.Error("expected positive memory usage")
	}
	// 云服务应该占用很少内存
	if memory > 50 {
		t.Error("cloud service should use minimal memory")
	}
}

func TestBaiduOCREngineLoadUnload(t *testing.T) {
	engine := NewBaiduOCREngine(&BaiduOCRConfig{
		APIKey:    "test-key",
		SecretKey: "test-secret",
	})

	if engine.IsLoaded() {
		t.Error("should not be loaded initially")
	}

	err := engine.Load(nil)
	if err != nil {
		t.Errorf("load should not fail with valid config: %v", err)
	}

	if !engine.IsLoaded() {
		t.Error("should be loaded after Load()")
	}

	err = engine.Unload()
	if err != nil {
		t.Errorf("unload should not fail: %v", err)
	}

	if engine.IsLoaded() {
		t.Error("should not be loaded after Unload()")
	}
}

func TestBaiduOCREngineLoadWithoutKey(t *testing.T) {
	engine := NewBaiduOCREngine(nil)

	err := engine.Load(nil)
	if err == nil {
		t.Error("expected error without API key")
	}
}

func TestBaiduOCREngineStats(t *testing.T) {
	engine := NewBaiduOCREngine(&BaiduOCRConfig{
		APIKey: "test",
	})

	stats := engine.Stats()
	if stats.Engine != ocr.EngineBaidu {
		t.Errorf("expected baidu")
	}
	if stats.TotalCalls != 0 {
		t.Errorf("expected 0 total calls")
	}
	if !stats.Available {
		t.Error("should be available with API key")
	}
}

func TestBaiduOCREngineHealth(t *testing.T) {
	// 没有 API Key
	engine := NewBaiduOCREngine(nil)
	err := engine.Health(nil)
	if err == nil {
		t.Error("expected error without API key")
	}

	// 有 API Key
	engine = NewBaiduOCREngine(&BaiduOCRConfig{
		APIKey:    "test",
		SecretKey: "test",
	})
	// 注意：这里不会真正调用 API，只是检查配置
}

func TestBaiduOCREngineWithMockServer(t *testing.T) {
	// 创建 mock 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth/2.0/token":
			resp := map[string]interface{}{
				"access_token": "mock-token",
				"expires_in":   3600,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/rest/2.0/ocr/v1/general_basic":
			resp := map[string]interface{}{
				"words_result_num": 2,
				"words_result": []map[string]string{
					{"words": "Hello"},
					{"words": "World"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// 注意：实际测试需要修改百度 OCR 引擎以支持自定义 endpoint
	// 这里只是展示测试结构
	t.Log("Mock server created at:", server.URL)
}

func TestBaiduOCRRecordFailure(t *testing.T) {
	engine := NewBaiduOCREngine(nil)

	engine.recordFailure()
	engine.recordFailure()

	stats := engine.Stats()
	if stats.FailedCalls != 2 {
		t.Errorf("expected 2 failed calls, got %d", stats.FailedCalls)
	}
}
