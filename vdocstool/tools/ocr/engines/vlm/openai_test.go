package vlm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

func TestDefaultOpenAIVLConfig(t *testing.T) {
	config := DefaultOpenAIVLConfig()
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

func TestNewOpenAIVLEngine(t *testing.T) {
	// 测试 nil 配置
	engine := NewOpenAIVLEngine(nil)
	if engine == nil {
		t.Fatal("expected engine")
	}
	if engine.Name() != ocr.EngineGPT4V {
		t.Errorf("expected gpt4v, got %s", engine.Name())
	}

	// 测试自定义配置
	config := &OpenAIVLConfig{
		APIKey: "sk-test-key",
		Model:  "gpt-4-vision-preview",
	}
	engine = NewOpenAIVLEngine(config)
	if engine.config.APIKey != "sk-test-key" {
		t.Errorf("expected sk-test-key")
	}
}

func TestOpenAIVLEngineProperties(t *testing.T) {
	engine := NewOpenAIVLEngine(nil)

	if engine.Name() != ocr.EngineGPT4V {
		t.Errorf("expected gpt4v")
	}

	if engine.Layer() != ocr.LayerVLM {
		t.Errorf("expected VLM layer")
	}

	scenes := engine.SupportedScenes()
	if len(scenes) == 0 {
		t.Error("expected supported scenes")
	}
}

func TestOpenAIVLEngineMemoryUsage(t *testing.T) {
	engine := NewOpenAIVLEngine(nil)
	memory := engine.MemoryUsage()
	if memory <= 0 {
		t.Error("expected positive memory usage")
	}
	if memory > 50 {
		t.Error("cloud service should use minimal memory")
	}
}

func TestOpenAIVLEngineLoadUnload(t *testing.T) {
	engine := NewOpenAIVLEngine(&OpenAIVLConfig{
		APIKey: "sk-test-key",
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
}

func TestOpenAIVLEngineLoadWithoutKey(t *testing.T) {
	engine := NewOpenAIVLEngine(nil)

	err := engine.Load(nil)
	if err == nil {
		t.Error("expected error without API key")
	}
}

func TestOpenAIVLEngineStats(t *testing.T) {
	engine := NewOpenAIVLEngine(&OpenAIVLConfig{
		APIKey: "sk-test",
	})

	stats := engine.Stats()
	if stats.Engine != ocr.EngineGPT4V {
		t.Errorf("expected gpt4v")
	}
	if stats.TotalCalls != 0 {
		t.Errorf("expected 0 total calls")
	}
	if !stats.Available {
		t.Error("should be available with API key")
	}
}

func TestOpenAIVLEngineHealth(t *testing.T) {
	// 没有 API Key
	engine := NewOpenAIVLEngine(nil)
	err := engine.Health(nil)
	if err == nil {
		t.Error("expected error without API key")
	}

	// 有 API Key
	engine = NewOpenAIVLEngine(&OpenAIVLConfig{APIKey: "sk-test"})
	err = engine.Health(nil)
	if err != nil {
		t.Errorf("health should pass with API key: %v", err)
	}
}

func TestOpenAIVLRecordFailure(t *testing.T) {
	engine := NewOpenAIVLEngine(nil)

	engine.recordFailure()
	engine.recordFailure()

	stats := engine.Stats()
	if stats.FailedCalls != 2 {
		t.Errorf("expected 2 failed calls, got %d", stats.FailedCalls)
	}
}

func TestOpenAIVLEngineWithMockServer(t *testing.T) {
	// 创建 mock 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "This is a test document containing text.",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &OpenAIVLConfig{
		APIKey:   "sk-test-key",
		Endpoint: server.URL,
	}
	engine := NewOpenAIVLEngine(config)

	// 测试 ChatWithSystem
	result, err := engine.ChatWithSystem(context.Background(), "dGVzdA==", "system prompt", "user prompt")
	if err != nil {
		t.Errorf("ChatWithSystem failed: %v", err)
	}
	if result != "This is a test document containing text." {
		t.Errorf("unexpected result: %s", result)
	}
}
