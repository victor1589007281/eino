package vlm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

func TestDefaultClaudeVLConfig(t *testing.T) {
	config := DefaultClaudeVLConfig()
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

func TestNewClaudeVLEngine(t *testing.T) {
	// 测试 nil 配置
	engine := NewClaudeVLEngine(nil)
	if engine == nil {
		t.Fatal("expected engine")
	}
	if engine.Name() != ocr.EngineClaude {
		t.Errorf("expected claude, got %s", engine.Name())
	}

	// 测试自定义配置
	config := &ClaudeVLConfig{
		APIKey: "sk-ant-test-key",
		Model:  "claude-3-opus-20240229",
	}
	engine = NewClaudeVLEngine(config)
	if engine.config.APIKey != "sk-ant-test-key" {
		t.Errorf("expected sk-ant-test-key")
	}
}

func TestClaudeVLEngineProperties(t *testing.T) {
	engine := NewClaudeVLEngine(nil)

	if engine.Name() != ocr.EngineClaude {
		t.Errorf("expected claude")
	}

	if engine.Layer() != ocr.LayerVLM {
		t.Errorf("expected VLM layer")
	}

	scenes := engine.SupportedScenes()
	if len(scenes) == 0 {
		t.Error("expected supported scenes")
	}
}

func TestClaudeVLEngineMemoryUsage(t *testing.T) {
	engine := NewClaudeVLEngine(nil)
	memory := engine.MemoryUsage()
	if memory <= 0 {
		t.Error("expected positive memory usage")
	}
	if memory > 50 {
		t.Error("cloud service should use minimal memory")
	}
}

func TestClaudeVLEngineLoadUnload(t *testing.T) {
	engine := NewClaudeVLEngine(&ClaudeVLConfig{
		APIKey: "sk-ant-test-key",
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

func TestClaudeVLEngineLoadWithoutKey(t *testing.T) {
	engine := NewClaudeVLEngine(nil)

	err := engine.Load(nil)
	if err == nil {
		t.Error("expected error without API key")
	}
}

func TestClaudeVLEngineStats(t *testing.T) {
	engine := NewClaudeVLEngine(&ClaudeVLConfig{
		APIKey: "sk-ant-test",
	})

	stats := engine.Stats()
	if stats.Engine != ocr.EngineClaude {
		t.Errorf("expected claude")
	}
	if stats.TotalCalls != 0 {
		t.Errorf("expected 0 total calls")
	}
	if !stats.Available {
		t.Error("should be available with API key")
	}
}

func TestClaudeVLEngineHealth(t *testing.T) {
	// 没有 API Key
	engine := NewClaudeVLEngine(nil)
	err := engine.Health(nil)
	if err == nil {
		t.Error("expected error without API key")
	}

	// 有 API Key
	engine = NewClaudeVLEngine(&ClaudeVLConfig{APIKey: "sk-ant-test"})
	err = engine.Health(nil)
	if err != nil {
		t.Errorf("health should pass with API key: %v", err)
	}
}

func TestClaudeVLRecordFailure(t *testing.T) {
	engine := NewClaudeVLEngine(nil)

	engine.recordFailure()

	stats := engine.Stats()
	if stats.FailedCalls != 1 {
		t.Errorf("expected 1 failed call, got %d", stats.FailedCalls)
	}
}

func TestClaudeVLEngineWithMockServer(t *testing.T) {
	// 创建 mock 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "Document text extracted successfully.",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &ClaudeVLConfig{
		APIKey:   "sk-ant-test-key",
		Endpoint: server.URL,
	}
	engine := NewClaudeVLEngine(config)

	// 测试 ChatWithSystem
	result, err := engine.ChatWithSystem(context.Background(), "dGVzdA==", "system prompt", "user prompt")
	if err != nil {
		t.Errorf("ChatWithSystem failed: %v", err)
	}
	if result != "Document text extracted successfully." {
		t.Errorf("unexpected result: %s", result)
	}
}
