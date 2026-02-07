package vlm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

func TestDefaultQwenVLConfig(t *testing.T) {
	config := DefaultQwenVLConfig()
	if config.Model == "" {
		t.Error("expected default model")
	}
	if config.Endpoint == "" {
		t.Error("expected default endpoint")
	}
	if config.Timeout == 0 {
		t.Error("expected default timeout")
	}
}

func TestNewQwenVLEngine(t *testing.T) {
	// 测试 nil 配置
	engine := NewQwenVLEngine(nil)
	if engine == nil {
		t.Fatal("expected engine")
	}
	if engine.Name() != ocr.EngineQwenVL {
		t.Errorf("expected qwen_vl, got %s", engine.Name())
	}

	// 测试自定义配置
	config := &QwenVLConfig{
		APIKey: "test-key",
		Model:  "qwen-vl-plus",
	}
	engine = NewQwenVLEngine(config)
	if engine.config.APIKey != "test-key" {
		t.Errorf("expected test-key")
	}
	if engine.config.Model != "qwen-vl-plus" {
		t.Errorf("expected qwen-vl-plus")
	}
}

func TestQwenVLEngineProperties(t *testing.T) {
	engine := NewQwenVLEngine(nil)

	if engine.Name() != ocr.EngineQwenVL {
		t.Errorf("expected qwen_vl")
	}

	if engine.Layer() != ocr.LayerVLM {
		t.Errorf("expected VLM layer")
	}

	scenes := engine.SupportedScenes()
	if len(scenes) == 0 {
		t.Error("expected supported scenes")
	}

	// VLM 应该支持所有场景
	expectedScenes := []ocr.SceneType{
		ocr.SceneGeneral,
		ocr.SceneDocument,
		ocr.SceneTable,
		ocr.SceneInvoice,
		ocr.SceneIDCard,
		ocr.SceneHandwriting,
	}

	sceneMap := make(map[ocr.SceneType]bool)
	for _, s := range scenes {
		sceneMap[s] = true
	}

	for _, expected := range expectedScenes {
		if !sceneMap[expected] {
			t.Errorf("expected support for %s", expected)
		}
	}
}

func TestQwenVLEngineMemoryUsage(t *testing.T) {
	engine := NewQwenVLEngine(nil)
	memory := engine.MemoryUsage()
	if memory <= 0 {
		t.Error("expected positive memory usage")
	}
	// 云服务应该占用很少内存
	if memory > 50 {
		t.Error("cloud service should use minimal memory")
	}
}

func TestQwenVLEngineLoadUnload(t *testing.T) {
	engine := NewQwenVLEngine(&QwenVLConfig{
		APIKey: "test-key",
	})

	if engine.IsLoaded() {
		t.Error("should not be loaded initially")
	}

	err := engine.Load(nil)
	if err != nil {
		t.Errorf("load should not fail with API key: %v", err)
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

func TestQwenVLEngineLoadWithoutKey(t *testing.T) {
	engine := NewQwenVLEngine(nil)

	err := engine.Load(nil)
	if err == nil {
		t.Error("expected error without API key")
	}
}

func TestQwenVLEngineStats(t *testing.T) {
	engine := NewQwenVLEngine(&QwenVLConfig{
		APIKey: "test",
	})

	stats := engine.Stats()
	if stats.Engine != ocr.EngineQwenVL {
		t.Errorf("expected qwen_vl")
	}
	if stats.TotalCalls != 0 {
		t.Errorf("expected 0 total calls")
	}
	if !stats.Available {
		t.Error("should be available with API key")
	}
}

func TestQwenVLEngineHealth(t *testing.T) {
	// 没有 API Key
	engine := NewQwenVLEngine(nil)
	err := engine.Health(nil)
	if err == nil {
		t.Error("expected error without API key")
	}

	// 有 API Key
	engine = NewQwenVLEngine(&QwenVLConfig{APIKey: "test"})
	err = engine.Health(nil)
	if err != nil {
		t.Errorf("health should pass with API key: %v", err)
	}
}

func TestQwenVLRecordFailure(t *testing.T) {
	engine := NewQwenVLEngine(nil)

	engine.recordFailure()

	stats := engine.Stats()
	if stats.FailedCalls != 1 {
		t.Errorf("expected 1 failed call, got %d", stats.FailedCalls)
	}
}

func TestQwenVLEngineWithMockServer(t *testing.T) {
	// 创建 mock 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"output": map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"content": []map[string]string{
								{"text": "识别结果:\nHello World"},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &QwenVLConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
	}
	engine := NewQwenVLEngine(config)

	// 测试 ChatWithSystem
	result, err := engine.ChatWithSystem(context.Background(), "dGVzdA==", "system prompt", "user prompt")
	if err != nil {
		t.Errorf("ChatWithSystem failed: %v", err)
	}
	if result != "识别结果:\nHello World" {
		t.Errorf("unexpected result: %s", result)
	}
}
